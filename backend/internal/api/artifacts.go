package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	defaultArtifactVisibility      = "private"
	defaultArtifactRetentionStatus = "active"
	defaultMultipartMaxMemory      = 10 << 20
	artifactExpiryGracePeriod      = 30 * time.Second
)

var (
	errArtifactTokenInvalid       = errors.New("artifact token is invalid")
	errArtifactTokenExpired       = errors.New("artifact token is expired")
	errArtifactTypeInvalid        = errors.New("artifact type is invalid")
	errArtifactFileRequired       = errors.New("artifact file is required")
	errArtifactTooLarge           = errors.New("artifact exceeds maximum upload size")
	errArtifactMetadataInvalid    = errors.New("artifact metadata is invalid")
	errArtifactAssociationInvalid = errors.New("artifact association is invalid")
	errArtifactContentTypeInvalid = errors.New("artifact content type is invalid")
	artifactTypePattern           = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

var disallowedArtifactMediaTypes = map[string]struct{}{
	"application/javascript": {},
	"application/xhtml+xml":  {},
	"image/svg+xml":          {},
	"text/html":              {},
	"text/javascript":        {},
}

type ArtifactRepository interface {
	CreateArtifact(ctx context.Context, params repository.CreateArtifactParams) (repository.Artifact, error)
	GetArtifactByID(ctx context.Context, artifactID uuid.UUID) (repository.Artifact, error)
	GetRunByID(ctx context.Context, runID uuid.UUID) (domain.Run, error)
	GetRunAgentByID(ctx context.Context, runAgentID uuid.UUID) (domain.RunAgent, error)
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
}

type ArtifactService interface {
	UploadArtifact(ctx context.Context, caller Caller, input UploadArtifactInput) (UploadArtifactResult, error)
	GetArtifactDownload(ctx context.Context, caller Caller, artifactID uuid.UUID, baseURL string) (GetArtifactDownloadResult, error)
	GetArtifactContent(ctx context.Context, artifactID uuid.UUID, expiresAt time.Time, signature string) (GetArtifactContentResult, error)
}

type UploadArtifactInput struct {
	WorkspaceID    uuid.UUID
	ArtifactType   string
	RunID          *uuid.UUID
	RunAgentID     *uuid.UUID
	Filename       string
	DeclaredType   string
	Body           multipart.File
	MetadataJSON   string
	MaxUploadBytes int64
}

type UploadArtifactResult struct {
	Artifact repository.Artifact
}

type GetArtifactDownloadResult struct {
	Artifact  repository.Artifact
	URL       string
	ExpiresAt time.Time
}

type GetArtifactContentResult struct {
	Artifact    repository.Artifact
	Content     io.ReadCloser
	ContentType string
}

type ArtifactManager struct {
	authorizer     WorkspaceAuthorizer
	repo           ArtifactRepository
	store          storage.Store
	signingSecret  []byte
	signedURLTTL   time.Duration
	maxUploadBytes int64
}

func NewArtifactManager(authorizer WorkspaceAuthorizer, repo ArtifactRepository, store storage.Store, signingSecret string, signedURLTTL time.Duration, maxUploadBytes int64) *ArtifactManager {
	return &ArtifactManager{
		authorizer:     authorizer,
		repo:           repo,
		store:          store,
		signingSecret:  []byte(signingSecret),
		signedURLTTL:   signedURLTTL,
		maxUploadBytes: maxUploadBytes,
	}
}

func (m *ArtifactManager) UploadArtifact(ctx context.Context, caller Caller, input UploadArtifactInput) (UploadArtifactResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionUploadArtifact); err != nil {
		return UploadArtifactResult{}, err
	}
	if !artifactTypePattern.MatchString(input.ArtifactType) {
		return UploadArtifactResult{}, errArtifactTypeInvalid
	}
	if input.Body == nil {
		return UploadArtifactResult{}, errArtifactFileRequired
	}

	_, _, organizationID, err := m.validateArtifactAssociations(ctx, input.WorkspaceID, input.RunID, input.RunAgentID)
	if err != nil {
		return UploadArtifactResult{}, err
	}

	fileInfo, metadataJSON, err := prepareArtifactUpload(input, m.maxUploadBytes)
	if err != nil {
		return UploadArtifactResult{}, err
	}
	defer os.Remove(fileInfo.TempPath)

	objectKey := buildArtifactStorageKey(input.WorkspaceID, input.ArtifactType, fileInfo.ChecksumSHA256)
	tempFile, err := os.Open(fileInfo.TempPath)
	if err != nil {
		return UploadArtifactResult{}, fmt.Errorf("open prepared artifact upload: %w", err)
	}
	defer tempFile.Close()

	objectMeta, err := m.store.PutObject(ctx, storage.PutObjectInput{
		Key:         objectKey,
		Body:        tempFile,
		SizeBytes:   fileInfo.SizeBytes,
		ContentType: fileInfo.ContentType,
	})
	if err != nil {
		return UploadArtifactResult{}, fmt.Errorf("store artifact object: %w", err)
	}

	artifact, err := m.repo.CreateArtifact(ctx, repository.CreateArtifactParams{
		OrganizationID:  organizationID,
		WorkspaceID:     input.WorkspaceID,
		RunID:           input.RunID,
		RunAgentID:      input.RunAgentID,
		ArtifactType:    input.ArtifactType,
		StorageBucket:   objectMeta.Bucket,
		StorageKey:      objectMeta.Key,
		ContentType:     artifactStringPtr(fileInfo.ContentType),
		SizeBytes:       artifactInt64Ptr(fileInfo.SizeBytes),
		ChecksumSHA256:  artifactStringPtr(fileInfo.ChecksumSHA256),
		Visibility:      defaultArtifactVisibility,
		RetentionStatus: defaultArtifactRetentionStatus,
		Metadata:        metadataJSON,
	})
	if err != nil {
		if cleanupErr := m.store.DeleteObject(ctx, objectMeta.Key); cleanupErr != nil && !errors.Is(cleanupErr, storage.ErrObjectNotFound) {
			return UploadArtifactResult{}, fmt.Errorf("create artifact metadata: %w (cleanup failed: %v)", err, cleanupErr)
		}
		return UploadArtifactResult{}, err
	}

	return UploadArtifactResult{Artifact: artifact}, nil
}

func (m *ArtifactManager) GetArtifactDownload(ctx context.Context, caller Caller, artifactID uuid.UUID, baseURL string) (GetArtifactDownloadResult, error) {
	artifact, err := m.repo.GetArtifactByID(ctx, artifactID)
	if err != nil {
		return GetArtifactDownloadResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, artifact.WorkspaceID); err != nil {
		return GetArtifactDownloadResult{}, err
	}

	expiresAt := storage.ExpiresAt(time.Now(), m.signedURLTTL)
	signature := m.signArtifactToken(artifact.ID, expiresAt)
	contentURL, err := buildSignedArtifactURL(baseURL, artifact.ID, expiresAt, signature)
	if err != nil {
		return GetArtifactDownloadResult{}, fmt.Errorf("build artifact download url: %w", err)
	}

	return GetArtifactDownloadResult{
		Artifact:  artifact,
		URL:       contentURL,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *ArtifactManager) GetArtifactContent(ctx context.Context, artifactID uuid.UUID, expiresAt time.Time, signature string) (GetArtifactContentResult, error) {
	if time.Now().After(expiresAt.Add(artifactExpiryGracePeriod)) {
		return GetArtifactContentResult{}, errArtifactTokenExpired
	}
	if !m.verifyArtifactToken(artifactID, expiresAt, signature) {
		return GetArtifactContentResult{}, errArtifactTokenInvalid
	}

	artifact, err := m.repo.GetArtifactByID(ctx, artifactID)
	if err != nil {
		return GetArtifactContentResult{}, err
	}

	content, meta, err := m.store.OpenObject(ctx, artifact.StorageKey)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return GetArtifactContentResult{}, repository.ErrArtifactNotFound
		}
		return GetArtifactContentResult{}, err
	}

	contentType := derefString(artifact.ContentType)
	if contentType == "" {
		contentType = meta.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return GetArtifactContentResult{
		Artifact:    artifact,
		Content:     content,
		ContentType: contentType,
	}, nil
}

func (m *ArtifactManager) validateArtifactAssociations(ctx context.Context, workspaceID uuid.UUID, runID *uuid.UUID, runAgentID *uuid.UUID) (*domain.Run, *domain.RunAgent, uuid.UUID, error) {
	var run *domain.Run
	var runAgent *domain.RunAgent
	organizationID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, nil, uuid.Nil, err
	}

	if runID != nil {
		value, err := m.repo.GetRunByID(ctx, *runID)
		if err != nil {
			return nil, nil, uuid.Nil, err
		}
		if value.WorkspaceID != workspaceID {
			return nil, nil, uuid.Nil, fmt.Errorf("%w: run does not belong to workspace", ErrForbidden)
		}
		run = &value
		organizationID = value.OrganizationID
	}

	if runAgentID != nil {
		value, err := m.repo.GetRunAgentByID(ctx, *runAgentID)
		if err != nil {
			return nil, nil, uuid.Nil, err
		}
		if value.WorkspaceID != workspaceID {
			return nil, nil, uuid.Nil, fmt.Errorf("%w: run agent does not belong to workspace", ErrForbidden)
		}
		if run != nil && value.RunID != run.ID {
			return nil, nil, uuid.Nil, fmt.Errorf("%w: run agent does not belong to supplied run", errArtifactAssociationInvalid)
		}
		runAgent = &value
		organizationID = value.OrganizationID
	}

	return run, runAgent, organizationID, nil
}

type preparedArtifactUpload struct {
	TempPath       string
	SizeBytes      int64
	ContentType    string
	ChecksumSHA256 string
}

func prepareArtifactUpload(input UploadArtifactInput, defaultMax int64) (preparedArtifactUpload, json.RawMessage, error) {
	maxUploadBytes := input.MaxUploadBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMax
	}

	tempFile, err := os.CreateTemp("", "agentclash-artifact-*")
	if err != nil {
		return preparedArtifactUpload{}, nil, fmt.Errorf("create temp artifact file: %w", err)
	}
	defer tempFile.Close()

	hash := sha256.New()
	limitedReader := io.LimitReader(input.Body, maxUploadBytes+1)
	written, err := io.Copy(io.MultiWriter(tempFile, hash), limitedReader)
	if err != nil {
		return preparedArtifactUpload{}, nil, fmt.Errorf("buffer artifact upload: %w", err)
	}
	if written > maxUploadBytes {
		return preparedArtifactUpload{}, nil, errArtifactTooLarge
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		return preparedArtifactUpload{}, nil, fmt.Errorf("rewind artifact upload: %w", err)
	}

	head := make([]byte, 512)
	n, err := tempFile.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return preparedArtifactUpload{}, nil, fmt.Errorf("read artifact header: %w", err)
	}

	contentType, err := normalizeContentType(input.DeclaredType, head[:n])
	if err != nil {
		return preparedArtifactUpload{}, nil, err
	}

	metadataJSON, err := normalizeArtifactMetadata(input.MetadataJSON, sanitizeDownloadFilename(input.Filename))
	if err != nil {
		return preparedArtifactUpload{}, nil, err
	}

	return preparedArtifactUpload{
		TempPath:       tempFile.Name(),
		SizeBytes:      written,
		ContentType:    contentType,
		ChecksumSHA256: hex.EncodeToString(hash.Sum(nil)),
	}, metadataJSON, nil
}

func normalizeContentType(declared string, sniffed []byte) (string, error) {
	contentType := strings.TrimSpace(declared)
	if contentType != "" {
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return "", fmt.Errorf("invalid content type: %w", err)
		}
		if err := validateArtifactContentType(mediaType); err != nil {
			return "", err
		}
		if len(params) == 0 {
			return mediaType, nil
		}
		return mime.FormatMediaType(mediaType, params), nil
	}

	detected := http.DetectContentType(sniffed)
	if detected == "" {
		return "application/octet-stream", nil
	}
	mediaType, _, err := mime.ParseMediaType(detected)
	if err != nil {
		return "", fmt.Errorf("invalid detected content type: %w", err)
	}
	if err := validateArtifactContentType(mediaType); err != nil {
		return "", err
	}
	return detected, nil
}

func validateArtifactContentType(mediaType string) error {
	if _, blocked := disallowedArtifactMediaTypes[strings.ToLower(strings.TrimSpace(mediaType))]; blocked {
		return fmt.Errorf("%w: %s is not allowed", errArtifactContentTypeInvalid, mediaType)
	}
	return nil
}

func normalizeArtifactMetadata(raw string, originalFilename string) (json.RawMessage, error) {
	metadata := map[string]any{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
			return nil, fmt.Errorf("%w: %v", errArtifactMetadataInvalid, err)
		}
	}
	metadata["original_filename"] = originalFilename

	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal artifact metadata: %w", err)
	}
	return encoded, nil
}

func buildArtifactStorageKey(workspaceID uuid.UUID, artifactType string, checksum string) string {
	prefix := checksum
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return filepath.ToSlash(filepath.Join(
		"workspaces",
		workspaceID.String(),
		artifactType,
		fmt.Sprintf("%s-%s-%s", time.Now().UTC().Format("20060102T150405Z"), prefix, uuid.NewString()),
	))
}

func buildSignedArtifactURL(baseURL string, artifactID uuid.UUID, expiresAt time.Time, signature string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/artifacts/" + artifactID.String() + "/content"
	query := parsed.Query()
	query.Set("expires", strconv.FormatInt(expiresAt.Unix(), 10))
	query.Set("signature", signature)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (m *ArtifactManager) signArtifactToken(artifactID uuid.UUID, expiresAt time.Time) string {
	mac := hmac.New(sha256.New, m.signingSecret)
	_, _ = mac.Write([]byte(artifactID.String()))
	_, _ = mac.Write([]byte(":"))
	_, _ = mac.Write([]byte(strconv.FormatInt(expiresAt.Unix(), 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

func (m *ArtifactManager) verifyArtifactToken(artifactID uuid.UUID, expiresAt time.Time, signature string) bool {
	expected := m.signArtifactToken(artifactID, expiresAt)
	return hmac.Equal([]byte(expected), []byte(signature))
}

type uploadArtifactResponse struct {
	ID             uuid.UUID             `json:"id"`
	WorkspaceID    uuid.UUID             `json:"workspace_id"`
	RunID          *uuid.UUID            `json:"run_id,omitempty"`
	RunAgentID     *uuid.UUID            `json:"run_agent_id,omitempty"`
	ArtifactType   string                `json:"artifact_type"`
	ContentType    *string               `json:"content_type,omitempty"`
	SizeBytes      *int64                `json:"size_bytes,omitempty"`
	ChecksumSHA256 *string               `json:"checksum_sha256,omitempty"`
	Visibility     string                `json:"visibility"`
	Metadata       json.RawMessage       `json:"metadata"`
	CreatedAt      time.Time             `json:"created_at"`
	Download       *artifactDownloadLink `json:"download,omitempty"`
}

type artifactDownloadLink struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func uploadArtifactHandler(logger *slog.Logger, service ArtifactService, maxUploadBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1024)
		if err := r.ParseMultipartForm(minInt64(maxUploadBytes, defaultMultipartMaxMemory)); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_multipart_form", "multipart form payload is invalid or too large")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "artifact_file_required", "file field is required")
			return
		}
		defer file.Close()

		runID, err := optionalUUIDFormValue(r, "run_id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
			return
		}
		runAgentID, err := optionalUUIDFormValue(r, "run_agent_id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_agent_id", err.Error())
			return
		}

		result, err := service.UploadArtifact(r.Context(), caller, UploadArtifactInput{
			WorkspaceID:    workspaceID,
			ArtifactType:   r.FormValue("artifact_type"),
			RunID:          runID,
			RunAgentID:     runAgentID,
			Filename:       header.Filename,
			DeclaredType:   header.Header.Get("Content-Type"),
			Body:           file,
			MetadataJSON:   r.FormValue("metadata"),
			MaxUploadBytes: maxUploadBytes,
		})
		if err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			case errors.Is(err, errArtifactTypeInvalid):
				writeError(w, http.StatusBadRequest, "invalid_artifact_type", "artifact_type must match [a-z0-9][a-z0-9._-]{0,63}")
			case errors.Is(err, errArtifactTooLarge):
				writeError(w, http.StatusRequestEntityTooLarge, "artifact_too_large", "artifact exceeds maximum upload size")
			case errors.Is(err, errArtifactMetadataInvalid):
				writeError(w, http.StatusBadRequest, "invalid_artifact_metadata", "metadata must be valid JSON object")
			case errors.Is(err, errArtifactContentTypeInvalid):
				writeError(w, http.StatusBadRequest, "invalid_artifact_content_type", err.Error())
			case errors.Is(err, errArtifactAssociationInvalid):
				writeError(w, http.StatusBadRequest, "invalid_artifact_association", err.Error())
			case errors.Is(err, repository.ErrRunNotFound):
				writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			case errors.Is(err, repository.ErrRunAgentNotFound):
				writeError(w, http.StatusNotFound, "run_agent_not_found", "run agent not found")
			default:
				logger.Error("artifact upload request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"workspace_id", workspaceID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusCreated, buildUploadArtifactResponse(result.Artifact, nil))
	}
}

type getArtifactDownloadResponse struct {
	ID             uuid.UUID       `json:"id"`
	WorkspaceID    uuid.UUID       `json:"workspace_id"`
	ArtifactType   string          `json:"artifact_type"`
	ContentType    *string         `json:"content_type,omitempty"`
	SizeBytes      *int64          `json:"size_bytes,omitempty"`
	ChecksumSHA256 *string         `json:"checksum_sha256,omitempty"`
	Metadata       json.RawMessage `json:"metadata"`
	URL            string          `json:"url"`
	ExpiresAt      time.Time       `json:"expires_at"`
}

func getArtifactDownloadHandler(logger *slog.Logger, service ArtifactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		artifactID, err := artifactIDFromURLParam("artifactID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_artifact_id", err.Error())
			return
		}

		result, err := service.GetArtifactDownload(r.Context(), caller, artifactID, requestBaseURL(r))
		if err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			case errors.Is(err, repository.ErrArtifactNotFound):
				writeError(w, http.StatusNotFound, "artifact_not_found", "artifact not found")
			default:
				logger.Error("artifact download request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"artifact_id", artifactID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusOK, getArtifactDownloadResponse{
			ID:             result.Artifact.ID,
			WorkspaceID:    result.Artifact.WorkspaceID,
			ArtifactType:   result.Artifact.ArtifactType,
			ContentType:    result.Artifact.ContentType,
			SizeBytes:      result.Artifact.SizeBytes,
			ChecksumSHA256: result.Artifact.ChecksumSHA256,
			Metadata:       result.Artifact.Metadata,
			URL:            result.URL,
			ExpiresAt:      result.ExpiresAt,
		})
	}
}

func getArtifactContentHandler(logger *slog.Logger, service ArtifactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		artifactID, err := artifactIDFromURLParam("artifactID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_artifact_id", err.Error())
			return
		}

		expiresRaw := r.URL.Query().Get("expires")
		signature := r.URL.Query().Get("signature")
		expiresUnix, err := strconv.ParseInt(expiresRaw, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_artifact_signature", "expires must be a unix timestamp")
			return
		}

		result, err := service.GetArtifactContent(r.Context(), artifactID, time.Unix(expiresUnix, 0).UTC(), signature)
		if err != nil {
			switch {
			case errors.Is(err, errArtifactTokenInvalid), errors.Is(err, errArtifactTokenExpired):
				writeError(w, http.StatusUnauthorized, "artifact_signature_invalid", "artifact signature is invalid or expired")
			case errors.Is(err, repository.ErrArtifactNotFound):
				writeError(w, http.StatusNotFound, "artifact_not_found", "artifact not found")
			default:
				logger.Error("artifact content request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"artifact_id", artifactID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}
		defer result.Content.Close()

		w.Header().Set("Content-Type", result.ContentType)
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if result.Artifact.SizeBytes != nil {
			w.Header().Set("Content-Length", strconv.FormatInt(*result.Artifact.SizeBytes, 10))
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", artifactDownloadFilename(result.Artifact)))
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, result.Content); err != nil {
			logger.Error("stream artifact content failed",
				"method", r.Method,
				"path", r.URL.Path,
				"artifact_id", artifactID,
				"error", err,
			)
		}
	}
}

func buildUploadArtifactResponse(artifact repository.Artifact, download *artifactDownloadLink) uploadArtifactResponse {
	return uploadArtifactResponse{
		ID:             artifact.ID,
		WorkspaceID:    artifact.WorkspaceID,
		RunID:          artifact.RunID,
		RunAgentID:     artifact.RunAgentID,
		ArtifactType:   artifact.ArtifactType,
		ContentType:    artifact.ContentType,
		SizeBytes:      artifact.SizeBytes,
		ChecksumSHA256: artifact.ChecksumSHA256,
		Visibility:     artifact.Visibility,
		Metadata:       artifact.Metadata,
		CreatedAt:      artifact.CreatedAt,
		Download:       download,
	}
}

func artifactIDFromURLParam(name string) func(*http.Request) (uuid.UUID, error) {
	return func(r *http.Request) (uuid.UUID, error) {
		raw := chi.URLParam(r, name)
		if raw == "" {
			return uuid.Nil, errors.New("artifact id is required")
		}
		value, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, errors.New("artifact id is malformed")
		}
		return value, nil
	}
}

func optionalUUIDFormValue(r *http.Request, name string) (*uuid.UUID, error) {
	raw := strings.TrimSpace(r.FormValue(name))
	if raw == "" {
		return nil, nil
	}

	value, err := uuid.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s is malformed", name)
	}
	return &value, nil
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		switch strings.ToLower(strings.TrimSpace(strings.Split(forwarded, ",")[0])) {
		case "http", "https":
			scheme = strings.ToLower(strings.TrimSpace(strings.Split(forwarded, ",")[0]))
		}
	}
	return scheme + "://" + r.Host
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func artifactStringPtr(value string) *string {
	return &value
}

func artifactInt64Ptr(value int64) *int64 {
	return &value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sanitizeDownloadFilename(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, `"`, "")
	base = strings.ReplaceAll(base, "\x00", "")
	if base == "." || base == "" || base == string(filepath.Separator) {
		return "artifact.bin"
	}
	return base
}

func artifactDownloadFilename(artifact repository.Artifact) string {
	var metadata map[string]any
	if err := json.Unmarshal(artifact.Metadata, &metadata); err == nil {
		if raw, ok := metadata["original_filename"].(string); ok && strings.TrimSpace(raw) != "" {
			return sanitizeDownloadFilename(raw)
		}
	}

	return sanitizeDownloadFilename(artifact.ArtifactType + "-" + artifact.ID.String())
}
