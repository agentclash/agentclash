package api

import (
	"context"
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

// DB-backed create-turn handler tests for the 4e allowance: the conversation guard runs BEFORE metering,
// so an invalid conversation never burns a turn, and a valid conversation with an exhausted allowance
// returns a clean pre-SSE 402. Skips when DATABASE_URL is unset (seeds via TRUNCATE — throwaway DB only).

func postTurn(mgr *VibeEvalAgentManager, caller Caller, workspaceID, conversationID uuid.UUID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, caller))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workspaceID", workspaceID.String())
	rctx.URLParams.Add("conversationID", conversationID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	createVibeEvalTurnHandler(slog.Default(), mgr).ServeHTTP(rec, req)
	return rec
}

func TestCreateTurnHandler_AllowanceAndConversationGuard(t *testing.T) {
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
	caller := vibeEvalCaller(user, ws, "workspace_admin")
	mgrWith := func(meter *fakeGuideMeter) *VibeEvalAgentManager {
		return &VibeEvalAgentManager{authorizer: fakeVibeEvalAuthorizer{}, repo: repo, meter: meter, now: time.Now}
	}

	t.Run("exhausted allowance on a valid conversation returns pre-SSE 402", func(t *testing.T) {
		meter := &fakeGuideMeter{err: quotaGateError()}
		rec := postTurn(mgrWith(meter), caller, ws, conv.ID)
		if rec.Code != http.StatusPaymentRequired {
			t.Fatalf("status = %d, want 402; body=%s", rec.Code, rec.Body.String())
		}
		if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
			t.Fatalf("content-type = %q, want a normal JSON error (no SSE switch)", rec.Header().Get("Content-Type"))
		}
		if meter.calls != 1 {
			t.Fatalf("meter calls = %d, want 1 (valid conversation meters once)", meter.calls)
		}
	})

	t.Run("missing conversation does not meter", func(t *testing.T) {
		meter := &fakeGuideMeter{}
		rec := postTurn(mgrWith(meter), caller, ws, uuid.New()) // conversation that does not exist
		if rec.Code == http.StatusOK || rec.Code == http.StatusPaymentRequired {
			t.Fatalf("status = %d, want a normal error (not 200/402)", rec.Code)
		}
		if meter.calls != 0 {
			t.Fatalf("meter calls = %d, want 0 (a missing conversation must not burn a turn)", meter.calls)
		}
	})

	t.Run("cross-workspace conversation does not meter", func(t *testing.T) {
		otherWS := uuid.New()
		otherCaller := vibeEvalCaller(user, otherWS, "workspace_admin") // authorized for otherWS, not ws
		meter := &fakeGuideMeter{}
		// conv belongs to ws; the request targets otherWS — RequireConversation rejects the locality.
		rec := postTurn(mgrWith(meter), otherCaller, otherWS, conv.ID)
		if rec.Code == http.StatusOK || rec.Code == http.StatusPaymentRequired {
			t.Fatalf("status = %d, want a normal error (not 200/402)", rec.Code)
		}
		if meter.calls != 0 {
			t.Fatalf("meter calls = %d, want 0 (a cross-workspace conversation must not burn a turn)", meter.calls)
		}
	})
}
