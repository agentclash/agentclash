package api

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/posthog"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// recordingPostHog is a posthog.Client spy that captures every emitted event so
// tests can assert exactly which requests produce analytics events and what
// properties they carry.
type recordingPostHog struct {
	mu     sync.Mutex
	events []posthog.Event
}

func (s *recordingPostHog) Capture(event posthog.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *recordingPostHog) Identify(string, map[string]any) {}
func (s *recordingPostHog) Close() error                    { return nil }

func (s *recordingPostHog) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func (s *recordingPostHog) last() *posthog.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		return nil
	}
	event := s.events[len(s.events)-1]
	return &event
}

func TestShouldSkipTracking(t *testing.T) {
	cases := []struct {
		path string
		skip bool
	}{
		{"/healthz", true},
		{"/healthz/ready", true},
		{"/v1/cli-auth/device/token", true}, // noisy login polling — skipped
		{"/v1/cli-auth/device", false},      // one-shot initiation — kept
		{"/v1/runs", false},
		{"/v1/workspaces/abc/runs", false},
		{"/v1/auth/session", false},
	}
	for _, tc := range cases {
		if got := shouldSkipTracking(tc.path); got != tc.skip {
			t.Errorf("shouldSkipTracking(%q) = %v, want %v", tc.path, got, tc.skip)
		}
	}
}

// captureFor builds a minimal chi router that injects the given caller (and
// optional workspace-id context) ahead of trackUsage, serves a GET to
// requestPath matched by routePattern, and returns the single captured event
// (or nil if none was emitted). This exercises trackUsage's real attribution
// logic — including post-serve chi URLParam reads — without standing up the
// full application router.
func captureFor(t *testing.T, caller *Caller, wsCtx *uuid.UUID, routePattern, requestPath string) *posthog.Event {
	t.Helper()
	spy := &recordingPostHog{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if caller != nil {
				ctx = context.WithValue(ctx, callerContextKey{}, *caller)
			}
			if wsCtx != nil {
				ctx = context.WithValue(ctx, workspaceIDContextKey{}, *wsCtx)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Use(trackUsage(logger, spy))
	router.Get(routePattern, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, requestPath, nil))
	return spy.last()
}

func singleOrgCaller(userID, orgID uuid.UUID) *Caller {
	return &Caller{
		UserID: userID,
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: "org_admin"},
		},
	}
}

func TestTrackUsageAttribution(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	wsURL := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	wsCtx := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	orgA := uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000000")
	orgB := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000000")

	t.Run("workspace_id falls back to URL param when context is absent", func(t *testing.T) {
		event := captureFor(t, singleOrgCaller(userID, orgA), nil,
			"/v1/workspaces/{workspaceID}/runs", "/v1/workspaces/"+wsURL.String()+"/runs")
		if event == nil {
			t.Fatal("expected an event")
		}
		if got := event.Properties["workspace_id"]; got != wsURL.String() {
			t.Errorf("workspace_id = %v, want %v", got, wsURL)
		}
		if got := event.Properties["org_id"]; got != orgA.String() {
			t.Errorf("org_id = %v, want %v (single membership)", got, orgA)
		}
	})

	t.Run("workspace_id context wins over URL param", func(t *testing.T) {
		event := captureFor(t, singleOrgCaller(userID, orgA), &wsCtx,
			"/v1/workspaces/{workspaceID}/runs", "/v1/workspaces/"+wsURL.String()+"/runs")
		if got := event.Properties["workspace_id"]; got != wsCtx.String() {
			t.Errorf("workspace_id = %v, want context value %v", got, wsCtx)
		}
	})

	t.Run("org_id comes from the organizationID route param", func(t *testing.T) {
		caller := &Caller{UserID: userID, OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgA: {OrganizationID: orgA}, orgB: {OrganizationID: orgB},
		}}
		event := captureFor(t, caller, nil,
			"/v1/organizations/{organizationID}/workspaces", "/v1/organizations/"+orgB.String()+"/workspaces")
		if got := event.Properties["org_id"]; got != orgB.String() {
			t.Errorf("org_id = %v, want route param %v", got, orgB)
		}
	})

	t.Run("org_id is omitted for multi-org callers without a route param", func(t *testing.T) {
		caller := &Caller{UserID: userID, OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgA: {OrganizationID: orgA}, orgB: {OrganizationID: orgB},
		}}
		event := captureFor(t, caller, nil,
			"/v1/workspaces/{workspaceID}/runs", "/v1/workspaces/"+wsURL.String()+"/runs")
		if _, ok := event.Properties["org_id"]; ok {
			t.Errorf("org_id should be omitted for ambiguous multi-org caller, got %v", event.Properties["org_id"])
		}
	})

	t.Run("anonymous request is not profiled", func(t *testing.T) {
		event := captureFor(t, nil, nil, "/v1/runs", "/v1/runs")
		if event == nil {
			t.Fatal("expected an event")
		}
		if event.DistinctID != posthog.AnonymousDistinctID() {
			t.Errorf("distinct_id = %v, want anonymous", event.DistinctID)
		}
		if event.Properties["$process_person_profile"] != false {
			t.Errorf("$process_person_profile = %v, want false", event.Properties["$process_person_profile"])
		}
		if _, ok := event.Properties["workspace_id"]; ok {
			t.Error("anonymous event should not carry workspace_id")
		}
	})
}

// TestTrackUsageDeviceFlowEmitsNoEvents proves against the real application
// router that the unauthenticated CLI device-login endpoints (init + poll) do
// not emit analytics events — they are registered on the top-level router,
// outside the authenticated /v1 group that carries trackUsage — while a
// protected route does emit exactly one event.
func TestTrackUsageDeviceFlowEmitsNoEvents(t *testing.T) {
	spy := &recordingPostHog{}
	handler := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     slog.New(slog.NewTextHandler(io.Discard, nil)),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		posthogClient:              spy,
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		evalPackReadService:   stubEvalPackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		evalPackAuthoringSvc:  stubEvalPackAuthoringService{},
		cliAuthServices:            []CLIAuthService{stubCLIAuthService{}},
	})

	for _, path := range []string{"/v1/cli-auth/device", "/v1/cli-auth/device/token"} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	if n := spy.count(); n != 0 {
		t.Fatalf("device-flow endpoints emitted %d analytics events, want 0", n)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set(headerUserID, "11111111-1111-1111-1111-111111111111")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if n := spy.count(); n != 1 {
		t.Fatalf("protected route produced %d events, want 1", n)
	}
}
