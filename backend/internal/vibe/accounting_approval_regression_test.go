package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestIntegrationReconcileAttemptOverrunFreezesBackingAccounts(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		name := "running"
		if stopped {
			name = "cancelled"
		}
		t.Run(name, func(t *testing.T) {
			s := integrationStore(t)
			ctx := context.Background()
			v := anonSession(t, s)
			cfg := testConfig()
			models := DefaultModels()
			// This database-only fixture must not disable a shared real profile.
			models.Target = "test/reconciliation-" + uuid.NewString()
			sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "check", Models: models}
			plan := Plan{Submission: sub, Anonymous: true, Cases: []string{"lost-output"}, ChecksPerCase: 1, Calls: 2, MaxCost: 100000000}
			o, err := s.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = s.Start(ctx, o.ID); err != nil {
				t.Fatal(err)
			}
			a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target:lost-output", Role: Target, Model: models.Target, Policy: json.RawMessage(`{}`), InputBound: 1000, MaxOutput: 100, MaxCost: 10000000}
			if err = s.BeginAttempt(ctx, a); err != nil {
				t.Fatal(err)
			}
			if err = s.Generation(ctx, a.ID, "gen-"+a.ID.String()); err != nil {
				t.Fatal(err)
			}
			if err = s.EndAttempt(ctx, a, "", json.RawMessage(`{}`), nil, &Fault{"worker_interrupted", "Output and cost were not recovered."}); err != nil {
				t.Fatal(err)
			}
			if stopped {
				if err = s.Stop(ctx, v.Actor, o.ID); err != nil {
					t.Fatal(err)
				}
				current, err := s.Operation(ctx, o.ID)
				if err != nil || current.State != Cancelled || current.Billing != Reconciling || current.ActualCost != nil {
					t.Fatalf("stop lost uncertain accounting: %+v, err=%v", current, err)
				}
			}
			var total, held, disabled int
			if err = s.DB.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE a.held=r.amount AND r.amount=$2 AND r.settled_amount IS NULL),count(*) FILTER(WHERE a.disabled)
 FROM vibe_reservations r JOIN vibe_accounts a ON a.id=r.account_id WHERE r.operation_id=$1`, o.ID, o.MaxCost).Scan(&total, &held, &disabled); err != nil {
				t.Fatal(err)
			}
			if total != 3 || held != total || disabled != 0 {
				t.Fatalf("expected three enabled, fully held trial/subsidy accounts: total=%d held=%d disabled=%d", total, held, disabled)
			}
			before, err := s.GetCase(ctx, v.Actor, o.ID, "lost-output")
			if err != nil {
				t.Fatal(err)
			}
			const cost int64 = 11000000 // Exceeds the attempt cap, not the operation reservation.
			if err = s.ReconcileCost(ctx, a.ID, cost, json.RawMessage(`{"cost":0.011}`)); err != nil {
				t.Fatal(err)
			}
			if err = s.DB.QueryRow(ctx, `SELECT count(*) FILTER(WHERE a.disabled) FROM vibe_reservations r JOIN vibe_accounts a ON a.id=r.account_id WHERE r.operation_id=$1`, o.ID).Scan(&disabled); err != nil {
				t.Fatal(err)
			}
			if disabled != total {
				t.Fatalf("attempt overrun below operation reservation left backing accounts enabled: disabled=%d total=%d", disabled, total)
			}
			var targetDisabled, evaluatorDisabled bool
			if err = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_disabled_profiles WHERE model=$1),EXISTS(SELECT 1 FROM vibe_disabled_profiles WHERE model=$2)`, models.Target, models.Evaluator).Scan(&targetDisabled, &evaluatorDisabled); err != nil {
				t.Fatal(err)
			}
			if !targetDisabled || evaluatorDisabled {
				t.Fatalf("profile isolation lost: target disabled=%v evaluator disabled=%v", targetDisabled, evaluatorDisabled)
			}
			if !stopped {
				other := a
				other.ID, other.Step, other.Role, other.Model = uuid.New(), "judge:lost-output", Evaluator, models.Evaluator
				requireRegressionFault(t, s.BeginAttempt(ctx, other), "accounting_unavailable")
				current, err := s.Operation(ctx, o.ID)
				if err != nil || current.State != Running || current.ModelCalls != 1 {
					t.Fatalf("reconciliation changed execution or admitted another model: %+v err=%v", current, err)
				}
				if err = s.Stop(ctx, v.Actor, o.ID); err != nil {
					t.Fatal(err)
				}
			}
			// Repeated evidence is idempotent; conflicting evidence cannot rewrite cost.
			if err = s.ReconcileCost(ctx, a.ID, cost, json.RawMessage(`{"cost":0.011}`)); err != nil {
				t.Fatal(err)
			}
			requireRegressionFault(t, s.ReconcileCost(ctx, a.ID, cost+1, json.RawMessage(`{}`)), "reconciliation_conflict")
			requireRegressionFault(t, s.BeginAttempt(ctx, a), "operation_stopped")
			current, err := s.Operation(ctx, o.ID)
			if err != nil || current.State != Cancelled || current.Billing != Settled || current.ActualCost == nil || *current.ActualCost != cost || current.ModelCalls != 1 {
				t.Fatalf("reconciliation restarted work or changed settlement: %+v err=%v", current, err)
			}
			var settled int
			if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_reservations r JOIN vibe_accounts a ON a.id=r.account_id JOIN vibe_grants g ON g.account_id=a.id AND g.source='initial:'||a.id
 WHERE r.operation_id=$1 AND a.disabled AND a.held=0 AND r.amount=$2 AND r.settled_amount=$3 AND a.balance=g.amount-$3`, o.ID, o.MaxCost, cost).Scan(&settled); err != nil || settled != total {
				t.Fatalf("backing accounts not settled exactly once while frozen: settled=%d total=%d err=%v", settled, total, err)
			}
			after, err := s.GetCase(ctx, v.Actor, o.ID, "lost-output")
			if err != nil || !reflect.DeepEqual(before, after) || after.Verdict != Unknown || after.Output != "" {
				t.Fatalf("accounting invented or changed lost evidence: before=%+v after=%+v err=%v", before, after, err)
			}
			latest, err := s.GetSession(ctx, v.Actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(latest.Operations) != 1 || latest.Operations[0].Scorecard == nil || latest.Operations[0].Scorecard.Unknown != 1 {
				t.Fatal("reconciliation lost the UNKNOWN scorecard")
			}
			// A model switch cannot spend the frozen funding on a new operation.
			next := Submission{ClientID: uuid.New(), Revision: latest.Revision, Kind: "message", Models: DefaultModels(), Content: "Use another model"}
			_, err = s.Submit(ctx, v.Actor, v.ID, next, Plan{Submission: next, Anonymous: true, Calls: 1, MaxCost: 1000000}, cfg)
			requireRegressionFault(t, err, "insufficient_credits")
			var operations, attempts int
			if err = s.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM vibe_operations WHERE session_id=$1),(SELECT count(*) FROM vibe_attempts WHERE operation_id=$2)`, v.ID, o.ID).Scan(&operations, &attempts); err != nil || operations != 1 || attempts != 1 {
				t.Fatalf("blocked admission or replay left work behind: operations=%d attempts=%d err=%v", operations, attempts, err)
			}
		})
	}
}

func TestIntegrationAuthoringApprovalLocksAcceptedDocumentEdits(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	v := approvalRegressionWorkspace(t, s)
	cfg := testConfig()
	requirementID, sourceID, artifactID := uuid.New(), uuid.New(), uuid.New()
	if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error {
		v.Document.Messages = append(v.Document.Messages, Message{ID: sourceID, Role: "user", Content: "Refund within 30 days", CreatedAt: timestamp()})
		v.Document.Requirements = append(v.Document.Requirements, Requirement{ID: requirementID, Statement: "Refund within 30 days", Status: "proposed", SourceMessageID: sourceID, ProposedBy: "assistant"})
		v.Document.Artifacts = append(v.Document.Artifacts, Artifact{ID: artifactID, Title: "Refund assistant", AgentPrompt: "Refund within 30 days", Blueprint: json.RawMessage(`{}`), SourceMessageID: sourceID, Accepted: true, CreatedAt: timestamp()})
		v.Document.ActiveArtifactID = &artifactID
		return DecideRequirement(v, requirementID, "accepted", nil)
	}); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "build", Content: "Improve the wording without changing the refund policy", Models: DefaultModels()}
	plan := Plan{Submission: sub, Document: v.Document, Artifact: &v.Document.Artifacts[0], Calls: 2, MaxCost: 800000000}
	o, err := s.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
	if err != nil || o.State != AwaitingApproval || o.Billing != Unreserved {
		t.Fatalf("expected an unfunded authoring quote: %+v err=%v", o, err)
	}
	before, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	replacement := "Refund within 14 days"
	mutateRequirement := func(v *Session) error {
		return DecideRequirement(v, requirementID, "superseded", &replacement)
	}
	// Ownership and revision checks still precede the active-writer guard.
	requireRegressionFault(t, s.Edit(ctx, "user:"+uuid.NewString(), v.ID, before.Revision, mutateRequirement), "not_found")
	requireRegressionFault(t, s.Edit(ctx, v.Actor, v.ID, sub.Revision, mutateRequirement), "revision_conflict")
	for _, edit := range []struct {
		name string
		fn   func(*Session) error
	}{
		{"accepted requirement", mutateRequirement},
		{"accepted artifact", func(v *Session) error {
			next := v.Document.Artifacts[0]
			next.ID, next.ParentID, next.AgentPrompt = uuid.New(), &artifactID, replacement
			v.Document.Artifacts = append(v.Document.Artifacts, next)
			v.Document.ActiveArtifactID = &next.ID
			return nil
		}},
	} {
		called := false
		err = s.Edit(ctx, v.Actor, v.ID, before.Revision, func(v *Session) error {
			called = true
			return edit.fn(v)
		})
		var f *Fault
		if !errors.As(err, &f) || f.Code != "operation_running" || called {
			t.Fatalf("pending authoring quote allowed %s edit: callback_called=%v err=%v", edit.name, called, err)
		}
	}
	after, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil || after.Revision != before.Revision || after.EventCursor != before.EventCursor || !reflect.DeepEqual(after.Document, before.Document) {
		t.Fatalf("rejected quote edit changed document, revision, or events: err=%v", err)
	}
	var reservations int
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_reservations WHERE operation_id=$1", o.ID).Scan(&reservations); err != nil || reservations != 0 {
		t.Fatalf("unapproved quote reserved funding: count=%d err=%v", reservations, err)
	}
	if err = s.Approve(ctx, v.Actor, o.ID, cfg); err != nil {
		t.Fatal(err)
	}
	started, current, err := s.Start(ctx, o.ID)
	if err != nil || started.State != Running || started.Billing != BillingReserved || started.ModelCalls != 0 {
		t.Fatalf("approval did not preserve the normal dispatch path: %+v err=%v", started, err)
	}
	var quoted Plan
	if err = json.Unmarshal(started.Input, &quoted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(quoted.Document, plan.Document) || !reflect.DeepEqual(quoted.Artifact, plan.Artifact) || !reflect.DeepEqual(current.Document, before.Document) {
		t.Fatal("approved authoring would execute against stale accepted context")
	}
	if err = s.Stop(ctx, v.Actor, o.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Edit(ctx, v.Actor, v.ID, current.Revision, mutateRequirement); err != nil {
		t.Fatalf("stopping the quote did not unlock accepted requirement replacement: %v", err)
	}
	final, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil || final.Revision != current.Revision+1 || len(final.Document.Requirements) != 2 {
		t.Fatalf("unlocked edit was not persisted: %+v err=%v", final, err)
	}
	old, next := final.Document.Requirements[0], final.Document.Requirements[1]
	if old.Status != "superseded" || next.Status != "accepted" || next.Statement != replacement || next.AcceptedBy != v.Actor || next.SupersedesID == nil || *next.SupersedesID != requirementID {
		t.Fatalf("unlocked replacement lost acceptance/provenance: old=%+v next=%+v", old, next)
	}
	stopped, err := s.Operation(ctx, o.ID)
	if err != nil || stopped.State != Cancelled || stopped.Billing != Released || stopped.ModelCalls != 0 || !reflect.DeepEqual(stopped.Input, started.Input) {
		t.Fatalf("post-stop edit rewrote quoted work or billed an undispatched call: %+v err=%v", stopped, err)
	}
}

func approvalRegressionWorkspace(t *testing.T, s *Store) Session {
	t.Helper()
	ctx := context.Background()
	org, ws, user := uuid.New(), uuid.New(), uuid.New()
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO organizations(id,name,slug) VALUES($1,'Approval regression',$2)", []any{org, org.String()}},
		{"INSERT INTO workspaces(id,organization_id,name,slug) VALUES($1,$2,'Approval regression',$3)", []any{ws, org, ws.String()}},
		{"INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", []any{user, user.String(), user.String() + "@example.invalid"}},
		{"INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) VALUES($1,$2,'org_admin','active')", []any{org, user}},
	} {
		if _, err := s.DB.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	v, err := s.CreateSession(ctx, "user:"+user.String(), &ws, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, s, v.ID)
	if err = s.Grant(ctx, "org:"+org.String(), "test:approval:"+org.String(), NanoUSD); err != nil {
		t.Fatal(err)
	}
	return v
}

func requireRegressionFault(t *testing.T, err error, code string) {
	t.Helper()
	var f *Fault
	if !errors.As(err, &f) || f.Code != code {
		t.Fatalf("want fault %q, got %v", code, err)
	}
}
