package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/google/uuid"
)

type memoryTryoutAttachmentStore struct {
	objects map[string][]byte
}

func newMemoryTryoutAttachmentStore() *memoryTryoutAttachmentStore {
	return &memoryTryoutAttachmentStore{objects: map[string][]byte{}}
}

func (s *memoryTryoutAttachmentStore) Bucket() string { return "test-bucket" }

func (s *memoryTryoutAttachmentStore) PutObject(_ context.Context, input storage.PutObjectInput) (storage.ObjectMetadata, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return storage.ObjectMetadata{}, err
	}
	s.objects[input.Key] = body
	return storage.ObjectMetadata{Key: input.Key, SizeBytes: int64(len(body))}, nil
}

func (s *memoryTryoutAttachmentStore) OpenObject(_ context.Context, key string) (io.ReadCloser, storage.ObjectMetadata, error) {
	body, ok := s.objects[key]
	if !ok {
		return nil, storage.ObjectMetadata{}, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(body)), storage.ObjectMetadata{Key: key, SizeBytes: int64(len(body))}, nil
}

func (s *memoryTryoutAttachmentStore) DeleteObject(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

func TestAgentTryoutManagerResolvesInputAttachments(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	store := newMemoryTryoutAttachmentStore()
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).
		WithInputAttachmentStore(store, defaultAgentTryoutInputAttachmentMaxBytes)

	fingerprint := "203.0.113.10"
	attachment, err := manager.UploadAnonymousTryoutInputAttachment(context.Background(), UploadAgentTryoutInputAttachmentInput{
		AnonymousFingerprint: fingerprint,
		Filename:             "brief.pdf",
		DeclaredType:         "application/pdf",
		Body:                 bytes.NewReader([]byte("%PDF-1.4 test")),
	})
	if err != nil {
		t.Fatalf("UploadAnonymousTryoutInputAttachment returned error: %v", err)
	}

	tryout, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
		TemplateSlug: "meeting-minutes",
		Input: json.RawMessage(`{
			"notes":"Summarize the attached brief",
			"input_attachments":[{"id":"` + attachment.ID + `"}]
		}`),
		AnonymousFingerprint: fingerprint,
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}

	var snapshot map[string]any
	if err := json.Unmarshal(tryout.InputSnapshot, &snapshot); err != nil {
		t.Fatalf("unmarshal input snapshot: %v", err)
	}
	items, ok := snapshot["input_attachments"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("input_attachments = %v, want one resolved attachment", snapshot["input_attachments"])
	}
	resolved, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("resolved attachment type = %T, want map", items[0])
	}
	if resolved["workspace_path"] != "input/brief.pdf" {
		t.Fatalf("workspace_path = %v, want input/brief.pdf", resolved["workspace_path"])
	}
	if strings.TrimSpace(stringFromSnapshot(resolved["storage_key"])) == "" {
		t.Fatalf("storage_key missing in resolved attachment: %v", resolved)
	}
}

func TestAgentTryoutManagerRejectsForeignInputAttachment(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	store := newMemoryTryoutAttachmentStore()
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).
		WithInputAttachmentStore(store, defaultAgentTryoutInputAttachmentMaxBytes)

	attachment, err := manager.UploadAnonymousTryoutInputAttachment(context.Background(), UploadAgentTryoutInputAttachmentInput{
		AnonymousFingerprint: "203.0.113.10",
		Filename:             "brief.pdf",
		DeclaredType:         "application/pdf",
		Body:                 bytes.NewReader([]byte("%PDF-1.4 test")),
	})
	if err != nil {
		t.Fatalf("UploadAnonymousTryoutInputAttachment returned error: %v", err)
	}

	_, err = manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
		TemplateSlug: "meeting-minutes",
		Input: json.RawMessage(`{
			"notes":"Summarize the attached brief",
			"input_attachments":[{"id":"` + attachment.ID + `"}]
		}`),
		AnonymousFingerprint: "198.51.100.2",
	})
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrInvalidAgentTryoutInput", err)
	}
}

func stringFromSnapshot(value any) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(raw)
}
