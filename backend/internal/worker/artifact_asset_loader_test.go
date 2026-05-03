package worker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/google/uuid"
)

func TestArtifactAssetLoaderReadsWorkspaceArtifactFromStorage(t *testing.T) {
	workspaceID := uuid.New()
	artifactID := uuid.New()
	contentType := "text/csv"
	repo := fakeArtifactRepository{
		artifacts: map[uuid.UUID]repository.Artifact{
			artifactID: {
				ID:            artifactID,
				WorkspaceID:   workspaceID,
				StorageBucket: "asset-bucket",
				StorageKey:    "workspaces/ws/assets/data.csv",
				ContentType:   &contentType,
			},
		},
	}
	store := fakeArtifactStorage{
		bucket: "asset-bucket",
		objects: map[string]fakeStoredObject{
			"workspaces/ws/assets/data.csv": {
				content:     []byte("name,value\nalpha,1\n"),
				contentType: "application/octet-stream",
			},
		},
	}

	got, err := NewArtifactAssetLoader(repo, store).LoadAsset(context.Background(), workspaceID, artifactID)
	if err != nil {
		t.Fatalf("LoadAsset returned error: %v", err)
	}
	if string(got.Content) != "name,value\nalpha,1\n" {
		t.Fatalf("content = %q, want storage bytes", string(got.Content))
	}
	if got.ContentType != contentType {
		t.Fatalf("content type = %q, want %q", got.ContentType, contentType)
	}
}

func TestArtifactAssetLoaderRejectsCrossWorkspaceArtifact(t *testing.T) {
	requestWorkspaceID := uuid.New()
	artifactWorkspaceID := uuid.New()
	artifactID := uuid.New()
	repo := fakeArtifactRepository{
		artifacts: map[uuid.UUID]repository.Artifact{
			artifactID: {
				ID:            artifactID,
				WorkspaceID:   artifactWorkspaceID,
				StorageBucket: "asset-bucket",
				StorageKey:    "workspaces/other/assets/data.csv",
			},
		},
	}
	store := fakeArtifactStorage{bucket: "asset-bucket"}

	if _, err := NewArtifactAssetLoader(repo, store).LoadAsset(context.Background(), requestWorkspaceID, artifactID); err == nil {
		t.Fatal("LoadAsset returned nil error")
	}
}

type fakeArtifactRepository struct {
	artifacts map[uuid.UUID]repository.Artifact
}

func (r fakeArtifactRepository) GetArtifactByID(_ context.Context, artifactID uuid.UUID) (repository.Artifact, error) {
	artifact, ok := r.artifacts[artifactID]
	if !ok {
		return repository.Artifact{}, repository.ErrArtifactNotFound
	}
	return artifact, nil
}

type fakeStoredObject struct {
	content     []byte
	contentType string
}

type fakeArtifactStorage struct {
	bucket  string
	objects map[string]fakeStoredObject
}

func (s fakeArtifactStorage) Bucket() string {
	return s.bucket
}

func (s fakeArtifactStorage) PutObject(context.Context, storage.PutObjectInput) (storage.ObjectMetadata, error) {
	return storage.ObjectMetadata{}, errors.New("not implemented")
}

func (s fakeArtifactStorage) OpenObject(_ context.Context, key string) (io.ReadCloser, storage.ObjectMetadata, error) {
	object, ok := s.objects[key]
	if !ok {
		return nil, storage.ObjectMetadata{}, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(object.content)), storage.ObjectMetadata{
		Bucket:      s.bucket,
		Key:         key,
		SizeBytes:   int64(len(object.content)),
		ContentType: object.contentType,
	}, nil
}

func (s fakeArtifactStorage) DeleteObject(context.Context, string) error {
	return errors.New("not implemented")
}
