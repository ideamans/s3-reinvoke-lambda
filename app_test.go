package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type NormalSystemAwsClientMock struct {
	t           *testing.T
	InvokedKeys []string
}

func NewNormalSystemAwsClientMock(t *testing.T) *NormalSystemAwsClientMock {
	return &NormalSystemAwsClientMock{
		t: t,
	}
}

func (c *NormalSystemAwsClientMock) GetConfig() *aws.Config {
	return &aws.Config{
		Region: "us-east-1",
	}
}

func (c *NormalSystemAwsClientMock) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
	if *input.Bucket != "my-bucket" {
		c.t.Fatalf("Unexpected bucket: %s", *input.Bucket)
	}
	if *input.Prefix != "my-prefix" {
		c.t.Fatalf("Unexpected prefix: %s", *input.Prefix)
	}
	if *input.StartAfter != "my-prefix/0.jpg" {
		c.t.Fatalf("Unexpected start after: %s", *input.StartAfter)
	}

	old := time.Date(2024, 6, 19, 0, 0, 0, 0, time.UTC)
	new := time.Date(2024, 6, 21, 0, 0, 0, 0, time.UTC)

	if input.ContinuationToken == nil || *input.ContinuationToken == "" {
		// First call
		return &s3.ListObjectsV2Output{
			Contents: []types.Object{
				// Target
				{
					Key:          aws.String("my-prefix/1.jpg"),
					LastModified: &old,
				},
				// Not target because of extension
				{
					Key:          aws.String("my-prefix/2.txt"),
					LastModified: &old,
				},
				// Target
				{
					Key:          aws.String("my-prefix/3.png"),
					LastModified: &old,
				},
				// Not target because of modified date
				{
					Key:          aws.String("my-prefix/new.jpg"),
					LastModified: &new,
				},
			},
			NextContinuationToken: aws.String("my-continuation-token"),
			IsTruncated:           aws.Bool(true),
		}, nil
	} else {
		// Second call
		if *input.ContinuationToken != "my-continuation-token" {
			c.t.Fatalf("Unexpected continuation token: %s", *input.ContinuationToken)
		}

		return &s3.ListObjectsV2Output{
			Contents: []types.Object{
				// Target
				{
					Key:          aws.String("my-prefix/4.jpg"),
					LastModified: &old,
				},
				// Target but expected error in Invoke
				{
					Key:          aws.String("my-prefix/error.jpg"),
					LastModified: &old,
				},
			},
			IsTruncated: aws.Bool(false),
		}, nil
	}
}

func (c *NormalSystemAwsClientMock) Invoke(ctx context.Context, input *lambda.InvokeInput) (*lambda.InvokeOutput, error) {
	if *input.FunctionName != "my-function" {
		c.t.Fatalf("Unexpected function name: %s", *input.FunctionName)
	}

	var event events.S3Event
	err := json.Unmarshal(input.Payload, &event)
	if err != nil {
		c.t.Fatalf("Failed to unmarshal payload %v: %v", input.Payload, err)
	}

	key := event.Records[0].S3.Object.Key
	if key == "my-prefix/error.jpg" {
		return nil, fmt.Errorf("expected error")
	}

	c.InvokedKeys = append(c.InvokedKeys, event.Records[0].S3.Object.Key)

	// dummy sleep
	time.Sleep(100 * time.Millisecond)

	return nil, nil
}

func TestNormalSituation(t *testing.T) {
	before := time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC)
	setting := Setting{
		Bucket:          "my-bucket",
		Prefix:          "my-prefix",
		StartAfter:      "my-prefix/0.jpg",
		ModifiedBefore:  &before,
		LowerExtensions: []string{".jpg", ".png"},
		Parallelism:     100,
		FunctionName:    "my-function",
	}

	mock := NewNormalSystemAwsClientMock(t)
	summary, err := Run(context.Background(), setting, mock)
	if err != nil {
		t.Fatalf("An error occurred: %v", err)
	}

	if summary.Total != 6 {
		t.Fatalf("Unexpected total in summary: %d", summary.Total)
	}
	if summary.Done != 3 {
		t.Fatalf("Unexpected done in summary: %d", summary.Done)
	}
	if summary.Skipped != 2 {
		t.Fatalf("Unexpected skipped in summary: %d", summary.Skipped)
	}
	if summary.Errored != 1 {
		t.Fatalf("Unexpected errored in summary: %d", summary.Errored)
	}

	if len(mock.InvokedKeys) != 3 {
		t.Fatalf("Unexpected number of invocations: %d", len(mock.InvokedKeys))
	}

	slices.Sort[[]string](mock.InvokedKeys)
	if !reflect.DeepEqual(mock.InvokedKeys, []string{"my-prefix/1.jpg", "my-prefix/3.png", "my-prefix/4.jpg"}) {
		t.Fatalf("Unexpected invoked keys: %v", mock.InvokedKeys)
	}
}
