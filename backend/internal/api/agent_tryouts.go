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
	"reflect"
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
	ErrAgentTryoutTemplateNotFound          = errors.New("agent tryout template not found")
	ErrAgentTryoutTemplateUnavailable       = errors.New("agent tryout template unavailable")
	ErrInvalidAgentTryoutInput              = errors.New("invalid agent tryout input")
	ErrAgentTryoutAnonymousQuotaExhausted   = errors.New("agent tryout anonymous quota exhausted")
	ErrAgentTryoutAnonymousQuotaUnavailable = errors.New("agent tryout anonymous quota unavailable")
	ErrAgentTryoutHostedSpendUnavailable    = errors.New("agent tryout hosted spend unavailable")
	ErrAgentTryoutHostedSpendExhausted      = errors.New("agent tryout hosted spend exhausted")
	ErrAgentTryoutCostCapExceeded           = errors.New("agent tryout cost cap exceeded")
	ErrAgentTryoutRedactionNotReady         = errors.New("agent tryout redaction not ready")

	// Conversion-flow errors (#947: rerun / compare / promote-to-eval).
	ErrAgentTryoutSignInRequired             = errors.New("agent tryout sign-in required")
	ErrAgentTryoutModelPolicyInvalid         = errors.New("agent tryout model policy invalid")
	ErrAgentTryoutModelUnavailable           = errors.New("agent tryout model unavailable")
	ErrAgentTryoutRerunProviderKeyRequired   = errors.New("agent tryout rerun provider key required")
	ErrAgentTryoutRerunInsufficientCredits   = errors.New("agent tryout rerun insufficient credits")
	ErrAgentTryoutCompareCardinality         = errors.New("agent tryout compare requires 2-4 tryouts")
	ErrAgentTryoutPromotionTargetUnsupported = errors.New("agent tryout promotion target unsupported")
	ErrAgentTryoutNotPromotable              = errors.New("agent tryout is not completed")
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
	CountAnonymousAgentTryoutsByFingerprint(ctx context.Context, fingerprintHash string, since time.Time) (int64, error)
	SumAnonymousAgentTryoutCostLimitUSD(ctx context.Context, windowStart, windowEnd time.Time) (float64, error)
	WithinAnonymousAgentTryoutQuotaLock(ctx context.Context, fn func(repository.AnonymousAgentTryoutQuotaTx) error) error
	GetAgentTryoutByID(ctx context.Context, id uuid.UUID) (repository.AgentTryout, error)
	ListRunEventsByRunIDAfter(ctx context.Context, runID uuid.UUID, afterID int64, limit int32) ([]repository.RunEvent, error)
	ListAgentTryoutEventsAfter(ctx context.Context, tryoutID uuid.UUID, afterID int64, limit int32) ([]repository.AgentTryoutEvent, error)
	AppendAgentTryoutTurn(ctx context.Context, params repository.AppendAgentTryoutTurnParams) (repository.AgentTryoutTurn, error)
	ListArtifactsByRunID(ctx context.Context, runID uuid.UUID) ([]repository.Artifact, error)
	ListAgentTryoutsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error)
	LinkAgentTryoutRunIfUnset(ctx context.Context, params repository.LinkAgentTryoutRunParams) (repository.AgentTryout, error)
	UpdateAgentTryoutStatus(ctx context.Context, params repository.UpdateAgentTryoutStatusParams) (repository.AgentTryout, error)
	ClaimAgentTryout(ctx context.Context, params repository.ClaimAgentTryoutParams) (repository.AgentTryout, error)
	CreatePublicShareLink(ctx context.Context, params repository.CreatePublicShareLinkParams) (repository.PublicShareLink, error)
	GetActivePublicShareLinkByKey(ctx context.Context, key string) (repository.PublicShareLink, error)
	CreateVibeEvalConversation(ctx context.Context, params repository.CreateVibeEvalConversationParams) (repository.VibeEvalConversation, error)
	CreateVibeEvalDraft(ctx context.Context, params repository.CreateVibeEvalDraftParams) (repository.VibeEvalDraft, error)
}

type AgentTryoutService interface {
	ListTemplates(ctx context.Context) ([]AgentTryoutTemplate, error)
	CreateAnonymousTryout(ctx context.Context, input CreateAnonymousAgentTryoutInput) (repository.AgentTryout, error)
	SubmitAnonymousTryoutTurn(ctx context.Context, id uuid.UUID, input SubmitAgentTryoutTurnInput) error
	CreateWorkspaceTryout(ctx context.Context, caller Caller, input CreateWorkspaceAgentTryoutInput) (repository.AgentTryout, error)
	GetPublicTryout(ctx context.Context, id uuid.UUID) (repository.AgentTryout, error)
	GetWorkspaceTryout(ctx context.Context, caller Caller, id uuid.UUID) (repository.AgentTryout, error)
	GetPublicTryoutEvents(ctx context.Context, id uuid.UUID, cursor TryoutEventsCursor) (AgentTryoutEventsResult, error)
	GetSharedTryoutEvents(ctx context.Context, token string, cursor TryoutEventsCursor) (AgentTryoutEventsResult, error)
	GetWorkspaceTryoutEvents(ctx context.Context, caller Caller, id uuid.UUID, cursor TryoutEventsCursor) (AgentTryoutEventsResult, error)
	ListWorkspaceTryouts(ctx context.Context, caller Caller, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error)
	ClaimTryout(ctx context.Context, caller Caller, input ClaimAgentTryoutInput) (repository.AgentTryout, error)
	CreatePrivateShare(ctx context.Context, caller Caller, id uuid.UUID) (CreateAgentTryoutShareResult, error)
	RerunWorkspaceTryout(ctx context.Context, caller Caller, input RerunAgentTryoutInput) (repository.AgentTryout, error)
	CompareWorkspaceTryouts(ctx context.Context, caller Caller, input CompareAgentTryoutsInput) (AgentTryoutCompareResult, error)
	PromoteTryoutToEval(ctx context.Context, caller Caller, input PromoteAgentTryoutInput) (AgentTryoutPromotionResult, error)
	ListWorkspaceTryoutArtifacts(ctx context.Context, caller Caller, tryoutID uuid.UUID, baseURL string) ([]AgentTryoutArtifact, error)
}

type AgentTryoutTemplate struct {
	Slug               string          `json:"slug"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Available          bool            `json:"available"`
	UnavailableReason  string          `json:"unavailable_reason,omitempty"`
	InputSchema        json.RawMessage `json:"input_schema"`
	ToolPolicy         json.RawMessage `json:"tool_policy"`
	Runtime            json.RawMessage `json:"runtime"`
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
	SelectedHarnessKind  string
	AnonymousFingerprint string
	Now                  time.Time
}

// SubmitAgentTryoutTurnInput is a user message in an interactive tryout chat.
// End=true closes the session instead of sending a message.
type SubmitAgentTryoutTurnInput struct {
	Message string
	End     bool
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
	HostedProvider         string
	HostedCredentialRef    string
	PublicCreatedByUserID  *uuid.UUID
	ConcurrencyLimit       int
}

type PublicAgentTryoutExecutionWorkflowStarter interface {
	StartPublicAgentTryoutExecutionWorkflow(ctx context.Context, tryoutID uuid.UUID) (AgentHarnessExecutionWorkflowRef, error)
}

type AgentTryoutQuotaConfig struct {
	AnonymousLimit            int
	AnonymousWindow           time.Duration
	HostedDailySpendCapUSD    float64
	AnonymousPerRunCostCapUSD float64
}

type AgentTryoutManager struct {
	authorizer     WorkspaceAuthorizer
	repo           AgentTryoutRepository
	now            func() time.Time
	templates      map[string]AgentTryoutTemplate
	execution      *agentTryoutExecutionDispatcher
	quota          *AgentTryoutQuotaConfig
	rerunGate      AgentTryoutRerunGate
	artifactSigner ArtifactContentSigner
}

// WithArtifactSigner enables signed download URLs on a tryout's captured output
// artifacts. Without it, artifact listing still works but omits download links.
func (m *AgentTryoutManager) WithArtifactSigner(signer ArtifactContentSigner) *AgentTryoutManager {
	m.artifactSigner = signer
	return m
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
	config = normalizeAgentTryoutExecutionConfig(config)
	if m.execution != nil {
		m.execution.starter = starter
		m.execution.config = config
		return m
	}
	m.execution = &agentTryoutExecutionDispatcher{
		repo:    m.repo,
		starter: starter,
		config:  config,
	}
	return m
}

func (m *AgentTryoutManager) WithPublicExecution(starter PublicAgentTryoutExecutionWorkflowStarter, config AgentTryoutExecutionConfig) *AgentTryoutManager {
	if starter == nil {
		return m
	}
	config = normalizeAgentTryoutExecutionConfig(config)
	if m.execution != nil {
		m.execution.publicStarter = starter
		m.execution.config = config
		return m
	}
	m.execution = &agentTryoutExecutionDispatcher{
		repo:          m.repo,
		publicStarter: starter,
		config:        config,
	}
	return m
}

func (m *AgentTryoutManager) WithQuota(config AgentTryoutQuotaConfig) *AgentTryoutManager {
	normalized := normalizeAgentTryoutQuotaConfig(config)
	m.quota = &normalized
	return m
}

func (m *AgentTryoutManager) ListTemplates(context.Context) ([]AgentTryoutTemplate, error) {
	templates := builtinAgentTryoutTemplates()
	return templates, nil
}

// normalizeSelectedHarnessKind validates a user-chosen agent harness. An empty
// choice returns nil so the worker falls back to its configured default.
func normalizeSelectedHarnessKind(kind string) (*string, error) {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return nil, nil
	}
	if !domain.IsPublicSelectableAgentHarnessKind(trimmed) {
		return nil, fmt.Errorf("%w: unsupported agent %q", ErrInvalidAgentTryoutInput, trimmed)
	}
	return &trimmed, nil
}

func (m *AgentTryoutManager) CreateAnonymousTryout(ctx context.Context, input CreateAnonymousAgentTryoutInput) (repository.AgentTryout, error) {
	template, err := m.lookupTemplate(input.TemplateSlug)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if err := ensureAgentTryoutTemplateAvailable(template); err != nil {
		return repository.AgentTryout{}, err
	}
	if !template.AnonymousEnabled {
		return repository.AgentTryout{}, fmt.Errorf("%w: template does not allow anonymous tryouts", ErrInvalidAgentTryoutInput)
	}
	if err := validateAgentTryoutInput(template, input.Input); err != nil {
		return repository.AgentTryout{}, err
	}
	selectedHarness, err := normalizeSelectedHarnessKind(input.SelectedHarnessKind)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	now := input.Now
	if now.IsZero() {
		now = m.now()
	}
	expiresAt := now.UTC().Add(defaultAgentTryoutTTL)
	fingerprintHash := hashAnonymousFingerprint(input.AnonymousFingerprint)
	createParams := repository.CreateAgentTryoutParams{
		TemplateSlug:             template.Slug,
		Status:                   repository.AgentTryoutStatusQueued,
		InputSnapshot:            input.Input,
		TemplateSnapshot:         templateSnapshot(template),
		ToolPolicySnapshot:       template.ToolPolicy,
		EvaluationSpecSnapshot:   template.EvaluationSpec,
		SelectedModelPolicy:      template.DefaultModelPolicy,
		SelectedHarnessKind:      selectedHarness,
		Summary:                  json.RawMessage(`{}`),
		RedactionStatus:          repository.AgentTryoutRedactionPending,
		CostLimitUSD:             template.MaxCostUSD,
		MaxDurationSeconds:       template.MaxDurationSeconds,
		AnonymousFingerprintHash: &fingerprintHash,
		ExpiresAt:                &expiresAt,
	}

	// When quotas are disabled there is nothing to serialize, so skip the lock.
	if m.quota == nil {
		tryout, err := m.repo.CreateAgentTryout(ctx, createParams)
		if err != nil {
			return repository.AgentTryout{}, err
		}
		return m.dispatchCreatedTryout(ctx, tryout, template)
	}

	// Fail fast on the static per-run cost cap before taking the global lock.
	if err := m.enforceAnonymousPerRunCostCap(template); err != nil {
		return repository.AgentTryout{}, err
	}

	// Enforce the per-fingerprint quota and hosted daily-spend cap, then create
	// the tryout, all under a single advisory lock so concurrent requests cannot
	// both pass the gate before either commits (TOCTOU-safe).
	var tryout repository.AgentTryout
	err = m.repo.WithinAnonymousAgentTryoutQuotaLock(ctx, func(qtx repository.AnonymousAgentTryoutQuotaTx) error {
		if err := m.enforceAnonymousQuota(ctx, qtx, template, fingerprintHash, now); err != nil {
			return err
		}
		created, err := qtx.CreateAgentTryout(ctx, createParams)
		if err != nil {
			return err
		}
		tryout = created
		return nil
	})
	if err != nil {
		return repository.AgentTryout{}, err
	}
	return m.dispatchCreatedTryout(ctx, tryout, template)
}

// SubmitAnonymousTryoutTurn appends a user message (or an end signal) to a
// public interactive tryout. The long-running session activity claims pending
// turns and replies via the agent. Only public, still-active tryouts accept it.
func (m *AgentTryoutManager) SubmitAnonymousTryoutTurn(ctx context.Context, id uuid.UUID, input SubmitAgentTryoutTurnInput) error {
	tryout, err := m.repo.GetAgentTryoutByID(ctx, id)
	if err != nil {
		return err
	}
	if tryout.WorkspaceID != nil {
		return repository.ErrAgentTryoutNotFound
	}
	if tryout.Status != repository.AgentTryoutStatusRunning &&
		tryout.Status != repository.AgentTryoutStatusQueued {
		return fmt.Errorf("%w: this tryout session is no longer active", ErrInvalidAgentTryoutInput)
	}
	role := "user"
	message := strings.TrimSpace(input.Message)
	if input.End {
		role = "system_end"
		message = ""
	} else {
		if message == "" {
			return fmt.Errorf("%w: message is required", ErrInvalidAgentTryoutInput)
		}
		if len(message) > maxAgentTryoutTurnBytes {
			return fmt.Errorf("%w: message exceeds %d bytes", ErrInvalidAgentTryoutInput, maxAgentTryoutTurnBytes)
		}
	}
	if _, err := m.repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{
		AgentTryoutID: id,
		Role:          role,
		Message:       message,
	}); err != nil {
		return err
	}
	return nil
}

func (m *AgentTryoutManager) CreateWorkspaceTryout(ctx context.Context, caller Caller, input CreateWorkspaceAgentTryoutInput) (repository.AgentTryout, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.AgentTryout{}, err
	}
	template, err := m.lookupTemplate(input.TemplateSlug)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if err := ensureAgentTryoutTemplateAvailable(template); err != nil {
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

func normalizeAgentTryoutQuotaConfig(config AgentTryoutQuotaConfig) AgentTryoutQuotaConfig {
	if config.AnonymousLimit < 0 {
		config.AnonymousLimit = 0
	}
	if config.AnonymousWindow <= 0 {
		config.AnonymousWindow = 24 * time.Hour
	}
	if config.HostedDailySpendCapUSD < 0 {
		config.HostedDailySpendCapUSD = 0
	}
	if config.AnonymousPerRunCostCapUSD < 0 {
		config.AnonymousPerRunCostCapUSD = 0
	}
	return config
}

// enforceAnonymousPerRunCostCap rejects templates whose per-run cost limit
// exceeds the configured anonymous cap. It is pure (no DB access) so callers can
// run it before acquiring the quota lock.
func (m *AgentTryoutManager) enforceAnonymousPerRunCostCap(template AgentTryoutTemplate) error {
	if template.MaxCostUSD > m.quota.AnonymousPerRunCostCapUSD {
		return fmt.Errorf("%w: template cost limit %.4f exceeds anonymous per-run cap %.4f", ErrAgentTryoutCostCapExceeded, template.MaxCostUSD, m.quota.AnonymousPerRunCostCapUSD)
	}
	return nil
}

// enforceAnonymousQuota checks the per-fingerprint quota and hosted daily-spend
// cap against qtx. Callers must invoke it inside WithinAnonymousAgentTryoutQuotaLock
// (and create the tryout via the same qtx) so the reads and the insert are atomic.
func (m *AgentTryoutManager) enforceAnonymousQuota(ctx context.Context, qtx repository.AnonymousAgentTryoutQuotaTx, template AgentTryoutTemplate, fingerprintHash string, now time.Time) error {
	config := *m.quota
	count, err := qtx.CountAnonymousAgentTryoutsByFingerprint(ctx, fingerprintHash, now.UTC().Add(-config.AnonymousWindow))
	if err != nil {
		return fmt.Errorf("%w: count anonymous tryouts: %v", ErrAgentTryoutAnonymousQuotaUnavailable, err)
	}
	if count >= int64(config.AnonymousLimit) {
		return ErrAgentTryoutAnonymousQuotaExhausted
	}
	// The hosted spend cap is intentionally a fixed UTC calendar-day budget
	// (HostedDailySpendCapUSD resets at UTC midnight), independent of the rolling
	// per-fingerprint AnonymousWindow above. The two windows serve different
	// purposes — abuse throttling per visitor vs. a global daily spend ceiling —
	// so they are not derived from one another.
	windowStart := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(24 * time.Hour)
	spend, err := qtx.SumAnonymousAgentTryoutCostLimitUSD(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("%w: sum hosted spend: %v", ErrAgentTryoutHostedSpendUnavailable, err)
	}
	if spend+template.MaxCostUSD > config.HostedDailySpendCapUSD {
		return ErrAgentTryoutHostedSpendExhausted
	}
	return nil
}

type agentTryoutExecutionDispatcher struct {
	repo          AgentTryoutRepository
	starter       AgentHarnessExecutionWorkflowStarter
	publicStarter PublicAgentTryoutExecutionWorkflowStarter
	config        AgentTryoutExecutionConfig
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
	if strings.TrimSpace(config.HostedProvider) == "" {
		config.HostedProvider = "openai"
	}
	if strings.TrimSpace(config.HostedCredentialRef) == "" {
		config.HostedCredentialRef = "env://OPENAI_API_KEY"
	}
	if config.ConcurrencyLimit <= 0 {
		config.ConcurrencyLimit = 3
	}
	return config
}

func (d *agentTryoutExecutionDispatcher) dispatch(ctx context.Context, tryout repository.AgentTryout, template AgentTryoutTemplate) (repository.AgentTryout, error) {
	if tryout.WorkspaceID == nil {
		return d.dispatchPublic(ctx, tryout)
	}
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
	if execution.RunID == nil || *execution.RunID == uuid.Nil {
		return d.failTryout(ctx, tryout, "execution_link_missing", "We could not link this tryout to an execution run. Please try again.")
	}
	runID := *execution.RunID
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

func (d *agentTryoutExecutionDispatcher) dispatchPublic(ctx context.Context, tryout repository.AgentTryout) (repository.AgentTryout, error) {
	if d.publicStarter == nil {
		return d.failTryout(ctx, tryout, "execution_not_configured", "Hosted public tryouts are not configured yet.")
	}
	ref, err := d.publicStarter.StartPublicAgentTryoutExecutionWorkflow(ctx, tryout.ID)
	if err != nil {
		return d.failTryout(ctx, tryout, "execution_start_failed", "We could not start this hosted public tryout. Please try again.")
	}
	_ = ref
	return tryout, nil
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
	if agentTryoutTerminal(tryout.Status) && !agentTryoutTerminal(status) {
		return tryout, nil
	}
	summary := agentTryoutExecutionSummary(execution)
	latency := agentTryoutLatencyMS(execution)
	if !agentTryoutRefreshChanged(tryout, status, summary, latency, redaction) {
		return tryout, nil
	}
	return d.repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              tryout.ID,
		Status:          status,
		Summary:         summary,
		LatencyMS:       latency,
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
			"runtime":       json.RawMessage(template.Runtime),
			"tool_policy":   json.RawMessage(template.ToolPolicy),
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
		"template_runtime":           json.RawMessage(template.Runtime),
		"tool_policy":                json.RawMessage(template.ToolPolicy),
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
		"template_runtime":         json.RawMessage(template.Runtime),
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

func agentTryoutTerminal(status repository.AgentTryoutStatus) bool {
	switch status {
	case repository.AgentTryoutStatusCompleted,
		repository.AgentTryoutStatusFailed,
		repository.AgentTryoutStatusCancelled:
		return true
	default:
		return false
	}
}

func agentTryoutRefreshChanged(tryout repository.AgentTryout, status repository.AgentTryoutStatus, summary json.RawMessage, latency *int64, redaction *repository.AgentTryoutRedactionStatus) bool {
	if tryout.Status != status {
		return true
	}
	if !agentTryoutJSONEqual(tryout.Summary, summary) {
		return true
	}
	if !agentTryoutInt64PtrEqual(tryout.LatencyMS, latency) {
		return true
	}
	return redaction != nil && tryout.RedactionStatus != *redaction
}

func agentTryoutJSONEqual(a json.RawMessage, b json.RawMessage) bool {
	var left any
	var right any
	if len(a) == 0 || len(b) == 0 {
		return string(a) == string(b)
	}
	if err := json.Unmarshal(a, &left); err != nil {
		return string(a) == string(b)
	}
	if err := json.Unmarshal(b, &right); err != nil {
		return string(a) == string(b)
	}
	return reflect.DeepEqual(left, right)
}

func agentTryoutInt64PtrEqual(a *int64, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
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
	if !tryout.RedactionStatus.ShareReady() {
		return CreateAgentTryoutShareResult{}, ErrAgentTryoutRedactionNotReady
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
	TemplateSlug        string          `json:"template_slug"`
	Input               json.RawMessage `json:"input"`
	SelectedHarnessKind string          `json:"selected_harness_kind,omitempty"`
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
	SelectedHarnessKind    *string                               `json:"selected_harness_kind,omitempty"`
	Summary                json.RawMessage                       `json:"summary"`
	RedactionStatus        repository.AgentTryoutRedactionStatus `json:"redaction_status"`
	RunID                  *uuid.UUID                            `json:"run_id,omitempty"`
	ParentTryoutID         *uuid.UUID                            `json:"parent_tryout_id,omitempty"`
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
	SelectedHarnessKind    *string                               `json:"selected_harness_kind,omitempty"`
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
	router.Post("/agent-tryouts/{tryoutID}/turns", submitAnonymousAgentTryoutTurnHandler(logger, service))
	router.Get("/agent-tryouts/{tryoutID}", getPublicAgentTryoutHandler(logger, service))
	router.Get("/agent-tryouts/{tryoutID}/events", getPublicAgentTryoutEventsHandler(logger, service))
	router.Get("/agent-tryouts/shared/{token}/events", getSharedAgentTryoutEventsHandler(logger, service))
}

func registerProtectedAgentTryoutRoutes(router chi.Router, logger *slog.Logger, service AgentTryoutService) {
	router.Post("/workspaces/{workspaceID}/agent-tryouts", createWorkspaceAgentTryoutHandler(logger, service))
	router.Get("/workspaces/{workspaceID}/agent-tryouts", listWorkspaceAgentTryoutsHandler(logger, service))
	router.Get("/workspaces/{workspaceID}/agent-tryouts/{tryoutID}", getWorkspaceAgentTryoutHandler(logger, service))
	router.Get("/workspaces/{workspaceID}/agent-tryouts/{tryoutID}/events", getWorkspaceAgentTryoutEventsHandler(logger, service))
	router.Get("/workspaces/{workspaceID}/agent-tryouts/{tryoutID}/artifacts", listAgentTryoutArtifactsHandler(logger, service))
	router.Post("/agent-tryouts/{tryoutID}/claim", claimAgentTryoutHandler(logger, service))
	router.Post("/agent-tryouts/{tryoutID}/share", createAgentTryoutShareHandler(logger, service))
	router.Post("/agent-tryouts/{tryoutID}/rerun", rerunAgentTryoutHandler(logger, service))
	router.Post("/agent-tryouts/{tryoutID}/promote-to-eval", promoteAgentTryoutHandler(logger, service))
	router.Post("/workspaces/{workspaceID}/agent-tryouts/compare", compareAgentTryoutsHandler(logger, service))
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
			SelectedHarnessKind:  req.SelectedHarnessKind,
			AnonymousFingerprint: anonymousFingerprintFromRequest(r),
		})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapPublicAgentTryoutResponse(tryout))
	}
}

const maxAgentTryoutTurnBytes = 16 * 1024

type submitAgentTryoutTurnRequest struct {
	Message string `json:"message"`
	End     bool   `json:"end,omitempty"`
}

func submitAnonymousAgentTryoutTurnHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		var req submitAgentTryoutTurnRequest
		if err := decodeAgentTryoutJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if err := service.SubmitAnonymousTryoutTurn(r.Context(), id, SubmitAgentTryoutTurnInput{
			Message: req.Message,
			End:     req.End,
		}); err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "submitted"})
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
	if err := validateAgentTryoutInputSchema(template, object); err != nil {
		return err
	}
	return nil
}

func ensureAgentTryoutTemplateAvailable(template AgentTryoutTemplate) error {
	if template.Available {
		return nil
	}
	reason := strings.TrimSpace(template.UnavailableReason)
	if reason == "" {
		reason = "template runtime is not available"
	}
	return fmt.Errorf("%w: %s", ErrAgentTryoutTemplateUnavailable, reason)
}

type agentTryoutInputSchema struct {
	Type                 string                                    `json:"type"`
	Required             []string                                  `json:"required"`
	Properties           map[string]agentTryoutInputPropertySchema `json:"properties"`
	AdditionalProperties *bool                                     `json:"additionalProperties"`
}

type agentTryoutInputPropertySchema struct {
	Type string `json:"type"`
}

func validateAgentTryoutInputSchema(template AgentTryoutTemplate, object map[string]any) error {
	var schema agentTryoutInputSchema
	if err := json.Unmarshal(template.InputSchema, &schema); err != nil {
		return fmt.Errorf("%w: template input schema is invalid", ErrInvalidAgentTryoutInput)
	}
	if schema.Type != "" && schema.Type != "object" {
		return fmt.Errorf("%w: template input schema must describe an object", ErrInvalidAgentTryoutInput)
	}
	for _, key := range schema.Required {
		value, ok := object[key]
		if !ok || value == nil {
			return fmt.Errorf("%w: missing required field %q", ErrInvalidAgentTryoutInput, key)
		}
	}
	if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
		for key := range object {
			if _, ok := schema.Properties[key]; !ok {
				return fmt.Errorf("%w: unsupported field %q", ErrInvalidAgentTryoutInput, key)
			}
		}
	}
	for key, property := range schema.Properties {
		value, ok := object[key]
		if !ok || strings.TrimSpace(property.Type) == "" {
			continue
		}
		if value == nil {
			return fmt.Errorf("%w: field %q must be %s", ErrInvalidAgentTryoutInput, key, property.Type)
		}
		if !agentTryoutInputValueMatchesType(value, property.Type) {
			return fmt.Errorf("%w: field %q must be %s", ErrInvalidAgentTryoutInput, key, property.Type)
		}
	}
	return nil
}

func agentTryoutInputValueMatchesType(value any, schemaType string) bool {
	switch schemaType {
	case "string":
		_, ok := value.(string)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		number, ok := value.(float64)
		return ok && number == float64(int64(number))
	default:
		return true
	}
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
	case errors.Is(err, ErrAgentTryoutTemplateUnavailable):
		writeError(w, http.StatusConflict, "template_unavailable", agentTryoutTemplateUnavailableMessage(err))
	case errors.Is(err, ErrAgentTryoutAnonymousQuotaExhausted):
		writeError(w, http.StatusTooManyRequests, "anonymous_quota_exhausted", "Free tryout limit reached. Sign in to save and rerun tryouts, or try again later.")
	case errors.Is(err, ErrAgentTryoutAnonymousQuotaUnavailable):
		writeError(w, http.StatusServiceUnavailable, "anonymous_quota_unavailable", "Free tryout quota accounting is unavailable. Please try again later.")
	case errors.Is(err, ErrAgentTryoutHostedSpendUnavailable):
		writeError(w, http.StatusServiceUnavailable, "hosted_spend_unavailable", "Hosted tryout spend accounting is unavailable. Please try again later.")
	case errors.Is(err, ErrAgentTryoutHostedSpendExhausted):
		writeError(w, http.StatusTooManyRequests, "hosted_spend_exhausted", "Hosted free tryouts are temporarily unavailable. Please try again later.")
	case errors.Is(err, ErrAgentTryoutCostCapExceeded):
		writeError(w, http.StatusBadRequest, "tryout_cost_cap_exceeded", "This template exceeds the hosted free tryout cost cap.")
	case errors.Is(err, repository.ErrAgentTryoutNotFound):
		writeError(w, http.StatusNotFound, "agent_tryout_not_found", "agent tryout not found")
	case errors.Is(err, repository.ErrAgentTryoutAlreadyClaimed):
		writeError(w, http.StatusConflict, "agent_tryout_already_claimed", "agent tryout is already claimed")
	case errors.Is(err, ErrAgentTryoutRedactionNotReady):
		writeError(w, http.StatusConflict, "agent_tryout_redaction_not_ready", "This tryout's evidence is still being redacted for safe sharing. Try again once it has finished.")
	case errors.Is(err, ErrAgentTryoutSignInRequired):
		writeError(w, http.StatusUnauthorized, "sign_in_required", "Sign in to rerun, compare, or promote tryouts.")
	case errors.Is(err, ErrAgentTryoutModelPolicyInvalid):
		writeError(w, http.StatusBadRequest, "invalid_model_policy", err.Error())
	case errors.Is(err, ErrAgentTryoutModelUnavailable):
		writeError(w, http.StatusUnprocessableEntity, "model_unavailable", err.Error())
	case errors.Is(err, ErrAgentTryoutRerunProviderKeyRequired):
		writeError(w, http.StatusPaymentRequired, "provider_key_required", "Connect a provider key for this model to rerun.")
	case errors.Is(err, ErrAgentTryoutRerunInsufficientCredits):
		writeError(w, http.StatusPaymentRequired, "insufficient_credits", "Not enough hosted credits to rerun. Add credits or connect a provider key.")
	case errors.Is(err, ErrAgentTryoutCompareCardinality):
		writeError(w, http.StatusBadRequest, "compare_cardinality", "Compare requires between 2 and 4 tryouts.")
	case errors.Is(err, ErrAgentTryoutPromotionTargetUnsupported):
		writeError(w, http.StatusBadRequest, "promotion_target_unsupported", err.Error())
	case errors.Is(err, ErrAgentTryoutNotPromotable):
		writeError(w, http.StatusConflict, "agent_tryout_not_promotable", "Only completed tryouts can be promoted to an eval.")
	case errors.Is(err, ErrInvalidAgentTryoutInput):
		writeError(w, http.StatusBadRequest, "invalid_agent_tryout_input", err.Error())
	case errors.Is(err, ErrForbidden):
		writeAuthzError(w, err)
	default:
		logger.Error("agent tryout request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func agentTryoutTemplateUnavailableMessage(err error) string {
	message := strings.TrimPrefix(err.Error(), ErrAgentTryoutTemplateUnavailable.Error()+": ")
	if strings.TrimSpace(message) == "" {
		return "agent tryout template is unavailable"
	}
	return message
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
		SelectedHarnessKind:    tryout.SelectedHarnessKind,
		Summary:                tryout.Summary,
		RedactionStatus:        tryout.RedactionStatus,
		RunID:                  tryout.RunID,
		ParentTryoutID:         tryout.ParentTryoutID,
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
		SelectedHarnessKind:    tryout.SelectedHarnessKind,
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
		"available":            template.Available,
		"unavailable_reason":   template.UnavailableReason,
		"runtime":              json.RawMessage(template.Runtime),
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
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["notes"],"additionalProperties":false,"properties":{"notes":{"type":"string"},"audience":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"meeting_minutes_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"action_plan","type":"markdown","path":"action-plan.md"},{"key":"structured_minutes","type":"json","path":"minutes.json"}],"validation":{"validators":[{"key":"has_summary","type":"json_field","field":"summary"},{"key":"has_action_items","type":"json_field","field":"action_items"}],"score_dimensions":["correctness","reliability","latency","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"has_summary","type":"json_field","field":"summary"},{"key":"has_action_items","type":"json_field","field":"action_items"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
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
			Available:          false,
			UnavailableReason:  "structured data validator runtime is not enabled yet",
			InputSchema:        json.RawMessage(`{"type":"object","required":["text"],"additionalProperties":false,"properties":{"text":{"type":"string"},"schema":{"type":"object"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["schema_validator","file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"structured_data_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"extracted_rows","type":"json","path":"extracted-rows.json"}],"validation":{"validators":[{"key":"valid_json","type":"json_schema"}],"score_dimensions":["correctness","reliability","latency","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"valid_json","type":"json_schema"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 120,
			MaxCostUSD:         0.25,
		},
		{
			Slug:               "slide-deck",
			Name:               "Brief to Slide Deck",
			Description:        "Turn a rough brief into a structured slide deck outline with speaker notes.",
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["brief"],"additionalProperties":false,"properties":{"brief":{"type":"string"},"audience":{"type":"string"},"slide_count":{"type":"integer","minimum":3,"maximum":20}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"slide_deck_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"deck_outline","type":"markdown","path":"deck.md"},{"key":"structured_deck","type":"json","path":"deck.json"}],"validation":{"validators":[{"key":"has_title","type":"json_field","field":"title"},{"key":"has_slides","type":"json_field","field":"slides"}],"score_dimensions":["correctness","reliability","latency","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"has_title","type":"json_field","field":"title"},{"key":"has_slides","type":"json_field","field":"slides"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 180,
			MaxCostUSD:         0.35,
		},
		{
			Slug:               "spreadsheet-builder",
			Name:               "Data to Spreadsheet",
			Description:        "Turn pasted data or a description into a clean spreadsheet with derived columns and a summary of insights.",
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["data"],"additionalProperties":false,"properties":{"data":{"type":"string"},"instructions":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"spreadsheet_builder_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"spreadsheet","type":"csv","path":"spreadsheet.csv"},{"key":"analysis","type":"json","path":"analysis.json"}],"validation":{"validators":[{"key":"has_columns","type":"json_field","field":"columns"},{"key":"has_insights","type":"json_field","field":"insights"}],"score_dimensions":["correctness","reliability","latency","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"has_columns","type":"json_field","field":"columns"},{"key":"has_insights","type":"json_field","field":"insights"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 180,
			MaxCostUSD:         0.35,
		},
		{
			Slug:               "status-report",
			Name:               "Updates to Status Report",
			Description:        "Turn scattered bullet updates into a polished status report with highlights, risks, and next steps.",
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["updates"],"additionalProperties":false,"properties":{"updates":{"type":"string"},"period":{"type":"string"},"audience":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"status_report_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"report","type":"markdown","path":"status-report.md"},{"key":"structured_report","type":"json","path":"report.json"}],"validation":{"validators":[{"key":"has_highlights","type":"json_field","field":"highlights"},{"key":"has_next_steps","type":"json_field","field":"next_steps"}],"score_dimensions":["correctness","reliability","latency","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"has_highlights","type":"json_field","field":"highlights"},{"key":"has_next_steps","type":"json_field","field":"next_steps"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 120,
			MaxCostUSD:         0.25,
		},
		{
			Slug:               "inbox-triage",
			Name:               "Inbox Triage and Draft Replies",
			Description:        "Prioritize a batch of pasted emails and draft a reply for each one that needs a response.",
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["emails"],"additionalProperties":false,"properties":{"emails":{"type":"string"},"priorities":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"inbox_triage_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"triage_board","type":"markdown","path":"triage.md"},{"key":"structured_triage","type":"json","path":"triage.json"}],"validation":{"validators":[{"key":"has_queue","type":"json_field","field":"queue"}],"score_dimensions":["correctness","reliability","latency","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"has_queue","type":"json_field","field":"queue"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 150,
			MaxCostUSD:         0.3,
		},
		{
			Slug:               "tiny-bugfix",
			Name:               "Fix a Tiny Bug",
			Description:        "Run an agent against a small fixture, inspect the diff, and see whether tests pass.",
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["task"],"additionalProperties":false,"properties":{"task":{"type":"string"},"fixture":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["sandbox_shell","file_editor"],"sandbox":{"filesystem":"workspace","shell":"allowed"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"tiny_bugfix_v1","sandbox":{"filesystem":"workspace","shell":"allowed","network":"disabled"},"expected_artifacts":[{"key":"diff","type":"patch","path":"changes.patch"},{"key":"test_result","type":"json","path":"test-result.json"}],"validation":{"validators":[{"key":"tests_pass","type":"command_exit_code"}],"score_dimensions":["correctness","reliability","latency","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"tests_pass","type":"command_exit_code"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   false,
			MaxInputBytes:      32 * 1024,
			MaxDurationSeconds: 300,
			MaxCostUSD:         0.75,
		},
		// Enterprise eval templates — the highest-budget, fastest-payback agent
		// use-cases enterprises evaluate before integrating. Each runs on the
		// generic public sandbox (no per-task worker adapter) and produces an
		// inspectable JSON artifact the scorecard validates.
		{
			Slug:               "support-ticket-resolution",
			Name:               "Resolve a Support Ticket",
			Description:        "Draft a grounded customer-support reply, decide whether to escalate, and flag policy/compliance risks — the #1 enterprise agent use-case.",
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["ticket"],"additionalProperties":false,"properties":{"ticket":{"type":"string"},"knowledge_base":{"type":"string"},"policy":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"office_generic_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"reply","type":"markdown","path":"reply.md"},{"key":"resolution","type":"json","path":"resolution.json"}],"instructions":"Resolve the support ticket using only the supplied knowledge base and policy. Write reply.md (the customer-facing reply) and resolution.json with fields: answer (string), escalate (boolean), confidence (number 0-1), citations (array), policy_flags (array). Never invent facts not in the knowledge base.","validation":{"validators":[{"key":"answer","type":"json_field","field":"answer"},{"key":"escalate","type":"json_field","field":"escalate"}],"score_dimensions":["accuracy","escalation","compliance","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"answer","type":"json_field","field":"answer"},{"key":"escalate","type":"json_field","field":"escalate"},{"key":"citations","type":"json_field","field":"citations"}],"scorecard":{"dimensions":["accuracy","escalation","compliance","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      96 * 1024,
			MaxDurationSeconds: 150,
			MaxCostUSD:         0.3,
		},
		{
			Slug:               "document-extraction",
			Name:               "Extract Invoice / Document Data",
			Description:        "Pull structured fields (line items, totals, vendor) from a messy invoice or document and flag exceptions — replaces manual AP data entry.",
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["document"],"additionalProperties":false,"properties":{"document":{"type":"string"},"fields":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"office_generic_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"extracted","type":"json","path":"extracted.json"},{"key":"review","type":"markdown","path":"review.md"}],"instructions":"Extract the requested fields from the document into extracted.json with fields: vendor (string), total (number), currency (string), line_items (array of {description, quantity, amount}), exceptions (array of strings for anything ambiguous or missing). Write review.md summarizing confidence and any exceptions. Do not fabricate values.","validation":{"validators":[{"key":"line_items","type":"json_field","field":"line_items"},{"key":"total","type":"json_field","field":"total"}],"score_dimensions":["accuracy","exceptions","latency","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"line_items","type":"json_field","field":"line_items"},{"key":"total","type":"json_field","field":"total"},{"key":"vendor","type":"json_field","field":"vendor"}],"scorecard":{"dimensions":["accuracy","exceptions","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      96 * 1024,
			MaxDurationSeconds: 150,
			MaxCostUSD:         0.3,
		},
		{
			Slug:               "contract-review",
			Name:               "Review a Contract Clause",
			Description:        "Extract key clauses, surface risks, and propose redlines against a checklist — high-value legal work where hallucination rates run ~6%.",
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["contract"],"additionalProperties":false,"properties":{"contract":{"type":"string"},"checklist":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"office_generic_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"review","type":"json","path":"review.json"},{"key":"summary","type":"markdown","path":"summary.md"}],"instructions":"Review the contract against the checklist. Write review.json with fields: clauses (array of {name, summary, location}), risks (array of {issue, severity, clause}), redlines (array of suggested edits). Write summary.md for a non-lawyer. Quote the contract verbatim for every claim; never invent clauses or citations.","validation":{"validators":[{"key":"clauses","type":"json_field","field":"clauses"},{"key":"risks","type":"json_field","field":"risks"}],"score_dimensions":["accuracy","hallucination","compliance","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"clauses","type":"json_field","field":"clauses"},{"key":"risks","type":"json_field","field":"risks"},{"key":"redlines","type":"json_field","field":"redlines"}],"scorecard":{"dimensions":["accuracy","hallucination","compliance","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      128 * 1024,
			MaxDurationSeconds: 180,
			MaxCostUSD:         0.5,
		},
		{
			Slug:               "sdr-outreach",
			Name:               "Qualify a Lead & Draft Outreach",
			Description:        "Score a prospect against an ideal-customer profile and draft a personalized outbound email — the fastest-payback sales agent use-case.",
			Available:          true,
			InputSchema:        json.RawMessage(`{"type":"object","required":["prospect","offer"],"additionalProperties":false,"properties":{"prospect":{"type":"string"},"offer":{"type":"string"},"tone":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"sandbox":{"filesystem":"workspace","shell":"disabled"},"network":{"mode":"disabled"},"external_side_effects":false}`),
			Runtime:            json.RawMessage(`{"adapter":"office_generic_v1","sandbox":{"filesystem":"workspace","shell":"disabled","network":"disabled"},"expected_artifacts":[{"key":"email","type":"markdown","path":"email.md"},{"key":"outreach","type":"json","path":"outreach.json"}],"instructions":"Qualify the prospect for the offer and draft outreach. Write outreach.json with fields: subject (string), body (string), qualification (object with fit_score 0-1 and reasons array), personalization (array). Write email.md as the send-ready email. Keep claims grounded in the prospect details provided.","validation":{"validators":[{"key":"subject","type":"json_field","field":"subject"},{"key":"body","type":"json_field","field":"body"}],"score_dimensions":["relevance","tone","deliverability","cost"]}}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"subject","type":"json_field","field":"subject"},{"key":"body","type":"json_field","field":"body"},{"key":"qualification","type":"json_field","field":"qualification"}],"scorecard":{"dimensions":["relevance","tone","deliverability","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 120,
			MaxCostUSD:         0.25,
		},
	}
}
