package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/provider"
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

func (listChallengePacksTool) Name() string                { return "list_challenge_packs" }
func (listChallengePacksTool) Phases() []string            { return []string{vibeeval.PhasePlan, vibeeval.PhaseAuthor} }
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
