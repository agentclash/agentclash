package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/google/uuid"
)

const (
	maxAgentTryoutInputAttachments              = 5
	defaultAgentTryoutInputAttachmentMaxBytes = 15 << 20
	agentTryoutInputAttachmentPrefix            = "agent-tryout-input-attachments"
)

var (
	ErrAgentTryoutInputAttachmentNotFound   = errors.New("agent tryout input attachment not found")
	ErrAgentTryoutInputAttachmentTooLarge   = errors.New("agent tryout input attachment exceeds maximum size")
	ErrAgentTryoutInputAttachmentInvalid    = errors.New("agent tryout input attachment is invalid")
	ErrAgentTryoutInputAttachmentUnavailable = errors.New("agent tryout input attachments are unavailable")

	allowedTryoutInputAttachmentMediaTypes = map[string]struct{}{
		"application/pdf": {},
		"image/jpeg":      {},
		"image/png":       {},
		"image/gif":       {},
		"image/webp":      {},
	}
)

type AgentTryoutInputAttachment struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	MediaType string `json:"media_type"`
	SizeBytes int64  `json:"size_bytes"`
}

type UploadAgentTryoutInputAttachmentInput struct {
	AnonymousFingerprint string
	Filename             string
	DeclaredType         string
	Body                 io.Reader
}

type agentTryoutInputAttachmentMeta struct {
	ID              string    `json:"id"`
	FingerprintHash string    `json:"fingerprint_hash"`
	Filename        string    `json:"filename"`
	MediaType       string    `json:"media_type"`
	SizeBytes       int64     `json:"size_bytes"`
	ContentKey      string    `json:"content_key"`
	CreatedAt       time.Time `json:"created_at"`
}

func (m *AgentTryoutManager) WithInputAttachmentStore(store storage.Store, maxBytes int64) *AgentTryoutManager {
	m.inputAttachmentStore = store
	if maxBytes > 0 {
		m.inputAttachmentMaxBytes = maxBytes
	} else {
		m.inputAttachmentMaxBytes = defaultAgentTryoutInputAttachmentMaxBytes
	}
	return m
}

func (m *AgentTryoutManager) UploadAnonymousTryoutInputAttachment(
	ctx context.Context,
	input UploadAgentTryoutInputAttachmentInput,
) (AgentTryoutInputAttachment, error) {
	if m.inputAttachmentStore == nil {
		return AgentTryoutInputAttachment{}, ErrAgentTryoutInputAttachmentUnavailable
	}
	if input.Body == nil {
		return AgentTryoutInputAttachment{}, fmt.Errorf("%w: file is required", ErrAgentTryoutInputAttachmentInvalid)
	}
	filename := sanitizeTryoutAttachmentFilename(input.Filename)
	if filename == "" {
		return AgentTryoutInputAttachment{}, fmt.Errorf("%w: filename is required", ErrAgentTryoutInputAttachmentInvalid)
	}

	limitedReader := io.LimitReader(input.Body, m.inputAttachmentMaxBytes+1)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return AgentTryoutInputAttachment{}, fmt.Errorf("read attachment upload: %w", err)
	}
	if int64(len(content)) > m.inputAttachmentMaxBytes {
		return AgentTryoutInputAttachment{}, ErrAgentTryoutInputAttachmentTooLarge
	}
	if len(content) == 0 {
		return AgentTryoutInputAttachment{}, fmt.Errorf("%w: file is empty", ErrAgentTryoutInputAttachmentInvalid)
	}

	head := content
	if len(head) > 512 {
		head = head[:512]
	}
	mediaType, err := normalizeTryoutInputAttachmentContentType(input.DeclaredType, head)
	if err != nil {
		return AgentTryoutInputAttachment{}, err
	}

	id := uuid.New()
	fingerprintHash := hashAnonymousFingerprint(input.AnonymousFingerprint)
	contentKey := path.Join(agentTryoutInputAttachmentPrefix, id.String(), "content")
	metaKey := path.Join(agentTryoutInputAttachmentPrefix, id.String(), "meta.json")
	meta := agentTryoutInputAttachmentMeta{
		ID:              id.String(),
		FingerprintHash: fingerprintHash,
		Filename:        filename,
		MediaType:       mediaType,
		SizeBytes:       int64(len(content)),
		ContentKey:      contentKey,
		CreatedAt:       m.now().UTC(),
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return AgentTryoutInputAttachment{}, fmt.Errorf("marshal attachment metadata: %w", err)
	}

	if _, err := m.inputAttachmentStore.PutObject(ctx, storage.PutObjectInput{
		Key:         contentKey,
		Body:        bytes.NewReader(content),
		SizeBytes:   int64(len(content)),
		ContentType: mediaType,
	}); err != nil {
		return AgentTryoutInputAttachment{}, fmt.Errorf("store attachment content: %w", err)
	}
	if _, err := m.inputAttachmentStore.PutObject(ctx, storage.PutObjectInput{
		Key:         metaKey,
		Body:        bytes.NewReader(metaJSON),
		SizeBytes:   int64(len(metaJSON)),
		ContentType: "application/json",
	}); err != nil {
		_ = m.inputAttachmentStore.DeleteObject(ctx, contentKey)
		return AgentTryoutInputAttachment{}, fmt.Errorf("store attachment metadata: %w", err)
	}

	return AgentTryoutInputAttachment{
		ID:        meta.ID,
		Filename:  meta.Filename,
		MediaType: meta.MediaType,
		SizeBytes: meta.SizeBytes,
	}, nil
}

func (m *AgentTryoutManager) resolveTryoutInputForCreate(
	ctx context.Context,
	fingerprintHash string,
	raw json.RawMessage,
) (json.RawMessage, error) {
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("%w: input must be a JSON object", ErrInvalidAgentTryoutInput)
	}
	if object == nil {
		return nil, fmt.Errorf("%w: input must be a JSON object", ErrInvalidAgentTryoutInput)
	}
	if err := m.resolveTryoutInputAttachments(ctx, fingerprintHash, object); err != nil {
		return nil, err
	}
	resolved, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("marshal resolved tryout input: %w", err)
	}
	return resolved, nil
}

func (m *AgentTryoutManager) resolveTryoutInputAttachments(
	ctx context.Context,
	fingerprintHash string,
	object map[string]any,
) error {
	value, ok := object["input_attachments"]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%w: field %q must be array", ErrInvalidAgentTryoutInput, "input_attachments")
	}
	if len(items) > maxAgentTryoutInputAttachments {
		return fmt.Errorf("%w: field %q supports up to %d files", ErrInvalidAgentTryoutInput, "input_attachments", maxAgentTryoutInputAttachments)
	}
	if len(items) > 0 && m.inputAttachmentStore == nil {
		return ErrAgentTryoutInputAttachmentUnavailable
	}

	usedPaths := make(map[string]struct{}, len(items))
	resolved := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: field %q entries must be objects", ErrInvalidAgentTryoutInput, "input_attachments")
		}
		rawID, ok := obj["id"].(string)
		if !ok || strings.TrimSpace(rawID) == "" {
			return fmt.Errorf("%w: field %q entries require id", ErrInvalidAgentTryoutInput, "input_attachments")
		}
		id, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			return fmt.Errorf("%w: field %q entry id must be a UUID", ErrInvalidAgentTryoutInput, "input_attachments")
		}
		meta, err := m.loadTryoutInputAttachmentMeta(ctx, id)
		if err != nil {
			return err
		}
		if meta.FingerprintHash != fingerprintHash {
			return fmt.Errorf("%w: attachment %q is not owned by this session", ErrInvalidAgentTryoutInput, id.String())
		}
		workspacePath := uniqueTryoutAttachmentWorkspacePath(meta.Filename, usedPaths)
		resolved = append(resolved, map[string]any{
			"id":             meta.ID,
			"filename":       meta.Filename,
			"media_type":     meta.MediaType,
			"size_bytes":     meta.SizeBytes,
			"storage_key":    meta.ContentKey,
			"workspace_path": workspacePath,
		})
	}
	object["input_attachments"] = resolved
	return nil
}

func (m *AgentTryoutManager) loadTryoutInputAttachmentMeta(ctx context.Context, id uuid.UUID) (agentTryoutInputAttachmentMeta, error) {
	metaKey := path.Join(agentTryoutInputAttachmentPrefix, id.String(), "meta.json")
	reader, _, err := m.inputAttachmentStore.OpenObject(ctx, metaKey)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return agentTryoutInputAttachmentMeta{}, fmt.Errorf("%w: %s", ErrAgentTryoutInputAttachmentNotFound, id.String())
		}
		return agentTryoutInputAttachmentMeta{}, fmt.Errorf("open attachment metadata: %w", err)
	}
	defer reader.Close()

	var meta agentTryoutInputAttachmentMeta
	if err := json.NewDecoder(reader).Decode(&meta); err != nil {
		return agentTryoutInputAttachmentMeta{}, fmt.Errorf("%w: attachment metadata is invalid", ErrAgentTryoutInputAttachmentInvalid)
	}
	if strings.TrimSpace(meta.ContentKey) == "" || strings.TrimSpace(meta.FingerprintHash) == "" {
		return agentTryoutInputAttachmentMeta{}, fmt.Errorf("%w: attachment metadata is incomplete", ErrAgentTryoutInputAttachmentInvalid)
	}
	return meta, nil
}

func validateTryoutInputAttachmentsField(value any) error {
	if value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%w: field %q must be array", ErrInvalidAgentTryoutInput, "input_attachments")
	}
	if len(items) > maxAgentTryoutInputAttachments {
		return fmt.Errorf("%w: field %q supports up to %d files", ErrInvalidAgentTryoutInput, "input_attachments", maxAgentTryoutInputAttachments)
	}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: field %q entries must be objects", ErrInvalidAgentTryoutInput, "input_attachments")
		}
		rawID, ok := obj["id"].(string)
		if !ok || strings.TrimSpace(rawID) == "" {
			return fmt.Errorf("%w: field %q entries require id", ErrInvalidAgentTryoutInput, "input_attachments")
		}
		if _, err := uuid.Parse(strings.TrimSpace(rawID)); err != nil {
			return fmt.Errorf("%w: field %q entry id must be a UUID", ErrInvalidAgentTryoutInput, "input_attachments")
		}
	}
	return nil
}

func normalizeTryoutInputAttachmentContentType(declared string, sniffed []byte) (string, error) {
	contentType := strings.TrimSpace(declared)
	if contentType != "" {
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return "", fmt.Errorf("%w: invalid content type", ErrAgentTryoutInputAttachmentInvalid)
		}
		if err := validateTryoutInputAttachmentContentType(mediaType); err != nil {
			return "", err
		}
		if len(params) == 0 {
			return mediaType, nil
		}
		return mime.FormatMediaType(mediaType, params), nil
	}

	detected := http.DetectContentType(sniffed)
	if detected == "" {
		return "", fmt.Errorf("%w: could not detect content type", ErrAgentTryoutInputAttachmentInvalid)
	}
	mediaType, _, err := mime.ParseMediaType(detected)
	if err != nil {
		return "", fmt.Errorf("%w: invalid detected content type", ErrAgentTryoutInputAttachmentInvalid)
	}
	if err := validateTryoutInputAttachmentContentType(mediaType); err != nil {
		return "", err
	}
	return detected, nil
}

func validateTryoutInputAttachmentContentType(mediaType string) error {
	if _, ok := allowedTryoutInputAttachmentMediaTypes[strings.ToLower(strings.TrimSpace(mediaType))]; !ok {
		return fmt.Errorf("%w: %s is not allowed (use PDF or common image formats)", ErrAgentTryoutInputAttachmentInvalid, mediaType)
	}
	return nil
}

func sanitizeTryoutAttachmentFilename(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, `"`, "")
	base = strings.ReplaceAll(base, "\x00", "")
	if base == "." || base == "" || base == string(filepath.Separator) {
		return ""
	}
	return base
}

func uniqueTryoutAttachmentWorkspacePath(filename string, used map[string]struct{}) string {
	safe := sanitizeTryoutAttachmentFilename(filename)
	if safe == "" {
		safe = "attachment"
	}
	basePath := path.Join("input", safe)
	if _, exists := used[basePath]; !exists {
		used[basePath] = struct{}{}
		return basePath
	}
	ext := path.Ext(safe)
	stem := strings.TrimSuffix(safe, ext)
	for i := 2; i < 100; i++ {
		candidate := path.Join("input", fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
	}
	fallback := path.Join("input", uuid.NewString()+ext)
	used[fallback] = struct{}{}
	return fallback
}

func uploadAgentTryoutInputAttachmentHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, defaultAgentTryoutInputAttachmentMaxBytes+1024)
		if err := r.ParseMultipartForm(defaultMultipartMaxMemory); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid multipart upload")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file_required", "file is required")
			return
		}
		defer file.Close()

		attachment, err := service.UploadAnonymousTryoutInputAttachment(r.Context(), UploadAgentTryoutInputAttachmentInput{
			AnonymousFingerprint: anonymousFingerprintFromRequest(r),
			Filename:             header.Filename,
			DeclaredType:         header.Header.Get("Content-Type"),
			Body:                 file,
		})
		if err != nil {
			writeAgentTryoutAttachmentError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, attachment)
	}
}

func writeAgentTryoutAttachmentError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, ErrAgentTryoutInputAttachmentTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "attachment_too_large", "Attachment exceeds the maximum upload size.")
	case errors.Is(err, ErrAgentTryoutInputAttachmentInvalid):
		writeError(w, http.StatusBadRequest, "invalid_attachment", err.Error())
	case errors.Is(err, ErrAgentTryoutInputAttachmentNotFound):
		writeError(w, http.StatusNotFound, "attachment_not_found", "Attachment not found.")
	case errors.Is(err, ErrAgentTryoutInputAttachmentUnavailable):
		writeError(w, http.StatusServiceUnavailable, "attachments_unavailable", "File attachments are temporarily unavailable.")
	default:
		logger.Error("agent tryout attachment request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
