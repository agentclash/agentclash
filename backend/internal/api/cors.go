package api

import "net/http"

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

func newCORSMiddleware(authMode string) func(http.Handler) http.Handler {
	allowedHeaders := corsAllowedHeaders(authMode)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
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
