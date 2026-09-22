package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
	"github.com/google/uuid"
)

func TestVibeInteractionHTTPBoundary(t *testing.T) {
	h := newReliabilityHarness(t, 1)
	h.svc.Config.PreciseActions = true
	scope, msg, qid := uuid.NewString(), uuid.New(), uuid.NewString()
	q := interaction.Question{ID: qid, ScopeID: scope, Revision: 1, OriginMessageID: msg.String(), Purpose: "clarify_rule", Status: "active", Text: "Return within 14 days or 30 days?", Options: []interaction.Option{{ID: "14", Label: "14 days"}, {ID: "30", Label: "30 days"}}, MaxSelections: 1}
	if err := h.svc.Store.Edit(h.ctx, h.actor, h.session.ID, h.session.Revision, func(s *vibe.Session) error {
		s.Document.Messages = append(s.Document.Messages, vibe.Message{ID: msg, Role: "assistant", Content: q.Text})
		s.Document.ConversationState = &vibe.ConversationState{Version: 1, ActionsVersion: 1, Brief: interaction.Brief{ScopeID: scope, Revision: 1, Facts: []interaction.Fact{}}, PendingQuestion: &q}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h.reload()
	api := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	a := interaction.Action{IdempotencyKey: uuid.NewString(), ScopeID: scope, SessionRevision: h.session.Revision, Kind: "answer_question", TargetID: qid, TargetRevision: 1, OptionIDs: []string{"14"}}
	request := func(a interaction.Action, user uuid.UUID, extra bool) *httptest.ResponseRecorder {
		payload, _ := json.Marshal(a)
		var p map[string]any
		_ = json.Unmarshal(payload, &p)
		if extra {
			p["run"] = true
		}
		b, _ := json.Marshal(map[string]any{"version": 1, "kind": "action", "payload": p})
		r := httptest.NewRequest(http.MethodPost, "/sessions/"+h.session.ID.String()+"/actions", bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set(headerUserID, user.String())
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	if w := request(a, h.user, true); w.Code != 400 {
		t.Fatal("unknown instruction accepted", w.Code, w.Body.String())
	}
	for _, kind := range []string{"run", "save"} {
		bad := a
		bad.Kind = kind
		if w := request(bad, h.user, false); w.Code != 400 {
			t.Fatal("setup action initiated execution", w.Code)
		}
	}
	if w := request(a, uuid.New(), false); w.Code != 404 && w.Code != 403 {
		t.Fatal("cross-session choice authorized", w.Code, w.Body.String())
	}
	if w := request(a, h.user, false); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(a, h.user, false); w.Code != 200 {
		t.Fatal("identical delivery did not recover", w.Code, w.Body.String())
	}
	h.reload()
	if len(h.session.Operations) != 0 || len(h.session.Document.ConversationState.Answers) != 1 {
		t.Fatal("HTTP action dispatched inference or duplicated answer")
	}
	bad := a
	bad.IdempotencyKey = uuid.NewString()
	if w := request(bad, h.user, false); w.Code != 409 {
		t.Fatal("stale tab changed state", w.Code, w.Body.String())
	}
	bad = a
	bad.OptionIDs = []string{"30"}
	if w := request(bad, h.user, false); w.Code != 409 {
		t.Fatal("idempotency key reused for different option", w.Code)
	}
}
