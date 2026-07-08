package localstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type FileArtifactStore struct {
	root string
}

func NewFileArtifactStore(root string) (FileArtifactStore, error) {
	if strings.TrimSpace(root) == "" {
		return FileArtifactStore{}, errors.New("artifact store root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return FileArtifactStore{}, fmt.Errorf("resolve artifact store root: %w", err)
	}
	return FileArtifactStore{root: abs}, nil
}

func (s FileArtifactStore) Write(ctx context.Context, workspaceID uuid.UUID, key string, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(workspaceID, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}
	return nil
}

func (s FileArtifactStore) Read(ctx context.Context, workspaceID uuid.UUID, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.path(workspaceID, key)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read artifact: %w", err)
	}
	return content, nil
}

func (s FileArtifactStore) path(workspaceID uuid.UUID, key string) (string, error) {
	if workspaceID == uuid.Nil {
		return "", errors.New("workspace id is required")
	}
	rel, err := cleanRelativeKey(key)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, workspaceID.String(), rel), nil
}

func cleanRelativeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("artifact key is required")
	}
	if filepath.IsAbs(key) {
		return "", errors.New("artifact key must be relative")
	}
	cleaned := filepath.Clean(key)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", errors.New("artifact key must stay within the workspace")
	}
	return cleaned, nil
}
