package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FilesystemStore struct {
	root   string
	bucket string
}

func NewFilesystemStore(cfg Config) (*FilesystemStore, error) {
	if strings.TrimSpace(cfg.FilesystemRoot) == "" {
		return nil, errors.New("filesystem root is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("bucket is required")
	}

	return &FilesystemStore{
		root:   filepath.Clean(cfg.FilesystemRoot),
		bucket: cfg.Bucket,
	}, nil
}

func (s *FilesystemStore) Bucket() string {
	return s.bucket
}

func (s *FilesystemStore) PutObject(_ context.Context, input PutObjectInput) (ObjectMetadata, error) {
	targetPath, err := s.objectPath(input.Key)
	if err != nil {
		return ObjectMetadata{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return ObjectMetadata{}, fmt.Errorf("create object directory: %w", err)
	}

	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return ObjectMetadata{}, fmt.Errorf("create object file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, input.Body)
	if err != nil {
		return ObjectMetadata{}, fmt.Errorf("write object file: %w", err)
	}

	return ObjectMetadata{
		Bucket:      s.bucket,
		Key:         input.Key,
		SizeBytes:   written,
		ContentType: input.ContentType,
	}, nil
}

func (s *FilesystemStore) OpenObject(_ context.Context, key string) (io.ReadCloser, ObjectMetadata, error) {
	targetPath, err := s.objectPath(key)
	if err != nil {
		return nil, ObjectMetadata{}, err
	}

	file, err := os.Open(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ObjectMetadata{}, ErrObjectNotFound
		}
		return nil, ObjectMetadata{}, fmt.Errorf("open object file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, ObjectMetadata{}, fmt.Errorf("stat object file: %w", err)
	}

	return file, ObjectMetadata{
		Bucket:    s.bucket,
		Key:       key,
		SizeBytes: info.Size(),
	}, nil
}

func (s *FilesystemStore) objectPath(key string) (string, error) {
	cleanKey := strings.TrimPrefix(filepath.Clean("/"+key), "/")
	if cleanKey == "." || cleanKey == "" {
		return "", errors.New("object key is required")
	}

	root := filepath.Join(s.root, s.bucket)
	target := filepath.Join(root, cleanKey)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", fmt.Errorf("resolve object path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("object key escapes storage root")
	}

	return target, nil
}
