package vibe

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func failedRetrySource(t *testing.T, s *Store) (Session, Operation, Plan) {
	t.Helper()
	v, o, p := outcomeOperation(t, s, 10)
	if err := s.Finish(context.Background(), o.ID, &Fault{Code: "invalid_response", Message: "Tests were not prepared."}); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetSession(context.Background(), v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	o, err = s.Operation(context.Background(), o.ID)
	if err != nil {
		t.Fatal(err)
	}
	return v, o, p
}

func TestIntegrationVibeRetryPreservesSourceAndDeduplicates(t *testing.T) {
	s := integrationStore(t)
	v, original, _ := failedRetrySource(t, s)
	ctx := context.Background()
	service := &Service{Store: s, Config: testConfig(), Gate: testGate(t)}
	request := RetryRequest{ClientID: uuid.New(), Revision: v.Revision}
	retried, err := service.Retry(ctx, v.Actor, v.ID, original.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.Retry(ctx, v.Actor, v.ID, original.ID, request)
	if err != nil || again.ID != retried.ID {
		t.Fatalf("retry acknowledgement was not idempotent: %+v %v", again, err)
	}
	current, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	var plan Plan
	if err = json.Unmarshal(retried.Input, &plan); err != nil {
		t.Fatal(err)
	}
	if len(current.Document.Messages) != 1 || len(current.Operations) != 2 || plan.Retry == nil || plan.Retry.OperationID != original.ID || plan.sourceMessageID() != current.Document.Messages[0].ID || plan.Submission.Content != current.Document.Messages[0].Content || plan.Submission.Models != original.Models {
		t.Fatalf("retry lost its source or duplicated the user turn: %+v %+v", plan, current.Document)
	}
	_, err = service.Retry(ctx, v.Actor, v.ID, original.ID, RetryRequest{ClientID: uuid.New(), Revision: current.Revision})
	requireFault(t, err, "retry_running")
	if _, _, err = s.Start(ctx, retried.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteDocument(ctx, retried.ID, "Prepared on retry.", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, retried.ID, nil); err != nil {
		t.Fatal(err)
	}
	again, err = service.Retry(ctx, v.Actor, v.ID, original.ID, request)
	if err != nil || again.ID != retried.ID || again.Completion == nil {
		t.Fatalf("completed retry acknowledgement changed: %+v %v", again, err)
	}
}

func TestIntegrationVibeRetryRejectsCrossSessionAndStaleRevision(t *testing.T) {
	s := integrationStore(t)
	v, original, _ := failedRetrySource(t, s)
	other := anonSession(t, s)
	service := &Service{Store: s, Config: testConfig(), Gate: testGate(t)}
	_, err := service.Retry(context.Background(), other.Actor, other.ID, original.ID, RetryRequest{ClientID: uuid.New(), Revision: other.Revision})
	requireFault(t, err, "not_found")
	_, err = service.Retry(context.Background(), v.Actor, v.ID, original.ID, RetryRequest{ClientID: uuid.New(), Revision: v.Revision - 1})
	requireFault(t, err, "revision_conflict")
}

func TestIntegrationVibeRetryRejectsUncertainCostAndChangedBase(t *testing.T) {
	for _, uncertain := range []bool{true, false} {
		t.Run(map[bool]string{true: "uncertain cost", false: "changed base"}[uncertain], func(t *testing.T) {
			s := integrationStore(t)
			v, o, _ := outcomeOperation(t, s, 10)
			ctx := context.Background()
			if uncertain {
				a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "route", Role: Assistant, Model: o.Models.Assistant, Policy: json.RawMessage(`{}`), InputBound: 10, MaxOutput: 10, MaxCost: 100}
				if err := s.BeginAttempt(ctx, a); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Finish(ctx, o.ID, &Fault{Code: "worker_interrupted", Message: "Interrupted"}); err != nil {
				t.Fatal(err)
			}
			v, err := s.GetSession(ctx, v.Actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !uncertain {
				if err = s.Edit(ctx, v.Actor, v.ID, v.Revision, func(current *Session) error {
					current.Document.Artifacts = append(current.Document.Artifacts, Artifact{ID: uuid.New(), Kind: "test_suite", Title: "Newer tests", Blueprint: json.RawMessage(`{"cases":[{}]}`), CreatedAt: timestamp().Add(time.Second)})
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				v, _ = s.GetSession(ctx, v.Actor, v.ID)
			}
			service := &Service{Store: s, Config: testConfig(), Gate: testGate(t)}
			_, err = service.Retry(ctx, v.Actor, v.ID, o.ID, RetryRequest{ClientID: uuid.New(), Revision: v.Revision})
			requireFault(t, err, map[bool]string{true: "retry_uncertain", false: "retry_stale"}[uncertain])
		})
	}
}

func TestIntegrationVibeRetryAllowsManualRateLimitDuringReconciliation(t *testing.T) {
	s := integrationStore(t)
	v, original, _ := outcomeOperation(t, s, 11)
	ctx := context.Background()
	attempt := Attempt{ID: uuid.New(), OperationID: original.ID, Step: "route", Role: Assistant, Model: original.Models.Assistant, Policy: json.RawMessage(`{}`), InputBound: 10, MaxOutput: 10, MaxCost: 100}
	if err := s.BeginAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	rateLimit := &Fault{Code: "provider_rate_limit", Message: "The selected model's provider is busy."}
	if err := s.EndAttempt(ctx, attempt, "", json.RawMessage(`{}`), nil, rateLimit); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(ctx, original.ID, rateLimit); err != nil {
		t.Fatal(err)
	}

	v, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := s.Operation(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Billing != Reconciling || !v.Operations[0].Retryable {
		t.Fatalf("rate limit should retain accounting while allowing manual retry: operation=%+v summary=%+v", failed, v.Operations[0])
	}

	service := &Service{Store: s, Config: testConfig(), Gate: testGate(t)}
	retried, err := service.Retry(ctx, v.Actor, v.ID, original.ID, RetryRequest{ClientID: uuid.New(), Revision: v.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if retried.State != Queued || retried.RetryOfOperationID == nil || *retried.RetryOfOperationID != original.ID {
		t.Fatalf("manual rate-limit retry was not admitted as a new operation: %+v", retried)
	}
	failed, err = s.Operation(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Billing != Reconciling {
		t.Fatalf("manual retry must not release an uncertain original cost: %+v", failed)
	}
}

func TestIntegrationVibeRetryAdmissionCannotChangeOriginalRequest(t *testing.T) {
	s := integrationStore(t)
	v, original, originalPlan := failedRetrySource(t, s)
	sub := retrySubmission(originalPlan.Submission, original.ID, RetryRequest{ClientID: uuid.New(), Revision: v.Revision})
	retry := retryContext(original, originalPlan)
	sub.Content = "Silently replace the original request."
	p := originalPlan
	p.Submission, p.Retry = sub, &retry
	_, err := s.Submit(context.Background(), v.Actor, v.ID, sub, p, testConfig())
	requireFault(t, err, "retry_not_allowed")
}
