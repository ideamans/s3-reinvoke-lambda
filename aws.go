package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type IAwsClient interface {
	GetConfig() *aws.Config
	ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
	Invoke(ctx context.Context, input *lambda.InvokeInput) (*lambda.InvokeOutput, error)
}

type DefaultAwsClient struct {
	config       *aws.Config
	s3Client     *s3.Client
	lambdaClient *lambda.Client
}

func NewAwsClient() (*DefaultAwsClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &DefaultAwsClient{
		config:       &cfg,
		s3Client:     s3.NewFromConfig(cfg),
		lambdaClient: lambda.NewFromConfig(cfg),
	}, nil
}

func (c *DefaultAwsClient) GetConfig() *aws.Config {
	return c.config
}

func (c *DefaultAwsClient) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
	return c.s3Client.ListObjectsV2(ctx, input)
}

func (c *DefaultAwsClient) Invoke(ctx context.Context, input *lambda.InvokeInput) (*lambda.InvokeOutput, error) {
	return c.lambdaClient.Invoke(ctx, input)
}
