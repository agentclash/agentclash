package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var tryoutEventRunAgentID = uuid.New()

func tryoutRunEvent(id int64, runID uuid.UUID, seq int64, eventType runevents.Type, payload string) repository.RunEvent {
	if payload == "" {
		payload = "{}"
	}
	return repository.RunEvent{
		ID:             id,
		RunID:          runID,
		RunAgentID:     tryoutEventRunAgentID,
		SequenceNumber: seq,
		EventType:      eventType,
		Source:         runevents.SourceAgentHarnessWorker,
		OccurredAt:     time.Unix(1_700_000_000+seq, 0).UTC(),
		Payload:        json.RawMessage(payload),
	}
}

func TestClassifyTryoutEvent(t *testing.T) {
	tests := []struct {
		eventType   runevents.Type
		wantType    TryoutTimelineEventType
		wantInclude bool
	}{
		{runevents.EventTypeModelOutputDelta, "", false},
		{runevents.EventTypeSystemRunStarted, TryoutTimelineStarted, true},
		{runevents.EventTypeModelCallStarted, TryoutTimelinePlanning, true},
		{runevents.EventTypeToolCallCompleted, TryoutTimelineToolCall, true},
		{runevents.EventTypeSandboxCommandStarted, TryoutTimelineSandboxCommand, true},
		{runevents.EventTypeSandboxFileWritten, TryoutTimelineFileWritten, true},
		{runevents.EventTypeSandboxFileRead, TryoutTimelineFileActivity, true},
		{runevents.EventTypeGraderVerificationCodeExecuted, TryoutTimelineValidation, true},
		{runevents.EventTypeScoringMetricRecorded, TryoutTimelineScoring, true},
		{runevents.EventTypeSystemRunCompleted, TryoutTimelineFinished, true},
		// Unrelated voice event still maps to a stable catch-all, never dropped.
		{runevents.EventTypeTurnUserMessage, TryoutTimelineActivity, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			got, include := classifyTryoutEvent(tt.eventType)
			if include != tt.wantInclude {
				t.Fatalf("include = %v, want %v", include, tt.wantInclude)
			}
			if include && got != tt.wantType {
				t.Fatalf("type = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestMapTryoutTimelineEventRedactsPublicPayload(t *testing.T) {
	runID := uuid.New()
	// Payload mixes safe fields (tool_name, exit_code) with sensitive ones that
	// must never reach a public consumer (command text, stdout).
	payload := `{"tool_name":"read_file","exit_code":0,"command":"cat /etc/passwd","stdout":"SECRET_TOKEN=abc123"}`
	event := tryoutRunEvent(1, runID, 1, runevents.EventTypeToolCallCompleted, payload)

	redacted, include := mapTryoutTimelineEvent(event, true)
	if !include {
		t.Fatal("expected tool call to be included")
	}
	if redacted.Summary != "Tool call completed: read_file" {
		t.Fatalf("summary = %q", redacted.Summary)
	}
	body := string(redacted.Payload)
	if !strings.Contains(body, "tool_name") || !strings.Contains(body, "exit_code") {
		t.Fatalf("redacted payload missing safe fields: %s", body)
	}
	for _, leak := range []string{"command", "passwd", "stdout", "SECRET_TOKEN"} {
		if strings.Contains(body, leak) {
			t.Fatalf("redacted payload leaked %q: %s", leak, body)
		}
	}
	if strings.Contains(redacted.Summary, "passwd") || strings.Contains(redacted.Summary, "SECRET_TOKEN") {
		t.Fatalf("summary leaked sensitive content: %q", redacted.Summary)
	}

	// Workspace consumers (redact=false) receive the full payload verbatim.
	full, _ := mapTryoutTimelineEvent(event, false)
	if !strings.Contains(string(full.Payload), "command") || !strings.Contains(string(full.Payload), "SECRET_TOKEN") {
		t.Fatalf("workspace payload should be unredacted, got %s", full.Payload)
	}
}

func TestMapTryoutTimelineEventSummaryNeverLeaksCommand(t *testing.T) {
	runID := uuid.New()
	event := tryoutRunEvent(1, runID, 1, runevents.EventTypeSandboxCommandStarted, `{"command":"export AWS_SECRET=topsecret && deploy"}`)
	mapped, _ := mapTryoutTimelineEvent(event, true)
	if mapped.Summary != "Sandbox command started" {
		t.Fatalf("summary = %q, want generic sandbox summary", mapped.Summary)
	}
	if mapped.Payload != nil {
		t.Fatalf("payload should be nil when no safe fields present, got %s", mapped.Payload)
	}
}

func TestGetPublicTryoutEventsReturnsRedactedTimeline(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	runID := uuid.New()
	tryoutID := uuid.New()
	repo.tryouts[tryoutID] = repository.AgentTryout{
		ID:     tryoutID,
		Status: repository.AgentTryoutStatusRunning,
		RunID:  &runID,
	}
	repo.runEvents = []repository.RunEvent{
		tryoutRunEvent(1, runID, 1, runevents.EventTypeSystemRunStarted, "{}"),
		tryoutRunEvent(2, runID, 2, runevents.EventTypeModelOutputDelta, `{"text":"hi"}`), // skipped
		tryoutRunEvent(3, runID, 3, runevents.EventTypeToolCallCompleted, `{"tool_name":"writer","stdout":"secret"}`),
	}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	result, err := manager.GetPublicTryoutEvents(ctx, tryoutID, TryoutEventsCursor{})
	if err != nil {
		t.Fatalf("GetPublicTryoutEvents returned error: %v", err)
	}
	if len(result.Events) != 2 {
		t.Fatalf("events = %d, want 2 (delta skipped)", len(result.Events))
	}
	if result.Events[0].Type != TryoutTimelineStarted {
		t.Fatalf("first event type = %q", result.Events[0].Type)
	}
	if result.NextCursor != 3 {
		t.Fatalf("next cursor = %d, want 3 (advances past skipped delta)", result.NextCursor)
	}
	if strings.Contains(string(result.Events[1].Payload), "secret") {
		t.Fatalf("public timeline leaked secret: %s", result.Events[1].Payload)
	}
}

func TestGetPublicTryoutEventsRejectsWorkspaceTryout(t *testing.T) {
	ctx := context.Background()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(uuid.New(), workspaceID)
	tryoutID := uuid.New()
	repo.tryouts[tryoutID] = repository.AgentTryout{ID: tryoutID, WorkspaceID: &workspaceID}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.GetPublicTryoutEvents(ctx, tryoutID, TryoutEventsCursor{})
	if !errors.Is(err, repository.ErrAgentTryoutNotFound) {
		t.Fatalf("error = %v, want ErrAgentTryoutNotFound", err)
	}
}

func TestGetWorkspaceTryoutEventsAuthorization(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	runID := uuid.New()
	tryoutID := uuid.New()
	repo.tryouts[tryoutID] = repository.AgentTryout{
		ID:          tryoutID,
		WorkspaceID: &workspaceID,
		Status:      repository.AgentTryoutStatusRunning,
		RunID:       &runID,
	}
	repo.runEvents = []repository.RunEvent{
		tryoutRunEvent(1, runID, 1, runevents.EventTypeToolCallCompleted, `{"tool_name":"writer","command":"secret"}`),
	}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	// Member: gets the full (unredacted) payload.
	result, err := manager.GetWorkspaceTryoutEvents(ctx, callerWithWorkspace(workspaceID), tryoutID, TryoutEventsCursor{})
	if err != nil {
		t.Fatalf("member GetWorkspaceTryoutEvents returned error: %v", err)
	}
	if len(result.Events) != 1 || !strings.Contains(string(result.Events[0].Payload), "command") {
		t.Fatalf("workspace member should see full payload, got %#v", result.Events)
	}

	// Non-member: forbidden.
	_, err = manager.GetWorkspaceTryoutEvents(ctx, callerWithWorkspace(uuid.New()), tryoutID, TryoutEventsCursor{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-member error = %v, want ErrForbidden", err)
	}

	// Missing tryout: not found.
	_, err = manager.GetWorkspaceTryoutEvents(ctx, callerWithWorkspace(workspaceID), uuid.New(), TryoutEventsCursor{})
	if !errors.Is(err, repository.ErrAgentTryoutNotFound) {
		t.Fatalf("missing tryout error = %v, want ErrAgentTryoutNotFound", err)
	}
}

func TestBuildTryoutEventsPagination(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	runID := uuid.New()
	tryoutID := uuid.New()
	repo.tryouts[tryoutID] = repository.AgentTryout{ID: tryoutID, RunID: &runID, Status: repository.AgentTryoutStatusRunning}
	repo.runEvents = []repository.RunEvent{
		tryoutRunEvent(10, runID, 1, runevents.EventTypeSystemRunStarted, "{}"),
		tryoutRunEvent(20, runID, 2, runevents.EventTypeToolCallStarted, `{"tool_name":"a"}`),
		tryoutRunEvent(30, runID, 3, runevents.EventTypeToolCallCompleted, `{"tool_name":"a"}`),
	}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	first, err := manager.GetPublicTryoutEvents(ctx, tryoutID, TryoutEventsCursor{Limit: 2})
	if err != nil {
		t.Fatalf("first page error: %v", err)
	}
	if len(first.Events) != 2 || !first.HasMore || first.NextCursor != 20 {
		t.Fatalf("first page = %d events has_more=%v next=%d, want 2/true/20", len(first.Events), first.HasMore, first.NextCursor)
	}

	second, err := manager.GetPublicTryoutEvents(ctx, tryoutID, TryoutEventsCursor{After: first.NextCursor, Limit: 2})
	if err != nil {
		t.Fatalf("second page error: %v", err)
	}
	if len(second.Events) != 1 || second.HasMore || second.NextCursor != 30 {
		t.Fatalf("second page = %d events has_more=%v next=%d, want 1/false/30", len(second.Events), second.HasMore, second.NextCursor)
	}
}

func TestGetPublicTryoutEventsEmptyWhenNoRun(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	tryoutID := uuid.New()
	repo.tryouts[tryoutID] = repository.AgentTryout{ID: tryoutID, Status: repository.AgentTryoutStatusQueued}
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	result, err := manager.GetPublicTryoutEvents(ctx, tryoutID, TryoutEventsCursor{After: 7})
	if err != nil {
		t.Fatalf("GetPublicTryoutEvents returned error: %v", err)
	}
	if len(result.Events) != 0 {
		t.Fatalf("events = %d, want 0 before a run is linked", len(result.Events))
	}
	if result.NextCursor != 7 || result.HasMore {
		t.Fatalf("cursor = %d has_more=%v, want 7/false", result.NextCursor, result.HasMore)
	}
}

func TestPublicAgentTryoutEventsHandler(t *testing.T) {
	runID := uuid.New()
	service := &fakeAgentTryoutService{
		eventsResult: AgentTryoutEventsResult{
			Tryout:     repository.AgentTryout{ID: uuid.New(), Status: repository.AgentTryoutStatusRunning, RunID: &runID},
			Events:     []TryoutTimelineEvent{{Cursor: 1, Sequence: 1, Type: TryoutTimelineStarted, Summary: "Run started"}},
			NextCursor: 1,
		},
	}
	handler := getPublicAgentTryoutEventsHandler(slog.Default(), service)

	router := chi.NewRouter()
	router.Get("/v1/agent-tryouts/{tryoutID}/events", handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/agent-tryouts/"+uuid.NewString()+"/events?after=5&limit=10", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if service.eventsCursor.After != 5 || service.eventsCursor.Limit != 10 {
		t.Fatalf("cursor = %+v, want after=5 limit=10", service.eventsCursor)
	}
	var resp agentTryoutEventsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Events) != 1 || resp.NextCursor != 1 || resp.Status != repository.AgentTryoutStatusRunning {
		t.Fatalf("response = %#v", resp)
	}
}

func TestPublicAgentTryoutEventsHandlerInvalidID(t *testing.T) {
	service := &fakeAgentTryoutService{}
	handler := getPublicAgentTryoutEventsHandler(slog.Default(), service)
	router := chi.NewRouter()
	router.Get("/v1/agent-tryouts/{tryoutID}/events", handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/agent-tryouts/not-a-uuid/events", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func TestWorkspaceAgentTryoutEventsHandlerWorkspaceMismatch(t *testing.T) {
	requestWorkspace := uuid.New()
	otherWorkspace := uuid.New()
	service := &fakeAgentTryoutService{
		eventsResult: AgentTryoutEventsResult{
			Tryout: repository.AgentTryout{ID: uuid.New(), WorkspaceID: &otherWorkspace},
		},
	}
	handler := getWorkspaceAgentTryoutEventsHandler(slog.Default(), service)
	router := chi.NewRouter()
	router.Get("/v1/workspaces/{workspaceID}/agent-tryouts/{tryoutID}/events", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), callerContextKey{}, callerWithWorkspace(requestWorkspace))
		handler(w, r.WithContext(ctx))
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+requestWorkspace.String()+"/agent-tryouts/"+uuid.NewString()+"/events", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for workspace mismatch; body=%s", rr.Code, rr.Body.String())
	}
}

func TestWorkspaceAgentTryoutEventsHandlerMissingCaller(t *testing.T) {
	service := &fakeAgentTryoutService{}
	handler := getWorkspaceAgentTryoutEventsHandler(slog.Default(), service)
	router := chi.NewRouter()
	router.Get("/v1/workspaces/{workspaceID}/agent-tryouts/{tryoutID}/events", handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+uuid.NewString()+"/agent-tryouts/"+uuid.NewString()+"/events", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 when caller missing; body=%s", rr.Code, rr.Body.String())
	}
}
