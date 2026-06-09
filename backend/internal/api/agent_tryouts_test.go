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

func TestAgentTryoutOfficeTemplatesAreAvailableAndExecutable(t *testing.T) {
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), newFakeAgentTryoutRepository(uuid.New(), uuid.New()))
	templates, err := manager.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates returned error: %v", err)
	}
	bySlug := make(map[string]AgentTryoutTemplate, len(templates))
	for _, template := range templates {
		bySlug[template.Slug] = template
	}
	for _, slug := range []string{"slide-deck", "spreadsheet-builder", "status-report", "inbox-triage"} {
		template, ok := bySlug[slug]
		if !ok {
			t.Fatalf("office template %q not found", slug)
		}
		if !template.Available || template.UnavailableReason != "" {
			t.Fatalf("%s availability = %v reason=%q, want available", slug, template.Available, template.UnavailableReason)
		}
		// Office templates draft documents only: no shell, no network, no side effects.
		if !bytes.Contains(template.ToolPolicy, []byte(`"file_writer"`)) ||
			!bytes.Contains(template.ToolPolicy, []byte(`"shell":"disabled"`)) ||
			!bytes.Contains(template.ToolPolicy, []byte(`"network":{"mode":"disabled"`)) {
			t.Fatalf("%s tool policy = %s, want file writer with disabled shell and network", slug, template.ToolPolicy)
		}
		if !bytes.Contains(template.Runtime, []byte(`"expected_artifacts"`)) ||
			!bytes.Contains(template.Runtime, []byte(`"validation"`)) {
			t.Fatalf("%s runtime = %s, want expected artifacts and validation", slug, template.Runtime)
		}
		// json_field is the only validator type these templates may use: it is the
		// one the harness evaluation path supports without a dedicated runtime.
		var runtime struct {
			Validation struct {
				Validators []struct {
					Type string `json:"type"`
				} `json:"validators"`
			} `json:"validation"`
		}
		if err := json.Unmarshal(template.Runtime, &runtime); err != nil {
			t.Fatalf("%s runtime is not valid JSON: %v", slug, err)
		}
		if len(runtime.Validation.Validators) == 0 {
			t.Fatalf("%s runtime declares no validators", slug)
		}
		for _, validator := range runtime.Validation.Validators {
			if validator.Type != "json_field" {
				t.Fatalf("%s validator type = %q, want json_field", slug, validator.Type)
			}
		}
		for _, raw := range []json.RawMessage{template.InputSchema, template.EvaluationSpec, template.DefaultModelPolicy} {
			if !json.Valid(raw) {
				t.Fatalf("%s template carries invalid JSON: %s", slug, raw)
			}
		}
	}
}

func TestAgentTryoutManagerRejectsInvalidTemplateInputBeforeCreate(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	for _, tc := range []struct {
		name  string
		input json.RawMessage
	}{
		{name: "missing required field", input: json.RawMessage(`{"audience":"execs"}`)},
		{name: "required null field", input: json.RawMessage(`{"notes":null}`)},
		{name: "optional null field", input: json.RawMessage(`{"notes":"hello","audience":null}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
				TemplateSlug:         "meeting-minutes",
				Input:                tc.input,
				AnonymousFingerprint: "203.0.113.10",
			})
			if !errors.Is(err, ErrInvalidAgentTryoutInput) {
				t.Fatalf("CreateAnonymousTryout error = %v, want ErrInvalidAgentTryoutInput", err)
			}
		})
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

func TestAgentTryoutManagerAllowsAnonymousWithinQuotaAndSpendCaps(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithQuota(AgentTryoutQuotaConfig{
		AnonymousLimit:            1,
		AnonymousWindow:           24 * time.Hour,
		HostedDailySpendCapUSD:    1,
		AnonymousPerRunCostCapUSD: 1,
	})

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"within cap"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	if tryout.CostLimitUSD != 0.25 {
		t.Fatalf("cost limit = %v, want template limit 0.25", tryout.CostLimitUSD)
	}
	if len(repo.tryouts) != 1 {
		t.Fatalf("tryouts created = %d, want 1", len(repo.tryouts))
	}
}

func TestAgentTryoutManagerRejectsAnonymousQuotaExhaustedBeforeCreate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithQuota(AgentTryoutQuotaConfig{
		AnonymousLimit:            1,
		AnonymousWindow:           24 * time.Hour,
		HostedDailySpendCapUSD:    1,
		AnonymousPerRunCostCapUSD: 1,
	})
	input := CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"first"}`),
		AnonymousFingerprint: "203.0.113.10",
	}
	if _, err := manager.CreateAnonymousTryout(ctx, input); err != nil {
		t.Fatalf("first CreateAnonymousTryout returned error: %v", err)
	}
	input.Input = json.RawMessage(`{"notes":"second"}`)
	_, err := manager.CreateAnonymousTryout(ctx, input)
	if !errors.Is(err, ErrAgentTryoutAnonymousQuotaExhausted) {
		t.Fatalf("second CreateAnonymousTryout error = %v, want ErrAgentTryoutAnonymousQuotaExhausted", err)
	}
	if len(repo.tryouts) != 1 {
		t.Fatalf("tryouts created = %d, want still 1 after quota block", len(repo.tryouts))
	}
}

func TestAgentTryoutManagerRejectsQuotaAccountingUnavailableBeforeCreate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*fakeAgentTryoutRepository)
		wantErr error
	}{
		{
			name: "fingerprint count fails",
			setup: func(repo *fakeAgentTryoutRepository) {
				repo.countAnonymousErr = errors.New("count unavailable")
			},
			// A quota-read failure must surface as a quota-specific error, not as
			// the hosted-spend sentinel — the spend sum never ran.
			wantErr: ErrAgentTryoutAnonymousQuotaUnavailable,
		},
		{
			name: "hosted spend sum fails",
			setup: func(repo *fakeAgentTryoutRepository) {
				repo.sumAnonymousErr = errors.New("sum unavailable")
			},
			wantErr: ErrAgentTryoutHostedSpendUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
			tt.setup(repo)
			manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithQuota(AgentTryoutQuotaConfig{
				AnonymousLimit:            1,
				AnonymousWindow:           24 * time.Hour,
				HostedDailySpendCapUSD:    1,
				AnonymousPerRunCostCapUSD: 1,
			})

			_, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
				TemplateSlug:         "meeting-minutes",
				Input:                json.RawMessage(`{"notes":"fail closed"}`),
				AnonymousFingerprint: "203.0.113.10",
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreateAnonymousTryout error = %v, want %v", err, tt.wantErr)
			}
			if len(repo.tryouts) != 0 {
				t.Fatalf("tryouts created = %d, want 0 when quota accounting unavailable", len(repo.tryouts))
			}
		})
	}
}

func TestAgentTryoutManagerRejectsHostedSpendCapExceededBeforeCreate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	repo.hostedSpendUSD = 0.90
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithQuota(AgentTryoutQuotaConfig{
		AnonymousLimit:            1,
		AnonymousWindow:           24 * time.Hour,
		HostedDailySpendCapUSD:    1,
		AnonymousPerRunCostCapUSD: 1,
	})

	_, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"over daily cap"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if !errors.Is(err, ErrAgentTryoutHostedSpendExhausted) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrAgentTryoutHostedSpendExhausted", err)
	}
	if len(repo.tryouts) != 0 {
		t.Fatalf("tryouts created = %d, want 0 when daily cap exhausted", len(repo.tryouts))
	}
}

func TestAgentTryoutManagerRejectsPerRunCostCapExceededBeforeCreate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithQuota(AgentTryoutQuotaConfig{
		AnonymousLimit:            1,
		AnonymousWindow:           24 * time.Hour,
		HostedDailySpendCapUSD:    1,
		AnonymousPerRunCostCapUSD: 0.10,
	})

	_, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"over per run cap"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if !errors.Is(err, ErrAgentTryoutCostCapExceeded) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrAgentTryoutCostCapExceeded", err)
	}
	if len(repo.tryouts) != 0 {
		t.Fatalf("tryouts created = %d, want 0 when per-run cap exceeded", len(repo.tryouts))
	}
}

func TestAgentTryoutManagerDoesNotApplyAnonymousQuotaToWorkspaceTryout(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithQuota(AgentTryoutQuotaConfig{
		AnonymousLimit:            0,
		AnonymousWindow:           24 * time.Hour,
		HostedDailySpendCapUSD:    0,
		AnonymousPerRunCostCapUSD: 0,
	})

	tryout, err := manager.CreateWorkspaceTryout(ctx, Caller{
		UserID: userID,
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, CreateWorkspaceAgentTryoutInput{
		WorkspaceID:  workspaceID,
		TemplateSlug: "meeting-minutes",
		Input:        json.RawMessage(`{"notes":"workspace tryout"}`),
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceTryout returned error: %v", err)
	}
	if tryout.WorkspaceID == nil || *tryout.WorkspaceID != workspaceID {
		t.Fatalf("workspace id = %v, want %s", tryout.WorkspaceID, workspaceID)
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

	// A freshly created tryout is redaction_status=pending and must not be
	// shareable until its evidence has cleared redaction.
	if _, err := manager.CreatePrivateShare(ctx, caller, tryout.ID); !errors.Is(err, ErrAgentTryoutRedactionNotReady) {
		t.Fatalf("CreatePrivateShare while pending error = %v, want ErrAgentTryoutRedactionNotReady", err)
	}

	// Once redaction passes the share can be created.
	shareable := repo.tryouts[tryout.ID]
	shareable.RedactionStatus = repository.AgentTryoutRedactionPassed
	repo.tryouts[tryout.ID] = shareable

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
	if strings.Contains(rr.Body.String(), ErrAgentTryoutTemplateUnavailable.Error()) ||
		!strings.Contains(rr.Body.String(), `"message":"structured data validator runtime is not enabled yet"`) {
		t.Fatalf("body = %s, want product-facing reason without sentinel prefix", rr.Body.String())
	}
}

func TestCreateAnonymousAgentTryoutHandlerMapsQuotaAndSpendErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "anonymous quota exhausted",
			err:        ErrAgentTryoutAnonymousQuotaExhausted,
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "anonymous_quota_exhausted",
		},
		{
			name:       "anonymous quota unavailable",
			err:        ErrAgentTryoutAnonymousQuotaUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "anonymous_quota_unavailable",
		},
		{
			name:       "hosted spend unavailable",
			err:        ErrAgentTryoutHostedSpendUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "hosted_spend_unavailable",
		},
		{
			name:       "hosted spend exhausted",
			err:        ErrAgentTryoutHostedSpendExhausted,
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "hosted_spend_exhausted",
		},
		{
			name:       "tryout cost cap exceeded",
			err:        ErrAgentTryoutCostCapExceeded,
			wantStatus: http.StatusBadRequest,
			wantCode:   "tryout_cost_cap_exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeAgentTryoutService{createAnonymousErr: tt.err}
			handler := createAnonymousAgentTryoutHandler(slog.Default(), service)
			req := httptest.NewRequest(http.MethodPost, "/v1/agent-tryouts", strings.NewReader(`{"template_slug":"meeting-minutes","input":{"notes":"quota"}}`))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tt.wantCode) {
				t.Fatalf("body = %s, want code %q", rr.Body.String(), tt.wantCode)
			}
		})
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
	countAnonymousErr  error
	sumAnonymousErr    error
	hostedSpendUSD     float64
	runEvents           []repository.RunEvent
	runEventsErr        error
	createdConversation repository.CreateVibeEvalConversationParams
	createdDraft        repository.CreateVibeEvalDraftParams
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

func (r *fakeAgentTryoutRepository) ListRunEventsByRunIDAfter(_ context.Context, runID uuid.UUID, afterID int64, limit int32) ([]repository.RunEvent, error) {
	if r.runEventsErr != nil {
		return nil, r.runEventsErr
	}
	out := make([]repository.RunEvent, 0, len(r.runEvents))
	for _, event := range r.runEvents {
		if event.RunID != runID || event.ID <= afterID {
			continue
		}
		out = append(out, event)
		if int32(len(out)) == limit {
			break
		}
	}
	return out, nil
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
		ParentTryoutID:           params.ParentTryoutID,
		ExpiresAt:                params.ExpiresAt,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	r.tryouts[tryout.ID] = tryout
	return tryout, nil
}

func (r *fakeAgentTryoutRepository) CountAnonymousAgentTryoutsByFingerprint(_ context.Context, fingerprintHash string, since time.Time) (int64, error) {
	if r.countAnonymousErr != nil {
		return 0, r.countAnonymousErr
	}
	var count int64
	for _, tryout := range r.tryouts {
		if tryout.WorkspaceID == nil &&
			tryout.OrganizationID == nil &&
			tryout.AnonymousFingerprintHash != nil &&
			*tryout.AnonymousFingerprintHash == fingerprintHash &&
			!tryout.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (r *fakeAgentTryoutRepository) SumAnonymousAgentTryoutCostLimitUSD(_ context.Context, windowStart, windowEnd time.Time) (float64, error) {
	if r.sumAnonymousErr != nil {
		return 0, r.sumAnonymousErr
	}
	total := r.hostedSpendUSD
	for _, tryout := range r.tryouts {
		if tryout.WorkspaceID == nil &&
			tryout.OrganizationID == nil &&
			tryout.AnonymousFingerprintHash != nil &&
			!tryout.CreatedAt.Before(windowStart) &&
			tryout.CreatedAt.Before(windowEnd) {
			total += tryout.CostLimitUSD
		}
	}
	return total, nil
}

func (r *fakeAgentTryoutRepository) WithinAnonymousAgentTryoutQuotaLock(_ context.Context, fn func(repository.AnonymousAgentTryoutQuotaTx) error) error {
	return fn(r)
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

func (r *fakeAgentTryoutRepository) GetActivePublicShareLinkByKey(_ context.Context, key string) (repository.PublicShareLink, error) {
	if r.share.Key == key && r.share.IsActive {
		return r.share, nil
	}
	return repository.PublicShareLink{}, repository.ErrPublicShareLinkNotFound
}

func (r *fakeAgentTryoutRepository) CreateVibeEvalConversation(_ context.Context, params repository.CreateVibeEvalConversationParams) (repository.VibeEvalConversation, error) {
	r.createdConversation = params
	return repository.VibeEvalConversation{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		WorkspaceID:    params.WorkspaceID,
		Title:          params.Title,
		Phase:          params.Phase,
		Status:         params.Status,
	}, nil
}

func (r *fakeAgentTryoutRepository) CreateVibeEvalDraft(_ context.Context, params repository.CreateVibeEvalDraftParams) (repository.VibeEvalDraft, error) {
	r.createdDraft = params
	return repository.VibeEvalDraft{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		WorkspaceID:    params.WorkspaceID,
		ConversationID: params.ConversationID,
		DraftKind:      params.DraftKind,
		Content:        params.Content,
	}, nil
}

type fakeAgentTryoutService struct {
	tryout               repository.AgentTryout
	templates            []AgentTryoutTemplate
	createAnonymousErr   error
	createAnonymousInput CreateAnonymousAgentTryoutInput
	listLimit            int32
	listOffset           int32
	getWorkspaceCalls    int
	eventsResult         AgentTryoutEventsResult
	eventsErr            error
	eventsCursor         TryoutEventsCursor
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

func (s *fakeAgentTryoutService) GetPublicTryoutEvents(_ context.Context, _ uuid.UUID, cursor TryoutEventsCursor) (AgentTryoutEventsResult, error) {
	s.eventsCursor = cursor
	if s.eventsErr != nil {
		return AgentTryoutEventsResult{}, s.eventsErr
	}
	return s.eventsResult, nil
}

func (s *fakeAgentTryoutService) GetSharedTryoutEvents(_ context.Context, _ string, cursor TryoutEventsCursor) (AgentTryoutEventsResult, error) {
	s.eventsCursor = cursor
	if s.eventsErr != nil {
		return AgentTryoutEventsResult{}, s.eventsErr
	}
	return s.eventsResult, nil
}

func (s *fakeAgentTryoutService) GetWorkspaceTryoutEvents(_ context.Context, _ Caller, _ uuid.UUID, cursor TryoutEventsCursor) (AgentTryoutEventsResult, error) {
	s.eventsCursor = cursor
	if s.eventsErr != nil {
		return AgentTryoutEventsResult{}, s.eventsErr
	}
	return s.eventsResult, nil
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

func (s *fakeAgentTryoutService) RerunWorkspaceTryout(context.Context, Caller, RerunAgentTryoutInput) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, nil
}

func (s *fakeAgentTryoutService) CompareWorkspaceTryouts(context.Context, Caller, CompareAgentTryoutsInput) (AgentTryoutCompareResult, error) {
	return AgentTryoutCompareResult{}, nil
}

func (s *fakeAgentTryoutService) PromoteTryoutToEval(context.Context, Caller, PromoteAgentTryoutInput) (AgentTryoutPromotionResult, error) {
	return AgentTryoutPromotionResult{}, nil
}
