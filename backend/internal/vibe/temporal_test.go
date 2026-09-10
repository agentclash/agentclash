package vibe

import (
	"context"
	"errors"
	"github.com/agentclash/agentclash/runtime/scoring"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
)

func TestPaidTemporalActivityNeverRetries(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	paid, finalized := 0, 0
	env.RegisterActivityWithOptions(func(context.Context, string) error {
		paid++
		return errors.New("worker lost acknowledgement after paid response")
	}, activity.RegisterOptions{Name: "vibe.execute"})
	env.RegisterActivityWithOptions(func(_ context.Context, _ string, f *Fault) error {
		finalized++
		if f == nil || f.Code != "worker_interrupted" {
			t.Fatalf("unexpected worker failure: %+v", f)
		}
		return nil
	}, activity.RegisterOptions{Name: "vibe.finalize-with-fault"})
	env.ExecuteWorkflow(OperationWorkflow, "durable-operation-id")
	if !env.IsWorkflowCompleted() || env.GetWorkflowError() != nil || paid != 1 || finalized != 1 {
		t.Fatalf("paid=%d finalize=%d error=%v", paid, finalized, env.GetWorkflowError())
	}
}

func TestVibeTemporalPreservesDomainFaults(t *testing.T) {
	for _, code := range []string{"context_limit", "invalid_draft", "rate_limit", "free_capacity_reached", "usage_unknown"} {
		t.Run(code, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			want := &Fault{Code: code, Message: "A safe, specific explanation."}
			calls, finalized := 0, 0
			env.RegisterActivityWithOptions(func(context.Context, string) error {
				calls++
				return activityFailure(want)
			}, activity.RegisterOptions{Name: "vibe.execute"})
			env.RegisterActivityWithOptions(func(_ context.Context, _ string, got *Fault) error {
				finalized++
				if got == nil || *got != *want {
					t.Fatalf("fault was masked: %+v", got)
				}
				return nil
			}, activity.RegisterOptions{Name: "vibe.finalize-with-fault"})
			env.ExecuteWorkflow(OperationWorkflow, "operation")
			if env.GetWorkflowError() != nil || calls != 1 || finalized != 1 {
				t.Fatalf("unexpected retries or finalization: calls=%d finalized=%d err=%v", calls, finalized, env.GetWorkflowError())
			}
		})
	}
	converter := temporal.GetDefaultFailureConverter()
	malformed := converter.FailureToError(converter.ErrorToFailure(temporal.NewNonRetryableApplicationError("private provider response", "vibe_fault", nil, "invalid details")))
	if got := operationFailure(malformed); got.Code != "worker_interrupted" {
		t.Fatal("malformed error details leaked")
	}
}

func TestVibeTemporalOldHistoriesKeepOriginalFinalizer(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnGetVersion(faultFinalizerVersion, workflow.DefaultVersion, workflow.Version(1)).Return(workflow.DefaultVersion)
	finalized := 0
	env.RegisterActivityWithOptions(func(context.Context, string) error { return errors.New("old failure") }, activity.RegisterOptions{Name: "vibe.execute"})
	env.RegisterActivityWithOptions(func(_ context.Context, _ string, code string) error {
		finalized++
		if code != "worker_interrupted" {
			t.Fatal("changed old history finalization")
		}
		return nil
	}, activity.RegisterOptions{Name: "vibe.finalize"})
	env.ExecuteWorkflow(OperationWorkflow, "old-operation")
	if env.GetWorkflowError() != nil || finalized != 1 {
		t.Fatalf("legacy finalization failed: %v", env.GetWorkflowError())
	}
}
func TestInvalidJudgeIsUnknown(t *testing.T) {
	for _, b := range []string{`{}`, `{"reasoning":"fine"}`, `{"pass":"yes","reasoning":"x"}`, `{"pass":true,"reasoning":"x","tool":"spend"}`} {
		r, err := ParseJudge(scoring.LLMJudgeDeclaration{Mode: scoring.JudgeMethodAssertion}, []byte(b), LimitsFor(true))
		if err == nil || r.Verdict != Unknown {
			t.Fatalf("%s became %s", b, r.Verdict)
		}
	}
}
