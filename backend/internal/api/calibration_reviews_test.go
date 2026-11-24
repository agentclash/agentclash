package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed testdata/calibration_reviews.sql
var calibrationReviewFixtureSQL string

func TestCalibrationReviewCreate(t *testing.T) {
	fixture := newCalibrationReviewFixture(t)
	tests := []struct {
		name      string
		agent     uuid.UUID
		turn      string
		score     string
		want      int
		errorCode string
	}{
		{"score below range", fixture.agent, "0", "0.99", 400, "invalid_calibration_score"},
		{"score above range", fixture.agent, "0", "5.01", 400, "invalid_calibration_score"},
		{"nonfinite JSON token", fixture.agent, "0", "NaN", 400, "invalid_json"},
		{"JSON score overflow", fixture.agent, "0", "1e1000", 400, "invalid_json"},
		{"negative turn", fixture.agent, "-1", "4", 400, "invalid_turn_index"},
		{"turn integer overflow", fixture.agent, "2147483648", "4", 400, "invalid_turn_index"},
		{"turn past last completed", fixture.agent, "2", "4", 400, "invalid_turn_index"},
		{"unknown agent", uuid.New(), "1", "4", 404, "not_found"},
		{"foreign agent", fixture.foreignAgent, "1", "4", 404, "not_found"},
		{"valid first turn and minimum score", fixture.agent, "0", "1", 201, ""},
		{"valid final turn and maximum score", fixture.agent, "1", "5", 201, ""},
	}
	var unknownAgentResponse string
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"run_agent_id":%q,"turn_index":%s,"score":%s,"rubric_key":"recovery","notes":"reviewed"}`, tc.agent, tc.turn, tc.score)
			rr := fixture.request(http.MethodPost, fixture.workspace, body)
			if tc.errorCode != "" {
				assertMultiTurnError(t, rr, tc.want, tc.errorCode)
				if tc.name == "unknown agent" {
					unknownAgentResponse = rr.Body.String()
				}
				if tc.name == "foreign agent" && rr.Body.String() != unknownAgentResponse {
					t.Fatal("foreign and unknown agents returned different error bodies")
				}
				return
			}
			if rr.Code != tc.want {
				t.Fatalf("status = %d body=%s, want %d", rr.Code, rr.Body, tc.want)
			}
			var response map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil || response["status"] != "created" {
				t.Fatalf("create response = %s, decode error = %v", rr.Body, err)
			}
		})
	}

	var count int
	if err := fixture.db.QueryRow(t.Context(), calibrationFixtureStatement(t, "count reviews"), fixture.workspace).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("stored %d reviews, want 2", count)
	}
}

func TestCalibrationReviewCreateRejectsNonfiniteScore(t *testing.T) {
	workspace := uuid.New()
	caller := callerWithWorkspace(workspace)
	manager := NewMultiTurnManager(NewCallerWorkspaceAuthorizer(), repository.New(nil), nil)
	for _, score := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		t.Run(fmt.Sprint(score), func(t *testing.T) {
			err := manager.CreateCalibrationReview(t.Context(), caller, workspace, CalibrationReviewRequest{Score: score})
			if err == nil {
				t.Fatal("expected score validation error")
			}
			rr := httptest.NewRecorder()
			writeMultiTurnError(rr, slog.Default(), httptest.NewRequest(http.MethodPost, "/", nil), err)
			assertMultiTurnError(t, rr, http.StatusBadRequest, "invalid_calibration_score")
		})
	}
}

func TestCalibrationReviewListWorkspace(t *testing.T) {
	fixture := newCalibrationReviewFixture(t)
	ctx := t.Context()
	if _, err := fixture.db.Exec(ctx, calibrationFixtureStatement(t, "legacy reviews"),
		fixture.workspace, fixture.agent, fixture.foreignWorkspace, fixture.foreignAgent, fixture.user); err != nil {
		t.Fatal(err)
	}

	rr := fixture.request(http.MethodGet, fixture.workspace, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body)
	}
	var response struct {
		Items []CalibrationReviewResponse `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("got %d reviews, want 1", len(response.Items))
	}
	if review := response.Items[0]; review.RunAgentID != fixture.agent || review.Score != 4.5 || review.TurnIndex != 1 || review.ReviewerID != fixture.user {
		t.Fatalf("unexpected review: %+v", review)
	}

	assertMultiTurnError(t, fixture.request(http.MethodGet, fixture.foreignWorkspace, ""), http.StatusForbidden, "forbidden")
}

type calibrationReviewFixture struct {
	db               *pgxpool.Pool
	router           *chi.Mux
	workspace        uuid.UUID
	foreignWorkspace uuid.UUID
	agent            uuid.UUID
	foreignAgent     uuid.UUID
	user             uuid.UUID
}

func (f calibrationReviewFixture) request(method string, workspace uuid.UUID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/v1/workspaces/"+workspace.String()+"/calibration-reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.router.ServeHTTP(rr, req)
	return rr
}

func calibrationFixtureStatement(t *testing.T, name string) string {
	t.Helper()
	_, body, found := strings.Cut(calibrationReviewFixtureSQL, "-- "+name+"\n")
	if !found {
		t.Fatalf("missing calibration fixture statement %q", name)
	}
	statement, _, _ := strings.Cut(body, "\n-- ")
	return statement
}

func newCalibrationReviewFixture(t *testing.T) calibrationReviewFixture {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("requires a migrated test DATABASE_URL")
	}
	ctx := t.Context()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	org, workspace, foreignWorkspace := uuid.New(), uuid.New(), uuid.New()
	caller := callerWithWorkspaceRole(workspace, RoleWorkspaceMember)
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, calibrationFixtureStatement(t, statement), args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("org", org, org.String())
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), calibrationFixtureStatement(t, "delete org"), org); err != nil {
			t.Error(err)
		}
		if _, err := db.Exec(context.Background(), calibrationFixtureStatement(t, "delete user"), caller.UserID); err != nil {
			t.Error(err)
		}
	})
	exec("user", caller.UserID, caller.UserID.String(), caller.UserID.String()+"@example.invalid")
	seedAgent := func(workspace uuid.UUID) uuid.UUID {
		t.Helper()
		run, agent := uuid.New(), uuid.New()
		exec("workspace", workspace, org, workspace.String())
		exec("run", run, org, workspace)
		exec("agent", agent, org, workspace, run)
		exec("turns", run, agent)
		return agent
	}

	resolver := runevents.NewResolver(runevents.OpenFunc(func(_ context.Context, key string) (io.ReadCloser, error) {
		if key != "run-events/calibration/turn-1.json" {
			return nil, fmt.Errorf("unexpected event payload key %q", key)
		}
		return io.NopCloser(strings.NewReader(`{"turn_index":1,"mismatch":false}`)), nil
	}), 1)
	manager := NewMultiTurnManager(NewCallerWorkspaceAuthorizer(), repository.New(db).WithPayloadResolver(resolver), nil)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callerContextKey{}, caller)))
		})
	})
	const path = "/v1/workspaces/{workspaceID}/calibration-reviews"
	router.Post(path, createCalibrationReviewHandler(slog.Default(), manager))
	router.Get(path, listCalibrationReviewsHandler(slog.Default(), manager))
	return calibrationReviewFixture{db, router, workspace, foreignWorkspace, seedAgent(workspace), seedAgent(foreignWorkspace), caller.UserID}
}
