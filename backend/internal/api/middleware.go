package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/posthog"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// requestIDHeader is the canonical response header carrying the correlation id
// assigned to every incoming request. Matches X-Request-Id used elsewhere in
// the Go ecosystem (gRPC-gateway, chi defaults).
const requestIDHeader = "X-Request-Id"

type requestIDContextKey struct{}

// requestIDMiddleware assigns a UUID to each request, stashes it in the
// context (consumed by trackUsage), and echoes it back on the response. The
// id is also folded into the request-completed log line and the PostHog
// event ($request_id) so operators can correlate logs with analytics.
func requestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := uuid.New()
			ctx := context.WithValue(r.Context(), requestIDContextKey{}, id)
			w.Header().Set(requestIDHeader, id.String())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFromContext returns the request id assigned by requestIDMiddleware.
// Returns uuid.Nil when the middleware did not run (tests, public webhook
// routes that never traverse the chain).
func RequestIDFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(requestIDContextKey{}).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(recorder, r)

			logger.Info("http request completed",
				"request_id", RequestIDFromContext(r.Context()).String(),
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.Status(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}

// userAgentPattern matches our CLI's User-Agent header. The leaf binary sets
// "agentclash-cli/<ver>" for self-hosters and adds a "(...)" segment with
// "cmd=...; os=...; arch=...; go=..." key/value pairs when pointed at the
// hosted backend. See cli/internal/api/client.go.
var userAgentPattern = regexp.MustCompile(`^agentclash-cli/([\w.\-+]+)(?:\s+\(([^)]+)\))?$`)

type cliUserAgent struct {
	Version string
	Command string
	OS      string
	Arch    string
}

func parseCLIUserAgent(raw string) (cliUserAgent, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cliUserAgent{}, false
	}
	m := userAgentPattern.FindStringSubmatch(raw)
	if m == nil {
		return cliUserAgent{}, false
	}
	ua := cliUserAgent{Version: m[1]}
	if len(m) > 2 && m[2] != "" {
		for _, pair := range strings.Split(m[2], ";") {
			kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(kv) != 2 {
				continue
			}
			value := strings.TrimSpace(kv[1])
			switch strings.TrimSpace(kv[0]) {
			case "cmd":
				ua.Command = value
			case "os":
				ua.OS = value
			case "arch":
				ua.Arch = value
			}
		}
	}
	return ua, true
}

// trackingSkipPrefixes lists path prefixes that must not emit analytics
// events — health checks would dominate event volume.
var trackingSkipPrefixes = []string{
	"/healthz",
}

// trackingSkipExact lists exact paths to skip in addition to prefix matches.
var trackingSkipExact = map[string]struct{}{
	"/v1/model-catalog": {},
}

func shouldSkipTracking(path string) bool {
	if _, ok := trackingSkipExact[path]; ok {
		return true
	}
	for _, p := range trackingSkipPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// surface values recorded on every analytics event.
const (
	surfaceCLI = "cli"
	surfaceWeb = "web"
	surfaceAPI = "api"
)

// trackUsage emits one PostHog event per request for usage analytics. It runs
// after authenticateRequest so the Caller — and thus the user UUID used as the
// PostHog distinct_id — is available. Emission is fire-and-forget: the request
// hot path never blocks on PostHog, and an unconfigured (noop) client is a
// zero-cost no-op.
//
// distinct_id is the authenticated user's UUID so CLI (server-side) and web
// (client-side) events stitch onto the same PostHog person. Anonymous requests
// use a stable anonymous id with $process_person_profile=false so they don't
// create junk person profiles.
func trackUsage(logger *slog.Logger, hog posthog.Client) func(http.Handler) http.Handler {
	_ = logger // kept for signature parity with the other middleware constructors
	if hog == nil {
		hog = posthog.Noop{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			rec := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(rec, r)

			if shouldSkipTracking(r.URL.Path) {
				return
			}

			durationMs := int(time.Since(startedAt).Milliseconds())
			routePattern := ""
			if rc := chi.RouteContext(r.Context()); rc != nil {
				routePattern = rc.RoutePattern()
			}
			if routePattern == "" {
				routePattern = r.URL.Path
			}

			ua, isCLI := parseCLIUserAgent(r.Header.Get("User-Agent"))
			surface := surfaceAPI
			switch {
			case isCLI:
				surface = surfaceCLI
			case isLikelyWebOrigin(r):
				surface = surfaceWeb
			}

			requestID := RequestIDFromContext(r.Context())
			if requestID == uuid.Nil {
				requestID = uuid.New()
			}

			props := map[string]any{
				"route":       routePattern,
				"method":      r.Method,
				"status_code": rec.Status(),
				"duration_ms": durationMs,
				"surface":     surface,
				"$request_id": requestID.String(),
			}
			if ua.Command != "" {
				props["command"] = ua.Command
			}
			if ua.Version != "" {
				props["cli_version"] = ua.Version
			}
			if ua.OS != "" {
				props["os"] = ua.OS
			}
			if ua.Arch != "" {
				props["arch"] = ua.Arch
			}

			distinctID := posthog.AnonymousDistinctID()
			authenticated := false
			if caller, err := CallerFromContext(r.Context()); err == nil {
				authenticated = true
				distinctID = caller.UserID.String()
				if wsID, wsErr := WorkspaceIDFromContext(r.Context()); wsErr == nil {
					props["workspace_id"] = wsID.String()
				}
				// Best-effort org attribution: pick the first membership.
				for orgID := range caller.OrganizationMemberships {
					props["org_id"] = orgID.String()
					break
				}
			}
			if !authenticated {
				// Don't create a PostHog person profile for anonymous traffic.
				props["$process_person_profile"] = false
			}

			eventName := "api.request"
			switch {
			case isCLI:
				eventName = "cli.command.invoked"
			case surface == surfaceWeb:
				eventName = "web.api.request"
			}

			hog.Capture(posthog.Event{
				DistinctID: distinctID,
				EventName:  eventName,
				Properties: props,
			})
		})
	}
}

// isLikelyWebOrigin returns true if the request carries an Origin or Referer
// matching the FrontendURL pattern. Best-effort signal for analytics splitting;
// not a security boundary.
func isLikelyWebOrigin(r *http.Request) bool {
	if r.Header.Get("Origin") != "" {
		return true
	}
	if r.Header.Get("Referer") != "" {
		return true
	}
	return false
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered from http handler",
						"method", r.Method,
						"path", r.URL.Path,
						"panic", fmt.Sprint(recovered),
						"stack", string(debug.Stack()),
					)
					writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
