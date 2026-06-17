package api

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Manager-level resolve matrix for Step 3b-2, incl. the no-double-execute guard (#875 §5.3/§5.4).
//
// WARNING — DESTRUCTIVE: seeds by TRUNCATEing organizations/users CASCADE. Point DATABASE_URL at a
// DISPOSABLE database (throwaway container), never a dev/shared DB. Skips when DATABASE_URL unset.

type fakeResolveLoop struct {
	resumeApprovals []bool
	continueCalls   int
}

func (f *fakeResolveLoop) RunTurn(context.Context, vibeeval.Actor, vibeeval.WorkspaceAuthorizer, vibeeval.Conversation, string, vibeeval.EventSink) (vibeeval.TurnResult, error) {
	return vibeeval.TurnResult{}, nil
}
func (f *fakeResolveLoop) ResumeConfirmedTurn(_ context.Context, _ vibeeval.Actor, _ vibeeval.WorkspaceAuthorizer, _ vibeeval.Conversation, _ vibeeval.PendingConfirmation, approve bool, _ vibeeval.EventSink) (vibeeval.TurnResult, error) {
	f.resumeApprovals = append(f.resumeApprovals, approve)
	return vibeeval.TurnResult{StopReason: "completed"}, nil
}
func (f *fakeResolveLoop) ContinueTurn(context.Context, vibeeval.Actor, vibeeval.WorkspaceAuthorizer, vibeeval.Conversation, vibeeval.EventSink) (vibeeval.TurnResult, error) {
	f.continueCalls++
	return vibeeval.TurnResult{StopReason: "completed"}, nil
}

func openVibeEvalConfirmTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	db, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func seedVibeEvalConfirmFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) (org, ws, user uuid.UUID) {
	t.Helper()
	if _, err := db.Exec(ctx, "TRUNCATE TABLE organizations, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	org, ws, user = uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1,$2,$3)`, org, "Org", "org-"+org.String()[:8]); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO workspaces (id, organization_id, name, slug) VALUES ($1,$2,$3,$4)`, ws, org, "WS", "ws-"+ws.String()[:8]); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users (id, workos_user_id, email, display_name) VALUES ($1,$2,$3,$4)`, user, "wu-"+user.String()[:8], user.String()[:8]+"@example.com", "U"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return org, ws, user
}

func TestVibeEvalManagerResolveConfirmationMatrix(t *testing.T) {
	ctx := context.Background()
	db := openVibeEvalConfirmTestDB(t)
	org, ws, user := seedVibeEvalConfirmFixture(t, ctx, db)
	repo := repository.New(db)

	conv, err := repo.CreateVibeEvalConversation(ctx, repository.CreateVibeEvalConversationParams{
		OrganizationID: org, WorkspaceID: ws, CreatedByUserID: user, Title: "c", Phase: "author", Status: "active",
	})
	if err != nil {
		t.Fatalf("CreateVibeEvalConversation: %v", err)
	}
	caller := Caller{UserID: user}

	newPending := func(hash, toolCallID string) repository.VibeEvalPendingConfirmation {
		pc, err := repo.CreateVibeEvalPendingConfirmation(ctx, repository.CreateVibeEvalPendingConfirmationParams{
			OrganizationID: org, WorkspaceID: ws, ConversationID: conv.ID, ProposedByUserID: user,
			ToolName: "publish_draft", ToolCallID: toolCallID, Action: "publish_challenge_pack",
			RiskTier: "workspace_write", PayloadHash: hash, Summary: "s", ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateVibeEvalPendingConfirmation: %v", err)
		}
		return pc
	}
	mgrWith := func(loop vibeEvalLoopRunner) *VibeEvalAgentManager {
		return &VibeEvalAgentManager{repo: repo, loop: loop}
	}

	t.Run("approve fresh resumes with execute", func(t *testing.T) {
		f := &fakeResolveLoop{}
		pc := newPending("h-fresh", "tc-fresh")
		if _, err := mgrWith(f).ResolveConfirmation(ctx, caller, conv, pc, true, "h-fresh", nil); err != nil {
			t.Fatalf("ResolveConfirmation: %v", err)
		}
		if len(f.resumeApprovals) != 1 || !f.resumeApprovals[0] || f.continueCalls != 0 {
			t.Fatalf("want one ResumeConfirmedTurn(approve=true), got resume=%v continue=%d", f.resumeApprovals, f.continueCalls)
		}
	})

	t.Run("deny resumes with deny", func(t *testing.T) {
		f := &fakeResolveLoop{}
		pc := newPending("h-deny", "tc-deny")
		if _, err := mgrWith(f).ResolveConfirmation(ctx, caller, conv, pc, false, "h-deny", nil); err != nil {
			t.Fatalf("deny: %v", err)
		}
		if len(f.resumeApprovals) != 1 || f.resumeApprovals[0] {
			t.Fatalf("want one ResumeConfirmedTurn(approve=false), got %v", f.resumeApprovals)
		}
	})

	t.Run("hash mismatch rejects with no loop call", func(t *testing.T) {
		f := &fakeResolveLoop{}
		pc := newPending("h-real", "tc-mismatch")
		_, err := mgrWith(f).ResolveConfirmation(ctx, caller, conv, pc, true, "h-WRONG", nil)
		if !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
			t.Fatalf("err = %v, want NotResolvable", err)
		}
		if len(f.resumeApprovals) != 0 || f.continueCalls != 0 {
			t.Fatal("no loop call expected on hash mismatch")
		}
	})

	t.Run("already denied rejects", func(t *testing.T) {
		f := &fakeResolveLoop{}
		pc := newPending("h-2x", "tc-2x")
		if _, err := repo.DenyVibeEvalPendingConfirmation(ctx, pc.ID, "h-2x", user); err != nil {
			t.Fatalf("pre-deny: %v", err)
		}
		_, err := mgrWith(f).ResolveConfirmation(ctx, caller, conv, pc, true, "h-2x", nil)
		if !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
			t.Fatalf("err = %v, want NotResolvable", err)
		}
	})

	t.Run("claimed but effect not run re-executes", func(t *testing.T) {
		f := &fakeResolveLoop{}
		pc := newPending("h-claim", "tc-claim")
		if _, err := repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, "h-claim", user); err != nil {
			t.Fatalf("pre-approve: %v", err)
		}
		// No tool-result message exists → effect never ran → re-execute.
		if _, err := mgrWith(f).ResolveConfirmation(ctx, caller, conv, pc, true, "h-claim", nil); err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if len(f.resumeApprovals) != 1 || !f.resumeApprovals[0] || f.continueCalls != 0 {
			t.Fatalf("want re-execute via ResumeConfirmedTurn, got resume=%v continue=%d", f.resumeApprovals, f.continueCalls)
		}
	})

	// appendExecutionAudit records an ok/error audit row for the confirmed tool (the durable
	// outcome evidence), plus its tool-result message — simulating a prior attempt that executed.
	appendExecutionAudit := func(t *testing.T, pc repository.VibeEvalPendingConfirmation, outcome string) {
		t.Helper()
		if _, err := repo.AppendVibeEvalMessage(ctx, repository.AppendVibeEvalMessageParams{
			ConversationID: conv.ID, Role: "tool", Content: "evidence", RedactionState: "applied", ToolCallID: pc.ToolCallID, ToolName: pc.ToolName,
		}); err != nil {
			t.Fatalf("append tool result: %v", err)
		}
		cid := pc.ID
		if _, err := repo.AppendVibeEvalToolInvocation(ctx, repository.AppendVibeEvalToolInvocationParams{
			OrganizationID: org, WorkspaceID: ws, ConversationID: conv.ID, ActorUserID: user,
			ToolName: pc.ToolName, Action: pc.Action, RiskTier: "workspace_write", PayloadHash: pc.PayloadHash,
			ConfirmationID: &cid, Outcome: outcome,
		}); err != nil {
			t.Fatalf("append audit: %v", err)
		}
	}
	assertFinalizeNoReExecute := func(t *testing.T, f *fakeResolveLoop, pcID uuid.UUID, wantStatus string) {
		t.Helper()
		if f.continueCalls != 1 || len(f.resumeApprovals) != 0 {
			t.Fatalf("double-execute guard failed: continue=%d resume=%v (want continue=1, no resume)", f.continueCalls, f.resumeApprovals)
		}
		reloaded, err := repo.GetVibeEvalPendingConfirmationByID(ctx, pcID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if reloaded.Status != wantStatus {
			t.Fatalf("status = %q, want %q", reloaded.Status, wantStatus)
		}
	}

	t.Run("no double execute, ok outcome finalizes succeeded", func(t *testing.T) {
		f := &fakeResolveLoop{}
		pc := newPending("h-ok", "tc-ok")
		if _, err := repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, "h-ok", user); err != nil {
			t.Fatalf("pre-approve: %v", err)
		}
		appendExecutionAudit(t, pc, "ok")
		if _, err := mgrWith(f).ResolveConfirmation(ctx, caller, conv, pc, true, "h-ok", nil); err != nil {
			t.Fatalf("resolve: %v", err)
		}
		assertFinalizeNoReExecute(t, f, pc.ID, "succeeded")
	})

	t.Run("no double execute, error outcome finalizes failed not succeeded", func(t *testing.T) {
		f := &fakeResolveLoop{}
		pc := newPending("h-err", "tc-err")
		if _, err := repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, "h-err", user); err != nil {
			t.Fatalf("pre-approve: %v", err)
		}
		appendExecutionAudit(t, pc, "error")
		if _, err := mgrWith(f).ResolveConfirmation(ctx, caller, conv, pc, true, "h-err", nil); err != nil {
			t.Fatalf("resolve: %v", err)
		}
		// A failed effect must NOT be promoted to succeeded.
		assertFinalizeNoReExecute(t, f, pc.ID, "failed")
	})

	t.Run("ambiguous message-only evidence finalizes failed", func(t *testing.T) {
		f := &fakeResolveLoop{}
		pc := newPending("h-amb", "tc-amb")
		if _, err := repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, "h-amb", user); err != nil {
			t.Fatalf("pre-approve: %v", err)
		}
		// Tool-result message exists but NO audit outcome row (lost best-effort audit write).
		if _, err := repo.AppendVibeEvalMessage(ctx, repository.AppendVibeEvalMessageParams{
			ConversationID: conv.ID, Role: "tool", Content: "evidence", RedactionState: "applied", ToolCallID: "tc-amb", ToolName: "publish_draft",
		}); err != nil {
			t.Fatalf("append tool result: %v", err)
		}
		if _, err := mgrWith(f).ResolveConfirmation(ctx, caller, conv, pc, true, "h-amb", nil); err != nil {
			t.Fatalf("resolve: %v", err)
		}
		// Ran (message exists) but outcome unknown → conservative failed, never succeeded; no re-exec.
		assertFinalizeNoReExecute(t, f, pc.ID, "failed")
	})
}

type allowAccessAuthorizer struct{}

func (allowAccessAuthorizer) AuthorizeWorkspace(context.Context, Caller, uuid.UUID) error {
	return nil
}

// Codex gate: a caller who can enter the workspace but lacks the bound action gets forbidden BEFORE
// SSE and before any resolve transition.
func TestVibeEvalManagerForbiddenResolve(t *testing.T) {
	ctx := context.Background()
	db := openVibeEvalConfirmTestDB(t)
	org, ws, user := seedVibeEvalConfirmFixture(t, ctx, db)
	repo := repository.New(db)
	conv, err := repo.CreateVibeEvalConversation(ctx, repository.CreateVibeEvalConversationParams{
		OrganizationID: org, WorkspaceID: ws, CreatedByUserID: user, Title: "c", Phase: "author", Status: "active",
	})
	if err != nil {
		t.Fatalf("CreateVibeEvalConversation: %v", err)
	}
	pc, err := repo.CreateVibeEvalPendingConfirmation(ctx, repository.CreateVibeEvalPendingConfirmationParams{
		OrganizationID: org, WorkspaceID: ws, ConversationID: conv.ID, ProposedByUserID: user,
		ToolName: "publish_draft", ToolCallID: "tc-f", Action: "publish_challenge_pack",
		RiskTier: "workspace_write", PayloadHash: "h-f", Summary: "s", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateVibeEvalPendingConfirmation: %v", err)
	}

	mgr := &VibeEvalAgentManager{authorizer: allowAccessAuthorizer{}, repo: repo}
	// Viewer can enter the workspace but cannot publish_challenge_pack.
	viewer := Caller{UserID: user, WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{ws: {WorkspaceID: ws, Role: RoleWorkspaceViewer}}}

	if _, _, err := mgr.LoadConfirmationForResolve(ctx, viewer, ws, conv.ID, pc.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	// No resolve transition happened — the confirmation is untouched.
	reloaded, err := repo.GetVibeEvalPendingConfirmationByID(ctx, pc.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != "pending" {
		t.Fatalf("status = %q, want pending (no transition on forbidden)", reloaded.Status)
	}
}
