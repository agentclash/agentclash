package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestAgentHarnessManagerCreateValidatesRequiredFields(t *testing.T) {
	workspaceID := uuid.New()
	caller := testAgentHarnessCaller(workspaceID)
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), &fakeAgentHarnessRepo{
		organizationID: uuid.New(),
	})

	tests := []struct {
		name  string
		input CreateAgentHarnessInput
		code  string
	}{
		{
			name: "name required",
			input: CreateAgentHarnessInput{
				TaskPrompt: "Do the task",
				AuthMode:   AgentHarnessAuthModeAPIKeySecret,
			},
			code: "invalid_name",
		},
		{
			name: "task prompt required",
			input: CreateAgentHarnessInput{
				Name:     "Codex harness",
				AuthMode: AgentHarnessAuthModeAPIKeySecret,
			},
			code: "invalid_task_prompt",
		},
		{
			name: "known auth mode required",
			input: CreateAgentHarnessInput{
				Name:       "Codex harness",
				TaskPrompt: "Do the task",
				AuthMode:   "oauth_magic",
			},
			code: "invalid_auth_mode",
		},
		{
			name: "api key auth needs secret",
			input: CreateAgentHarnessInput{
				Name:       "Codex harness",
				TaskPrompt: "Do the task",
				AuthMode:   AgentHarnessAuthModeAPIKeySecret,
			},
			code: "missing_openai_secret",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := manager.CreateAgentHarness(context.Background(), caller, workspaceID, tc.input)
			var validationErr AgentHarnessValidationError
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !errors.As(err, &validationErr) {
				t.Fatalf("error type = %T, want AgentHarnessValidationError", err)
			}
			if validationErr.Code != tc.code {
				t.Fatalf("code = %q, want %q", validationErr.Code, tc.code)
			}
		})
	}
}

func TestAgentHarnessManagerCreatePersistsHarnessDefaults(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	repo := &fakeAgentHarnessRepo{organizationID: orgID}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	harness, err := manager.CreateAgentHarness(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, CreateAgentHarnessInput{
		Name:                   " Codex Long Runner ",
		Description:            "  checks long tasks  ",
		TaskPrompt:             "  implement the requested change  ",
		AuthMode:               AgentHarnessAuthModeAPIKeySecret,
		OpenAIAPIKeySecretName: " OPENAI_API_KEY ",
		RepositoryURL:          " https://github.com/acme/repo ",
		BaseBranch:             " main ",
		EvaluationConfig:       json.RawMessage(`{"validators":[{"type":"command","command":"go test ./..."}]}`),
	})
	if err != nil {
		t.Fatalf("CreateAgentHarness error: %v", err)
	}

	if harness.OrganizationID != orgID {
		t.Fatalf("organization_id = %s, want %s", harness.OrganizationID, orgID)
	}
	if harness.Name != "Codex Long Runner" {
		t.Fatalf("name = %q", harness.Name)
	}
	if harness.Slug != "codex-long-runner" {
		t.Fatalf("slug = %q", harness.Slug)
	}
	if harness.CodexTemplate != "codex" {
		t.Fatalf("codex_template = %q, want codex", harness.CodexTemplate)
	}
	if harness.OpenAIAPIKeySecretName == nil || *harness.OpenAIAPIKeySecretName != "OPENAI_API_KEY" {
		t.Fatalf("openai secret = %#v", harness.OpenAIAPIKeySecretName)
	}
	if string(repo.created.EvaluationConfig) == "{}" {
		t.Fatal("evaluation_config was not persisted")
	}
}

func TestAgentHarnessManagerCreateValidatesGitHubRepositoryBinding(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeAgentHarnessRepo{
		organizationID: uuid.New(),
		githubRepoErr:  repository.ErrGitHubRepositoryNotInstalled,
	}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.CreateAgentHarness(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, CreateAgentHarnessInput{
		Name:                   "GitHub harness",
		TaskPrompt:             "Do the task",
		AuthMode:               AgentHarnessAuthModeAPIKeySecret,
		OpenAIAPIKeySecretName: "OPENAI_API_KEY",
		RepositoryProvider:     "github",
		GitHubRepositoryID:     123,
	})
	var validationErr AgentHarnessValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %T %v, want AgentHarnessValidationError", err, err)
	}
	if validationErr.Code != "github_repo_not_installed" {
		t.Fatalf("code = %q, want github_repo_not_installed", validationErr.Code)
	}
}

func TestAgentHarnessManagerCreatePersistsGitHubRepositoryMetadata(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	repo := &fakeAgentHarnessRepo{
		organizationID: orgID,
		githubRepo: repository.GitHubInstallationRepository{
			GitHubInstallationID: 456,
			GitHubRepositoryID:   123,
			FullName:             "acme/private-repo",
			DefaultBranch:        "trunk",
			HTMLURL:              "https://github.com/acme/private-repo",
			CloneURL:             "https://github.com/acme/private-repo.git",
		},
	}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	harness, err := manager.CreateAgentHarness(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, CreateAgentHarnessInput{
		Name:                   "GitHub harness",
		TaskPrompt:             "Do the task",
		AuthMode:               AgentHarnessAuthModeAPIKeySecret,
		OpenAIAPIKeySecretName: "OPENAI_API_KEY",
		RepositoryProvider:     "github",
		GitHubRepositoryID:     123,
	})
	if err != nil {
		t.Fatalf("CreateAgentHarness error: %v", err)
	}
	if harness.RepositoryProvider == nil || *harness.RepositoryProvider != "github" {
		t.Fatalf("repository_provider = %#v", harness.RepositoryProvider)
	}
	if harness.GitHubRepositoryID == nil || *harness.GitHubRepositoryID != 123 {
		t.Fatalf("github_repository_id = %#v", harness.GitHubRepositoryID)
	}
	if harness.GitHubInstallationID == nil || *harness.GitHubInstallationID != 456 {
		t.Fatalf("github_installation_id = %#v", harness.GitHubInstallationID)
	}
	if harness.RepositoryFullName == nil || *harness.RepositoryFullName != "acme/private-repo" {
		t.Fatalf("repository_full_name = %#v", harness.RepositoryFullName)
	}
	if harness.RepositoryCloneURL == nil || *harness.RepositoryCloneURL != "https://github.com/acme/private-repo.git" {
		t.Fatalf("repository_clone_url = %#v", harness.RepositoryCloneURL)
	}
	if harness.BaseBranch == nil || *harness.BaseBranch != "trunk" {
		t.Fatalf("base_branch = %#v", harness.BaseBranch)
	}
	if repo.githubLookupWorkspaceID != workspaceID {
		t.Fatalf("lookup workspace = %s, want %s", repo.githubLookupWorkspaceID, workspaceID)
	}
}

func TestAgentHarnessRoutesCreateAndList(t *testing.T) {
	workspaceID := uuid.New()
	service := &fakeAgentHarnessService{
		harnesses: []repository.AgentHarness{
			testAgentHarnessRecord(workspaceID, "Existing harness"),
		},
	}
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), callerContextKey{}, testAgentHarnessCaller(workspaceID))
			ctx = context.WithValue(ctx, workspaceIDContextKey{}, workspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Post("/v1/workspaces/{workspaceID}/agent-harnesses", createAgentHarnessHandler(slog.Default(), service))
	router.Get("/v1/workspaces/{workspaceID}/agent-harnesses", listAgentHarnessesHandler(slog.Default(), service))
	router.Get("/v1/workspaces/{workspaceID}/agent-harnesses/{harnessID}", getAgentHarnessHandler(slog.Default(), service))

	createReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/agent-harnesses", bytes.NewBufferString(`{
		"name": "Codex autonomy check",
		"task_prompt": "Make the requested change and run tests.",
		"auth_mode": "api_key_secret",
		"openai_api_key_secret_name": "OPENAI_API_KEY",
		"evaluation_config": {"llm_judges": [{"key": "autonomy"}]}
	}`))
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", createRec.Code, createRec.Body.String())
	}
	if service.createdInput.AuthMode != AgentHarnessAuthModeAPIKeySecret {
		t.Fatalf("auth_mode = %q", service.createdInput.AuthMode)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-harnesses", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d", listRec.Code)
	}
	var listed listAgentHarnessesResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Name != "Existing harness" {
		t.Fatalf("items = %#v", listed.Items)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-harnesses/"+service.harnesses[0].ID.String(), nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body %s", getRec.Code, getRec.Body.String())
	}
}

func TestAgentHarnessRouteReturnsConflictOnDuplicateSlug(t *testing.T) {
	workspaceID := uuid.New()
	service := &fakeAgentHarnessService{createErr: repository.ErrAgentHarnessSlugConflict}
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), callerContextKey{}, testAgentHarnessCaller(workspaceID))
			ctx = context.WithValue(ctx, workspaceIDContextKey{}, workspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Post("/v1/workspaces/{workspaceID}/agent-harnesses", createAgentHarnessHandler(slog.Default(), service))

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/agent-harnesses", bytes.NewBufferString(`{
		"name": "Codex autonomy check",
		"task_prompt": "Make the requested change and run tests.",
		"auth_mode": "api_key_secret",
		"openai_api_key_secret_name": "OPENAI_API_KEY"
	}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestAgentHarnessManagerGetChecksWorkspaceBeforeFetch(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeAgentHarnessRepo{organizationID: uuid.New()}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.GetAgentHarness(context.Background(), testAgentHarnessCaller(uuid.New()), workspaceID, uuid.New())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
	if repo.getByIDCalls != 0 {
		t.Fatalf("GetAgentHarnessByID calls = %d, want 0", repo.getByIDCalls)
	}
}

func TestAgentHarnessManagerGetReturnsNotFoundForWorkspaceMismatch(t *testing.T) {
	workspaceID := uuid.New()
	harness := testAgentHarnessRecord(uuid.New(), "Other workspace harness")
	repo := &fakeAgentHarnessRepo{organizationID: uuid.New(), harness: harness}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.GetAgentHarness(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, harness.ID)
	if !errors.Is(err, repository.ErrAgentHarnessNotFound) {
		t.Fatalf("error = %v, want ErrAgentHarnessNotFound", err)
	}
}

func TestAgentHarnessExecutionManagerStartSnapshotsHarness(t *testing.T) {
	workspaceID := uuid.New()
	harness := testAgentHarnessRecord(workspaceID, "Codex execution harness")
	harness.ExecutionConfig = json.RawMessage(`{"timeout_seconds":600}`)
	harness.EvaluationConfig = json.RawMessage(`{"validators":[{"type":"command"}]}`)
	repo := &fakeAgentHarnessRepo{organizationID: harness.OrganizationID, harness: harness}
	starter := &fakeAgentHarnessWorkflowStarter{}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo, starter)

	execution, err := manager.StartAgentHarnessExecution(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, harness.ID, StartAgentHarnessExecutionInput{})
	if err != nil {
		t.Fatalf("StartAgentHarnessExecution error: %v", err)
	}

	if execution.Status != "queued" {
		t.Fatalf("status = %q, want queued", execution.Status)
	}
	if repo.createdExecution.AgentHarnessID != harness.ID {
		t.Fatalf("agent_harness_id = %s, want %s", repo.createdExecution.AgentHarnessID, harness.ID)
	}
	if string(repo.createdExecution.ExecutionConfigSnapshot) != string(harness.ExecutionConfig) {
		t.Fatalf("execution snapshot = %s", repo.createdExecution.ExecutionConfigSnapshot)
	}
	if string(repo.createdExecution.EvaluationConfigSnapshot) != string(harness.EvaluationConfig) {
		t.Fatalf("evaluation snapshot = %s", repo.createdExecution.EvaluationConfigSnapshot)
	}
	if starter.timeoutSeconds != 600 {
		t.Fatalf("workflow timeout seconds = %d, want 600", starter.timeoutSeconds)
	}
	var snapshot agentHarnessResponse
	if err := json.Unmarshal(repo.createdExecution.HarnessSnapshot, &snapshot); err != nil {
		t.Fatalf("decode harness snapshot: %v", err)
	}
	if snapshot.ID != harness.ID || snapshot.Name != harness.Name {
		t.Fatalf("snapshot = %#v, want harness id/name", snapshot)
	}
}

func TestAgentHarnessExecutionManagerStartUsesChatMessagePrompt(t *testing.T) {
	workspaceID := uuid.New()
	harness := testAgentHarnessRecord(workspaceID, "Codex chat harness")
	repo := &fakeAgentHarnessRepo{organizationID: harness.OrganizationID, harness: harness}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.StartAgentHarnessExecution(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, harness.ID, StartAgentHarnessExecutionInput{
		Message: "Investigate the failing test and patch it.",
	})
	if err != nil {
		t.Fatalf("StartAgentHarnessExecution error: %v", err)
	}

	var snapshot agentHarnessResponse
	if err := json.Unmarshal(repo.createdExecution.HarnessSnapshot, &snapshot); err != nil {
		t.Fatalf("decode harness snapshot: %v", err)
	}
	if snapshot.TaskPrompt != "Investigate the failing test and patch it." {
		t.Fatalf("task prompt = %q, want chat message", snapshot.TaskPrompt)
	}
}

func TestAgentHarnessExecutionManagerMarksFailedWhenWorkflowStartFails(t *testing.T) {
	workspaceID := uuid.New()
	harness := testAgentHarnessRecord(workspaceID, "Codex execution harness")
	repo := &fakeAgentHarnessRepo{organizationID: harness.OrganizationID, harness: harness}
	starterErr := errors.New("temporal unavailable")
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo, &fakeAgentHarnessWorkflowStarter{err: starterErr})

	_, err := manager.StartAgentHarnessExecution(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, harness.ID, StartAgentHarnessExecutionInput{})
	if !errors.Is(err, starterErr) {
		t.Fatalf("error = %v, want workflow start error", err)
	}
	if repo.transitionedStatus != repository.AgentHarnessExecutionStatusFailed {
		t.Fatalf("transitioned status = %q, want failed", repo.transitionedStatus)
	}
	if repo.transitionedReason == nil || *repo.transitionedReason != starterErr.Error() {
		t.Fatalf("transitioned reason = %#v, want starter error", repo.transitionedReason)
	}
}

func TestAgentHarnessExecutionManagerStartChecksWorkspaceBeforeHarnessFetch(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeAgentHarnessRepo{organizationID: uuid.New()}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.StartAgentHarnessExecution(context.Background(), testAgentHarnessCaller(uuid.New()), workspaceID, uuid.New(), StartAgentHarnessExecutionInput{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
	if repo.getByIDCalls != 0 {
		t.Fatalf("GetAgentHarnessByID calls = %d, want 0", repo.getByIDCalls)
	}
}

func TestAgentHarnessExecutionManagerGetReturnsNotFoundForWorkspaceMismatch(t *testing.T) {
	workspaceID := uuid.New()
	execution := testAgentHarnessExecutionRecord(uuid.New(), uuid.New())
	repo := &fakeAgentHarnessRepo{organizationID: uuid.New(), execution: execution}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.GetAgentHarnessExecution(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, execution.ID)
	if !errors.Is(err, repository.ErrAgentHarnessExecutionNotFound) {
		t.Fatalf("error = %v, want ErrAgentHarnessExecutionNotFound", err)
	}
}

func TestAgentHarnessExecutionRoutes(t *testing.T) {
	workspaceID := uuid.New()
	harness := testAgentHarnessRecord(workspaceID, "Existing harness")
	execution := testAgentHarnessExecutionRecord(workspaceID, harness.ID)
	event := repository.AgentHarnessExecutionEvent{
		ID:                      1,
		AgentHarnessExecutionID: execution.ID,
		SequenceNumber:          1,
		EventType:               "execution.queued",
		ActorType:               "system",
		OccurredAt:              time.Now().UTC(),
		Payload:                 json.RawMessage(`{"message":"queued"}`),
	}
	service := &fakeAgentHarnessService{
		harnesses:  []repository.AgentHarness{harness},
		executions: []repository.AgentHarnessExecution{execution},
		events:     []repository.AgentHarnessExecutionEvent{event},
	}
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), callerContextKey{}, testAgentHarnessCaller(workspaceID))
			ctx = context.WithValue(ctx, workspaceIDContextKey{}, workspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Post("/v1/workspaces/{workspaceID}/agent-harnesses/{harnessID}/executions", startAgentHarnessExecutionHandler(slog.Default(), service))
	router.Get("/v1/workspaces/{workspaceID}/agent-harness-executions", listAgentHarnessExecutionsHandler(slog.Default(), service))
	router.Get("/v1/workspaces/{workspaceID}/agent-harness-executions/{executionID}", getAgentHarnessExecutionHandler(slog.Default(), service))

	startReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/agent-harnesses/"+harness.ID.String()+"/executions", bytes.NewBufferString(`{"message":"Patch the failing test."}`))
	startRec := httptest.NewRecorder()
	router.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusCreated {
		t.Fatalf("start status = %d, body %s", startRec.Code, startRec.Body.String())
	}
	if service.startedHarnessID != harness.ID {
		t.Fatalf("started harness id = %s, want %s", service.startedHarnessID, harness.ID)
	}
	if service.startedInput.Message != "Patch the failing test." {
		t.Fatalf("started message = %q", service.startedInput.Message)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-executions?harness_id="+harness.ID.String(), nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body %s", listRec.Code, listRec.Body.String())
	}
	if service.listExecutionsHarnessID == nil || *service.listExecutionsHarnessID != harness.ID {
		t.Fatalf("list harness id = %#v, want %s", service.listExecutionsHarnessID, harness.ID)
	}
	var listed listAgentHarnessExecutionsResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode listed executions: %v", err)
	}
	if len(listed.Items) != 1 || len(listed.Items[0].Events) != 1 || listed.Items[0].Events[0].SequenceNumber != 1 {
		t.Fatalf("listed events = %#v, want one sequenced event", listed.Items)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-executions/"+execution.ID.String(), nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body %s", getRec.Code, getRec.Body.String())
	}
	var gotExecution agentHarnessExecutionResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &gotExecution); err != nil {
		t.Fatalf("decode execution: %v", err)
	}
	if len(gotExecution.Events) != 1 || gotExecution.Events[0].SequenceNumber != 1 {
		t.Fatalf("events = %#v, want one sequenced event", gotExecution.Events)
	}
}

func testAgentHarnessCaller(workspaceID uuid.UUID) Caller {
	return Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_admin"},
		},
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{},
	}
}

func testAgentHarnessRecord(workspaceID uuid.UUID, name string) repository.AgentHarness {
	now := time.Now().UTC()
	return repository.AgentHarness{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		WorkspaceID:      workspaceID,
		Name:             name,
		Slug:             generateSlug(name),
		Description:      "description",
		Status:           "draft",
		HarnessKind:      "codex_e2b",
		TaskPrompt:       "Do the task",
		CodexTemplate:    "codex",
		AuthMode:         AgentHarnessAuthModeAPIKeySecret,
		ExecutionConfig:  json.RawMessage(`{}`),
		EvaluationConfig: json.RawMessage(`{}`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func testAgentHarnessExecutionRecord(workspaceID uuid.UUID, harnessID uuid.UUID) repository.AgentHarnessExecution {
	now := time.Now().UTC()
	return repository.AgentHarnessExecution{
		ID:                       uuid.New(),
		OrganizationID:           uuid.New(),
		WorkspaceID:              workspaceID,
		AgentHarnessID:           harnessID,
		Status:                   "queued",
		HarnessSnapshot:          json.RawMessage(`{}`),
		ExecutionConfigSnapshot:  json.RawMessage(`{}`),
		EvaluationConfigSnapshot: json.RawMessage(`{}`),
		CreatedAt:                now,
		UpdatedAt:                now,
	}
}

type fakeAgentHarnessRepo struct {
	organizationID          uuid.UUID
	created                 repository.CreateAgentHarnessParams
	createdExecution        repository.CreateAgentHarnessExecutionParams
	transitionedStatus      repository.AgentHarnessExecutionStatus
	transitionedReason      *string
	harness                 repository.AgentHarness
	execution               repository.AgentHarnessExecution
	executions              []repository.AgentHarnessExecution
	getByIDCalls            int
	githubRepo              repository.GitHubInstallationRepository
	githubRepoErr           error
	githubLookupWorkspaceID uuid.UUID
}

func (f *fakeAgentHarnessRepo) GetOrganizationIDByWorkspaceID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return f.organizationID, nil
}

func (f *fakeAgentHarnessRepo) CreateAgentHarness(_ context.Context, p repository.CreateAgentHarnessParams) (repository.AgentHarness, error) {
	f.created = p
	now := time.Now().UTC()
	return repository.AgentHarness{
		ID:                     uuid.New(),
		OrganizationID:         p.OrganizationID,
		WorkspaceID:            p.WorkspaceID,
		CreatedByUserID:        p.CreatedByUserID,
		Name:                   p.Name,
		Slug:                   p.Slug,
		Description:            p.Description,
		Status:                 "draft",
		HarnessKind:            "codex_e2b",
		TaskPrompt:             p.TaskPrompt,
		CodexTemplate:          p.CodexTemplate,
		CodexModel:             p.CodexModel,
		AuthMode:               p.AuthMode,
		OpenAIAPIKeySecretName: p.OpenAIAPIKeySecretName,
		RepositoryURL:          p.RepositoryURL,
		RepositoryProvider:     p.RepositoryProvider,
		GitHubRepositoryID:     p.GitHubRepositoryID,
		GitHubInstallationID:   p.GitHubInstallationID,
		RepositoryFullName:     p.RepositoryFullName,
		RepositoryCloneURL:     p.RepositoryCloneURL,
		BaseBranch:             p.BaseBranch,
		ExecutionConfig:        p.ExecutionConfig,
		EvaluationConfig:       p.EvaluationConfig,
		CreatedAt:              now,
		UpdatedAt:              now,
	}, nil
}

func (f *fakeAgentHarnessRepo) GetWorkspaceGitHubRepository(_ context.Context, workspaceID uuid.UUID, _ int64, _ *int64) (repository.GitHubInstallationRepository, error) {
	f.githubLookupWorkspaceID = workspaceID
	if f.githubRepoErr != nil {
		return repository.GitHubInstallationRepository{}, f.githubRepoErr
	}
	return f.githubRepo, nil
}

func (f *fakeAgentHarnessRepo) GetAgentHarnessByID(_ context.Context, id uuid.UUID) (repository.AgentHarness, error) {
	f.getByIDCalls++
	if f.harness.ID == id {
		return f.harness, nil
	}
	return repository.AgentHarness{}, repository.ErrAgentHarnessNotFound
}

func (f *fakeAgentHarnessRepo) ListAgentHarnessesByWorkspaceID(context.Context, uuid.UUID) ([]repository.AgentHarness, error) {
	return nil, nil
}

func (f *fakeAgentHarnessRepo) CreateAgentHarnessExecution(_ context.Context, p repository.CreateAgentHarnessExecutionParams) (repository.AgentHarnessExecution, error) {
	f.createdExecution = p
	now := time.Now().UTC()
	return repository.AgentHarnessExecution{
		ID:                       uuid.New(),
		OrganizationID:           p.OrganizationID,
		WorkspaceID:              p.WorkspaceID,
		AgentHarnessID:           p.AgentHarnessID,
		CreatedByUserID:          p.CreatedByUserID,
		Status:                   "queued",
		HarnessSnapshot:          p.HarnessSnapshot,
		ExecutionConfigSnapshot:  p.ExecutionConfigSnapshot,
		EvaluationConfigSnapshot: p.EvaluationConfigSnapshot,
		CreatedAt:                now,
		UpdatedAt:                now,
	}, nil
}

func (f *fakeAgentHarnessRepo) TransitionAgentHarnessExecutionStatus(_ context.Context, p repository.TransitionAgentHarnessExecutionStatusParams) (repository.AgentHarnessExecution, error) {
	f.transitionedStatus = p.ToStatus
	f.transitionedReason = p.Reason
	f.execution.Status = string(p.ToStatus)
	f.execution.ErrorMessage = p.Reason
	return f.execution, nil
}

func (f *fakeAgentHarnessRepo) GetAgentHarnessExecutionByID(_ context.Context, id uuid.UUID) (repository.AgentHarnessExecution, error) {
	if f.execution.ID == id {
		return f.execution, nil
	}
	return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
}

func (f *fakeAgentHarnessRepo) ListAgentHarnessExecutions(context.Context, repository.ListAgentHarnessExecutionsParams) ([]repository.AgentHarnessExecution, error) {
	return f.executions, nil
}

func (f *fakeAgentHarnessRepo) ListAgentHarnessExecutionEvents(context.Context, uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error) {
	return nil, nil
}

type fakeAgentHarnessService struct {
	harnesses               []repository.AgentHarness
	executions              []repository.AgentHarnessExecution
	events                  []repository.AgentHarnessExecutionEvent
	createdInput            CreateAgentHarnessInput
	startedHarnessID        uuid.UUID
	startedInput            StartAgentHarnessExecutionInput
	listExecutionsHarnessID *uuid.UUID
	createErr               error
}

func (f *fakeAgentHarnessService) CreateAgentHarness(_ context.Context, _ Caller, workspaceID uuid.UUID, input CreateAgentHarnessInput) (repository.AgentHarness, error) {
	f.createdInput = input
	if f.createErr != nil {
		return repository.AgentHarness{}, f.createErr
	}
	return testAgentHarnessRecord(workspaceID, input.Name), nil
}

func (f *fakeAgentHarnessService) GetAgentHarness(_ context.Context, _ Caller, _ uuid.UUID, id uuid.UUID) (repository.AgentHarness, error) {
	for _, harness := range f.harnesses {
		if harness.ID == id {
			return harness, nil
		}
	}
	return repository.AgentHarness{}, repository.ErrAgentHarnessNotFound
}

func (f *fakeAgentHarnessService) ListAgentHarnesses(context.Context, Caller, uuid.UUID) ([]repository.AgentHarness, error) {
	return f.harnesses, nil
}

func (f *fakeAgentHarnessService) StartAgentHarnessExecution(_ context.Context, _ Caller, workspaceID uuid.UUID, harnessID uuid.UUID, input StartAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error) {
	f.startedHarnessID = harnessID
	f.startedInput = input
	return testAgentHarnessExecutionRecord(workspaceID, harnessID), nil
}

func (f *fakeAgentHarnessService) GetAgentHarnessExecution(_ context.Context, _ Caller, _ uuid.UUID, id uuid.UUID) (repository.AgentHarnessExecution, error) {
	for _, execution := range f.executions {
		if execution.ID == id {
			return execution, nil
		}
	}
	return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
}

func (f *fakeAgentHarnessService) ListAgentHarnessExecutionEvents(context.Context, Caller, uuid.UUID, uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error) {
	return f.events, nil
}

func (f *fakeAgentHarnessService) ListAgentHarnessExecutions(_ context.Context, _ Caller, _ uuid.UUID, harnessID *uuid.UUID) ([]repository.AgentHarnessExecution, error) {
	f.listExecutionsHarnessID = harnessID
	return f.executions, nil
}

type fakeAgentHarnessWorkflowStarter struct {
	err            error
	timeoutSeconds int
}

func (f *fakeAgentHarnessWorkflowStarter) StartAgentHarnessExecutionWorkflow(_ context.Context, _ uuid.UUID, timeoutSeconds int) error {
	f.timeoutSeconds = timeoutSeconds
	return f.err
}
