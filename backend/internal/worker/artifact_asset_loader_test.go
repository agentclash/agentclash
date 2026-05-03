package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/google/uuid"
)

func TestArtifactAssetLoaderReadsWorkspaceArtifactFromStorage(t *testing.T) {
	workspaceID := uuid.New()
	artifactID := uuid.New()
	contentType := "text/csv"
	content := []byte("name,value\nalpha,1\n")
	checksum := sha256Hex(content)
	sizeBytes := int64(len(content))
	repo := fakeArtifactRepository{
		artifacts: map[uuid.UUID]repository.Artifact{
			artifactID: {
				ID:             artifactID,
				WorkspaceID:    workspaceID,
				StorageBucket:  "asset-bucket",
				StorageKey:     "workspaces/ws/assets/data.csv",
				ContentType:    &contentType,
				SizeBytes:      &sizeBytes,
				ChecksumSHA256: &checksum,
			},
		},
	}
	store := fakeArtifactStorage{
		bucket: "asset-bucket",
		objects: map[string]fakeStoredObject{
			"workspaces/ws/assets/data.csv": {
				content:     content,
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
	openCalls := 0
	store := fakeArtifactStorage{bucket: "asset-bucket", openCalls: &openCalls}

	if _, err := NewArtifactAssetLoader(repo, store).LoadAsset(context.Background(), requestWorkspaceID, artifactID); err == nil {
		t.Fatal("LoadAsset returned nil error")
	}
}

func TestArtifactAssetLoaderRejectsArtifactAboveLimitBeforeOpeningStorage(t *testing.T) {
	workspaceID := uuid.New()
	artifactID := uuid.New()
	sizeBytes := int64(11)
	repo := fakeArtifactRepository{
		artifacts: map[uuid.UUID]repository.Artifact{
			artifactID: {
				ID:            artifactID,
				WorkspaceID:   workspaceID,
				StorageBucket: "asset-bucket",
				StorageKey:    "workspaces/ws/assets/large.bin",
				SizeBytes:     &sizeBytes,
			},
		},
	}
	openCalls := 0
	store := fakeArtifactStorage{bucket: "asset-bucket", openCalls: &openCalls}

	_, err := NewArtifactAssetLoader(repo, store).WithMaxBytes(10).LoadAsset(context.Background(), workspaceID, artifactID)
	if err == nil {
		t.Fatal("LoadAsset returned nil error")
	}
	if !strings.Contains(err.Error(), "above sandbox asset limit") {
		t.Fatalf("error = %v, want size limit", err)
	}
	if openCalls != 0 {
		t.Fatalf("OpenObject calls = %d, want 0", openCalls)
	}
}

func TestArtifactAssetLoaderRejectsObjectStreamAboveLimit(t *testing.T) {
	workspaceID := uuid.New()
	artifactID := uuid.New()
	sizeBytes := int64(10)
	repo := fakeArtifactRepository{
		artifacts: map[uuid.UUID]repository.Artifact{
			artifactID: {
				ID:            artifactID,
				WorkspaceID:   workspaceID,
				StorageBucket: "asset-bucket",
				StorageKey:    "workspaces/ws/assets/large.bin",
				SizeBytes:     &sizeBytes,
			},
		},
	}
	store := fakeArtifactStorage{
		bucket: "asset-bucket",
		objects: map[string]fakeStoredObject{
			"workspaces/ws/assets/large.bin": {
				content:   []byte("01234567890"),
				sizeBytes: 10,
			},
		},
	}

	_, err := NewArtifactAssetLoader(repo, store).WithMaxBytes(10).LoadAsset(context.Background(), workspaceID, artifactID)
	if err == nil {
		t.Fatal("LoadAsset returned nil error")
	}
	if !strings.Contains(err.Error(), "exceeded sandbox asset limit") {
		t.Fatalf("error = %v, want stream size limit", err)
	}
}

func TestArtifactAssetLoaderRejectsChecksumMismatch(t *testing.T) {
	workspaceID := uuid.New()
	artifactID := uuid.New()
	badChecksum := sha256Hex([]byte("different content"))
	repo := fakeArtifactRepository{
		artifacts: map[uuid.UUID]repository.Artifact{
			artifactID: {
				ID:             artifactID,
				WorkspaceID:    workspaceID,
				StorageBucket:  "asset-bucket",
				StorageKey:     "workspaces/ws/assets/data.csv",
				ChecksumSHA256: &badChecksum,
			},
		},
	}
	store := fakeArtifactStorage{
		bucket: "asset-bucket",
		objects: map[string]fakeStoredObject{
			"workspaces/ws/assets/data.csv": {
				content: []byte("name,value\nalpha,1\n"),
			},
		},
	}

	_, err := NewArtifactAssetLoader(repo, store).LoadAsset(context.Background(), workspaceID, artifactID)
	if err == nil {
		t.Fatal("LoadAsset returned nil error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v, want checksum mismatch", err)
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
	sizeBytes   int64
}

type fakeArtifactStorage struct {
	bucket    string
	objects   map[string]fakeStoredObject
	openCalls *int
}

func (s fakeArtifactStorage) Bucket() string {
	return s.bucket
}

func (s fakeArtifactStorage) PutObject(context.Context, storage.PutObjectInput) (storage.ObjectMetadata, error) {
	return storage.ObjectMetadata{}, errors.New("not implemented")
}

func (s fakeArtifactStorage) OpenObject(_ context.Context, key string) (io.ReadCloser, storage.ObjectMetadata, error) {
	if s.openCalls != nil {
		(*s.openCalls)++
	}
	object, ok := s.objects[key]
	if !ok {
		return nil, storage.ObjectMetadata{}, storage.ErrObjectNotFound
	}
	sizeBytes := object.sizeBytes
	if sizeBytes == 0 {
		sizeBytes = int64(len(object.content))
	}
	return io.NopCloser(bytes.NewReader(object.content)), storage.ObjectMetadata{
		Bucket:      s.bucket,
		Key:         key,
		SizeBytes:   sizeBytes,
		ContentType: object.contentType,
	}, nil
}

func (s fakeArtifactStorage) DeleteObject(context.Context, string) error {
	return errors.New("not implemented")
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
