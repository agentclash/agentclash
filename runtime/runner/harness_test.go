package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
)

type terminalObserver struct {
	NoopObserver
	completeResult Result
	completeErr    error
	failureErr     error
	failureSeen    error
}

func (o *terminalObserver) OnRunComplete(_ context.Context, result Result) error {
	o.completeResult = result
	return o.completeErr
}

func (o *terminalObserver) OnRunFailure(_ context.Context, err error) error {
	o.failureSeen = err
	return o.failureErr
}

func TestFinishWithObserverRecordsCompletion(t *testing.T) {
	observer := &terminalObserver{}
	result := Result{
		FinalOutput: "done",
		StopReason:  StopReasonCompleted,
		Usage:       provider.Usage{InputTokens: 12},
	}
	var err error

	FinishWithObserver(context.Background(), observer, &result, &err, TerminalObserverMessages{
		Failure:    "failure message",
		Completion: "completion message",
	})

	if err != nil {
		t.Fatalf("FinishWithObserver err = %v; want nil", err)
	}
	if observer.completeResult.FinalOutput != "done" {
		t.Fatalf("OnRunComplete result = %+v; want final output done", observer.completeResult)
	}
	if result.FinalOutput != "done" {
		t.Fatalf("result was mutated on success: %+v", result)
	}
}

func TestFinishWithObserverConvertsCompletionObserverError(t *testing.T) {
	observerErr := errors.New("observer failed")
	observer := &terminalObserver{completeErr: observerErr}
	result := Result{FinalOutput: "done", StopReason: StopReasonCompleted}
	var err error

	FinishWithObserver(context.Background(), observer, &result, &err, TerminalObserverMessages{
		Failure:    "failure message",
		Completion: "completion message",
	})

	if result != (Result{}) {
		t.Fatalf("result = %+v; want zero result", result)
	}
	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("err = %T; want Failure", err)
	}
	if failure.StopReason != StopReasonObserverError {
		t.Fatalf("StopReason = %q; want %q", failure.StopReason, StopReasonObserverError)
	}
	if failure.Message != "completion message" {
		t.Fatalf("Message = %q; want completion message", failure.Message)
	}
	if !errors.Is(err, observerErr) {
		t.Fatal("observer error must be preserved")
	}
}

func TestFinishWithObserverJoinsFailureObserverError(t *testing.T) {
	runErr := errors.New("run failed")
	observerErr := errors.New("observer failed")
	observer := &terminalObserver{failureErr: observerErr}
	result := Result{FinalOutput: "ignored"}
	err := runErr

	FinishWithObserver(context.Background(), observer, &result, &err, TerminalObserverMessages{
		Failure:    "failure message",
		Completion: "completion message",
	})

	if !errors.Is(err, runErr) {
		t.Fatal("run error must be preserved")
	}
	if !errors.Is(err, observerErr) {
		t.Fatal("observer error must be joined")
	}
	if observer.failureSeen != runErr {
		t.Fatalf("OnRunFailure saw %v; want run error", observer.failureSeen)
	}
	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("joined err = %T; want Failure present", err)
	}
	if failure.StopReason != StopReasonObserverError {
		t.Fatalf("StopReason = %q; want %q", failure.StopReason, StopReasonObserverError)
	}
}

func TestFinishWithObserverAllowsNilObserver(t *testing.T) {
	result := Result{FinalOutput: "done", StopReason: StopReasonCompleted}
	var err error

	FinishWithObserver(context.Background(), nil, &result, &err, TerminalObserverMessages{})

	if err != nil {
		t.Fatalf("FinishWithObserver with nil observer err = %v; want nil", err)
	}
}

func TestWithRuntimeTimeoutNoopsWhenUnset(t *testing.T) {
	parent := context.Background()
	timed := WithRuntimeTimeout(parent, 0)
	defer timed.Cancel()

	if timed.Context != parent {
		t.Fatal("WithRuntimeTimeout should return parent context for non-positive timeout")
	}
	if _, ok := timed.Context.Deadline(); ok {
		t.Fatal("non-positive timeout should not set a deadline")
	}
}

func TestWithRuntimeTimeoutAppliesDeadline(t *testing.T) {
	timed := WithRuntimeTimeout(context.Background(), time.Minute)
	defer timed.Cancel()

	deadline, ok := timed.Context.Deadline()
	if !ok {
		t.Fatal("positive timeout should set a deadline")
	}
	if time.Until(deadline) <= 0 {
		t.Fatalf("deadline = %v; want future deadline", deadline)
	}
}
