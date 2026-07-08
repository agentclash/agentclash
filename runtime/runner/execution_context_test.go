package runner

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExecutionContextTimeoutHelpers(t *testing.T) {
	executionContext := ExecutionContext{}
	if StepTimeout(executionContext) != 0 {
		t.Fatal("empty step timeout should be zero")
	}
	if RunTimeout(executionContext) != 0 {
		t.Fatal("empty run timeout should be zero")
	}

	executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds = 7
	executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds = 11
	if StepTimeout(executionContext) != 7*time.Second {
		t.Fatalf("StepTimeout = %v; want 7s", StepTimeout(executionContext))
	}
	if RunTimeout(executionContext) != 11*time.Second {
		t.Fatalf("RunTimeout = %v; want 11s", RunTimeout(executionContext))
	}
}

func TestMaxIterationsLimitUsesExecutionPlanOverride(t *testing.T) {
	executionContext := ExecutionContext{}
	executionContext.Deployment.RuntimeProfile.MaxIterations = 9
	executionContext.Run.ExecutionPlan = json.RawMessage(`{"runtime_limits":{"max_iterations":2}}`)

	if got := MaxIterationsLimit(executionContext); got != 2 {
		t.Fatalf("MaxIterationsLimit = %d; want 2", got)
	}
}

func TestMaxIterationsLimitIgnoresInvalidExecutionPlanOverride(t *testing.T) {
	for _, raw := range []string{
		`{"runtime_limits":{"max_iterations":0}}`,
		`{"runtime_limits":{"max_iterations":1001}}`,
		`{"runtime_limits":{}}`,
		`not-json`,
	} {
		executionContext := ExecutionContext{}
		executionContext.Deployment.RuntimeProfile.MaxIterations = 9
		executionContext.Run.ExecutionPlan = json.RawMessage(raw)

		if got := MaxIterationsLimit(executionContext); got != 9 {
			t.Fatalf("MaxIterationsLimit(%s) = %d; want 9", raw, got)
		}
	}
}
