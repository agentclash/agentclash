package api

import (
	"net/http"
	"strings"
)

// corsAllowedHeaders returns the set of allowed CORS headers based on auth mode.
// In dev mode, the X-Agentclash-* identity headers are allowed so the development
// authenticator works from browser requests. In production (workos), only
// standard headers are permitted — browser-controlled identity headers are blocked.
func corsAllowedHeaders(authMode string) string {
	const base = "Content-Type, Authorization"
	if authMode == "dev" {
		return base + ", X-Agentclash-User-Id, X-Agentclash-WorkOS-User-Id, X-Agentclash-User-Email, X-Agentclash-User-Display-Name, X-Agentclash-Org-Memberships, X-Agentclash-Workspace-Memberships"
	}
	return base
}

// newCORSMiddleware builds CORS middleware that controls which origins are
// allowed. allowedOrigins is a pre-parsed set of origin strings (e.g.
// "https://app.agentclash.com"). When the set is empty and authMode is "dev",
// the middleware falls back to the wildcard "*". When the set is empty and
// authMode is anything else (production), no Access-Control-Allow-Origin header
// is sent — the browser's same-origin policy remains in effect.
func newCORSMiddleware(authMode string, allowedOrigins map[string]struct{}) func(http.Handler) http.Handler {
	allowedHeaders := corsAllowedHeaders(authMode)
	wildcard := len(allowedOrigins) == 0 && authMode == "dev"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" {
				if _, ok := allowedOrigins[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// parseCORSOrigins converts a comma-separated origin string into a lookup set.
// Empty input returns an empty map.
func parseCORSOrigins(raw string) map[string]struct{} {
	origins := make(map[string]struct{})
	if raw == "" {
		return origins
	}
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			origins[trimmed] = struct{}{}
		}
	}
	return origins
}
