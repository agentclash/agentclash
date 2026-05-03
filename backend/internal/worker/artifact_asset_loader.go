package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/google/uuid"
)

const defaultArtifactAssetMaxBytes int64 = 100 << 20

type ArtifactRepository interface {
	GetArtifactByID(ctx context.Context, artifactID uuid.UUID) (repository.Artifact, error)
}

type ArtifactAssetLoader struct {
	repo     ArtifactRepository
	store    storage.Store
	maxBytes int64
}

func NewArtifactAssetLoader(repo ArtifactRepository, store storage.Store) ArtifactAssetLoader {
	return ArtifactAssetLoader{repo: repo, store: store, maxBytes: defaultArtifactAssetMaxBytes}
}

func (l ArtifactAssetLoader) WithMaxBytes(maxBytes int64) ArtifactAssetLoader {
	if maxBytes > 0 {
		l.maxBytes = maxBytes
	}
	return l
}

func (l ArtifactAssetLoader) LoadAsset(ctx context.Context, workspaceID uuid.UUID, artifactID uuid.UUID) (engine.AssetContent, error) {
	if l.repo == nil {
		return engine.AssetContent{}, errors.New("artifact repository is not configured")
	}
	if l.store == nil {
		return engine.AssetContent{}, errors.New("artifact storage is not configured")
	}

	artifact, err := l.repo.GetArtifactByID(ctx, artifactID)
	if err != nil {
		return engine.AssetContent{}, fmt.Errorf("get artifact %s: %w", artifactID, err)
	}
	if artifact.WorkspaceID != workspaceID {
		return engine.AssetContent{}, fmt.Errorf("artifact %s belongs to workspace %s, not %s", artifactID, artifact.WorkspaceID, workspaceID)
	}
	if artifact.StorageBucket != "" && artifact.StorageBucket != l.store.Bucket() {
		return engine.AssetContent{}, fmt.Errorf("artifact %s is stored in bucket %q, worker is configured for %q", artifactID, artifact.StorageBucket, l.store.Bucket())
	}
	maxBytes := l.effectiveMaxBytes()
	if artifact.SizeBytes != nil && *artifact.SizeBytes > maxBytes {
		return engine.AssetContent{}, fmt.Errorf("artifact %s is %d bytes, above sandbox asset limit %d", artifactID, *artifact.SizeBytes, maxBytes)
	}

	reader, metadata, err := l.store.OpenObject(ctx, artifact.StorageKey)
	if err != nil {
		return engine.AssetContent{}, fmt.Errorf("open artifact object %q: %w", artifact.StorageKey, err)
	}
	defer reader.Close()
	if metadata.SizeBytes > maxBytes {
		return engine.AssetContent{}, fmt.Errorf("artifact object %q is %d bytes, above sandbox asset limit %d", artifact.StorageKey, metadata.SizeBytes, maxBytes)
	}

	content, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return engine.AssetContent{}, fmt.Errorf("read artifact object %q: %w", artifact.StorageKey, err)
	}
	if int64(len(content)) > maxBytes {
		return engine.AssetContent{}, fmt.Errorf("artifact object %q exceeded sandbox asset limit %d", artifact.StorageKey, maxBytes)
	}
	if err := verifyArtifactChecksum(artifact, content); err != nil {
		return engine.AssetContent{}, err
	}

	contentType := metadata.ContentType
	if artifact.ContentType != nil && *artifact.ContentType != "" {
		contentType = *artifact.ContentType
	}

	return engine.AssetContent{
		Content:     content,
		ContentType: contentType,
	}, nil
}

func (l ArtifactAssetLoader) effectiveMaxBytes() int64 {
	if l.maxBytes <= 0 {
		return defaultArtifactAssetMaxBytes
	}
	return l.maxBytes
}

func verifyArtifactChecksum(artifact repository.Artifact, content []byte) error {
	if artifact.ChecksumSHA256 == nil || strings.TrimSpace(*artifact.ChecksumSHA256) == "" {
		return nil
	}
	sum := sha256.Sum256(content)
	got := hex.EncodeToString(sum[:])
	want := strings.TrimSpace(*artifact.ChecksumSHA256)
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("artifact %s checksum mismatch: got %s, want %s", artifact.ID, got, want)
	}
	return nil
}
