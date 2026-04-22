package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/pubsub"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestExtractStreamEventIDReadsWireFormat(t *testing.T) {
	// Marshal a real envelope, then feed the bytes through
	// extractStreamEventID. If the JSON tag drifts away from the envelope's
	// snake_case wire format again, this test catches it before every SSE
	// event ships with a non-unique id.
	runAgentID := uuid.New()
	env := runevents.Envelope{RunAgentID: runAgentID, SequenceNumber: 42}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	want := persistedStreamEventID(runAgentID, 42)
	if got := extractStreamEventID(data); got != want {
		t.Fatalf("extractStreamEventID(%s) = %q, want %q", data, got, want)
	}
}

func TestExtractStreamEventIDFallsBackOnInvalidJSON(t *testing.T) {
	if got := extractStreamEventID([]byte("not json")); got != "0" {
		t.Fatalf("expected fallback 0, got %q", got)
	}
}

func TestRunEventsStreamAuthenticatesWithAuthorizationHeader(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	auth := &capturingSSEAuthenticator{caller: Caller{UserID: uuid.New()}}
	runReadService := &fakeSSERunReadService{
		streamResults: []ListRunEventStreamResult{
			{
				Run: persistedStreamRun(runID, domain.RunStatusCompleted),
				Events: []repository.RunEvent{
					persistedTestRunEvent(runID, runAgentID, 7, time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)),
				},
			},
		},
	}

	recorder := serveRunEventsSSE(t, auth, runReadService, pubsub.NoopSubscriber{}, runID, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer cli-token")
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if auth.gotAuth != "Bearer cli-token" {
		t.Fatalf("auth header = %q, want Bearer cli-token", auth.gotAuth)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "id: "+persistedStreamEventID(runAgentID, 7)+"\n") || !strings.Contains(body, "event: run_event\n") {
		t.Fatalf("unexpected SSE body: %q", body)
	}
}

func TestRunEventsStreamFallsBackToQueryToken(t *testing.T) {
	runID := uuid.New()
	auth := &capturingSSEAuthenticator{caller: Caller{UserID: uuid.New()}}

	recorder := serveRunEventsSSE(t, auth, &fakeSSERunReadService{
		streamResults: []ListRunEventStreamResult{{Run: persistedStreamRun(runID, domain.RunStatusCompleted)}},
	}, pubsub.NoopSubscriber{}, runID, func(req *http.Request) {
		q := req.URL.Query()
		q.Set("token", "browser-token")
		req.URL.RawQuery = q.Encode()
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if auth.gotAuth != "Bearer browser-token" {
		t.Fatalf("auth header = %q, want Bearer browser-token", auth.gotAuth)
	}
}

func TestRunEventsStreamPrefersAuthorizationHeaderOverQueryToken(t *testing.T) {
	runID := uuid.New()
	auth := &capturingSSEAuthenticator{caller: Caller{UserID: uuid.New()}}

	recorder := serveRunEventsSSE(t, auth, &fakeSSERunReadService{
		streamResults: []ListRunEventStreamResult{{Run: persistedStreamRun(runID, domain.RunStatusCompleted)}},
	}, pubsub.NoopSubscriber{}, runID, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer cli-token")
		q := req.URL.Query()
		q.Set("token", "browser-token")
		req.URL.RawQuery = q.Encode()
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if auth.gotAuth != "Bearer cli-token" {
		t.Fatalf("auth header = %q, want Bearer cli-token", auth.gotAuth)
	}
}

func TestRunEventsStreamRejectsMissingCredentials(t *testing.T) {
	runID := uuid.New()
	auth := &capturingSSEAuthenticator{caller: Caller{UserID: uuid.New()}}

	recorder := serveRunEventsSSE(t, auth, &fakeSSERunReadService{}, pubsub.NoopSubscriber{}, runID, nil)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"missing_token"`) {
		t.Fatalf("body = %s, want missing_token", recorder.Body.String())
	}
	if auth.called {
		t.Fatal("authenticator should not be called without header or query token")
	}
}

func TestRunEventsStreamRejectsInvalidCredentials(t *testing.T) {
	runID := uuid.New()
	auth := &capturingSSEAuthenticator{err: ErrUnauthenticated}

	recorder := serveRunEventsSSE(t, auth, &fakeSSERunReadService{}, pubsub.NoopSubscriber{}, runID, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer bad-token")
	})

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"unauthorized"`) {
		t.Fatalf("body = %s, want unauthorized", recorder.Body.String())
	}
	if auth.gotAuth != "Bearer bad-token" {
		t.Fatalf("auth header = %q, want Bearer bad-token", auth.gotAuth)
	}
}

func TestRunEventsStreamEmitsPersistedEventsBeforeLiveTail(t *testing.T) {
	runID := uuid.New()
	firstAgentID := uuid.New()
	secondAgentID := uuid.New()
	auth := &capturingSSEAuthenticator{caller: Caller{UserID: uuid.New()}}
	runReadService := &fakeSSERunReadService{
		streamResults: []ListRunEventStreamResult{
			{
				Run: persistedStreamRun(runID, domain.RunStatusCompleted),
				Events: []repository.RunEvent{
					persistedTestRunEvent(runID, firstAgentID, 1, time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)),
					persistedTestRunEvent(runID, secondAgentID, 2, time.Date(2026, 4, 22, 10, 0, 1, 0, time.UTC)),
				},
			},
		},
	}

	recorder := serveRunEventsSSE(t, auth, runReadService, pubsub.NoopSubscriber{}, runID, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer cli-token")
	})

	body := recorder.Body.String()
	if !strings.Contains(body, "id: "+persistedStreamEventID(firstAgentID, 1)+"\n") {
		t.Fatalf("body missing first persisted event: %q", body)
	}
	if !strings.Contains(body, "id: "+persistedStreamEventID(secondAgentID, 2)+"\n") {
		t.Fatalf("body missing second persisted event: %q", body)
	}
}

func TestRunEventsStreamUsesUniqueCompoundEventIDs(t *testing.T) {
	runID := uuid.New()
	firstAgentID := uuid.New()
	secondAgentID := uuid.New()
	auth := &capturingSSEAuthenticator{caller: Caller{UserID: uuid.New()}}
	runReadService := &fakeSSERunReadService{
		streamResults: []ListRunEventStreamResult{
			{
				Run: persistedStreamRun(runID, domain.RunStatusCompleted),
				Events: []repository.RunEvent{
					persistedTestRunEvent(runID, firstAgentID, 1, time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)),
					persistedTestRunEvent(runID, secondAgentID, 1, time.Date(2026, 4, 22, 10, 0, 1, 0, time.UTC)),
				},
			},
		},
	}

	recorder := serveRunEventsSSE(t, auth, runReadService, pubsub.NoopSubscriber{}, runID, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer cli-token")
	})

	body := recorder.Body.String()
	firstID := persistedStreamEventID(firstAgentID, 1)
	secondID := persistedStreamEventID(secondAgentID, 1)
	if firstID == secondID {
		t.Fatal("compound SSE ids must be unique across run agents")
	}
	if !strings.Contains(body, "id: "+firstID+"\n") || !strings.Contains(body, "id: "+secondID+"\n") {
		t.Fatalf("body missing unique compound ids: %q", body)
	}
}

func TestRunEventsStreamPollsPersistedEventsWhenNoLiveMessagesArrive(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	auth := &capturingSSEAuthenticator{caller: Caller{UserID: uuid.New()}}
	originalPollInterval := runEventStreamPollInterval
	runEventStreamPollInterval = 5 * time.Millisecond
	defer func() {
		runEventStreamPollInterval = originalPollInterval
	}()

	firstEvent := persistedTestRunEvent(runID, runAgentID, 1, time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC))
	secondEvent := persistedTestRunEvent(runID, runAgentID, 2, time.Date(2026, 4, 22, 10, 0, 1, 0, time.UTC))
	runReadService := &fakeSSERunReadService{
		streamResults: []ListRunEventStreamResult{
			{
				Run:    persistedStreamRun(runID, domain.RunStatusRunning),
				Events: []repository.RunEvent{firstEvent},
			},
			{
				Run:    persistedStreamRun(runID, domain.RunStatusCompleted),
				Events: []repository.RunEvent{firstEvent, secondEvent},
			},
		},
	}

	recorder := serveRunEventsSSE(t, auth, runReadService, &fakeSSESubscriber{}, runID, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer cli-token")
	})

	body := recorder.Body.String()
	if !strings.Contains(body, "id: "+persistedStreamEventID(runAgentID, 1)+"\n") {
		t.Fatalf("body missing initial persisted event: %q", body)
	}
	if !strings.Contains(body, "id: "+persistedStreamEventID(runAgentID, 2)+"\n") {
		t.Fatalf("body missing polled persisted event: %q", body)
	}
}

func serveRunEventsSSE(
	t *testing.T,
	auth Authenticator,
	runReadService RunReadService,
	subscriber pubsub.EventSubscriber,
	runID uuid.UUID,
	mutate func(*http.Request),
) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()
	registerEventStreamRoute(router, slog.New(slog.NewTextHandler(io.Discard, nil)), auth, runReadService, subscriber)

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String()+"/events/stream", nil)
	if mutate != nil {
		mutate(req)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

type capturingSSEAuthenticator struct {
	caller  Caller
	err     error
	called  bool
	gotAuth string
}

func (a *capturingSSEAuthenticator) Authenticate(r *http.Request) (Caller, error) {
	a.called = true
	a.gotAuth = r.Header.Get("Authorization")
	if a.err != nil {
		return Caller{}, a.err
	}
	if a.gotAuth == "" {
		return Caller{}, ErrUnauthenticated
	}
	return a.caller, nil
}

type fakeSSERunReadService struct {
	err           error
	streamResults []ListRunEventStreamResult
	streamCalls   int
}

func (f *fakeSSERunReadService) GetRun(context.Context, Caller, uuid.UUID) (GetRunResult, error) {
	if f.err != nil {
		return GetRunResult{}, f.err
	}
	return GetRunResult{}, nil
}

func (f *fakeSSERunReadService) GetEvalSession(context.Context, Caller, uuid.UUID) (GetEvalSessionResult, error) {
	return GetEvalSessionResult{}, nil
}

func (f *fakeSSERunReadService) GetRunRanking(context.Context, Caller, uuid.UUID, GetRunRankingInput) (GetRunRankingResult, error) {
	return GetRunRankingResult{}, nil
}

func (f *fakeSSERunReadService) GenerateRunRankingInsights(context.Context, Caller, uuid.UUID, GenerateRunRankingInsightsInput) (GenerateRunRankingInsightsResult, error) {
	return GenerateRunRankingInsightsResult{}, nil
}

func (f *fakeSSERunReadService) ListEvalSessions(context.Context, Caller, ListEvalSessionsInput) (ListEvalSessionsResult, error) {
	return ListEvalSessionsResult{}, nil
}

func (f *fakeSSERunReadService) ListRunAgents(context.Context, Caller, uuid.UUID) (ListRunAgentsResult, error) {
	return ListRunAgentsResult{}, nil
}

func (f *fakeSSERunReadService) ListRunFailures(context.Context, Caller, ListRunFailuresInput) (ListRunFailuresResult, error) {
	return ListRunFailuresResult{}, nil
}

func (f *fakeSSERunReadService) ListRuns(context.Context, Caller, ListRunsInput) (ListRunsResult, error) {
	return ListRunsResult{}, nil
}

func (f *fakeSSERunReadService) ListRunEventStream(context.Context, Caller, uuid.UUID) (ListRunEventStreamResult, error) {
	if f.err != nil {
		return ListRunEventStreamResult{}, f.err
	}
	if len(f.streamResults) == 0 {
		return ListRunEventStreamResult{}, nil
	}
	if f.streamCalls >= len(f.streamResults) {
		return f.streamResults[len(f.streamResults)-1], nil
	}
	result := f.streamResults[f.streamCalls]
	f.streamCalls++
	return result, nil
}

type fakeSSESubscriber struct {
	events [][]byte
	err    error
	called bool
}

func (s *fakeSSESubscriber) Subscribe(context.Context, uuid.UUID) (<-chan []byte, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan []byte, len(s.events))
	for _, event := range s.events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (s *fakeSSESubscriber) Close() error {
	return nil
}

func persistedStreamRun(runID uuid.UUID, status domain.RunStatus) domain.Run {
	return domain.Run{
		ID:     runID,
		Status: status,
	}
}

func persistedTestRunEvent(runID uuid.UUID, runAgentID uuid.UUID, sequenceNumber int64, occurredAt time.Time) repository.RunEvent {
	return repository.RunEvent{
		RunID:          runID,
		RunAgentID:     runAgentID,
		SequenceNumber: sequenceNumber,
		EventType:      runevents.EventTypeSystemRunStarted,
		Source:         runevents.SourceNativeEngine,
		OccurredAt:     occurredAt,
		Payload:        []byte(`{"phase":"testing"}`),
	}
}
