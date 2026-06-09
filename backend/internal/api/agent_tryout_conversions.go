package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

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
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return fmt.Errorf("%w: model policy is required", ErrAgentTryoutModelPolicyInvalid)
	}
	if trimmed[0] != '{' {
		return fmt.Errorf("%w: model policy must be a JSON object", ErrAgentTryoutModelPolicyInvalid)
	}
	var policy tryoutModelPolicy
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return fmt.Errorf("%w: %v", ErrAgentTryoutModelPolicyInvalid, err)
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

// RerunAgentTryoutInput requests a rerun of an existing workspace tryout with a
// different model policy.
type RerunAgentTryoutInput struct {
	SourceTryoutID      uuid.UUID
	SelectedModelPolicy json.RawMessage
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

	callerID := caller.UserID
	parentID := source.ID
	tryout, err := m.repo.CreateAgentTryout(ctx, repository.CreateAgentTryoutParams{
		OrganizationID:         source.OrganizationID,
		WorkspaceID:            source.WorkspaceID,
		TemplateSlug:           template.Slug,
		Status:                 repository.AgentTryoutStatusQueued,
		InputSnapshot:          cloneRawJSON(source.InputSnapshot),
		TemplateSnapshot:       templateSnapshot(template),
		ToolPolicySnapshot:     template.ToolPolicy,
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
	if len(input.TryoutIDs) < 2 || len(input.TryoutIDs) > 4 {
		return AgentTryoutCompareResult{}, ErrAgentTryoutCompareCardinality
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionReadWorkspace); err != nil {
		return AgentTryoutCompareResult{}, err
	}

	seen := make(map[uuid.UUID]bool, len(input.TryoutIDs))
	participants := make([]AgentTryoutCompareParticipant, 0, len(input.TryoutIDs))
	for _, id := range input.TryoutIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
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

	content := promotionDraftContent(source)
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
func promotionDraftContent(source repository.AgentTryout) json.RawMessage {
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
		return json.RawMessage(`{}`)
	}
	return encoded
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
