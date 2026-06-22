package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

// VibeEvalAgentManager runs guide-agent turns. It owns the boundary wiring: it builds the
// vibeeval loop with a repo-backed MessageStore, the read-only tool adapters, the shared
// provider router, and the AgentClash-owned guide model — and bridges the authenticated
// api.Caller into the loop per turn. The vibeeval core never imports api (§11.1).
// vibeEvalLoopRunner is the slice of *vibeeval.AgentLoop the manager drives. Kept as an interface
// so the resolve/no-double-execute decision logic is unit-testable with a fake loop + real repo
// (provider.Router is a concrete struct, so the real loop can't run scripted models in a test).
type vibeEvalLoopRunner interface {
	RunTurn(ctx context.Context, actor vibeeval.Actor, authorizer vibeeval.WorkspaceAuthorizer, conv vibeeval.Conversation, userMessage string, sink vibeeval.EventSink) (vibeeval.TurnResult, error)
	ResumeConfirmedTurn(ctx context.Context, actor vibeeval.Actor, authorizer vibeeval.WorkspaceAuthorizer, conv vibeeval.Conversation, pc vibeeval.PendingConfirmation, approve bool, sink vibeeval.EventSink) (vibeeval.TurnResult, error)
	ContinueTurn(ctx context.Context, actor vibeeval.Actor, authorizer vibeeval.WorkspaceAuthorizer, conv vibeeval.Conversation, sink vibeeval.EventSink) (vibeeval.TurnResult, error)
}

// guideTurnMeter atomically consumes one guide-agent turn against the workspace's monthly allowance (4e),
// returning billingpkg.GateError when exhausted. BillingManager satisfies it. Kept narrow so the vibeeval
// manager depends only on the metering capability, not the whole billing surface.
type guideTurnMeter interface {
	ConsumeGuideAgentTurn(ctx context.Context, workspaceID uuid.UUID) error
}

type VibeEvalAgentManager struct {
	authorizer     WorkspaceAuthorizer
	repo           *repository.Repository
	loop           vibeEvalLoopRunner
	meter          guideTurnMeter
	now            func() time.Time
	recoveryProbes map[string]recoveryProbe // per-tool-name ambiguous-recovery effect probes
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
	drafts vibeEvalDraftAuthor,
	runCreator vibeEvalRunCreator,
	sessionCreator vibeEvalSessionCreator,
	meter guideTurnMeter,
) (*VibeEvalAgentManager, error) {
	// The guide-turn allowance meter is required in production (every fresh turn / approve is metered).
	// Fail fast rather than risk a nil-pointer at the first turn.
	if meter == nil {
		return nil, fmt.Errorf("vibe-eval guide-turn meter is required")
	}
	registry := vibeeval.NewRegistry()
	tools := []vibeeval.Tool{
		getRunStatusTool{runs: runs},
		readScorecardTool{scorecards: scorecards},
		listChallengePacksTool{packs: packs},
		createDraftTool{drafts: drafts},
		updateDraftTool{drafts: drafts},
		validateDraftTool{drafts: drafts},
		publishDraftTool{drafts: drafts},
		estimateEvalCostTool{estimator: runCreator},
		createRunTool{runs: runCreator},
		createEvalSessionTool{sessions: sessionCreator},
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
	).
		WithConfirmationStore(vibeEvalConfirmationStore{repo: repo}).
		WithAuditWriter(vibeEvalAuditWriter{repo: repo})

	mgr := &VibeEvalAgentManager{authorizer: authorizer, repo: repo, loop: loop, meter: meter, now: time.Now}
	// publish_draft is idempotent by effect identity → its ambiguous-recovery probe consults the
	// draft's published version rather than re-executing.
	mgr.recoveryProbes = map[string]recoveryProbe{
		"publish_draft": mgr.publishEffectSucceeded,
	}
	return mgr, nil
}

// AuthorizeTurn checks the caller may run a guide turn in the workspace (member+). The
// handler calls this BEFORE switching to SSE so a 403 returns as a normal HTTP error.
func (m *VibeEvalAgentManager) AuthorizeTurn(ctx context.Context, caller Caller, workspaceID uuid.UUID) error {
	return AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionManageVibeEvalDrafts)
}

// RequireConversation verifies the conversation exists and belongs to the workspace. The create-turn
// handler calls this BEFORE metering (4e blocker): otherwise a caller authorized for the workspace could
// POST to a missing or cross-workspace conversation and burn a guide-agent turn for a non-genuine turn.
// RunTurn re-checks the same locality (a cheap duplicate read) before it runs the model.
func (m *VibeEvalAgentManager) RequireConversation(ctx context.Context, workspaceID, conversationID uuid.UUID) error {
	conv, err := m.repo.GetVibeEvalConversationByID(ctx, conversationID)
	if err != nil {
		return err
	}
	if conv.WorkspaceID != workspaceID {
		return repository.ErrVibeEvalConversationNotFound
	}
	return nil
}

// MeterFreshTurn consumes one guide-agent turn allowance for a fresh user turn (4e). The handler calls
// this BEFORE the SSE switch (and AFTER RequireConversation), so an exhausted allowance returns a clean
// 402 (billingpkg.GateError) with no model call. A turn that passes is counted even if it later errors
// mid-stream (consumed at accept-time).
func (m *VibeEvalAgentManager) MeterFreshTurn(ctx context.Context, workspaceID uuid.UUID) error {
	return m.meter.ConsumeGuideAgentTurn(ctx, workspaceID)
}

// MeterConfirmationResolve consumes one guide-agent turn allowance ONLY for a genuine fresh APPROVE
// resume (4e, B1-adjusted + D): deny is always allowed and never counted; an approve for a non-resolvable
// confirmation (wrong payload hash, expired, or already resolved/executing) is never counted, since it
// will not drive a model call. The genuineness predicate is the pre-SSE check; the atomic consume
// provides the 402. A rare resolve-retry that races the claim may over-count by one (accepted).
func (m *VibeEvalAgentManager) MeterConfirmationResolve(ctx context.Context, workspaceID uuid.UUID, pc repository.VibeEvalPendingConfirmation, presentedHash string, approve bool) error {
	if !approve {
		return nil
	}
	if pc.Status != "pending" || pc.PayloadHash != presentedHash || !m.now().UTC().Before(pc.ExpiresAt.UTC()) {
		return nil
	}
	return m.meter.ConsumeGuideAgentTurn(ctx, workspaceID)
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

// LoadConfirmationForResolve loads the conversation + pending confirmation, verifies both belong to
// the workspace, and authorizes the confirmation's BOUND action for the caller. The handler calls
// this BEFORE switching to SSE, so a not-found/forbidden returns as a normal HTTP error.
func (m *VibeEvalAgentManager) LoadConfirmationForResolve(ctx context.Context, caller Caller, workspaceID, conversationID, confirmationID uuid.UUID) (repository.VibeEvalConversation, repository.VibeEvalPendingConfirmation, error) {
	conv, err := m.repo.GetVibeEvalConversationByID(ctx, conversationID)
	if err != nil {
		return repository.VibeEvalConversation{}, repository.VibeEvalPendingConfirmation{}, err
	}
	if conv.WorkspaceID != workspaceID {
		return repository.VibeEvalConversation{}, repository.VibeEvalPendingConfirmation{}, repository.ErrVibeEvalConversationNotFound
	}
	pc, err := m.repo.GetVibeEvalPendingConfirmationByID(ctx, confirmationID)
	if err != nil {
		return repository.VibeEvalConversation{}, repository.VibeEvalPendingConfirmation{}, err
	}
	if pc.WorkspaceID != workspaceID || pc.ConversationID != conversationID {
		return repository.VibeEvalConversation{}, repository.VibeEvalPendingConfirmation{}, repository.ErrVibeEvalConfirmationNotFound
	}
	// Authorize the action the confirmation will perform (e.g. publish_challenge_pack), not just the
	// turn-level manage-drafts gate.
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, Action(pc.Action)); err != nil {
		return repository.VibeEvalConversation{}, repository.VibeEvalPendingConfirmation{}, err
	}
	return conv, pc, nil
}

// ResolveConfirmation atomically approves or denies a pending confirmation and streams the
// continuation turn. On approve it follows the crash-safe retry algorithm: claim (pending →
// executing); if already claimed, re-enter ONLY if the effect has not already run (no double
// execute) — otherwise finalize and continue. conv/pc come from LoadConfirmationForResolve.
func (m *VibeEvalAgentManager) ResolveConfirmation(ctx context.Context, caller Caller, conv repository.VibeEvalConversation, pc repository.VibeEvalPendingConfirmation, approve bool, presentedHash string, sink vibeeval.EventSink) (vibeeval.TurnResult, error) {
	vconv := vibeeval.Conversation{ID: conv.ID, WorkspaceID: conv.WorkspaceID, OrganizationID: conv.OrganizationID, Phase: conv.Phase}
	authz := vibeEvalAuthorizer{authorizer: m.authorizer, caller: caller}
	actor := vibeeval.Actor{UserID: caller.UserID}

	if !approve {
		denied, err := m.repo.DenyVibeEvalPendingConfirmation(ctx, pc.ID, presentedHash, caller.UserID)
		if err != nil {
			return vibeeval.TurnResult{}, err // ErrVibeEvalConfirmationNotResolvable → handler 409
		}
		return m.loop.ResumeConfirmedTurn(ctx, actor, authz, vconv, toVibeevalPendingConfirmation(denied), false, sink)
	}

	// Approve: atomic claim pending → executing.
	approved, err := m.repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, presentedHash, caller.UserID)
	if err == nil {
		return m.loop.ResumeConfirmedTurn(ctx, actor, authz, vconv, toVibeevalPendingConfirmation(approved), true, sink)
	}
	if !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
		return vibeeval.TurnResult{}, err
	}

	// Not freshly resolvable: a prior attempt may have claimed it. Re-enter ONLY if it is still
	// executing for the same payload hash; otherwise it is denied/succeeded/expired/mismatch → reject.
	executing, gerr := m.repo.GetVibeEvalPendingConfirmationForResume(ctx, pc.ID, presentedHash)
	if gerr != nil {
		return vibeeval.TurnResult{}, gerr
	}
	// No-double-execute guard: if the confirmed effect already ran, do NOT re-run it — finalize
	// OUTCOME-AWARELY (never promote a failed/ambiguous effect to succeeded) and let the model
	// respond over the existing history.
	outcome, found, err := m.repo.GetVibeEvalConfirmedToolOutcome(ctx, pc.ID)
	if err != nil {
		return vibeeval.TurnResult{}, err
	}
	if found {
		if err := m.finalizeRanConfirmation(ctx, pc.ID, outcome == "ok"); err != nil {
			return vibeeval.TurnResult{}, err
		}
		return m.loop.ContinueTurn(ctx, actor, authz, vconv, sink)
	}
	// No durable audit outcome. Fall back to message-only evidence (weaker): if a tool-result for
	// the confirmed call exists, the effect ran but the outcome is unknown (lost audit write) —
	// treat as ambiguous: finalize as FAILED (never succeeded) and do NOT re-execute.
	ran, err := m.confirmedToolResultExists(ctx, conv.ID, executing.ToolCallID)
	if err != nil {
		return vibeeval.TurnResult{}, err
	}
	if ran {
		// Narrow per-tool recovery probe: an idempotent write tool (e.g. publish_draft) can confirm
		// its effect succeeded from durable effect state — a stronger signal than the lost audit row.
		// It never re-executes (re-entering ResumeConfirmedTurn would duplicate the tool_call_id in
		// the transcript). Unknown → keep the conservative failed fallback.
		succeeded := false
		if probe, ok := m.recoveryProbes[executing.ToolName]; ok {
			s, perr := probe(ctx, executing)
			if perr != nil {
				return vibeeval.TurnResult{}, perr
			}
			succeeded = s
		}
		if err := m.finalizeRanConfirmation(ctx, pc.ID, succeeded); err != nil {
			return vibeeval.TurnResult{}, err
		}
		return m.loop.ContinueTurn(ctx, actor, authz, vconv, sink)
	}
	// Claimed but the effect never ran (crash before execution) → safe to execute now.
	return m.loop.ResumeConfirmedTurn(ctx, actor, authz, vconv, toVibeevalPendingConfirmation(executing), true, sink)
}

// recoveryProbe reports whether an ambiguous (executing, no audit row) confirmation's effect actually
// succeeded, by consulting the tool's durable effect state. (false, nil) means "unknown" → the caller
// keeps the conservative failed fallback.
type recoveryProbe func(ctx context.Context, pc repository.VibeEvalPendingConfirmation) (bool, error)

// publishEffectSucceeded is the publish_draft recovery probe: the effect succeeded iff the bound
// draft now records a published challenge-pack version (effect identity) AND belongs to THIS
// confirmation's conversation+workspace — the same locality the live tool path enforces, so a bound
// draft_id from another conversation/workspace can never be recovered as success here.
func (m *VibeEvalAgentManager) publishEffectSucceeded(ctx context.Context, pc repository.VibeEvalPendingConfirmation) (bool, error) {
	var args struct {
		DraftID string `json:"draft_id"`
	}
	if err := json.Unmarshal(pc.BoundArgs, &args); err != nil {
		return false, nil
	}
	draftID, err := uuid.Parse(args.DraftID)
	if err != nil {
		return false, nil
	}
	draft, err := m.repo.GetVibeEvalDraftByID(ctx, draftID)
	if err != nil {
		return false, nil
	}
	if draft.ConversationID != pc.ConversationID || draft.WorkspaceID != pc.WorkspaceID {
		return false, nil
	}
	return draft.PublishedChallengePackVersionID != nil, nil
}

// finalizeRanConfirmation marks an already-executed confirmation succeeded or failed. A
// not-resolvable result is benign (a concurrent finalizer won the terminal transition).
func (m *VibeEvalAgentManager) finalizeRanConfirmation(ctx context.Context, id uuid.UUID, succeeded bool) error {
	var err error
	if succeeded {
		_, err = m.repo.MarkVibeEvalPendingConfirmationSucceeded(ctx, id)
	} else {
		_, err = m.repo.MarkVibeEvalPendingConfirmationFailed(ctx, id)
	}
	if err != nil && !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
		return err
	}
	return nil
}

// confirmedToolResultExists reports whether a tool-result message exists for toolCallID.
func (m *VibeEvalAgentManager) confirmedToolResultExists(ctx context.Context, conversationID uuid.UUID, toolCallID string) (bool, error) {
	msgs, err := m.repo.ListVibeEvalMessagesByConversationID(ctx, conversationID)
	if err != nil {
		return false, err
	}
	for _, msg := range msgs {
		if msg.Role == "tool" && msg.ToolCallID == toolCallID {
			return true, nil
		}
	}
	return false, nil
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
