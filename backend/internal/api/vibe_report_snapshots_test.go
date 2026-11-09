package api

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The real source→HTTP→database owner catches disclosure by whole-object copy,
// lost numeric precision, private-source ACL widening, and retention coupling.
func TestVibeReportSnapshotIndependentSelection(t *testing.T) {
	h, ws, a := sdkHarness(t)
	api := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	source := `{"schema_version":1,"report_id":"PRIVATE_REPORT","generated_at":"2026-10-26T12:00:00Z","runner":{"name":"agentclash-evals","version":"1"},"summary":{"total":1,"passed":0,"failed":1,"skipped":0,"errored":0},"cases":[{"case":{"case_id":"PRIVATE_CASE","name":"PRIVATE_TITLE"},"status":"failed","metrics":[{"key":"PRIVATE_KEY","name":"PRIVATE_NAME","passed":false,"score":0.12345678901234567890123456789,"reason":"PRIVATE_REASON"}],"agent_result":{"input":"PRIVATE_INPUT","output":"PRIVATE_OUTPUT"}}],"exit_code":1}`
	claim := "PRIVATE_REVISION"
	report, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "eval_report", SourceSchemaVersion: 1, SourceJSON: source, Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, ApplicationRevision: &claim, Fingerprints: vibe.Fingerprints{Model: &claim}, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "PRIVATE_CASE", CaseKey: "eligible"}}}}))
	if err != nil {
		t.Fatal(err)
	}
	observed, err := h.svc.Store.ObservedEvidence(h.ctx, h.actor, h.session.ID, report.Binding.ResultRef, &a.ID)
	if err != nil {
		t.Fatal(err)
	}
	entry := observed.Cases[0].Entries[0]
	selection := map[string]any{"client_id": uuid.New(), "artifact_id": a.ID, "result_ref": report.Binding.ResultRef, "evidence_hash": observed.EvidenceHash, "cases": []any{map[string]any{"entry_id": observed.Cases[0].EntryID, "entries": []any{map[string]any{"entry_id": entry.EntryID, "include_text": false}}}}}
	base := "/sessions/" + h.session.ID.String() + "/report-snapshots"
	preview := sdkRequest(api, h.user, "POST", base+"/preview", mustJSON(t, selection))
	if preview.Code != 200 {
		t.Fatalf("selected preview: want 200 got %d %s", preview.Code, preview.Body.String())
	}
	if strings.Contains(preview.Body.String(), "PRIVATE_") || !strings.Contains(preview.Body.String(), "0.12345678901234567890123456789") {
		t.Fatal("unsafe or rounded preview", preview.Body.String())
	}

	// A preview is private and cannot persist a copy. Wrong hashes and foreign
	// coordinates fail at the same owner that creates the copy.
	var count int
	if err = h.db.QueryRow(h.ctx, "SELECT count(*) FROM report_snapshots WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 0 {
		t.Fatal("preview persisted", count, err)
	}
	selection["evidence_hash"] = strings.Repeat("0", 64)
	if w := sdkRequest(api, h.user, "POST", base+"/preview", mustJSON(t, selection)); w.Code == 200 {
		t.Fatal("stale evidence accepted")
	}
	selection["evidence_hash"] = observed.EvidenceHash
	choices := selection["cases"].([]any)[0].(map[string]any)["entries"].([]any)[0].(map[string]any)
	choices["entry_id"] = "foreign-entry"
	if w := sdkRequest(api, h.user, "POST", base+"/preview", mustJSON(t, selection)); w.Code == 200 {
		t.Fatal("foreign coordinate accepted")
	}
	choices["entry_id"] = entry.EntryID
	choices["include_text"] = true
	rawPreview := sdkRequest(api, h.user, "POST", base+"/preview", mustJSON(t, selection))
	if rawPreview.Code != 200 || !strings.Contains(rawPreview.Body.String(), "PRIVATE_REASON") || !strings.Contains(rawPreview.Body.String(), "PRIVATE_NAME") || strings.Contains(rawPreview.Body.String(), "PRIVATE_REVISION") || strings.Contains(rawPreview.Body.String(), "PRIVATE_INPUT") {
		t.Fatal("explicit source text projection", rawPreview.Code, rawPreview.Body.String())
	}
	choices["include_text"] = false
	selection["preview_hash"] = strings.Repeat("0", 64)
	if w := sdkRequest(api, h.user, "POST", base, mustJSON(t, selection)); w.Code == 201 {
		t.Fatal("unreviewed copy accepted")
	}
	var p map[string]any
	json.Unmarshal(preview.Body.Bytes(), &p)
	selection["preview_hash"] = p["content_hash"]
	created := sdkRequest(api, h.user, "POST", base, mustJSON(t, selection))
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var saved map[string]any
	json.Unmarshal(created.Body.Bytes(), &saved)
	id := saved["id"].(string)
	retry := sdkRequest(api, h.user, "POST", base, mustJSON(t, selection))
	var again map[string]any
	json.Unmarshal(retry.Body.Bytes(), &again)
	if retry.Code != 201 || again["id"] != id {
		t.Fatal("retry duplicated copy", retry.Body.String())
	}
	viewer, member, foreign := uuid.New(), uuid.New(), uuid.New()
	for _, u := range []uuid.UUID{viewer, member, foreign} {
		if _, err = h.db.Exec(h.ctx, "INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", u, u.String(), u.String()+"@test.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	for _, x := range []struct {
		u    uuid.UUID
		role string
	}{{viewer, "workspace_viewer"}, {member, "workspace_member"}} {
		if _, err = h.db.Exec(h.ctx, "INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) SELECT organization_id,$1,'org_member','active' FROM workspaces WHERE id=$2", x.u, ws); err != nil {
			t.Fatal(err)
		}
		if _, err = h.db.Exec(h.ctx, "INSERT INTO workspace_memberships(workspace_id,organization_id,user_id,role,membership_status) SELECT id,organization_id,$2,$3,'active' FROM workspaces WHERE id=$1", ws, x.u, x.role); err != nil {
			t.Fatal(err)
		}
	}
	detail := "/workspaces/" + ws.String() + "/report-snapshots/" + id
	for _, u := range []uuid.UUID{viewer, member} {
		w := sdkRequest(api, u, "GET", detail, nil)
		if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE_") {
			t.Fatal("recipient read", w.Code, w.Body.String())
		}
		w = sdkRequest(api, u, "GET", "/sessions/"+h.session.ID.String()+"/reports/"+report.ID.String(), nil)
		if w.Code != 404 {
			t.Fatal("raw ACL widened", w.Code)
		}
	}
	note := map[string]any{"client_id": uuid.New(), "target_id": entry.EntryID, "text": "Member disagrees; original grade is retained."}
	if w := sdkRequest(api, viewer, "POST", detail+"/notes", mustJSON(t, note)); w.Code == 201 {
		t.Fatal("viewer wrote note")
	}
	if w := sdkRequest(api, member, "POST", detail+"/notes", mustJSON(t, note)); w.Code != 201 {
		t.Fatal("member note", w.Code, w.Body.String())
	}

	if w := sdkRequest(api, member, "POST", detail+"/notes", mustJSON(t, note)); w.Code != 201 {
		t.Fatal("note retry", w.Body.String())
	}
	if after := sdkRequest(api, viewer, "GET", detail, nil); after.Body.String() != created.Body.String() {
		t.Fatal("annotation changed original copied grades", after.Body.String())
	}
	notes := sdkRequest(api, viewer, "GET", detail+"/notes", nil)
	var copiedNotes []vibe.ReportNote
	if err = json.Unmarshal(notes.Body.Bytes(), &copiedNotes); err != nil || len(copiedNotes) != 1 {
		t.Fatal("note retry duplicated", err, notes.Body.String())
	}
	if _, err = h.db.Exec(h.ctx, "UPDATE report_snapshot_notes SET text='replacement' WHERE snapshot_id=$1", id); err == nil {
		t.Fatal("annotation rewritten")
	}
	if _, err = h.db.Exec(h.ctx, "UPDATE report_snapshots SET body='{}' WHERE id=$1", id); err == nil {
		t.Fatal("original grade rewritten")
	}
	note["text"] = "conflicting retry"
	if w := sdkRequest(api, member, "POST", detail+"/notes", mustJSON(t, note)); w.Code == 201 {
		t.Fatal("note client ID conflict accepted")
	}
	if w := sdkRequest(api, viewer, "GET", detail+"/links", nil); w.Code == 200 {
		t.Fatal("viewer enumerated capabilities")
	}
	if w := sdkRequest(api, member, "POST", base, mustJSON(t, selection)); w.Code == 201 {
		t.Fatal("member copied actor-private source")
	}
	if w := sdkRequest(api, foreign, "GET", detail, nil); w.Code == 200 {
		t.Fatal("foreign read")
	}
	// Source deletion exercises production cleanup, rather than deleting a fake raw fixture.
	h.reload()
	if _, err = h.db.Exec(h.ctx, `UPDATE vibe_sessions SET document=jsonb_set(document,'{evaluation}','{"door":"test","title":"Fixture"}'::jsonb) WHERE id=$1`, h.session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = h.svc.Store.DeleteProject(h.ctx, h.actor, h.session.ID, h.session.Revision); err != nil {
		t.Fatal(err)
	}
	if err = h.svc.Store.CleanupProjects(h.ctx); err != nil {
		t.Fatal(err)
	}
	selection["client_id"] = uuid.New()
	if w := sdkRequest(api, h.user, "POST", base, mustJSON(t, selection)); w.Code == 201 {
		t.Fatal("copy created after source deletion")
	}
	list := sdkRequest(api, viewer, "GET", "/workspaces/"+ws.String()+"/report-snapshots", nil)
	if list.Code != 200 || !strings.Contains(list.Body.String(), id) {
		t.Fatal("independent recipient navigation", list.Body.String())
	}
	var remaining int
	h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_sdk_reports WHERE id=$1", report.ID).Scan(&remaining)
	if remaining != 0 {
		t.Fatal("source not physically purged")
	}
	w := sdkRequest(api, viewer, "GET", detail, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "0.12345678901234567890123456789") {
		t.Fatal("copy depends on deleted source", w.Code, w.Body.String())
	}

	repo := repository.New(h.db)
	manager := NewPublicShareManager(NewCallerWorkspaceAuthorizer(repo), repo, "http://127.0.0.1:53518")
	memberCaller := callerWithWorkspaceRole(ws, RoleWorkspaceMember)
	memberCaller.UserID = member
	viewerCaller := callerWithWorkspaceRole(ws, RoleWorkspaceViewer)
	viewerCaller.UserID = viewer
	expiry := time.Now().Add(time.Hour)
	input := CreateShareLinkInput{ResourceType: "report_snapshot", ResourceID: uuid.MustParse(id), ExpiresAt: &expiry}
	if _, err = manager.CreateShareLink(h.ctx, viewerCaller, input); err == nil {
		t.Fatal("viewer exposed report")
	}
	link, err := manager.CreateShareLink(h.ctx, memberCaller, input)
	if err != nil {
		t.Fatal("public report link", err)
	}

	// Exercise the actual generic route entrypoints, not just manager methods.
	routes := chi.NewRouter()
	routes.Post("/v1/share-links", createShareLinkHandler(slog.Default(), manager))
	routes.Patch("/v1/share-links/{shareID}", updateShareLinkHandler(slog.Default(), manager))
	routes.Delete("/v1/share-links/{shareID}", revokeShareLinkHandler(slog.Default(), manager))
	routes.Get("/public/shares/{token}", getPublicShareHandler(slog.Default(), manager))
	request := func(caller Caller, method, path string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(string(body)))
		req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, caller))
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, req)
		return w
	}
	body := mustJSON(t, map[string]any{"resource_type": "report_snapshot", "resource_id": id, "expires_at": expiry})
	for _, call := range []struct {
		method, path string
		body         []byte
	}{{"POST", "/v1/share-links", body}, {"PATCH", "/v1/share-links/" + link.Share.ID.String(), []byte(`{"search_indexing":false}`)}, {"DELETE", "/v1/share-links/" + link.Share.ID.String(), nil}} {
		w := request(viewerCaller, call.method, call.path, call.body)
		if w.Code != 403 {
			t.Fatal("generic viewer mutation", call.method, w.Code, w.Body.String())
		}
	}
	if w := request(memberCaller, "PATCH", "/v1/share-links/"+link.Share.ID.String(), []byte(`{"search_indexing":true}`)); w.Code < 400 {
		t.Fatal("generic indexing bypass")
	}
	if w := request(memberCaller, "POST", "/v1/share-links", body); w.Code != 201 {
		t.Fatal("generic member create", w.Code, w.Body.String())
	}
	if w := request(Caller{}, "GET", "/public/shares/"+link.Token, nil); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("actual public token", w.Code, w.Body.String())
	}
	if w := sdkRequest(api, member, "GET", detail+"/links", nil); w.Code != 200 || !strings.Contains(w.Body.String(), link.Token) {
		t.Fatal("persisted link controls", w.Code, w.Body.String())
	}
	payload, err := manager.GetPublicShare(h.ctx, link.Token, "")
	if err != nil {
		t.Fatal(err)
	}
	b := mustJSON(t, payload)
	if strings.Contains(string(b), "PRIVATE_") || strings.Contains(string(b), "Member disagrees") {
		t.Fatal("public raw/annotation leak", string(b))
	}
	if err = manager.RevokeShareLink(h.ctx, viewerCaller, link.Share.ID); err == nil {
		t.Fatal("viewer revoked")
	}
	for _, caller := range []Caller{viewerCaller, memberCaller} {
		if err = manager.SetShareSearchIndexing(h.ctx, caller, link.Share.ID, true); err == nil {
			t.Fatal("report indexed")
		}
	}
	input.SearchIndexing = true
	if _, err = manager.CreateShareLink(h.ctx, memberCaller, input); err == nil {
		t.Fatal("indexing create bypass")
	}
	if _, err = h.db.Exec(h.ctx, "UPDATE public_share_links SET search_indexing=true WHERE id=$1", link.Share.ID); err != nil {
		t.Fatal(err)
	}
	if public, readErr := manager.GetPublicShare(h.ctx, link.Token, ""); readErr != nil || public.Share.SearchIndexing {
		t.Fatal("report capability advertises indexing", readErr)
	}
	assertNotPublished := func() {
		t.Helper()
		if _, err = manager.GetPublication(h.ctx, link.Share.ID); err == nil {
			t.Fatal("forged publication indexed")
		}
		publications, err := manager.ListPublications(h.ctx, nil, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range publications.Items {
			if p.Publication.ID == link.Share.ID {
				t.Fatal("publication list leak")
			}
		}
	}
	assertNotPublished()
	// Active dedup must repair a forged indexing bit without replacing the
	// capability or extending its original expiry, even when asked for longer.
	dedupBody := mustJSON(t, map[string]any{"resource_type": "report_snapshot", "resource_id": id, "search_indexing": false, "expires_at": expiry.Add(time.Hour)})
	dedup := request(memberCaller, "POST", "/v1/share-links", dedupBody)
	if dedup.Code != http.StatusCreated {
		t.Fatal("generic report dedup", dedup.Code, dedup.Body.String())
	}
	var dedupResult createShareLinkResponse
	if err := json.Unmarshal(dedup.Body.Bytes(), &dedupResult); err != nil {
		t.Fatal(err)
	}
	if dedupResult.Share.SearchIndexing || strings.Contains(dedup.Body.String(), `"publication_url"`) {
		t.Error("deduplicated report advertises indexing or publication", dedup.Body.String())
	}
	if dedupResult.Token != link.Token || dedupResult.Share.ID != link.Share.ID || dedupResult.Share.ExpiresAt == nil || !dedupResult.Share.ExpiresAt.Equal(*link.Share.ExpiresAt) {
		t.Error("dedup changed active token or persisted expiry", dedup.Body.String())
	}
	persisted, err := repo.GetPublicShareLinkByID(h.ctx, link.Share.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.SearchIndexing || persisted.Key != link.Token || persisted.ExpiresAt == nil || !persisted.ExpiresAt.Equal(*link.Share.ExpiresAt) {
		t.Error("dedup did not reconcile indexing while preserving capability", persisted)
	}
	assertNotPublished()
	if _, err = h.db.Exec(h.ctx, "UPDATE public_share_links SET expires_at=now()-interval '1 second' WHERE id=$1", link.Share.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.GetPublicShare(h.ctx, link.Token, ""); err == nil {
		t.Fatal("expired capability read")
	}
	input.SearchIndexing = false
	renewed, err := manager.CreateShareLink(h.ctx, memberCaller, input)
	if err != nil || renewed.Token == link.Token {
		t.Fatal("renewal", err)
	}
	if err = manager.RevokeShareLink(h.ctx, memberCaller, renewed.Share.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.GetPublicShare(h.ctx, renewed.Token, ""); err == nil {
		t.Fatal("revoked capability read")
	}

	// Authorization is live: stale caller claims cannot create/manage a capability
	// once actual membership is removed, and copied reads no longer authorize.
	if _, err = h.db.Exec(h.ctx, "UPDATE workspace_memberships SET membership_status='suspended' WHERE user_id=$1", member); err != nil {
		t.Fatal(err)
	}
	if w := sdkRequest(api, member, "GET", detail, nil); w.Code == 200 {
		t.Fatal("removed member reads copy")
	}
	if w := request(memberCaller, "POST", "/v1/share-links", body); w.Code == 201 {
		t.Fatal("stale member exposed report")
	}
	// Actual generic token handler must prohibit cache even on unavailable response.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.Background())
	res := httptest.NewRecorder()
	getPublicShareHandler(slog.Default(), manager)(res, req)
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("token response may be cached")
	}

}

func TestVibeReportSnapshotCanonicalLateEvidence(t *testing.T) {
	h, _, a, run, agent := canonicalHarness(t)
	repo := repository.New(h.db)
	record := func(id, output string) {
		_, err := repo.RecordHostedRunEvent(h.ctx, repository.RecordHostedRunEventParams{Event: runevents.Envelope{EventID: id, SchemaVersion: runevents.SchemaVersionV1, RunID: run, RunAgentID: agent, EventType: runevents.EventTypeSystemOutputFinalized, Source: runevents.SourceHostedExternal, OccurredAt: time.Now().UTC(), Payload: mustJSON(t, map[string]any{"output": output})}})
		if err != nil {
			t.Fatal(err)
		}
	}
	record("initial", "PRIVATE_CANONICAL_OUTPUT")
	handler := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	ref := vibe.ResultRef{Kind: "run", ID: run}
	source, err := h.svc.Store.ObservedEvidence(h.ctx, h.actor, h.session.ID, ref, &a.ID)
	if err != nil {
		t.Fatal(err)
	}
	sel := vibe.ReportSelection{ClientID: uuid.New(), ArtifactID: a.ID, ResultRef: ref, EvidenceHash: source.EvidenceHash}
	for _, c := range source.Cases {
		for _, e := range c.Entries {
			if e.Kind == "output" {
				sel.Cases = append(sel.Cases, vibe.ReportCaseSelection{EntryID: c.EntryID, Entries: []vibe.ReportEntrySelection{{EntryID: e.EntryID, IncludeText: true}}})
				break
			}
		}
		if len(sel.Cases) > 0 {
			break
		}
	}
	base := "/sessions/" + h.session.ID.String() + "/report-snapshots"
	preview := sdkRequest(handler, h.user, "POST", base+"/preview", mustJSON(t, sel))
	var p vibe.ReportSnapshot
	if preview.Code != 200 || json.Unmarshal(preview.Body.Bytes(), &p) != nil {
		t.Fatal("canonical preview", preview.Code, preview.Body.String())
	}
	if !strings.Contains(preview.Body.String(), "PRIVATE_CANONICAL_OUTPUT") || strings.Contains(preview.Body.String(), "APP_TOKEN") || strings.Contains(preview.Body.String(), "https://example.com") {
		t.Fatal("canonical field selection", preview.Body.String())
	}
	if p.Content.Fingerprints == nil || p.Content.Fingerprints.Model != nil {
		t.Fatal("external model identity invented", p.Content.Fingerprints)
	}
	sel.PreviewHash = p.ContentHash
	// The run is already terminal. A retained late event changes evidence without
	// changing the project revision or selected coordinate.
	record("late", "PRIVATE_LATE_OUTPUT")
	if w := sdkRequest(handler, h.user, "POST", base, mustJSON(t, sel)); w.Code == 201 {
		t.Fatal("terminal result accepted stale preview", w.Body.String())
	}
	fresh, err := h.svc.Store.ObservedEvidence(h.ctx, h.actor, h.session.ID, ref, &a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.EvidenceHash == source.EvidenceHash {
		t.Fatal("fixture did not advance normalized evidence")
	}
	sel.EvidenceHash = fresh.EvidenceHash
	preview = sdkRequest(handler, h.user, "POST", base+"/preview", mustJSON(t, sel))
	if preview.Code != 200 || json.Unmarshal(preview.Body.Bytes(), &p) != nil {
		t.Fatal(preview.Body.String())
	}
	sel.PreviewHash = p.ContentHash
	created := sdkRequest(handler, h.user, "POST", base, mustJSON(t, sel))
	if created.Code != 201 || !strings.Contains(created.Body.String(), "PRIVATE_CANONICAL_OUTPUT") || strings.Contains(created.Body.String(), "PRIVATE_LATE_OUTPUT") {
		t.Fatal("reconciled selected copy changed scope", created.Code, created.Body.String())
	}

	// A delete in progress owns the same private-parent lock. The create must
	// wait and then reject the newly deleted parent, never resurrect a copy.
	tx, err := h.db.Begin(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(h.ctx)
	if _, err = tx.Exec(h.ctx, "UPDATE vibe_sessions SET deleted_at=now() WHERE id=$1", h.session.ID); err != nil {
		t.Fatal(err)
	}
	sel.ClientID = uuid.New()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- sdkRequest(handler, h.user, "POST", base, mustJSON(t, sel)) }()
	deadline := time.Now().Add(3 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		select {
		case w := <-done:
			t.Fatalf("copy bypassed deletion lock: %d %s", w.Code, w.Body.String())
		default:
		}
		if err = h.db.QueryRow(h.ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id,actor,workspace_id%')`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("copy never reached the parent lock")
	}
	if err = tx.Commit(h.ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-done:
		if w.Code == 201 {
			t.Fatal("copy survived concurrent parent deletion")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("copy did not settle after deletion")
	}
	var copies int
	if err = h.db.QueryRow(h.ctx, "SELECT count(*) FROM report_snapshots WHERE workspace_id=$1", h.session.WorkspaceID).Scan(&copies); err != nil || copies != 1 {
		t.Fatal("deletion admitted another copy", copies, err)
	}
}
