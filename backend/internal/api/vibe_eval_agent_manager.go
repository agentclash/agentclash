package api

import (
	"context"
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

// VibeEvalAgentManager runs guide-agent turns. It owns the boundary wiring: it builds the
// vibeeval loop with a repo-backed MessageStore, the read-only tool adapters, the shared
// provider router, and the AgentClash-owned guide model — and bridges the authenticated
// api.Caller into the loop per turn. The vibeeval core never imports api (§11.1).
type VibeEvalAgentManager struct {
	authorizer WorkspaceAuthorizer
	repo       *repository.Repository
	loop       *vibeeval.AgentLoop
}

// NewVibeEvalAgentManager wires the read-only Step-2 agent. router is the api-server's
// shared provider.Router (reused, §Q4). It validates each tool's action string against the
// api.Action matrix at construction (fail-fast) before registering it.
func NewVibeEvalAgentManager(
	authorizer WorkspaceAuthorizer,
	repo *repository.Repository,
	router provider.Router,
	cfg VibeEvalGuideConfig,
	runs runStatusReader,
	scorecards scorecardReader,
	packs challengePackLister,
) (*VibeEvalAgentManager, error) {
	registry := vibeeval.NewRegistry()
	tools := []vibeeval.Tool{
		getRunStatusTool{runs: runs},
		readScorecardTool{scorecards: scorecards},
		listChallengePacksTool{packs: packs},
	}
	for _, t := range tools {
		if !roleAllows(RoleWorkspaceAdmin, Action(t.RequiredAction())) {
			return nil, fmt.Errorf("vibe-eval tool %q declares unknown action %q", t.Name(), t.RequiredAction())
		}
		registry.Register(t)
	}

	loop := vibeeval.NewAgentLoop(
		router,
		registry,
		vibeEvalMessageStore{repo: repo},
		vibeeval.NewEvidenceRedactor(),
		vibeeval.GuideModel{ProviderKey: cfg.ProviderKey, Model: cfg.Model, CredentialReference: cfg.CredentialReference},
		vibeeval.DefaultLimits(),
	)

	return &VibeEvalAgentManager{authorizer: authorizer, repo: repo, loop: loop}, nil
}

// AuthorizeTurn checks the caller may run a guide turn in the workspace (member+). The
// handler calls this BEFORE switching to SSE so a 403 returns as a normal HTTP error.
func (m *VibeEvalAgentManager) AuthorizeTurn(ctx context.Context, caller Caller, workspaceID uuid.UUID) error {
	return AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionManageVibeEvalDrafts)
}

// RunTurn loads the conversation, bridges the caller, and runs one bounded turn. The handler
// has already verified workspace access; the per-turn WorkspaceAuthorizer re-checks each
// tool's action against this caller.
func (m *VibeEvalAgentManager) RunTurn(ctx context.Context, caller Caller, workspaceID, conversationID uuid.UUID, userMessage string, sink vibeeval.EventSink) (vibeeval.TurnResult, error) {
	// Turn-level authz: running the guide is a member+ draft-tier interaction. Per-tool
	// authz happens again inside the loop via the WorkspaceAuthorizer bridge.
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionManageVibeEvalDrafts); err != nil {
		return vibeeval.TurnResult{}, err
	}

	conv, err := m.repo.GetVibeEvalConversationByID(ctx, conversationID)
	if err != nil {
		return vibeeval.TurnResult{}, err
	}
	if conv.WorkspaceID != workspaceID {
		return vibeeval.TurnResult{}, repository.ErrVibeEvalConversationNotFound
	}

	vconv := vibeeval.Conversation{
		ID:             conv.ID,
		WorkspaceID:    conv.WorkspaceID,
		OrganizationID: conv.OrganizationID,
		Phase:          conv.Phase,
	}
	authz := vibeEvalAuthorizer{authorizer: m.authorizer, caller: caller}
	actor := vibeeval.Actor{UserID: caller.UserID}
	return m.loop.RunTurn(ctx, actor, authz, vconv, userMessage, sink)
}

// AuthorizeRead checks the caller may read the workspace (viewer+). Used for the
// transcript listing.
func (m *VibeEvalAgentManager) AuthorizeRead(ctx context.Context, caller Caller, workspaceID uuid.UUID) error {
	return AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace)
}

// ListMessages returns a conversation's transcript, verifying the conversation belongs to
// the workspace.
func (m *VibeEvalAgentManager) ListMessages(ctx context.Context, workspaceID, conversationID uuid.UUID) ([]repository.VibeEvalMessage, error) {
	conv, err := m.repo.GetVibeEvalConversationByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conv.WorkspaceID != workspaceID {
		return nil, repository.ErrVibeEvalConversationNotFound
	}
	return m.repo.ListVibeEvalMessagesByConversationID(ctx, conversationID)
}
