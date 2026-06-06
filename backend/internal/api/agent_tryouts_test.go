package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestAgentTryoutManagerCreateAnonymousPersistsGuardrailSnapshots(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"ship the first tryout"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	if tryout.OrganizationID != nil || tryout.WorkspaceID != nil {
		t.Fatalf("anonymous tryout should not be workspace-owned: org=%v workspace=%v", tryout.OrganizationID, tryout.WorkspaceID)
	}
	if tryout.Status != repository.AgentTryoutStatusQueued {
		t.Fatalf("status = %q, want queued", tryout.Status)
	}
	if tryout.ExpiresAt == nil || !tryout.ExpiresAt.Equal(now.Add(defaultAgentTryoutTTL)) {
		t.Fatalf("expires_at = %v, want %v", tryout.ExpiresAt, now.Add(defaultAgentTryoutTTL))
	}
	if tryout.AnonymousFingerprintHash == nil || *tryout.AnonymousFingerprintHash == "203.0.113.10" {
		t.Fatalf("anonymous fingerprint should be hashed, got %v", tryout.AnonymousFingerprintHash)
	}
	if len(tryout.TemplateSnapshot) == 0 || len(tryout.ToolPolicySnapshot) == 0 || len(tryout.EvaluationSpecSnapshot) == 0 {
		t.Fatalf("tryout should persist template/tool/evaluation snapshots: %+v", tryout)
	}
	if !bytes.Contains(tryout.TemplateSnapshot, []byte(`"available":true`)) ||
		!bytes.Contains(tryout.TemplateSnapshot, []byte(`"expected_artifacts"`)) ||
		!bytes.Contains(tryout.ToolPolicySnapshot, []byte(`"network":{"mode":"disabled"`)) {
		t.Fatalf("tryout snapshots should include runtime policy: template=%s tool=%s", tryout.TemplateSnapshot, tryout.ToolPolicySnapshot)
	}
}

func TestAgentTryoutManagerRejectsAnonymousWhenTemplateDisabled(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "tiny-bugfix",
		Input:                json.RawMessage(`{"task":"fix it"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrInvalidAgentTryoutInput", err)
	}
}

func TestAgentTryoutTemplatesExposeRuntimeMetadata(t *testing.T) {
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), newFakeAgentTryoutRepository(uuid.New(), uuid.New()))
	templates, err := manager.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates returned error: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("ListTemplates returned no templates")
	}
	var meeting AgentTryoutTemplate
	for _, template := range templates {
		if template.Slug == "meeting-minutes" {
			meeting = template
			break
		}
	}
	if meeting.Slug == "" {
		t.Fatal("meeting-minutes template not found")
	}
	if !meeting.Available || meeting.UnavailableReason != "" {
		t.Fatalf("meeting availability = %v reason=%q, want available", meeting.Available, meeting.UnavailableReason)
	}
	if !bytes.Contains(meeting.ToolPolicy, []byte(`"file_writer"`)) ||
		!bytes.Contains(meeting.ToolPolicy, []byte(`"network":{"mode":"disabled"`)) {
		t.Fatalf("meeting tool policy = %s, want file writer with disabled network", meeting.ToolPolicy)
	}
	if !bytes.Contains(meeting.Runtime, []byte(`"expected_artifacts"`)) ||
		!bytes.Contains(meeting.Runtime, []byte(`"validation"`)) ||
		!bytes.Contains(meeting.Runtime, []byte(`"sandbox"`)) {
		t.Fatalf("meeting runtime = %s, want artifacts, validation, and sandbox policy", meeting.Runtime)
	}
}

func TestAgentTryoutManagerRejectsInvalidTemplateInputBeforeCreate(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"audience":"execs"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrInvalidAgentTryoutInput", err)
	}
	if len(repo.tryouts) != 0 || len(repo.createdExecutions) != 0 {
		t.Fatalf("invalid input should not create or dispatch tryouts: tryouts=%d executions=%d", len(repo.tryouts), len(repo.createdExecutions))
	}
}

func TestAgentTryoutManagerRejectsUnavailableTemplateBeforeCreate(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "structured-data",
		Input:                json.RawMessage(`{"text":"name: Ada"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if !errors.Is(err, ErrAgentTryoutTemplateUnavailable) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrAgentTryoutTemplateUnavailable", err)
	}
	if len(repo.tryouts) != 0 || len(repo.createdExecutions) != 0 {
		t.Fatalf("unavailable template should not create or dispatch tryouts: tryouts=%d executions=%d", len(repo.tryouts), len(repo.createdExecutions))
	}
}

func TestAgentTryoutManagerRejectsClientRuntimePolicyFields(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"hello","tools":["sandbox_shell"],"network":"enabled","cost_limit_usd":99}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrInvalidAgentTryoutInput", err)
	}
	if len(repo.tryouts) != 0 {
		t.Fatalf("client runtime policy override should not create tryout, created %d", len(repo.tryouts))
	}
}

func TestAgentTryoutManagerRejectsOversizedInput(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	oversized := `{"notes":"` + strings.Repeat("x", 65*1024) + `"}`

	_, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(oversized),
		AnonymousFingerprint: "203.0.113.10",
	})
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrInvalidAgentTryoutInput", err)
	}
}

func TestAgentTryoutManagerCreateWorkspaceClaimAndShare(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	caller := callerWithWorkspace(workspaceID)

	tryout, err := manager.CreateWorkspaceTryout(ctx, caller, CreateWorkspaceAgentTryoutInput{
		WorkspaceID:  workspaceID,
		TemplateSlug: "tiny-bugfix",
		Input:        json.RawMessage(`{"task":"fix a nil check"}`),
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceTryout returned error: %v", err)
	}
	if tryout.OrganizationID == nil || *tryout.OrganizationID != orgID || tryout.WorkspaceID == nil || *tryout.WorkspaceID != workspaceID {
		t.Fatalf("workspace tryout scope = org %v workspace %v, want org %s workspace %s", tryout.OrganizationID, tryout.WorkspaceID, orgID, workspaceID)
	}

	anonymous, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"claim this"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	claimed, err := manager.ClaimTryout(ctx, caller, ClaimAgentTryoutInput{ID: anonymous.ID, WorkspaceID: workspaceID})
	if err != nil {
		t.Fatalf("ClaimTryout returned error: %v", err)
	}
	if claimed.ClaimedByUserID == nil || *claimed.ClaimedByUserID != caller.UserID {
		t.Fatalf("claimed_by_user_id = %v, want %s", claimed.ClaimedByUserID, caller.UserID)
	}

	result, err := manager.CreatePrivateShare(ctx, caller, tryout.ID)
	if err != nil {
		t.Fatalf("CreatePrivateShare returned error: %v", err)
	}
	if result.Share.ResourceType != repository.PublicShareResourceAgentTryout {
		t.Fatalf("share resource type = %q, want agent_tryout", result.Share.ResourceType)
	}
	if result.Share.SearchIndexing {
		t.Fatalf("agent tryout shares should default search_indexing=false")
	}
}

func TestAgentTryoutManagerDispatchesAnonymousTryout(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	starter := &fakeAgentHarnessWorkflowStarter{}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithExecution(starter, AgentTryoutExecutionConfig{
		PublicWorkspaceID:      &workspaceID,
		E2BTemplateID:          "agentclash-tryout-e2b",
		OpenAIAPIKeySecretName: "TRYOUT_OPENAI_API_KEY",
	})

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"ship the execution bridge"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	if tryout.Status != repository.AgentTryoutStatusRunning {
		t.Fatalf("status = %q, want running", tryout.Status)
	}
	if tryout.RunID == nil {
		t.Fatal("run_id was not linked")
	}
	if len(repo.createdHarnesses) != 1 || len(repo.createdExecutions) != 1 {
		t.Fatalf("created harnesses/executions = %d/%d, want 1/1", len(repo.createdHarnesses), len(repo.createdExecutions))
	}
	if starter.startedCount != 1 || starter.timeoutSeconds != 120 {
		t.Fatalf("starter count/timeout = %d/%d, want 1/120", starter.startedCount, starter.timeoutSeconds)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(repo.createdExecutions[0].HarnessSnapshot, &snapshot); err != nil {
		t.Fatalf("decode harness snapshot: %v", err)
	}
	if snapshot["codex_template"] != "agentclash-tryout-e2b" || snapshot["openai_api_key_secret_name"] != "TRYOUT_OPENAI_API_KEY" {
		t.Fatalf("snapshot provider config = %#v", snapshot)
	}
	if !strings.Contains(snapshot["task_prompt"].(string), "ship the execution bridge") {
		t.Fatalf("task prompt did not include input: %s", snapshot["task_prompt"])
	}
}

func TestAgentTryoutManagerDispatchIsIdempotentWhenRunAlreadyLinked(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	starter := &fakeAgentHarnessWorkflowStarter{}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithExecution(starter, AgentTryoutExecutionConfig{PublicWorkspaceID: &workspaceID})

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"first run"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	template, _ := manager.lookupTemplate("meeting-minutes")
	again, err := manager.execution.dispatch(ctx, tryout, template)
	if err != nil {
		t.Fatalf("second dispatch returned error: %v", err)
	}
	if again.RunID == nil || *again.RunID != *tryout.RunID {
		t.Fatalf("second dispatch run_id = %v, want %s", again.RunID, *tryout.RunID)
	}
	if len(repo.createdExecutions) != 1 || starter.startedCount != 1 {
		t.Fatalf("created executions/start count = %d/%d, want 1/1", len(repo.createdExecutions), starter.startedCount)
	}
}

func TestAgentTryoutManagerMarksFailedWhenDispatchStartFails(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	starterErr := errors.New("temporal unavailable")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithExecution(&fakeAgentHarnessWorkflowStarter{err: starterErr}, AgentTryoutExecutionConfig{PublicWorkspaceID: &workspaceID})

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"fail safely"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	if tryout.Status != repository.AgentTryoutStatusFailed {
		t.Fatalf("status = %q, want failed", tryout.Status)
	}
	if repo.transitionedStatus != repository.AgentHarnessExecutionStatusFailed {
		t.Fatalf("harness status = %q, want failed", repo.transitionedStatus)
	}
	var summary map[string]any
	if err := json.Unmarshal(tryout.Summary, &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary["code"] != "execution_start_failed" || strings.Contains(string(tryout.Summary), starterErr.Error()) {
		t.Fatalf("unsafe failure summary: %s", tryout.Summary)
	}
}

func TestAgentTryoutManagerDoesNotRevertFailedTryoutWhenHarnessTransitionFails(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	repo.transitionErr = errors.New("transition failed")
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithExecution(&fakeAgentHarnessWorkflowStarter{err: errors.New("temporal unavailable")}, AgentTryoutExecutionConfig{PublicWorkspaceID: &workspaceID})

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"fail and stay failed"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	if tryout.Status != repository.AgentTryoutStatusFailed {
		t.Fatalf("created status = %q, want failed", tryout.Status)
	}
	refreshed, err := manager.GetPublicTryout(ctx, tryout.ID)
	if err != nil {
		t.Fatalf("GetPublicTryout returned error: %v", err)
	}
	if refreshed.Status != repository.AgentTryoutStatusFailed {
		t.Fatalf("refreshed status = %q, want failed", refreshed.Status)
	}
}

func TestAgentTryoutManagerRejectsHarnessExecutionWithoutRunID(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	repo.omitExecutionRunID = true
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithExecution(&fakeAgentHarnessWorkflowStarter{}, AgentTryoutExecutionConfig{PublicWorkspaceID: &workspaceID})

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"missing run id"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	if tryout.Status != repository.AgentTryoutStatusFailed {
		t.Fatalf("status = %q, want failed", tryout.Status)
	}
	if tryout.RunID != nil {
		t.Fatalf("run_id = %v, want nil when execution run id is missing", tryout.RunID)
	}
	var summary map[string]any
	if err := json.Unmarshal(tryout.Summary, &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary["code"] != "execution_link_missing" {
		t.Fatalf("summary code = %v, want execution_link_missing", summary["code"])
	}
}

func TestAgentTryoutManagerMapsHarnessExecutionStatusOnRead(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithExecution(&fakeAgentHarnessWorkflowStarter{}, AgentTryoutExecutionConfig{PublicWorkspaceID: &workspaceID})

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"complete it"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	execution := repo.executionsByRunID[*tryout.RunID]
	started := time.Now().UTC().Add(-2 * time.Second)
	completed := time.Now().UTC()
	execution.Status = string(repository.AgentHarnessExecutionStatusCompleted)
	execution.StartedAt = &started
	execution.CompletedAt = &completed
	repo.executionsByRunID[*tryout.RunID] = execution

	refreshed, err := manager.GetPublicTryout(ctx, tryout.ID)
	if err != nil {
		t.Fatalf("GetPublicTryout returned error: %v", err)
	}
	if refreshed.Status != repository.AgentTryoutStatusCompleted || refreshed.RedactionStatus != repository.AgentTryoutRedactionPassed {
		t.Fatalf("refreshed tryout = %#v, want completed/passed", refreshed)
	}
	if refreshed.LatencyMS == nil || *refreshed.LatencyMS <= 0 {
		t.Fatalf("latency_ms = %v, want positive", refreshed.LatencyMS)
	}
}

func TestAgentTryoutManagerSkipsNoopRefreshWrite(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithExecution(&fakeAgentHarnessWorkflowStarter{}, AgentTryoutExecutionConfig{PublicWorkspaceID: &workspaceID})

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"avoid writes"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	_, err = manager.GetPublicTryout(ctx, tryout.ID)
	if err != nil {
		t.Fatalf("first GetPublicTryout returned error: %v", err)
	}
	updatesAfterFirstRefresh := repo.updateStatusCalls
	_, err = manager.GetPublicTryout(ctx, tryout.ID)
	if err != nil {
		t.Fatalf("second GetPublicTryout returned error: %v", err)
	}
	if repo.updateStatusCalls != updatesAfterFirstRefresh {
		t.Fatalf("update calls = %d, want unchanged %d after noop refresh", repo.updateStatusCalls, updatesAfterFirstRefresh)
	}
}

func TestBuildAgentTryoutHarnessPayload(t *testing.T) {
	template := builtinAgentTryoutTemplates()[0]
	tryout := repository.AgentTryout{ID: uuid.New(), InputSnapshot: json.RawMessage(`{"notes":"turn this into actions"}`)}
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	dispatcher := agentTryoutExecutionDispatcher{repo: repo, starter: &fakeAgentHarnessWorkflowStarter{}, config: normalizeAgentTryoutExecutionConfig(AgentTryoutExecutionConfig{E2BTemplateID: "tryout-template"})}
	harness := repository.AgentHarness{ID: uuid.New(), OrganizationID: repo.orgID, WorkspaceID: repo.workspaceID}
	executionConfig := dispatcher.executionConfig(template)
	evaluationConfig := agentTryoutEvaluationConfig(template, tryout)

	snapshot, err := dispatcher.harnessSnapshot(harness, template, tryout, executionConfig, evaluationConfig)
	if err != nil {
		t.Fatalf("harnessSnapshot returned error: %v", err)
	}
	var decoded struct {
		TaskPrompt       string          `json:"task_prompt"`
		CodexTemplate    string          `json:"codex_template"`
		ExecutionConfig  json.RawMessage `json:"execution_config"`
		EvaluationConfig json.RawMessage `json:"evaluation_config"`
	}
	if err := json.Unmarshal(snapshot, &decoded); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if decoded.CodexTemplate != "tryout-template" || !strings.Contains(decoded.TaskPrompt, "turn this into actions") {
		t.Fatalf("snapshot = %+v", decoded)
	}
	if !bytes.Contains(decoded.ExecutionConfig, []byte(`"timeout_seconds":120`)) {
		t.Fatalf("execution config = %s, want timeout", decoded.ExecutionConfig)
	}
	if !bytes.Contains(decoded.ExecutionConfig, []byte(`"runtime"`)) ||
		!bytes.Contains(decoded.ExecutionConfig, []byte(`"tool_policy"`)) ||
		!bytes.Contains(decoded.ExecutionConfig, []byte(`"expected_artifacts"`)) {
		t.Fatalf("execution config = %s, want runtime policy", decoded.ExecutionConfig)
	}
	if !bytes.Contains(decoded.EvaluationConfig, []byte(`"kind":"agent_tryout"`)) {
		t.Fatalf("evaluation config = %s, want agent_tryout metadata", decoded.EvaluationConfig)
	}
	if !bytes.Contains(decoded.EvaluationConfig, []byte(`"validation"`)) {
		t.Fatalf("evaluation config = %s, want template validation metadata", decoded.EvaluationConfig)
	}
}

func TestCreateAnonymousAgentTryoutHandler(t *testing.T) {
	service := &fakeAgentTryoutService{
		tryout: repository.AgentTryout{
			ID:                     uuid.New(),
			TemplateSlug:           "meeting-minutes",
			Status:                 repository.AgentTryoutStatusQueued,
			InputSnapshot:          json.RawMessage(`{"notes":"hello"}`),
			TemplateSnapshot:       json.RawMessage(`{"slug":"meeting-minutes"}`),
			ToolPolicySnapshot:     json.RawMessage(`{"tools":[]}`),
			EvaluationSpecSnapshot: json.RawMessage(`{"validators":[]}`),
			SelectedModelPolicy:    json.RawMessage(`{"mode":"hosted_default"}`),
			Summary:                json.RawMessage(`{}`),
			RedactionStatus:        repository.AgentTryoutRedactionPending,
			CostLimitUSD:           0.25,
			MaxDurationSeconds:     120,
			CreatedAt:              time.Now().UTC(),
			UpdatedAt:              time.Now().UTC(),
		},
	}
	handler := createAnonymousAgentTryoutHandler(slog.Default(), service)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent-tryouts", bytes.NewBufferString(`{"template_slug":"meeting-minutes","input":{"notes":"hello"}}`))
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s, want 201", rr.Code, rr.Body.String())
	}
	if service.createAnonymousInput.TemplateSlug != "meeting-minutes" {
		t.Fatalf("template slug = %q, want meeting-minutes", service.createAnonymousInput.TemplateSlug)
	}
	if service.createAnonymousInput.AnonymousFingerprint != "203.0.113.10" {
		t.Fatalf("fingerprint = %q, want first forwarded IP", service.createAnonymousInput.AnonymousFingerprint)
	}
}

func TestListAgentTryoutTemplatesHandlerReturnsRuntimeMetadata(t *testing.T) {
	service := &fakeAgentTryoutService{templates: builtinAgentTryoutTemplates()}
	handler := listAgentTryoutTemplatesHandler(slog.Default(), service)
	req := httptest.NewRequest(http.MethodGet, "/v1/agent-tryout-templates", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"available":true`) ||
		!strings.Contains(rr.Body.String(), `"runtime"`) ||
		!strings.Contains(rr.Body.String(), `"expected_artifacts"`) {
		t.Fatalf("template response missing runtime metadata: %s", rr.Body.String())
	}
}

func TestCreateAnonymousAgentTryoutHandlerMapsUnavailableTemplate(t *testing.T) {
	service := &fakeAgentTryoutService{createAnonymousErr: fmt.Errorf("%w: structured data validator runtime is not enabled yet", ErrAgentTryoutTemplateUnavailable)}
	handler := createAnonymousAgentTryoutHandler(slog.Default(), service)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent-tryouts", bytes.NewBufferString(`{"template_slug":"structured-data","input":{"text":"name: Ada"}}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s, want 409", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"code":"template_unavailable"`) {
		t.Fatalf("body = %s, want template_unavailable", rr.Body.String())
	}
}

func TestGetPublicAgentTryoutHandlerReturnsNarrowResponse(t *testing.T) {
	expiresAt := time.Now().UTC().Add(defaultAgentTryoutTTL)
	service := &fakeAgentTryoutService{
		tryout: repository.AgentTryout{
			ID:                     uuid.New(),
			TemplateSlug:           "meeting-minutes",
			Status:                 repository.AgentTryoutStatusQueued,
			InputSnapshot:          json.RawMessage(`{"notes":"hello"}`),
			TemplateSnapshot:       json.RawMessage(`{"slug":"meeting-minutes"}`),
			ToolPolicySnapshot:     json.RawMessage(`{"tools":[]}`),
			EvaluationSpecSnapshot: json.RawMessage(`{"validators":[]}`),
			SelectedModelPolicy:    json.RawMessage(`{"mode":"hosted_default"}`),
			Summary:                json.RawMessage(`{}`),
			RedactionStatus:        repository.AgentTryoutRedactionPending,
			CostLimitUSD:           0.25,
			MaxDurationSeconds:     120,
			ExpiresAt:              &expiresAt,
			CreatedAt:              time.Now().UTC(),
			UpdatedAt:              time.Now().UTC(),
		},
	}
	router := chi.NewRouter()
	router.Get("/v1/agent-tryouts/{tryoutID}", getPublicAgentTryoutHandler(slog.Default(), service))
	req := httptest.NewRequest(http.MethodGet, "/v1/agent-tryouts/"+service.tryout.ID.String(), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := payload["expires_at"]; ok {
		t.Fatalf("public tryout response leaked expires_at: %s", rr.Body.String())
	}
	if _, ok := payload["created_by_user_id"]; ok {
		t.Fatalf("public tryout response leaked created_by_user_id: %s", rr.Body.String())
	}
}

func TestListWorkspaceAgentTryoutsHandlerPassesPagination(t *testing.T) {
	workspaceID := uuid.New()
	service := &fakeAgentTryoutService{}
	router := chi.NewRouter()
	router.Get("/v1/workspaces/{workspaceID}/agent-tryouts", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), callerContextKey{}, callerWithWorkspace(workspaceID))
		listWorkspaceAgentTryoutsHandler(slog.Default(), service).ServeHTTP(w, r.WithContext(ctx))
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-tryouts?limit=17&offset=34", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}
	if service.listLimit != 17 || service.listOffset != 34 {
		t.Fatalf("pagination = limit %d offset %d, want 17/34", service.listLimit, service.listOffset)
	}
}

func TestGetWorkspaceAgentTryoutHandlerValidatesWorkspaceIDBeforeServiceCall(t *testing.T) {
	service := &fakeAgentTryoutService{}
	router := chi.NewRouter()
	router.Get("/v1/workspaces/{workspaceID}/agent-tryouts/{tryoutID}", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), callerContextKey{}, callerWithWorkspace(uuid.New()))
		getWorkspaceAgentTryoutHandler(slog.Default(), service).ServeHTTP(w, r.WithContext(ctx))
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/not-a-uuid/agent-tryouts/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", rr.Code, rr.Body.String())
	}
	if service.getWorkspaceCalls != 0 {
		t.Fatalf("GetWorkspaceTryout calls = %d, want 0 before malformed workspace id is rejected", service.getWorkspaceCalls)
	}
}

type fakeAgentTryoutRepository struct {
	orgID              uuid.UUID
	workspaceID        uuid.UUID
	tryouts            map[uuid.UUID]repository.AgentTryout
	harnessBySlug      map[string]repository.AgentHarness
	executionsByRunID  map[uuid.UUID]repository.AgentHarnessExecution
	createdHarnesses   []repository.CreateAgentHarnessParams
	createdExecutions  []repository.CreateAgentHarnessExecutionParams
	transitionedStatus repository.AgentHarnessExecutionStatus
	transitionedReason *string
	transitionErr      error
	omitExecutionRunID bool
	updateStatusCalls  int
	share              repository.PublicShareLink
}

func newFakeAgentTryoutRepository(orgID, workspaceID uuid.UUID) *fakeAgentTryoutRepository {
	return &fakeAgentTryoutRepository{
		orgID:             orgID,
		workspaceID:       workspaceID,
		tryouts:           map[uuid.UUID]repository.AgentTryout{},
		harnessBySlug:     map[string]repository.AgentHarness{},
		executionsByRunID: map[uuid.UUID]repository.AgentHarnessExecution{},
	}
}

func (r *fakeAgentTryoutRepository) GetOrganizationIDByWorkspaceID(_ context.Context, workspaceID uuid.UUID) (uuid.UUID, error) {
	if workspaceID != r.workspaceID {
		return uuid.Nil, repository.ErrWorkspaceSecretNotFound
	}
	return r.orgID, nil
}

func (r *fakeAgentTryoutRepository) CreateAgentTryout(_ context.Context, params repository.CreateAgentTryoutParams) (repository.AgentTryout, error) {
	now := time.Now().UTC()
	tryout := repository.AgentTryout{
		ID:                       uuid.New(),
		OrganizationID:           params.OrganizationID,
		WorkspaceID:              params.WorkspaceID,
		TemplateSlug:             params.TemplateSlug,
		Status:                   params.Status,
		InputSnapshot:            params.InputSnapshot,
		TemplateSnapshot:         params.TemplateSnapshot,
		ToolPolicySnapshot:       params.ToolPolicySnapshot,
		EvaluationSpecSnapshot:   params.EvaluationSpecSnapshot,
		SelectedModelPolicy:      params.SelectedModelPolicy,
		Summary:                  params.Summary,
		RedactionStatus:          params.RedactionStatus,
		RunID:                    params.RunID,
		CostLimitUSD:             params.CostLimitUSD,
		ActualCostUSD:            params.ActualCostUSD,
		LatencyMS:                params.LatencyMS,
		MaxDurationSeconds:       params.MaxDurationSeconds,
		AnonymousFingerprintHash: params.AnonymousFingerprintHash,
		CreatedByUserID:          params.CreatedByUserID,
		ExpiresAt:                params.ExpiresAt,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	r.tryouts[tryout.ID] = tryout
	return tryout, nil
}

func (r *fakeAgentTryoutRepository) GetAgentHarnessByWorkspaceSlug(_ context.Context, workspaceID uuid.UUID, slug string) (repository.AgentHarness, error) {
	harness, ok := r.harnessBySlug[workspaceID.String()+"/"+slug]
	if !ok {
		return repository.AgentHarness{}, repository.ErrAgentHarnessNotFound
	}
	return harness, nil
}

func (r *fakeAgentTryoutRepository) CreateAgentHarness(_ context.Context, params repository.CreateAgentHarnessParams) (repository.AgentHarness, error) {
	r.createdHarnesses = append(r.createdHarnesses, params)
	now := time.Now().UTC()
	harness := repository.AgentHarness{
		ID:                     uuid.New(),
		OrganizationID:         params.OrganizationID,
		WorkspaceID:            params.WorkspaceID,
		CreatedByUserID:        params.CreatedByUserID,
		Name:                   params.Name,
		Slug:                   params.Slug,
		Description:            params.Description,
		Status:                 "draft",
		HarnessKind:            params.HarnessKind,
		TaskPrompt:             params.TaskPrompt,
		CodexTemplate:          params.CodexTemplate,
		AuthMode:               params.AuthMode,
		OpenAIAPIKeySecretName: params.OpenAIAPIKeySecretName,
		ExecutionConfig:        params.ExecutionConfig,
		EvaluationConfig:       params.EvaluationConfig,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	r.harnessBySlug[params.WorkspaceID.String()+"/"+params.Slug] = harness
	return harness, nil
}

func (r *fakeAgentTryoutRepository) CreateAgentHarnessExecution(_ context.Context, params repository.CreateAgentHarnessExecutionParams) (repository.AgentHarnessExecution, error) {
	r.createdExecutions = append(r.createdExecutions, params)
	now := time.Now().UTC()
	runID := uuid.New()
	runAgentID := uuid.New()
	var runIDPtr *uuid.UUID
	if !r.omitExecutionRunID {
		runIDPtr = &runID
	}
	execution := repository.AgentHarnessExecution{
		ID:                       uuid.New(),
		OrganizationID:           params.OrganizationID,
		WorkspaceID:              params.WorkspaceID,
		AgentHarnessID:           params.AgentHarnessID,
		CreatedByUserID:          params.CreatedByUserID,
		RunID:                    runIDPtr,
		RunAgentID:               &runAgentID,
		Status:                   string(repository.AgentHarnessExecutionStatusQueued),
		HarnessSnapshot:          params.HarnessSnapshot,
		ExecutionConfigSnapshot:  params.ExecutionConfigSnapshot,
		EvaluationConfigSnapshot: params.EvaluationConfigSnapshot,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if execution.RunID != nil {
		r.executionsByRunID[runID] = execution
	}
	return execution, nil
}

func (r *fakeAgentTryoutRepository) SetAgentHarnessExecutionTemporalIDs(_ context.Context, params repository.SetAgentHarnessExecutionTemporalIDsParams) (repository.AgentHarnessExecution, error) {
	for runID, execution := range r.executionsByRunID {
		if execution.ID == params.ExecutionID {
			workflowID := params.TemporalWorkflowID
			runIDValue := params.TemporalRunID
			execution.TemporalWorkflowID = &workflowID
			execution.TemporalRunID = &runIDValue
			r.executionsByRunID[runID] = execution
			return execution, nil
		}
	}
	return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
}

func (r *fakeAgentTryoutRepository) TransitionAgentHarnessExecutionStatus(_ context.Context, params repository.TransitionAgentHarnessExecutionStatusParams) (repository.AgentHarnessExecution, error) {
	r.transitionedStatus = params.ToStatus
	r.transitionedReason = params.Reason
	if r.transitionErr != nil {
		return repository.AgentHarnessExecution{}, r.transitionErr
	}
	for runID, execution := range r.executionsByRunID {
		if execution.ID == params.ExecutionID {
			execution.Status = string(params.ToStatus)
			execution.ErrorMessage = params.Reason
			r.executionsByRunID[runID] = execution
			return execution, nil
		}
	}
	return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
}

func (r *fakeAgentTryoutRepository) GetAgentHarnessExecutionByRunID(_ context.Context, runID uuid.UUID) (repository.AgentHarnessExecution, error) {
	execution, ok := r.executionsByRunID[runID]
	if !ok {
		return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
	}
	return execution, nil
}

func (r *fakeAgentTryoutRepository) GetAgentTryoutByID(_ context.Context, id uuid.UUID) (repository.AgentTryout, error) {
	tryout, ok := r.tryouts[id]
	if !ok {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	return tryout, nil
}

func (r *fakeAgentTryoutRepository) LinkAgentTryoutRunIfUnset(_ context.Context, params repository.LinkAgentTryoutRunParams) (repository.AgentTryout, error) {
	tryout, ok := r.tryouts[params.ID]
	if !ok {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	if tryout.RunID == nil {
		tryout.RunID = &params.RunID
		if tryout.Status == repository.AgentTryoutStatusQueued {
			tryout.Status = params.Status
		}
		if len(params.Summary) > 0 {
			tryout.Summary = params.Summary
		}
	}
	r.tryouts[tryout.ID] = tryout
	return tryout, nil
}

func (r *fakeAgentTryoutRepository) UpdateAgentTryoutStatus(_ context.Context, params repository.UpdateAgentTryoutStatusParams) (repository.AgentTryout, error) {
	r.updateStatusCalls++
	tryout, ok := r.tryouts[params.ID]
	if !ok {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	tryout.Status = params.Status
	if len(params.Summary) > 0 {
		tryout.Summary = params.Summary
	}
	if params.ActualCostUSD != nil {
		tryout.ActualCostUSD = params.ActualCostUSD
	}
	if params.LatencyMS != nil {
		tryout.LatencyMS = params.LatencyMS
	}
	if params.RedactionStatus != nil {
		tryout.RedactionStatus = *params.RedactionStatus
	}
	r.tryouts[tryout.ID] = tryout
	return tryout, nil
}

func (r *fakeAgentTryoutRepository) ListAgentTryoutsByWorkspaceID(_ context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error) {
	items := []repository.AgentTryout{}
	for _, tryout := range r.tryouts {
		if tryout.WorkspaceID != nil && *tryout.WorkspaceID == workspaceID {
			items = append(items, tryout)
		}
	}
	if offset >= int32(len(items)) {
		return []repository.AgentTryout{}, nil
	}
	end := offset + limit
	if end > int32(len(items)) {
		end = int32(len(items))
	}
	return items[offset:end], nil
}

func (r *fakeAgentTryoutRepository) ClaimAgentTryout(_ context.Context, params repository.ClaimAgentTryoutParams) (repository.AgentTryout, error) {
	tryout, ok := r.tryouts[params.ID]
	if !ok {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	if tryout.WorkspaceID != nil {
		return repository.AgentTryout{}, repository.ErrAgentTryoutAlreadyClaimed
	}
	tryout.OrganizationID = &params.OrganizationID
	tryout.WorkspaceID = &params.WorkspaceID
	tryout.ClaimedByUserID = &params.ClaimedByUserID
	tryout.ClaimedAt = &params.ClaimedAt
	r.tryouts[tryout.ID] = tryout
	return tryout, nil
}

func (r *fakeAgentTryoutRepository) CreatePublicShareLink(_ context.Context, params repository.CreatePublicShareLinkParams) (repository.PublicShareLink, error) {
	r.share = repository.PublicShareLink{
		ID:              uuid.New(),
		Key:             params.Key,
		OrganizationID:  params.OrganizationID,
		WorkspaceID:     params.WorkspaceID,
		ResourceType:    params.ResourceType,
		ResourceID:      params.ResourceID,
		CreatedByUserID: params.CreatedByUserID,
		SearchIndexing:  params.SearchIndexing,
		IsActive:        true,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	return r.share, nil
}

type fakeAgentTryoutService struct {
	tryout               repository.AgentTryout
	templates            []AgentTryoutTemplate
	createAnonymousErr   error
	createAnonymousInput CreateAnonymousAgentTryoutInput
	listLimit            int32
	listOffset           int32
	getWorkspaceCalls    int
}

func (s *fakeAgentTryoutService) ListTemplates(context.Context) ([]AgentTryoutTemplate, error) {
	return s.templates, nil
}

func (s *fakeAgentTryoutService) CreateAnonymousTryout(_ context.Context, input CreateAnonymousAgentTryoutInput) (repository.AgentTryout, error) {
	s.createAnonymousInput = input
	if s.createAnonymousErr != nil {
		return repository.AgentTryout{}, s.createAnonymousErr
	}
	return s.tryout, nil
}

func (s *fakeAgentTryoutService) CreateWorkspaceTryout(context.Context, Caller, CreateWorkspaceAgentTryoutInput) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, nil
}

func (s *fakeAgentTryoutService) GetPublicTryout(context.Context, uuid.UUID) (repository.AgentTryout, error) {
	return s.tryout, nil
}

func (s *fakeAgentTryoutService) GetWorkspaceTryout(context.Context, Caller, uuid.UUID) (repository.AgentTryout, error) {
	s.getWorkspaceCalls++
	return s.tryout, nil
}

func (s *fakeAgentTryoutService) ListWorkspaceTryouts(_ context.Context, _ Caller, _ uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error) {
	s.listLimit = limit
	s.listOffset = offset
	return nil, nil
}

func (s *fakeAgentTryoutService) ClaimTryout(context.Context, Caller, ClaimAgentTryoutInput) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, nil
}

func (s *fakeAgentTryoutService) CreatePrivateShare(context.Context, Caller, uuid.UUID) (CreateAgentTryoutShareResult, error) {
	return CreateAgentTryoutShareResult{}, nil
}
