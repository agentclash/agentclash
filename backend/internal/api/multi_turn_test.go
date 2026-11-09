package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type fakeMultiTurnService struct {
	noopMultiTurnService
	voteErr error
}

func (f fakeMultiTurnService) SubmitArenaVote(context.Context, Caller, uuid.UUID, ArenaVoteRequest) error {
	return f.voteErr
}

func TestWriteMultiTurnErrorMapsStateConflictsTo409(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "run agent not executing", err: ErrRunAgentNotExecuting, wantStatus: http.StatusConflict, wantCode: "run_agent_not_executing"},
		{name: "wrapped run agent not executing", err: fmt.Errorf("submit: %w", ErrRunAgentNotExecuting), wantStatus: http.StatusConflict, wantCode: "run_agent_not_executing"},
		{name: "arena task not pending", err: repository.ErrArenaTaskNotPending, wantStatus: http.StatusConflict, wantCode: "arena_task_not_pending"},
		{name: "arena task not found", err: repository.ErrArenaTaskNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "human turn not awaiting", err: repository.ErrHumanTurnNotAwaiting, wantStatus: http.StatusConflict, wantCode: "not_awaiting_human"},
		{name: "unknown error", err: errors.New("boom"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			writeMultiTurnError(rr, slog.Default(), req, tc.err)
			assertMultiTurnError(t, rr, tc.wantStatus, tc.wantCode)
		})
	}
}

func TestSubmitArenaVoteHandlerReturnsConflictForCompletedTask(t *testing.T) {
	workspaceID := uuid.New()
	service := fakeMultiTurnService{voteErr: repository.ErrArenaTaskNotPending}
	router := chi.NewRouter()
	router.Post("/v1/workspaces/{workspaceID}/arena/votes", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), callerContextKey{}, callerWithWorkspace(workspaceID))
		submitArenaVoteHandler(slog.Default(), service).ServeHTTP(w, r.WithContext(ctx))
	})
	body := fmt.Sprintf(`{"task_id":%q,"winner_run_agent_id":%q}`, uuid.New(), uuid.New())
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/arena/votes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assertMultiTurnError(t, rr, http.StatusConflict, "arena_task_not_pending")
}

func assertMultiTurnError(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rr.Code != wantStatus {
		t.Fatalf("status = %d body=%s, want %d", rr.Code, rr.Body.String(), wantStatus)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if envelope.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", envelope.Error.Code, wantCode)
	}
}
