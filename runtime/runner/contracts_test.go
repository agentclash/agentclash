package runner

import (
	"context"
	"errors"
	"testing"
)

func TestFailureUnwrapAndAsFailure(t *testing.T) {
	cause := errors.New("boom")
	err := NewFailure(StopReasonSandboxError, "sandbox failed", cause)

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("AsFailure returned ok=false for %T", err)
	}
	if failure.StopReason != StopReasonSandboxError {
		t.Fatalf("StopReason = %q; want %q", failure.StopReason, StopReasonSandboxError)
	}
	if !errors.Is(err, cause) {
		t.Fatal("Failure must unwrap the original cause")
	}
}

func TestNoopObserverSatisfiesObserver(t *testing.T) {
	var observer Observer = NoopObserver{}
	if err := observer.OnStepStart(context.Background(), 1); err != nil {
		t.Fatalf("NoopObserver.OnStepStart: %v", err)
	}
}
