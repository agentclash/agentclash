package localstore

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestFileArtifactStoreRoundTrip(t *testing.T) {
	store, err := NewFileArtifactStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileArtifactStore: %v", err)
	}
	workspaceID := uuid.New()

	if err := store.Write(context.Background(), workspaceID, "captures/out.txt", []byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := store.Read(context.Background(), workspaceID, "captures/out.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("Read = %q; want hello", got)
	}
}

func TestFileArtifactStoreRejectsTraversal(t *testing.T) {
	store, err := NewFileArtifactStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileArtifactStore: %v", err)
	}
	if err := store.Write(context.Background(), uuid.New(), "../escape.txt", []byte("nope")); err == nil {
		t.Fatal("Write with traversal key succeeded; want error")
	}
}

func TestFileArtifactStoreMissingRead(t *testing.T) {
	store, err := NewFileArtifactStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileArtifactStore: %v", err)
	}
	_, err = store.Read(context.Background(), uuid.New(), "missing.txt")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read err = %v; want ErrNotFound", err)
	}
}
