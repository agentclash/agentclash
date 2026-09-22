package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func outcomeOperation(t *testing.T, s *Store, version int) (Session, Operation, Plan) {
	t.Helper()
	v := anonSession(t, s)
	sub := Submission{ClientID: uuid.New(), Kind: "message", Content: "Prepare tests for unopened returns within 14 days.", Models: DefaultModels(), TestJourney: true}
	p := Plan{AuthoringVersion: version, Anonymous: true, Submission: sub, Document: v.Document, Calls: 1, MaxCost: NanoUSD / 100}
	o, err := s.Submit(context.Background(), v.Actor, v.ID, sub, p, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	o, _, err = s.Start(context.Background(), o.ID)
	if err != nil {
		t.Fatal(err)
	}
	return v, o, p
}

func requireFault(t *testing.T, err error, code string) {
	t.Helper()
	var f *Fault
	if !errors.As(err, &f) || f.Code != code {
		t.Fatalf("wanted %s, got %v", code, err)
	}
}

func TestVibeOperationTimeoutKeepsLegacyPlans(t *testing.T) {
	frozen := Limits{OperationSeconds: 315}
	if got := (Plan{AuthoringVersion: 10, Anonymous: true, ExecutionLimits: &frozen}).operationTimeout(); got != 180*time.Second {
		t.Fatalf("legacy timeout changed to %v", got)
	}
	if got := (Plan{AuthoringVersion: 11, Anonymous: true, ExecutionLimits: &frozen}).operationTimeout(); got != 315*time.Second {
		t.Fatalf("frozen timeout ignored: %v", got)
	}
}

func TestIntegrationVibeCompletionReceiptSurvivesLostAcknowledgement(t *testing.T) {
	s := integrationStore(t)
	v, o, _ := outcomeOperation(t, s, 11)
	ctx := context.Background()
	a := Artifact{ID: uuid.New(), Kind: "test_suite", Title: "Returns", Blueprint: json.RawMessage(`{"cases":[{"key":"one"},{"key":"two"}]}`), CreatedAt: timestamp()}
	completion := AuthoringCompletion{Outcome: &CompletionReceipt{Action: "prepare_tests", CommandHash: Hash([]byte("create returns tests")), ValidationStatus: "supported"}}
	if err := s.CompleteDocument(ctx, o.ID, "", &a, nil, completion); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteDocument(ctx, o.ID, "optional prose changed", &a, nil, completion); err != nil {
		t.Fatalf("identical domain effect was not idempotent: %v", err)
	}
	requireFault(t, s.BeginAttempt(ctx, Attempt{ID: uuid.New(), OperationID: o.ID, Step: "route", Role: Assistant, Model: o.Models.Assistant, InputBound: 10, MaxOutput: 10, MaxCost: 100}), "operation_stopped")
	if err := s.Finish(ctx, o.ID, &Fault{Code: "worker_interrupted", Message: "Lost acknowledgement"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(ctx, o.ID, &Fault{Code: "worker_interrupted", Message: "Duplicate finalizer"}); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Document.Artifacts) != 1 || len(current.Document.Messages) != 2 || current.Document.Messages[1].Content != "2 tests are ready." {
		t.Fatalf("completion was duplicated or misrepresented: %+v", current.Document)
	}
	result := current.Operations[0]
	if result.State != Completed || result.Error != nil || result.Completion == nil || result.Completion.CaseCount != 2 || result.Completion.ArtifactID == nil || *result.Completion.ArtifactID != a.ID || result.Completion.MessageID != current.Document.Messages[1].ID {
		t.Fatalf("lost acknowledgement changed completion: %+v", result)
	}
	var events int
	if err := s.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_events WHERE operation_id=$1 AND kind='operation.finished'", o.ID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("completion events=%d err=%v", events, err)
	}
	b := a
	b.Title = "Conflicting replacement"
	requireFault(t, s.CompleteDocument(ctx, o.ID, "", &b, nil, completion), "completion_conflict")
}

func TestIntegrationVibeCompletionAndStopHaveOneWinner(t *testing.T) {
	for _, commitFirst := range []bool{false, true} {
		t.Run(map[bool]string{true: "commit first", false: "stop first"}[commitFirst], func(t *testing.T) {
			s := integrationStore(t)
			v, o, _ := outcomeOperation(t, s, 10)
			ctx := context.Background()
			if commitFirst {
				if err := s.CompleteDocument(ctx, o.ID, "A recorded answer.", nil, nil); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Stop(ctx, v.Actor, o.ID); err != nil {
				t.Fatal(err)
			}
			if !commitFirst {
				requireFault(t, s.CompleteDocument(ctx, o.ID, "Too late.", nil, nil), "operation_stopped")
			}
			result, err := s.Operation(ctx, o.ID)
			if err != nil {
				t.Fatal(err)
			}
			if commitFirst && (result.State != Completed || result.Completion == nil) || !commitFirst && (result.State != Cancelled || result.Completion != nil) {
				t.Fatalf("incorrect lock winner: %+v", result)
			}
		})
	}
}

func TestIntegrationVibeCompletionRollsBackWithInvalidDomainState(t *testing.T) {
	s := integrationStore(t)
	v, o, _ := outcomeOperation(t, s, 11)
	ctx := context.Background()
	err := s.CompleteDocument(ctx, o.ID, "", nil, nil, AuthoringCompletion{Policy: &PolicySnapshot{ID: uuid.New()}})
	if err == nil {
		t.Fatal("policy committed without matching artifact validation")
	}
	current, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil || len(current.Document.Messages) != 1 || len(current.Document.Artifacts) != 0 || current.Operations[0].Completion != nil {
		t.Fatalf("partial completion survived rollback: %+v %v", current, err)
	}
}

func TestIntegrationVibeLegacyCommittedResponseRecoveredWithoutDispatch(t *testing.T) {
	s := integrationStore(t)
	v, o, _ := outcomeOperation(t, s, 10)
	ctx := context.Background()
	if err := s.CompleteDocument(ctx, o.ID, "Already saved before migration.", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, "UPDATE vibe_operations SET completion_receipt=NULL,state='FAILED' WHERE id=$1", o.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(ctx, o.ID, &Fault{Code: "worker_interrupted", Message: "Old worker stopped"}); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil || current.Operations[0].State != Completed || current.Operations[0].Completion == nil || current.Operations[0].ModelCalls != 0 || len(current.Document.Messages) != 2 {
		t.Fatalf("legacy completion recovery failed: %+v %v", current, err)
	}
}

func TestIntegrationVibeDomainJournalAndRecordedResponse(t *testing.T) {
	s := integrationStore(t)
	_, o, _ := outcomeOperation(t, s, 11)
	ctx := context.Background()
	format := json.RawMessage(`{"type":"json_object"}`)
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "route", Role: Assistant, Model: o.Models.Assistant, Policy: raw(map[string]any{"response_format": format}), RequestHash: Hash([]byte("request")), InputBound: 100, MaxOutput: 20, MaxCost: 1000}
	if err := s.BeginAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	_, err := s.RecordedResponse(ctx, o.ID, a.Step, a.RequestHash, format)
	requireFault(t, err, "recovery_unavailable")
	cost := int64(100)
	response := provider.Response{OutputText: `{"intent":"chat","reply":"Hello"}`}
	if err = s.EndAttempt(ctx, a, response.OutputText, raw(response), &cost, nil); err != nil {
		t.Fatal(err)
	}
	outcome := DomainOutcome{Stage: "route", Status: "accepted", SchemaVersion: "v11", CandidateHash: Hash([]byte("candidate"))}
	if err = s.RecordDomainOutcome(ctx, o.ID, a.Step, outcome); err != nil {
		t.Fatal(err)
	}
	if err = s.RecordDomainOutcome(ctx, o.ID, a.Step, outcome); err != nil {
		t.Fatal(err)
	}
	outcome.Status = "rejected"
	requireFault(t, s.RecordDomainOutcome(ctx, o.ID, a.Step, outcome), "outcome_conflict")
	got, err := s.RecordedResponse(ctx, o.ID, a.Step, a.RequestHash, format)
	if err != nil || got.OutputText != response.OutputText {
		t.Fatalf("complete response was unavailable: %+v %v", got, err)
	}
	_, err = s.RecordedResponse(ctx, o.ID, a.Step, Hash([]byte("different request")), format)
	requireFault(t, err, "recovery_unavailable")
	_, err = s.RecordedResponse(ctx, o.ID, a.Step, a.RequestHash, json.RawMessage(`{"type":"different"}`))
	requireFault(t, err, "recovery_unavailable")
	if _, err = s.DB.Exec(ctx, "UPDATE vibe_attempts SET state='RECONCILED' WHERE id=$1", a.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.RecordedResponse(ctx, o.ID, a.Step, a.RequestHash, format)
	requireFault(t, err, "recovery_unavailable")
}
