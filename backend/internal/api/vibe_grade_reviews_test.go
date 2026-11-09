package api

import (
	"bytes"
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
	"net/http/httptest"
	"testing"
)

func TestVibeGradeReviewBindsExactEvidenceAndIdempotency(t *testing.T) {
	h := newVibeAPIHarness(t)
	a := vibe.Artifact{ID: uuid.New(), Kind: "test_suite"}
	sub := vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "check", Models: h.svc.Config.DefaultModels(), ArtifactID: &a.ID}
	p := vibe.Plan{Submission: sub, Artifact: &a, Cases: []string{"case"}, ChecksPerCase: 1, Calls: 1, Free: true, LocalTesting: true}
	op, err := h.svc.Store.Submit(h.ctx, h.actor, h.session.ID, sub, p, h.svc.Config)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.svc.Store.Stop(h.ctx, h.actor, op.ID); err != nil {
		t.Fatal(err)
	}
	c := vibe.CaseResult{CaseKey: "case", Version: a.ID.String(), Output: "Original response.", Expected: "Follow the rule.", ExpectedChecks: 1, Verdict: vibe.Fail, Checks: []vibe.CheckResult{{Key: "rule", Verdict: vibe.Fail, Evidence: "Wrong response."}}}
	if err = h.svc.Store.PutResult(h.ctx, op.ID, c); err != nil {
		t.Fatal(err)
	}
	api := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	request := func(method string, body any, user uuid.UUID) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/sessions/"+h.session.ID.String()+"/grade-reviews", bytes.NewReader(mustJSON(t, body)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set(headerUserID, user.String())
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	body := map[string]any{"source_evidence_hash": vibe.Hash(mustJSON(t, c)), "client_id": uuid.New(), "result_ref": map[string]any{"kind": "vibe_operation", "id": op.ID}, "artifact_id": a.ID, "case_key": "case", "check_key": "rule", "reason": "The rule permits this response.", "quotes": []map[string]string{{"message_id": "output", "text": "Original response."}}}
	w := request("POST", body, h.user)
	if w.Code != 201 {
		t.Fatal("grade dispute unavailable", w.Code, w.Body.String())
	}
	var review map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &review)
	w = request("POST", body, h.user)
	var replay map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &replay)
	if w.Code != 201 || review["id"] != replay["id"] {
		t.Fatal("lost response duplicated review", w.Code, w.Body.String())
	}
	body["reason"] = "Changed reason"
	if w = request("POST", body, h.user); w.Code != 409 {
		t.Fatal("review identity accepted different content", w.Code)
	}
	body["client_id"] = uuid.New()
	body["quotes"] = []map[string]string{{"message_id": "output", "text": "Never present"}}
	if w = request("POST", body, h.user); w.Code != 400 {
		t.Fatal("fabricated evidence quote accepted", w.Code, w.Body.String())
	}
	if w = request("GET", nil, uuid.New()); w.Code != 404 && w.Code != 403 {
		t.Fatal("foreign actor read private reviews", w.Code)
	}
	c.Output = "Late overwritten response."
	c.Verdict = vibe.Pass
	if err = h.svc.Store.PutResult(h.ctx, op.ID, c); err != nil {
		t.Fatal(err)
	}
	body["quotes"] = []map[string]string{}
	body["client_id"] = uuid.New()
	if w = request("POST", body, h.user); w.Code != 409 {
		t.Fatal("late evidence retargeted a new review instead of rejecting stale context", w.Code, w.Body.String())
	}
	w = request("GET", nil, h.user)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var reviews []struct {
		Evidence        vibe.CaseResult `json:"evidence"`
		EvidenceChanged bool            `json:"evidence_changed"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &reviews); err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 1 || reviews[0].Evidence.Output != "Original response." || reviews[0].Evidence.Verdict != vibe.Fail || !reviews[0].EvidenceChanged {
		t.Fatal("review silently retargeted late evidence", w.Body.String())
	}
}
