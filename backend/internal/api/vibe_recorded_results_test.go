package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

// Exercise the current public intake, not a seeded legacy conversation artifact.
func TestVibeCurrentRecordedIntakeQuoteAndExecution(t *testing.T) {
	h := newVibeAPIHarness(t)
	h.svc.Config.TwoDoor = true
	api := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	request := func(path string, body any) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, bytes.NewReader(mustJSON(t, body)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set(headerUserID, h.user.String())
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	id := uuid.New()
	w := request("/sessions", map[string]any{"id": id, "door": "test"})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var session vibe.Session
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	calls := 0
	h.runner.Gateway.Client = groundingClient(func(_ context.Context, r provider.Request) (provider.Response, error) {
		calls++
		if !strings.Contains(r.Messages[1].Content, "I can refund that.") {
			t.Fatal("recorded reply missing; target generation must not replace it")
		}
		zero := json.Number("0")
		return provider.Response{OutputText: `{"checks":[{"key":"returns","verdict":"FAIL","evidence":"Opened purchases cannot be refunded.","message_ids":["c1-m2"]}]}`, ProviderModelID: r.Model, Usage: provider.Usage{CostUSD: &zero}}, nil
	})
	input := map[string]any{"client_id": uuid.New(), "revision": session.Revision, "label": "App replies", "content": "Customer: I opened it.\r\nAgent: I can refund that.\r\n", "expectations": []vibe.Expectation{{ID: "returns", Statement: "Opened purchases cannot be refunded."}}}
	path := "/sessions/" + id.String()
	w = request(path+"/evidence", input)
	if w.Code != 200 {
		t.Fatal("current V1 recorded intake unavailable", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || len(session.Operations) != 0 || len(session.Document.Artifacts) != 1 || len(session.Document.EvidenceSets) != 1 {
		t.Fatal("intake executed or failed atomic artifact/evidence creation")
	}
	a := session.Document.Artifacts[0]
	e := session.Document.EvidenceSets[0]
	if !a.IsConversationEvaluation() || a.ConversationEvaluation.EvidenceSetID != e.ID || e.Raw != input["content"] {
		t.Fatal("source bytes or artifact binding changed")
	}
	// A lost response must recover the exact receipt despite the advanced revision.
	w = request(path+"/evidence", input)
	if w.Code != 200 {
		t.Fatal("intake receipt not recovered", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if len(session.Document.Artifacts) != 1 || len(session.Document.EvidenceSets) != 1 || session.Document.Artifacts[0].ID != a.ID {
		t.Fatal("ambiguous response duplicated source")
	}
	input["content"] = "Customer: Different\nAgent: Different"
	if w = request(path+"/evidence", input); w.Code != 409 {
		t.Fatal("conflicting intake identity accepted", w.Code, w.Body.String())
	}

	// A receipt still names the original source after another intake advances the project.
	input["content"] = "Customer: I opened it.\r\nAgent: I can refund that.\r\n"
	later := map[string]any{}
	_ = json.Unmarshal(mustJSON(t, input), &later)
	later["client_id"] = uuid.New()
	later["revision"] = session.Revision
	w = request(path+"/evidence", later)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(path+"/evidence", input)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &session)
	if session.EvidenceReceipt == nil || session.EvidenceReceipt.ArtifactID != a.ID || len(session.Document.Artifacts) != 2 {
		t.Fatal("receipt retargeted to latest intake", session.EvidenceReceipt)
	}
	sub := map[string]any{"client_id": uuid.New(), "revision": session.Revision, "kind": "check", "artifact_id": a.ID, "evidence_set_id": e.ID, "approve_artifact": true, "models": h.svc.Config.DefaultModels()}
	w = request(path+"/run-quote", sub)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var q vibe.RunQuote
	_ = json.Unmarshal(w.Body.Bytes(), &q)
	if q.Calls != 1 || q.Cases != 1 || calls != 0 {
		t.Fatal("quote generated evidence or included target calls", q)
	}
	sub["run_quote_id"] = q.ID
	w = request(path+"/messages", sub)
	if w.Code != 202 && w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var admitted vibe.Operation
	_ = json.Unmarshal(w.Body.Bytes(), &admitted)
	h.session = session
	result, err := h.executeOperation(admitted.ID)
	if err != nil || calls != 1 || result.Scorecard == nil || result.Scorecard.Failed != 1 || result.Source.Kind != "provided_conversations" {
		t.Fatal("recorded evaluator-only execution failed", result, err, calls)
	}
	var projection map[string]any
	_ = json.Unmarshal(mustJSON(t, result), &projection)
	metadata, ok := projection["evidence_metadata"].(map[string]any)
	if !ok || metadata["origin"] != "recorded" {
		t.Fatal("recorded provenance missing from authoritative result", projection["evidence_metadata"])
	}
	saved, err := h.svc.Store.GetCase(h.ctx, h.actor, result.ID, result.Results[0].CaseKey)
	if err != nil || len(saved.Messages) != 2 || saved.Messages[1].Content != "I can refund that.\r\n" {
		t.Fatal("recorded original lost", saved, err)
	}
}
