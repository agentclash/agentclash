package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
)

func responseWithUsage(input, output int64) provider.Response {
	return provider.Response{
		Usage: provider.Usage{
			InputTokens:  input,
			OutputTokens: output,
			TotalTokens:  input + output,
		},
	}
}

func TestBudgetGuardObserver_UnderLimit(t *testing.T) {
	inner := &recordingObserver{}
	// Hard limit $1.00, input $10/M, output $30/M.
	guard := NewBudgetGuardObserver(inner, 1.0, 10.0, 30.0)

	// 1000 input tokens + 500 output tokens =
	// (1000/1M)*10 + (500/1M)*30 = 0.01 + 0.015 = $0.025
	resp := responseWithUsage(1000, 500)
	err := guard.OnProviderResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	calls := inner.getCalls()
	if len(calls) != 1 || calls[0] != "OnProviderResponse" {
		t.Errorf("expected [OnProviderResponse], got %v", calls)
	}
}

func TestBudgetGuardObserver_ExceedsLimit(t *testing.T) {
	inner := &recordingObserver{}
	// Hard limit $0.01, input $10/M, output $30/M.
	guard := NewBudgetGuardObserver(inner, 0.01, 10.0, 30.0)

	// 1000 input + 500 output = $0.025, exceeds $0.01.
	resp := responseWithUsage(1000, 500)
	err := guard.OnProviderResponse(context.Background(), resp)
	if err == nil {
		t.Fatal("expected budget exceeded error, got nil")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected error to contain 'budget exceeded', got: %v", err)
	}
	if !strings.Contains(err.Error(), "hard limit") {
		t.Errorf("expected error to contain 'hard limit', got: %v", err)
	}

	// Inner observer should NOT have been called.
	calls := inner.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected no inner calls when budget exceeded, got %v", calls)
	}
}

func TestBudgetGuardObserver_AccumulatesAcrossCalls(t *testing.T) {
	inner := &recordingObserver{}
	// Hard limit $0.05, input $10/M, output $30/M.
	guard := NewBudgetGuardObserver(inner, 0.05, 10.0, 30.0)

	// Each call: 1000 input + 500 output = $0.025.
	resp := responseWithUsage(1000, 500)

	// First call: accumulated = $0.025, under limit.
	err := guard.OnProviderResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("first call: expected no error, got: %v", err)
	}

	// Second call: accumulated = $0.050, still under limit (not strictly greater).
	err = guard.OnProviderResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("second call: expected no error at exact limit, got: %v", err)
	}

	// Third call: accumulated = $0.075, exceeds $0.05.
	err = guard.OnProviderResponse(context.Background(), resp)
	if err == nil {
		t.Fatal("third call: expected budget exceeded error, got nil")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected error to contain 'budget exceeded', got: %v", err)
	}

	// Inner should have been called twice (first two calls delegated).
	calls := inner.getCalls()
	if len(calls) != 2 {
		t.Errorf("expected 2 inner calls, got %d: %v", len(calls), calls)
	}
}

func TestBudgetGuardObserver_ZeroLimit(t *testing.T) {
	inner := &recordingObserver{}
	// Hard limit 0 means disabled.
	guard := NewBudgetGuardObserver(inner, 0, 10.0, 30.0)

	// Even a large usage should pass through.
	resp := responseWithUsage(1_000_000, 1_000_000)
	err := guard.OnProviderResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("expected no error with zero limit (disabled), got: %v", err)
	}

	calls := inner.getCalls()
	if len(calls) != 1 || calls[0] != "OnProviderResponse" {
		t.Errorf("expected [OnProviderResponse], got %v", calls)
	}
}

func TestBudgetGuardObserver_NegativeLimit(t *testing.T) {
	inner := &recordingObserver{}
	// Negative hard limit also means disabled.
	guard := NewBudgetGuardObserver(inner, -5.0, 10.0, 30.0)

	resp := responseWithUsage(1_000_000, 1_000_000)
	err := guard.OnProviderResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("expected no error with negative limit (disabled), got: %v", err)
	}

	calls := inner.getCalls()
	if len(calls) != 1 || calls[0] != "OnProviderResponse" {
		t.Errorf("expected [OnProviderResponse], got %v", calls)
	}
}

func TestBudgetGuardObserver_DelegatesToInner(t *testing.T) {
	inner := &recordingObserver{}
	guard := NewBudgetGuardObserver(inner, 100.0, 10.0, 30.0)
	ctx := context.Background()

	// Call every method and verify delegation.
	if err := guard.OnStepStart(ctx, 1); err != nil {
		t.Fatalf("OnStepStart: %v", err)
	}
	if err := guard.OnProviderCall(ctx, provider.Request{}); err != nil {
		t.Fatalf("OnProviderCall: %v", err)
	}
	if err := guard.OnProviderOutput(ctx, provider.Request{}, provider.StreamDelta{}); err != nil {
		t.Fatalf("OnProviderOutput: %v", err)
	}
	if err := guard.OnProviderResponse(ctx, provider.Response{}); err != nil {
		t.Fatalf("OnProviderResponse: %v", err)
	}
	if err := guard.OnToolExecution(ctx, engine.ToolExecutionRecord{}); err != nil {
		t.Fatalf("OnToolExecution: %v", err)
	}
	if err := guard.OnStepEnd(ctx, 1); err != nil {
		t.Fatalf("OnStepEnd: %v", err)
	}
	if err := guard.OnRunComplete(ctx, engine.Result{}); err != nil {
		t.Fatalf("OnRunComplete: %v", err)
	}

	expected := []string{
		"OnStepStart",
		"OnProviderCall",
		"OnProviderOutput",
		"OnProviderResponse",
		"OnToolExecution",
		"OnStepEnd",
		"OnRunComplete",
	}
	calls := inner.getCalls()
	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}
	for i, want := range expected {
		if calls[i] != want {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], want)
		}
	}
}

func TestBudgetGuardObserver_DelegatesOnRunFailure(t *testing.T) {
	inner := &recordingObserver{}
	guard := NewBudgetGuardObserver(inner, 100.0, 10.0, 30.0)

	if err := guard.OnRunFailure(context.Background(), nil); err != nil {
		t.Fatalf("OnRunFailure: %v", err)
	}

	calls := inner.getCalls()
	if len(calls) != 1 || calls[0] != "OnRunFailure" {
		t.Errorf("expected [OnRunFailure], got %v", calls)
	}
}
