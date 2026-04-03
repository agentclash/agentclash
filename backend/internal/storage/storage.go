package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	BackendFilesystem = "filesystem"
	BackendS3         = "s3"
)

var ErrObjectNotFound = errors.New("object not found")

type Config struct {
	Backend          string
	Bucket           string
	FilesystemRoot   string
	S3Region         string
	S3Endpoint       string
	S3AccessKeyID    string
	S3SecretKey      string
	S3ForcePathStyle bool
}

type PutObjectInput struct {
	Key         string
	Body        io.Reader
	SizeBytes   int64
	ContentType string
}

type ObjectMetadata struct {
	Bucket      string
	Key         string
	SizeBytes   int64
	ContentType string
}

type Store interface {
	Bucket() string
	PutObject(ctx context.Context, input PutObjectInput) (ObjectMetadata, error)
	OpenObject(ctx context.Context, key string) (io.ReadCloser, ObjectMetadata, error)
}

func NewStore(ctx context.Context, cfg Config) (Store, error) {
	switch cfg.Backend {
	case BackendFilesystem:
		return NewFilesystemStore(cfg)
	case BackendS3:
		return NewS3Store(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported artifact storage backend %q", cfg.Backend)
	}
}

func ExpiresAt(now time.Time, ttl time.Duration) time.Time {
	return now.UTC().Add(ttl)
}
