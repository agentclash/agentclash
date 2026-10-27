package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
)

func TestCaseFanoutConfig_DefaultsOff(t *testing.T) {
	cfg := parseCaseFanoutConfig(repository.RunAgentExecutionContext{})
	if cfg.Enabled {
		t.Fatalf("expected case_fanout disabled by default")
	}
	if cfg.MaxInFlight != defaultMaxInFlightCases {
		t.Fatalf("max in flight = %d, want %d", cfg.MaxInFlight, defaultMaxInFlightCases)
	}
}

func TestCaseFanoutConfig_ParsesFlags(t *testing.T) {
	ctx := repository.RunAgentExecutionContext{}
	ctx.Deployment.RuntimeProfile.ProfileConfig = json.RawMessage(`{"case_fanout":true,"max_in_flight_cases":3}`)
	cfg := parseCaseFanoutConfig(ctx)
	if !cfg.Enabled {
		t.Fatalf("expected case_fanout enabled")
	}
	if cfg.MaxInFlight != 3 {
		t.Fatalf("max in flight = %d, want 3", cfg.MaxInFlight)
	}
}

func TestNarrowExecutionContextToCase(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	base := nativeExecutionContext(runID, runAgentID)
	base.ChallengeInputSet = &repository.ChallengeInputSetExecutionContext{
		Cases: []repository.ChallengeCaseExecutionContext{
			{CaseKey: "a", ItemKey: "a", ChallengeKey: "ch", Payload: []byte(`{"n":1}`)},
			{CaseKey: "b", ItemKey: "b", ChallengeKey: "ch", Payload: []byte(`{"n":2}`)},
		},
	}
	narrowed, err := narrowExecutionContextToCase(base, "b")
	if err != nil {
		t.Fatalf("narrow: %v", err)
	}
	if len(narrowed.ChallengeInputSet.Cases) != 1 {
		t.Fatalf("cases = %d, want 1", len(narrowed.ChallengeInputSet.Cases))
	}
	if narrowed.ChallengeInputSet.Cases[0].CaseKey != "b" {
		t.Fatalf("case key = %q, want b", narrowed.ChallengeInputSet.Cases[0].CaseKey)
	}
	if narrowed.ExecutionCaseKey != "b" || base.ExecutionCaseKey != "" {
		t.Fatalf("case scope: narrowed=%q base=%q", narrowed.ExecutionCaseKey, base.ExecutionCaseKey)
	}
	if _, err := narrowExecutionContextToCase(base, "missing"); err == nil {
		t.Fatalf("expected error for missing case")
	}
}

func TestRunAgentWorkflowCaseFanoutDisabledUsesMegaActivity(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	ec := nativeExecutionContextWithCases(runID, runAgentID, 5)
	// Explicitly leave case_fanout unset / false.
	ec.Deployment.RuntimeProfile.ProfileConfig = json.RawMessage(`{"case_fanout":false}`)
	repo.setExecutionContext(runAgentID, ec)

	invoker := &trackingNativeModelInvoker{delay: 0}
	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{NativeModelInvoker: invoker})
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{RunID: runID, RunAgentID: runAgentID})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if invoker.calls.Load() != 1 {
		t.Fatalf("native invoker calls = %d, want 1 (mega-activity)", invoker.calls.Load())
	}
	for _, call := range invoker.caseKeys() {
		if call != "" && len(invoker.snapshots) > 0 {
			// Mega path passes full case set (5), not narrowed.
		}
	}
	if got := len(invoker.snapshots[0].ChallengeInputSet.Cases); got != 5 {
		t.Fatalf("mega-activity case count = %d, want 5", got)
	}
}

func TestRunAgentWorkflowCaseFanoutConcurrentActivities(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	ec := nativeExecutionContextWithCases(runID, runAgentID, 20)
	ec.Deployment.RuntimeProfile.ProfileConfig = json.RawMessage(`{"case_fanout":true,"max_in_flight_cases":4}`)
	repo.setExecutionContext(runAgentID, ec)

	invoker := &trackingNativeModelInvoker{
		delay:             30 * time.Millisecond,
		requireConcurrent: 2,
	}
	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{NativeModelInvoker: invoker})

	var (
		mu          sync.Mutex
		caseStarts  int
		maxInFlight int
		inFlight    int
	)
	env.SetOnActivityStartedListener(func(info *sdkactivity.Info, _ context.Context, _ converter.EncodedValues) {
		if info.ActivityType.Name != executeRunAgentCaseActivityName {
			return
		}
		mu.Lock()
		caseStarts++
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()
	})
	env.SetOnActivityCompletedListener(func(info *sdkactivity.Info, _ converter.EncodedValue, _ error) {
		if info.ActivityType.Name != executeRunAgentCaseActivityName {
			return
		}
		mu.Lock()
		inFlight--
		mu.Unlock()
	})

	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{RunID: runID, RunAgentID: runAgentID})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	mu.Lock()
	starts := caseStarts
	peak := maxInFlight
	mu.Unlock()

	if starts != 20 {
		t.Fatalf("case activity starts = %d, want 20", starts)
	}
	if peak < 2 {
		t.Fatalf("peak concurrent case activities = %d, want ≥2 (history listener)", peak)
	}
	if peak > 4 {
		t.Fatalf("peak concurrent case activities = %d, want ≤4", peak)
	}
	if invoker.calls.Load() != 20 {
		t.Fatalf("native invoker calls = %d, want 20", invoker.calls.Load())
	}
	if peakInvoker := invoker.peakInFlight.Load(); peakInvoker < 2 {
		t.Fatalf("peak concurrent invoker calls = %d, want ≥2", peakInvoker)
	}
	for i, snap := range invoker.snapshots {
		if snap.ChallengeInputSet == nil || len(snap.ChallengeInputSet.Cases) != 1 {
			t.Fatalf("invocation %d case count = %v, want 1", i, snap.ChallengeInputSet)
		}
	}
	if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusEvaluating {
		t.Fatalf("status = %s, want evaluating", got)
	}
}

func TestRunAgentWorkflowCaseFanoutPartialFailureCompletes(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	ec := nativeExecutionContextWithCases(runID, runAgentID, 4)
	ec.Deployment.RuntimeProfile.ProfileConfig = json.RawMessage(`{"case_fanout":true,"max_in_flight_cases":2}`)
	repo.setExecutionContext(runAgentID, ec)

	invoker := &trackingNativeModelInvoker{
		failCaseKeys: map[string]error{
			"case-1": temporal.NewNonRetryableApplicationError(
				"case timed out",
				engineFailureErrorTypePrefix+string(engine.StopReasonTimeout),
				engine.NewFailure(engine.StopReasonTimeout, "case timed out", nil),
			),
		},
	}
	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{NativeModelInvoker: invoker})
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{RunID: runID, RunAgentID: runAgentID})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow should complete despite one case failure; got %v", err)
	}
	if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusEvaluating {
		t.Fatalf("status = %s, want evaluating (partial)", got)
	}
	if invoker.calls.Load() < 4 {
		// Failed case may retry up to 3 times; successes once each.
		t.Fatalf("expected all cases attempted; calls=%d", invoker.calls.Load())
	}
}

func TestRunAgentWorkflowCaseFanoutRetryOnlyFailedCase(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	ec := nativeExecutionContextWithCases(runID, runAgentID, 3)
	ec.Deployment.RuntimeProfile.ProfileConfig = json.RawMessage(`{"case_fanout":true,"max_in_flight_cases":3}`)
	repo.setExecutionContext(runAgentID, ec)

	invoker := &trackingNativeModelInvoker{
		// Retryable failure on case-1 for the first two attempts, then succeed.
		retryableFailCounts: map[string]int{"case-1": 2},
	}
	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{NativeModelInvoker: invoker})
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{RunID: runID, RunAgentID: runAgentID})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}

	counts := invoker.callsByCase()
	if counts["case-0"] != 1 {
		t.Fatalf("case-0 calls = %d, want 1", counts["case-0"])
	}
	if counts["case-2"] != 1 {
		t.Fatalf("case-2 calls = %d, want 1", counts["case-2"])
	}
	if counts["case-1"] != 3 {
		t.Fatalf("case-1 calls = %d, want 3 (2 retries + success)", counts["case-1"])
	}
}

func TestExecuteRunAgentCase_NarrowsAndSucceeds(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	ec := nativeExecutionContextWithCases(runID, runAgentID, 2)
	repo.setExecutionContext(runAgentID, ec)

	invoker := &trackingNativeModelInvoker{}
	activities := NewActivities(repo, FakeWorkHooks{NativeModelInvoker: invoker})
	result, err := activities.ExecuteRunAgentCase(context.Background(), ExecuteRunAgentCaseInput{
		RunID: runID, RunAgentID: runAgentID, CaseKey: "case-1", CaseIndex: 1,
	})
	if err != nil {
		t.Fatalf("ExecuteRunAgentCase: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success")
	}
	if len(invoker.snapshots) != 1 || len(invoker.snapshots[0].ChallengeInputSet.Cases) != 1 {
		t.Fatalf("expected single-case context")
	}
	if invoker.snapshots[0].ChallengeInputSet.Cases[0].CaseKey != "case-1" {
		t.Fatalf("case key = %q", invoker.snapshots[0].ChallengeInputSet.Cases[0].CaseKey)
	}
}

func nativeExecutionContextWithCases(runID, runAgentID uuid.UUID, n int) repository.RunAgentExecutionContext {
	ec := nativeExecutionContext(runID, runAgentID)
	cases := make([]repository.ChallengeCaseExecutionContext, 0, n)
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("case-%d", i)
		cases = append(cases, repository.ChallengeCaseExecutionContext{
			ID:           uuid.New(),
			ChallengeKey: "challenge",
			CaseKey:      key,
			ItemKey:      key,
			Payload:      json.RawMessage(fmt.Sprintf(`{"i":%d}`, i)),
		})
	}
	ec.ChallengeInputSet = &repository.ChallengeInputSetExecutionContext{
		ID:    uuid.New(),
		Cases: cases,
	}
	ec.Deployment.RuntimeProfile.RunTimeoutSeconds = 30
	return ec
}

type trackingNativeModelInvoker struct {
	mu                  sync.Mutex
	delay               time.Duration
	requireConcurrent   int
	calls               atomic.Int64
	inFlight            atomic.Int64
	peakInFlight        atomic.Int64
	gate                chan struct{}
	gateOnce            sync.Once
	closeOnce           sync.Once
	snapshots           []repository.RunAgentExecutionContext
	failCaseKeys        map[string]error
	retryableFailCounts map[string]int // remaining retryable failures per case
	attemptCounts       map[string]int
}

func (f *trackingNativeModelInvoker) InvokeNativeModel(_ context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error) {
	f.calls.Add(1)
	cur := f.inFlight.Add(1)
	for {
		peak := f.peakInFlight.Load()
		if cur <= peak || f.peakInFlight.CompareAndSwap(peak, cur) {
			break
		}
	}
	defer f.inFlight.Add(-1)

	if f.requireConcurrent > 0 {
		f.gateOnce.Do(func() { f.gate = make(chan struct{}) })
		if int(cur) >= f.requireConcurrent {
			f.closeOnce.Do(func() { close(f.gate) })
		}
		select {
		case <-f.gate:
		case <-time.After(10 * time.Second):
			return engine.Result{}, errors.New("timed out waiting for concurrent case activities")
		}
	}

	f.mu.Lock()
	f.snapshots = append(f.snapshots, executionContext)
	caseKey := ""
	if executionContext.ChallengeInputSet != nil && len(executionContext.ChallengeInputSet.Cases) == 1 {
		caseKey = executionContext.ChallengeInputSet.Cases[0].CaseKey
	}
	if f.attemptCounts == nil {
		f.attemptCounts = map[string]int{}
	}
	f.attemptCounts[caseKey]++
	if f.retryableFailCounts != nil {
		if remaining, ok := f.retryableFailCounts[caseKey]; ok && remaining > 0 {
			f.retryableFailCounts[caseKey] = remaining - 1
			f.mu.Unlock()
			return engine.Result{}, temporal.NewApplicationError(
				"transient sandbox error",
				engineFailureErrorTypePrefix+string(engine.StopReasonSandboxError),
				errors.New("sandbox busy"),
			)
		}
	}
	if err, ok := f.failCaseKeys[caseKey]; ok {
		f.mu.Unlock()
		return engine.Result{}, err
	}
	delay := f.delay
	f.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	return engine.Result{FinalOutput: "ok-" + caseKey, StopReason: engine.StopReasonCompleted}, nil
}

func (f *trackingNativeModelInvoker) caseKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.snapshots))
	for _, snap := range f.snapshots {
		if snap.ChallengeInputSet != nil && len(snap.ChallengeInputSet.Cases) == 1 {
			keys = append(keys, snap.ChallengeInputSet.Cases[0].CaseKey)
		}
	}
	return keys
}

func (f *trackingNativeModelInvoker) callsByCase() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]int, len(f.attemptCounts))
	for k, v := range f.attemptCounts {
		out[k] = v
	}
	return out
}
