package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	maxAgentTryoutRequestBytes = 512 * 1024
	defaultAgentTryoutTTL      = 24 * time.Hour
)

var (
	ErrAgentTryoutTemplateNotFound = errors.New("agent tryout template not found")
	ErrInvalidAgentTryoutInput     = errors.New("invalid agent tryout input")
)

type AgentTryoutRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	GetAgentHarnessByWorkspaceSlug(ctx context.Context, workspaceID uuid.UUID, slug string) (repository.AgentHarness, error)
	CreateAgentHarness(ctx context.Context, p repository.CreateAgentHarnessParams) (repository.AgentHarness, error)
	CreateAgentHarnessExecution(ctx context.Context, p repository.CreateAgentHarnessExecutionParams) (repository.AgentHarnessExecution, error)
	SetAgentHarnessExecutionTemporalIDs(ctx context.Context, p repository.SetAgentHarnessExecutionTemporalIDsParams) (repository.AgentHarnessExecution, error)
	TransitionAgentHarnessExecutionStatus(ctx context.Context, p repository.TransitionAgentHarnessExecutionStatusParams) (repository.AgentHarnessExecution, error)
	GetAgentHarnessExecutionByRunID(ctx context.Context, runID uuid.UUID) (repository.AgentHarnessExecution, error)
	CreateAgentTryout(ctx context.Context, params repository.CreateAgentTryoutParams) (repository.AgentTryout, error)
	GetAgentTryoutByID(ctx context.Context, id uuid.UUID) (repository.AgentTryout, error)
	ListAgentTryoutsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error)
	LinkAgentTryoutRunIfUnset(ctx context.Context, params repository.LinkAgentTryoutRunParams) (repository.AgentTryout, error)
	UpdateAgentTryoutStatus(ctx context.Context, params repository.UpdateAgentTryoutStatusParams) (repository.AgentTryout, error)
	ClaimAgentTryout(ctx context.Context, params repository.ClaimAgentTryoutParams) (repository.AgentTryout, error)
	CreatePublicShareLink(ctx context.Context, params repository.CreatePublicShareLinkParams) (repository.PublicShareLink, error)
}

type AgentTryoutService interface {
	ListTemplates(ctx context.Context) ([]AgentTryoutTemplate, error)
	CreateAnonymousTryout(ctx context.Context, input CreateAnonymousAgentTryoutInput) (repository.AgentTryout, error)
	CreateWorkspaceTryout(ctx context.Context, caller Caller, input CreateWorkspaceAgentTryoutInput) (repository.AgentTryout, error)
	GetPublicTryout(ctx context.Context, id uuid.UUID) (repository.AgentTryout, error)
	GetWorkspaceTryout(ctx context.Context, caller Caller, id uuid.UUID) (repository.AgentTryout, error)
	ListWorkspaceTryouts(ctx context.Context, caller Caller, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error)
	ClaimTryout(ctx context.Context, caller Caller, input ClaimAgentTryoutInput) (repository.AgentTryout, error)
	CreatePrivateShare(ctx context.Context, caller Caller, id uuid.UUID) (CreateAgentTryoutShareResult, error)
}

type AgentTryoutTemplate struct {
	Slug               string          `json:"slug"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	InputSchema        json.RawMessage `json:"input_schema"`
	ToolPolicy         json.RawMessage `json:"tool_policy"`
	EvaluationSpec     json.RawMessage `json:"evaluation_spec"`
	DefaultModelPolicy json.RawMessage `json:"default_model_policy"`
	AnonymousEnabled   bool            `json:"anonymous_enabled"`
	MaxInputBytes      int64           `json:"max_input_bytes"`
	MaxDurationSeconds int32           `json:"max_duration_seconds"`
	MaxCostUSD         float64         `json:"max_cost_usd"`
}

type CreateAnonymousAgentTryoutInput struct {
	TemplateSlug         string
	Input                json.RawMessage
	AnonymousFingerprint string
	Now                  time.Time
}

type CreateWorkspaceAgentTryoutInput struct {
	WorkspaceID  uuid.UUID
	TemplateSlug string
	Input        json.RawMessage
	Now          time.Time
}

type ClaimAgentTryoutInput struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Now         time.Time
}

type CreateAgentTryoutShareResult struct {
	Share repository.PublicShareLink
	Token string
}

type AgentTryoutExecutionConfig struct {
	PublicWorkspaceID      *uuid.UUID
	HarnessKind            string
	E2BTemplateID          string
	OpenAIAPIKeySecretName string
	PublicCreatedByUserID  *uuid.UUID
	ConcurrencyLimit       int
}

type AgentTryoutManager struct {
	authorizer WorkspaceAuthorizer
	repo       AgentTryoutRepository
	now        func() time.Time
	templates  map[string]AgentTryoutTemplate
	execution  *agentTryoutExecutionDispatcher
}

func NewAgentTryoutManager(authorizer WorkspaceAuthorizer, repo AgentTryoutRepository) *AgentTryoutManager {
	templates := builtinAgentTryoutTemplates()
	bySlug := make(map[string]AgentTryoutTemplate, len(templates))
	for _, template := range templates {
		bySlug[template.Slug] = template
	}
	return &AgentTryoutManager{
		authorizer: authorizer,
		repo:       repo,
		now:        time.Now,
		templates:  bySlug,
	}
}

func (m *AgentTryoutManager) WithExecution(starter AgentHarnessExecutionWorkflowStarter, config AgentTryoutExecutionConfig) *AgentTryoutManager {
	if starter == nil {
		return m
	}
	m.execution = &agentTryoutExecutionDispatcher{
		repo:    m.repo,
		starter: starter,
		config:  normalizeAgentTryoutExecutionConfig(config),
	}
	return m
}

func (m *AgentTryoutManager) ListTemplates(context.Context) ([]AgentTryoutTemplate, error) {
	templates := builtinAgentTryoutTemplates()
	return templates, nil
}

func (m *AgentTryoutManager) CreateAnonymousTryout(ctx context.Context, input CreateAnonymousAgentTryoutInput) (repository.AgentTryout, error) {
	template, err := m.lookupTemplate(input.TemplateSlug)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if !template.AnonymousEnabled {
		return repository.AgentTryout{}, fmt.Errorf("%w: template does not allow anonymous tryouts", ErrInvalidAgentTryoutInput)
	}
	if err := validateAgentTryoutInput(template, input.Input); err != nil {
		return repository.AgentTryout{}, err
	}
	now := input.Now
	if now.IsZero() {
		now = m.now()
	}
	expiresAt := now.UTC().Add(defaultAgentTryoutTTL)
	fingerprintHash := hashAnonymousFingerprint(input.AnonymousFingerprint)
	tryout, err := m.repo.CreateAgentTryout(ctx, repository.CreateAgentTryoutParams{
		TemplateSlug:             template.Slug,
		Status:                   repository.AgentTryoutStatusQueued,
		InputSnapshot:            input.Input,
		TemplateSnapshot:         templateSnapshot(template),
		ToolPolicySnapshot:       template.ToolPolicy,
		EvaluationSpecSnapshot:   template.EvaluationSpec,
		SelectedModelPolicy:      template.DefaultModelPolicy,
		Summary:                  json.RawMessage(`{}`),
		RedactionStatus:          repository.AgentTryoutRedactionPending,
		CostLimitUSD:             template.MaxCostUSD,
		MaxDurationSeconds:       template.MaxDurationSeconds,
		AnonymousFingerprintHash: &fingerprintHash,
		ExpiresAt:                &expiresAt,
	})
	if err != nil {
		return repository.AgentTryout{}, err
	}
	return m.dispatchCreatedTryout(ctx, tryout, template)
}

func (m *AgentTryoutManager) CreateWorkspaceTryout(ctx context.Context, caller Caller, input CreateWorkspaceAgentTryoutInput) (repository.AgentTryout, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.AgentTryout{}, err
	}
	template, err := m.lookupTemplate(input.TemplateSlug)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if err := validateAgentTryoutInput(template, input.Input); err != nil {
		return repository.AgentTryout{}, err
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	callerID := caller.UserID
	tryout, err := m.repo.CreateAgentTryout(ctx, repository.CreateAgentTryoutParams{
		OrganizationID:         &orgID,
		WorkspaceID:            &input.WorkspaceID,
		TemplateSlug:           template.Slug,
		Status:                 repository.AgentTryoutStatusQueued,
		InputSnapshot:          input.Input,
		TemplateSnapshot:       templateSnapshot(template),
		ToolPolicySnapshot:     template.ToolPolicy,
		EvaluationSpecSnapshot: template.EvaluationSpec,
		SelectedModelPolicy:    template.DefaultModelPolicy,
		Summary:                json.RawMessage(`{}`),
		RedactionStatus:        repository.AgentTryoutRedactionPending,
		CostLimitUSD:           template.MaxCostUSD,
		MaxDurationSeconds:     template.MaxDurationSeconds,
		CreatedByUserID:        &callerID,
	})
	if err != nil {
		return repository.AgentTryout{}, err
	}
	return m.dispatchCreatedTryout(ctx, tryout, template)
}

func (m *AgentTryoutManager) GetPublicTryout(ctx context.Context, id uuid.UUID) (repository.AgentTryout, error) {
	tryout, err := m.repo.GetAgentTryoutByID(ctx, id)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if tryout.WorkspaceID != nil {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	return m.refreshTryoutFromExecution(ctx, tryout), nil
}

func (m *AgentTryoutManager) GetWorkspaceTryout(ctx context.Context, caller Caller, id uuid.UUID) (repository.AgentTryout, error) {
	tryout, err := m.repo.GetAgentTryoutByID(ctx, id)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if tryout.WorkspaceID == nil {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, *tryout.WorkspaceID); err != nil {
		return repository.AgentTryout{}, err
	}
	return m.refreshTryoutFromExecution(ctx, tryout), nil
}

func (m *AgentTryoutManager) ListWorkspaceTryouts(ctx context.Context, caller Caller, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	tryouts, err := m.repo.ListAgentTryoutsByWorkspaceID(ctx, workspaceID, limit, offset)
	if err != nil {
		return nil, err
	}
	for i := range tryouts {
		tryouts[i] = m.refreshTryoutFromExecution(ctx, tryouts[i])
	}
	return tryouts, nil
}

func (m *AgentTryoutManager) dispatchCreatedTryout(ctx context.Context, tryout repository.AgentTryout, template AgentTryoutTemplate) (repository.AgentTryout, error) {
	if m.execution == nil {
		return tryout, nil
	}
	return m.execution.dispatch(ctx, tryout, template)
}

func (m *AgentTryoutManager) refreshTryoutFromExecution(ctx context.Context, tryout repository.AgentTryout) repository.AgentTryout {
	if m.execution == nil || tryout.RunID == nil {
		return tryout
	}
	refreshed, err := m.execution.refresh(ctx, tryout)
	if err != nil {
		return tryout
	}
	return refreshed
}

type agentTryoutExecutionDispatcher struct {
	repo    AgentTryoutRepository
	starter AgentHarnessExecutionWorkflowStarter
	config  AgentTryoutExecutionConfig
}

func normalizeAgentTryoutExecutionConfig(config AgentTryoutExecutionConfig) AgentTryoutExecutionConfig {
	if strings.TrimSpace(config.HarnessKind) == "" {
		config.HarnessKind = domain.AgentHarnessKindCodexE2B
	}
	if strings.TrimSpace(config.E2BTemplateID) == "" {
		config.E2BTemplateID = defaultCodexE2BTemplate
	}
	if strings.TrimSpace(config.OpenAIAPIKeySecretName) == "" {
		config.OpenAIAPIKeySecretName = "OPENAI_API_KEY"
	}
	if config.ConcurrencyLimit <= 0 {
		config.ConcurrencyLimit = 3
	}
	return config
}

func (d *agentTryoutExecutionDispatcher) dispatch(ctx context.Context, tryout repository.AgentTryout, template AgentTryoutTemplate) (repository.AgentTryout, error) {
	if tryout.RunID != nil {
		return d.refresh(ctx, tryout)
	}
	workspaceID, createdBy, ok := d.executionScope(tryout)
	if !ok {
		return d.failTryout(ctx, tryout, "execution_not_configured", "Agent tryouts are not configured for public execution yet.")
	}
	orgID, err := d.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	harness, err := d.getOrCreateTemplateHarness(ctx, orgID, workspaceID, createdBy, template)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	executionConfig := d.executionConfig(template)
	evaluationConfig := agentTryoutEvaluationConfig(template, tryout)
	harnessSnapshot, err := d.harnessSnapshot(harness, template, tryout, executionConfig, evaluationConfig)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	execution, err := d.repo.CreateAgentHarnessExecution(ctx, repository.CreateAgentHarnessExecutionParams{
		OrganizationID:           orgID,
		WorkspaceID:              workspaceID,
		AgentHarnessID:           harness.ID,
		CreatedByUserID:          createdBy,
		HarnessSnapshot:          harnessSnapshot,
		ExecutionConfigSnapshot:  executionConfig,
		EvaluationConfigSnapshot: evaluationConfig,
		ConcurrencyLimit:         d.config.ConcurrencyLimit,
	})
	if err != nil {
		return repository.AgentTryout{}, err
	}
	runID := derefUUID(execution.RunID)
	linked, err := d.repo.LinkAgentTryoutRunIfUnset(ctx, repository.LinkAgentTryoutRunParams{
		ID:      tryout.ID,
		RunID:   runID,
		Status:  repository.AgentTryoutStatusRunning,
		Summary: agentTryoutDispatchSummary(execution),
	})
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if linked.RunID == nil || *linked.RunID != runID {
		return d.refresh(ctx, linked)
	}
	if err := d.startWorkflow(ctx, execution); err != nil {
		reason := err.Error()
		_, _ = d.repo.TransitionAgentHarnessExecutionStatus(ctx, repository.TransitionAgentHarnessExecutionStatusParams{
			ExecutionID:     execution.ID,
			ToStatus:        repository.AgentHarnessExecutionStatusFailed,
			Reason:          &reason,
			ChangedByUserID: createdBy,
		})
		return d.failTryout(ctx, linked, "execution_start_failed", "We could not start this tryout. Please try again.")
	}
	return d.refresh(ctx, linked)
}

func (d *agentTryoutExecutionDispatcher) executionScope(tryout repository.AgentTryout) (uuid.UUID, *uuid.UUID, bool) {
	if tryout.WorkspaceID != nil {
		return *tryout.WorkspaceID, tryout.CreatedByUserID, true
	}
	if d.config.PublicWorkspaceID == nil {
		return uuid.Nil, nil, false
	}
	return *d.config.PublicWorkspaceID, d.config.PublicCreatedByUserID, true
}

func (d *agentTryoutExecutionDispatcher) getOrCreateTemplateHarness(ctx context.Context, orgID uuid.UUID, workspaceID uuid.UUID, createdBy *uuid.UUID, template AgentTryoutTemplate) (repository.AgentHarness, error) {
	slug := agentTryoutHarnessSlug(template)
	harness, err := d.repo.GetAgentHarnessByWorkspaceSlug(ctx, workspaceID, slug)
	if err == nil {
		return harness, nil
	}
	if !errors.Is(err, repository.ErrAgentHarnessNotFound) {
		return repository.AgentHarness{}, err
	}
	harness, err = d.repo.CreateAgentHarness(ctx, repository.CreateAgentHarnessParams{
		OrganizationID:         orgID,
		WorkspaceID:            workspaceID,
		CreatedByUserID:        createdBy,
		Name:                   "Agent Tryout: " + template.Name,
		Slug:                   slug,
		Description:            "Internal harness used by AgentClash agent tryouts.",
		HarnessKind:            d.config.HarnessKind,
		TaskPrompt:             "AgentClash tryout template dispatcher.",
		CodexTemplate:          d.config.E2BTemplateID,
		AuthMode:               AgentHarnessAuthModeAPIKeySecret,
		OpenAIAPIKeySecretName: optionalHarnessString(d.config.OpenAIAPIKeySecretName),
		ExecutionConfig:        d.executionConfig(template),
		EvaluationConfig:       agentTryoutEvaluationConfig(template, repository.AgentTryout{TemplateSlug: template.Slug}),
	})
	if errors.Is(err, repository.ErrAgentHarnessSlugConflict) {
		return d.repo.GetAgentHarnessByWorkspaceSlug(ctx, workspaceID, slug)
	}
	return harness, err
}

func (d *agentTryoutExecutionDispatcher) startWorkflow(ctx context.Context, execution repository.AgentHarnessExecution) error {
	ref, err := d.starter.StartAgentHarnessExecutionWorkflow(ctx, execution.ID, agentHarnessExecutionTimeoutSeconds(execution.ExecutionConfigSnapshot))
	if err != nil {
		return err
	}
	if strings.TrimSpace(ref.WorkflowID) == "" {
		ref.WorkflowID = defaultAgentHarnessExecutionWorkflowID(execution.ID)
	}
	_, err = d.repo.SetAgentHarnessExecutionTemporalIDs(ctx, repository.SetAgentHarnessExecutionTemporalIDsParams{
		ExecutionID:        execution.ID,
		TemporalWorkflowID: ref.WorkflowID,
		TemporalRunID:      ref.RunID,
	})
	if err != nil {
		if controller, ok := d.starter.(AgentHarnessExecutionWorkflowController); ok {
			_ = controller.CancelAgentHarnessExecutionWorkflow(ctx, ref.WorkflowID, ref.RunID)
		}
	}
	return err
}

func (d *agentTryoutExecutionDispatcher) refresh(ctx context.Context, tryout repository.AgentTryout) (repository.AgentTryout, error) {
	if tryout.RunID == nil {
		return tryout, nil
	}
	execution, err := d.repo.GetAgentHarnessExecutionByRunID(ctx, *tryout.RunID)
	if err != nil {
		if errors.Is(err, repository.ErrAgentHarnessExecutionNotFound) {
			return tryout, nil
		}
		return repository.AgentTryout{}, err
	}
	status, redaction := agentTryoutStatusFromHarnessExecution(execution)
	return d.repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              tryout.ID,
		Status:          status,
		Summary:         agentTryoutExecutionSummary(execution),
		LatencyMS:       agentTryoutLatencyMS(execution),
		RedactionStatus: redaction,
	})
}

func (d *agentTryoutExecutionDispatcher) failTryout(ctx context.Context, tryout repository.AgentTryout, code string, message string) (repository.AgentTryout, error) {
	redaction := repository.AgentTryoutRedactionNotRequired
	return d.repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              tryout.ID,
		Status:          repository.AgentTryoutStatusFailed,
		Summary:         safeAgentTryoutFailureSummary(code, message),
		RedactionStatus: &redaction,
	})
}

func (d *agentTryoutExecutionDispatcher) executionConfig(template AgentTryoutTemplate) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"timeout_seconds": template.MaxDurationSeconds,
		"agent_tryout": map[string]any{
			"template_slug": template.Slug,
		},
	})
	return payload
}

func (d *agentTryoutExecutionDispatcher) harnessSnapshot(harness repository.AgentHarness, template AgentTryoutTemplate, tryout repository.AgentTryout, executionConfig json.RawMessage, evaluationConfig json.RawMessage) (json.RawMessage, error) {
	payload := map[string]any{
		"id":                         harness.ID,
		"workspace_id":               harness.WorkspaceID,
		"organization_id":            harness.OrganizationID,
		"harness_kind":               d.config.HarnessKind,
		"task_prompt":                agentTryoutTaskPrompt(template, tryout.InputSnapshot),
		"codex_template":             d.config.E2BTemplateID,
		"auth_mode":                  AgentHarnessAuthModeAPIKeySecret,
		"openai_api_key_secret_name": d.config.OpenAIAPIKeySecretName,
		"execution_config":           json.RawMessage(executionConfig),
		"evaluation_config":          json.RawMessage(evaluationConfig),
	}
	return json.Marshal(payload)
}

func agentTryoutHarnessSlug(template AgentTryoutTemplate) string {
	return "agent-tryout-" + generateSlug(template.Slug)
}

func agentTryoutTaskPrompt(template AgentTryoutTemplate, input json.RawMessage) string {
	return strings.Join([]string{
		"You are running an AgentClash tryout template.",
		"Template: " + template.Name + " (" + template.Slug + ")",
		"Goal: " + template.Description,
		"Use only the available sandbox tools and produce a concise, inspectable result for the user.",
		"User input JSON:",
		"```json",
		string(input),
		"```",
	}, "\n")
}

func agentTryoutEvaluationConfig(template AgentTryoutTemplate, tryout repository.AgentTryout) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"template_evaluation_spec": json.RawMessage(template.EvaluationSpec),
		"privacy": map[string]any{
			"redact_replay":    true,
			"redact_artifacts": true,
			"retention_days":   1,
		},
		"result": map[string]any{
			"kind":          "agent_tryout",
			"template_slug": template.Slug,
			"tryout_id":     tryout.ID,
		},
	})
	return payload
}

func agentTryoutDispatchSummary(execution repository.AgentHarnessExecution) json.RawMessage {
	return mustAgentTryoutJSON(map[string]any{
		"code":          "execution_started",
		"message":       "Tryout execution started.",
		"execution_id":  execution.ID,
		"run_id":        execution.RunID,
		"run_agent_id":  execution.RunAgentID,
		"status_source": "agent_harness_execution",
	})
}

func agentTryoutExecutionSummary(execution repository.AgentHarnessExecution) json.RawMessage {
	code := "execution_" + strings.TrimSpace(execution.Status)
	message := "Tryout execution is " + strings.TrimSpace(execution.Status) + "."
	if repository.AgentHarnessExecutionStatus(execution.Status) == repository.AgentHarnessExecutionStatusFailed {
		code = "execution_failed"
		message = "The tryout did not complete successfully. Inspect the run evidence for details."
	}
	return mustAgentTryoutJSON(map[string]any{
		"code":          code,
		"message":       message,
		"execution_id":  execution.ID,
		"run_id":        execution.RunID,
		"run_agent_id":  execution.RunAgentID,
		"status_source": "agent_harness_execution",
	})
}

func safeAgentTryoutFailureSummary(code string, message string) json.RawMessage {
	return mustAgentTryoutJSON(map[string]any{"code": code, "message": message})
}

func agentTryoutStatusFromHarnessExecution(execution repository.AgentHarnessExecution) (repository.AgentTryoutStatus, *repository.AgentTryoutRedactionStatus) {
	switch repository.AgentHarnessExecutionStatus(execution.Status) {
	case repository.AgentHarnessExecutionStatusCompleted:
		redaction := repository.AgentTryoutRedactionPassed
		return repository.AgentTryoutStatusCompleted, &redaction
	case repository.AgentHarnessExecutionStatusFailed:
		redaction := repository.AgentTryoutRedactionNotRequired
		return repository.AgentTryoutStatusFailed, &redaction
	case repository.AgentHarnessExecutionStatusCancelled:
		redaction := repository.AgentTryoutRedactionNotRequired
		return repository.AgentTryoutStatusCancelled, &redaction
	default:
		return repository.AgentTryoutStatusRunning, nil
	}
}

func agentTryoutLatencyMS(execution repository.AgentHarnessExecution) *int64 {
	if execution.StartedAt == nil || execution.CompletedAt == nil {
		return nil
	}
	value := execution.CompletedAt.Sub(*execution.StartedAt).Milliseconds()
	if value < 0 {
		value = 0
	}
	return &value
}

func derefUUID(value *uuid.UUID) uuid.UUID {
	if value == nil {
		return uuid.Nil
	}
	return *value
}

func mustAgentTryoutJSON(value any) json.RawMessage {
	payload, _ := json.Marshal(value)
	return payload
}

func (m *AgentTryoutManager) ClaimTryout(ctx context.Context, caller Caller, input ClaimAgentTryoutInput) (repository.AgentTryout, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.AgentTryout{}, err
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	now := input.Now
	if now.IsZero() {
		now = m.now()
	}
	return m.repo.ClaimAgentTryout(ctx, repository.ClaimAgentTryoutParams{
		ID:              input.ID,
		OrganizationID:  orgID,
		WorkspaceID:     input.WorkspaceID,
		ClaimedByUserID: caller.UserID,
		ClaimedAt:       now,
	})
}

func (m *AgentTryoutManager) CreatePrivateShare(ctx context.Context, caller Caller, id uuid.UUID) (CreateAgentTryoutShareResult, error) {
	tryout, err := m.GetWorkspaceTryout(ctx, caller, id)
	if err != nil {
		return CreateAgentTryoutShareResult{}, err
	}
	if tryout.OrganizationID == nil || tryout.WorkspaceID == nil {
		return CreateAgentTryoutShareResult{}, repository.ErrAgentTryoutNotFound
	}
	key, err := newShareKey()
	if err != nil {
		return CreateAgentTryoutShareResult{}, err
	}
	callerID := caller.UserID
	share, err := m.repo.CreatePublicShareLink(ctx, repository.CreatePublicShareLinkParams{
		Key:             key,
		OrganizationID:  *tryout.OrganizationID,
		WorkspaceID:     *tryout.WorkspaceID,
		ResourceType:    repository.PublicShareResourceAgentTryout,
		ResourceID:      tryout.ID,
		CreatedByUserID: &callerID,
		SearchIndexing:  false,
	})
	if err != nil {
		return CreateAgentTryoutShareResult{}, err
	}
	return CreateAgentTryoutShareResult{Share: share, Token: share.Key}, nil
}

func (m *AgentTryoutManager) lookupTemplate(slug string) (AgentTryoutTemplate, error) {
	template, ok := m.templates[strings.TrimSpace(slug)]
	if !ok {
		return AgentTryoutTemplate{}, ErrAgentTryoutTemplateNotFound
	}
	return template, nil
}

type createAgentTryoutRequest struct {
	TemplateSlug string          `json:"template_slug"`
	Input        json.RawMessage `json:"input"`
}

type claimAgentTryoutRequest struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
}

type listAgentTryoutTemplatesResponse struct {
	Items []AgentTryoutTemplate `json:"items"`
}

type listAgentTryoutsResponse struct {
	Items []agentTryoutResponse `json:"items"`
}

type agentTryoutResponse struct {
	ID                     uuid.UUID                             `json:"id"`
	OrganizationID         *uuid.UUID                            `json:"organization_id,omitempty"`
	WorkspaceID            *uuid.UUID                            `json:"workspace_id,omitempty"`
	TemplateSlug           string                                `json:"template_slug"`
	Status                 repository.AgentTryoutStatus          `json:"status"`
	InputSnapshot          json.RawMessage                       `json:"input_snapshot"`
	TemplateSnapshot       json.RawMessage                       `json:"template_snapshot"`
	ToolPolicySnapshot     json.RawMessage                       `json:"tool_policy_snapshot"`
	EvaluationSpecSnapshot json.RawMessage                       `json:"evaluation_spec_snapshot"`
	SelectedModelPolicy    json.RawMessage                       `json:"selected_model_policy"`
	Summary                json.RawMessage                       `json:"summary"`
	RedactionStatus        repository.AgentTryoutRedactionStatus `json:"redaction_status"`
	RunID                  *uuid.UUID                            `json:"run_id,omitempty"`
	CostLimitUSD           float64                               `json:"cost_limit_usd"`
	ActualCostUSD          *float64                              `json:"actual_cost_usd,omitempty"`
	LatencyMS              *int64                                `json:"latency_ms,omitempty"`
	MaxDurationSeconds     int32                                 `json:"max_duration_seconds"`
	CreatedByUserID        *uuid.UUID                            `json:"created_by_user_id,omitempty"`
	ClaimedByUserID        *uuid.UUID                            `json:"claimed_by_user_id,omitempty"`
	ClaimedAt              *time.Time                            `json:"claimed_at,omitempty"`
	ExpiresAt              *time.Time                            `json:"expires_at,omitempty"`
	CreatedAt              time.Time                             `json:"created_at"`
	UpdatedAt              time.Time                             `json:"updated_at"`
}

type publicAgentTryoutResponse struct {
	ID                     uuid.UUID                             `json:"id"`
	TemplateSlug           string                                `json:"template_slug"`
	Status                 repository.AgentTryoutStatus          `json:"status"`
	InputSnapshot          json.RawMessage                       `json:"input_snapshot"`
	TemplateSnapshot       json.RawMessage                       `json:"template_snapshot"`
	ToolPolicySnapshot     json.RawMessage                       `json:"tool_policy_snapshot"`
	EvaluationSpecSnapshot json.RawMessage                       `json:"evaluation_spec_snapshot"`
	SelectedModelPolicy    json.RawMessage                       `json:"selected_model_policy"`
	Summary                json.RawMessage                       `json:"summary"`
	RedactionStatus        repository.AgentTryoutRedactionStatus `json:"redaction_status"`
	RunID                  *uuid.UUID                            `json:"run_id,omitempty"`
	CostLimitUSD           float64                               `json:"cost_limit_usd"`
	ActualCostUSD          *float64                              `json:"actual_cost_usd,omitempty"`
	LatencyMS              *int64                                `json:"latency_ms,omitempty"`
	MaxDurationSeconds     int32                                 `json:"max_duration_seconds"`
	CreatedAt              time.Time                             `json:"created_at"`
	UpdatedAt              time.Time                             `json:"updated_at"`
}

type agentTryoutShareResponse struct {
	Share publicShareLinkResponse `json:"share"`
	Token string                  `json:"token"`
}

func registerPublicAgentTryoutRoutes(router chi.Router, logger *slog.Logger, service AgentTryoutService) {
	router.Get("/agent-tryout-templates", listAgentTryoutTemplatesHandler(logger, service))
	router.Post("/agent-tryouts", createAnonymousAgentTryoutHandler(logger, service))
	router.Get("/agent-tryouts/{tryoutID}", getPublicAgentTryoutHandler(logger, service))
}

func registerProtectedAgentTryoutRoutes(router chi.Router, logger *slog.Logger, service AgentTryoutService) {
	router.Post("/workspaces/{workspaceID}/agent-tryouts", createWorkspaceAgentTryoutHandler(logger, service))
	router.Get("/workspaces/{workspaceID}/agent-tryouts", listWorkspaceAgentTryoutsHandler(logger, service))
	router.Get("/workspaces/{workspaceID}/agent-tryouts/{tryoutID}", getWorkspaceAgentTryoutHandler(logger, service))
	router.Post("/agent-tryouts/{tryoutID}/claim", claimAgentTryoutHandler(logger, service))
	router.Post("/agent-tryouts/{tryoutID}/share", createAgentTryoutShareHandler(logger, service))
}

func listAgentTryoutTemplatesHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := service.ListTemplates(r.Context())
		if err != nil {
			logger.Error("list agent tryout templates failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, listAgentTryoutTemplatesResponse{Items: items})
	}
}

func createAnonymousAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createAgentTryoutRequest
		if err := decodeAgentTryoutJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		tryout, err := service.CreateAnonymousTryout(r.Context(), CreateAnonymousAgentTryoutInput{
			TemplateSlug:         req.TemplateSlug,
			Input:                req.Input,
			AnonymousFingerprint: anonymousFingerprintFromRequest(r),
		})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapPublicAgentTryoutResponse(tryout))
	}
}

func getPublicAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		tryout, err := service.GetPublicTryout(r.Context(), id)
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, mapPublicAgentTryoutResponse(tryout))
	}
}

func createWorkspaceAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace_id must be a UUID")
			return
		}
		var req createAgentTryoutRequest
		if err := decodeAgentTryoutJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		tryout, err := service.CreateWorkspaceTryout(r.Context(), caller, CreateWorkspaceAgentTryoutInput{
			WorkspaceID:  workspaceID,
			TemplateSlug: req.TemplateSlug,
			Input:        req.Input,
		})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapAgentTryoutResponse(tryout))
	}
}

func listWorkspaceAgentTryoutsHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace_id must be a UUID")
			return
		}
		limit, offset := parseAgentTryoutPagination(r)
		items, err := service.ListWorkspaceTryouts(r.Context(), caller, workspaceID, limit, offset)
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		response := listAgentTryoutsResponse{Items: make([]agentTryoutResponse, 0, len(items))}
		for _, item := range items {
			response.Items = append(response.Items, mapAgentTryoutResponse(item))
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func getWorkspaceAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace_id must be a UUID")
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		tryout, err := service.GetWorkspaceTryout(r.Context(), caller, id)
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		if tryout.WorkspaceID == nil || *tryout.WorkspaceID != workspaceID {
			writeError(w, http.StatusNotFound, "agent_tryout_not_found", "agent tryout not found")
			return
		}
		writeJSON(w, http.StatusOK, mapAgentTryoutResponse(tryout))
	}
}

func parseAgentTryoutPagination(r *http.Request) (int32, int32) {
	limit := int32(50)
	offset := int32(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 32); err == nil {
			limit = int32(value)
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 32); err == nil {
			offset = int32(value)
		}
	}
	return limit, offset
}

func claimAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		var req claimAgentTryoutRequest
		if err := decodeAgentTryoutJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		tryout, err := service.ClaimTryout(r.Context(), caller, ClaimAgentTryoutInput{ID: id, WorkspaceID: req.WorkspaceID})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAgentTryoutResponse(tryout))
	}
}

func createAgentTryoutShareHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		result, err := service.CreatePrivateShare(r.Context(), caller, id)
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, agentTryoutShareResponse{
			Share: mapPublicShareLink(result.Share, ""),
			Token: result.Token,
		})
	}
}

func validateAgentTryoutInput(template AgentTryoutTemplate, raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("%w: input is required", ErrInvalidAgentTryoutInput)
	}
	if int64(len(raw)) > template.MaxInputBytes {
		return fmt.Errorf("%w: input exceeds %d bytes", ErrInvalidAgentTryoutInput, template.MaxInputBytes)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return fmt.Errorf("%w: input must be a JSON object", ErrInvalidAgentTryoutInput)
	}
	if object == nil {
		return fmt.Errorf("%w: input must be a JSON object", ErrInvalidAgentTryoutInput)
	}
	return nil
}

func decodeAgentTryoutJSON(r *http.Request, dest any) error {
	defer r.Body.Close()
	reader := http.MaxBytesReader(nil, r.Body, maxAgentTryoutRequestBytes)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		if errors.As(err, new(*http.MaxBytesError)) {
			return fmt.Errorf("request body exceeds %d bytes", maxAgentTryoutRequestBytes)
		}
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain a single JSON object")
		}
		return err
	}
	return nil
}

func writeAgentTryoutError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, ErrAgentTryoutTemplateNotFound):
		writeError(w, http.StatusNotFound, "template_not_found", "agent tryout template not found")
	case errors.Is(err, repository.ErrAgentTryoutNotFound):
		writeError(w, http.StatusNotFound, "agent_tryout_not_found", "agent tryout not found")
	case errors.Is(err, repository.ErrAgentTryoutAlreadyClaimed):
		writeError(w, http.StatusConflict, "agent_tryout_already_claimed", "agent tryout is already claimed")
	case errors.Is(err, ErrInvalidAgentTryoutInput):
		writeError(w, http.StatusBadRequest, "invalid_agent_tryout_input", err.Error())
	case errors.Is(err, ErrForbidden):
		writeAuthzError(w, err)
	default:
		logger.Error("agent tryout request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func mapAgentTryoutResponse(tryout repository.AgentTryout) agentTryoutResponse {
	return agentTryoutResponse{
		ID:                     tryout.ID,
		OrganizationID:         tryout.OrganizationID,
		WorkspaceID:            tryout.WorkspaceID,
		TemplateSlug:           tryout.TemplateSlug,
		Status:                 tryout.Status,
		InputSnapshot:          tryout.InputSnapshot,
		TemplateSnapshot:       tryout.TemplateSnapshot,
		ToolPolicySnapshot:     tryout.ToolPolicySnapshot,
		EvaluationSpecSnapshot: tryout.EvaluationSpecSnapshot,
		SelectedModelPolicy:    tryout.SelectedModelPolicy,
		Summary:                tryout.Summary,
		RedactionStatus:        tryout.RedactionStatus,
		RunID:                  tryout.RunID,
		CostLimitUSD:           tryout.CostLimitUSD,
		ActualCostUSD:          tryout.ActualCostUSD,
		LatencyMS:              tryout.LatencyMS,
		MaxDurationSeconds:     tryout.MaxDurationSeconds,
		CreatedByUserID:        tryout.CreatedByUserID,
		ClaimedByUserID:        tryout.ClaimedByUserID,
		ClaimedAt:              tryout.ClaimedAt,
		ExpiresAt:              tryout.ExpiresAt,
		CreatedAt:              tryout.CreatedAt,
		UpdatedAt:              tryout.UpdatedAt,
	}
}

func mapPublicAgentTryoutResponse(tryout repository.AgentTryout) publicAgentTryoutResponse {
	return publicAgentTryoutResponse{
		ID:                     tryout.ID,
		TemplateSlug:           tryout.TemplateSlug,
		Status:                 tryout.Status,
		InputSnapshot:          tryout.InputSnapshot,
		TemplateSnapshot:       tryout.TemplateSnapshot,
		ToolPolicySnapshot:     tryout.ToolPolicySnapshot,
		EvaluationSpecSnapshot: tryout.EvaluationSpecSnapshot,
		SelectedModelPolicy:    tryout.SelectedModelPolicy,
		Summary:                tryout.Summary,
		RedactionStatus:        tryout.RedactionStatus,
		RunID:                  tryout.RunID,
		CostLimitUSD:           tryout.CostLimitUSD,
		ActualCostUSD:          tryout.ActualCostUSD,
		LatencyMS:              tryout.LatencyMS,
		MaxDurationSeconds:     tryout.MaxDurationSeconds,
		CreatedAt:              tryout.CreatedAt,
		UpdatedAt:              tryout.UpdatedAt,
	}
}

func templateSnapshot(template AgentTryoutTemplate) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"slug":                 template.Slug,
		"name":                 template.Name,
		"description":          template.Description,
		"anonymous_enabled":    template.AnonymousEnabled,
		"max_input_bytes":      template.MaxInputBytes,
		"max_duration_seconds": template.MaxDurationSeconds,
		"max_cost_usd":         template.MaxCostUSD,
	})
	return payload
}

func anonymousFingerprintFromRequest(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
		}
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func hashAnonymousFingerprint(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func builtinAgentTryoutTemplates() []AgentTryoutTemplate {
	return []AgentTryoutTemplate{
		{
			Slug:               "meeting-minutes",
			Name:               "Meeting Minutes to Action Plan",
			Description:        "Turn notes or a transcript into minutes, decisions, risks, and action items.",
			InputSchema:        json.RawMessage(`{"type":"object","required":["notes"],"properties":{"notes":{"type":"string"},"audience":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"network":"disabled","external_side_effects":false}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"has_action_items","type":"jsonpath","path":"$.action_items"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 120,
			MaxCostUSD:         0.25,
		},
		{
			Slug:               "structured-data",
			Name:               "Extract Structured Data",
			Description:        "Extract rows from messy text into JSON or CSV and validate the shape.",
			InputSchema:        json.RawMessage(`{"type":"object","required":["text"],"properties":{"text":{"type":"string"},"schema":{"type":"object"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["schema_validator","file_writer"],"network":"disabled","external_side_effects":false}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"valid_json","type":"json_schema"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 120,
			MaxCostUSD:         0.25,
		},
		{
			Slug:               "tiny-bugfix",
			Name:               "Fix a Tiny Bug",
			Description:        "Run an agent against a small fixture, inspect the diff, and see whether tests pass.",
			InputSchema:        json.RawMessage(`{"type":"object","required":["task"],"properties":{"task":{"type":"string"},"fixture":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["sandbox_shell","file_editor"],"network":"disabled","external_side_effects":false}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"tests_pass","type":"command_exit_code"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   false,
			MaxInputBytes:      32 * 1024,
			MaxDurationSeconds: 300,
			MaxCostUSD:         0.75,
		},
	}
}
