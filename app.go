package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Setting struct {
	Bucket          string
	Prefix          string
	StartAfter      string
	ModifiedBefore  *time.Time
	LowerExtensions []string
	Parallelism     int
	FunctionName    string
	DryRun          bool
}

type Summary struct {
	Total      uint64
	Done       uint64
	Skipped    uint64
	Errored    uint64
	DurationMs uint64
}

func buildS3EventPayload(cfg *aws.Config, bucket string, obj *types.Object) ([]byte, error) {
	region := "us-east-1"
	if cfg != nil {
		region = cfg.Region
	}

	objectRecord := events.S3Object{}
	if obj.Key != nil {
		objectRecord.Key = *obj.Key
	}
	if obj.Size != nil {
		objectRecord.Size = *obj.Size
	}
	if obj.ETag != nil {
		objectRecord.ETag = *obj.ETag
	}

	event := events.S3Event{
		Records: []events.S3EventRecord{
			{
				EventVersion: "2.1",
				EventSource:  "aws:s3",
				AWSRegion:    region,
				EventTime:    time.Now(),
				EventName:    "ObjectCreated:Put",

				RequestParameters: events.S3RequestParameters{
					SourceIPAddress: "127.0.0.1",
				},
				ResponseElements: map[string]string{
					"x-amz-request-id": "s3-reinvoke-lambda",
					"x-amz-id-2":       "s3-reinvoke-lambda",
				},
				S3: events.S3Entity{
					SchemaVersion:   "1.0",
					ConfigurationID: "s3-reinvoke-lambda",
					Bucket: events.S3Bucket{
						Name: bucket,
						Arn:  fmt.Sprintf("arn:aws:s3:::%s", bucket),
					},
					Object: objectRecord,
				},
			},
		},
	}

	return json.Marshal(event)
}

func Run(ctx context.Context, setting Setting, client IAwsClient) (*Summary, error) {
	cfg := client.GetConfig()
	var continuationToken *string
	var wg sync.WaitGroup
	sem := make(chan struct{}, setting.Parallelism)
	apiCtx := context.Background()
	var summary Summary
	hasExts := len(setting.LowerExtensions) > 0

All:
	for {
		listInput := &s3.ListObjectsV2Input{
			Bucket:            &setting.Bucket,
			Prefix:            &setting.Prefix,
			StartAfter:        &setting.StartAfter,
			ContinuationToken: continuationToken,
			MaxKeys:           aws.Int32(1000),
		}

		listOutput, err := client.ListObjectsV2(apiCtx, listInput)
		if err != nil {
			wg.Wait()
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range listOutput.Contents {
			objCopy := obj
			summary.Total++

			select {
			case <-ctx.Done():
				break All
			default:
				key := *obj.Key

				// Extension filter
				if hasExts {
					lowerExt := path.Ext(key)
					if !slices.Contains(setting.LowerExtensions, lowerExt) {
						summary.Skipped++
						continue
					}
				}

				// Modified before filter
				if setting.ModifiedBefore != nil && obj.LastModified != nil {
					if setting.ModifiedBefore.Before(*obj.LastModified) {
						summary.Skipped++
						continue
					}
				}

				// Invoke lambda function
				wg.Add(1)
				sem <- struct{}{}
				go func(key string) {
					defer wg.Done()
					defer func() { <-sem }()

					payload, err := buildS3EventPayload(cfg, setting.Bucket, &objCopy)
					if err != nil {
						slog.Error("failed to build S3 event payload", "key", key, "error", err)
						return
					}

					if setting.DryRun {
						summary.Done++
						slog.Info("dry run (no invocation)", "key", key)
					} else {
						invokeInput := &lambda.InvokeInput{
							FunctionName: &setting.FunctionName,
							Payload:      payload,
						}
						started := time.Now()
						_, err = client.Invoke(apiCtx, invokeInput)
						if err != nil {
							summary.Errored++
							slog.Error("errored", "key", key, "error", err)
						} else {
							summary.Done++
							summary.DurationMs += uint64(time.Since(started).Milliseconds())
							slog.Info("done", "key", key)
						}
					}
				}(key)
			}
		}

		if *listOutput.IsTruncated {
			continuationToken = listOutput.NextContinuationToken
		} else {
			break
		}
	}

	wg.Wait()

	return &summary, nil
}
