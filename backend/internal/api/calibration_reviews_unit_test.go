package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type fakeCalibrationReviewRepository struct {
	multiTurnRepository
	runAgents      map[uuid.UUID]domain.RunAgent
	runs           map[uuid.UUID]domain.Run
	completedTurns map[uuid.UUID]int32
	inserts        []repository.CreateCalibrationReviewParams
}

func (f *fakeCalibrationReviewRepository) GetRunAgentByID(_ context.Context, id uuid.UUID) (domain.RunAgent, error) {
	agent, ok := f.runAgents[id]
	if !ok {
		return domain.RunAgent{}, repository.ErrRunAgentNotFound
	}
	return agent, nil
}

func (f *fakeCalibrationReviewRepository) GetRunByID(_ context.Context, id uuid.UUID) (domain.Run, error) {
	run, ok := f.runs[id]
	if !ok {
		return domain.Run{}, repository.ErrRunNotFound
	}
	return run, nil
}

func (f *fakeCalibrationReviewRepository) HasCompletedCalibrationTurn(_ context.Context, runAgentID uuid.UUID, turnIndex int32) (bool, error) {
	completedTurn, ok := f.completedTurns[runAgentID]
	return ok && turnIndex == completedTurn, nil
}

func (f *fakeCalibrationReviewRepository) CreateCalibrationReview(_ context.Context, params repository.CreateCalibrationReviewParams) error {
	f.inserts = append(f.inserts, params)
	return nil
}

func TestCalibrationReviewCreateWithoutDatabase(t *testing.T) {
	workspaceID, foreignWorkspaceID := uuid.New(), uuid.New()
	runID, foreignRunID := uuid.New(), uuid.New()
	agentID, foreignAgentID := uuid.New(), uuid.New()
	zeroTurnAgentID := uuid.New()
	caller := callerWithWorkspaceRole(workspaceID, RoleWorkspaceMember)
	const completedTurn = 2
	const validScore = 4.5

	requestWithTurnField := func(t *testing.T, runAgentID uuid.UUID, turnField string, score float64) (*httptest.ResponseRecorder, []repository.CreateCalibrationReviewParams) {
		t.Helper()
		repo := &fakeCalibrationReviewRepository{
			runAgents: map[uuid.UUID]domain.RunAgent{
				agentID:         {RunID: runID},
				foreignAgentID:  {RunID: foreignRunID},
				zeroTurnAgentID: {RunID: runID},
			},
			runs: map[uuid.UUID]domain.Run{
				runID:        {WorkspaceID: workspaceID},
				foreignRunID: {WorkspaceID: foreignWorkspaceID},
			},
			completedTurns: map[uuid.UUID]int32{agentID: completedTurn, foreignAgentID: completedTurn, zeroTurnAgentID: 0},
		}
		manager := NewMultiTurnManager(NewCallerWorkspaceAuthorizer(), repo, nil)
		router := chi.NewRouter()
		router.Post("/v1/workspaces/{workspaceID}/calibration-reviews", createCalibrationReviewHandler(slog.Default(), manager))
		body := fmt.Sprintf(`{"run_agent_id":%q,%s"score":%g,"rubric_key":"recovery","notes":"reviewed"}`, runAgentID, turnField, score)
		ctx := context.WithValue(t.Context(), callerContextKey{}, caller)
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/calibration-reviews", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr, repo.inserts
	}
	request := func(t *testing.T, runAgentID uuid.UUID, turnIndex int64, score float64) (*httptest.ResponseRecorder, []repository.CreateCalibrationReviewParams) {
		t.Helper()
		return requestWithTurnField(t, runAgentID, fmt.Sprintf(`"turn_index":%d,`, turnIndex), score)
	}

	t.Run("success", func(t *testing.T) {
		rr, inserts := request(t, agentID, completedTurn, validScore)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d body=%s, want %d", rr.Code, rr.Body, http.StatusCreated)
		}
		var response map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil || response["status"] != "created" {
			t.Fatalf("create response = %s, decode error = %v", rr.Body, err)
		}
		if len(inserts) != 1 {
			t.Fatalf("got %d inserts, want 1", len(inserts))
		}
		want := repository.CreateCalibrationReviewParams{
			WorkspaceID:    workspaceID,
			RunAgentID:     agentID,
			TurnIndex:      completedTurn,
			ReviewerUserID: caller.UserID,
			Score:          validScore,
			RubricKey:      "recovery",
			Notes:          "reviewed",
		}
		if inserts[0] != want {
			t.Fatalf("insert = %+v, want %+v", inserts[0], want)
		}
	})

	assertRejected := func(t *testing.T, rr *httptest.ResponseRecorder, inserts []repository.CreateCalibrationReviewParams, status int, code string) {
		t.Helper()
		if len(inserts) != 0 {
			t.Errorf("got %d inserts for a rejected request, want 0", len(inserts))
		}
		assertMultiTurnError(t, rr, status, code)
	}

	for _, tc := range []struct {
		name      string
		turnField string
	}{
		{"missing turn index", ""},
		{"null turn index", `"turn_index":null,`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr, inserts := requestWithTurnField(t, zeroTurnAgentID, tc.turnField, validScore)
			assertRejected(t, rr, inserts, http.StatusBadRequest, "invalid_turn_index")
		})
	}

	t.Run("explicit zero turn index", func(t *testing.T) {
		rr, inserts := request(t, zeroTurnAgentID, 0, validScore)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d body=%s, want %d", rr.Code, rr.Body, http.StatusCreated)
		}
		if len(inserts) != 1 || inserts[0].TurnIndex != 0 {
			t.Fatalf("inserts = %+v, want one insert for turn 0", inserts)
		}
	})

	tests := []struct {
		name      string
		turnIndex int64
		score     float64
		errorCode string
	}{
		{"zero score", completedTurn, 0, "invalid_calibration_score"},
		{"score just above maximum", completedTurn, 5.01, "invalid_calibration_score"},
		{"negative score", completedTurn, -1, "invalid_calibration_score"},
		{"score above maximum", completedTurn, 6, "invalid_calibration_score"},
		{"negative turn index", -1, validScore, "invalid_turn_index"},
		{"turn index overflows int32", 2147483648, validScore, "invalid_turn_index"},
		{"turn without completed event", completedTurn + 1, validScore, "invalid_turn_index"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr, inserts := request(t, agentID, tc.turnIndex, tc.score)
			assertRejected(t, rr, inserts, http.StatusBadRequest, tc.errorCode)
		})
	}

	t.Run("unknown and foreign agents return the same error", func(t *testing.T) {
		unknown, unknownInserts := request(t, uuid.New(), completedTurn, validScore)
		foreign, foreignInserts := request(t, foreignAgentID, completedTurn, validScore)
		assertRejected(t, unknown, unknownInserts, http.StatusNotFound, "not_found")
		assertRejected(t, foreign, foreignInserts, http.StatusNotFound, "not_found")
		if unknown.Body.String() != foreign.Body.String() {
			t.Fatalf("unknown agent body = %s, foreign agent body = %s", unknown.Body, foreign.Body)
		}
	})
}
