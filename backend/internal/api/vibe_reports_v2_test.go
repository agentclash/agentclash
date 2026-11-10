package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/productanalytics"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func refundImport(t *testing.T, a vibe.Artifact) vibe.SDKReportImport {
	t.Helper()
	b, err := os.ReadFile("../../../schemas/refund-report-v2/fixtures/openai-correct-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	return vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "eval_report", SourceSchemaVersion: 2, SourceJSON: " \r\n" + string(b) + "\t\n", Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "DEMO-repeat-refund-01", CaseKey: "eligible"}}}}
}
func TestVibeSDKRefundV2HTTPRoundtrip(t *testing.T) {
	h, ws, a := sdkHarness(t)
	spy := &recordingPostHog{}
	h.svc.Store.WithProductAnalytics(productanalytics.New(spy))
	h.svc.Config.GroundedJudging = true
	api := chi.NewRouter()
	api.Mount("/v1/vibe", (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes())
	server := httptest.NewServer(api)
	defer server.Close()
	request := func(method, path string, body []byte) (int, []byte, http.Header) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer fixture")
		req.Header.Set(headerUserID, h.user.String())
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, b, resp.Header
	}
	base := "/v1/vibe/sessions/" + h.session.ID.String()
	in := refundImport(t, a)
	body := mustJSON(t, in)
	code, b, _ := request("GET", "/v1/vibe/config", nil)
	if code != 200 || !bytes.Contains(b, []byte(`"source_schema_version":2`)) {
		t.Fatal("discovery", code, string(b))
	}
	code, b, _ = request("POST", base+"/reports", body)
	if code != 201 {
		t.Fatal("import", code, string(b))
	}
	var first vibe.SDKReport
	json.Unmarshal(b, &first)
	if first.SourceSchemaVersion != 2 || first.SourceKind != "eval_report" || first.ContentHash != vibe.Hash([]byte(in.SourceJSON)) || first.EvidenceMetadata.Origin != "imported" || len(first.Diagnostics) == 0 || len(first.SourceProvenance) == 0 {
		t.Fatal("receipt", first)
	}
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			code, b, _ := request("POST", base+"/reports", body)
			var retry vibe.SDKReport
			json.Unmarshal(b, &retry)
			if code != 201 || retry.ID != first.ID || !retry.CreatedAt.Equal(first.CreatedAt) {
				t.Errorf("retry: %d %s", code, b)
			}
		}()
	}
	group.Wait()
	if spy.count() != 1 {
		t.Fatal("retry duplicated analytics", spy.count())
	}
	detail := base + "/reports/" + first.ID.String()
	code, b, headers := request("GET", detail+"/original?hash="+first.ContentHash, nil)
	if code != 200 || string(b) != in.SourceJSON || headers.Get("X-Content-SHA256") != first.ContentHash {
		t.Fatal("original mismatch", code)
	}
	code, b, _ = request("GET", detail, nil)
	var reopened vibe.SDKReport
	json.Unmarshal(b, &reopened)
	if code != 200 || reopened.ID != first.ID || reopened.SourceSchemaVersion != 2 || reopened.Binding != first.Binding {
		t.Fatal("reopen identity", code, string(b))
	}
	code, b, _ = request("GET", detail+"/case?source_case_id=DEMO-repeat-refund-01&key=eligible", nil)
	var c vibe.CaseResult
	json.Unmarshal(b, &c)
	if code != 200 || c.Verdict != vibe.Pass || len(c.Checks) != 10 || c.Output != "" || c.Expected != "" {
		t.Fatal("case view", code, string(b))
	}
	for _, change := range []func(*vibe.SDKReportImport){func(x *vibe.SDKReportImport) { x.SourceJSON += " " }, func(x *vibe.SDKReportImport) { revision := "other"; x.Metadata.ApplicationRevision = &revision }} {
		copy := in
		change(&copy)
		code, b, _ = request("POST", base+"/reports", mustJSON(t, copy))
		if code != 409 {
			t.Fatal("identity conflict", code, string(b))
		}
	}
	h.reload()
	sub := vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "retest", Purpose: "regrade", ArtifactID: &a.ID, Models: h.svc.Config.DefaultModels(), RegradeSourceResultRef: &first.Binding.ResultRef, SourceEvidenceHash: first.ContentHash}
	code, b, _ = request("POST", base+"/run-quote", mustJSON(t, sub))
	if code != 400 || !bytes.Contains(b, []byte("unsupported_capability")) {
		t.Fatal("v2 regrade must reject before compiler/provider", code, string(b))
	}
	promotion := vibe.SDKPromotion{ClientID: uuid.New(), Revision: h.session.Revision, SourceCaseID: "DEMO-repeat-refund-01", SourceEvidenceHash: first.ContentHash, Expected: "unsupported", Reviewed: true}
	code, b, _ = request("POST", detail+"/promote", mustJSON(t, promotion))
	if code != 400 || !bytes.Contains(b, []byte("unsupported_capability")) {
		t.Fatal("promotion", code, string(b))
	}
	var ops, quotes, promotions int
	if err := h.db.QueryRow(h.ctx, `SELECT (SELECT count(*) FROM vibe_operations WHERE session_id=$1),(SELECT count(*) FROM vibe_cycle_quotes WHERE session_id=$1),(SELECT count(*) FROM vibe_sdk_promotions WHERE session_id=$1)`, h.session.ID).Scan(&ops, &quotes, &promotions); err != nil || ops != 0 || quotes != 0 || promotions != 0 {
		t.Fatal("unsupported operation admitted", ops, quotes, promotions, err)
	}
	var stored []byte
	var accounted, actual int
	err := h.db.QueryRow(h.ctx, `SELECT source_json,storage_bytes,octet_length(source_json)+octet_length(body::text)+octet_length(external_report_id)+octet_length(request_hash)+octet_length(content_sha256) FROM vibe_sdk_reports WHERE id=$1`, first.ID).Scan(&stored, &accounted, &actual)
	if err != nil || !bytes.Equal(stored, []byte(in.SourceJSON)) || accounted != actual {
		t.Fatal("SQL bytes/quota", err, accounted, actual)
	}
	if _, err = h.db.Exec(h.ctx, "UPDATE vibe_sdk_reports SET source_json=$2 WHERE id=$1", first.ID, []byte(`{}`)); err == nil {
		t.Fatal("immutable original changed")
	}
	for _, change := range []func(*vibe.SDKReportImport){func(x *vibe.SDKReportImport) { x.Metadata.ArtifactID = uuid.New() }, func(x *vibe.SDKReportImport) { x.Metadata.Cases[0].CaseKey = "wrong" }, func(x *vibe.SDKReportImport) { foreign := uuid.New(); x.Metadata.ChallengePackVersionID = &foreign }, func(x *vibe.SDKReportImport) { foreign := uuid.New(); x.Metadata.Cases[0].PublishedCaseID = &foreign }} {
		var copy vibe.SDKReportImport
		json.Unmarshal(body, &copy)
		var source map[string]any
		json.Unmarshal([]byte(copy.SourceJSON), &source)
		source["report_id"] = uuid.NewString()
		source["run_id"] = source["report_id"]
		copy.SourceJSON = string(mustJSON(t, source))
		change(&copy)
		code, b, _ = request("POST", base+"/reports", mustJSON(t, copy))
		if code == 201 {
			t.Fatal("foreign binding accepted", string(b))
		}
	}
	// Original owner still needs current active organization and workspace permission.
	if _, err = h.db.Exec(h.ctx, "UPDATE organization_memberships SET role='org_member' WHERE user_id=$1", h.user); err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Exec(h.ctx, "INSERT INTO workspace_memberships(workspace_id,organization_id,user_id,role,membership_status) SELECT id,organization_id,$2,'workspace_viewer','active' FROM workspaces WHERE id=$1", ws, h.user); err != nil {
		t.Fatal(err)
	}
	code, _, _ = request("POST", base+"/reports", body)
	if code == 201 {
		t.Fatal("read-only write")
	}
	code, b, _ = request("GET", detail+"/original", nil)
	if code != 200 || string(b) != in.SourceJSON {
		t.Fatal("authorized viewer original", code)
	}
	if _, err = h.db.Exec(h.ctx, "UPDATE organization_memberships SET membership_status='suspended' WHERE user_id=$1", h.user); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST"} {
		path := detail + "/original"
		if method == "POST" {
			path = base + "/reports"
		}
		code, _, _ = request(method, path, body)
		if code < 400 {
			t.Fatal("suspended access", method, code)
		}
	}
	if spy.count() != 1 {
		t.Fatal("rejections emitted success", spy.count())
	}
	t.Logf("controlled HTTP + owned PostgreSQL: receipt=%s source_sha256=%s exact_bytes=%d; original/retry/time/claims/readonly/suspended/bindings/quota/unsupported passed", first.ID, first.ContentHash, len(in.SourceJSON))
}

func TestVibeSDKRefundV2WrongWorkspaceAndQuota(t *testing.T) {
	h, _, a := sdkHarness(t)
	other, _, otherArtifact := sdkHarness(t)
	in := refundImport(t, a)
	if _, err := h.svc.Store.ImportSDKReport(h.ctx, other.actor, h.session.ID, mustJSON(t, in)); err == nil {
		t.Fatal("foreign workspace actor")
	}
	in.Metadata.ArtifactID = otherArtifact.ID
	if _, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in)); err == nil {
		t.Fatal("foreign workspace artifact")
	}
	in.Metadata.ArtifactID = a.ID
	// All prior retained source bytes and the v2 projection count toward admission.

	if _, err := h.db.Exec(h.ctx, "UPDATE vibe_import_originals SET content=$2 WHERE session_id=$1", h.session.ID, bytes.Repeat([]byte(" "), 4<<20)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		size := 4 << 20
		if i == 3 {
			size -= len(in.SourceJSON)
		}
		if _, err := h.db.Exec(h.ctx, `INSERT INTO vibe_import_originals(id,session_id,artifact_id,filename,media_type,content_sha256,content) VALUES($1,$2,$3,'fixture.json','application/json','fixture',$4)`, uuid.New(), h.session.ID, uuid.New(), bytes.Repeat([]byte(" "), size)); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := h.svc.Store.ImportSDKReport(h.ctx, h.actor, h.session.ID, mustJSON(t, in)); err == nil || !strings.Contains(err.Error(), "allowance") {
		t.Fatal("quota accepted", err)
	}
	var count int
	if err := h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_sdk_reports WHERE session_id=$1", h.session.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("quota mutated receipt", count, err)
	}
}
