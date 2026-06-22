package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

type fakeSessionCreator struct {
	est          EvalSessionCostEstimate
	estErr       error
	createCalled bool
	createInput  CreateEvalSessionInput
	result       CreateEvalSessionResult
	createErr    error
}

func (f *fakeSessionCreator) EstimateEvalSessionCost(_ context.Context, _ Caller, _ CreateEvalSessionInput) (EvalSessionCostEstimate, error) {
	return f.est, f.estErr
}
func (f *fakeSessionCreator) CreateEvalSession(_ context.Context, _ Caller, in CreateEvalSessionInput) (CreateEvalSessionResult, error) {
	f.createCalled = true
	f.createInput = in
	return f.result, f.createErr
}

func sessionCtx() context.Context {
	return context.WithValue(context.Background(), callerContextKey{}, Caller{UserID: uuid.New()})
}

// boundCreateEvalSessionArgs is a minimal valid create_eval_session argument object.
func boundCreateEvalSessionArgs() json.RawMessage {
	return json.RawMessage(`{"challenge_pack_version_id":"` + uuid.New().String() + `","participants":[{"agent_deployment_id":"` + uuid.New().String() + `","label":"Primary"}],"execution_mode":"single_agent","eval_session":{"repetitions":2,"aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},"routing_task_snapshot":{"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}},"seed_fanout":{"strategy":"explicit","seeds":[1,2]},"schema_version":1}}`)
}

func sessionEnvelope(t *testing.T, sessionID uuid.UUID, children []createEvalSessionChildEnvelope) json.RawMessage {
	t.Helper()
	var aggregate int64
	for _, c := range children {
		aggregate += c.AmountMicros
	}
	raw, err := json.Marshal(createEvalSessionEnvelope{
		SchemaVersion:   1,
		Kind:            "create_eval_session",
		EvalSessionID:   sessionID,
		AggregateMicros: aggregate,
		Children:        children,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestCreateEvalSessionTool_ExecuteRefusesUnconfirmed(t *testing.T) {
	fake := &fakeSessionCreator{}
	_, err := createEvalSessionTool{sessions: fake}.Execute(sessionCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateEvalSessionArgs())
	if err == nil {
		t.Fatal("expected create_eval_session.Execute to refuse unconfirmed")
	}
	if fake.createCalled {
		t.Fatal("no session must be created on the unconfirmed path")
	}
}

func TestCreateEvalSessionTool_EstimateCostEnvelope(t *testing.T) {
	seed := int64(7)
	fake := &fakeSessionCreator{est: EvalSessionCostEstimate{
		AggregateMicros: 3_000_000,
		Children: []EvalSessionChildCostEstimate{
			{MatrixKey: "m1", AmountMicros: 2_000_000, BillingMode: "managed"},
			{MatrixKey: "m2", Seed: &seed, AmountMicros: 1_000_000, BillingMode: "managed"},
		},
	}}
	raw, err := createEvalSessionTool{sessions: fake}.EstimateCost(sessionCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateEvalSessionArgs())
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	env, err := parseCreateEvalSessionEnvelope(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if env.EvalSessionID == uuid.Nil || env.AggregateMicros != 3_000_000 || len(env.Children) != 2 {
		t.Fatalf("envelope = %+v, want session id + 3M aggregate + 2 children", env)
	}
	for _, c := range env.Children {
		if c.RunID == uuid.Nil {
			t.Fatalf("child %+v has no preallocated run id", c)
		}
	}
	if env.Children[1].Seed == nil || *env.Children[1].Seed != 7 {
		t.Fatalf("second child seed = %v, want 7 carried through", env.Children[1].Seed)
	}
}

func TestCreateEvalSessionTool_EstimateFailureNoEnvelope(t *testing.T) {
	fake := &fakeSessionCreator{estErr: ErrVibeEvalMixedBilling}
	_, err := createEvalSessionTool{sessions: fake}.EstimateCost(sessionCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateEvalSessionArgs())
	if err == nil {
		t.Fatal("expected estimate failure to propagate (no envelope)")
	}
}

func TestCreateEvalSessionTool_ConfirmedBindsApprovedPlan(t *testing.T) {
	sessionID := uuid.New()
	runA, runB := uuid.New(), uuid.New()
	// The fake's live estimate is irrelevant; confirmed execution binds the APPROVED envelope amounts.
	fake := &fakeSessionCreator{result: CreateEvalSessionResult{
		Session: domain.EvalSession{ID: sessionID, Status: domain.EvalSessionStatusQueued},
		RunIDs:  []uuid.UUID{runA, runB},
	}}
	env := sessionEnvelope(t, sessionID, []createEvalSessionChildEnvelope{
		{RunID: runA, AmountMicros: 2_000_000, BillingMode: "managed"},
		{RunID: runB, AmountMicros: 0, BillingMode: "byok"},
	})
	out, err := createEvalSessionTool{sessions: fake}.ExecuteConfirmed(sessionCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateEvalSessionArgs(), vibeeval.PendingConfirmation{Estimate: env})
	if err != nil {
		t.Fatalf("ExecuteConfirmed: %v", err)
	}
	if fake.createInput.EvalSessionID != sessionID {
		t.Fatalf("session id = %s, want approved %s", fake.createInput.EvalSessionID, sessionID)
	}
	if len(fake.createInput.ChildReservations) != 2 {
		t.Fatalf("child reservations = %d, want 2", len(fake.createInput.ChildReservations))
	}
	if fake.createInput.ChildReservations[0].RunID != runA || fake.createInput.ChildReservations[0].AmountMicros != 2_000_000 {
		t.Fatalf("managed binding = %+v, want runA 2M", fake.createInput.ChildReservations[0])
	}
	if fake.createInput.ChildReservations[1].AmountMicros != 0 {
		t.Fatalf("byok binding amount = %d, want 0", fake.createInput.ChildReservations[1].AmountMicros)
	}
	if out.AuditResult["reserved_micros"] != int64(2_000_000) {
		t.Fatalf("audit reserved = %v, want 2M aggregate", out.AuditResult["reserved_micros"])
	}
}

func TestCreateEvalSessionTool_ConfirmedWorkflowStartFailureIsSuccess(t *testing.T) {
	sessionID := uuid.New()
	runA := uuid.New()
	fake := &fakeSessionCreator{createErr: EvalSessionWorkflowStartError{
		Session: domain.EvalSession{ID: sessionID, Status: domain.EvalSessionStatusQueued},
		RunIDs:  []uuid.UUID{runA},
		Cause:   errors.New("temporal down"),
	}}
	env := sessionEnvelope(t, sessionID, []createEvalSessionChildEnvelope{{RunID: runA, AmountMicros: 1_000_000, BillingMode: "managed"}})
	out, err := createEvalSessionTool{sessions: fake}.ExecuteConfirmed(sessionCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateEvalSessionArgs(), vibeeval.PendingConfirmation{Estimate: env})
	if err != nil {
		t.Fatalf("workflow-start failure must finalize success-with-warning, got err: %v", err)
	}
	if out.AuditResult["workflow_start"] != "failed_pending_recovery" {
		t.Fatalf("audit = %+v, want failed_pending_recovery warning", out.AuditResult)
	}
}

func TestCreateEvalSessionTool_ConfirmedMalformedEnvelopeNoSideEffect(t *testing.T) {
	rid := uuid.New().String()
	sid := uuid.New().String()
	cases := map[string]json.RawMessage{
		"missing":           nil,
		"garbage":           json.RawMessage(`{not json`),
		"wrongKind":         json.RawMessage(`{"schema_version":1,"kind":"create_run","eval_session_id":"` + sid + `","children":[]}`),
		"noSessionID":       json.RawMessage(`{"schema_version":1,"kind":"create_eval_session","children":[{"run_id":"` + rid + `","amount_micros":1,"billing_mode":"managed"}],"aggregate_micros":1}`),
		"noChildren":        json.RawMessage(`{"schema_version":1,"kind":"create_eval_session","eval_session_id":"` + sid + `","children":[],"aggregate_micros":0}`),
		"aggregateMismatch": json.RawMessage(`{"schema_version":1,"kind":"create_eval_session","eval_session_id":"` + sid + `","children":[{"run_id":"` + rid + `","amount_micros":2,"billing_mode":"managed"}],"aggregate_micros":5}`),
		"byokPositive":      json.RawMessage(`{"schema_version":1,"kind":"create_eval_session","eval_session_id":"` + sid + `","children":[{"run_id":"` + rid + `","amount_micros":2,"billing_mode":"byok"}],"aggregate_micros":2}`),
		"managedZero":       json.RawMessage(`{"schema_version":1,"kind":"create_eval_session","eval_session_id":"` + sid + `","children":[{"run_id":"` + rid + `","amount_micros":0,"billing_mode":"managed"}],"aggregate_micros":0}`),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &fakeSessionCreator{}
			_, err := createEvalSessionTool{sessions: fake}.ExecuteConfirmed(sessionCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateEvalSessionArgs(), vibeeval.PendingConfirmation{Estimate: raw})
			if err == nil {
				t.Fatal("expected error for malformed/inconsistent envelope")
			}
			if fake.createCalled {
				t.Fatal("no session must be created when the approved envelope is invalid")
			}
		})
	}
}

func TestParseCreateEvalSessionEnvelope_DuplicateChildRunID(t *testing.T) {
	rid := uuid.New().String()
	sid := uuid.New().String()
	raw := json.RawMessage(`{"schema_version":1,"kind":"create_eval_session","eval_session_id":"` + sid + `","aggregate_micros":4,"children":[{"run_id":"` + rid + `","amount_micros":2,"billing_mode":"managed"},{"run_id":"` + rid + `","amount_micros":2,"billing_mode":"managed"}]}`)
	if _, err := parseCreateEvalSessionEnvelope(raw); err == nil {
		t.Fatal("expected duplicate child run id to be rejected")
	}
}

// childPlanWithLane builds a single-lane managed child plan for a given deployment + snapshot.
func childPlanWithLane(deploymentID, snapshotID uuid.UUID, byok bool) evalSessionChildPlan {
	d := repository.RunnableDeployment{ID: deploymentID, AgentDeploymentSnapshotID: snapshotID}
	if byok {
		acct := uuid.New()
		d.SourceProviderAccountID = &acct
	}
	return evalSessionChildPlan{
		Run:         repository.CreateQueuedRunParams{ExecutionMode: "single_agent"},
		Deployments: []repository.RunnableDeployment{d},
		MatrixKey:   "",
	}
}

func laneBinding(runID, deploymentID, snapshotID uuid.UUID, micros int64, managed bool) EvalSessionChildReservation {
	return EvalSessionChildReservation{
		RunID:        runID,
		AmountMicros: micros,
		Lanes:        []EvalSessionChildLane{{DeploymentID: deploymentID, AgentDeploymentSnapshotID: snapshotID, Managed: managed}},
	}
}

func TestBindEvalSessionChildReservations_MatchingLanesBind(t *testing.T) {
	dep, snap, runID := uuid.New(), uuid.New(), uuid.New()
	children := []evalSessionChildPlan{childPlanWithLane(dep, snap, false)}
	bindings := []EvalSessionChildReservation{laneBinding(runID, dep, snap, 2_000_000, true)}
	childRuns, err := bindEvalSessionChildReservations(children, bindings)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if childRuns[0].RunID != runID || childRuns[0].EvalCreditReservation == nil || childRuns[0].EvalCreditReservation.AmountMicros != 2_000_000 {
		t.Fatalf("childRun = %+v, want runID %s + 2M reservation", childRuns[0], runID)
	}
}

func TestBindEvalSessionChildReservations_LaneDriftFailsClosed(t *testing.T) {
	dep, snap, runID := uuid.New(), uuid.New(), uuid.New()
	children := []evalSessionChildPlan{childPlanWithLane(dep, snap, false)}

	cases := map[string]EvalSessionChildReservation{
		// Snapshot changed under the same deployment ⇒ a different frozen model/rate/BYOK lane.
		"changed_snapshot": laneBinding(runID, dep, uuid.New(), 2_000_000, true),
		// Deployment changed entirely.
		"changed_deployment": laneBinding(runID, uuid.New(), snap, 2_000_000, true),
		// Lane flipped managed→BYOK (envelope said managed; fresh lane is managed too, but binding claims BYOK).
		"flipped_managed_flag": laneBinding(runID, dep, snap, 2_000_000, false),
		// Lane count mismatch (binding has zero lanes).
		"lane_count_mismatch": {RunID: runID, AmountMicros: 2_000_000, Lanes: nil},
	}
	for name, binding := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := bindEvalSessionChildReservations(children, []EvalSessionChildReservation{binding})
			var verr RunCreationValidationError
			if !errors.As(err, &verr) || verr.Code != "stale_estimate" {
				t.Fatalf("err = %v, want stale_estimate validation error", err)
			}
		})
	}
}

func TestBindEvalSessionChildReservations_EmptyBindingsIsRESTPassthrough(t *testing.T) {
	dep, snap := uuid.New(), uuid.New()
	children := []evalSessionChildPlan{childPlanWithLane(dep, snap, false)}
	childRuns, err := bindEvalSessionChildReservations(children, nil)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if childRuns[0].RunID != uuid.Nil || childRuns[0].EvalCreditReservation != nil {
		t.Fatalf("REST path must not preallocate ids or reserve, got %+v", childRuns[0])
	}
}

func TestParseCreateEvalSessionEnvelope_ValidManagedAndBYOK(t *testing.T) {
	sid := uuid.New()
	runA, runB := uuid.New(), uuid.New()
	raw := sessionEnvelope(t, sid, []createEvalSessionChildEnvelope{
		{RunID: runA, AmountMicros: 2_000_000, BillingMode: "managed"},
		{RunID: runB, AmountMicros: 0, BillingMode: "byok"},
	})
	env, err := parseCreateEvalSessionEnvelope(raw)
	if err != nil {
		t.Fatalf("valid mixed managed/byok session rejected: %v", err)
	}
	if env.AggregateMicros != 2_000_000 || len(env.Children) != 2 {
		t.Fatalf("env = %+v, want 2M aggregate + 2 children", env)
	}
}
