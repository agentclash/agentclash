package api

import (
	"context"
	"net/http"
	"time"

	temporalsdk "go.temporal.io/sdk/client"
)

type healthResponse struct {
	OK      bool   `json:"ok"`
	Service string `json:"service"`
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		OK:      true,
		Service: "api-server",
	})
}

// dbPinger is the subset of the Postgres connection pool that the readiness
// probe depends on. *pgxpool.Pool satisfies this.
type dbPinger interface {
	Ping(ctx context.Context) error
}

// temporalHealthChecker is the subset of the Temporal client that the
// readiness probe depends on. temporalsdk.Client satisfies this.
type temporalHealthChecker interface {
	CheckHealth(ctx context.Context, request *temporalsdk.CheckHealthRequest) (*temporalsdk.CheckHealthResponse, error)
}

// readyResponse reports overall readiness plus a per-dependency breakdown,
// so an unhealthy instance is diagnosable from the response body alone.
type readyResponse struct {
	OK      bool              `json:"ok"`
	Service string            `json:"service"`
	Checks  map[string]string `json:"checks"`
}

type readinessOptions struct {
	redis           func(context.Context) error
	shutdownContext context.Context
}

func isDraining(ctx context.Context) bool { return ctx != nil && ctx.Err() != nil }

// This is a process shutdown gate, not a remotely activated migration fence.
func shutdownIngress(ctx context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isDraining(ctx) && r.URL.Path != "/healthz" && r.URL.Path != "/healthz/ready" {
				w.Header().Set("Retry-After", "5")
				writeError(w, http.StatusServiceUnavailable, "service_draining", "service is shutting down")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// healthzReadyHandler checks Postgres, Temporal and configured Redis. The
// shared deadline bounds network probes; absent required dependencies or a
// draining process are unavailable. Disabled Redis is supported for local use.
func healthzReadyHandler(db dbPinger, temporal temporalHealthChecker, options ...readinessOptions) http.HandlerFunc {
	var opts readinessOptions
	if len(options) > 0 {
		opts = options[0]
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if isDraining(opts.shutdownContext) {
			writeJSON(w, http.StatusServiceUnavailable, readyResponse{OK: false, Service: "api-server", Checks: map[string]string{"lifecycle": "draining"}})
			return
		}
		// Bound the entire readiness check so a stalled dependency
		// cannot leave the readiness request pending indefinitely.
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := make(map[string]string, 3)
		ready := true

		if db == nil {
			checks["postgres"] = "not configured"
			ready = false
		} else if err := db.Ping(ctx); err != nil {
			checks["postgres"] = "unreachable"
			ready = false
		} else {
			checks["postgres"] = "ok"
		}

		if temporal == nil {
			checks["temporal"] = "not configured"
			ready = false
		} else if _, err := temporal.CheckHealth(ctx, &temporalsdk.CheckHealthRequest{}); err != nil {
			checks["temporal"] = "unreachable"
			ready = false
		} else {
			checks["temporal"] = "ok"
		}

		if opts.redis == nil {
			checks["redis"] = "disabled"
		} else if err := opts.redis(ctx); err != nil {
			checks["redis"] = "unreachable"
			ready = false
		} else {
			checks["redis"] = "ok"
		}
		if isDraining(opts.shutdownContext) {
			checks["lifecycle"] = "draining"
			ready = false
		}

		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}

		writeJSON(w, status, readyResponse{
			OK:      ready,
			Service: "api-server",
			Checks:  checks,
		})
	}
}
