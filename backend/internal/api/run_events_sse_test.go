package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestRunEventsStreamAuthenticatesWithAuthorizationHeader(t *testing.T) {
	runID := uuid.New()
	auth := &capturingSSEAuthenticator{caller: Caller{UserID: uuid.New()}}
	subscriber := &fakeSSESubscriber{
		events: [][]byte{[]byte(`{"SequenceNumber":7,"EventType":"started"}`)},
	}

	recorder := serveRunEventsSSE(t, auth, &fakeSSERunReadService{}, subscriber, runID, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer cli-token")
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if auth.gotAuth != "Bearer cli-token" {
		t.Fatalf("auth header = %q, want Bearer cli-token", auth.gotAuth)
	}
	if !subscriber.called {
		t.Fatal("expected subscriber to be called")
	}
	if body := recorder.Body.String(); !strings.Contains(body, "id: 7\n") || !strings.Contains(body, "event: run_event\n") {
		t.Fatalf("unexpected SSE body: %q", body)
	}
}

func TestRunEventsStreamFallsBackToQueryToken(t *testing.T) {
	runID := uuid.New()
	auth := &capturingSSEAuthenticator{caller: Caller{UserID: uuid.New()}}

	recorder := serveRunEventsSSE(t, auth, &fakeSSERunReadService{}, &fakeSSESubscriber{}, runID, func(req *http.Request) {
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

	recorder := serveRunEventsSSE(t, auth, &fakeSSERunReadService{}, &fakeSSESubscriber{}, runID, func(req *http.Request) {
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

	recorder := serveRunEventsSSE(t, auth, &fakeSSERunReadService{}, &fakeSSESubscriber{}, runID, nil)

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

	recorder := serveRunEventsSSE(t, auth, &fakeSSERunReadService{}, &fakeSSESubscriber{}, runID, func(req *http.Request) {
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

func serveRunEventsSSE(
	t *testing.T,
	auth Authenticator,
	runReadService RunReadService,
	subscriber *fakeSSESubscriber,
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
	err error
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
