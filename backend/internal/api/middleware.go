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
	"github.com/agentclash/agentclash/backend/internal/productanalytics"
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

			attributes := []any{
				"request_id", RequestIDFromContext(r.Context()).String(),
				"method", r.Method,
				"path", safeRequestLogPath(r.URL.Path),
				"status", recorder.Status(),
				"response_bytes", recorder.BytesWritten(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			}
			if shouldLogAgentReadableAPIRequest(r) {
				attributes = append(attributes,
					"agent_family", classifyAgentAPIUserAgent(r.UserAgent()),
					"accept_class", classifyAgentAPIAccept(r.Header.Get("Accept")),
					"requested_representation", requestedAPIRepresentation(r.Header.Get("Accept")),
					"served_representation", "json",
					"route_kind", agentAPIRouteKind(r.URL.Path),
				)
			}
			logger.Info("http request completed", attributes...)
		})
	}
}

func safeRequestLogPath(path string) string {
	for _, prefix := range []string{
		"/public/shares/",
		"/agent-tryouts/shared/",
		"/v1/agent-tryouts/shared/",
		"/invites/organization/",
		"/invites/workspace/",
		"/v1/invites/organization/",
		"/v1/invites/workspace/",
	} {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		remainder := strings.TrimPrefix(path, prefix)
		if remainder == "" {
			return path
		}
		if slash := strings.IndexByte(remainder, '/'); slash >= 0 {
			return prefix + "{token}" + remainder[slash:]
		}
		return prefix + "{token}"
	}
	return path
}

func shouldLogAgentReadableAPIRequest(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/public/publications") ||
		classifyAgentAPIUserAgent(r.UserAgent()) != "unclassified" ||
		strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/markdown")
}

func classifyAgentAPIUserAgent(raw string) string {
	normalized := strings.ToLower(raw)
	families := []struct{ needle, label string }{
		{"gptbot", "GPTBot"},
		{"oai-searchbot", "OAI-SearchBot"},
		{"chatgpt-user", "ChatGPT-User"},
		{"claude-searchbot", "Claude-SearchBot"},
		{"claude-user", "Claude-User"},
		{"claudebot", "ClaudeBot"},
		{"perplexitybot", "PerplexityBot"},
		{"perplexity-user", "Perplexity-User"},
		{"google-extended", "Google-Extended"},
		{"ccbot", "CCBot"},
		{"bytespider", "Bytespider"},
		{"meta-externalagent", "meta-externalagent"},
		{"applebot-extended", "Applebot-Extended"},
		{"amazonbot", "Amazonbot"},
	}
	for _, family := range families {
		if strings.Contains(normalized, family.needle) {
			return family.label
		}
	}
	return "unclassified"
}

func classifyAgentAPIAccept(accept string) string {
	normalized := strings.ToLower(accept)
	if normalized == "" {
		return "missing"
	}
	if strings.Contains(normalized, "text/markdown") {
		return "markdown"
	}
	if strings.Contains(normalized, "text/html") {
		return "html"
	}
	if strings.Contains(normalized, "application/json") {
		return "json"
	}
	return "generic"
}

func requestedAPIRepresentation(accept string) string {
	if strings.Contains(strings.ToLower(accept), "text/markdown") {
		return "markdown"
	}
	return "json"
}

func agentAPIRouteKind(path string) string {
	if strings.HasPrefix(path, "/public/publications") {
		return "publication_api"
	}
	return "api"
}

// userAgentPattern matches our CLI's User-Agent header. The leaf binary sets
// "agentclash-cli/<ver>" for self-hosters and adds a "(...)" segment with
// "cmd=...; os=...; arch=...; go=..." key/value pairs when pointed at the
// hosted backend. See cli/internal/api/client.go.
var userAgentPattern = regexp.MustCompile(`^agentclash-cli/([\w.\-+]+)(?:\s+\(([^)]+)\))?$`)

type cliUserAgent struct {
	Host    string
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
	ua := cliUserAgent{Version: m[1], Host: "unknown"}
	if len(m) > 2 && m[2] != "" {
		for _, pair := range strings.Split(m[2], ";") {
			kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(kv) != 2 {
				continue
			}
			value := strings.TrimSpace(kv[1])
			switch strings.TrimSpace(kv[0]) {
			case "host":
				ua.Host = productanalytics.NormalizeObservedHost(value)
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
	// Device-token polling: `agentclash auth login` polls this endpoint every
	// ~5s for up to 10 minutes (~120 hits per login). It currently sits outside
	// the trackUsage middleware (registered on the top-level router in
	// server.go, not the authenticated /v1 group), so no events are emitted for
	// it today — but skip it explicitly so that if the route is ever moved under
	// tracking, polling can never flood analytics. The one-shot initiation
	// endpoint (/v1/cli-auth/device) is intentionally NOT skipped.
	"/v1/cli-auth/device/token": {},
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

func classifyRequestSurface(r *http.Request) productanalytics.Surface {
	if _, isCLI := parseCLIUserAgent(r.Header.Get("User-Agent")); isCLI {
		return productanalytics.SurfaceCLI
	}
	if isLikelyWebOrigin(r) {
		return productanalytics.SurfaceWeb
	}
	return productanalytics.SurfaceAPI
}

// requestAnalyticsSurface runs before authentication so account activation
// and every downstream manager observe the same web/CLI/API classification.
func requestAnalyticsSurface(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := productanalytics.WithSurface(r.Context(), classifyRequestSurface(r))
		ua, _ := parseCLIUserAgent(r.UserAgent())
		ctx = productanalytics.WithObservedHost(ctx, ua.Host)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

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
				routePattern = safeRequestLogPath(r.URL.Path)
			}

			ua, isCLI := parseCLIUserAgent(r.Header.Get("User-Agent"))
			surface := string(productanalytics.SurfaceFromContext(r.Context()))
			if surface == "" {
				surface = string(classifyRequestSurface(r))
			}

			requestID := RequestIDFromContext(r.Context())
			if requestID == uuid.Nil {
				requestID = uuid.New()
			}

			props := map[string]any{
				"route":         routePattern,
				"method":        r.Method,
				"status_code":   rec.Status(),
				"duration_ms":   durationMs,
				"surface":       surface,
				"observed_host": productanalytics.NormalizeObservedHost(ua.Host),
				"$request_id":   requestID.String(),
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

				// workspace_id: prefer the value set by authorizeWorkspaceAccess;
				// fall back to the {workspaceID} URL param for workspace-scoped
				// routes that resolve access in the handler rather than in
				// middleware (so the property is set consistently across both).
				if wsID, wsErr := WorkspaceIDFromContext(r.Context()); wsErr == nil {
					props["workspace_id"] = wsID.String()
				} else if urlWS, ok := uuidURLParam(r, "workspaceID"); ok {
					props["workspace_id"] = urlWS.String()
				}

				// org_id: prefer an explicit {organizationID} from the route;
				// otherwise attribute only when unambiguous (exactly one
				// membership). Never guess among multiple orgs — a stable but
				// possibly-wrong org_id pollutes per-org rollups, so omit it.
				if urlOrg, ok := uuidURLParam(r, "organizationID"); ok {
					props["org_id"] = urlOrg.String()
				} else if len(caller.OrganizationMemberships) == 1 {
					props["org_id"] = SortedOrganizationMemberships(caller.OrganizationMemberships)[0].OrganizationID.String()
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

// uuidURLParam reads a chi URL param and parses it as a UUID. It is safe to
// call post-serve: the RouteContext (and the params populated during routing)
// persists in the request context, which is exactly how RoutePattern() is read
// above. Returns ok=false when the param is absent or not a valid UUID.
func uuidURLParam(r *http.Request, name string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, name)
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
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
						"path", safeRequestLogPath(r.URL.Path),
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
