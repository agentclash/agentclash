package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type stubArtifactSigner struct{}

func (stubArtifactSigner) SignedArtifactContentURL(artifactID uuid.UUID, baseURL string, now time.Time) (string, time.Time, error) {
	return baseURL + "/v1/artifacts/" + artifactID.String() + "/content?sig=test", now.Add(time.Hour), nil
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func TestAgentTryoutListArtifactsReturnsCapturedOutputs(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "slide-deck")
	runID := uuid.New()
	source.RunID = &runID
	repo.tryouts[source.ID] = source

	size := int64(42)
	repo.artifacts = []repository.Artifact{{
		ID:           uuid.New(),
		RunID:        &runID,
		ArtifactType: "agent_tryout_pptx",
		ContentType:  ptrString("application/vnd.openxmlformats-officedocument.presentationml.presentation"),
		SizeBytes:    &size,
		Metadata:     json.RawMessage(`{"source":"agent_tryout","artifact_key":"presentation","relative_path":"deck.pptx"}`),
	}, {
		ID:           uuid.New(),
		RunID:        ptrUUID(uuid.New()), // different run; must be excluded
		ArtifactType: "agent_tryout_json",
	}}

	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithArtifactSigner(stubArtifactSigner{})
	artifacts, err := manager.ListWorkspaceTryoutArtifacts(ctx, callerWithWorkspace(workspaceID), source.ID, "https://api.example.com")
	if err != nil {
		t.Fatalf("ListWorkspaceTryoutArtifacts returned error: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifacts = %d, want 1 (only the run's artifact)", len(artifacts))
	}
	got := artifacts[0]
	if got.Key != "presentation" || got.Path != "deck.pptx" {
		t.Fatalf("artifact identity = %q/%q, want presentation/deck.pptx", got.Key, got.Path)
	}
	if got.DownloadURL == "" || got.DownloadExpiresAt == nil {
		t.Fatalf("expected a signed download URL, got %q (expires=%v)", got.DownloadURL, got.DownloadExpiresAt)
	}
}

func TestAgentTryoutListArtifactsEmptyWhenNoRun(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "slide-deck") // no run_id

	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithArtifactSigner(stubArtifactSigner{})
	artifacts, err := manager.ListWorkspaceTryoutArtifacts(ctx, callerWithWorkspace(workspaceID), source.ID, "https://api.example.com")
	if err != nil {
		t.Fatalf("ListWorkspaceTryoutArtifacts returned error: %v", err)
	}
	if len(artifacts) != 0 {
		t.Fatalf("artifacts = %d, want 0 for a tryout with no run", len(artifacts))
	}
}

func seedWorkspaceTryout(repo *fakeAgentTryoutRepository, orgID, workspaceID uuid.UUID, slug string) repository.AgentTryout {
	id := uuid.New()
	tryout := repository.AgentTryout{
		ID:                     id,
		OrganizationID:         &orgID,
		WorkspaceID:            &workspaceID,
		TemplateSlug:           slug,
		Status:                 repository.AgentTryoutStatusCompleted,
		RedactionStatus:        repository.AgentTryoutRedactionPassed,
		InputSnapshot:          json.RawMessage(`{"task":"fix a nil check"}`),
		TemplateSnapshot:       json.RawMessage(`{"slug":"` + slug + `","runtime":{"expected_artifacts":[{"key":"diff","type":"patch","path":"changes.patch"}]}}`),
		ToolPolicySnapshot:     json.RawMessage(`{"tools":["file_editor"]}`),
		EvaluationSpecSnapshot: json.RawMessage(`{"validators":[]}`),
		SelectedModelPolicy:    json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
		Summary:                json.RawMessage(`{"verdict":"ready"}`),
		CostLimitUSD:           0.75,
		MaxDurationSeconds:     120,
	}
	repo.tryouts[id] = tryout
	return tryout
}

func TestAgentTryoutRerunClonesSnapshotsWithNewModelPolicy(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	policy := json.RawMessage(`{"mode":"explicit","models":[{"provider":"openai","model":"gpt-5"}]}`)
	rerun, err := manager.RerunWorkspaceTryout(ctx, callerWithWorkspace(workspaceID), RerunAgentTryoutInput{
		SourceTryoutID:      source.ID,
		SelectedModelPolicy: policy,
	})
	if err != nil {
		t.Fatalf("RerunWorkspaceTryout returned error: %v", err)
	}
	if rerun.ID == source.ID {
		t.Fatalf("rerun must be a new tryout, got same id as source")
	}
	if rerun.ParentTryoutID == nil || *rerun.ParentTryoutID != source.ID {
		t.Fatalf("rerun parent_tryout_id = %v, want %s", rerun.ParentTryoutID, source.ID)
	}
	if string(rerun.SelectedModelPolicy) != string(policy) {
		t.Fatalf("rerun model policy = %s, want %s", rerun.SelectedModelPolicy, policy)
	}
	if string(rerun.InputSnapshot) != string(source.InputSnapshot) {
		t.Fatalf("rerun should reuse source input; got %s", rerun.InputSnapshot)
	}
	if rerun.WorkspaceID == nil || *rerun.WorkspaceID != workspaceID {
		t.Fatalf("rerun workspace = %v, want %s", rerun.WorkspaceID, workspaceID)
	}
	if rerun.Status != repository.AgentTryoutStatusQueued {
		t.Fatalf("rerun status = %q, want queued", rerun.Status)
	}
	if rerun.RedactionStatus != repository.AgentTryoutRedactionPending {
		t.Fatalf("rerun redaction = %q, want pending", rerun.RedactionStatus)
	}
}

func TestRerunAgentTryoutHandlerDispatchesOnlySupportedModelPolicies(t *testing.T) {
	cases := []struct {
		name      string
		policy    string
		wantCode  string
		wantModel string
	}{
		{name: "OpenAI override", policy: `{"mode":"explicit","models":[{"provider":"openai","model":" gpt-5 "}]}`, wantModel: "gpt-5"},
		{name: "harness default", policy: `{"mode":"hosted_default"}`},
		{name: "incompatible provider", policy: `{"mode":"explicit","models":[{"provider":"anthropic","model":"claude-sonnet"}]}`, wantCode: "unsupported_workspace_model_policy"},
		{name: "multiple models", policy: `{"mode":"explicit","models":[{"provider":"openai","model":"gpt-5"},{"provider":"openai","model":"gpt-5-mini"}]}`, wantCode: "unsupported_workspace_model_policy"},
		{name: "missing explicit model", policy: `{"mode":"explicit"}`, wantCode: "invalid_model_policy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orgID, workspaceID := uuid.New(), uuid.New()
			repo := newFakeAgentTryoutRepository(orgID, workspaceID)
			source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
			starter := &fakeAgentHarnessWorkflowStarter{}
			manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithExecution(starter, AgentTryoutExecutionConfig{})
			router := chi.NewRouter()
			router.Post("/v1/agent-tryouts/{tryoutID}/rerun", rerunAgentTryoutHandler(slog.Default(), manager))
			req := httptest.NewRequest(http.MethodPost, "/v1/agent-tryouts/"+source.ID.String()+"/rerun", strings.NewReader(`{"selected_model_policy":`+tc.policy+`}`))
			req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, callerWithWorkspace(workspaceID)))
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if tc.wantCode != "" {
				if rr.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
				}
				var body errorEnvelope
				if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Error.Code != tc.wantCode {
					t.Fatalf("error = %+v, want code %q", body.Error, tc.wantCode)
				}
				if len(repo.tryouts) != 1 || len(repo.createdExecutions) != 0 || starter.startedCount != 0 {
					t.Fatal("rejected rerun created a tryout or execution")
				}
				return
			}

			if rr.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201: %s", rr.Code, rr.Body.String())
			}
			if len(repo.createdExecutions) != 1 || starter.startedCount != 1 {
				t.Fatalf("created executions/workflow starts = %d/%d, want 1/1", len(repo.createdExecutions), starter.startedCount)
			}
			var snapshot struct {
				Model string `json:"codex_model"`
			}
			if err := json.Unmarshal(repo.createdExecutions[0].HarnessSnapshot, &snapshot); err != nil {
				t.Fatal(err)
			}
			if snapshot.Model != tc.wantModel {
				t.Fatalf("snapshot model = %q, want %q", snapshot.Model, tc.wantModel)
			}
		})
	}
}

func TestAgentTryoutRerunRejectsAnonymousSource(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	id := uuid.New()
	repo.tryouts[id] = repository.AgentTryout{ID: id, TemplateSlug: "tiny-bugfix"} // no workspace
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.RerunWorkspaceTryout(ctx, callerWithWorkspace(uuid.New()), RerunAgentTryoutInput{
		SourceTryoutID:      id,
		SelectedModelPolicy: json.RawMessage(`{"mode":"hosted_default"}`),
	})
	if !errors.Is(err, ErrAgentTryoutSignInRequired) {
		t.Fatalf("error = %v, want ErrAgentTryoutSignInRequired", err)
	}
}

func TestAgentTryoutRerunRejectsCrossWorkspace(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.RerunWorkspaceTryout(ctx, callerWithWorkspace(uuid.New()), RerunAgentTryoutInput{
		SourceTryoutID:      source.ID,
		SelectedModelPolicy: json.RawMessage(`{"mode":"hosted_default"}`),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestAgentTryoutRerunValidatesModelPolicy(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	caller := callerWithWorkspace(workspaceID)

	// Empty policy → invalid.
	if _, err := manager.RerunWorkspaceTryout(ctx, caller, RerunAgentTryoutInput{SourceTryoutID: source.ID, SelectedModelPolicy: json.RawMessage(`{}`)}); !errors.Is(err, ErrAgentTryoutModelPolicyInvalid) {
		t.Fatalf("empty policy error = %v, want ErrAgentTryoutModelPolicyInvalid", err)
	}
	// Unknown provider → unavailable.
	if _, err := manager.RerunWorkspaceTryout(ctx, caller, RerunAgentTryoutInput{SourceTryoutID: source.ID, SelectedModelPolicy: json.RawMessage(`{"models":[{"provider":"acme","model":"x"}]}`)}); !errors.Is(err, ErrAgentTryoutModelUnavailable) {
		t.Fatalf("unknown provider error = %v, want ErrAgentTryoutModelUnavailable", err)
	}
}

type fakeRerunGate struct{ err error }

func (g fakeRerunGate) AuthorizeRerun(context.Context, Caller, uuid.UUID, json.RawMessage) error {
	return g.err
}

func TestAgentTryoutRerunGateDenial(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).
		WithRerunGate(fakeRerunGate{err: ErrAgentTryoutRerunInsufficientCredits})

	_, err := manager.RerunWorkspaceTryout(ctx, callerWithWorkspace(workspaceID), RerunAgentTryoutInput{
		SourceTryoutID:      source.ID,
		SelectedModelPolicy: json.RawMessage(`{"mode":"hosted_default"}`),
	})
	if !errors.Is(err, ErrAgentTryoutRerunInsufficientCredits) {
		t.Fatalf("error = %v, want ErrAgentTryoutRerunInsufficientCredits", err)
	}
}

func TestValidateTryoutModelPolicy(t *testing.T) {
	cases := []struct {
		name    string
		policy  string
		wantErr error
	}{
		{"valid mode", `{"mode":"hosted_default","max_models":1}`, nil},
		{"valid explicit model", `{"models":[{"provider":"openai","model":"gpt-5"}]}`, nil},
		{"valid custom model", `{"models":[{"provider":"custom","model":"controlled-model"}]}`, nil},
		{"empty object", `{}`, ErrAgentTryoutModelPolicyInvalid},
		{"empty input", ``, ErrAgentTryoutModelPolicyInvalid},
		{"null", `null`, ErrAgentTryoutModelPolicyInvalid},
		{"not object", `[1,2]`, ErrAgentTryoutModelPolicyInvalid},
		{"bad max_models", `{"mode":"x","max_models":0}`, ErrAgentTryoutModelPolicyInvalid},
		{"unknown field", `{"mode":"x","bogus":true}`, ErrAgentTryoutModelPolicyInvalid},
		{"unknown provider", `{"models":[{"provider":"acme","model":"x"}]}`, ErrAgentTryoutModelUnavailable},
		{"missing model id", `{"models":[{"provider":"openai","model":""}]}`, ErrAgentTryoutModelPolicyInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTryoutModelPolicy(json.RawMessage(tc.policy))
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestAgentTryoutCompareAggregatesParticipants(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	t1 := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	t2 := seedWorkspaceTryout(repo, orgID, workspaceID, "meeting-minutes")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	result, err := manager.CompareWorkspaceTryouts(ctx, callerWithWorkspace(workspaceID), CompareAgentTryoutsInput{
		WorkspaceID: workspaceID,
		TryoutIDs:   []uuid.UUID{t1.ID, t2.ID},
	})
	if err != nil {
		t.Fatalf("CompareWorkspaceTryouts returned error: %v", err)
	}
	if len(result.Participants) != 2 {
		t.Fatalf("participants = %d, want 2", len(result.Participants))
	}
	for _, p := range result.Participants {
		if p.EventsURL == "" || !strings.Contains(p.EventsURL, p.ID.String()) {
			t.Fatalf("participant %s missing events_url: %q", p.ID, p.EventsURL)
		}
		if len(p.SelectedModelPolicy) == 0 {
			t.Fatalf("participant %s missing model policy", p.ID)
		}
	}
}

func TestAgentTryoutCompareRejectsCardinality(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	if _, err := manager.CompareWorkspaceTryouts(ctx, callerWithWorkspace(workspaceID), CompareAgentTryoutsInput{WorkspaceID: workspaceID, TryoutIDs: []uuid.UUID{source.ID}}); !errors.Is(err, ErrAgentTryoutCompareCardinality) {
		t.Fatalf("single-id error = %v, want ErrAgentTryoutCompareCardinality", err)
	}
	five := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	if _, err := manager.CompareWorkspaceTryouts(ctx, callerWithWorkspace(workspaceID), CompareAgentTryoutsInput{WorkspaceID: workspaceID, TryoutIDs: five}); !errors.Is(err, ErrAgentTryoutCompareCardinality) {
		t.Fatalf("five-id error = %v, want ErrAgentTryoutCompareCardinality", err)
	}
	// Duplicate ids must not slip past the bound by collapsing to one distinct id.
	dup := []uuid.UUID{source.ID, source.ID}
	if _, err := manager.CompareWorkspaceTryouts(ctx, callerWithWorkspace(workspaceID), CompareAgentTryoutsInput{WorkspaceID: workspaceID, TryoutIDs: dup}); !errors.Is(err, ErrAgentTryoutCompareCardinality) {
		t.Fatalf("duplicate-id error = %v, want ErrAgentTryoutCompareCardinality", err)
	}
}

func TestAgentTryoutPromoteRejectsNonCompleted(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	// Force a still-running source.
	running := repo.tryouts[source.ID]
	running.Status = repository.AgentTryoutStatusRunning
	repo.tryouts[source.ID] = running
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithPackDraftBuilder(&fakePackDraftCreator{})

	if _, err := manager.PromoteTryoutToEval(ctx, callerWithWorkspace(workspaceID), PromoteAgentTryoutInput{SourceTryoutID: source.ID, Target: "vibe_eval"}); !errors.Is(err, ErrAgentTryoutNotPromotable) {
		t.Fatalf("error = %v, want ErrAgentTryoutNotPromotable", err)
	}
}

func TestAgentTryoutCompareRejectsCrossWorkspace(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	mine := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	// A tryout owned by a different workspace.
	otherWS := uuid.New()
	other := seedWorkspaceTryout(repo, orgID, otherWS, "meeting-minutes")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.CompareWorkspaceTryouts(ctx, callerWithWorkspace(workspaceID), CompareAgentTryoutsInput{
		WorkspaceID: workspaceID,
		TryoutIDs:   []uuid.UUID{mine.ID, other.ID},
	})
	if !errors.Is(err, repository.ErrAgentTryoutNotFound) {
		t.Fatalf("cross-workspace compare error = %v, want ErrAgentTryoutNotFound", err)
	}
}

// fakePackDraftCreator stands in for ChallengePackBuilderManager, recording the
// draft a promotion asks the builder to create.
type fakePackDraftCreator struct {
	err      error
	received CreateDraftInput
	calls    int
}

func (f *fakePackDraftCreator) CreateDraft(_ context.Context, _ Caller, input CreateDraftInput) (repository.ChallengePackDraft, error) {
	f.calls++
	f.received = input
	if f.err != nil {
		return repository.ChallengePackDraft{}, f.err
	}
	return repository.ChallengePackDraft{
		ID:            uuid.New(),
		WorkspaceID:   input.WorkspaceID,
		Name:          input.Name,
		ExecutionMode: input.ExecutionMode,
		Composition:   input.Composition,
		Status:        "draft",
	}, nil
}

func TestAgentTryoutPromoteCreatesChallengePackDraft(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	builder := &fakePackDraftCreator{}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithPackDraftBuilder(builder)

	result, err := manager.PromoteTryoutToEval(ctx, callerWithWorkspace(workspaceID), PromoteAgentTryoutInput{
		SourceTryoutID: source.ID,
		Target:         "vibe_eval",
	})
	if err != nil {
		t.Fatalf("PromoteTryoutToEval returned error: %v", err)
	}
	if result.Target != "vibe_eval" || result.DraftID == uuid.Nil {
		t.Fatalf("unexpected promotion result: %+v", result)
	}
	if result.WorkspaceID != workspaceID {
		t.Fatalf("result workspace = %s, want %s", result.WorkspaceID, workspaceID)
	}
	if builder.calls != 1 {
		t.Fatalf("builder CreateDraft calls = %d, want 1", builder.calls)
	}
	if builder.received.WorkspaceID != workspaceID {
		t.Fatalf("draft workspace = %s, want %s", builder.received.WorkspaceID, workspaceID)
	}
	if builder.received.ExecutionMode != challengepack.ExecutionModeNative {
		t.Fatalf("draft execution mode = %q, want native", builder.received.ExecutionMode)
	}
	if builder.received.FromChallengePackVersionID != nil {
		t.Fatalf("promotion must not hydrate from a published version")
	}

	// The composition the builder stored must be the one the builder can
	// compile and publish — assert it round-trips all the way to the YAML
	// publish path, not just that it decodes.
	var composition challengepack.Composition
	if err := json.Unmarshal(builder.received.Composition, &composition); err != nil {
		t.Fatalf("decode stored composition: %v", err)
	}
	bundle, err := challengepack.ComposeBundle(composition, challengepack.ResolvedPieces{})
	if err != nil {
		t.Fatalf("ComposeBundle on promoted draft: %v", err)
	}
	if err := challengepack.ValidateBundle(bundle); err != nil {
		t.Fatalf("promoted draft does not compile: %v", err)
	}
	yamlBytes, err := challengepack.BundleYAML(bundle)
	if err != nil {
		t.Fatalf("BundleYAML on promoted draft: %v", err)
	}
	if _, err := challengepack.ParseYAML(yamlBytes); err != nil {
		t.Fatalf("promoted draft does not publish: %v", err)
	}

	for _, want := range []string{"tiny-bugfix", "changes.patch", "fix a nil check"} {
		if !strings.Contains(string(builder.received.Composition), want) {
			t.Fatalf("composition missing %q: %s", want, builder.received.Composition)
		}
	}
}

func TestAgentTryoutPromoteReportsUnavailableWithoutBuilder(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	if _, err := manager.PromoteTryoutToEval(ctx, callerWithWorkspace(workspaceID), PromoteAgentTryoutInput{SourceTryoutID: source.ID}); !errors.Is(err, ErrAgentTryoutPromotionUnavailable) {
		t.Fatalf("error = %v, want ErrAgentTryoutPromotionUnavailable", err)
	}
}

func TestAgentTryoutPromotePropagatesBuilderAuthorization(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	builder := &fakePackDraftCreator{err: ErrForbidden}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithPackDraftBuilder(builder)

	if _, err := manager.PromoteTryoutToEval(ctx, callerWithWorkspace(workspaceID), PromoteAgentTryoutInput{SourceTryoutID: source.ID}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestAgentTryoutPromoteRejectsUnsupportedTarget(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	if _, err := manager.PromoteTryoutToEval(ctx, callerWithWorkspace(workspaceID), PromoteAgentTryoutInput{SourceTryoutID: source.ID, Target: "challenge_pack"}); !errors.Is(err, ErrAgentTryoutPromotionTargetUnsupported) {
		t.Fatalf("error = %v, want ErrAgentTryoutPromotionTargetUnsupported", err)
	}
}

func TestAgentTryoutPromoteRejectsAnonymousSource(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	id := uuid.New()
	repo.tryouts[id] = repository.AgentTryout{ID: id, TemplateSlug: "tiny-bugfix"}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithPackDraftBuilder(&fakePackDraftCreator{})

	if _, err := manager.PromoteTryoutToEval(ctx, callerWithWorkspace(uuid.New()), PromoteAgentTryoutInput{SourceTryoutID: id, Target: "vibe_eval"}); !errors.Is(err, ErrAgentTryoutSignInRequired) {
		t.Fatalf("error = %v, want ErrAgentTryoutSignInRequired", err)
	}
}
