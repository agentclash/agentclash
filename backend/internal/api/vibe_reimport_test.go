package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
)

func TestVibeSemanticReimportRequiresReviewedNewVersion(t *testing.T) {
	h := newVibeAPIHarness(t)
	if err := h.svc.Import(h.ctx, h.actor, h.session.ID, h.session.Revision, []byte(vibeBlueprint)); err != nil {
		t.Fatal(err)
	}
	h.reload()
	a := h.session.Document.Artifacts[0]
	var definition map[string]any
	if err := json.Unmarshal(a.Blueprint, &definition); err != nil {
		t.Fatal(err)
	}
	cases := definition["cases"].([]any)
	cases[0].(map[string]any)["payload"].(map[string]any)["expected"] = "approved refund"
	// Reordering is not a deletion/addition; identity is the authored key.
	cases[0], cases[2] = cases[2], cases[0]
	changed := a
	changed.Blueprint = mustJSON(t, definition)
	exported := map[string]any{"format": "agentclash-evaluation-v2", "session_id": h.session.ID, "artifact_id": a.ID, "artifacts": []vibe.Artifact{changed}, "rules": []any{}, "runs": []any{map[string]any{"state": "COMPLETED", "scorecard": map[string]any{"passed": 999}}}, "case_mappings": []any{map[string]any{"case_key": "eligible", "challenge_key": "refund", "input_set_key": "main", "published_case_id": "retained-claim"}}, "manifest": map[string]any{"version": 2, "completeness": "partial", "partial_confirmed": true, "missing": []any{map[string]any{"kind": "case", "id": "missing-evidence"}}}}
	content := mustJSON(t, exported)
	api := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	request := func(body any) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/sessions/"+h.session.ID.String()+"/reimport", bytes.NewReader(mustJSON(t, body)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set(headerUserID, h.user.String())
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	in := map[string]any{"revision": h.session.Revision, "artifact_id": a.ID, "content": string(content)}
	w := request(in)
	if w.Code != 200 {
		t.Fatalf("semantic review: %d %s", w.Code, w.Body.String())
	}
	var preview struct {
		ReviewHash string `json:"review_hash"`
		Changes    []struct {
			Path   string `json:"path"`
			Before any    `json:"before"`
			After  any    `json:"after"`
		} `json:"changes"`
		Partial bool `json:"partial"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Partial || preview.ReviewHash == "" || len(preview.Changes) != 1 || preview.Changes[0].Path != "/evaluation/cases/eligible/payload/expected" || preview.Changes[0].Before != "refund" || preview.Changes[0].After != "approved refund" {
		t.Fatalf("wrong semantic review: %s", w.Body.String())
	}
	h.reload()
	if len(h.session.Document.Artifacts) != 1 || len(h.session.Operations) != 0 {
		t.Fatal("review mutated the project")
	}
	// A foreign export, omitted authored case, or changed bytes cannot reuse
	// the reviewed identity. None of these reads may allocate a version.
	foreign := make(map[string]any)
	for key, value := range exported {
		foreign[key] = value
	}
	foreignID := uuid.New()
	foreignArtifact := changed
	foreignArtifact.ID = foreignID
	foreign["artifact_id"] = foreignID
	foreign["artifacts"] = []vibe.Artifact{foreignArtifact}
	in["content"] = string(mustJSON(t, foreign))
	if w = request(in); w.Code != 400 {
		t.Fatalf("foreign source: %d %s", w.Code, w.Body.String())
	}
	omitted := make(map[string]any)
	_ = json.Unmarshal(changed.Blueprint, &omitted)
	omitted["cases"] = omitted["cases"].([]any)[:2]
	partialArtifact := changed
	partialArtifact.Blueprint = mustJSON(t, omitted)
	partial := make(map[string]any)
	for key, value := range exported {
		partial[key] = value
	}
	partial["artifacts"] = []vibe.Artifact{partialArtifact}
	in["content"] = string(mustJSON(t, partial))
	if w = request(in); w.Code != 400 {
		t.Fatalf("partial authored deletion: %d %s", w.Code, w.Body.String())
	}
	in["content"] = string(content) + " "
	in["apply"] = true
	in["review_hash"] = preview.ReviewHash
	if w = request(in); w.Code != 409 {
		t.Fatalf("changed exact bytes: %d %s", w.Code, w.Body.String())
	}
	in["content"] = string(content)
	in["apply"] = true
	in["review_hash"] = "unreviewed"
	if w = request(in); w.Code != 409 {
		t.Fatalf("unreviewed apply accepted: %d %s", w.Code, w.Body.String())
	}
	in["review_hash"] = preview.ReviewHash
	if w = request(in); w.Code != 200 {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}
	h.reload()
	if len(h.session.Document.Artifacts) != 2 || len(h.session.Operations) != 0 || len(h.session.Document.Policies) != 0 {
		t.Fatal("apply replaced history or invented execution/policy")
	}
	next := h.session.Document.Artifacts[1]
	var nextDefinition any
	var expectedDefinition any
	_ = json.Unmarshal(next.Blueprint, &nextDefinition)
	_ = json.Unmarshal(changed.Blueprint, &expectedDefinition)
	if next.ID == a.ID || next.ParentID == nil || *next.ParentID != a.ID || next.Accepted || !reflect.DeepEqual(nextDefinition, expectedDefinition) {
		t.Fatal("new unapproved version changed authored definition or lineage")
	}
	original, err := h.svc.Store.GetOriginal(h.ctx, h.actor, h.session.ID, next.ID)
	if err != nil || !bytes.Equal(original.Content, content) {
		t.Fatal("historical claims/mappings/fingerprints not retained exactly", err)
	}
	if w = request(in); w.Code != 409 {
		t.Fatalf("stale apply accepted: %d", w.Code)
	}
}
