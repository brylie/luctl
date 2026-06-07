// Package backup implements S3-compatible backup and restore for Luanti server data.
package backup

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/brylie/luctl/internal/project"
)

// ErrMissingCredentials is returned when S3 credential env vars are not set.
var ErrMissingCredentials = errors.New(
	"LUCTL_S3_ACCESS_KEY and LUCTL_S3_SECRET_KEY must be set",
)

// NewS3Client constructs an S3 client from the project backup config and env vars.
// Credentials are read from LUCTL_S3_ACCESS_KEY / LUCTL_S3_SECRET_KEY only.
func NewS3Client(cfg project.BackupConfig) (*s3.Client, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("backup.bucket is not configured in luanti.toml")
	}

	if cfg.Endpoint == "" {
		return nil, errors.New("backup.endpoint is not configured in luanti.toml")
	}

	accessKey := os.Getenv("LUCTL_S3_ACCESS_KEY")
	secretKey := os.Getenv("LUCTL_S3_SECRET_KEY")

	if accessKey == "" || secretKey == "" {
		return nil, ErrMissingCredentials
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")

	awsCfg := aws.Config{
		Region:      region,
		Credentials: creds,
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		// Required for DigitalOcean Spaces and most non-AWS S3 providers.
		o.UsePathStyle = true
	})

	return client, nil
}

// ValidateConfig checks that all required backup fields are present.
func ValidateConfig(cfg project.BackupConfig) error {
	if cfg.Bucket == "" {
		return errors.New("backup.bucket is required in luanti.toml")
	}

	if cfg.Endpoint == "" {
		return errors.New("backup.endpoint is required in luanti.toml")
	}

	return nil
}

// objectKey returns the full S3 object key for a backup file name.
func objectKey(prefix, name string) string {
	if prefix == "" {
		return name
	}

	return fmt.Sprintf("%s%s", prefix, name)
}

// headObject checks whether an object exists in S3 and returns its metadata.
func headObject(ctx context.Context, client *s3.Client, bucket, key string) (*s3.HeadObjectOutput, error) {
	out, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 HeadObject %s: %w", key, err)
	}

	return out, nil
}
