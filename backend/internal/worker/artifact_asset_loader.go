package worker

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/google/uuid"
)

type ArtifactRepository interface {
	GetArtifactByID(ctx context.Context, artifactID uuid.UUID) (repository.Artifact, error)
}

type ArtifactAssetLoader struct {
	repo  ArtifactRepository
	store storage.Store
}

func NewArtifactAssetLoader(repo ArtifactRepository, store storage.Store) ArtifactAssetLoader {
	return ArtifactAssetLoader{repo: repo, store: store}
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

	reader, metadata, err := l.store.OpenObject(ctx, artifact.StorageKey)
	if err != nil {
		return engine.AssetContent{}, fmt.Errorf("open artifact object %q: %w", artifact.StorageKey, err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return engine.AssetContent{}, fmt.Errorf("read artifact object %q: %w", artifact.StorageKey, err)
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
