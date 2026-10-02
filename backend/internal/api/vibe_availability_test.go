package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

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
