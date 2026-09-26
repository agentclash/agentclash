package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
)

func TestVibeBuildRecoveryHTTPKeepsTestsAndWorkingVersion(t *testing.T) {
	h := newReliabilityHarness(t, 1)
	source := "Unknown senders are spam."
	sid, pid, aid, workingID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	block := vibe.SourceBlock{ID: sid.String(), MessageID: sid, Text: source, Hash: vibe.Hash([]byte(source)), OriginalHash: vibe.Hash([]byte(source))}
	policy := vibe.PolicySnapshot{ID: pid, ScopeID: h.session.ID, SourceMessageID: sid, SourceVersion: vibe.SourcePolicyVersion, Sources: []vibe.SourceBlock{block}, Rules: []vibe.PolicyRule{{ID: "sender", Statement: source, SourceBlockIDs: []string{block.ID}, Evidence: []vibe.RuleEvidence{{SourceBlockID: block.ID, Quote: source, Kind: "requirement"}}}}}
	draft := vibe.Artifact{ID: aid, Kind: "test_suite", PolicyID: &pid, Title: "Saved draft", Blueprint: []byte(vibeBlueprint), SourceMessageID: sid, CreatedAt: time.Now()}
	err := h.svc.Store.Edit(h.ctx, h.actor, h.session.ID, h.session.Revision, func(s *vibe.Session) error {
		s.Document.Evaluation = &vibe.EvaluationContext{ID: s.ID, Door: "build"}
		s.Document.TestJourney = true
		s.Document.Messages = []vibe.Message{{ID: sid, Role: "user", Content: source}}
		s.Document.Policies = []vibe.PolicySnapshot{policy}
		s.Document.Artifacts = []vibe.Artifact{{ID: workingID, Kind: "test_suite", AgentPrompt: "Existing sample instructions", Blueprint: draft.Blueprint, Accepted: true}, draft}
		s.Document.ActiveArtifactID = &workingID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	h.reload()
	handler := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	body := mustJSON(t, map[string]any{"revision": h.session.Revision, "artifact_id": aid, "build_from_policy": true})
	patch := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/sessions/"+h.session.ID.String(), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer fixture")
		req.Header.Set(headerUserID, h.user.String())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if w := patch(); w.Code != http.StatusOK {
		t.Fatalf("recovery failed: %d %s", w.Code, w.Body.String())
	}
	h.reload()
	d := h.session.Document
	next := d.Artifacts[len(d.Artifacts)-1]
	if len(d.Artifacts) != 3 || next.ParentID == nil || *next.ParentID != aid || next.Accepted || *d.ActiveArtifactID != workingID || len(h.session.Operations) != 0 {
		t.Fatal("recovery selected or ran a proposal")
	}
	beforeHash, _ := vibe.CanonicalJSONHash(draft.Blueprint)
	afterHash, _ := vibe.CanonicalJSONHash(next.Blueprint)
	if beforeHash != afterHash || !strings.Contains(next.AgentPrompt, source) || strings.Contains(next.AgentPrompt, "30 days") {
		t.Fatal("recovery changed tests or copied their answer key")
	}
	if w := patch(); w.Code != http.StatusConflict {
		t.Fatalf("duplicate revision accepted: %d", w.Code)
	}
	h.reload()
	if len(h.session.Document.Artifacts) != 3 {
		t.Fatal("duplicate recovery created another version")
	}
}
