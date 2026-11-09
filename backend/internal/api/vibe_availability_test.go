package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/enquiries"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
)

// Disabled execution must not prevent an owner reading or deleting their work.
// Exercise real handlers and persisted state, rather than only a config flag.
func TestVibeIntegrationDisabledAdmissionPreservesHistory(t *testing.T) {
	h := newVibeAPIHarness(t)
	evaluation, err := h.svc.Store.CreateEvaluation(h.ctx, h.actor, h.session.ID, uuid.New(), "build")
	if err != nil {
		t.Fatal(err)
	}
	h.session = evaluation
	h.svc.Config.Enabled = false
	handler := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set(headerUserID, h.user.String())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/sessions/" + h.session.ID.String()
	if w := request("POST", "/sessions", map[string]any{"id": uuid.New(), "door": "build"}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled creation: %d %s", w.Code, w.Body.String())
	}
	if w := request("POST", path+"/build-quote", map[string]any{"content": "Summarize meeting notes", "models": h.svc.Config.DefaultModels()}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled quote: %d %s", w.Code, w.Body.String())
	}
	if w := request("GET", path, nil); w.Code != http.StatusOK {
		t.Fatalf("authorized history unavailable: %d %s", w.Code, w.Body.String())
	}
	var quotes int
	if err := h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_cycle_quotes WHERE session_id=$1", h.session.ID).Scan(&quotes); err != nil || quotes != 0 {
		t.Fatalf("disabled execution persisted a quote: %d %v", quotes, err)
	}
	if w := request("DELETE", path+"?revision="+strconv.FormatInt(h.session.Revision, 10), nil); w.Code != http.StatusAccepted {
		t.Fatalf("authorized deletion unavailable: %d %s", w.Code, w.Body.String())
	}
}

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
