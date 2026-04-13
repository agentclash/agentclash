package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/storage"
	"github.com/google/uuid"
)

func TestArtifactManagerUploadAndSignedDownloadFlow(t *testing.T) {
	t.Parallel()

	workspaceID := uuid.New()
	repo := &fakeArtifactRepository{
		organizationID: uuid.New(),
	}
	store, err := storage.NewFilesystemStore(storage.Config{
		Bucket:         "test-bucket",
		FilesystemRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewFilesystemStore returned error: %v", err)
	}

	manager := NewArtifactManager(NewCallerWorkspaceAuthorizer(), repo, store, "signing-secret", 5*time.Minute, 1024*1024)

	tmpFile, err := os.CreateTemp(t.TempDir(), "artifact-upload-*")
	if err != nil {
		t.Fatalf("CreateTemp returned error: %v", err)
	}
	if _, err := tmpFile.WriteString("hello artifact"); err != nil {
		t.Fatalf("WriteString returned error: %v", err)
	}
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek returned error: %v", err)
	}
	t.Cleanup(func() { tmpFile.Close() })

	result, err := manager.UploadArtifact(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, UploadArtifactInput{
		WorkspaceID:  workspaceID,
		ArtifactType: "fixture",
		Filename:     "../secret.txt",
		Body:         tmpFile,
	})
	if err != nil {
		t.Fatalf("UploadArtifact returned error: %v", err)
	}
	if result.Artifact.Visibility != defaultArtifactVisibility {
		t.Fatalf("visibility = %q, want %q", result.Artifact.Visibility, defaultArtifactVisibility)
	}

	var metadata map[string]any
	if err := json.Unmarshal(result.Artifact.Metadata, &metadata); err != nil {
		t.Fatalf("failed to decode metadata: %v", err)
	}
	if metadata["original_filename"] != "secret.txt" {
		t.Fatalf("original_filename = %v, want secret.txt", metadata["original_filename"])
	}

	download, err := manager.GetArtifactDownload(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, result.Artifact.ID, "http://example.test")
	if err != nil {
		t.Fatalf("GetArtifactDownload returned error: %v", err)
	}
	if !strings.Contains(download.URL, "/artifacts/"+result.Artifact.ID.String()+"/content") {
		t.Fatalf("download url = %q, want signed artifact content url", download.URL)
	}

	req := httptest.NewRequest(http.MethodGet, download.URL, nil)
	recorder := httptest.NewRecorder()
	newRouter("dev", nil,
		artifactTestLogger(t),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		manager,
		1024*1024,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); body != "hello artifact" {
		t.Fatalf("body = %q, want hello artifact", body)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="secret.txt"`) {
		t.Fatalf("content disposition = %q, want sanitized filename", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "sandbox") {
		t.Fatalf("Content-Security-Policy = %q, want sandbox policy", got)
	}
}

func TestArtifactContentRejectsInvalidSignature(t *testing.T) {
	t.Parallel()

	artifactID := uuid.New()
	manager := NewArtifactManager(NewCallerWorkspaceAuthorizer(), &fakeArtifactRepository{
		organizationID: uuid.New(),
		artifact: repository.Artifact{
			ID:           artifactID,
			WorkspaceID:  uuid.New(),
			StorageKey:   filepath.ToSlash("workspaces/demo/file"),
			ArtifactType: "fixture",
			Metadata:     []byte(`{}`),
		},
	}, &fakeArtifactStore{}, "signing-secret", 5*time.Minute, 1024*1024)

	_, err := manager.GetArtifactContent(context.Background(), artifactID, time.Now().Add(1*time.Minute), "bad-signature")
	if err == nil {
		t.Fatalf("expected invalid signature error")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("error = %v, want invalid", err)
	}
}

func TestArtifactManagerCleansUpStoredObjectWhenMetadataWriteFails(t *testing.T) {
	t.Parallel()

	workspaceID := uuid.New()
	store := &fakeArtifactStore{}
	manager := NewArtifactManager(NewCallerWorkspaceAuthorizer(), &fakeArtifactRepository{
		organizationID: uuid.New(),
		createErr:      errors.New("db unavailable"),
	}, store, "signing-secret", 5*time.Minute, 1024*1024)

	tmpFile, err := os.CreateTemp(t.TempDir(), "artifact-upload-*")
	if err != nil {
		t.Fatalf("CreateTemp returned error: %v", err)
	}
	if _, err := tmpFile.WriteString("hello artifact"); err != nil {
		t.Fatalf("WriteString returned error: %v", err)
	}
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek returned error: %v", err)
	}
	t.Cleanup(func() { tmpFile.Close() })

	_, err = manager.UploadArtifact(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, UploadArtifactInput{
		WorkspaceID:  workspaceID,
		ArtifactType: "fixture",
		Filename:     "secret.txt",
		Body:         tmpFile,
	})
	if err == nil {
		t.Fatalf("expected upload to fail when metadata write fails")
	}
	if store.deletedKey == "" {
		t.Fatalf("expected cleanup delete to run")
	}
}

func TestRequestBaseURLIgnoresInvalidForwardedProto(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://example.test/v1/artifacts/demo/download", nil)
	req.Host = "example.test"
	req.Header.Set("X-Forwarded-Proto", "javascript")

	got := requestBaseURL(req)
	if got != "http://example.test" {
		t.Fatalf("requestBaseURL = %q, want http://example.test", got)
	}
}

func TestArtifactContentAllowsShortExpirySkewGracePeriod(t *testing.T) {
	t.Parallel()

	artifactID := uuid.New()
	manager := NewArtifactManager(NewCallerWorkspaceAuthorizer(), &fakeArtifactRepository{
		organizationID: uuid.New(),
		artifact: repository.Artifact{
			ID:           artifactID,
			WorkspaceID:  uuid.New(),
			StorageKey:   filepath.ToSlash("workspaces/demo/file"),
			ArtifactType: "fixture",
			Metadata:     []byte(`{}`),
		},
	}, &fakeArtifactStore{}, "signing-secret", 5*time.Minute, 1024*1024)

	expiresAt := time.Now().Add(-10 * time.Second).UTC()
	signature := manager.signArtifactToken(artifactID, expiresAt)
	result, err := manager.GetArtifactContent(context.Background(), artifactID, expiresAt, signature)
	if err != nil {
		t.Fatalf("GetArtifactContent returned error: %v", err)
	}
	defer result.Content.Close()
}

func TestNormalizeContentTypeRejectsExecutableWebContent(t *testing.T) {
	t.Parallel()

	_, err := normalizeContentType("text/html; charset=utf-8", []byte("<html></html>"))
	if err == nil {
		t.Fatalf("expected executable content type to be rejected")
	}
	if !errors.Is(err, errArtifactContentTypeInvalid) {
		t.Fatalf("error = %v, want errArtifactContentTypeInvalid", err)
	}
}

type fakeArtifactRepository struct {
	organizationID uuid.UUID
	artifact       repository.Artifact
	createErr      error
}

func (r *fakeArtifactRepository) CreateArtifact(_ context.Context, params repository.CreateArtifactParams) (repository.Artifact, error) {
	if r.createErr != nil {
		return repository.Artifact{}, r.createErr
	}
	r.artifact = repository.Artifact{
		ID:              uuid.New(),
		OrganizationID:  params.OrganizationID,
		WorkspaceID:     params.WorkspaceID,
		RunID:           params.RunID,
		RunAgentID:      params.RunAgentID,
		ArtifactType:    params.ArtifactType,
		StorageBucket:   params.StorageBucket,
		StorageKey:      params.StorageKey,
		ContentType:     params.ContentType,
		SizeBytes:       params.SizeBytes,
		ChecksumSHA256:  params.ChecksumSHA256,
		Visibility:      params.Visibility,
		RetentionStatus: params.RetentionStatus,
		Metadata:        params.Metadata,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	return r.artifact, nil
}

func (r *fakeArtifactRepository) GetArtifactByID(_ context.Context, artifactID uuid.UUID) (repository.Artifact, error) {
	if r.artifact.ID != artifactID {
		return repository.Artifact{}, repository.ErrArtifactNotFound
	}
	return r.artifact, nil
}

func (r *fakeArtifactRepository) GetRunByID(_ context.Context, runID uuid.UUID) (domain.Run, error) {
	return domain.Run{}, repository.ErrRunNotFound
}

func (r *fakeArtifactRepository) GetRunAgentByID(_ context.Context, runAgentID uuid.UUID) (domain.RunAgent, error) {
	return domain.RunAgent{}, repository.ErrRunAgentNotFound
}

func (r *fakeArtifactRepository) GetOrganizationIDByWorkspaceID(_ context.Context, workspaceID uuid.UUID) (uuid.UUID, error) {
	return r.organizationID, nil
}

type fakeArtifactStore struct {
	deletedKey string
}

func (s *fakeArtifactStore) Bucket() string { return "test-bucket" }

func (s *fakeArtifactStore) PutObject(ctx context.Context, input storage.PutObjectInput) (storage.ObjectMetadata, error) {
	return storage.ObjectMetadata{Bucket: "test-bucket", Key: input.Key, ContentType: input.ContentType}, nil
}

func (s *fakeArtifactStore) OpenObject(ctx context.Context, key string) (io.ReadCloser, storage.ObjectMetadata, error) {
	return io.NopCloser(strings.NewReader("artifact")), storage.ObjectMetadata{Key: key, ContentType: "text/plain"}, nil
}

func (s *fakeArtifactStore) DeleteObject(ctx context.Context, key string) error {
	s.deletedKey = key
	return nil
}

func artifactTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, nil))
}
