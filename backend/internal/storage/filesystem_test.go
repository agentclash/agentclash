package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesystemStoreKeepsSanitizedKeysUnderRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := NewFilesystemStore(Config{
		Bucket:         "test-bucket",
		FilesystemRoot: root,
	})
	if err != nil {
		t.Fatalf("NewFilesystemStore returned error: %v", err)
	}

	_, err = store.PutObject(context.Background(), PutObjectInput{
		Key:         "../../etc/passwd",
		Body:        strings.NewReader("nope"),
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("PutObject returned error: %v", err)
	}

	target := filepath.Join(root, "test-bucket", "etc", "passwd")
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "nope" {
		t.Fatalf("content = %q, want nope", content)
	}

	reader, _, err := store.OpenObject(context.Background(), "../../etc/passwd")
	if err != nil {
		t.Fatalf("OpenObject returned error: %v", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(data) != "nope" {
		t.Fatalf("opened content = %q, want nope", data)
	}
}
