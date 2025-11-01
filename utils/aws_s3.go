package utils

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// CheckTenderFolderExists checks if a folder exists in S3
func CheckTenderFolderExists(bucket, tenderID string) (bool, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return false, fmt.Errorf("unable to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	prefix := fmt.Sprintf("tender-documents/%s/", tenderID)

	resp, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, fmt.Errorf("failed to list objects: %w", err)
	}
	return len(resp.Contents) > 0, nil
}
