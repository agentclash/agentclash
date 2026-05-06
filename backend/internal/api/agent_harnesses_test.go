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
			name: "known harness kind required",
			input: CreateAgentHarnessInput{
				Name:                   "Codex harness",
				HarnessKind:            "mystery_runner",
				TaskPrompt:             "Do the task",
				AuthMode:               AgentHarnessAuthModeAPIKeySecret,
				OpenAIAPIKeySecretName: "OPENAI_API_KEY",
			},
			code: "invalid_harness_kind",
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
	if repo.created.HarnessKind != AgentHarnessKindCodexE2B {
		t.Fatalf("harness_kind = %q, want codex_e2b", repo.created.HarnessKind)
	}
	if harness.OpenAIAPIKeySecretName == nil || *harness.OpenAIAPIKeySecretName != "OPENAI_API_KEY" {
		t.Fatalf("openai secret = %#v", harness.OpenAIAPIKeySecretName)
	}
	if string(repo.created.EvaluationConfig) == "{}" {
		t.Fatal("evaluation_config was not persisted")
	}
}

func TestAgentHarnessManagerCreatePersistsClaudeHarnessDefaults(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	repo := &fakeAgentHarnessRepo{organizationID: orgID}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	harness, err := manager.CreateAgentHarness(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, CreateAgentHarnessInput{
		Name:                   "Claude Long Runner",
		HarnessKind:            AgentHarnessKindClaudeE2B,
		TaskPrompt:             "implement the requested change",
		AuthMode:               AgentHarnessAuthModeAPIKeySecret,
		OpenAIAPIKeySecretName: "ANTHROPIC_API_KEY",
	})
	if err != nil {
		t.Fatalf("CreateAgentHarness error: %v", err)
	}

	if harness.HarnessKind != AgentHarnessKindClaudeE2B {
		t.Fatalf("harness_kind = %q, want claude_e2b", harness.HarnessKind)
	}
	if harness.CodexTemplate != defaultClaudeE2BTemplate {
		t.Fatalf("codex_template = %q, want %q", harness.CodexTemplate, defaultClaudeE2BTemplate)
	}
	if repo.created.HarnessKind != AgentHarnessKindClaudeE2B {
		t.Fatalf("created harness_kind = %q", repo.created.HarnessKind)
	}
}

func TestAgentHarnessManagerCreateRequiresRunPermission(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), &fakeAgentHarnessRepo{
		organizationID: uuid.New(),
	})

	_, err := manager.CreateAgentHarness(context.Background(), testAgentHarnessCallerWithRole(workspaceID, RoleWorkspaceViewer), workspaceID, CreateAgentHarnessInput{
		Name:                   "Viewer harness",
		TaskPrompt:             "Do the task",
		AuthMode:               AgentHarnessAuthModeAPIKeySecret,
		OpenAIAPIKeySecretName: "OPENAI_API_KEY",
		RepositoryURL:          "https://github.com/acme/repo",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
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

func TestAgentHarnessManagerCreateSuitePersistsVersionedTasks(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	repo := &fakeAgentHarnessRepo{organizationID: orgID}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	suite, err := manager.CreateAgentHarnessSuite(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, CreateAgentHarnessSuiteInput{
		Name:        "Rust repo private tasks",
		Description: "Autonomy checks for private Rust work",
		Metadata:    json.RawMessage(`{"suite_kind":"private_task_bank"}`),
		Tasks: []CreateAgentHarnessSuiteTaskInput{{
			Title:          "Fix ownership bug",
			PublicPrompt:   "Fix a Rust compile failure.",
			TaskPrompt:     "Fix the hidden borrow-checker failure and open a PR.",
			SourceType:     "github_issue",
			SourceSnapshot: json.RawMessage(`{"repository":"acme/rusty","number":17,"title":"Borrow checker failure","labels":["rust"]}`),
			RepositoryURL:  "https://github.com/acme/rusty",
			BaseBranch:     "main",
			ExecutionConfig: json.RawMessage(`{
				"setup_commands": ["cargo fetch"],
				"timeout_seconds": 900
			}`),
			EvaluationConfig: json.RawMessage(`{
				"validators": [{"type":"command","command":"cargo test --all"}],
				"llm_judges": [{"key":"pr_quality"}],
				"private": true
			}`),
			Metadata: json.RawMessage(`{"difficulty":"medium"}`),
		}},
	})
	if err != nil {
		t.Fatalf("CreateAgentHarnessSuite error: %v", err)
	}

	if suite.OrganizationID != orgID {
		t.Fatalf("organization_id = %s, want %s", suite.OrganizationID, orgID)
	}
	if suite.Slug != "rust-repo-private-tasks" {
		t.Fatalf("slug = %q", suite.Slug)
	}
	if suite.CurrentVersionNumber != 1 || suite.TaskCount != 1 {
		t.Fatalf("version/task count = %d/%d, want 1/1", suite.CurrentVersionNumber, suite.TaskCount)
	}
	if got := repo.createdSuite.Tasks[0]; got.SourceType != "github_issue" || string(got.EvaluationConfig) == "{}" {
		t.Fatalf("task source/eval = %q/%s, want github_issue with hidden eval config", got.SourceType, got.EvaluationConfig)
	}
	if string(repo.createdSuite.Tasks[0].SourceSnapshot) == "{}" {
		t.Fatal("github issue source snapshot was not preserved")
	}
}

func TestAgentHarnessManagerStartSuiteRunCreatesExecutionsFromTasks(t *testing.T) {
	workspaceID := uuid.New()
	suite := testAgentHarnessSuiteRecord(workspaceID, "Private Rust Tasks")
	harnessA := testAgentHarnessRecord(workspaceID, "Codex Rust")
	harnessB := testAgentHarnessRecord(workspaceID, "Claude Rust")
	harnessB.HarnessKind = AgentHarnessKindClaudeE2B
	harnessA.RepositoryURL = stringPtr("https://github.com/acme/rusty")
	harnessB.RepositoryURL = stringPtr("https://github.com/acme/rusty")
	taskA := testAgentHarnessSuiteTaskRecord(workspaceID, suite.CurrentVersionID, "Hidden borrow checker")
	taskA.ID = uuid.New()
	taskA.TaskPrompt = "Fix the hidden borrow checker bug and open a PR."
	taskA.RepositoryURL = stringPtr("https://github.com/acme/rusty")
	taskA.BaseBranch = stringPtr("trunk")
	taskA.ExecutionConfig = json.RawMessage(`{"timeout_seconds":1200}`)
	taskA.EvaluationConfig = json.RawMessage(`{
		"validators": [{"type":"command","command":"cargo test --all"}],
		"llm_judges": [{"key":"pr_quality"}],
		"privacy": {"redact_replay": true}
	}`)
	taskB := testAgentHarnessSuiteTaskRecord(workspaceID, suite.CurrentVersionID, "Unselected")
	repo := &fakeAgentHarnessRepo{
		organizationID: harnessA.OrganizationID,
		harnessesByID: map[uuid.UUID]repository.AgentHarness{
			harnessA.ID: harnessA,
			harnessB.ID: harnessB,
		},
		suite:      suite,
		suiteTasks: []repository.AgentHarnessSuiteTask{taskA, taskB},
	}
	starter := &fakeAgentHarnessWorkflowStarter{}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo, starter)

	executions, err := manager.StartAgentHarnessSuiteRun(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, suite.ID, StartAgentHarnessSuiteRunInput{
		HarnessIDs: []uuid.UUID{harnessA.ID, harnessB.ID},
		TaskIDs:    []uuid.UUID{taskA.ID},
	})
	if err != nil {
		t.Fatalf("StartAgentHarnessSuiteRun error: %v", err)
	}

	if len(executions) != 2 || len(repo.createdExecutions) != 2 {
		t.Fatalf("executions = %d created = %d, want 2", len(executions), len(repo.createdExecutions))
	}
	for index, created := range repo.createdExecutions {
		var snapshot agentHarnessResponse
		if err := json.Unmarshal(created.HarnessSnapshot, &snapshot); err != nil {
			t.Fatalf("decode harness snapshot %d: %v", index, err)
		}
		if snapshot.TaskPrompt != taskA.TaskPrompt {
			t.Fatalf("snapshot task prompt = %q, want task prompt", snapshot.TaskPrompt)
		}
		if snapshot.RepositoryURL == nil || *snapshot.RepositoryURL != "https://github.com/acme/rusty" {
			t.Fatalf("snapshot repository = %#v, want task repository", snapshot.RepositoryURL)
		}
		if snapshot.BaseBranch == nil || *snapshot.BaseBranch != "trunk" {
			t.Fatalf("snapshot base branch = %#v, want task base branch", snapshot.BaseBranch)
		}
		if string(created.ExecutionConfigSnapshot) != string(taskA.ExecutionConfig) {
			t.Fatalf("execution config = %s, want task config", created.ExecutionConfigSnapshot)
		}
		var eval struct {
			Validators []map[string]any `json:"validators"`
			LLMJudges  []map[string]any `json:"llm_judges"`
			Privacy    struct {
				Redact bool `json:"redact_replay"`
			} `json:"privacy"`
			Suite struct {
				SuiteID        uuid.UUID       `json:"suite_id"`
				SuiteVersionID uuid.UUID       `json:"suite_version_id"`
				TaskID         uuid.UUID       `json:"task_id"`
				TaskSource     string          `json:"task_source"`
				TaskMetadata   json.RawMessage `json:"task_metadata"`
			} `json:"suite"`
			Result map[string]any `json:"result"`
		}
		if err := json.Unmarshal(created.EvaluationConfigSnapshot, &eval); err != nil {
			t.Fatalf("decode evaluation config %d: %v", index, err)
		}
		if len(eval.Validators) != 1 || len(eval.LLMJudges) != 1 || !eval.Privacy.Redact {
			t.Fatalf("eval config did not preserve validators/judges/privacy: %#v", eval)
		}
		if eval.Suite.SuiteID != suite.ID || eval.Suite.SuiteVersionID != suite.CurrentVersionID || eval.Suite.TaskID != taskA.ID {
			t.Fatalf("suite metadata = %#v, want suite/task ids", eval.Suite)
		}
		if eval.Result["kind"] != "private_task_bank" {
			t.Fatalf("result metadata = %#v, want private_task_bank", eval.Result)
		}
	}
	if starter.startedCount != 2 || starter.timeoutSeconds != 1200 {
		t.Fatalf("starter count/timeout = %d/%d, want 2/1200", starter.startedCount, starter.timeoutSeconds)
	}
}

func TestAgentHarnessManagerStartSuiteRunInheritsHarnessConfigWhenTaskConfigOmitted(t *testing.T) {
	workspaceID := uuid.New()
	suite := testAgentHarnessSuiteRecord(workspaceID, "Private Tasks")
	harness := testAgentHarnessRecord(workspaceID, "Codex Rust")
	harness.ExecutionConfig = json.RawMessage(`{"timeout_seconds":600}`)
	harness.EvaluationConfig = json.RawMessage(`{"validators":[{"type":"command","command":"go test ./..."}]}`)
	task := testAgentHarnessSuiteTaskRecord(workspaceID, suite.CurrentVersionID, "No overrides")
	task.ExecutionConfig = json.RawMessage(`{}`)
	task.EvaluationConfig = json.RawMessage(`{}`)
	repo := &fakeAgentHarnessRepo{
		organizationID: harness.OrganizationID,
		harness:        harness,
		suite:          suite,
		suiteTasks:     []repository.AgentHarnessSuiteTask{task},
	}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.StartAgentHarnessSuiteRun(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, suite.ID, StartAgentHarnessSuiteRunInput{
		HarnessIDs: []uuid.UUID{harness.ID},
	})
	if err != nil {
		t.Fatalf("StartAgentHarnessSuiteRun error: %v", err)
	}
	if string(repo.createdExecution.ExecutionConfigSnapshot) != string(harness.ExecutionConfig) {
		t.Fatalf("execution config = %s, want inherited harness config", repo.createdExecution.ExecutionConfigSnapshot)
	}
	var eval struct {
		Validators []map[string]any `json:"validators"`
		Suite      map[string]any   `json:"suite"`
	}
	if err := json.Unmarshal(repo.createdExecution.EvaluationConfigSnapshot, &eval); err != nil {
		t.Fatalf("decode evaluation config: %v", err)
	}
	if len(eval.Validators) != 1 || eval.Suite == nil {
		t.Fatalf("evaluation config = %#v, want inherited validators plus suite metadata", eval)
	}
}

func TestMapAgentHarnessExecutionResponseRedactsPrivateSuiteSnapshots(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	execution := testAgentHarnessExecutionRecord(workspaceID, harnessID)
	execution.HarnessSnapshot = json.RawMessage(`{"id":"` + harnessID.String() + `","task_prompt":"hidden private task","name":"Harness"}`)
	execution.EvaluationConfigSnapshot = json.RawMessage(`{
		"validators": [{"type":"command","command":"cargo test --hidden"}],
		"llm_judges": [{"key":"secret"}],
		"suite": {"suite_id":"` + uuid.NewString() + `","public_prompt":"Fix a Rust test."},
		"result": {"publicity":"private"},
		"privacy": {"redact_replay": true}
	}`)

	response := mapAgentHarnessExecutionResponse(execution)

	if string(response.HarnessSnapshot) == string(execution.HarnessSnapshot) {
		t.Fatal("harness snapshot was not redacted")
	}
	var snapshot map[string]any
	if err := json.Unmarshal(response.HarnessSnapshot, &snapshot); err != nil {
		t.Fatalf("decode redacted snapshot: %v", err)
	}
	if snapshot["task_prompt"] != "Fix a Rust test." {
		t.Fatalf("task_prompt = %#v, want public prompt", snapshot["task_prompt"])
	}
	var evaluation map[string]any
	if err := json.Unmarshal(response.EvaluationConfigSnapshot, &evaluation); err != nil {
		t.Fatalf("decode redacted eval config: %v", err)
	}
	if _, ok := evaluation["validators"]; ok {
		t.Fatalf("redacted evaluation leaked validators: %#v", evaluation)
	}
	if _, ok := evaluation["llm_judges"]; ok {
		t.Fatalf("redacted evaluation leaked llm_judges: %#v", evaluation)
	}
	if evaluation["validator_count"].(float64) != 1 || evaluation["llm_judge_count"].(float64) != 1 {
		t.Fatalf("redacted counts = %#v, want validator/judge counts", evaluation)
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

func TestAgentHarnessSuiteRoutesCreateListAndRun(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	taskID := uuid.New()
	suite := testAgentHarnessSuiteRecord(workspaceID, "Nightly private tasks")
	service := &fakeAgentHarnessService{
		suites:     []repository.AgentHarnessSuite{suite},
		suiteTasks: []repository.AgentHarnessSuiteTask{testAgentHarnessSuiteTaskRecord(workspaceID, suite.CurrentVersionID, "Fix flaky Rust test")},
	}
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), callerContextKey{}, testAgentHarnessCaller(workspaceID))
			ctx = context.WithValue(ctx, workspaceIDContextKey{}, workspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Post("/v1/workspaces/{workspaceID}/agent-harness-suites", createAgentHarnessSuiteHandler(slog.Default(), service))
	router.Get("/v1/workspaces/{workspaceID}/agent-harness-suites", listAgentHarnessSuitesHandler(slog.Default(), service))
	router.Get("/v1/workspaces/{workspaceID}/agent-harness-suites/{suiteID}/tasks", listAgentHarnessSuiteTasksHandler(slog.Default(), service))
	router.Get("/v1/workspaces/{workspaceID}/agent-harness-suites/{suiteID}/rankings", getAgentHarnessSuiteRankingHandler(slog.Default(), service))
	router.Post("/v1/workspaces/{workspaceID}/agent-harness-suites/{suiteID}/runs", startAgentHarnessSuiteRunHandler(slog.Default(), service))

	createReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-suites", bytes.NewBufferString(`{
		"name": "Nightly private tasks",
		"description": "Private benchmark bank",
		"metadata": {"owner": "platform"},
		"tasks": [{
			"title": "Fix flaky Rust test",
			"public_prompt": "Fix a flaky Rust test.",
			"task_prompt": "Fix the hidden flaky Rust test and make a PR.",
			"source_type": "github_issue",
			"source_snapshot": {"repository": "acme/rusty", "number": 44},
			"evaluation_config": {"validators": [{"type": "command", "command": "cargo test --all"}]}
		}]
	}`))
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", createRec.Code, createRec.Body.String())
	}
	if service.createdSuiteInput.Tasks[0].SourceType != "github_issue" {
		t.Fatalf("source type = %q, want github_issue", service.createdSuiteInput.Tasks[0].SourceType)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-suites", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body %s", listRec.Code, listRec.Body.String())
	}
	var listed listAgentHarnessSuitesResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode suites: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != suite.ID {
		t.Fatalf("listed suites = %#v, want suite", listed.Items)
	}

	tasksReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-suites/"+suite.ID.String()+"/tasks", nil)
	tasksRec := httptest.NewRecorder()
	router.ServeHTTP(tasksRec, tasksReq)
	if tasksRec.Code != http.StatusOK {
		t.Fatalf("tasks status = %d, body %s", tasksRec.Code, tasksRec.Body.String())
	}
	var listedTasks listAgentHarnessSuiteTasksResponse
	if err := json.Unmarshal(tasksRec.Body.Bytes(), &listedTasks); err != nil {
		t.Fatalf("decode suite tasks: %v", err)
	}
	if len(listedTasks.Items) != 1 || listedTasks.Items[0].ID == uuid.Nil || listedTasks.Items[0].PublicPrompt == "" {
		t.Fatalf("listed tasks = %#v, want public task with id", listedTasks.Items)
	}

	service.ranking = repository.AgentHarnessSuiteRankingRecord{
		SuiteID:        suite.ID,
		SuiteVersionID: suite.CurrentVersionID,
		SchemaVersion:  "2026-05-06",
		Ranking:        json.RawMessage(`{"schema_version":"2026-05-06","rankings":[{"rank":1}]}`),
		ComputedAt:     time.Now().UTC(),
	}
	rankingReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-suites/"+suite.ID.String()+"/rankings?k=3", nil)
	rankingRec := httptest.NewRecorder()
	router.ServeHTTP(rankingRec, rankingReq)
	if rankingRec.Code != http.StatusOK {
		t.Fatalf("ranking status = %d, body %s", rankingRec.Code, rankingRec.Body.String())
	}
	var ranked getAgentHarnessSuiteRankingResponse
	if err := json.Unmarshal(rankingRec.Body.Bytes(), &ranked); err != nil {
		t.Fatalf("decode ranking: %v", err)
	}
	if !bytes.Contains(ranked.Ranking, []byte(`"rankings"`)) {
		t.Fatalf("ranking payload = %s, want raw ranking document", ranked.Ranking)
	}

	runReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-suites/"+suite.ID.String()+"/runs", bytes.NewBufferString(`{
		"harness_ids": ["`+harnessID.String()+`"],
		"task_ids": ["`+taskID.String()+`"]
	}`))
	runRec := httptest.NewRecorder()
	router.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusCreated {
		t.Fatalf("run status = %d, body %s", runRec.Code, runRec.Body.String())
	}
	if service.startedSuiteID != suite.ID {
		t.Fatalf("started suite = %s, want %s", service.startedSuiteID, suite.ID)
	}
	if len(service.startedSuiteInput.HarnessIDs) != 1 || service.startedSuiteInput.HarnessIDs[0] != harnessID {
		t.Fatalf("started harnesses = %#v, want harness id", service.startedSuiteInput.HarnessIDs)
	}
	if len(service.startedSuiteInput.TaskIDs) != 1 || service.startedSuiteInput.TaskIDs[0] != taskID {
		t.Fatalf("started tasks = %#v, want task id", service.startedSuiteInput.TaskIDs)
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

func TestAgentHarnessExecutionManagerStartRequiresRunPermission(t *testing.T) {
	workspaceID := uuid.New()
	harness := testAgentHarnessRecord(workspaceID, "Codex execution harness")
	repo := &fakeAgentHarnessRepo{organizationID: harness.OrganizationID, harness: harness}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.StartAgentHarnessExecution(context.Background(), testAgentHarnessCallerWithRole(workspaceID, RoleWorkspaceViewer), workspaceID, harness.ID, StartAgentHarnessExecutionInput{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
	if repo.getByIDCalls != 0 {
		t.Fatalf("GetAgentHarnessByID calls = %d, want 0", repo.getByIDCalls)
	}
}

func TestAgentHarnessExecutionManagerStartEnforcesConcurrencyLimit(t *testing.T) {
	workspaceID := uuid.New()
	harness := testAgentHarnessRecord(workspaceID, "Codex execution harness")
	repo := &fakeAgentHarnessRepo{organizationID: harness.OrganizationID, harness: harness, activeCount: 3}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.StartAgentHarnessExecution(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, harness.ID, StartAgentHarnessExecutionInput{})
	var validationErr AgentHarnessValidationError
	if !errors.As(err, &validationErr) || validationErr.Code != "concurrency_limit_exceeded" {
		t.Fatalf("error = %T %v, want concurrency validation", err, err)
	}
}

func TestAgentHarnessExecutionManagerCancelIsIdempotentAndAuthorized(t *testing.T) {
	workspaceID := uuid.New()
	execution := testAgentHarnessExecutionRecord(workspaceID, uuid.New())
	execution.Status = string(repository.AgentHarnessExecutionStatusRunning)
	repo := &fakeAgentHarnessRepo{organizationID: execution.OrganizationID, execution: execution}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	cancelled, err := manager.CancelAgentHarnessExecution(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, execution.ID)
	if err != nil {
		t.Fatalf("CancelAgentHarnessExecution error: %v", err)
	}
	if cancelled.Status != string(repository.AgentHarnessExecutionStatusCancelled) {
		t.Fatalf("status = %q, want cancelled", cancelled.Status)
	}
	if repo.transitionedStatus != repository.AgentHarnessExecutionStatusCancelled {
		t.Fatalf("transitioned status = %q, want cancelled", repo.transitionedStatus)
	}

	execution.Status = string(repository.AgentHarnessExecutionStatusCompleted)
	repo.transitionedStatus = ""
	completedRepo := &fakeAgentHarnessRepo{organizationID: execution.OrganizationID, execution: execution}
	manager = NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), completedRepo)
	got, err := manager.CancelAgentHarnessExecution(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, execution.ID)
	if err != nil {
		t.Fatalf("CancelAgentHarnessExecution terminal error: %v", err)
	}
	if got.Status != string(repository.AgentHarnessExecutionStatusCompleted) || completedRepo.transitionedStatus != "" {
		t.Fatalf("terminal cancel mutated execution: status=%q transition=%q", got.Status, completedRepo.transitionedStatus)
	}
}

func TestAgentHarnessExecutionManagerRetryIsIdempotent(t *testing.T) {
	workspaceID := uuid.New()
	previous := testAgentHarnessExecutionRecord(workspaceID, uuid.New())
	previous.Status = string(repository.AgentHarnessExecutionStatusFailed)
	retry := testAgentHarnessExecutionRecord(workspaceID, previous.AgentHarnessID)
	retry.RetryOfExecutionID = &previous.ID
	retryKey := "retry-1"
	retry.RetryIdempotencyKey = &retryKey
	repo := &fakeAgentHarnessRepo{organizationID: previous.OrganizationID, execution: previous, retryExecution: retry}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	got, err := manager.RetryAgentHarnessExecution(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, previous.ID, RetryAgentHarnessExecutionInput{IdempotencyKey: retryKey})
	if err != nil {
		t.Fatalf("RetryAgentHarnessExecution error: %v", err)
	}
	if got.ID != retry.ID {
		t.Fatalf("retry id = %s, want existing retry %s", got.ID, retry.ID)
	}
	if len(repo.createdExecutions) != 0 {
		t.Fatalf("created executions = %d, want idempotent existing retry", len(repo.createdExecutions))
	}
}

func TestAgentHarnessExecutionManagerRetryRejectsActiveExecution(t *testing.T) {
	workspaceID := uuid.New()
	previous := testAgentHarnessExecutionRecord(workspaceID, uuid.New())
	previous.Status = string(repository.AgentHarnessExecutionStatusRunning)
	repo := &fakeAgentHarnessRepo{organizationID: previous.OrganizationID, execution: previous}
	manager := NewAgentHarnessManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.RetryAgentHarnessExecution(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, previous.ID, RetryAgentHarnessExecutionInput{IdempotencyKey: "retry-1"})
	var validationErr AgentHarnessValidationError
	if !errors.As(err, &validationErr) || validationErr.Code != "execution_not_retryable" {
		t.Fatalf("error = %T %v, want execution_not_retryable", err, err)
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
	runID := uuid.New()
	runAgentID := uuid.New()
	evaluationSpecID := uuid.New()
	execution.RunID = &runID
	execution.RunAgentID = &runAgentID
	execution.EvaluationSpecID = &evaluationSpecID
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
	router.Post("/v1/workspaces/{workspaceID}/agent-harness-executions/{executionID}/cancel", cancelAgentHarnessExecutionHandler(slog.Default(), service))
	router.Post("/v1/workspaces/{workspaceID}/agent-harness-executions/{executionID}/retry", retryAgentHarnessExecutionHandler(slog.Default(), service))

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
	if gotExecution.RunID == nil || *gotExecution.RunID != runID {
		t.Fatalf("run_id = %#v, want %s", gotExecution.RunID, runID)
	}
	if gotExecution.RunAgentID == nil || *gotExecution.RunAgentID != runAgentID {
		t.Fatalf("run_agent_id = %#v, want %s", gotExecution.RunAgentID, runAgentID)
	}
	if gotExecution.EvaluationSpecID == nil || *gotExecution.EvaluationSpecID != evaluationSpecID {
		t.Fatalf("evaluation_spec_id = %#v, want %s", gotExecution.EvaluationSpecID, evaluationSpecID)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-executions/"+execution.ID.String()+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	router.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, body %s", cancelRec.Code, cancelRec.Body.String())
	}
	var cancelled agentHarnessExecutionResponse
	if err := json.Unmarshal(cancelRec.Body.Bytes(), &cancelled); err != nil {
		t.Fatalf("decode cancelled: %v", err)
	}
	if cancelled.Status != string(repository.AgentHarnessExecutionStatusCancelled) {
		t.Fatalf("cancelled status = %q, want cancelled", cancelled.Status)
	}

	retryReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-executions/"+execution.ID.String()+"/retry", bytes.NewBufferString(`{"idempotency_key":"retry-1"}`))
	retryRec := httptest.NewRecorder()
	router.ServeHTTP(retryRec, retryReq)
	if retryRec.Code != http.StatusCreated {
		t.Fatalf("retry status = %d, body %s", retryRec.Code, retryRec.Body.String())
	}
	var retried agentHarnessExecutionResponse
	if err := json.Unmarshal(retryRec.Body.Bytes(), &retried); err != nil {
		t.Fatalf("decode retried: %v", err)
	}
	if retried.RetryOfExecutionID == nil || *retried.RetryOfExecutionID != execution.ID {
		t.Fatalf("retry_of_execution_id = %#v, want %s", retried.RetryOfExecutionID, execution.ID)
	}
}

func TestAgentHarnessExecutionRoutesSerializeFailureStage(t *testing.T) {
	workspaceID := uuid.New()
	harness := testAgentHarnessRecord(workspaceID, "Existing harness")
	execution := testAgentHarnessExecutionRecord(workspaceID, harness.ID)
	execution.Status = string(repository.AgentHarnessExecutionStatusFailed)
	event := repository.AgentHarnessExecutionEvent{
		ID:                      1,
		AgentHarnessExecutionID: execution.ID,
		SequenceNumber:          1,
		EventType:               "setup.command.failed",
		ActorType:               "worker",
		OccurredAt:              time.Now().UTC(),
		Payload:                 json.RawMessage(`{"error":"setup failed"}`),
	}
	service := &fakeAgentHarnessService{
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
	router.Get("/v1/workspaces/{workspaceID}/agent-harness-executions", listAgentHarnessExecutionsHandler(slog.Default(), service))
	router.Get("/v1/workspaces/{workspaceID}/agent-harness-executions/{executionID}", getAgentHarnessExecutionHandler(slog.Default(), service))

	listReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-harness-executions", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body %s", listRec.Code, listRec.Body.String())
	}
	var listed listAgentHarnessExecutionsResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode listed executions: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].FailureStage == nil || *listed.Items[0].FailureStage != "setup" {
		t.Fatalf("listed failure_stage = %#v, want setup", listed.Items)
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
	if gotExecution.FailureStage == nil || *gotExecution.FailureStage != "setup" {
		t.Fatalf("failure_stage = %#v, want setup", gotExecution.FailureStage)
	}
}

func TestAgentHarnessExecutionFailureStage(t *testing.T) {
	tests := []struct {
		name   string
		status string
		events []repository.AgentHarnessExecutionEvent
		want   *string
	}{
		{
			name:   "setup",
			status: string(repository.AgentHarnessExecutionStatusFailed),
			events: []repository.AgentHarnessExecutionEvent{{
				EventType: "setup.command.failed",
			}},
			want: stringPtr("setup"),
		},
		{
			name:   "agent",
			status: string(repository.AgentHarnessExecutionStatusFailed),
			events: []repository.AgentHarnessExecutionEvent{{
				EventType: "codex.exec.failed",
			}},
			want: stringPtr("agent"),
		},
		{
			name:   "validator",
			status: string(repository.AgentHarnessExecutionStatusFailed),
			events: []repository.AgentHarnessExecutionEvent{{
				EventType: "validator.command.failed",
			}},
			want: stringPtr("validator"),
		},
		{
			name:   "non failed execution",
			status: string(repository.AgentHarnessExecutionStatusRunning),
			events: []repository.AgentHarnessExecutionEvent{{
				EventType: "setup.command.failed",
			}},
			want: nil,
		},
		{
			name:   "repository access revoked",
			status: string(repository.AgentHarnessExecutionStatusFailed),
			events: []repository.AgentHarnessExecutionEvent{{
				EventType: "github.repository_access_revoked",
			}},
			want: stringPtr("repository"),
		},
		{
			name:   "infrastructure fallback",
			status: string(repository.AgentHarnessExecutionStatusFailed),
			events: nil,
			want:   stringPtr("infrastructure"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agentHarnessExecutionFailureStage(tt.status, tt.events)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("failure stage = %q, want nil", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("failure stage = %#v, want %q", got, *tt.want)
			}
		})
	}
}

func testAgentHarnessCaller(workspaceID uuid.UUID) Caller {
	return testAgentHarnessCallerWithRole(workspaceID, RoleWorkspaceAdmin)
}

func testAgentHarnessCallerWithRole(workspaceID uuid.UUID, role string) Caller {
	return Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: role},
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

func testAgentHarnessSuiteRecord(workspaceID uuid.UUID, name string) repository.AgentHarnessSuite {
	now := time.Now().UTC()
	return repository.AgentHarnessSuite{
		ID:                   uuid.New(),
		OrganizationID:       uuid.New(),
		WorkspaceID:          workspaceID,
		Name:                 name,
		Slug:                 generateSlug(name),
		Description:          "description",
		Status:               "active",
		CurrentVersionNumber: 1,
		CurrentVersionID:     uuid.New(),
		TaskCount:            1,
		Metadata:             json.RawMessage(`{}`),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func testAgentHarnessSuiteTaskRecord(workspaceID uuid.UUID, versionID uuid.UUID, title string) repository.AgentHarnessSuiteTask {
	now := time.Now().UTC()
	return repository.AgentHarnessSuiteTask{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		WorkspaceID:      workspaceID,
		SuiteVersionID:   versionID,
		Title:            title,
		PublicPrompt:     "Fix a public task.",
		TaskPrompt:       "Fix the hidden task.",
		SourceType:       "manual",
		SourceSnapshot:   json.RawMessage(`{}`),
		ExecutionConfig:  json.RawMessage(`{}`),
		EvaluationConfig: json.RawMessage(`{}`),
		Metadata:         json.RawMessage(`{}`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

type fakeAgentHarnessRepo struct {
	organizationID          uuid.UUID
	created                 repository.CreateAgentHarnessParams
	createdSuite            repository.CreateAgentHarnessSuiteParams
	createdExecution        repository.CreateAgentHarnessExecutionParams
	createdExecutions       []repository.CreateAgentHarnessExecutionParams
	transitionedStatus      repository.AgentHarnessExecutionStatus
	transitionedReason      *string
	harness                 repository.AgentHarness
	harnessesByID           map[uuid.UUID]repository.AgentHarness
	suite                   repository.AgentHarnessSuite
	suites                  []repository.AgentHarnessSuite
	suiteTasks              []repository.AgentHarnessSuiteTask
	ranking                 repository.AgentHarnessSuiteRankingRecord
	execution               repository.AgentHarnessExecution
	retryExecution          repository.AgentHarnessExecution
	executions              []repository.AgentHarnessExecution
	activeCount             int
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
		HarnessKind:            p.HarnessKind,
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

func (f *fakeAgentHarnessRepo) CreateAgentHarnessSuite(_ context.Context, p repository.CreateAgentHarnessSuiteParams) (repository.AgentHarnessSuite, error) {
	f.createdSuite = p
	now := time.Now().UTC()
	return repository.AgentHarnessSuite{
		ID:                   uuid.New(),
		OrganizationID:       p.OrganizationID,
		WorkspaceID:          p.WorkspaceID,
		CreatedByUserID:      p.CreatedByUserID,
		Name:                 p.Name,
		Slug:                 p.Slug,
		Description:          p.Description,
		Status:               "active",
		CurrentVersionNumber: 1,
		CurrentVersionID:     uuid.New(),
		TaskCount:            len(p.Tasks),
		Metadata:             p.Metadata,
		CreatedAt:            now,
		UpdatedAt:            now,
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
	if f.harnessesByID != nil {
		if harness, ok := f.harnessesByID[id]; ok {
			return harness, nil
		}
	}
	if f.harness.ID == id {
		return f.harness, nil
	}
	return repository.AgentHarness{}, repository.ErrAgentHarnessNotFound
}

func (f *fakeAgentHarnessRepo) ListAgentHarnessesByWorkspaceID(context.Context, uuid.UUID) ([]repository.AgentHarness, error) {
	return nil, nil
}

func (f *fakeAgentHarnessRepo) GetAgentHarnessSuiteByID(_ context.Context, id uuid.UUID) (repository.AgentHarnessSuite, error) {
	if f.suite.ID == id {
		return f.suite, nil
	}
	return repository.AgentHarnessSuite{}, repository.ErrAgentHarnessSuiteNotFound
}

func (f *fakeAgentHarnessRepo) ListAgentHarnessSuitesByWorkspaceID(context.Context, uuid.UUID) ([]repository.AgentHarnessSuite, error) {
	return f.suites, nil
}

func (f *fakeAgentHarnessRepo) ListAgentHarnessSuiteTasksByVersionID(context.Context, uuid.UUID) ([]repository.AgentHarnessSuiteTask, error) {
	return f.suiteTasks, nil
}

func (f *fakeAgentHarnessRepo) BuildAgentHarnessSuiteRanking(context.Context, repository.BuildAgentHarnessSuiteRankingParams) (repository.AgentHarnessSuiteRankingRecord, error) {
	return f.ranking, nil
}

func (f *fakeAgentHarnessRepo) CreateAgentHarnessExecution(_ context.Context, p repository.CreateAgentHarnessExecutionParams) (repository.AgentHarnessExecution, error) {
	f.createdExecution = p
	f.createdExecutions = append(f.createdExecutions, p)
	now := time.Now().UTC()
	return repository.AgentHarnessExecution{
		ID:                       uuid.New(),
		OrganizationID:           p.OrganizationID,
		WorkspaceID:              p.WorkspaceID,
		AgentHarnessID:           p.AgentHarnessID,
		RunID:                    p.RunID,
		RunAgentID:               p.RunAgentID,
		EvaluationSpecID:         p.EvaluationSpecID,
		RetryOfExecutionID:       p.RetryOfExecutionID,
		RetryIdempotencyKey:      p.RetryIdempotencyKey,
		CreatedByUserID:          p.CreatedByUserID,
		Status:                   "queued",
		HarnessSnapshot:          p.HarnessSnapshot,
		ExecutionConfigSnapshot:  p.ExecutionConfigSnapshot,
		EvaluationConfigSnapshot: p.EvaluationConfigSnapshot,
		CreatedAt:                now,
		UpdatedAt:                now,
	}, nil
}

func (f *fakeAgentHarnessRepo) SetAgentHarnessExecutionTemporalIDs(_ context.Context, p repository.SetAgentHarnessExecutionTemporalIDsParams) (repository.AgentHarnessExecution, error) {
	f.execution.TemporalWorkflowID = &p.TemporalWorkflowID
	f.execution.TemporalRunID = &p.TemporalRunID
	return f.execution, nil
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

func (f *fakeAgentHarnessRepo) GetAgentHarnessRetryByIdempotencyKey(context.Context, uuid.UUID, uuid.UUID, string) (repository.AgentHarnessExecution, error) {
	if f.retryExecution.ID != uuid.Nil {
		return f.retryExecution, nil
	}
	return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
}

func (f *fakeAgentHarnessRepo) CountActiveAgentHarnessExecutionsByWorkspaceID(context.Context, uuid.UUID) (int, error) {
	return f.activeCount, nil
}

func (f *fakeAgentHarnessRepo) ListAgentHarnessExecutions(context.Context, repository.ListAgentHarnessExecutionsParams) ([]repository.AgentHarnessExecution, error) {
	return f.executions, nil
}

func (f *fakeAgentHarnessRepo) ListAgentHarnessExecutionEvents(context.Context, uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error) {
	return nil, nil
}

type fakeAgentHarnessService struct {
	harnesses               []repository.AgentHarness
	suites                  []repository.AgentHarnessSuite
	suiteTasks              []repository.AgentHarnessSuiteTask
	ranking                 repository.AgentHarnessSuiteRankingRecord
	executions              []repository.AgentHarnessExecution
	events                  []repository.AgentHarnessExecutionEvent
	createdInput            CreateAgentHarnessInput
	createdSuiteInput       CreateAgentHarnessSuiteInput
	startedSuiteID          uuid.UUID
	startedSuiteInput       StartAgentHarnessSuiteRunInput
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

func (f *fakeAgentHarnessService) CreateAgentHarnessSuite(_ context.Context, _ Caller, workspaceID uuid.UUID, input CreateAgentHarnessSuiteInput) (repository.AgentHarnessSuite, error) {
	f.createdSuiteInput = input
	now := time.Now().UTC()
	return repository.AgentHarnessSuite{
		ID:                   uuid.New(),
		OrganizationID:       uuid.New(),
		WorkspaceID:          workspaceID,
		Name:                 input.Name,
		Slug:                 generateSlug(input.Name),
		Description:          input.Description,
		Status:               "active",
		CurrentVersionNumber: 1,
		CurrentVersionID:     uuid.New(),
		TaskCount:            len(input.Tasks),
		Metadata:             input.Metadata,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

func (f *fakeAgentHarnessService) ListAgentHarnessSuites(context.Context, Caller, uuid.UUID) ([]repository.AgentHarnessSuite, error) {
	return f.suites, nil
}

func (f *fakeAgentHarnessService) ListAgentHarnessSuiteTasks(context.Context, Caller, uuid.UUID, uuid.UUID) ([]repository.AgentHarnessSuiteTask, error) {
	return f.suiteTasks, nil
}

func (f *fakeAgentHarnessService) GetAgentHarnessSuiteRanking(context.Context, Caller, uuid.UUID, uuid.UUID, int) (repository.AgentHarnessSuiteRankingRecord, error) {
	return f.ranking, nil
}

func (f *fakeAgentHarnessService) StartAgentHarnessSuiteRun(_ context.Context, _ Caller, workspaceID uuid.UUID, suiteID uuid.UUID, input StartAgentHarnessSuiteRunInput) ([]repository.AgentHarnessExecution, error) {
	f.startedSuiteID = suiteID
	f.startedSuiteInput = input
	executions := make([]repository.AgentHarnessExecution, 0, len(input.HarnessIDs))
	for _, harnessID := range input.HarnessIDs {
		executions = append(executions, testAgentHarnessExecutionRecord(workspaceID, harnessID))
	}
	return executions, nil
}

func (f *fakeAgentHarnessService) StartAgentHarnessExecution(_ context.Context, _ Caller, workspaceID uuid.UUID, harnessID uuid.UUID, input StartAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error) {
	f.startedHarnessID = harnessID
	f.startedInput = input
	return testAgentHarnessExecutionRecord(workspaceID, harnessID), nil
}

func (f *fakeAgentHarnessService) CancelAgentHarnessExecution(_ context.Context, _ Caller, _ uuid.UUID, executionID uuid.UUID) (repository.AgentHarnessExecution, error) {
	for _, execution := range f.executions {
		if execution.ID == executionID {
			execution.Status = string(repository.AgentHarnessExecutionStatusCancelled)
			return execution, nil
		}
	}
	return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
}

func (f *fakeAgentHarnessService) RetryAgentHarnessExecution(_ context.Context, _ Caller, workspaceID uuid.UUID, executionID uuid.UUID, _ RetryAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error) {
	for _, execution := range f.executions {
		if execution.ID == executionID {
			retry := testAgentHarnessExecutionRecord(workspaceID, execution.AgentHarnessID)
			retry.RetryOfExecutionID = &executionID
			return retry, nil
		}
	}
	return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
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
	startedCount   int
	timeoutSeconds int
}

func (f *fakeAgentHarnessWorkflowStarter) StartAgentHarnessExecutionWorkflow(_ context.Context, executionID uuid.UUID, timeoutSeconds int) (AgentHarnessExecutionWorkflowRef, error) {
	f.startedCount++
	f.timeoutSeconds = timeoutSeconds
	if f.err != nil {
		return AgentHarnessExecutionWorkflowRef{}, f.err
	}
	return AgentHarnessExecutionWorkflowRef{WorkflowID: defaultAgentHarnessExecutionWorkflowID(executionID), RunID: "run-id"}, nil
}
