package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Store struct {
	bucket   string
	client   *s3.Client
	uploader *manager.Uploader
}

func NewS3Store(ctx context.Context, cfg Config) (*S3Store, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("bucket is required")
	}
	if strings.TrimSpace(cfg.S3Region) == "" {
		return nil, errors.New("s3 region is required")
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.S3Region),
	}
	if cfg.S3AccessKeyID != "" || cfg.S3SecretKey != "" {
		if cfg.S3AccessKeyID == "" || cfg.S3SecretKey == "" {
			return nil, errors.New("both s3 access key id and secret key are required")
		}
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretKey, "")))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.S3ForcePathStyle
		if cfg.S3Endpoint != "" {
			options.BaseEndpoint = &cfg.S3Endpoint
		}
	})

	return &S3Store{
		bucket:   cfg.Bucket,
		client:   client,
		uploader: manager.NewUploader(client),
	}, nil
}

func (s *S3Store) Bucket() string {
	return s.bucket
}

func (s *S3Store) PutObject(ctx context.Context, input PutObjectInput) (ObjectMetadata, error) {
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &input.Key,
		Body:        input.Body,
		ContentType: &input.ContentType,
	})
	if err != nil {
		return ObjectMetadata{}, fmt.Errorf("upload s3 object: %w", err)
	}

	return ObjectMetadata{
		Bucket:      s.bucket,
		Key:         input.Key,
		SizeBytes:   input.SizeBytes,
		ContentType: input.ContentType,
	}, nil
}

func (s *S3Store) OpenObject(ctx context.Context, key string) (io.ReadCloser, ObjectMetadata, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, ObjectMetadata{}, ErrObjectNotFound
		}
		return nil, ObjectMetadata{}, fmt.Errorf("get s3 object: %w", err)
	}

	size := int64(0)
	if result.ContentLength != nil {
		size = *result.ContentLength
	}
	contentType := ""
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	return result.Body, ObjectMetadata{
		Bucket:      s.bucket,
		Key:         key,
		SizeBytes:   size,
		ContentType: contentType,
	}, nil
}
