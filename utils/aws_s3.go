package utils

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// NewS3Client builds an S3 client from the default AWS config.
//
// If AWS_ENDPOINT_URL is set (e.g. http://localstack:4566), the client is
// pointed at that endpoint with path-style addressing enabled — this is what
// LocalStack and other S3-compatible emulators require. When the variable is
// empty, the client talks to real AWS exactly as before.
func NewS3Client(ctx context.Context) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %w", err)
	}

	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	if endpoint == "" {
		return s3.NewFromConfig(cfg), nil
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	}), nil
}

// CheckTenderFolderExists checks if a folder exists in S3
func CheckTenderFolderExists(ctx context.Context, bucket, tenderID string) (bool, error) {
	client, err := NewS3Client(ctx)
	if err != nil {
		return false, err
	}

	prefix := fmt.Sprintf("tender-documents/%s/", tenderID)

	resp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, fmt.Errorf("failed to list objects: %w", err)
	}
	return len(resp.Contents) > 0, nil
}
