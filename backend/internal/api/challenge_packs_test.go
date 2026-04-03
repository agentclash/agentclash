package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"log/slog"
)

type fakeChallengePackReadRepository struct {
	lastWorkspaceID uuid.UUID
}

func (f *fakeChallengePackReadRepository) ListVisibleChallengePacks(_ context.Context, workspaceID uuid.UUID) ([]repository.ChallengePackSummary, error) {
	f.lastWorkspaceID = workspaceID
	return []repository.ChallengePackSummary{
		{
			ID:        uuid.New(),
			Name:      "Workspace Pack",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}, nil
}

func (f *fakeChallengePackReadRepository) ListRunnableChallengePVersionsByPackID(_ context.Context, challengePackID uuid.UUID) ([]repository.ChallengePackVersionSummary, error) {
	return []repository.ChallengePackVersionSummary{
		{
			ID:              uuid.New(),
			ChallengePackID: challengePackID,
			VersionNumber:   1,
			LifecycleStatus: "runnable",
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		},
	}, nil
}

type fakeChallengePackAuthoringRepository struct {
	published repository.PublishedChallengePack
}

func (f *fakeChallengePackAuthoringRepository) PublishChallengePackBundle(_ context.Context, _ repository.PublishChallengePackBundleParams) (repository.PublishedChallengePack, error) {
	if f.published.ChallengePackID == uuid.Nil {
		f.published = repository.PublishedChallengePack{
			ChallengePackID:        uuid.New(),
			ChallengePackVersionID: uuid.New(),
			EvaluationSpecID:       uuid.New(),
			InputSetIDs:            []uuid.UUID{uuid.New()},
		}
	}
	return f.published, nil
}

func TestChallengePackReadManagerUsesWorkspaceFromContext(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeChallengePackReadRepository{}
	manager := NewChallengePackReadManager(repo)

	ctx := context.WithValue(context.Background(), workspaceIDContextKey{}, workspaceID)
	result, err := manager.ListChallengePacks(ctx)
	if err != nil {
		t.Fatalf("ListChallengePacks returned error: %v", err)
	}

	if repo.lastWorkspaceID != workspaceID {
		t.Fatalf("workspaceID = %s, want %s", repo.lastWorkspaceID, workspaceID)
	}
	if len(result.Packs) != 1 {
		t.Fatalf("pack count = %d, want 1", len(result.Packs))
	}
}

func TestChallengePackAuthoringManagerValidateBundleReturnsFieldErrors(t *testing.T) {
	manager := NewChallengePackAuthoringManager(&fakeChallengePackAuthoringRepository{})

	result, err := manager.ValidateBundle(context.Background(), []byte("pack:\n  slug: \"\"\n"))
	if err != nil {
		t.Fatalf("ValidateBundle returned error: %v", err)
	}
	if result.Valid {
		t.Fatal("Valid = true, want false")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestPublishChallengePackHandlerReturnsCreatedResponse(t *testing.T) {
	logger := challengePackTestLogger(t)
	service := NewChallengePackAuthoringManager(&fakeChallengePackAuthoringRepository{})
	workspaceID := uuid.New()
	userID := uuid.New()

	body := []byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: easy
`)

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/challenge-packs", bytes.NewReader(body))
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	recorder := httptest.NewRecorder()

	newRouter(
		logger,
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		service,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	var response PublishChallengePackResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ChallengePackID == uuid.Nil {
		t.Fatal("challenge_pack_id is nil")
	}
}

func challengePackTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, nil))
}
