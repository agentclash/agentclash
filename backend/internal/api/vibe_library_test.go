package api

import (
	"bytes"
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/google/uuid"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestVibeLibraryOriginalBytesAndFrozenExport(t *testing.T) {
	h := newVibeAPIHarness(t)
	var err error
	h.session, err = h.svc.Store.CreateAgent(h.ctx, h.actor, nil, uuid.New(), "test", h.svc.Config.DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	api := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	request := func(method, path string, body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set(headerUserID, h.user.String())
		r.Header.Set("If-Match", strconv.FormatInt(h.session.Revision, 10))
		r.Header.Set("X-File-Name", "escaped%20original.json")
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	bundle := vibeReliabilityBundle(t)
	bundle.Version.EvaluationSpec.Metrics = []scoring.MetricDeclaration{{Key: "tokens", Type: scoring.MetricTypeNumeric, Collector: "run_total_tokens"}}
	encoded, _ := json.MarshalIndent(bundle, "", "  ")
	original := append([]byte{0xef, 0xbb, 0xbf}, []byte("\r\n"+strings.ReplaceAll(string(encoded), "\n", "\r\n")+"\r\n")...)
	base := "/sessions/" + h.session.ID.String()
	w := request("POST", base+"/import", original)
	if w.Code != 200 {
		t.Fatalf("original import: %d %s", w.Code, w.Body.String())
	}
	h.reload()
	a := h.session.Document.Artifacts[len(h.session.Document.Artifacts)-1]
	w = request("GET", base+"/artifacts/"+a.ID.String()+"/original", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), original) {
		t.Fatalf("original bytes changed: status %d, got %d want %d", w.Code, w.Body.Len(), len(original))
	}
	org, ws := uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO organizations(id,name,slug) VALUES($1,'Library test',$2)", []any{org, org.String()}},
		{"INSERT INTO workspaces(id,organization_id,name,slug) VALUES($1,$2,'Library test',$3)", []any{ws, org, ws.String()}},
		{"INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) VALUES($1,$2,'org_admin','active')", []any{org, h.user}},
	} {
		if _, err = h.db.Exec(h.ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	w = request("POST", base+"/advanced", mustJSON(t, map[string]any{"revision": h.session.Revision, "artifact_id": a.ID, "workspace_id": ws}))
	if w.Code != 200 {
		t.Fatalf("advanced handoff: %d %s", w.Code, w.Body.String())
	}
	var receipt struct {
		DraftID uuid.UUID `json:"draft_id"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	var copied []byte
	if err = h.db.QueryRow(h.ctx, "SELECT original_content FROM vibe_pack_sources WHERE draft_id=$1", receipt.DraftID).Scan(&copied); err != nil || !bytes.Equal(copied, original) {
		t.Fatal("advanced source copy changed bytes", err)
	}
	var composition []byte
	if err = h.db.QueryRow(h.ctx, "SELECT composition FROM challenge_pack_drafts WHERE id=$1", receipt.DraftID).Scan(&composition); err != nil || !bytes.Contains(composition, []byte("run_total_tokens")) {
		t.Fatal("advanced metric lost", err)
	}
	var comp challengepack.Composition
	if err = json.Unmarshal(composition, &comp); err != nil {
		t.Fatal(err)
	}
	composed, err := challengepack.ComposeBundle(comp, challengepack.ResolvedPieces{})
	if err != nil {
		t.Fatal(err)
	}
	repo := repository.New(h.db)
	published, err := repo.PublishChallengePackBundle(h.ctx, repository.PublishChallengePackBundleParams{OrganizationID: org, WorkspaceID: ws, Bundle: composed, Source: &repository.PackPublicationSource{DraftID: receipt.DraftID, Composition: composition}})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_published_case_bindings b JOIN challenge_input_items i ON i.id=b.published_case_id AND i.item_key=b.case_key WHERE b.draft_id=$1 AND b.challenge_pack_version_id=$2", receipt.DraftID, published.ChallengePackVersionID).Scan(&count); err != nil || count != len(composed.InputSets[0].Cases) {
		t.Fatal("published case IDs unresolved", count, err)
	}

	// Publishing an edited builder copy must not claim the original version's mapping.
	comp.Version.Number = 2
	edited, err := challengepack.ComposeBundle(comp, challengepack.ResolvedPieces{})
	if err != nil {
		t.Fatal(err)
	}
	publishedEdited, err := repo.PublishChallengePackBundle(h.ctx, repository.PublishChallengePackBundleParams{OrganizationID: org, WorkspaceID: ws, Bundle: edited, Source: &repository.PackPublicationSource{DraftID: receipt.DraftID, Composition: mustJSON(t, comp)}})
	if err != nil {
		t.Fatal(err)
	}
	if err = h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_published_case_bindings WHERE draft_id=$1 AND challenge_pack_version_id=$2", receipt.DraftID, publishedEdited.ChallengePackVersionID).Scan(&count); err != nil || count != 0 {
		t.Fatal("edited draft claimed original IDs", count, err)
	}

	if err = repo.DeleteChallengePackDraft(h.ctx, receipt.DraftID); err == nil {
		t.Fatal("published source receipt was deleted")
	}
	if err = h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_published_case_bindings WHERE draft_id=$1 AND challenge_pack_version_id=$2", receipt.DraftID, published.ChallengePackVersionID).Scan(&count); err != nil || count != len(composed.InputSets[0].Cases) {
		t.Fatal("earlier immutable mapping lost", count, err)
	}
	h.reload()
	sub := vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "check", Models: h.svc.Config.DefaultModels()}
	op, err := h.svc.Store.Submit(h.ctx, h.actor, h.session.ID, sub, vibe.Plan{Submission: sub, Artifact: &a, Cases: []string{"one"}, ChecksPerCase: 1, Calls: 1, Free: true, LocalTesting: true}, h.svc.Config)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.svc.Store.Stop(h.ctx, h.actor, op.ID); err != nil {
		t.Fatal(err)
	}
	c := vibe.CaseResult{CaseKey: "one", Version: a.ID.String(), Output: "Original reply", Verdict: vibe.Fail, ExpectedChecks: 1, Checks: []vibe.CheckResult{{Key: "rule", Verdict: vibe.Fail}}}
	if err = h.svc.Store.PutResult(h.ctx, op.ID, c); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 51; i++ {
		if _, err = h.svc.Store.AddGradeReview(h.ctx, h.actor, h.session.ID, vibe.GradeReviewInput{ClientID: uuid.New(), ResultRef: vibe.ResultRef{Kind: vibe.ResultVibeOperation, ID: op.ID}, ArtifactID: a.ID, CaseKey: "one", Reason: "Review this exact grade", SourceEvidenceHash: vibe.Hash(mustJSON(t, c))}); err != nil {
			t.Fatal(err)
		}
	}
	w = request("POST", base+"/export", mustJSON(t, map[string]any{"scope": "selected", "artifact_id": a.ID, "result_id": op.ID}))
	if w.Code != 200 {
		t.Fatalf("export preparation: %d %s", w.Code, w.Body.String())
	}
	var prepared vibe.ExportPreparation
	if err = json.Unmarshal(w.Body.Bytes(), &prepared); err != nil {
		t.Fatal(err)
	}
	if len(prepared.Runs) != 1 || prepared.Runs[0].Scorecard.Failed != 1 {
		t.Fatal("export source summary", w.Body.String())
	}
	var reviews int
	for _, item := range prepared.Items {
		if item.Kind == "grade_review" {
			if item.ArtifactID == nil || *item.ArtifactID != a.ID {
				t.Fatal("review lost its exact artifact identity", item)
			}
			reviews++
		}
	}
	if reviews != 51 {
		t.Fatal("export truncated review history", reviews)
	}
	var hash string
	for _, item := range prepared.Items {
		if item.Kind == "case" {
			hash = item.Hash
		}
	}
	if hash == "" {
		t.Fatal("source case hash missing", w.Body.String())
	}

	for _, item := range prepared.Items {
		if item.Kind == "grade_review" {
			w = request("GET", base+"/export/reviews/"+item.ID+"?kind=grade_review&hash="+item.Hash, nil)
			if w.Code != 200 {
				t.Fatal("exact review read", w.Code, w.Body.String())
			}
			break
		}
	}
	c.Output = "Late reply"
	c.Verdict = vibe.Pass
	if err = h.svc.Store.PutResult(h.ctx, op.ID, c); err != nil {
		t.Fatal(err)
	}
	w = request("GET", "/operations/"+op.ID.String()+"/case?key=one&hash="+hash, nil)
	if w.Code != 409 {
		t.Fatalf("late evidence accepted against frozen summary: %d %s", w.Code, w.Body.String())
	}

	// Older results without an admitted scope remain exportable only as explicit partial evidence.
	if _, err = h.db.Exec(h.ctx, "UPDATE vibe_operations SET input=input-'artifact'-'case_keys' WHERE id=$1", op.ID); err != nil {
		t.Fatal(err)
	}
	w = request("POST", base+"/export", mustJSON(t, map[string]any{"scope": "selected", "artifact_id": a.ID, "result_id": op.ID}))
	if w.Code != 200 {
		t.Fatal("legacy export preparation", w.Code, w.Body.String())
	}
	var legacy vibe.ExportPreparation
	if err = json.Unmarshal(w.Body.Bytes(), &legacy); err != nil {
		t.Fatal(err)
	}
	missingScope := false
	for _, item := range legacy.Items {
		if item.Kind == "scope" && item.Missing != "" {
			missingScope = true
		}
	}
	if !missingScope || len(legacy.Runs) != 1 {
		t.Fatal("legacy scope was invented", w.Body.String())
	}
	// Raw private copies disappear on deletion, independently saved copies survive.
	h.reload()
	if _, err = h.svc.Store.DeleteProject(h.ctx, h.actor, h.session.ID, h.session.Revision); err != nil {
		t.Fatal(err)
	}
	w = request("GET", base+"/artifacts/"+a.ID.String()+"/original", nil)
	if w.Code != 404 {
		t.Fatal("deleted private original remained accessible", w.Code)
	}
	w = request("GET", "/saved-originals/"+receipt.DraftID.String(), nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), original) {
		t.Fatal("independent saved original lost", w.Code)
	}

}

func TestVibeLibraryMetadataOwnershipConflictAndActiveArchive(t *testing.T) {
	h := newVibeAPIHarness(t)
	var createErr error
	h.session, createErr = h.svc.Store.CreateAgent(h.ctx, h.actor, nil, uuid.New(), "test", h.svc.Config.DefaultModels())
	if createErr != nil {
		t.Fatal(createErr)
	}
	api := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	request := func(method, path string, body any, user uuid.UUID) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(mustJSON(t, body)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set(headerUserID, user.String())
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	sub := vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "check", Models: h.svc.Config.DefaultModels()}
	a := vibe.Artifact{ID: uuid.New(), Kind: "test_suite"}
	op, err := h.svc.Store.Submit(h.ctx, h.actor, h.session.ID, sub, vibe.Plan{Submission: sub, Artifact: &a, Cases: []string{"one"}, ChecksPerCase: 1, Calls: 1, Free: true, LocalTesting: true}, h.svc.Config)
	if err != nil {
		t.Fatal(err)
	}
	current, err := h.svc.Store.GetSession(h.ctx, h.actor, h.session.ID)
	if err != nil {
		t.Fatal(err)
	}
	path := "/sessions/" + h.session.ID.String() + "/metadata"
	w := request("PATCH", path, map[string]any{"revision": current.Revision, "title": "  A named project  ", "archived": true}, h.user)
	if w.Code != 200 {
		t.Fatalf("metadata during active run: %d %s", w.Code, w.Body.String())
	}
	after, err := h.svc.Store.GetSession(h.ctx, h.actor, h.session.ID)
	if err != nil || after.Title != "A named project" || after.Revision != current.Revision+1 {
		t.Fatal("metadata not durable", after.Title, err)
	}
	savedOp, err := h.svc.Store.Operation(h.ctx, op.ID)
	if err != nil || savedOp.State != op.State || savedOp.Billing != op.Billing {
		t.Fatal("archive changed execution", err)
	}
	if w = request("PATCH", path, map[string]any{"revision": current.Revision, "title": "stale"}, h.user); w.Code != 409 {
		t.Fatal("stale rename accepted", w.Code)
	}
	if w = request("PATCH", path, map[string]any{"revision": after.Revision, "title": "foreign"}, uuid.New()); w.Code != 404 && w.Code != 403 {
		t.Fatal("foreign actor renamed", w.Code)
	}
	for _, mode := range []string{"", "?archived=only"} {
		w = request("GET", "/sessions"+mode, nil, h.user)
		var rows []vibe.Session
		if err = json.Unmarshal(w.Body.Bytes(), &rows); err != nil || w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		found := false
		for _, row := range rows {
			found = found || row.ID == after.ID
		}
		if found != (mode != "") {
			t.Fatal("archive discovery", mode, found)
		}
	}

	if w = request("PATCH", path, map[string]any{"revision": after.Revision, "title": strings.Repeat("é", 129)}, h.user); w.Code != 400 {
		t.Fatal("UTF-8 title overflow accepted", w.Code)
	}
	w = request("PATCH", path, map[string]any{"revision": after.Revision, "title": strings.Repeat("é", 128)}, h.user)
	if w.Code != 200 {
		t.Fatal("exact UTF-8 title boundary rejected", w.Code, w.Body.String())
	}
	after, err = h.svc.Store.GetSession(h.ctx, h.actor, h.session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if w = request("PATCH", path, map[string]any{"revision": after.Revision, "archived": false}, h.user); w.Code != 200 {
		t.Fatal("unarchive", w.Code, w.Body.String())
	}
}
