package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func canonicalHarness(t *testing.T) (*vibeAPIHarness, uuid.UUID, vibe.Artifact, uuid.UUID, uuid.UUID) {
	t.Helper()
	h, ws, _ := sdkHarness(t)
	if err := h.svc.Import(h.ctx, h.actor, h.session.ID, h.session.Revision, mustJSON(t, vibeReliabilityBundle(t))); err != nil {
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
	if err = h.db.QueryRow(h.ctx, `SELECT composition,(SELECT organization_id FROM workspaces WHERE id=challenge_pack_drafts.workspace_id) FROM challenge_pack_drafts WHERE id=$1`, draft).Scan(&composition, &org); err != nil {
		t.Fatal(err)
	}
	var comp challengepack.Composition
	if err = json.Unmarshal(composition, &comp); err != nil {
		t.Fatal(err)
	}
	bundle, err := challengepack.ComposeBundle(comp, challengepack.ResolvedPieces{})
	if err != nil {
		t.Fatal(err)
	}
	repo := repository.New(h.db)
	pub, err := repo.PublishChallengePackBundle(h.ctx, repository.PublishChallengePackBundleParams{OrganizationID: org, WorkspaceID: ws, Bundle: bundle, Source: &repository.PackPublicationSource{DraftID: draft, Composition: composition}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Exec(h.ctx, `DELETE FROM challenge_pack_drafts WHERE id=$1`, draft); err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Exec(h.ctx, `INSERT INTO workspace_secrets(workspace_id,key,encrypted_value,created_by) VALUES($1,'APP_TOKEN','fixture',$2)`, ws, h.user); err != nil {
		t.Fatal(err)
	}
	binding, err := repo.CreateConnector(h.ctx, repository.CreateConnectorParams{WorkspaceID: ws, ActorUserID: h.user, Name: "Canonical fixture", Config: connectors.Config{Adapter: "http_json", EndpointURL: "https://example.com/invoke", SecretKey: "APP_TOKEN", Auth: "bearer", InputFields: map[string]string{"question": "/prompt"}, ResponsePointer: "/answer", TimeoutSeconds: 60}})
	if err != nil {
		t.Fatal(err)
	}
	var input uuid.UUID
	if err = h.db.QueryRow(h.ctx, `SELECT id FROM challenge_input_sets WHERE challenge_pack_version_id=$1 ORDER BY input_key LIMIT 1`, pub.ChallengePackVersionID).Scan(&input); err != nil {
		t.Fatal(err)
	}
	run, agent := uuid.New(), uuid.New()
	if _, err = h.db.Exec(h.ctx, `INSERT INTO runs(id,organization_id,workspace_id,challenge_pack_version_id,challenge_input_set_id,name,status) VALUES($1,$2,$3,$4,$5,'Canonical fixture','completed')`, run, org, ws, pub.ChallengePackVersionID, input); err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Exec(h.ctx, `INSERT INTO run_agents(id,organization_id,workspace_id,run_id,agent_deployment_id,agent_deployment_snapshot_id,lane_index,label,status) VALUES($1,$2,$3,$4,$5,$6,0,'Application','completed')`, agent, org, ws, run, binding.DeploymentID, binding.SnapshotID); err != nil {
		t.Fatal(err)
	}
	return h, ws, a, run, agent
}
func TestVibeCanonicalMountedSourceAndExport(t *testing.T) {
	h, _, a, run, _ := canonicalHarness(t)
	api := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	base := "/sessions/" + h.session.ID.String()
	w := sdkRequest(api, h.user, "GET", base+"/canonical-results/run/"+run.String()+"?artifact_id="+a.ID.String(), nil)
	if w.Code != 200 {
		t.Fatalf("source=%d %s", w.Code, w.Body)
	}
	var source map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &source); err != nil {
		t.Fatal(err)
	}
	if source["binding"].(map[string]any)["artifact_id"] != a.ID.String() {
		t.Fatal("wrong binding", source)
	}
	w = sdkRequest(api, h.user, "POST", base+"/export", mustJSON(t, map[string]any{"scope": "selected", "artifact_id": a.ID, "result_ref": map[string]any{"kind": "run", "id": run}}))
	if w.Code != 200 {
		t.Fatalf("export=%d %s", w.Code, w.Body)
	}
	var export map[string]any
	json.Unmarshal(w.Body.Bytes(), &export)
	if len(export["canonical_results"].([]any)) != 1 || len(export["runs"].([]any)) != 0 {
		t.Fatal("canonical evidence cast to operation", export)
	}
}

func TestVibeCanonicalProjectACLAndCompleteEvalBinding(t *testing.T) {
	h, ws, a, run, _ := canonicalHarness(t)
	handler := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	base := "/sessions/" + h.session.ID.String() + "/canonical-results/"
	get := func(user uuid.UUID, kind string, id uuid.UUID, artifact uuid.UUID) int {
		return sdkRequest(handler, user, "GET", base+kind+"/"+id.String()+"?artifact_id="+artifact.String(), nil).Code
	}
	if code := get(uuid.New(), "run", run, a.ID); code < 400 {
		t.Fatal("foreign actor read", code)
	}
	if code := get(h.user, "run", run, uuid.New()); code != 404 {
		t.Fatal("foreign artifact", code)
	}
	eval := uuid.New()
	if _, e := h.db.Exec(h.ctx, `INSERT INTO eval_sessions(id,repetitions,schema_version) VALUES($1,1,1)`, eval); e != nil {
		t.Fatal(e)
	}
	if code := get(h.user, "eval_session", eval, a.ID); code != 404 {
		t.Fatal("unowned empty eval", code)
	}
	if _, e := h.db.Exec(h.ctx, `UPDATE runs SET eval_session_id=$2 WHERE id=$1`, run, eval); e != nil {
		t.Fatal(e)
	}
	if _, e := h.db.Exec(h.ctx, `UPDATE eval_sessions SET workspace_id=$2 WHERE id=$1`, eval, ws); e != nil {
		t.Fatal(e)
	}
	if code := get(h.user, "eval_session", eval, a.ID); code != 200 {
		t.Fatal("mapped eval", code)
	}
	if _, e := h.db.Exec(h.ctx, `UPDATE runs SET challenge_input_set_id=NULL WHERE id=$1`, run); e != nil {
		t.Fatal(e)
	}
	if code := get(h.user, "eval_session", eval, a.ID); code != 404 {
		t.Fatal("unmapped child amplified", code)
	}
	if _, e := h.db.Exec(h.ctx, `DELETE FROM organization_memberships WHERE user_id=$1 AND organization_id=(SELECT organization_id FROM workspaces WHERE id=$2)`, h.user, ws); e != nil {
		t.Fatal(e)
	}
	if code := get(h.user, "run", run, a.ID); code < 400 {
		t.Fatal("revoked membership", code)
	}
}
func TestVibeCanonicalFrozenExportChangesAndUnknownIdentity(t *testing.T) {
	h, _, a, run, _ := canonicalHarness(t)
	ref := vibe.ResultRef{Kind: vibe.ResultRun, ID: run}
	original, e := h.svc.Store.CanonicalResult(h.ctx, h.actor, h.session.ID, ref, &a.ID)
	if e != nil {
		t.Fatal(e)
	}
	f := original.EvidenceMetadata.Fingerprints
	if f.Model != nil || f.Build != nil || f.Config != nil || f.Test == nil || f.Judge == nil || !strings.HasPrefix(*f.Test, "canonical:tests:v1:sha256:") || !strings.HasPrefix(*f.Judge, "canonical:judge:v1:sha256:") {
		t.Fatalf("external fingerprint assurance %+v", f)
	}
	prepared, e := h.svc.Store.PrepareExport(h.ctx, h.actor, h.session.ID, vibe.ExportRequest{Scope: "selected", ArtifactID: a.ID, ResultRef: &ref})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = h.db.Exec(h.ctx, `UPDATE runs SET status='failed' WHERE id=$1`, run); e != nil {
		t.Fatal(e)
	}
	if _, e = h.db.Exec(h.ctx, `UPDATE runtime_profiles SET run_timeout_seconds=run_timeout_seconds+1 WHERE id IN (SELECT s.source_runtime_profile_id FROM agent_deployment_snapshots s JOIN run_agents a ON a.agent_deployment_snapshot_id=s.id WHERE a.run_id=$1)`, run); e != nil {
		t.Fatal(e)
	}
	current, e := h.svc.Store.CanonicalResult(h.ctx, h.actor, h.session.ID, ref, &a.ID)
	if e != nil {
		t.Fatal(e)
	}
	if current.EvidenceHash == original.EvidenceHash || prepared.CanonicalResults[0].Status != "completed" || prepared.CanonicalResults[0].EvidenceHash != original.EvidenceHash {
		t.Fatal("frozen source metadata mutated")
	}
	if current.EvidenceMetadata.Fingerprints.Config != nil || *current.EvidenceMetadata.Fingerprints.Test != *f.Test || *current.EvidenceMetadata.Fingerprints.Judge != *f.Judge || current.Participants[0].SnapshotID != original.Participants[0].SnapshotID {
		t.Fatal("mutable runtime defaults were represented as historical admitted settings")
	}
	handler := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	w := sdkRequest(handler, h.user, "GET", "/sessions/"+h.session.ID.String()+"/canonical-results/run/"+run.String()+"?artifact_id="+a.ID.String()+"&hash="+original.EvidenceHash, nil)
	if w.Code < 400 {
		t.Fatal("stale canonical hash accepted")
	}
	if len(original.Omissions) < 2 {
		t.Fatal("raw body omissions absent")
	}
}

func TestVibeCanonicalRejectsPartiallyProjectedParticipants(t *testing.T) {
	h, _, a, run, _ := canonicalHarness(t)
	// A legacy harness participant has no deployment snapshot. Omitting it from a
	// mixed run would falsely make the remaining application the whole result.
	_, err := h.db.Exec(h.ctx, `INSERT INTO run_agents(id,organization_id,workspace_id,run_id,source_type,lane_index,label,status) SELECT $1,organization_id,workspace_id,id,'agent_harness',1,'Other participant','completed' FROM runs WHERE id=$2`, uuid.New(), run)
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.svc.Store.CanonicalResult(h.ctx, h.actor, h.session.ID, vibe.ResultRef{Kind: vibe.ResultRun, ID: run}, &a.ID)
	if err == nil {
		t.Fatal("partially projected participants accepted as a complete canonical result")
	}
}

func TestVibeCanonicalProjectsPersistedEventIdentityWithoutRawBody(t *testing.T) {
	h, _, a, run, agent := canonicalHarness(t)
	_, err := repository.New(h.db).RecordHostedRunEvent(h.ctx, repository.RecordHostedRunEventParams{Event: runevents.Envelope{EventID: "stable-remote-id", SchemaVersion: runevents.SchemaVersionV1, RunID: run, RunAgentID: agent, EventType: runevents.EventTypeSystemOutputFinalized, Source: runevents.SourceHostedExternal, OccurredAt: time.Now().UTC(), Payload: json.RawMessage(`{"output":"private normalized output"}`)}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := h.svc.Store.CanonicalResult(h.ctx, h.actor, h.session.ID, vibe.ResultRef{Kind: vibe.ResultRun, ID: run}, &a.ID)
	if err != nil {
		t.Fatal(err)
	}
	events := result.Participants[0].Events
	if len(events) != 1 || events[0].SourceID == nil || *events[0].SourceID != "stable-remote-id" || events[0].PayloadHash == "" {
		t.Fatalf("missing persisted event identities: %+v", events)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "private normalized output") {
		t.Fatal("metadata projection leaked raw body")
	}
}

func TestVibeCanonicalMountedReadinessRejectsPartialConnectorSchema(t *testing.T) {
	h, _, _, _, _ := canonicalHarness(t)
	schema := "connector_ready_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, e := h.db.Exec(h.ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer h.db.Exec(h.ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, e := h.db.Exec(h.ctx, "CREATE TABLE "+schema+".connector_snapshots AS SELECT * FROM connector_snapshots WITH NO DATA"); e != nil {
		t.Fatal(e)
	}
	if _, e := h.db.Exec(h.ctx, "ALTER TABLE "+schema+".connector_snapshots DROP COLUMN secret_revision"); e != nil {
		t.Fatal(e)
	}
	cfg := h.db.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	partial, e := pgxpool.NewWithConfig(h.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer partial.Close()
	for _, db := range []*pgxpool.Pool{h.db, partial} {
		check := WithConnectorReadiness(func(context.Context) map[string]string { return map[string]string{"vibe_schema": "ok", "redis": "ok"} }, repository.New(db))
		router := buildRouter(routerOptions{logger: slog.Default(), dbReadiness: db, temporalReadiness: fakeTemporalHealthChecker{}, dependencyReadiness: check})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/healthz/ready", nil))
		want := 200
		if db == partial {
			want = 503
		}
		if w.Code != want {
			t.Fatalf("readiness=%d want=%d %s", w.Code, want, w.Body)
		}
	}
}

func TestVibeObservedCanonicalAuthorizedFrozenExport(t *testing.T) {
	h, _, a, run, agent := canonicalHarness(t)
	_, err := repository.New(h.db).RecordHostedRunEvent(h.ctx, repository.RecordHostedRunEventParams{Event: runevents.Envelope{EventID: "observed-remote", SchemaVersion: runevents.SchemaVersionV1, RunID: run, RunAgentID: agent, EventType: runevents.EventTypeSystemOutputFinalized, Source: runevents.SourceHostedExternal, OccurredAt: time.Now().UTC(), Payload: json.RawMessage(`{"output":"private mapped answer","tool_calls":[{"name":"not a typed execution"}]}`)}})
	if err != nil {
		t.Fatal(err)
	}
	api := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	base := "/sessions/" + h.session.ID.String()
	url := base + "/results/run/" + run.String() + "/observed-evidence?artifact_id=" + a.ID.String()
	w := sdkRequest(api, h.user, "GET", url, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var got vibe.ObservedEvidence
	if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var output *vibe.ObservedEntry
	for _, c := range got.Cases {
		for _, e := range c.Entries {
			if e.Kind == "tool_execution" {
				t.Fatal("arbitrary mapped output upgraded to tool execution")
			}
			if e.Kind == "output" {
				v := e
				output = &v
			}
		}
	}
	if output == nil || output.EventID == "" || output.SourceEventID == nil || *output.SourceEventID != "observed-remote" || output.PayloadHash == "" || output.ValueJSON != `"private mapped answer"` {
		t.Fatalf("source identity/body lost: %+v", output)
	}
	if w = sdkRequest(api, uuid.New(), "GET", url, nil); w.Code < 400 {
		t.Fatal("foreign actor evidence read")
	}
	if w = sdkRequest(api, h.user, "GET", url+"&hash=changed", nil); w.Code < 400 {
		t.Fatal("stale evidence hash accepted")
	}
	for _, include := range []bool{false, true} {
		w = sdkRequest(api, h.user, "POST", base+"/export", mustJSON(t, map[string]any{"scope": "selected", "artifact_id": a.ID, "result_ref": got.Binding.ResultRef, "include_observed": include}))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var p vibe.ExportPreparation
		if err = json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if include {
			if len(p.ObservedEvidence) != 1 || p.ObservedEvidence[0].EvidenceHash != got.EvidenceHash {
				t.Fatal("frozen observed export mismatch")
			}
		} else if len(p.ObservedEvidence) != 0 || strings.Contains(w.Body.String(), "private mapped answer") {
			t.Fatal("raw observed body included without selection")
		}
	}
}

func TestVibeCanonicalControlledAssuranceRequiresCompatibleDispatchGuard(t *testing.T) {
	h, ws, a, run, agent := canonicalHarness(t)
	useDeterministicCanonicalEvaluator(t, h, run)
	repo := repository.New(h.db)
	var snapshot, pack uuid.UUID
	if err := h.db.QueryRow(h.ctx, `SELECT a.agent_deployment_snapshot_id,r.challenge_pack_version_id FROM run_agents a JOIN runs r ON r.id=a.run_id WHERE a.id=$1`, agent).Scan(&snapshot, &pack); err != nil {
		t.Fatal(err)
	}
	receipt, err := repo.PreviewControlledSettings(h.ctx, ws, snapshot, pack, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Exec(h.ctx, `INSERT INTO run_agent_controlled_settings(run_agent_id,snapshot_id,settings_hash,settings) VALUES($1,$2,$3,$4)`, agent, snapshot, receipt.Hash, receipt.Settings); err != nil {
		t.Fatal(err)
	}
	for _, version := range []int{0, 99, 1} {
		if version > 0 {
			if _, err = h.db.Exec(h.ctx, `UPDATE run_agent_controlled_settings SET dispatch_validated_at=now(),dispatch_guard_version=$2 WHERE run_agent_id=$1`, agent, version); err != nil {
				t.Fatal(err)
			}
		}
		v, err := h.svc.Store.CanonicalResult(h.ctx, h.actor, h.session.ID, vibe.ResultRef{Kind: vibe.ResultRun, ID: run}, &a.ID)
		if err != nil {
			t.Fatal(err)
		}
		if (v.Participants[0].Fingerprints.Config != nil) != (version == 1) {
			t.Fatalf("version %d fabricated controlled configuration %+v", version, v.Participants[0])
		}
	}
}

func useDeterministicCanonicalEvaluator(t *testing.T, h *vibeAPIHarness, run uuid.UUID) {
	t.Helper()
	_, err := h.db.Exec(h.ctx, `UPDATE challenge_pack_versions SET manifest=jsonb_set(manifest,'{evaluation_spec}','{"name":"controlled","version_number":1,"judge_mode":"deterministic","validators":[{"key":"answer","type":"regex_match","target":"final_output","expected_from":"literal:.+"}],"scorecard":{"dimensions":[{"key":"output_present","source":"validators","validators":["answer"]}]}}'::jsonb) WHERE id=(SELECT challenge_pack_version_id FROM runs WHERE id=$1)`, run)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVibeCanonicalMountedReadinessRejectsPartialExperimentSchema(t *testing.T) {
	h := newVibeAPIHarness(t)
	for _, missing := range []struct{ table, column string }{{"eval_sessions", "workspace_id"}, {"run_agent_controlled_settings", "dispatch_guard_version"}, {"eval_set_admissions", "request_hash"}, {"eval_session_admissions", "request_hash"}} {
		t.Run(missing.table+"."+missing.column, func(t *testing.T) {
			schema := "experiment_ready_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			if _, e := h.db.Exec(h.ctx, "CREATE SCHEMA "+schema); e != nil {
				t.Fatal(e)
			}
			defer h.db.Exec(h.ctx, "DROP SCHEMA "+schema+" CASCADE")
			if _, e := h.db.Exec(h.ctx, "CREATE TABLE "+schema+"."+missing.table+" AS SELECT * FROM "+missing.table+" WITH NO DATA"); e != nil {
				t.Fatal(e)
			}
			if _, e := h.db.Exec(h.ctx, "ALTER TABLE "+schema+"."+missing.table+" DROP COLUMN "+missing.column); e != nil {
				t.Fatal(e)
			}
			cfg := h.db.Config().Copy()
			cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
			partial, e := pgxpool.NewWithConfig(h.ctx, cfg)
			if e != nil {
				t.Fatal(e)
			}
			defer partial.Close()
			check := WithConnectorReadiness(func(context.Context) map[string]string { return map[string]string{"vibe_schema": "ok", "redis": "ok"} }, repository.New(partial))
			router := buildRouter(routerOptions{logger: slog.Default(), dbReadiness: partial, temporalReadiness: fakeTemporalHealthChecker{}, dependencyReadiness: check})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", "/healthz/ready", nil))
			if w.Code != 503 || !strings.Contains(w.Body.String(), `"experiment_schema":"unavailable"`) {
				t.Fatalf("missing required field ready: %d %s", w.Code, w.Body)
			}
		})
	}
}

func TestControlledPreviewRejectsUnfixedEvaluatorInputs(t *testing.T) {
	h, ws, _, run, agent := canonicalHarness(t)
	repo := repository.New(h.db)
	var snapshot, pack uuid.UUID
	if err := h.db.QueryRow(h.ctx, `SELECT a.agent_deployment_snapshot_id,r.challenge_pack_version_id FROM run_agents a JOIN runs r ON r.id=a.run_id WHERE a.id=$1`, agent).Scan(&snapshot, &pack); err != nil {
		t.Fatal(err)
	}
	// This fixture's original fuzzy evaluator uses a process-default account.
	if _, err := repo.PreviewControlledSettings(h.ctx, ws, snapshot, pack, nil); !errors.Is(err, repository.ErrControlledModeUnsupported) {
		t.Fatal("external judge route represented as fixed", err)
	}
	useDeterministicCanonicalEvaluator(t, h, run)
	if _, err := repo.PreviewControlledSettings(h.ctx, ws, snapshot, pack, nil); err != nil {
		t.Fatal("deterministic evaluator unavailable", err)
	}
	if _, err := h.db.Exec(h.ctx, `UPDATE challenge_pack_versions SET manifest=jsonb_set(manifest,'{evaluation_spec,scorecard,dimensions}','[{"key":"human","source":"human_preference"}]'::jsonb) WHERE id=$1`, pack); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.PreviewControlledSettings(h.ctx, ws, snapshot, pack, nil); !errors.Is(err, repository.ErrControlledModeUnsupported) {
		t.Fatal("live human preference represented as fixed", err)
	}
}
