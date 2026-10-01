package api

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
)

func TestVibeRetryCooldownHTTP(t *testing.T) {
	available := time.Now().UTC().Add(30 * time.Second)
	w := httptest.NewRecorder()
	vibeError(w, &vibe.Fault{Code: "retry_cooldown", Message: "Wait for the provider", RetryAvailableAt: &available})
	var body struct {
		Error vibe.Fault `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	seconds, err := strconv.Atoi(w.Header().Get("Retry-After"))
	if err != nil || w.Code != 429 || seconds < 29 || seconds > 30 || body.Error.RetryAvailableAt == nil || !body.Error.RetryAvailableAt.Equal(available) {
		t.Fatalf("cooldown lost at HTTP boundary: %d %v %v", w.Code, w.Header(), body)
	}
}
