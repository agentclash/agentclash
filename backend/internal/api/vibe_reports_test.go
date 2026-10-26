package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func sdkHarness(t *testing.T) (*vibeAPIHarness, uuid.UUID, vibe.Artifact) {
	t.Helper()
	h := newVibeAPIHarness(t)
	org, ws := uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO organizations(id,name,slug) VALUES($1,'SDK fixture',$2)", []any{org, org.String()}},
		{"INSERT INTO workspaces(id,organization_id,name,slug) VALUES($1,$2,'SDK fixture',$3)", []any{ws, org, ws.String()}},
		{"INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) VALUES($1,$2,'org_admin','active')", []any{org, h.user}},
		{"UPDATE vibe_sessions SET workspace_id=$2 WHERE id=$1", []any{h.session.ID, ws}},
	} {
		if _, err := h.db.Exec(h.ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.svc.Import(h.ctx, h.actor, h.session.ID, h.session.Revision, []byte(vibeBlueprint)); err != nil {
		t.Fatal(err)
	}
	h.reload()
	return h, ws, h.session.Document.Artifacts[0]
}
func sdkRequest(api http.Handler, user uuid.UUID, method, path string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer fixture")
	r.Header.Set(headerUserID, user.String())
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	return w
}
func TestVibeSDKPrivateImmutableImport(t *testing.T) {
	h, ws, a := sdkHarness(t)
	_ = ws
	api := chi.NewRouter()
	(&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).RegisterSDKReports(api)
	base := "/sessions/" + h.session.ID.String() + "/reports"
	source := " \r\n{\"input\":\"hello\",\"output\":\"literal \\u0061\"}\t\n"
	in := vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "agent_result", SourceSchemaVersion: 1, ExternalReportID: "local-1", SourceJSON: source, Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "local-test", CaseKey: "eligible"}}}}
	body := mustJSON(t, in)
	w := sdkRequest(api, h.user, "POST", base, body)
	if w.Code != 201 {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}
	var first vibe.SDKReport
	json.Unmarshal(w.Body.Bytes(), &first)
	if first.EvidenceMetadata.Origin != "imported" || first.Binding.ArtifactID != a.ID || first.ContentHash != vibe.Hash([]byte(source)) {
		t.Fatal("wrong receipt", first)
	}
	var group sync.WaitGroup
	for i := 0; i < 5; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			w := sdkRequest(api, h.user, "POST", base, body)
			var r vibe.SDKReport
			json.Unmarshal(w.Body.Bytes(), &r)
			if w.Code != 201 || r.ID != first.ID {
				t.Errorf("retry: %d %s", w.Code, w.Body.String())
			}
		}()
	}
	group.Wait()
	in.Metadata.ApplicationRevision = new(string)
	*in.Metadata.ApplicationRevision = "different"
	if w := sdkRequest(api, h.user, "POST", base, mustJSON(t, in)); w.Code != 409 {
		t.Fatal("metadata conflict", w.Code, w.Body.String())
	}
	for _, change := range []func(*vibe.SDKReportImport){func(i *vibe.SDKReportImport) { i.Metadata.Cases[0].CaseKey = "unknown" }, func(i *vibe.SDKReportImport) { i.Metadata.ArtifactID = uuid.New() }, func(i *vibe.SDKReportImport) { i.Metadata.Cases = append(i.Metadata.Cases, i.Metadata.Cases[0]) }} {
		var other vibe.SDKReportImport
		json.Unmarshal(body, &other)
		other.ExternalReportID = uuid.NewString()
		change(&other)
		if w := sdkRequest(api, h.user, "POST", base, mustJSON(t, other)); w.Code == 201 {
			t.Fatal("invalid binding accepted")
		}
	}
	w = sdkRequest(api, h.user, "GET", base+"/"+first.ID.String()+"/original", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), []byte(source)) {
		t.Fatal("original changed", w.Code)
	}
	other := uuid.New()
	if _, err := h.db.Exec(h.ctx, "INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", other, other.String(), other.String()+"@test.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Exec(h.ctx, "INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) SELECT organization_id,$1,'org_admin','active' FROM workspaces WHERE id=$2", other, ws); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST"} {
		path := base
		if method == "GET" {
			path += "/" + first.ID.String() + "/original"
		}
		if w := sdkRequest(api, other, method, path, body); w.Code != 404 {
			t.Fatal("second member raw access", w.Code, w.Body.String())
		}
	}
	h.reload()
	if len(h.session.Operations) != 0 {
		t.Fatal("import scheduled work")
	}
	var rows, stored int
	if err := h.db.QueryRow(h.ctx, "SELECT count(*),sum(storage_bytes) FROM vibe_sdk_reports WHERE session_id=$1", h.session.ID).Scan(&rows, &stored); err != nil || rows != 1 || stored <= len(source) {
		t.Fatal("storage receipt", rows, stored, err)
	}
	var accounted, actual int
	if err := h.db.QueryRow(h.ctx, `SELECT storage_bytes,octet_length(source_json)+octet_length(body::text)+octet_length(external_report_id)+octet_length(request_hash)+octet_length(content_sha256) FROM vibe_sdk_reports WHERE id=$1`, first.ID).Scan(&accounted, &actual); err != nil || accounted != actual {
		t.Fatal("persisted projection bytes not accounted", accounted, actual, err)
	}
	if _, err := h.db.Exec(h.ctx, "UPDATE vibe_sdk_reports SET source_json=$2 WHERE id=$1", first.ID, []byte(`{}`)); err == nil {
		t.Fatal("immutable source changed")
	}
}
func TestVibeSDKReviewAndExportKeepSourceSeparate(t *testing.T) {
	h, ws, a := sdkHarness(t)
	handler := &VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}
	api := chi.NewRouter()
	handler.RegisterSDKReports(api)
	api.Mount("/", handler.Routes())
	source := `{"schema_version":1,"report_id":"sdk-grade","generated_at":"2026-10-26T12:00:00Z","runner":{"name":"agentclash-evals","version":"1"},"summary":{"total":1,"passed":0,"failed":1,"skipped":0,"errored":0},"cases":[{"case":{"case_id":"python_test","name":"Refund","expected":"Explain refund eligibility"},"status":"failed","metrics":[{"key":"imported-rule","name":"Rule","passed":false,"score":0.25,"reason":"Original reason"}],"agent_result":{"input":"hello","output":"Refund allowed."}}],"exit_code":1}`
	in := vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "eval_report", SourceSchemaVersion: 1, SourceJSON: source, Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "python_test", CaseKey: "eligible"}}}}
	r, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in))
	if err != nil {
		t.Fatal(err)
	}
	base := "/sessions/" + h.session.ID.String()
	detail := base + "/reports/" + r.ID.String()
	w := sdkRequest(api, h.user, "GET", detail+"/case?source_case_id=python_test", nil)
	if w.Code != 200 {
		t.Fatalf("case: %d %s", w.Code, w.Body.String())
	}
	var c vibe.CaseResult
	json.Unmarshal(w.Body.Bytes(), &c)
	review := vibe.GradeReviewInput{ClientID: uuid.New(), ResultRef: r.Binding.ResultRef, ArtifactID: a.ID, CaseKey: "eligible", SourceEvidenceHash: c.EvidenceHash, Reason: "I reviewed this original judgment"}
	saved, err := h.svc.Store.AddGradeReview(h.ctx, h.actor, h.session.ID, review)
	if err != nil {
		t.Fatal("review", err)
	}
	list, err := h.svc.Store.GradeReviews(h.ctx, h.actor, h.session.ID, &r.ID)
	if err != nil || len(list) != 1 || list[0].EvidenceChanged {
		t.Fatal("review source lookup", list, err)
	}
	w = sdkRequest(api, h.user, "POST", base+"/export", mustJSON(t, map[string]any{"scope": "selected", "artifact_id": a.ID, "result_ref": r.Binding.ResultRef}))
	if w.Code != 200 {
		t.Fatal("SDK export", w.Code, w.Body.String())
	}
	var exported map[string]any
	json.Unmarshal(w.Body.Bytes(), &exported)
	if len(exported["sdk_reports"].([]any)) != 1 || len(exported["runs"].([]any)) != 0 {
		t.Fatal("SDK fabricated run")
	}
	found := false
	for _, item := range exported["items"].([]any) {
		v := item.(map[string]any)
		if v["kind"] == "grade_review" && v["id"] == saved.ID.String() {
			found = true
		}
	}
	if !found {
		t.Fatal("SDK review omitted")
	}
	original, err := h.svc.Store.SDKReport(h.ctx, h.actor, h.session.ID, r.ID)
	if err != nil || original.SourceJSON != source {
		t.Fatal("review/export changed original", err)
	}
	// Definition import of the exported v2 archive never installs its SDK grades.
	delete(exported, "items")
	copy, err := h.svc.Store.CreateSession(h.ctx, h.actor, &ws, uuid.New(), h.svc.Config.DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	if err = h.svc.Import(h.ctx, h.actor, copy.ID, copy.Revision, mustJSON(t, exported)); err != nil {
		t.Fatal("SDK archive definition import", err)
	}
	copy, err = h.svc.Store.GetSession(h.ctx, h.actor, copy.ID)
	if err != nil || len(copy.Operations) != 0 || len(copy.Document.Artifacts) != 1 {
		t.Fatal("archive created execution", err)
	}
	imported, err := h.svc.Store.SDKReports(h.ctx, h.actor, copy.ID)
	if err != nil || len(imported) != 0 {
		t.Fatal("archive installed SDK evidence without explicit import", err)
	}
}
func TestVibeSDKRegradeUsesQuotedRetainedOutput(t *testing.T) {
	h, ws, a := sdkHarness(t)
	h.svc.Config.GroundedJudging = true
	h.svc.Config.FreeOnly = false
	h.svc.Config.LocalTesting = false
	profile := h.svc.Config.Profiles[h.svc.Config.DefaultModel]
	model := "openai/gpt-4o-mini"
	h.svc.Config.DefaultModel = model
	profile.ID = model
	profile.Route = "openai"
	profile.Free = false
	profile.InputNanoPerToken = 400
	profile.OutputNanoPerToken = 1600
	h.svc.Config.Profiles[model] = profile
	h.runner.Gateway.Config = h.svc.Config
	var org uuid.UUID
	if err := h.db.QueryRow(h.ctx, "SELECT organization_id FROM workspaces WHERE id=$1", ws).Scan(&org); err != nil {
		t.Fatal(err)
	}
	account := "org:" + org.String()
	source := `{"input":"Can I get a refund on day 10?","output":"A refund is allowed within 30 days.","tool_calls":[]}`
	in := vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "agent_result", SourceSchemaVersion: 1, SourceJSON: source, Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "python-test", CaseKey: "eligible"}}}}
	in.Metadata.CaptureScope.Tools = vibe.DimensionCapture{Scope: "complete", Instrumentation: "instrumented"}
	r, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in))
	if err != nil {
		t.Fatal(err)
	}
	sub := vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "retest", Purpose: "regrade", ArtifactID: &a.ID, Models: h.svc.Config.DefaultModels(), RegradeSourceResultRef: &r.Binding.ResultRef, SourceEvidenceHash: r.ContentHash}
	q, err := h.svc.QuoteRun(h.ctx, h.actor, h.session.ID, sub)
	if err != nil {
		t.Fatal("quote", err)
	}
	sub.RunQuoteID = &q.ID
	// Same-price source substitution must invalidate the quote as well.
	in.ExternalReportID = "another"
	in.SourceJSON = `{"input":"Can I get a refund on day 10?","output":"Another retained reply.","tool_calls":[]}`
	other, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in))
	if err != nil {
		t.Fatal(err)
	}
	forged := sub
	forged.RegradeSourceResultRef = &other.Binding.ResultRef
	forged.SourceEvidenceHash = other.ContentHash
	if _, err = h.svc.Prepare(h.ctx, h.actor, h.session.ID, forged); err == nil {
		t.Fatal("quote accepted another source")
	}
	if _, err = h.svc.Prepare(h.ctx, h.actor, h.session.ID, sub); err == nil {
		t.Fatal("unfunded SDK regrade admitted")
	}
	if _, err = h.db.Exec(h.ctx, "INSERT INTO vibe_accounts(id,balance) VALUES($1,$2)", account, vibe.NanoUSD); err != nil {
		t.Fatal(err)
	}
	op, err := h.svc.Prepare(h.ctx, h.actor, h.session.ID, sub)
	if err != nil {
		t.Fatal("admit", err)
	}
	var reserved int64
	if err = h.db.QueryRow(h.ctx, "SELECT held FROM vibe_accounts WHERE id=$1", account).Scan(&reserved); err != nil || reserved != q.MaxCost || reserved <= 0 {
		t.Fatal("regrade not funded", reserved, q.MaxCost, err)
	}
	if op.EvidenceMetadata == nil || op.EvidenceMetadata.Origin != "regraded" || op.EvidenceMetadata.SourceResultRef == nil || op.EvidenceMetadata.SourceResultRef.ID != r.ID {
		t.Fatal("regrade provenance", op)
	}
	if op.EvidenceMetadata.CaptureScope.Tools.Scope != "unknown" {
		t.Fatal("hosted regrade promoted imported capture claims", op.EvidenceMetadata.CaptureScope)
	}
	calls := 0
	h.runner.Gateway.Client = groundingClient(func(_ context.Context, req provider.Request) (provider.Response, error) {
		calls++
		if !strings.Contains(req.Messages[0].Content, "Every finding has exactly") {
			t.Fatal("SDK regrade called target/author")
		}
		zero := json.Number("0")
		return provider.Response{OutputText: `{"key":"accuracy","pass":true,"reasoning":"Retained reply satisfies the policy.","finding":{"kind":"observed","quotes":[{"message_id":"output","text":"A refund is allowed within 30 days."}],"missing":"","covered_message_ids":[]}}`, Usage: provider.Usage{CostUSD: &zero}}, nil
	})
	if _, err = h.executeOperation(op.ID); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("unexpected evaluator count", calls)
	}
	if err = h.db.QueryRow(h.ctx, "SELECT held FROM vibe_accounts WHERE id=$1", account).Scan(&reserved); err != nil || reserved != 0 {
		t.Fatal("settlement did not release regrade reservation", reserved, err)
	}
	original, err := h.svc.Store.SDKReport(h.ctx, h.actor, h.session.ID, r.ID)
	if err != nil || original.SourceJSON != source {
		t.Fatal("original altered", err)
	}
	var holds int
	if err = h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_operations WHERE session_id=$1", h.session.ID).Scan(&holds); err != nil || holds != 1 {
		t.Fatal("operation count", holds, err)
	}
}
func TestVibeSDKPromotionRequiresReviewedExactSource(t *testing.T) {
	h, ws, initial := sdkHarness(t)
	var createErr error
	h.session, createErr = h.svc.Store.CreateAgent(h.ctx, h.actor, &ws, uuid.New(), "test", h.svc.Config.DefaultModels())
	if createErr != nil {
		t.Fatal(createErr)
	}
	if createErr = h.svc.Import(h.ctx, h.actor, h.session.ID, h.session.Revision, initial.Blueprint); createErr != nil {
		t.Fatal(createErr)
	}
	h.reload()
	bp, err := h.svc.Compiler.Draft(vibe.DraftProposal{TestsOnly: true, Title: "Policy", Summary: "Explicit expectations", Scenarios: []vibe.TestScenario{{Input: "Hello", Expected: "Say hello"}}, SuccessCriteria: "Follow the expected behavior."}, h.svc.Config.Limits(false))
	if err != nil {
		t.Fatal(err)
	}
	if err = h.svc.Import(h.ctx, h.actor, h.session.ID, h.session.Revision, bp); err != nil {
		t.Fatal(err)
	}
	h.reload()
	a := h.session.Document.Artifacts[len(h.session.Document.Artifacts)-1]
	source := `{"schema_version":1,"report_id":"expectation","generated_at":"2026-10-26T12:00:00Z","runner":{"name":"agentclash-evals","version":"1"},"summary":{"total":1,"passed":0,"failed":1,"skipped":0,"errored":0},"cases":[{"case":{"case_id":"local","name":"Greeting","expected":"Say hello politely"},"status":"failed","metrics":[]}],"exit_code":1}`
	in := vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "eval_report", SourceSchemaVersion: 1, SourceJSON: source, Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "local", CaseKey: "case-1"}}}}
	report, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in))
	if err != nil {
		t.Fatal(err)
	}
	input := vibe.SDKPromotion{ClientID: uuid.New(), Revision: h.session.Revision, SourceCaseID: "local", SourceEvidenceHash: report.ContentHash, Expected: "Say hello politely"}
	if _, err = h.svc.PromoteSDKExpectation(h.ctx, h.actor, h.session.ID, report.ID, input); err == nil {
		t.Fatal("unreviewed expectation promoted")
	}
	input.Reviewed = true
	promoted, err := h.svc.PromoteSDKExpectation(h.ctx, h.actor, h.session.ID, report.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := h.svc.PromoteSDKExpectation(h.ctx, h.actor, h.session.ID, report.ID, input)
	if err != nil || promoted.ID != repeated.ID {
		t.Fatal("promotion retry", err)
	}
	h.reload()
	if len(h.session.Document.Artifacts) != 3 || promoted.ID == a.ID || string(h.session.Document.Artifacts[1].Blueprint) != string(a.Blueprint) || !bytes.Contains(promoted.Blueprint, []byte("Say hello politely")) {
		t.Fatal("promotion changed old tests or failed new version")
	}
	original, err := h.svc.Store.SDKReport(h.ctx, h.actor, h.session.ID, report.ID)
	if err != nil || original.SourceJSON != source {
		t.Fatal("promotion changed source", err)
	}
	// Deletion must erase the retained original and promotion snapshot, not
	// merely deny reads through the soft-deleted session.
	if _, err = h.svc.Store.DeleteProject(h.ctx, h.actor, h.session.ID, h.session.Revision); err != nil {
		t.Fatal(err)
	}
	if err = h.svc.Store.CleanupProjects(h.ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"vibe_sdk_reports", "vibe_sdk_promotions"} {
		var retained int
		if err = h.db.QueryRow(h.ctx, "SELECT count(*) FROM "+table+" WHERE session_id=$1", h.session.ID).Scan(&retained); err != nil || retained != 0 {
			t.Fatal("private SDK content survived physical cleanup", table, retained, err)
		}
	}
}

func TestVibeSDKPublishedMappingSurvivesDraftDeletion(t *testing.T) {
	h, ws, _ := sdkHarness(t)
	bundle := vibeReliabilityBundle(t)
	if err := h.svc.Import(h.ctx, h.actor, h.session.ID, h.session.Revision, mustJSON(t, bundle)); err != nil {
		t.Fatal(err)
	}
	h.reload()
	a := h.session.Document.Artifacts[len(h.session.Document.Artifacts)-1]
	draft, err := h.svc.SaveAdvanced(h.ctx, h.actor, h.session.ID, vibe.AdvancedRequest{Revision: h.session.Revision, ArtifactID: a.ID, WorkspaceID: ws})
	if err != nil {
		t.Fatal(err)
	}
	var composition []byte
	var org uuid.UUID
	if err = h.db.QueryRow(h.ctx, "SELECT composition FROM challenge_pack_drafts WHERE id=$1", draft).Scan(&composition); err != nil {
		t.Fatal(err)
	}
	if err = h.db.QueryRow(h.ctx, "SELECT organization_id FROM workspaces WHERE id=$1", ws).Scan(&org); err != nil {
		t.Fatal(err)
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
	published, err := repo.PublishChallengePackBundle(h.ctx, repository.PublishChallengePackBundleParams{OrganizationID: org, WorkspaceID: ws, Bundle: composed, Source: &repository.PackPublicationSource{DraftID: draft, Composition: composition}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Exec(h.ctx, "DELETE FROM challenge_pack_drafts WHERE id=$1", draft); err != nil {
		t.Fatal(err)
	}
	tuple := composed.InputSets[0].Cases[0]
	setKey := composed.InputSets[0].Key
	in := vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "agent_result", SourceSchemaVersion: 1, SourceJSON: `{"input":"Original input","output":"Retained reply"}`, ExternalReportID: "published-local", Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, ChallengePackVersionID: &published.ChallengePackVersionID, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "arbitrary python test ID", CaseKey: tuple.EffectiveKey()}}}}
	receipt, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in))
	if err != nil {
		t.Fatal("mapping requires deleted editable draft", err)
	}
	var canonicalID uuid.UUID
	if err = h.db.QueryRow(h.ctx, "SELECT b.published_case_id FROM vibe_published_case_bindings b JOIN challenge_input_items i ON i.id=b.published_case_id WHERE b.draft_id=$1 AND b.challenge_pack_version_id=$2 AND b.case_key=$3 AND b.input_set_key=$4 AND b.challenge_key=$5", draft, published.ChallengePackVersionID, tuple.EffectiveKey(), setKey, tuple.ChallengeKey).Scan(&canonicalID); err != nil {
		t.Fatal(err)
	}
	if receipt.Cases[0].PublishedCaseID == nil || *receipt.Cases[0].PublishedCaseID != canonicalID || receipt.Cases[0].InputSetKey == nil || *receipt.Cases[0].InputSetKey != setKey {
		t.Fatal("not actual canonical identity", receipt.Cases)
	}
	in.ExternalReportID = "wrong-case"
	foreign := uuid.New()
	in.Metadata.Cases[0].PublishedCaseID = &foreign
	if _, err = h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in)); err == nil {
		t.Fatal("foreign canonical case accepted")
	}
	in.ExternalReportID = "wrong-version"
	in.Metadata.Cases[0].PublishedCaseID = nil
	in.Metadata.ChallengePackVersionID = &foreign
	if _, err = h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in)); err == nil {
		t.Fatal("foreign canonical version accepted")
	}
	// An explicitly retained receipt can contain the same key in different input sets.
	// Exercise the consumer at its persisted producer boundary, not source order.
	mappings := []vibe.CaseMapping{{CaseKey: tuple.EffectiveKey(), InputSetKey: setKey, ChallengeKey: tuple.ChallengeKey}, {CaseKey: tuple.EffectiveKey(), InputSetKey: "another-set", ChallengeKey: tuple.ChallengeKey}}
	if _, err = h.db.Exec(h.ctx, "UPDATE vibe_pack_sources SET case_keys=$2 WHERE draft_id=$1", draft, mustJSON(t, mappings)); err != nil {
		t.Fatal(err)
	}
	in.ExternalReportID = "ambiguous"
	in.Metadata.ChallengePackVersionID = nil
	if _, err = h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in)); err == nil {
		t.Fatal("ambiguous key picked first")
	}
	in.Metadata.Cases[0].InputSetKey = &setKey
	if _, err = h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in)); err != nil {
		t.Fatal("exact local tuple rejected", err)
	}

	in.SourceKind = "eval_report"
	in.ExternalReportID = "repeated-keys"
	in.SourceJSON = `{"schema_version":1,"report_id":"repeated-keys","generated_at":"2026-10-26T12:00:00Z","runner":{"name":"agentclash-evals","version":"1"},"summary":{"total":2,"passed":1,"failed":1,"skipped":0,"errored":0},"cases":[{"case":{"case_id":"alpha","name":"First","expected":"First expectation"},"status":"passed","metrics":[],"agent_result":{"input":"First input","output":"First reply"}},{"case":{"case_id":"beta","name":"Second","expected":"Second expectation"},"status":"failed","metrics":[],"agent_result":{"input":"Second input","output":"Second reply"}}],"exit_code":1}`
	otherSet := "another-set"
	in.Metadata.Cases = []vibe.SDKCaseBinding{{SourceCaseID: "alpha", CaseKey: tuple.EffectiveKey(), InputSetKey: &setKey}, {SourceCaseID: "beta", CaseKey: tuple.EffectiveKey(), InputSetKey: &otherSet}}
	repeated, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in))
	if err != nil {
		t.Fatal("full tuples with repeated local key", err)
	}
	repeated, err = h.svc.Store.SDKReport(h.ctx, h.actor, h.session.ID, repeated.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = vibe.SDKReportCase(repeated, "", tuple.EffectiveKey()); err == nil {
		t.Fatal("detail guessed first ambiguous source case")
	}
	second, err := vibe.SDKReportCase(repeated, "beta", tuple.EffectiveKey())
	if err != nil || second.Output != "Second reply" {
		t.Fatal("detail chose another source", second, err)
	}
	review := vibe.GradeReviewInput{ClientID: uuid.New(), ResultRef: repeated.Binding.ResultRef, ArtifactID: a.ID, CaseKey: tuple.EffectiveKey(), SourceEvidenceHash: vibe.Hash(mustJSON(t, second)), Reason: "Review second source only"}
	if _, err = h.svc.Store.AddGradeReview(h.ctx, h.actor, h.session.ID, review); err == nil {
		t.Fatal("review guessed first ambiguous source case")
	}
	review.SourceCaseID = "beta"
	saved, err := h.svc.Store.AddGradeReview(h.ctx, h.actor, h.session.ID, review)
	if err != nil || saved.Evidence.Output != "Second reply" {
		t.Fatal("review lost source selector", err)
	}
	rows, err := h.svc.Store.GradeReviews(h.ctx, h.actor, h.session.ID, &repeated.ID)
	if err != nil || len(rows) != 1 || rows[0].EvidenceChanged {
		t.Fatal("ambiguous review reload lost source selector", err)
	}
}
func TestVibeSDKCountAndOriginalAllowanceAreAtomic(t *testing.T) {
	h, _, a := sdkHarness(t)
	in := vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "agent_result", SourceSchemaVersion: 1, SourceJSON: `{"input":"Original input","output":"retained"}`, Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "local", CaseKey: "eligible"}}}}
	var first vibe.SDKReport
	for i := 0; i < vibe.SDKReportCount; i++ {
		in.ExternalReportID = fmt.Sprintf("report-%d", i)
		r, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in))
		if err != nil {
			t.Fatalf("report %d: %v", i, err)
		}
		if i == 0 {
			first = r
		}
	}
	in.ExternalReportID = "too-many"
	if _, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in)); err == nil {
		t.Fatal("count limit ignored")
	}
	in.ExternalReportID = "report-0"
	retry, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in))
	if err != nil || retry.ID != first.ID {
		t.Fatal("retry charged count again", err)
	}
	prep, err := h.svc.Store.PrepareExport(h.ctx, h.actor, h.session.ID, vibe.ExportRequest{Scope: "history", ArtifactID: a.ID})
	if err != nil || len(prep.SDKReports) != vibe.SDKReportCount {
		t.Fatal("history omitted reports", err)
	}
	h2, _, a2 := sdkHarness(t)
	// Persisted original files already consume the shared allowance. Leave space
	// for the raw SDK bytes but not its binding/metadata projection.
	room := len(in.SourceJSON) + 1
	if _, err = h2.db.Exec(h2.ctx, "UPDATE vibe_import_originals SET content=$2 WHERE session_id=$1", h2.session.ID, bytes.Repeat([]byte(" "), 4<<20)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		size := 4 << 20
		if i == 3 {
			size -= room
		}
		if _, err = h2.db.Exec(h2.ctx, `INSERT INTO vibe_import_originals(id,session_id,artifact_id,filename,media_type,content_sha256,content) VALUES($1,$2,$3,'fixture.json','application/json','fixture',$4)`, uuid.New(), h2.session.ID, uuid.New(), bytes.Repeat([]byte(" "), size)); err != nil {
			t.Fatal(err)
		}
	}
	in.Metadata.ArtifactID = a2.ID
	in.ExternalReportID = "metadata-overflow"
	if _, err = h2.svc.Store.ImportSDKReport(h2.ctx, h2.actor, h2.session.ID, mustJSON(t, in)); err == nil {
		t.Fatal("projection storage was not counted")
	}
	var count int
	if err = h2.db.QueryRow(h2.ctx, "SELECT count(*) FROM vibe_sdk_reports WHERE session_id=$1", h2.session.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("rejected quota left receipt", count, err)
	}
}

func TestVibeSDKMountedTransportPreservesEscapedOriginal(t *testing.T) {
	h, _, a := sdkHarness(t)
	api := chi.NewRouter()
	api.Mount("/v1/vibe", (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes())
	base := "/v1/vibe/sessions/" + h.session.ID.String() + "/reports"
	messages := make([]map[string]string, 20)
	for i := range messages {
		messages[i] = map[string]string{"role": "assistant", "content": strings.Repeat("\"\\\n", 5000)}
	}
	source := " \r\n" + string(mustJSON(t, map[string]any{"input": "hello", "output": "retained reply", "messages": messages})) + "\t\n"
	in := vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "agent_result", SourceSchemaVersion: 1, SourceJSON: source, Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "local", CaseKey: "eligible"}}}}
	body := mustJSON(t, in)
	if len(source) <= 256<<10 || len(source) > vibe.SDKReportSourceBytes || len(body) <= 1<<20 {
		t.Fatal("fixture must exercise escaped transport", len(source), len(body))
	}
	w := sdkRequest(api, h.user, "POST", base, body)
	if w.Code != 201 {
		t.Fatalf("mounted SDK import: %d %s", w.Code, w.Body.String())
	}
	var report vibe.SDKReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	w = sdkRequest(api, h.user, "GET", base+"/"+report.ID.String()+"/original", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), []byte(source)) {
		t.Fatal("mounted original changed", w.Code)
	}
	// Only the exact import route gets the expanded ceiling; sibling mutations
	// still reject this large body before parsing/admission.
	for _, path := range []string{"/v1/vibe/sessions/" + h.session.ID.String() + "/messages", base + "/" + report.ID.String() + "/promote", base + "/extra"} {
		w = sdkRequest(api, h.user, "POST", path, body)
		if w.Code != 413 {
			t.Fatal("expanded unrelated mutation", path, w.Code, w.Body.String())
		}
	}
}
