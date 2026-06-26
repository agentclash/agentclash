package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AgentTryoutRerunGate authorizes a rerun against billing/provider entitlements
// (active plan, provider key availability, sufficient credits). It is optional
// and injected via WithRerunGate, mirroring WithExecution/WithQuota: when no gate
// is configured, reruns proceed (dev/default). Implementations return the
// product-facing sentinel errors below so handlers can map them to HTTP codes.
type AgentTryoutRerunGate interface {
	AuthorizeRerun(ctx context.Context, caller Caller, workspaceID uuid.UUID, selectedModelPolicy json.RawMessage) error
}

// WithRerunGate attaches a billing/provider gate that reruns must pass.
func (m *AgentTryoutManager) WithRerunGate(gate AgentTryoutRerunGate) *AgentTryoutManager {
	m.rerunGate = gate
	return m
}

// knownTryoutProviderKeys is the set of provider keys a rerun model policy may
// reference. It mirrors the provider router's registered adapters.
var knownTryoutProviderKeys = map[string]bool{
	"openai":     true,
	"anthropic":  true,
	"gemini":     true,
	"xai":        true,
	"openrouter": true,
	"mistral":    true,
}

// tryoutModelPolicy is the validated shape of a rerun's selected model policy.
type tryoutModelPolicy struct {
	Mode      string `json:"mode"`
	MaxModels *int   `json:"max_models"`
	Models    []struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	} `json:"models"`
}

// validateTryoutModelPolicy checks that a requested rerun model policy is a
// well-formed object that names at least a mode or an explicit model, and that
// any explicit models reference known providers. Returns
// ErrAgentTryoutModelPolicyInvalid for malformed input and
// ErrAgentTryoutModelUnavailable for unknown providers / empty model ids.
func validateTryoutModelPolicy(raw json.RawMessage) error {
	policy, err := parseTryoutModelPolicy(raw)
	if err != nil {
		return err
	}
	mode := strings.TrimSpace(policy.Mode)
	if mode == "" && len(policy.Models) == 0 {
		return fmt.Errorf("%w: model policy must set a mode or at least one model", ErrAgentTryoutModelPolicyInvalid)
	}
	if policy.MaxModels != nil && (*policy.MaxModels < 1 || *policy.MaxModels > 8) {
		return fmt.Errorf("%w: max_models must be between 1 and 8", ErrAgentTryoutModelPolicyInvalid)
	}
	for _, entry := range policy.Models {
		provider := strings.TrimSpace(entry.Provider)
		model := strings.TrimSpace(entry.Model)
		if provider == "" || model == "" {
			return fmt.Errorf("%w: each model needs a provider and model id", ErrAgentTryoutModelPolicyInvalid)
		}
		if !knownTryoutProviderKeys[provider] {
			return fmt.Errorf("%w: provider %q is not available", ErrAgentTryoutModelUnavailable, provider)
		}
	}
	return nil
}

func parseTryoutModelPolicy(raw json.RawMessage) (tryoutModelPolicy, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return tryoutModelPolicy{}, fmt.Errorf("%w: model policy is required", ErrAgentTryoutModelPolicyInvalid)
	}
	if trimmed[0] != '{' {
		return tryoutModelPolicy{}, fmt.Errorf("%w: model policy must be a JSON object", ErrAgentTryoutModelPolicyInvalid)
	}
	var policy tryoutModelPolicy
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return tryoutModelPolicy{}, fmt.Errorf("%w: %v", ErrAgentTryoutModelPolicyInvalid, err)
	}
	return policy, nil
}

// RerunAgentTryoutInput requests a rerun of an existing workspace tryout with a
// different model policy and, optionally, a different agent design.
type RerunAgentTryoutInput struct {
	SourceTryoutID      uuid.UUID
	SelectedModelPolicy json.RawMessage

	// Optional agent-design override. When AgentDesignProvided is true the rerun
	// replaces the source's agent_design with these fields (after normalization),
	// making the agent design — not just the model — the varied dimension. When
	// false the rerun inherits the source's agent_design unchanged.
	AgentDesignProvided bool
	AgentInstructions   string
	AgentToolSlugs      []string
	AgentName           string
}

// RerunWorkspaceTryout creates a new tryout that reuses the source tryout's
// template + input but runs under a different model policy, linked back to the
// source via parent_tryout_id. The rerun is independent immutable evidence; the
// source is never mutated.
func (m *AgentTryoutManager) RerunWorkspaceTryout(ctx context.Context, caller Caller, input RerunAgentTryoutInput) (repository.AgentTryout, error) {
	source, err := m.repo.GetAgentTryoutByID(ctx, input.SourceTryoutID)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	// Anonymous/unclaimed tryouts have no workspace — rerun is a signed-in action.
	if source.WorkspaceID == nil || source.OrganizationID == nil {
		return repository.AgentTryout{}, ErrAgentTryoutSignInRequired
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, *source.WorkspaceID); err != nil {
		return repository.AgentTryout{}, err
	}
	template, err := m.lookupTemplate(source.TemplateSlug)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if err := ensureAgentTryoutTemplateAvailable(template); err != nil {
		return repository.AgentTryout{}, err
	}
	if err := validateTryoutModelPolicy(input.SelectedModelPolicy); err != nil {
		return repository.AgentTryout{}, err
	}
	if m.rerunGate != nil {
		if err := m.rerunGate.AuthorizeRerun(ctx, caller, *source.WorkspaceID, input.SelectedModelPolicy); err != nil {
			return repository.AgentTryout{}, err
		}
	}

	// Clone the source input, then optionally override the agent design so the
	// rerun varies the agent (not just the model). When no override is supplied
	// the cloned snapshot — including any inherited agent_design — is reused.
	inputSnapshot := cloneRawJSON(source.InputSnapshot)
	toolPolicySnapshot := template.ToolPolicy
	if input.AgentDesignProvided {
		design, designPresent, designErr := normalizeAgentDesign(agentDesignInput{
			Name:         input.AgentName,
			Instructions: input.AgentInstructions,
			ToolSlugs:    input.AgentToolSlugs,
		})
		if designErr != nil {
			return repository.AgentTryout{}, designErr
		}
		inputSnapshot = replaceAgentDesignInInput(inputSnapshot, design, designPresent)
		if designPresent {
			toolPolicySnapshot = toolPolicyWithAgentToolKinds(toolPolicySnapshot, design.ToolSlugs)
		}
	}

	callerID := caller.UserID
	parentID := source.ID
	tryout, err := m.repo.CreateAgentTryout(ctx, repository.CreateAgentTryoutParams{
		OrganizationID:         source.OrganizationID,
		WorkspaceID:            source.WorkspaceID,
		TemplateSlug:           template.Slug,
		Status:                 repository.AgentTryoutStatusQueued,
		InputSnapshot:          inputSnapshot,
		TemplateSnapshot:       templateSnapshot(template),
		ToolPolicySnapshot:     toolPolicySnapshot,
		EvaluationSpecSnapshot: template.EvaluationSpec,
		SelectedModelPolicy:    input.SelectedModelPolicy,
		Summary:                json.RawMessage(`{}`),
		RedactionStatus:        repository.AgentTryoutRedactionPending,
		CostLimitUSD:           template.MaxCostUSD,
		MaxDurationSeconds:     template.MaxDurationSeconds,
		CreatedByUserID:        &callerID,
		ParentTryoutID:         &parentID,
	})
	if err != nil {
		return repository.AgentTryout{}, err
	}
	return m.dispatchCreatedTryout(ctx, tryout, template)
}

// CompareAgentTryoutsInput requests a side-by-side comparison of 2-4 workspace
// tryouts.
type CompareAgentTryoutsInput struct {
	WorkspaceID uuid.UUID
	TryoutIDs   []uuid.UUID
}

// AgentTryoutCompareParticipant is one tryout's slice of a comparison payload.
type AgentTryoutCompareParticipant struct {
	ID                  uuid.UUID                             `json:"id"`
	TemplateSlug        string                                `json:"template_slug"`
	Status              repository.AgentTryoutStatus          `json:"status"`
	RedactionStatus     repository.AgentTryoutRedactionStatus `json:"redaction_status"`
	SelectedModelPolicy json.RawMessage                       `json:"selected_model_policy"`
	RunID               *uuid.UUID                            `json:"run_id,omitempty"`
	ParentTryoutID      *uuid.UUID                            `json:"parent_tryout_id,omitempty"`
	CostLimitUSD        float64                               `json:"cost_limit_usd"`
	ActualCostUSD       *float64                              `json:"actual_cost_usd,omitempty"`
	LatencyMS           *int64                                `json:"latency_ms,omitempty"`
	Summary             json.RawMessage                       `json:"summary"`
	EventsURL           string                                `json:"events_url"`
}

// AgentTryoutCompareResult is the aggregated side-by-side comparison.
type AgentTryoutCompareResult struct {
	WorkspaceID  uuid.UUID                       `json:"workspace_id"`
	Participants []AgentTryoutCompareParticipant `json:"participants"`
}

// CompareWorkspaceTryouts loads 2-4 tryouts that all belong to the workspace and
// returns a side-by-side payload. It is read-only — reruns being compared are
// never mutated. Any id outside the workspace (or missing) fails closed as
// not-found so existence isn't leaked across workspaces.
func (m *AgentTryoutManager) CompareWorkspaceTryouts(ctx context.Context, caller Caller, input CompareAgentTryoutsInput) (AgentTryoutCompareResult, error) {
	// Deduplicate before the cardinality check so the 2-4 bound is enforced on
	// distinct tryouts — otherwise ["id-A","id-A"] would slip past len()>=2 and
	// return a single participant, violating the contract.
	seen := make(map[uuid.UUID]bool, len(input.TryoutIDs))
	distinctIDs := make([]uuid.UUID, 0, len(input.TryoutIDs))
	for _, id := range input.TryoutIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		distinctIDs = append(distinctIDs, id)
	}
	if len(distinctIDs) < 2 || len(distinctIDs) > 4 {
		return AgentTryoutCompareResult{}, ErrAgentTryoutCompareCardinality
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionReadWorkspace); err != nil {
		return AgentTryoutCompareResult{}, err
	}

	participants := make([]AgentTryoutCompareParticipant, 0, len(distinctIDs))
	for _, id := range distinctIDs {
		tryout, err := m.repo.GetAgentTryoutByID(ctx, id)
		if err != nil {
			return AgentTryoutCompareResult{}, err
		}
		// Fail closed across workspaces: a tryout that isn't owned by this
		// workspace is reported as not-found rather than leaking its existence.
		if tryout.WorkspaceID == nil || *tryout.WorkspaceID != input.WorkspaceID {
			return AgentTryoutCompareResult{}, repository.ErrAgentTryoutNotFound
		}
		tryout = m.refreshTryoutFromExecution(ctx, tryout)
		participants = append(participants, AgentTryoutCompareParticipant{
			ID:                  tryout.ID,
			TemplateSlug:        tryout.TemplateSlug,
			Status:              tryout.Status,
			RedactionStatus:     tryout.RedactionStatus,
			SelectedModelPolicy: tryout.SelectedModelPolicy,
			RunID:               tryout.RunID,
			ParentTryoutID:      tryout.ParentTryoutID,
			CostLimitUSD:        tryout.CostLimitUSD,
			ActualCostUSD:       tryout.ActualCostUSD,
			LatencyMS:           tryout.LatencyMS,
			Summary:             tryout.Summary,
			EventsURL:           fmt.Sprintf("/v1/workspaces/%s/agent-tryouts/%s/events", input.WorkspaceID, tryout.ID),
		})
	}
	return AgentTryoutCompareResult{WorkspaceID: input.WorkspaceID, Participants: participants}, nil
}

// PromoteAgentTryoutInput requests promoting a tryout into a repeatable eval.
type PromoteAgentTryoutInput struct {
	SourceTryoutID uuid.UUID
	Target         string
	Title          string
}

// AgentTryoutPromotionResult references the durable workspace draft a promotion
// produced.
type AgentTryoutPromotionResult struct {
	Target         string    `json:"target"`
	ConversationID uuid.UUID `json:"conversation_id"`
	DraftID        uuid.UUID `json:"draft_id"`
}

// PromoteTryoutToEval converts a completed workspace tryout into a durable
// Vibe Eval draft, preserving its template/input/tool-policy/evaluation-spec and
// the template's expected artifacts so the eval can be run again later. v1
// supports only the "vibe_eval" target.
func (m *AgentTryoutManager) PromoteTryoutToEval(ctx context.Context, caller Caller, input PromoteAgentTryoutInput) (AgentTryoutPromotionResult, error) {
	target := strings.TrimSpace(input.Target)
	if target == "" {
		target = "vibe_eval"
	}
	if target != "vibe_eval" {
		return AgentTryoutPromotionResult{}, fmt.Errorf("%w: %q", ErrAgentTryoutPromotionTargetUnsupported, target)
	}

	source, err := m.repo.GetAgentTryoutByID(ctx, input.SourceTryoutID)
	if err != nil {
		return AgentTryoutPromotionResult{}, err
	}
	if source.WorkspaceID == nil || source.OrganizationID == nil {
		return AgentTryoutPromotionResult{}, ErrAgentTryoutSignInRequired
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, *source.WorkspaceID); err != nil {
		return AgentTryoutPromotionResult{}, err
	}
	// Only completed tryouts carry the execution evidence worth promoting; a
	// queued/running/failed tryout would seed a draft backed by partial or no
	// results.
	if source.Status != repository.AgentTryoutStatusCompleted {
		return AgentTryoutPromotionResult{}, fmt.Errorf("%w: status %q", ErrAgentTryoutNotPromotable, source.Status)
	}

	content, err := promotionDraftContent(source)
	if err != nil {
		return AgentTryoutPromotionResult{}, err
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = fmt.Sprintf("Tryout: %s", source.TemplateSlug)
	}
	conversation, err := m.repo.CreateVibeEvalConversation(ctx, repository.CreateVibeEvalConversationParams{
		OrganizationID:  *source.OrganizationID,
		WorkspaceID:     *source.WorkspaceID,
		CreatedByUserID: caller.UserID,
		Title:           title,
		Phase:           "plan",
		Status:          "active",
	})
	if err != nil {
		return AgentTryoutPromotionResult{}, err
	}

	draft, err := m.repo.CreateVibeEvalDraft(ctx, repository.CreateVibeEvalDraftParams{
		OrganizationID:   *source.OrganizationID,
		WorkspaceID:      *source.WorkspaceID,
		ConversationID:   conversation.ID,
		DraftKind:        "eval_plan",
		Content:          content,
		ValidationState:  "unknown",
		ValidationErrors: json.RawMessage(`[]`),
		CreatedByUserID:  caller.UserID,
		UpdatedByUserID:  caller.UserID,
	})
	if err != nil {
		return AgentTryoutPromotionResult{}, err
	}
	return AgentTryoutPromotionResult{Target: target, ConversationID: conversation.ID, DraftID: draft.ID}, nil
}

// promotionDraftContent packs a tryout's reusable definition into a Vibe Eval
// draft body. It includes only the durable, workspace-owned definition (no run
// ids, costs, or identifiers) so the draft is a clean starting point for a
// repeatable eval.
func promotionDraftContent(source repository.AgentTryout) (json.RawMessage, error) {
	body := map[string]any{
		"kind":                     "agent_tryout_promotion",
		"source_tryout_id":         source.ID.String(),
		"template_slug":            source.TemplateSlug,
		"input_snapshot":           rawOrNull(source.InputSnapshot),
		"tool_policy_snapshot":     rawOrNull(source.ToolPolicySnapshot),
		"evaluation_spec_snapshot": rawOrNull(source.EvaluationSpecSnapshot),
		"expected_artifacts":       templateExpectedArtifacts(source.TemplateSnapshot),
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		// Surface the failure instead of persisting an empty draft + false 201.
		return nil, fmt.Errorf("marshal promotion draft content: %w", err)
	}
	return encoded, nil
}

// templateExpectedArtifacts extracts runtime.expected_artifacts from a stored
// template snapshot so the promoted eval records the expected outputs. Returns
// an empty slice when none are declared.
func templateExpectedArtifacts(templateSnapshot json.RawMessage) []map[string]any {
	out := []map[string]any{}
	if len(templateSnapshot) == 0 {
		return out
	}
	var snapshot struct {
		Runtime struct {
			ExpectedArtifacts []map[string]any `json:"expected_artifacts"`
		} `json:"runtime"`
	}
	if err := json.Unmarshal(templateSnapshot, &snapshot); err != nil {
		return out
	}
	if snapshot.Runtime.ExpectedArtifacts != nil {
		return snapshot.Runtime.ExpectedArtifacts
	}
	return out
}

// --- HTTP handlers ---

type rerunAgentTryoutRequest struct {
	SelectedModelPolicy json.RawMessage `json:"selected_model_policy"`
	// Agent-design override. agent_design_provided gates whether the rerun
	// replaces the source's design; when false the source design is inherited.
	AgentDesignProvided bool     `json:"agent_design_provided,omitempty"`
	AgentInstructions   string   `json:"agent_instructions,omitempty"`
	AgentToolSlugs      []string `json:"agent_tool_slugs,omitempty"`
	AgentName           string   `json:"agent_name,omitempty"`
}

func rerunAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
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
		var req rerunAgentTryoutRequest
		if err := decodeAgentTryoutJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		tryout, err := service.RerunWorkspaceTryout(r.Context(), caller, RerunAgentTryoutInput{
			SourceTryoutID:      id,
			SelectedModelPolicy: req.SelectedModelPolicy,
			AgentDesignProvided: req.AgentDesignProvided,
			AgentInstructions:   req.AgentInstructions,
			AgentToolSlugs:      req.AgentToolSlugs,
			AgentName:           req.AgentName,
		})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapAgentTryoutResponse(tryout))
	}
}

type compareAgentTryoutsRequest struct {
	TryoutIDs []uuid.UUID `json:"tryout_ids"`
}

func compareAgentTryoutsHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
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
		var req compareAgentTryoutsRequest
		if err := decodeAgentTryoutJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		result, err := service.CompareWorkspaceTryouts(r.Context(), caller, CompareAgentTryoutsInput{
			WorkspaceID: workspaceID,
			TryoutIDs:   req.TryoutIDs,
		})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

type promoteAgentTryoutRequest struct {
	Target string `json:"target"`
	Title  string `json:"title"`
}

func promoteAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
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
		var req promoteAgentTryoutRequest
		// The body is optional — an empty POST promotes to the default target.
		if r.ContentLength != 0 {
			if err := decodeAgentTryoutJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
		}
		result, err := service.PromoteTryoutToEval(r.Context(), caller, PromoteAgentTryoutInput{
			SourceTryoutID: id,
			Target:         req.Target,
			Title:          req.Title,
		})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

// AgentTryoutArtifact is one captured output file of a tryout, with a signed,
// time-limited download URL when a signer is configured.
type AgentTryoutArtifact struct {
	ID                uuid.UUID  `json:"id"`
	Key               string     `json:"key,omitempty"`
	Path              string     `json:"path,omitempty"`
	ArtifactType      string     `json:"artifact_type"`
	ContentType       *string    `json:"content_type,omitempty"`
	SizeBytes         *int64     `json:"size_bytes,omitempty"`
	DownloadURL       string     `json:"download_url,omitempty"`
	DownloadExpiresAt *time.Time `json:"download_expires_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// ListWorkspaceTryoutArtifacts returns the captured output files for a workspace
// tryout (e.g. the slide deck the agent produced), each with a signed download
// URL. Unlike the public share view, the workspace owner sees every captured
// artifact — there is no template allowlist redaction here.
func (m *AgentTryoutManager) ListWorkspaceTryoutArtifacts(ctx context.Context, caller Caller, tryoutID uuid.UUID, baseURL string) ([]AgentTryoutArtifact, error) {
	tryout, err := m.repo.GetAgentTryoutByID(ctx, tryoutID)
	if err != nil {
		return nil, err
	}
	if tryout.WorkspaceID == nil {
		return nil, ErrAgentTryoutSignInRequired
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, *tryout.WorkspaceID); err != nil {
		return nil, err
	}
	if tryout.RunID == nil {
		return []AgentTryoutArtifact{}, nil
	}
	artifacts, err := m.repo.ListArtifactsByRunID(ctx, *tryout.RunID)
	if err != nil {
		return nil, err
	}
	now := m.now()
	out := make([]AgentTryoutArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		key, path := artifactClaimedIdentity(artifact.Metadata)
		item := AgentTryoutArtifact{
			ID:           artifact.ID,
			Key:          key,
			Path:         path,
			ArtifactType: artifact.ArtifactType,
			ContentType:  artifact.ContentType,
			SizeBytes:    artifact.SizeBytes,
			CreatedAt:    artifact.CreatedAt,
		}
		if m.artifactSigner != nil {
			if url, expiresAt, signErr := m.artifactSigner.SignedArtifactContentURL(artifact.ID, baseURL, now); signErr == nil {
				item.DownloadURL = url
				item.DownloadExpiresAt = &expiresAt
			}
		}
		out = append(out, item)
	}
	return out, nil
}

type listAgentTryoutArtifactsResponse struct {
	Items []AgentTryoutArtifact `json:"items"`
}

func listAgentTryoutArtifactsHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
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
		artifacts, err := service.ListWorkspaceTryoutArtifacts(r.Context(), caller, id, requestBaseURL(r))
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, listAgentTryoutArtifactsResponse{Items: artifacts})
	}
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	cloned := make(json.RawMessage, len(raw))
	copy(cloned, raw)
	return cloned
}

func rawOrNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`null`)
	}
	return raw
}
