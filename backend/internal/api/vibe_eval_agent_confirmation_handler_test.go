package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Finding 2: `approve` is required — a body omitting it must NOT silently deny. No DB needed: the
// validation happens before any manager call.
func TestResolveVibeEvalConfirmationRequiresApprove(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"payload_hash":"abc"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, Caller{UserID: uuid.New()}))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workspaceID", uuid.NewString())
	rctx.URLParams.Add("conversationID", uuid.NewString())
	rctx.URLParams.Add("confirmationID", uuid.NewString())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	resolveVibeEvalConfirmationHandler(slog.Default(), &VibeEvalAgentManager{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "approve is required") {
		t.Fatalf("body = %s, want approve-required validation error", rec.Body.String())
	}
}
