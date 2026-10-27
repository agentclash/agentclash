package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/enquiries"
	"github.com/agentclash/agentclash/backend/internal/vibe"
)

// Public config owns capability discovery; a flag cannot promise execution.
func TestVibeConfigReportsUnavailableWithoutWorkerEvidence(t *testing.T) {
	cfg := vibe.Config{Enabled: true, Credential: "fixture-secret-do-not-expose", MaterialBuild: false, Profiles: map[string]vibe.ModelProfile{}}
	h := &VibeHandler{Service: &vibe.Service{Config: cfg}, CookieSecret: strings.Repeat("s", 32), Enquiries: &enquiries.Store{Recipient: "operator@example.invalid"}}
	w := httptest.NewRecorder()
	before := time.Now().Add(-time.Second)
	h.Routes().ServeHTTP(w, httptest.NewRequest("GET", "/config", nil))
	if w.Code != 200 {
		t.Fatalf("config unavailable: %d %s", w.Code, w.Body)
	}
	var body struct {
		MaterialBuild *bool     `json:"material_build"`
		ServerTime    time.Time `json:"server_time"`
		Availability  map[string]struct {
			Available  bool   `json:"available"`
			ReasonCode string `json:"reason_code"`
		} `json:"availability"`
		Contact struct {
			Available bool   `json:"available"`
			Email     string `json:"email"`
		} `json:"contact"`
		AnonymousLimits vibe.Limits `json:"anonymous_limits"`
		SignedInLimits  vibe.Limits `json:"signed_in_limits"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.MaterialBuild == nil || *body.MaterialBuild {
		t.Error("missing truthful material_build capability")
	}
	if body.ServerTime.Before(before) || body.ServerTime.After(time.Now().Add(time.Second)) {
		t.Error("missing current server_time")
	}
	for _, scope := range []string{"guest", "workspace"} {
		a, ok := body.Availability[scope]
		if !ok || a.Available || a.ReasonCode == "" {
			t.Errorf("%s availability must fail closed with a reason: %+v", scope, a)
		}
	}
	if body.Contact.Available || body.Contact.Email != "" {
		t.Error("unusable contact destination advertised")
	}
	if body.AnonymousLimits != vibe.LimitsFor(true) || body.SignedInLimits != vibe.LimitsFor(false) {
		t.Error("public UTF-8 byte limits differ from admission limits")
	}
	if strings.Contains(w.Body.String(), cfg.Credential) {
		t.Error("public config exposed provider credential")
	}
}
