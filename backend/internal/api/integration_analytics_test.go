package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/productanalytics"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
)

func TestTaggedClaimAnalytics(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		t.Run(host, func(t *testing.T) {
			ctx := productanalytics.WithObservedHost(context.Background(), host)
			ctx = productanalytics.WithSurface(ctx, productanalytics.SurfaceCLI)
			ctx = context.WithValue(ctx, tryoutProofContextKey{}, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
			org, ws := uuid.New(), uuid.New()
			repo := newFakeAgentTryoutRepository(org, ws)
			spy := &recordingPostHog{}
			manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithProductAnalytics(productanalytics.New(spy))
			caller := callerWithWorkspace(ws)
			trial, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{OwnerProofHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", TemplateSlug: "meeting-minutes", AnonymousFingerprint: "fixture", Input: []byte(`{"notes":"fixture"}`)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = manager.ClaimTryout(ctx, caller, ClaimAgentTryoutInput{ID: trial.ID, WorkspaceID: ws}); !errors.Is(err, repository.ErrAgentTryoutActive) {
				t.Fatal(err)
			}
			if spy.count() != 0 {
				t.Fatal("failed claim emitted success")
			}
			trial.Status = repository.AgentTryoutStatusCompleted
			repo.tryouts[trial.ID] = trial
			for i := 0; i < 2; i++ {
				claimed, err := manager.ClaimTryout(ctx, caller, ClaimAgentTryoutInput{ID: trial.ID, WorkspaceID: ws})
				if err != nil || claimed.ClaimedByUserID == nil {
					t.Fatal(err)
				}
			}
			if spy.count() != 1 {
				t.Fatalf("claim/retry emitted %d events", spy.count())
			}
			e := spy.last()
			if e.EventName != productanalytics.TryoutClaimed || e.DistinctID != caller.UserID.String() || e.Properties["observed_host"] != host || e.Properties["workspace_id"] != ws.String() || e.Properties["workflow_kind"] != "platform_demo" {
				t.Fatal(e)
			}
			if e.UUID != productanalytics.DeterministicUUID(productanalytics.TryoutClaimed, trial.ID, "").String() {
				t.Fatal(e)
			}
			t.Logf("synthetic manager claim: host=%s user=%s tryout=%s event_uuid=%s", host, e.DistinctID, trial.ID, e.UUID)
		})
	}
}

func TestTaggedSDKImportAnalytics(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		t.Run(host, func(t *testing.T) {
			h, ws, a := sdkHarness(t)
			spy := &recordingPostHog{}
			h.svc.Store.WithProductAnalytics(productanalytics.New(spy))
			ctx := productanalytics.WithObservedHost(h.ctx, host)
			ctx = productanalytics.WithSurface(ctx, productanalytics.SurfaceCLI)
			in := vibe.SDKReportImport{EnvelopeVersion: 1, SourceKind: "agent_result", SourceSchemaVersion: 1, ExternalReportID: "fixture-report", SourceJSON: `{"input":"fixture","output":"supplied evidence"}`, Metadata: vibe.SDKImportMetadata{ArtifactID: a.ID, CaptureScope: vibe.UnknownCaptureScope(), Cases: []vibe.SDKCaseBinding{{SourceCaseID: "local", CaseKey: "eligible"}}}}
			first, err := h.svc.Store.ImportSDKReport(ctx, h.actor, h.session.ID, mustJSON(t, in))
			if err != nil {
				t.Fatal(err)
			}
			stored, err := h.svc.Store.SDKReport(ctx, h.actor, h.session.ID, first.ID)
			if err != nil || stored.ID != first.ID {
				t.Fatal(err)
			}
			retry, err := h.svc.Store.ImportSDKReport(ctx, h.actor, h.session.ID, mustJSON(t, in))
			if err != nil || retry.ID != first.ID {
				t.Fatal(err)
			}
			in.SourceJSON = `{"input":"changed"}`
			if _, err = h.svc.Store.ImportSDKReport(ctx, h.actor, h.session.ID, mustJSON(t, in)); err == nil {
				t.Fatal("conflict accepted")
			}
			if spy.count() != 1 {
				t.Fatalf("import/retry/conflict emitted %d events", spy.count())
			}
			e := spy.last()
			if e.EventName != productanalytics.SDKReportImported || e.DistinctID != h.user.String() || e.Properties["sdk_report_id"] != first.ID.String() || e.Properties["workspace_id"] != ws.String() || e.Properties["observed_host"] != host || e.Properties["workflow_kind"] != "imported_local_evidence" {
				t.Fatal(e)
			}
			cancelled, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
			defer cancel()
			in.ExternalReportID = "cancelled"
			if _, err = h.svc.Store.ImportSDKReport(cancelled, h.actor, h.session.ID, mustJSON(t, in)); err == nil {
				t.Fatal("cancelled transaction accepted")
			}
			if spy.count() != 1 {
				t.Fatal("transaction failure emitted success")
			}
			// Force an actual deferred commit failure after the INSERT and event write.
			_, err = h.db.Exec(ctx, `CREATE FUNCTION pg_temp.reject_analytics_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture commit failure'; END $$;
     CREATE CONSTRAINT TRIGGER acq10_reject_receipt AFTER INSERT ON vibe_sdk_reports DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.external_report_id='reject-commit') EXECUTE FUNCTION pg_temp.reject_analytics_receipt()`)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_, _ = h.db.Exec(context.Background(), "DROP TRIGGER IF EXISTS acq10_reject_receipt ON vibe_sdk_reports")
			})
			in.ExternalReportID = "reject-commit"
			if _, err = h.svc.Store.ImportSDKReport(ctx, h.actor, h.session.ID, mustJSON(t, in)); err == nil {
				t.Fatal("deferred commit failure accepted")
			}
			var count int
			if err = h.db.QueryRow(ctx, "SELECT count(*) FROM vibe_sdk_reports WHERE session_id=$1 AND external_report_id='reject-commit'", h.session.ID).Scan(&count); err != nil || count != 0 || spy.count() != 1 {
				t.Fatalf("rollback count=%d events=%d err=%v", count, spy.count(), err)
			}
			if _, err = h.db.Exec(ctx, "DROP TRIGGER acq10_reject_receipt ON vibe_sdk_reports"); err != nil {
				t.Fatal(err)
			}
			h.svc.Store.WithProductAnalytics(nil)
			in.ExternalReportID = "analytics-disabled"
			if _, err = h.svc.Store.ImportSDKReport(ctx, h.actor, h.session.ID, mustJSON(t, in)); err != nil {
				t.Fatal(err)
			}
			t.Logf("synthetic PostgreSQL receipt: host=%s user=%s workspace=%s report=%s event_uuid=%s", host, e.DistinctID, ws, first.ID, e.UUID)
		})
	}
}

func TestTaggedClaimAnalyticsPostgreSQL(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		t.Run(host, func(t *testing.T) {
			h, ws, _ := sdkHarness(t)
			repo := repository.New(h.db)
			proof := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
			expiry := time.Now().Add(time.Hour)
			trial, err := repo.CreateAgentTryout(h.ctx, repository.CreateAgentTryoutParams{TemplateSlug: "meeting-minutes", Status: repository.AgentTryoutStatusCompleted, InputSnapshot: []byte(`{}`), TemplateSnapshot: []byte(`{}`), ToolPolicySnapshot: []byte(`{}`), EvaluationSpecSnapshot: []byte(`{}`), SelectedModelPolicy: []byte(`{}`), Summary: []byte(`{}`), RedactionStatus: repository.AgentTryoutRedactionPending, CostLimitUSD: 0.25, MaxDurationSeconds: 120, AnonymousOwnerProofHash: &proof, ExpiresAt: &expiry})
			if err != nil {
				t.Fatal(err)
			}
			spy := &recordingPostHog{}
			manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo).WithProductAnalytics(productanalytics.New(spy))
			caller := callerWithWorkspace(ws)
			caller.UserID = h.user
			ctx := productanalytics.WithObservedHost(context.WithValue(h.ctx, tryoutProofContextKey{}, proof), host)
			for i := 0; i < 2; i++ {
				if _, err = manager.ClaimTryout(ctx, caller, ClaimAgentTryoutInput{ID: trial.ID, WorkspaceID: ws}); err != nil {
					t.Fatal(err)
				}
			}
			stored, err := repo.GetAgentTryoutByID(ctx, trial.ID)
			if err != nil || stored.ClaimedByUserID == nil || *stored.ClaimedByUserID != h.user || spy.count() != 1 {
				t.Fatal(fmt.Sprintf("durable claim events=%d err=%v", spy.count(), err))
			}
			t.Logf("synthetic PostgreSQL claim (terminal row seeded, no execution): host=%s user=%s tryout=%s event_uuid=%s", host, h.user, trial.ID, spy.last().UUID)
		})
	}
}
