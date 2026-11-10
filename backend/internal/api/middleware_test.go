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
	"github.com/agentclash/agentclash/backend/internal/productanalytics"
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

func TestClassifyRequestSurface(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    productanalytics.Surface
	}{
		{
			name: "cli user agent wins over origin",
			headers: map[string]string{
				"User-Agent": "agentclash-cli/1.2.3 (cmd=run create; os=linux; arch=amd64)",
				"Origin":     "https://app.agentclash.dev",
			},
			want: productanalytics.SurfaceCLI,
		},
		{
			name:    "browser origin",
			headers: map[string]string{"Origin": "https://app.agentclash.dev"},
			want:    productanalytics.SurfaceWeb,
		},
		{
			name:    "browser referrer",
			headers: map[string]string{"Referer": "https://app.agentclash.dev/workspaces/id"},
			want:    productanalytics.SurfaceWeb,
		},
		{name: "direct API client", want: productanalytics.SurfaceAPI},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
			for key, value := range test.headers {
				request.Header.Set(key, value)
			}
			if got := classifyRequestSurface(request); got != test.want {
				t.Fatalf("classifyRequestSurface() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAgentReadableRequestLogClassifiers(t *testing.T) {
	t.Run("redacts capability token paths", func(t *testing.T) {
		cases := map[string]string{
			"/public/shares/a-secret-token":                  "/public/shares/{token}",
			"/agent-tryouts/shared/a-secret-token/events":    "/agent-tryouts/shared/{token}/events",
			"/v1/agent-tryouts/shared/a-secret-token/events": "/v1/agent-tryouts/shared/{token}/events",
			"/invites/organization/a-secret-token":           "/invites/organization/{token}",
			"/invites/workspace/a-secret-token":              "/invites/workspace/{token}",
			"/v1/invites/organization/a-secret-token":        "/v1/invites/organization/{token}",
			"/v1/invites/workspace/a-secret-token":           "/v1/invites/workspace/{token}",
			"/public/shares/a-secret-token/extra":            "/public/shares/{token}/extra",
		}
		for path, want := range cases {
			if got := safeRequestLogPath(path); got != want {
				t.Errorf("safeRequestLogPath(%q) = %q, want %q", path, got, want)
			}
		}
		if got := safeRequestLogPath("/public/publications/123"); got != "/public/publications/123" {
			t.Fatalf("safeRequestLogPath() = %q, want publication path", got)
		}
	})

	t.Run("classifies known agents without storing raw user agents", func(t *testing.T) {
		cases := map[string]string{
			"Mozilla/5.0 compatible; GPTBot/1.2": "GPTBot",
			"ClaudeBot/1.0":                      "ClaudeBot",
			"ordinary-browser":                   "unclassified",
		}
		for raw, want := range cases {
			if got := classifyAgentAPIUserAgent(raw); got != want {
				t.Errorf("classifyAgentAPIUserAgent(%q) = %q, want %q", raw, got, want)
			}
		}
	})

	t.Run("classifies representation requests", func(t *testing.T) {
		if got := classifyAgentAPIAccept("text/markdown, text/html;q=0.5"); got != "markdown" {
			t.Fatalf("classifyAgentAPIAccept() = %q, want markdown", got)
		}
		if got := requestedAPIRepresentation("application/json"); got != "json" {
			t.Fatalf("requestedAPIRepresentation() = %q, want json", got)
		}
	})

	t.Run("targets publication and agent requests", func(t *testing.T) {
		publication := httptest.NewRequest(http.MethodGet, "/public/publications", nil)
		if !shouldLogAgentReadableAPIRequest(publication) {
			t.Fatal("publication request should receive enriched request logging")
		}

		agent := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		agent.Header.Set("User-Agent", "OAI-SearchBot/1.0")
		if !shouldLogAgentReadableAPIRequest(agent) {
			t.Fatal("known AI agent request should receive enriched request logging")
		}
	})
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
		challengePackReadService:   stubChallengePackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		challengePackAuthoringSvc:  stubChallengePackAuthoringService{},
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

func TestExplicitObservedHostContext(t *testing.T) {
	for _, tc := range []struct{ ua, want string }{
		{"agentclash-cli/dev (cmd=run.list; host=claude)", "claude"},
		{"agentclash-cli/dev (cmd=run.list; host=codex)", "codex"},
		{"agentclash-cli/dev (cmd=run.list; host=secret.invalid)", "unknown"},
		{"ClaudeBot", "unknown"}, {"agentclash-cli/dev", "unknown"},
	} {
		request := httptest.NewRequest("GET", "/v1/runs?utm_source=claude", nil)
		request.Header.Set("User-Agent", tc.ua)
		requestAnalyticsSurface(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := productanalytics.ObservedHostFromContext(r.Context()); got != tc.want {
				t.Fatalf("%s: %s", tc.ua, got)
			}
		})).ServeHTTP(httptest.NewRecorder(), request)
	}
}
