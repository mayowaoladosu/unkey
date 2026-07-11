package blobstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type S3Config struct {
	Endpoint        string `toml:"endpoint" config:"required,nonempty"`
	Region          string `toml:"region" config:"default=auto"`
	Bucket          string `toml:"bucket" config:"required,nonempty"`
	AccessKeyID     string `toml:"access_key_id" config:"required,nonempty"`
	SecretAccessKey string `toml:"secret_access_key" config:"required,nonempty"`
	UsePathStyle    bool   `toml:"use_path_style"`
	CreateBucket    bool   `toml:"create_bucket"`
}

type s3Store struct {
	client *awss3.Client
	bucket string
}

func NewS3(ctx context.Context, config S3Config) (Store, error) {
	if config.Endpoint == "" || config.Bucket == "" || config.AccessKeyID == "" || config.SecretAccessKey == "" {
		return nil, fmt.Errorf("blob store endpoint, bucket, and credentials are required")
	}
	region := config.Region
	if region == "" {
		region = "auto"
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKeyID,
			config.SecretAccessKey,
			"",
		)),
		awsconfig.WithRetryMode(aws.RetryModeStandard),
		awsconfig.WithRetryMaxAttempts(3),
	)
	if err != nil {
		return nil, fmt.Errorf("load blob store configuration: %w", err)
	}

	client := awss3.NewFromConfig(cfg, func(options *awss3.Options) {
		options.BaseEndpoint = aws.String(config.Endpoint)
		options.UsePathStyle = config.UsePathStyle
	})
	if config.CreateBucket {
		_, err = client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(config.Bucket)})
		if err != nil && !isBucketExists(err) {
			return nil, fmt.Errorf("create blob store bucket: %w", err)
		}
	}

	return &s3Store{client: client, bucket: config.Bucket}, nil
}

func (s *s3Store) Put(ctx context.Context, key string, body []byte, metadata Metadata) error {
	input := &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	}
	if metadata.ContentType != "" {
		input.ContentType = aws.String(metadata.ContentType)
	}
	if metadata.CacheControl != "" {
		input.CacheControl = aws.String(metadata.CacheControl)
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("put blob %q: %w", key, err)
	}
	return nil
}

func (s *s3Store) Get(ctx context.Context, key string, maxBytes int64) (Object, bool, error) {
	response, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return Object{}, false, nil
		}
		return Object{}, false, fmt.Errorf("get blob %q: %w", key, err)
	}
	defer func() { _ = response.Body.Close() }()

	reader := io.Reader(response.Body)
	if maxBytes > 0 {
		reader = io.LimitReader(response.Body, maxBytes+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return Object{}, false, fmt.Errorf("read blob %q: %w", key, err)
	}
	if maxBytes > 0 && int64(len(body)) > maxBytes {
		return Object{}, false, fmt.Errorf("blob %q exceeds %d bytes", key, maxBytes)
	}
	return Object{
		Body: body,
		Metadata: Metadata{
			ContentType:  aws.ToString(response.ContentType),
			CacheControl: aws.ToString(response.CacheControl),
		},
		ETag: strings.Trim(aws.ToString(response.ETag), `"`),
	}, true, nil
}

func isBucketExists(err error) bool {
	var apiError smithy.APIError
	if !errors.As(err, &apiError) {
		return false
	}
	return apiError.ErrorCode() == "BucketAlreadyExists" || apiError.ErrorCode() == "BucketAlreadyOwnedByYou"
}

func isNotFound(err error) bool {
	var apiError smithy.APIError
	if !errors.As(err, &apiError) {
		return false
	}
	return apiError.ErrorCode() == "NoSuchKey" || apiError.ErrorCode() == "NotFound"
}
