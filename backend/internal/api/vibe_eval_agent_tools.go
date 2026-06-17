package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

// Read-only guide-agent tool adapters (Step 2). Each is a thin, stateless wrapper over an
// existing api manager: it pulls the authenticated api.Caller from context (the handler put
// it there) and calls the manager, which performs the authoritative data-aware authz. The
// loop has already coarse-authorized via WorkspaceAuthorizer using the tool's action string.
// vibeeval.Actor is audit identity only — never used to call managers.

// small reader interfaces the concrete managers satisfy (accept interfaces).
type runStatusReader interface {
	GetRun(ctx context.Context, caller Caller, runID uuid.UUID) (GetRunResult, error)
}

type scorecardReader interface {
	GetRunAgentScorecard(ctx context.Context, caller Caller, runAgentID uuid.UUID) (GetRunAgentScorecardResult, error)
}

// challengePackLister lists the packs visible to the request's workspace. ListChallengePacks
// scopes by the workspace in context, which the turns route's authorizeWorkspaceAccess
// middleware populates — so the guide tool needs no explicit workspace argument.
type challengePackLister interface {
	ListChallengePacks(ctx context.Context) (ListChallengePacksResult, error)
}

// vibeEvalDraftAuthor is the manager surface the draft-authoring tools wrap. The guide tool and the
// REST handlers call the SAME manager methods (Q2 — one source of truth for auth/validation/audit).
type vibeEvalDraftAuthor interface {
	CreateDraft(ctx context.Context, caller Caller, input CreateVibeEvalDraftInput) (repository.VibeEvalDraft, error)
	GetDraft(ctx context.Context, caller Caller, input GetVibeEvalDraftInput) (repository.VibeEvalDraft, error)
	UpdateDraft(ctx context.Context, caller Caller, input UpdateVibeEvalDraftInput) (repository.VibeEvalDraft, error)
	ValidateDraft(ctx context.Context, caller Caller, input ValidateVibeEvalDraftInput) (ValidateVibeEvalDraftResult, error)
}

const draftKindEnumJSON = `{"type":"string","enum":["eval_plan","challenge_pack","input_cases","scoring","runtime"]}`

func draftToolOutput(draft repository.VibeEvalDraft) vibeeval.ToolOutput {
	return vibeeval.ToolOutput{
		Result: map[string]any{
			"draft_id":         draft.ID.String(),
			"draft_kind":       draft.DraftKind,
			"validation_state": draft.ValidationState,
		},
		AuditResult: map[string]any{"draft_id": draft.ID.String(), "draft_kind": draft.DraftKind},
	}
}

// --- create_draft (draft tier, no confirmation, audited) ---

type createDraftTool struct{ drafts vibeEvalDraftAuthor }

func (createDraftTool) Name() string                { return "create_draft" }
func (createDraftTool) Phases() []string            { return []string{vibeeval.PhasePlan, vibeeval.PhaseAuthor} }
func (createDraftTool) RiskTier() vibeeval.RiskTier { return vibeeval.DraftTier }
func (createDraftTool) RequiredAction() string      { return string(ActionManageVibeEvalDrafts) }
func (createDraftTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "create_draft",
		Description: "Create a draft artifact in the current conversation and make it the active draft.",
		Parameters:  json.RawMessage(`{"type":"object","required":["draft_kind","content"],"properties":{"draft_kind":` + draftKindEnumJSON + `,"content":{"type":"object","description":"the draft body"}}}`),
	}
}

func (t createDraftTool) Execute(ctx context.Context, _ vibeeval.Actor, conv vibeeval.Conversation, args json.RawMessage) (vibeeval.ToolOutput, error) {
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	var in struct {
		DraftKind string          `json:"draft_kind"`
		Content   json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("invalid args: %w", err)
	}
	draft, err := t.drafts.CreateDraft(ctx, caller, CreateVibeEvalDraftInput{
		WorkspaceID:    conv.WorkspaceID,
		ConversationID: conv.ID,
		DraftKind:      in.DraftKind,
		Content:        in.Content,
	})
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	return draftToolOutput(draft), nil
}

// --- update_draft (draft tier, no confirmation, audited) ---

type updateDraftTool struct{ drafts vibeEvalDraftAuthor }

func (updateDraftTool) Name() string                { return "update_draft" }
func (updateDraftTool) Phases() []string            { return []string{vibeeval.PhasePlan, vibeeval.PhaseAuthor} }
func (updateDraftTool) RiskTier() vibeeval.RiskTier { return vibeeval.DraftTier }
func (updateDraftTool) RequiredAction() string      { return string(ActionManageVibeEvalDrafts) }
func (updateDraftTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "update_draft",
		Description: "Replace the content of an existing draft by its UUID.",
		Parameters:  json.RawMessage(`{"type":"object","required":["draft_id","content"],"properties":{"draft_id":{"type":"string","description":"draft UUID"},"content":{"type":"object","description":"the new draft body"}}}`),
	}
}

func (t updateDraftTool) Execute(ctx context.Context, _ vibeeval.Actor, conv vibeeval.Conversation, args json.RawMessage) (vibeeval.ToolOutput, error) {
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	var in struct {
		DraftID string          `json:"draft_id"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("invalid args: %w", err)
	}
	draftID, err := uuid.Parse(in.DraftID)
	if err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("draft_id is not a UUID: %w", err)
	}
	// The shared manager UpdateDraft is workspace-scoped; confine the AGENT to drafts of THIS
	// conversation, so it can't mutate another conversation's draft just by knowing the UUID.
	current, err := t.drafts.GetDraft(ctx, caller, GetVibeEvalDraftInput{WorkspaceID: conv.WorkspaceID, DraftID: draftID})
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	if current.ConversationID != conv.ID {
		return vibeeval.ToolOutput{}, repository.ErrVibeEvalDraftNotFound
	}
	draft, err := t.drafts.UpdateDraft(ctx, caller, UpdateVibeEvalDraftInput{
		WorkspaceID: conv.WorkspaceID,
		DraftID:     draftID,
		Content:     in.Content,
	})
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	return draftToolOutput(draft), nil
}

// --- validate_draft (draft tier, no confirmation, audited) ---

type validateDraftTool struct{ drafts vibeEvalDraftAuthor }

func (validateDraftTool) Name() string { return "validate_draft" }
func (validateDraftTool) Phases() []string {
	return []string{vibeeval.PhaseAuthor, vibeeval.PhaseValidate}
}
func (validateDraftTool) RiskTier() vibeeval.RiskTier { return vibeeval.DraftTier }
func (validateDraftTool) RequiredAction() string      { return string(ActionManageVibeEvalDrafts) }
func (validateDraftTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "validate_draft",
		Description: "Validate a challenge_pack draft's bundle and record its validation state and errors.",
		Parameters:  json.RawMessage(`{"type":"object","required":["draft_id"],"properties":{"draft_id":{"type":"string","description":"draft UUID"}}}`),
	}
}

func (t validateDraftTool) Execute(ctx context.Context, _ vibeeval.Actor, conv vibeeval.Conversation, args json.RawMessage) (vibeeval.ToolOutput, error) {
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	var in struct {
		DraftID string `json:"draft_id"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("invalid args: %w", err)
	}
	draftID, err := uuid.Parse(in.DraftID)
	if err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("draft_id is not a UUID: %w", err)
	}
	// Conversation locality: the agent may only validate drafts of THIS conversation.
	current, err := t.drafts.GetDraft(ctx, caller, GetVibeEvalDraftInput{WorkspaceID: conv.WorkspaceID, DraftID: draftID})
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	if current.ConversationID != conv.ID {
		return vibeeval.ToolOutput{}, repository.ErrVibeEvalDraftNotFound
	}
	res, err := t.drafts.ValidateDraft(ctx, caller, ValidateVibeEvalDraftInput{WorkspaceID: conv.WorkspaceID, DraftID: draftID})
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	return vibeeval.ToolOutput{
		// Structured validation metadata only — never the bundle content.
		Result:      map[string]any{"draft_id": res.Draft.ID.String(), "valid": res.Valid, "validation_state": res.Draft.ValidationState, "errors": res.Errors},
		AuditResult: map[string]any{"draft_id": res.Draft.ID.String(), "valid": res.Valid, "error_count": len(res.Errors)},
	}, nil
}

// --- get_run_status ---

type getRunStatusTool struct{ runs runStatusReader }

func (getRunStatusTool) Name() string                { return "get_run_status" }
func (getRunStatusTool) Phases() []string            { return []string{vibeeval.PhaseRun, vibeeval.PhaseAnalyze} }
func (getRunStatusTool) RiskTier() vibeeval.RiskTier { return vibeeval.ReadTier }
func (getRunStatusTool) RequiredAction() string      { return string(ActionReadWorkspace) }
func (getRunStatusTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "get_run_status",
		Description: "Get the status, mode, and timing of a run by its UUID.",
		Parameters:  json.RawMessage(`{"type":"object","required":["run_id"],"properties":{"run_id":{"type":"string","description":"run UUID"}}}`),
	}
}

func (t getRunStatusTool) Execute(ctx context.Context, _ vibeeval.Actor, _ vibeeval.Conversation, args json.RawMessage) (vibeeval.ToolOutput, error) {
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	var in struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("invalid args: %w", err)
	}
	runID, err := uuid.Parse(in.RunID)
	if err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("run_id is not a UUID: %w", err)
	}
	res, err := t.runs.GetRun(ctx, caller, runID)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	return vibeeval.ToolOutput{Result: res, AuditResult: map[string]any{"run_id": in.RunID}}, nil
}

// --- read_scorecard ---

type readScorecardTool struct{ scorecards scorecardReader }

func (readScorecardTool) Name() string                { return "read_scorecard" }
func (readScorecardTool) Phases() []string            { return []string{vibeeval.PhaseAnalyze} }
func (readScorecardTool) RiskTier() vibeeval.RiskTier { return vibeeval.ReadTier }
func (readScorecardTool) RequiredAction() string      { return string(ActionReadWorkspace) }
func (readScorecardTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "read_scorecard",
		Description: "Read the scorecard (dimensions, validators) for one run agent by its UUID.",
		Parameters:  json.RawMessage(`{"type":"object","required":["run_agent_id"],"properties":{"run_agent_id":{"type":"string","description":"run agent UUID"}}}`),
	}
}

func (t readScorecardTool) Execute(ctx context.Context, _ vibeeval.Actor, _ vibeeval.Conversation, args json.RawMessage) (vibeeval.ToolOutput, error) {
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	var in struct {
		RunAgentID string `json:"run_agent_id"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("invalid args: %w", err)
	}
	runAgentID, err := uuid.Parse(in.RunAgentID)
	if err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("run_agent_id is not a UUID: %w", err)
	}
	res, err := t.scorecards.GetRunAgentScorecard(ctx, caller, runAgentID)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	return vibeeval.ToolOutput{Result: res, AuditResult: map[string]any{"run_agent_id": in.RunAgentID}}, nil
}

// --- list_challenge_packs ---

type listChallengePacksTool struct{ packs challengePackLister }

func (listChallengePacksTool) Name() string { return "list_challenge_packs" }
func (listChallengePacksTool) Phases() []string {
	return []string{vibeeval.PhasePlan, vibeeval.PhaseAuthor}
}
func (listChallengePacksTool) RiskTier() vibeeval.RiskTier { return vibeeval.ReadTier }
func (listChallengePacksTool) RequiredAction() string      { return string(ActionReadWorkspace) }
func (listChallengePacksTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "list_challenge_packs",
		Description: "List the challenge packs visible in the current workspace, with their runnable versions.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func (t listChallengePacksTool) Execute(ctx context.Context, _ vibeeval.Actor, _ vibeeval.Conversation, _ json.RawMessage) (vibeeval.ToolOutput, error) {
	res, err := t.packs.ListChallengePacks(ctx)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	return vibeeval.ToolOutput{Result: res, AuditResult: map[string]any{"count": len(res.Packs)}}, nil
}
