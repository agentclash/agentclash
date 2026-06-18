package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

type fakeRunCreator struct {
	est          CostEstimate
	estErr       error
	createCalled bool
	createInput  CreateRunInput
	result       CreateRunResult
	createErr    error
}

func (f *fakeRunCreator) EstimateEvalCost(_ context.Context, _ Caller, _ EstimateEvalCostInput) (CostEstimate, error) {
	return f.est, f.estErr
}
func (f *fakeRunCreator) CreateRun(_ context.Context, _ Caller, in CreateRunInput) (CreateRunResult, error) {
	f.createCalled = true
	f.createInput = in
	return f.result, f.createErr
}

func createRunCtx() context.Context {
	return context.WithValue(context.Background(), callerContextKey{}, Caller{UserID: uuid.New()})
}

func boundCreateRunArgs() json.RawMessage {
	return json.RawMessage(`{"challenge_pack_version_id":"` + uuid.New().String() + `","agent_deployment_ids":["` + uuid.New().String() + `"]}`)
}

func envelopeJSON(t *testing.T, amount int64, runID uuid.UUID, mode string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(createRunEstimateEnvelope{SchemaVersion: 1, Kind: "create_run", AmountMicros: amount, RunID: runID, BillingMode: mode})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestCreateRunTool_ExecuteRefusesUnconfirmed(t *testing.T) {
	fake := &fakeRunCreator{}
	_, err := createRunTool{runs: fake}.Execute(createRunCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateRunArgs())
	if err == nil {
		t.Fatal("expected create_run.Execute to refuse unconfirmed")
	}
	if fake.createCalled {
		t.Fatal("no run must be created on the unconfirmed path")
	}
}

func TestCreateRunTool_EstimateCostEnvelope(t *testing.T) {
	fake := &fakeRunCreator{est: CostEstimate{TotalMicros: 1_500_000, Lanes: []EvalCostLaneEstimate{{Micros: 1_500_000}}}}
	raw, err := createRunTool{runs: fake}.EstimateCost(createRunCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateRunArgs())
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	env, err := parseCreateRunEnvelope(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if env.AmountMicros != 1_500_000 || env.RunID == uuid.Nil || env.BillingMode != "managed" {
		t.Fatalf("envelope = %+v, want 1.5M managed with a run id", env)
	}
}

func TestCreateRunTool_ConfirmedReservesApprovedAmount(t *testing.T) {
	// The fake's live estimate (9.9M) differs from the APPROVED envelope (1.5M); confirmed execution
	// must reserve the approved amount, never the recomputed one.
	runID := uuid.New()
	fake := &fakeRunCreator{est: CostEstimate{TotalMicros: 9_900_000}, result: CreateRunResult{Run: domain.Run{ID: runID, Status: domain.RunStatusQueued}}}
	pc := vibeeval.PendingConfirmation{ToolName: "create_run", Estimate: envelopeJSON(t, 1_500_000, runID, "managed")}
	out, err := createRunTool{runs: fake}.ExecuteConfirmed(createRunCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateRunArgs(), pc)
	if err != nil {
		t.Fatalf("ExecuteConfirmed: %v", err)
	}
	if fake.createInput.RunID != runID || fake.createInput.ReservationMicros != 1_500_000 {
		t.Fatalf("create input = {RunID:%s, Micros:%d}, want approved {%s, 1.5M}", fake.createInput.RunID, fake.createInput.ReservationMicros, runID)
	}
	if out.AuditResult["reserved_micros"] != int64(1_500_000) {
		t.Fatalf("audit reserved = %v, want 1.5M", out.AuditResult["reserved_micros"])
	}
}

func TestCreateRunTool_ConfirmedMalformedEstimateNoSideEffect(t *testing.T) {
	for name, raw := range map[string]json.RawMessage{
		"missing":   nil,
		"garbage":   json.RawMessage(`{not json`),
		"wrongKind": json.RawMessage(`{"schema_version":1,"kind":"create_eval_session","run_id":"` + uuid.New().String() + `"}`),
		"noRunID":   json.RawMessage(`{"schema_version":1,"kind":"create_run","amount_micros":100}`),
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeRunCreator{}
			_, err := createRunTool{runs: fake}.ExecuteConfirmed(createRunCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateRunArgs(), vibeeval.PendingConfirmation{Estimate: raw})
			if err == nil {
				t.Fatal("expected error for malformed/missing estimate")
			}
			if fake.createCalled {
				t.Fatal("no run must be created when the approved estimate is invalid")
			}
		})
	}
}

func TestCreateRunTool_ConfirmedWorkflowStartFailureIsSuccess(t *testing.T) {
	runID := uuid.New()
	fake := &fakeRunCreator{createErr: RunWorkflowStartError{Run: domain.Run{ID: runID, Status: domain.RunStatusQueued}, Cause: errors.New("temporal down")}}
	out, err := createRunTool{runs: fake}.ExecuteConfirmed(createRunCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateRunArgs(), vibeeval.PendingConfirmation{Estimate: envelopeJSON(t, 1_000_000, runID, "managed")})
	if err != nil {
		t.Fatalf("workflow-start failure must finalize success-with-warning, got err: %v", err)
	}
	if out.AuditResult["workflow_start"] != "failed_pending_recovery" {
		t.Fatalf("audit = %+v, want failed_pending_recovery warning", out.AuditResult)
	}
}

func TestCreateRunTool_ConfirmedBYOKZeroReservation(t *testing.T) {
	runID := uuid.New()
	fake := &fakeRunCreator{result: CreateRunResult{Run: domain.Run{ID: runID, Status: domain.RunStatusQueued}}}
	_, err := createRunTool{runs: fake}.ExecuteConfirmed(createRunCtx(), vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()}, boundCreateRunArgs(), vibeeval.PendingConfirmation{Estimate: envelopeJSON(t, 0, runID, "byok")})
	if err != nil {
		t.Fatalf("ExecuteConfirmed: %v", err)
	}
	if fake.createInput.ReservationMicros != 0 {
		t.Fatalf("BYOK reservation = %d, want 0 (no reservation)", fake.createInput.ReservationMicros)
	}
}

func TestParseCreateRunEnvelope_BillingModeConsistency(t *testing.T) {
	rid := uuid.New().String()
	cases := map[string]json.RawMessage{
		"byokWithPositiveAmount": json.RawMessage(`{"schema_version":1,"kind":"create_run","run_id":"` + rid + `","billing_mode":"byok","amount_micros":100}`),
		"managedWithZeroAmount":  json.RawMessage(`{"schema_version":1,"kind":"create_run","run_id":"` + rid + `","billing_mode":"managed","amount_micros":0}`),
		"unknownMode":            json.RawMessage(`{"schema_version":1,"kind":"create_run","run_id":"` + rid + `","billing_mode":"weird","amount_micros":100}`),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseCreateRunEnvelope(raw); err == nil {
				t.Fatal("expected billing_mode/amount inconsistency to be rejected")
			}
		})
	}
	// Valid managed + valid byok pass.
	if _, err := parseCreateRunEnvelope(json.RawMessage(`{"schema_version":1,"kind":"create_run","run_id":"` + rid + `","billing_mode":"managed","amount_micros":100}`)); err != nil {
		t.Fatalf("valid managed rejected: %v", err)
	}
	if _, err := parseCreateRunEnvelope(json.RawMessage(`{"schema_version":1,"kind":"create_run","run_id":"` + rid + `","billing_mode":"byok","amount_micros":0}`)); err != nil {
		t.Fatalf("valid byok rejected: %v", err)
	}
}
