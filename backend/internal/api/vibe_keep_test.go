package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
)

func TestVibeExpiredAuthenticationHTTP(t *testing.T) {
	h := (&VibeHandler{Service: &vibe.Service{}, Auth: NewDevelopmentAuthenticator()}).Routes()
	for _, endpoint := range []string{"/saved-checks", "/sessions/a8000000-0000-4000-8000-000000000001/save", "/sessions/a8000000-0000-4000-8000-000000000001/save-brief"} {
		method := http.MethodPost
		if endpoint == "/saved-checks" {
			method = http.MethodGet
		}
		r := httptest.NewRequest(method, endpoint, strings.NewReader("{}"))
		r.Header.Set("Authorization", "Bearer expired")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "unauthenticated") {
			t.Fatalf("%s lost authentication failure: %d %s", endpoint, w.Code, w.Body.String())
		}
	}
}
