package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/hostedruns"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

func TestEvalSessionWorkflowHappyPath(t *testing.T) {
	sessionID := uuid.New()
	firstRunID := uuid.New()
	secondRunID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setEvalSession(
		fixtureEvalSession(sessionID, domain.EvalSessionStatusQueued),
		fixtureChildRun(firstRunID, sessionID),
		fixtureChildRun(secondRunID, sessionID),
	)

	var started []uuid.UUID
	var startedMu sync.Mutex
	env := newEvalSessionWorkflowTestEnvironment(repo, nil, func(ctx sdkworkflow.Context, input RunWorkflowInput) error {
		startedMu.Lock()
		started = append(started, input.RunID)
		startedMu.Unlock()
		return nil
	})
	env.ExecuteWorkflow(EvalSessionWorkflow, EvalSessionWorkflowInput{EvalSessionID: sessionID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("EvalSessionWorkflow returned error: %v", err)
	}
	if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusCompleted {
		t.Fatalf("eval session status = %s, want %s", got, domain.EvalSessionStatusCompleted)
	}
	if got := repo.evalSessionStatusSequence(sessionID); !equalEvalSessionStatuses(got, []domain.EvalSessionStatus{
		domain.EvalSessionStatusRunning,
		domain.EvalSessionStatusAggregating,
		domain.EvalSessionStatusCompleted,
	}) {
		t.Fatalf("eval session statuses = %v, want [running aggregating completed]", got)
	}
	if repo.callCountWithPrefix("AggregateEvalSession:") != 1 {
		t.Fatalf("AggregateEvalSession call count = %d, want 1", repo.callCountWithPrefix("AggregateEvalSession:"))
	}
	sort.Slice(started, func(i, j int) bool { return started[i].String() < started[j].String() })
	wantStarted := []uuid.UUID{firstRunID, secondRunID}
	sort.Slice(wantStarted, func(i, j int) bool { return wantStarted[i].String() < wantStarted[j].String() })
	if fmt.Sprint(started) != fmt.Sprint(wantStarted) {
		t.Fatalf("started child runs = %v, want %v", started, wantStarted)
	}
}

func TestEvalSessionWorkflowPartialChildFailureStillAggregates(t *testing.T) {
	sessionID := uuid.New()
	firstRunID := uuid.New()
	secondRunID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setEvalSession(
		fixtureEvalSession(sessionID, domain.EvalSessionStatusQueued),
		fixtureChildRun(firstRunID, sessionID),
		fixtureChildRun(secondRunID, sessionID),
	)

	env := newEvalSessionWorkflowTestEnvironment(repo, map[uuid.UUID]error{
		secondRunID: errors.New("child failed"),
	}, nil)
	env.ExecuteWorkflow(EvalSessionWorkflow, EvalSessionWorkflowInput{EvalSessionID: sessionID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("EvalSessionWorkflow returned error: %v", err)
	}
	if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusCompleted {
		t.Fatalf("eval session status = %s, want %s", got, domain.EvalSessionStatusCompleted)
	}
	if got := repo.evalSessionStatusSequence(sessionID); !equalEvalSessionStatuses(got, []domain.EvalSessionStatus{
		domain.EvalSessionStatusRunning,
		domain.EvalSessionStatusAggregating,
		domain.EvalSessionStatusCompleted,
	}) {
		t.Fatalf("eval session statuses = %v, want [running aggregating completed]", got)
	}
}

func TestEvalSessionWorkflowAggregationFailureMarksSessionFailed(t *testing.T) {
	sessionID := uuid.New()
	runID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setEvalSession(
		fixtureEvalSession(sessionID, domain.EvalSessionStatusQueued),
		fixtureChildRun(runID, sessionID),
	)
	repo.aggregateEvalSessionErr = repository.ErrEvalSessionAggregateUnavailable

	env := newEvalSessionWorkflowTestEnvironment(repo, nil, nil)
	env.ExecuteWorkflow(EvalSessionWorkflow, EvalSessionWorkflowInput{EvalSessionID: sessionID})

	if err := env.GetWorkflowError(); err == nil {
		t.Fatalf("expected workflow failure")
	}
	if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusFailed {
		t.Fatalf("eval session status = %s, want %s", got, domain.EvalSessionStatusFailed)
	}
	if got := repo.evalSessionStatusSequence(sessionID); !equalEvalSessionStatuses(got, []domain.EvalSessionStatus{
		domain.EvalSessionStatusRunning,
		domain.EvalSessionStatusAggregating,
		domain.EvalSessionStatusFailed,
	}) {
		t.Fatalf("eval session statuses = %v, want [running aggregating failed]", got)
	}
}

func TestEvalSessionWorkflowAllChildrenFailMarksSessionFailed(t *testing.T) {
	sessionID := uuid.New()
	firstRunID := uuid.New()
	secondRunID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setEvalSession(
		fixtureEvalSession(sessionID, domain.EvalSessionStatusQueued),
		fixtureChildRun(firstRunID, sessionID),
		fixtureChildRun(secondRunID, sessionID),
	)

	env := newEvalSessionWorkflowTestEnvironment(repo, map[uuid.UUID]error{
		firstRunID:  errors.New("first child failed"),
		secondRunID: errors.New("second child failed"),
	}, nil)
	env.ExecuteWorkflow(EvalSessionWorkflow, EvalSessionWorkflowInput{EvalSessionID: sessionID})

	if err := env.GetWorkflowError(); err == nil {
		t.Fatalf("expected workflow failure")
	}
	if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusFailed {
		t.Fatalf("eval session status = %s, want %s", got, domain.EvalSessionStatusFailed)
	}
	if got := repo.evalSessionStatusSequence(sessionID); !equalEvalSessionStatuses(got, []domain.EvalSessionStatus{
		domain.EvalSessionStatusRunning,
		domain.EvalSessionStatusFailed,
	}) {
		t.Fatalf("eval session statuses = %v, want [running failed]", got)
	}
}

func TestEvalSessionWorkflowRunTransitionConflictStillMarksSessionFailed(t *testing.T) {
	sessionID := uuid.New()
	firstRunID := uuid.New()
	secondRunID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setEvalSession(
		fixtureEvalSession(sessionID, domain.EvalSessionStatusQueued),
		fixtureChildRun(firstRunID, sessionID),
		fixtureChildRun(secondRunID, sessionID),
	)

	conflictErr := temporal.NewNonRetryableApplicationError(
		"child run transition conflict",
		repositoryTransitionConflictType,
		errors.New("conflict"),
	)
	env := newEvalSessionWorkflowTestEnvironment(repo, map[uuid.UUID]error{
		firstRunID:  conflictErr,
		secondRunID: conflictErr,
	}, nil)
	env.ExecuteWorkflow(EvalSessionWorkflow, EvalSessionWorkflowInput{EvalSessionID: sessionID})

	if err := env.GetWorkflowError(); err == nil {
		t.Fatalf("expected workflow failure")
	}
	if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusFailed {
		t.Fatalf("eval session status = %s, want %s", got, domain.EvalSessionStatusFailed)
	}
}

func TestEvalSessionWorkflowCancellationMarksSessionCancelled(t *testing.T) {
	sessionID := uuid.New()
	runID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setEvalSession(
		fixtureEvalSession(sessionID, domain.EvalSessionStatusQueued),
		fixtureChildRun(runID, sessionID),
	)

	env := newEvalSessionWorkflowTestEnvironment(repo, nil, func(ctx sdkworkflow.Context, input RunWorkflowInput) error {
		return sdkworkflow.Await(ctx, func() bool { return false })
	})
	env.RegisterDelayedCallback(func() {
		env.CancelWorkflow()
	}, fakeStageDelay/2)
	env.ExecuteWorkflow(EvalSessionWorkflow, EvalSessionWorkflowInput{EvalSessionID: sessionID})

	err := env.GetWorkflowError()
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
	if !temporal.IsCanceledError(err) {
		t.Fatalf("workflow error = %v, want canceled error", err)
	}
	if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusCancelled {
		t.Fatalf("eval session status = %s, want %s", got, domain.EvalSessionStatusCancelled)
	}
}

func TestEvalSessionWorkflowRequiresQueuedSession(t *testing.T) {
	sessionID := uuid.New()
	runID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setEvalSession(
		fixtureEvalSession(sessionID, domain.EvalSessionStatusRunning),
		fixtureChildRun(runID, sessionID),
	)

	env := newEvalSessionWorkflowTestEnvironment(repo, nil, nil)
	env.ExecuteWorkflow(EvalSessionWorkflow, EvalSessionWorkflowInput{EvalSessionID: sessionID})

	err := env.GetWorkflowError()
	if err == nil {
		t.Fatalf("expected workflow error")
	}
	if !strings.Contains(err.Error(), ErrEvalSessionMustBeQueued.Error()) {
		t.Fatalf("workflow error = %v, want ErrEvalSessionMustBeQueued", err)
	}
	if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusRunning {
		t.Fatalf("eval session status = %s, want %s", got, domain.EvalSessionStatusRunning)
	}
}

func TestEvalSessionWorkflowNoChildRunsFails(t *testing.T) {
	sessionID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setEvalSession(fixtureEvalSession(sessionID, domain.EvalSessionStatusQueued))

	env := newEvalSessionWorkflowTestEnvironment(repo, nil, nil)
	env.ExecuteWorkflow(EvalSessionWorkflow, EvalSessionWorkflowInput{EvalSessionID: sessionID})

	err := env.GetWorkflowError()
	if err == nil {
		t.Fatalf("expected workflow failure")
	}
	if !strings.Contains(err.Error(), ErrEvalSessionHasNoRuns.Error()) {
		t.Fatalf("workflow error = %v, want ErrEvalSessionHasNoRuns", err)
	}
	if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusFailed {
		t.Fatalf("eval session status = %s, want %s", got, domain.EvalSessionStatusFailed)
	}
}

func TestEvalSessionWorkflowWaitsForAllChildrenBeforeAggregating(t *testing.T) {
	sessionID := uuid.New()
	fastRunID := uuid.New()
	slowRunID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setEvalSession(
		fixtureEvalSession(sessionID, domain.EvalSessionStatusQueued),
		fixtureChildRun(fastRunID, sessionID),
		fixtureChildRun(slowRunID, sessionID),
	)

	env := newEvalSessionWorkflowTestEnvironment(repo, nil, func(ctx sdkworkflow.Context, input RunWorkflowInput) error {
		if input.RunID == slowRunID {
			return sdkworkflow.Sleep(ctx, 5*time.Second)
		}
		return nil
	})
	env.RegisterDelayedCallback(func() {
		if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusRunning {
			t.Fatalf("eval session status before slow child completes = %s, want running", got)
		}
	}, time.Second)
	env.ExecuteWorkflow(EvalSessionWorkflow, EvalSessionWorkflowInput{EvalSessionID: sessionID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("EvalSessionWorkflow returned error: %v", err)
	}
	if got := repo.currentEvalSession(sessionID).Status; got != domain.EvalSessionStatusCompleted {
		t.Fatalf("eval session status = %s, want %s", got, domain.EvalSessionStatusCompleted)
	}
}

func TestRunWorkflowHappyPath(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}
	if !env.IsWorkflowCompleted() {
		t.Fatalf("workflow did not complete")
	}

	run := repo.currentRun()
	if run.Status != domain.RunStatusCompleted {
		t.Fatalf("run status = %s, want %s", run.Status, domain.RunStatusCompleted)
	}
	if run.TemporalWorkflowID == nil || *run.TemporalWorkflowID != "test-run-workflow" {
		t.Fatalf("temporal workflow id = %v, want %q", run.TemporalWorkflowID, "test-run-workflow")
	}
	if run.TemporalRunID == nil || *run.TemporalRunID == "" {
		t.Fatalf("temporal run id was not stored")
	}

	runStatuses := repo.runStatusSequence()
	wantRunStatuses := []domain.RunStatus{
		domain.RunStatusProvisioning,
		domain.RunStatusRunning,
		domain.RunStatusScoring,
		domain.RunStatusCompleted,
	}
	if !equalRunStatuses(runStatuses, wantRunStatuses) {
		t.Fatalf("run statuses = %v, want %v", runStatuses, wantRunStatuses)
	}

	runAgent := repo.currentRunAgent(runAgentID)
	if runAgent.Status != domain.RunAgentStatusCompleted {
		t.Fatalf("run agent status = %s, want %s", runAgent.Status, domain.RunAgentStatusCompleted)
	}
	wantRunAgentStatuses := []domain.RunAgentStatus{
		domain.RunAgentStatusReady,
		domain.RunAgentStatusExecuting,
		domain.RunAgentStatusEvaluating,
		domain.RunAgentStatusCompleted,
	}
	if got := repo.runAgentStatusSequence(runAgentID); !equalRunAgentStatuses(got, wantRunAgentStatuses) {
		t.Fatalf("run-agent statuses = %v, want %v", got, wantRunAgentStatuses)
	}

	if repo.setTemporalIDsCount() != 1 {
		t.Fatalf("set temporal ids call count = %d, want 1", repo.setTemporalIDsCount())
	}
	if !repo.hasCallPrefix("TransitionRunStatus") {
		t.Fatalf("expected repository TransitionRunStatus to be used")
	}
	if !repo.hasCallPrefix("TransitionRunAgentStatus") {
		t.Fatalf("expected repository TransitionRunAgentStatus to be used")
	}
	if repo.callCountWithPrefix("BuildRunScorecard:") != 1 {
		t.Fatalf("BuildRunScorecard call count = %d, want 1", repo.callCountWithPrefix("BuildRunScorecard:"))
	}
}

func TestRunWorkflowStartsOneChildPerRunAgent(t *testing.T) {
	runID := uuid.New()
	firstRunAgentID := uuid.New()
	secondRunAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, firstRunAgentID, 0),
		fixtureRunAgent(runID, secondRunAgentID, 1),
	)

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}
	if repo.callCountWithPrefix("GetRunAgentByID:") != 2 {
		t.Fatalf("GetRunAgentByID call count = %d, want 2", repo.callCountWithPrefix("GetRunAgentByID:"))
	}
	if repo.currentRunAgent(firstRunAgentID).Status != domain.RunAgentStatusCompleted {
		t.Fatalf("first run agent did not complete")
	}
	if repo.currentRunAgent(secondRunAgentID).Status != domain.RunAgentStatusCompleted {
		t.Fatalf("second run agent did not complete")
	}
	if len(repo.evaluations) != 2 {
		t.Fatalf("evaluation count = %d, want 2", len(repo.evaluations))
	}
}

func TestRunAgentWorkflowHappyPath(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{})
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{
		RunID:      runID,
		RunAgentID: runAgentID,
	})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunAgentWorkflow returned error: %v", err)
	}

	runAgent := repo.currentRunAgent(runAgentID)
	if runAgent.Status != domain.RunAgentStatusEvaluating {
		t.Fatalf("run agent status = %s, want %s", runAgent.Status, domain.RunAgentStatusEvaluating)
	}
	if repo.callCountWithPrefix("BuildRunAgentReplay:") != 1 {
		t.Fatalf("BuildRunAgentReplay call count = %d, want 1", repo.callCountWithPrefix("BuildRunAgentReplay:"))
	}
}

func TestRunAgentWorkflowNativePathUsesProviderBoundary(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

	invoker := &fakeNativeModelInvoker{
		result: engine.Result{
			FinalOutput: "ok",
			StopReason:  engine.StopReasonCompleted,
		},
	}

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		NativeModelInvoker: invoker,
	})
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{
		RunID:      runID,
		RunAgentID: runAgentID,
	})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunAgentWorkflow returned error: %v", err)
	}
	if invoker.callCount != 1 {
		t.Fatalf("native invoker call count = %d, want 1", invoker.callCount)
	}
	if invoker.executionContext.RunAgent.ID != runAgentID {
		t.Fatalf("native invoker run agent id = %s, want %s", invoker.executionContext.RunAgent.ID, runAgentID)
	}
}

func TestRunAgentWorkflowReplayBuildFailureAfterSuccessDoesNotFailWorkflow(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))
	repo.buildReplayErr = errors.New("replay write unavailable")

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			result: engine.Result{
				FinalOutput: "ok",
				StopReason:  engine.StopReasonCompleted,
			},
		},
	})
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{
		RunID:      runID,
		RunAgentID: runAgentID,
	})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunAgentWorkflow returned error: %v", err)
	}
	if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusEvaluating {
		t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusEvaluating)
	}
	if repo.callCountWithPrefix("BuildRunAgentReplay:") != 1 {
		t.Fatalf("BuildRunAgentReplay call count = %d, want 1", repo.callCountWithPrefix("BuildRunAgentReplay:"))
	}
}

func TestRunWorkflowScoringEventFailureAfterPersistenceDoesNotFailWorkflow(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))
	repo.recordRunEventErr = errors.New("scoring event write unavailable")

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			result: engine.Result{
				FinalOutput: "ok",
				StopReason:  engine.StopReasonCompleted,
			},
		},
	})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}
	if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusCompleted {
		t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusCompleted)
	}
	if _, ok := repo.evaluations[runAgentID]; !ok {
		t.Fatalf("expected evaluation results to be persisted")
	}
	if repo.callCountWithPrefix("CreateEvaluationSpec:") != 1 {
		t.Fatalf("CreateEvaluationSpec call count = %d, want 1", repo.callCountWithPrefix("CreateEvaluationSpec:"))
	}
	if repo.currentRun().Status != domain.RunStatusCompleted {
		t.Fatalf("run status = %s, want %s", repo.currentRun().Status, domain.RunStatusCompleted)
	}
}

func TestRunWorkflowScoringCreatesPersistedEvaluationSpecWhenMissing(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			result: engine.Result{
				FinalOutput: "ok",
				StopReason:  engine.StopReasonCompleted,
			},
		},
	})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}
	if repo.callCountWithPrefix("CreateEvaluationSpec:") != 1 {
		t.Fatalf("CreateEvaluationSpec call count = %d, want 1", repo.callCountWithPrefix("CreateEvaluationSpec:"))
	}
	if _, ok := repo.evaluations[runAgentID]; !ok {
		t.Fatalf("expected evaluation results to be persisted")
	}
	if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusCompleted {
		t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusCompleted)
	}
}

func TestRunWorkflowScoringTransitionFailureDoesNotFailRun(t *testing.T) {
	runID := uuid.New()
	firstRunAgentID := uuid.New()
	secondRunAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, firstRunAgentID, 0),
		fixtureRunAgent(runID, secondRunAgentID, 1),
	)
	repo.setExecutionContext(firstRunAgentID, nativeExecutionContext(runID, firstRunAgentID))
	repo.setExecutionContext(secondRunAgentID, nativeExecutionContext(runID, secondRunAgentID))
	repo.runAgentStatusErrs[runAgentTransitionKey(firstRunAgentID, domain.RunAgentStatusCompleted)] = errors.New("db timeout")

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			result: engine.Result{
				FinalOutput: "ok",
				StopReason:  engine.StopReasonCompleted,
			},
		},
	})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}
	if got := repo.currentRun().Status; got != domain.RunStatusCompleted {
		t.Fatalf("run status = %s, want %s", got, domain.RunStatusCompleted)
	}
	if _, ok := repo.evaluations[firstRunAgentID]; !ok {
		t.Fatalf("expected first evaluation results to be persisted")
	}
	if _, ok := repo.evaluations[secondRunAgentID]; !ok {
		t.Fatalf("expected second evaluation results to be persisted")
	}
	if got := repo.currentRunAgent(firstRunAgentID).Status; got != domain.RunAgentStatusEvaluating {
		t.Fatalf("first run agent status = %s, want %s", got, domain.RunAgentStatusEvaluating)
	}
	if got := repo.currentRunAgent(secondRunAgentID).Status; got != domain.RunAgentStatusCompleted {
		t.Fatalf("second run agent status = %s, want %s", got, domain.RunAgentStatusCompleted)
	}
}

func TestRunWorkflowScoringFailureDoesNotFailRun(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	executionContext := nativeExecutionContext(runID, runAgentID)
	executionContext.ChallengePackVersion.Manifest = []byte(`{}`)
	repo.setExecutionContext(runAgentID, executionContext)

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			result: engine.Result{
				FinalOutput: "ok",
				StopReason:  engine.StopReasonCompleted,
			},
		},
	})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}
	if repo.currentRun().Status != domain.RunStatusCompleted {
		t.Fatalf("run status = %s, want %s", repo.currentRun().Status, domain.RunStatusCompleted)
	}
	if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusCompleted {
		t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusCompleted)
	}
	if _, ok := repo.evaluations[runAgentID]; ok {
		t.Fatalf("did not expect evaluation results to be persisted")
	}
}

func TestNativeModelActivityOptionsUseRuntimeStepTimeout(t *testing.T) {
	executionContext := nativeExecutionContext(uuid.New(), uuid.New())
	executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds = 42

	options := nativeModelActivityOptions(executionContext)
	want := 42*time.Second + nativeActivityBootBuffer + nativeActivityCleanupBuffer
	if options.StartToCloseTimeout != want {
		t.Fatalf("start to close timeout = %s, want %s", options.StartToCloseTimeout, want)
	}
	if options.RetryPolicy == nil || options.RetryPolicy.MaximumAttempts != 3 {
		t.Fatalf("retry policy maximum attempts = %d, want 3", options.RetryPolicy.MaximumAttempts)
	}
	if options.RetryPolicy.InitialInterval != 10*time.Second {
		t.Fatalf("retry policy initial interval = %s, want 10s", options.RetryPolicy.InitialInterval)
	}
	if options.RetryPolicy.BackoffCoefficient != 2.0 {
		t.Fatalf("retry policy backoff coefficient = %f, want 2.0", options.RetryPolicy.BackoffCoefficient)
	}
	if options.RetryPolicy.MaximumInterval != 2*time.Minute {
		t.Fatalf("retry policy maximum interval = %s, want 2m", options.RetryPolicy.MaximumInterval)
	}
	if len(options.RetryPolicy.NonRetryableErrorTypes) == 0 {
		t.Fatalf("retry policy should have non-retryable error types")
	}
}

func TestExecutePromptEvalStepFailsWhenInvokerNotConfigured(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)

	activities := NewActivities(repo, FakeWorkHooks{})

	err := activities.ExecutePromptEvalStep(context.Background(), RunAgentWorkflowInput{
		RunID:      runID,
		RunAgentID: runAgentID,
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected temporal application error, got %T", err)
	}
	if !appErr.NonRetryable() {
		t.Fatalf("application error should be non-retryable, got %v", appErr)
	}
	if appErr.Type() != "workflow.prompt_eval_invoker_missing" {
		t.Fatalf("application error type = %q, want workflow.prompt_eval_invoker_missing", appErr.Type())
	}
}

func TestExecuteNativeModelStepWrapsNonRetryableProviderFailures(t *testing.T) {
	runAgentID := uuid.New()
	executionContext := nativeExecutionContext(uuid.New(), runAgentID)
	repo := newFakeRunRepository(
		fixtureRun(executionContext.Run.ID, domain.RunStatusRunning),
		fixtureRunAgent(executionContext.Run.ID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, executionContext)

	activities := NewActivities(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			err: provider.NewFailure("openai", provider.FailureCodeAuth, "bad api key", false, nil),
		},
	})

	err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
		RunID:      executionContext.Run.ID,
		RunAgentID: runAgentID,
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected temporal application error, got %T", err)
	}
	if !appErr.NonRetryable() {
		t.Fatalf("expected non-retryable application error")
	}
	if appErr.Type() != providerFailureErrorTypePrefix+string(provider.FailureCodeAuth) {
		t.Fatalf("error type = %q, want %q", appErr.Type(), providerFailureErrorTypePrefix+string(provider.FailureCodeAuth))
	}
}

func TestRunAgentWorkflowHostedBlackBoxSuccess(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, hostedExecutionContext(runID, runAgentID))

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		HostedRunStarter: fakeHostedRunStarter{response: hostedruns.StartResponse{
			Accepted:      true,
			ExternalRunID: "ext-123",
		}},
	})
	env.RegisterDelayedCallback(func() {
		status := hostedruns.FinalStatusCompleted
		env.SignalWorkflow(HostedRunEventSignal, hostedruns.Event{
			RunAgentID:    runAgentID,
			ExternalRunID: "ext-123",
			EventType:     hostedruns.EventTypeRunFinished,
			OccurredAt:    time.Now().UTC(),
			FinalStatus:   &status,
			Output:        []byte(`{"answer":"done"}`),
		})
	}, fakeStageDelay/2)
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{
		RunID:      runID,
		RunAgentID: runAgentID,
	})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunAgentWorkflow returned error: %v", err)
	}
	if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusEvaluating {
		t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusEvaluating)
	}
}

func TestRunAgentWorkflowHostedMalformedStartResponseFails(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, hostedExecutionContext(runID, runAgentID))

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		HostedRunStarter: fakeHostedRunStarter{response: hostedruns.StartResponse{
			Accepted: true,
		}},
	})
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{
		RunID:      runID,
		RunAgentID: runAgentID,
	})

	if err := env.GetWorkflowError(); err == nil {
		t.Fatalf("expected workflow failure")
	}
	runAgent := repo.currentRunAgent(runAgentID)
	if runAgent.Status != domain.RunAgentStatusFailed {
		t.Fatalf("run agent status = %s, want failed", runAgent.Status)
	}
}

func TestRunAgentWorkflowHostedTimeoutFails(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	executionContext := hostedExecutionContext(runID, runAgentID)
	executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds = 1
	repo.setExecutionContext(runAgentID, executionContext)

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		HostedRunStarter: fakeHostedRunStarter{response: hostedruns.StartResponse{
			Accepted:      true,
			ExternalRunID: "ext-timeout",
		}},
	})
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{
		RunID:      runID,
		RunAgentID: runAgentID,
	})

	if err := env.GetWorkflowError(); err == nil {
		t.Fatalf("expected workflow failure")
	}
	runAgent := repo.currentRunAgent(runAgentID)
	if runAgent.Status != domain.RunAgentStatusFailed {
		t.Fatalf("run agent status = %s, want failed", runAgent.Status)
	}
}

func TestRunAgentWorkflowHostedMalformedEventFails(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, hostedExecutionContext(runID, runAgentID))

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		HostedRunStarter: fakeHostedRunStarter{response: hostedruns.StartResponse{
			Accepted:      true,
			ExternalRunID: "ext-bad",
		}},
	})
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(HostedRunEventSignal, hostedruns.Event{
			RunAgentID:    runAgentID,
			ExternalRunID: "wrong-external-id",
			EventType:     hostedruns.EventTypeRunFinished,
			OccurredAt:    time.Now().UTC(),
		})
	}, fakeStageDelay/2)
	env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{
		RunID:      runID,
		RunAgentID: runAgentID,
	})

	if err := env.GetWorkflowError(); err == nil {
		t.Fatalf("expected workflow failure")
	}
	if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusFailed {
		t.Fatalf("run agent status = %s, want failed", got)
	}
}

func TestRunWorkflowCancellationMarksRunCancelled(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		PrepareExecutionLane: func(ctx context.Context, input RunAgentWorkflowInput) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	env.RegisterDelayedCallback(func() {
		env.CancelWorkflow()
	}, fakeStageDelay/2)
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	err := env.GetWorkflowError()
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
	if !temporal.IsCanceledError(err) {
		t.Fatalf("workflow error = %v, want canceled error", err)
	}

	run := repo.currentRun()
	if run.Status != domain.RunStatusCancelled {
		t.Fatalf("run status = %s, want %s", run.Status, domain.RunStatusCancelled)
	}

	runAgent := repo.currentRunAgent(runAgentID)
	if runAgent.Status != domain.RunAgentStatusReady {
		t.Fatalf("run agent status after cancellation = %s, want %s", runAgent.Status, domain.RunAgentStatusReady)
	}
}

func TestRunWorkflowPartialChildFailureDoesNotCancelOtherAgents(t *testing.T) {
	runID := uuid.New()
	successAgentID := uuid.New()
	failAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, successAgentID, 0),
		fixtureRunAgent(runID, failAgentID, 1),
	)
	repo.setExecutionContext(successAgentID, nativeExecutionContext(runID, successAgentID))
	repo.setExecutionContext(failAgentID, nativeExecutionContext(runID, failAgentID))

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		NativeModelInvoker: &perAgentNativeModelInvoker{
			results: map[uuid.UUID]engine.Result{
				successAgentID: {FinalOutput: "ok", StopReason: engine.StopReasonCompleted},
			},
			errs: map[uuid.UUID]error{
				failAgentID: errors.New("simulated execution failure"),
			},
		},
	})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}
	if repo.currentRun().Status != domain.RunStatusCompleted {
		t.Fatalf("run status = %s, want completed", repo.currentRun().Status)
	}
	if repo.currentRunAgent(successAgentID).Status != domain.RunAgentStatusCompleted {
		t.Fatalf("success agent status = %s, want completed", repo.currentRunAgent(successAgentID).Status)
	}
	if repo.currentRunAgent(failAgentID).Status != domain.RunAgentStatusFailed {
		t.Fatalf("fail agent status = %s, want failed", repo.currentRunAgent(failAgentID).Status)
	}
	if _, ok := repo.evaluations[successAgentID]; !ok {
		t.Fatalf("expected success agent to have evaluation results")
	}
	if _, ok := repo.evaluations[failAgentID]; ok {
		t.Fatalf("did not expect fail agent to have evaluation results")
	}
}

func TestRunWorkflowChildFailureMarksRunAndRunAgentFailed(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			err: errors.New("simulated execution failure"),
		},
	})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	err := env.GetWorkflowError()
	if err == nil {
		t.Fatalf("expected workflow failure")
	}

	run := repo.currentRun()
	if run.Status != domain.RunStatusFailed {
		t.Fatalf("run status = %s, want %s", run.Status, domain.RunStatusFailed)
	}

	runAgent := repo.currentRunAgent(runAgentID)
	if runAgent.Status != domain.RunAgentStatusFailed {
		t.Fatalf("run agent status = %s, want %s", runAgent.Status, domain.RunAgentStatusFailed)
	}
	if runAgent.FailureReason == nil || !strings.Contains(*runAgent.FailureReason, "simulated execution failure") {
		t.Fatalf("run agent failure reason = %v, want simulated execution failure", runAgent.FailureReason)
	}
	if repo.callCountWithPrefix("BuildRunAgentReplay:") != 1 {
		t.Fatalf("BuildRunAgentReplay call count = %d, want 1", repo.callCountWithPrefix("BuildRunAgentReplay:"))
	}
}

func TestRunWorkflowChildFailureReturnsOriginalErrorWhenReplayBuildFails(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))
	repo.buildReplayErr = errors.New("replay write unavailable")

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			err: errors.New("simulated execution failure"),
		},
	})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	err := env.GetWorkflowError()
	if err == nil {
		t.Fatalf("expected workflow failure")
	}
	if !strings.Contains(err.Error(), "simulated execution failure") {
		t.Fatalf("workflow error = %v, want original execution failure", err)
	}
	if strings.Contains(err.Error(), "replay write unavailable") {
		t.Fatalf("workflow error = %v, should not be replaced by replay build failure", err)
	}
	if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusFailed {
		t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusFailed)
	}
}

func TestRunWorkflowTemporalIDConflictDoesNotRebindOrAdvanceStatus(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	existingWorkflowID := "existing-workflow"
	existingRunID := "existing-run-id"
	run := fixtureRun(runID, domain.RunStatusQueued)
	run.TemporalWorkflowID = &existingWorkflowID
	run.TemporalRunID = &existingRunID
	repo := newFakeRunRepository(
		run,
		fixtureRunAgent(runID, runAgentID, 0),
	)

	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{})
	env.ExecuteWorkflow(RunWorkflow, RunWorkflowInput{RunID: runID})

	err := env.GetWorkflowError()
	if err == nil {
		t.Fatalf("expected workflow error")
	}
	if !hasApplicationErrorType(err, repositoryTemporalIDConflictType) {
		t.Fatalf("workflow error = %v, want temporal id conflict application error", err)
	}

	persistedRun := repo.currentRun()
	if persistedRun.Status != domain.RunStatusQueued {
		t.Fatalf("run status = %s, want %s", persistedRun.Status, domain.RunStatusQueued)
	}
	if persistedRun.TemporalWorkflowID == nil || *persistedRun.TemporalWorkflowID != existingWorkflowID {
		t.Fatalf("temporal workflow id = %v, want %q", persistedRun.TemporalWorkflowID, existingWorkflowID)
	}
	if repo.runStatusTransitionCount() != 0 {
		t.Fatalf("run status transition count = %d, want 0", repo.runStatusTransitionCount())
	}
}

func newTestWorkflowEnvironment(repo *fakeRunRepository, hooks FakeWorkHooks) *testsuite.TestWorkflowEnvironment {
	var suite testsuite.WorkflowTestSuite
	suite.SetDisableRegistrationAliasing(true)

	env := suite.NewTestWorkflowEnvironment()
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID:        "test-run-workflow",
		TaskQueue: "workflow-test",
	})
	Register(env, NewActivities(repo, hooks))

	return env
}

func newEvalSessionWorkflowTestEnvironment(
	repo *fakeRunRepository,
	childErrs map[uuid.UUID]error,
	childWorkflow func(ctx sdkworkflow.Context, input RunWorkflowInput) error,
) *testsuite.TestWorkflowEnvironment {
	var suite testsuite.WorkflowTestSuite
	suite.SetDisableRegistrationAliasing(true)

	env := suite.NewTestWorkflowEnvironment()
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{
		ID:        "test-eval-session-workflow",
		TaskQueue: "workflow-test",
	})

	activities := NewActivities(repo, FakeWorkHooks{})
	env.RegisterWorkflowWithOptions(EvalSessionWorkflow, sdkworkflow.RegisterOptions{Name: EvalSessionWorkflowName})
	env.RegisterActivityWithOptions(activities.LoadEvalSession, sdkactivity.RegisterOptions{Name: loadEvalSessionActivityName})
	env.RegisterActivityWithOptions(activities.ListEvalSessionRuns, sdkactivity.RegisterOptions{Name: listEvalSessionRunsActivityName})
	env.RegisterActivityWithOptions(activities.TransitionEvalSessionStatus, sdkactivity.RegisterOptions{Name: transitionEvalSessionStatusActivityName})
	env.RegisterActivityWithOptions(activities.AggregateEvalSession, sdkactivity.RegisterOptions{Name: aggregateEvalSessionActivityName})
	if childWorkflow == nil {
		childWorkflow = func(ctx sdkworkflow.Context, input RunWorkflowInput) error {
			if childErrs != nil {
				if err := childErrs[input.RunID]; err != nil {
					return err
				}
			}
			return nil
		}
	}
	env.RegisterWorkflowWithOptions(childWorkflow, sdkworkflow.RegisterOptions{Name: RunWorkflowName})

	return env
}

type fakeRunRepository struct {
	mu                      sync.Mutex
	run                     domain.Run
	evalSessions            map[uuid.UUID]domain.EvalSession
	evalSessionRuns         map[uuid.UUID][]domain.Run
	runAgents               map[uuid.UUID]domain.RunAgent
	executionContexts       map[uuid.UUID]repository.RunAgentExecutionContext
	evaluationSpecs         map[string]repository.EvaluationSpecRecord
	hostedExecutions        map[uuid.UUID]repository.HostedRunExecution
	replays                 map[uuid.UUID]repository.RunAgentReplay
	runEvents               map[uuid.UUID][]repository.RunEvent
	evaluations             map[uuid.UUID]scoring.RunAgentEvaluation
	runScorecards           map[uuid.UUID]repository.RunScorecard
	evalSessionAggregates   map[uuid.UUID]repository.EvalSessionAggregateRecord
	runAgentStatusErrs      map[string]error
	buildReplayErr          error
	aggregateEvalSessionErr error
	recordRunEventErr       error
	callLog                 []string
	runStatusCalls          []repository.TransitionRunStatusParams
	evalSessionStatusCalls  []repository.TransitionEvalSessionStatusParams
	runAgentStatusCalls     []repository.TransitionRunAgentStatusParams
	setTemporalIDsCalls     []repository.SetRunTemporalIDsParams
	buildReplayCalls        []uuid.UUID
}

func newFakeRunRepository(run domain.Run, runAgents ...domain.RunAgent) *fakeRunRepository {
	runAgentMap := make(map[uuid.UUID]domain.RunAgent, len(runAgents))
	for _, runAgent := range runAgents {
		runAgentMap[runAgent.ID] = cloneRunAgent(runAgent)
	}

	return &fakeRunRepository{
		run:                   cloneRun(run),
		evalSessions:          make(map[uuid.UUID]domain.EvalSession),
		evalSessionRuns:       make(map[uuid.UUID][]domain.Run),
		runAgents:             runAgentMap,
		executionContexts:     make(map[uuid.UUID]repository.RunAgentExecutionContext),
		evaluationSpecs:       make(map[string]repository.EvaluationSpecRecord),
		hostedExecutions:      make(map[uuid.UUID]repository.HostedRunExecution),
		replays:               make(map[uuid.UUID]repository.RunAgentReplay),
		runEvents:             make(map[uuid.UUID][]repository.RunEvent),
		evaluations:           make(map[uuid.UUID]scoring.RunAgentEvaluation),
		runScorecards:         make(map[uuid.UUID]repository.RunScorecard),
		evalSessionAggregates: make(map[uuid.UUID]repository.EvalSessionAggregateRecord),
		runAgentStatusErrs:    make(map[string]error),
	}
}

func (r *fakeRunRepository) setEvalSession(session domain.EvalSession, runs ...domain.Run) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.evalSessions[session.ID] = cloneEvalSession(session)
	clonedRuns := make([]domain.Run, 0, len(runs))
	for _, run := range runs {
		clonedRuns = append(clonedRuns, cloneRun(run))
	}
	r.evalSessionRuns[session.ID] = clonedRuns
}

func (r *fakeRunRepository) GetEvalSessionByID(_ context.Context, id uuid.UUID) (domain.EvalSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.evalSessions[id]
	if !ok {
		return domain.EvalSession{}, repository.ErrEvalSessionNotFound
	}
	r.callLog = append(r.callLog, fmt.Sprintf("GetEvalSessionByID:%s", id))
	return cloneEvalSession(session), nil
}

func (r *fakeRunRepository) ListRunsByEvalSessionID(_ context.Context, evalSessionID uuid.UUID) ([]domain.Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callLog = append(r.callLog, fmt.Sprintf("ListRunsByEvalSessionID:%s", evalSessionID))
	runs := r.evalSessionRuns[evalSessionID]
	cloned := make([]domain.Run, 0, len(runs))
	for _, run := range runs {
		cloned = append(cloned, cloneRun(run))
	}
	return cloned, nil
}

func (r *fakeRunRepository) TransitionEvalSessionStatus(_ context.Context, params repository.TransitionEvalSessionStatusParams) (domain.EvalSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.evalSessions[params.EvalSessionID]
	if !ok {
		return domain.EvalSession{}, repository.ErrEvalSessionNotFound
	}
	if !session.Status.CanTransitionTo(params.ToStatus) {
		return domain.EvalSession{}, repository.IllegalSessionTransitionError{
			From: string(session.Status),
			To:   string(params.ToStatus),
		}
	}

	session.Status = params.ToStatus
	now := time.Now().UTC()
	if params.ToStatus == domain.EvalSessionStatusRunning {
		session.StartedAt = &now
	}
	if params.ToStatus.Terminal() {
		session.FinishedAt = &now
	}
	session.UpdatedAt = now
	r.evalSessions[params.EvalSessionID] = session
	r.evalSessionStatusCalls = append(r.evalSessionStatusCalls, params)
	r.callLog = append(r.callLog, fmt.Sprintf("TransitionEvalSessionStatus:%s:%s", params.EvalSessionID, params.ToStatus))

	return cloneEvalSession(session), nil
}

func (r *fakeRunRepository) AggregateEvalSession(_ context.Context, evalSessionID uuid.UUID) (repository.EvalSessionAggregateRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callLog = append(r.callLog, fmt.Sprintf("AggregateEvalSession:%s", evalSessionID))
	if r.aggregateEvalSessionErr != nil {
		return repository.EvalSessionAggregateRecord{}, r.aggregateEvalSessionErr
	}
	record, ok := r.evalSessionAggregates[evalSessionID]
	if !ok {
		record = repository.EvalSessionAggregateRecord{
			ID:               uuid.New(),
			EvalSessionID:    evalSessionID,
			SchemaVersion:    1,
			ChildRunCount:    int32(len(r.evalSessionRuns[evalSessionID])),
			ScoredChildCount: int32(len(r.evalSessionRuns[evalSessionID])),
			Aggregate:        []byte(`{"schema_version":1}`),
			Evidence:         []byte(`{"warnings":[]}`),
			ComputedAt:       time.Now().UTC(),
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}
		r.evalSessionAggregates[evalSessionID] = record
	}
	return record, nil
}

func (r *fakeRunRepository) GetRunByID(_ context.Context, id uuid.UUID) (domain.Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.run.ID != id {
		return domain.Run{}, repository.ErrRunNotFound
	}
	r.callLog = append(r.callLog, "GetRunByID")

	return cloneRun(r.run), nil
}

func (r *fakeRunRepository) ListRunAgentsByRunID(_ context.Context, runID uuid.UUID) ([]domain.RunAgent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callLog = append(r.callLog, "ListRunAgentsByRunID")

	runAgents := make([]domain.RunAgent, 0, len(r.runAgents))
	for _, runAgent := range r.runAgents {
		if runAgent.RunID == runID {
			runAgents = append(runAgents, cloneRunAgent(runAgent))
		}
	}
	sort.Slice(runAgents, func(i, j int) bool {
		return runAgents[i].LaneIndex < runAgents[j].LaneIndex
	})

	return runAgents, nil
}

func (r *fakeRunRepository) GetRunAgentByID(_ context.Context, id uuid.UUID) (domain.RunAgent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callLog = append(r.callLog, fmt.Sprintf("GetRunAgentByID:%s", id))

	runAgent, ok := r.runAgents[id]
	if !ok {
		return domain.RunAgent{}, repository.ErrRunAgentNotFound
	}

	return cloneRunAgent(runAgent), nil
}

func (r *fakeRunRepository) GetRunAgentExecutionContextByID(_ context.Context, id uuid.UUID) (repository.RunAgentExecutionContext, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	runAgent, ok := r.runAgents[id]
	if !ok {
		return repository.RunAgentExecutionContext{}, repository.ErrRunAgentNotFound
	}
	if executionContext, ok := r.executionContexts[id]; ok {
		return executionContext, nil
	}

	executionContext := repository.RunAgentExecutionContext{
		Run:      cloneRun(r.run),
		RunAgent: cloneRunAgent(runAgent),
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			ID:       uuid.New(),
			Manifest: fixtureEvaluationManifest(),
		},
		Deployment: repository.AgentDeploymentExecutionContext{
			AgentDeploymentID:         runAgent.AgentDeploymentID,
			AgentDeploymentSnapshotID: runAgent.AgentDeploymentSnapshotID,
			DeploymentType:            "native",
			SnapshotConfig:            []byte(`{"mode":"test"}`),
			RuntimeProfile: repository.RuntimeProfileExecutionContext{
				ExecutionTarget:   "native",
				RunTimeoutSeconds: 5,
			},
		},
	}
	r.executionContexts[id] = executionContext
	return executionContext, nil
}

func (r *fakeRunRepository) LoadWorkspaceSecrets(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *fakeRunRepository) BuildRunAgentReplay(_ context.Context, runAgentID uuid.UUID) (repository.RunAgentReplay, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.runAgents[runAgentID]; !ok {
		return repository.RunAgentReplay{}, repository.ErrRunAgentNotFound
	}

	r.callLog = append(r.callLog, fmt.Sprintf("BuildRunAgentReplay:%s", runAgentID))
	r.buildReplayCalls = append(r.buildReplayCalls, runAgentID)
	if r.buildReplayErr != nil {
		return repository.RunAgentReplay{}, r.buildReplayErr
	}

	replay, ok := r.replays[runAgentID]
	if !ok {
		replay = repository.RunAgentReplay{
			ID:         uuid.New(),
			RunAgentID: runAgentID,
			Summary:    []byte(`{"headline":"ready"}`),
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
	} else {
		replay.UpdatedAt = time.Now().UTC()
	}
	r.replays[runAgentID] = replay

	return replay, nil
}

func (r *fakeRunRepository) CreateEvaluationSpec(_ context.Context, params repository.CreateEvaluationSpecParams) (repository.EvaluationSpecRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := evaluationSpecKey(params.ChallengePackVersionID, params.Name, params.VersionNumber)
	if existing, ok := r.evaluationSpecs[key]; ok {
		return existing, fmt.Errorf("create evaluation spec: duplicate key")
	}

	record := repository.EvaluationSpecRecord{
		ID:                     uuid.New(),
		ChallengePackVersionID: params.ChallengePackVersionID,
		Name:                   params.Name,
		VersionNumber:          params.VersionNumber,
		JudgeMode:              params.JudgeMode,
		Definition:             append([]byte(nil), params.Definition...),
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}
	r.evaluationSpecs[key] = record
	r.callLog = append(r.callLog, fmt.Sprintf("CreateEvaluationSpec:%s:%d", params.Name, params.VersionNumber))
	return record, nil
}

func (r *fakeRunRepository) GetEvaluationSpecByChallengePackVersionAndVersion(_ context.Context, challengePackVersionID uuid.UUID, name string, versionNumber int32) (repository.EvaluationSpecRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := evaluationSpecKey(challengePackVersionID, name, versionNumber)
	if record, ok := r.evaluationSpecs[key]; ok {
		return record, nil
	}

	return repository.EvaluationSpecRecord{}, repository.ErrEvaluationSpecNotFound
}

func (r *fakeRunRepository) ListRunEventsByRunAgentID(_ context.Context, runAgentID uuid.UUID) ([]repository.RunEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	events := r.runEvents[runAgentID]
	cloned := make([]repository.RunEvent, 0, len(events))
	for _, event := range events {
		cloned = append(cloned, event)
	}
	return cloned, nil
}

func (r *fakeRunRepository) RecordRunEvent(_ context.Context, params repository.RecordRunEventParams) (repository.RunEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recordRunEventErr != nil {
		return repository.RunEvent{}, r.recordRunEventErr
	}

	sequenceNumber := int64(len(r.runEvents[params.Event.RunAgentID]) + 1)
	event := repository.RunEvent{
		ID:             sequenceNumber,
		RunID:          params.Event.RunID,
		RunAgentID:     params.Event.RunAgentID,
		SequenceNumber: sequenceNumber,
		EventType:      params.Event.EventType,
		Source:         params.Event.Source,
		OccurredAt:     params.Event.OccurredAt.UTC(),
		Payload:        append([]byte(nil), params.Event.Payload...),
	}
	r.runEvents[params.Event.RunAgentID] = append(r.runEvents[params.Event.RunAgentID], event)
	r.callLog = append(r.callLog, fmt.Sprintf("RecordRunEvent:%s", params.Event.EventType))
	return event, nil
}

func (r *fakeRunRepository) StoreRunAgentEvaluationResults(_ context.Context, evaluation scoring.RunAgentEvaluation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.evaluations[evaluation.RunAgentID] = evaluation
	r.callLog = append(r.callLog, fmt.Sprintf("StoreRunAgentEvaluationResults:%s", evaluation.RunAgentID))
	return nil
}

func (r *fakeRunRepository) BuildRunScorecard(_ context.Context, runID uuid.UUID) (repository.RunScorecard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	scorecard := repository.RunScorecard{
		ID:               uuid.New(),
		RunID:            runID,
		EvaluationSpecID: uuid.New(),
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	r.runScorecards[runID] = scorecard
	r.callLog = append(r.callLog, fmt.Sprintf("BuildRunScorecard:%s", runID))
	return scorecard, nil
}

func (r *fakeRunRepository) setExecutionContext(runAgentID uuid.UUID, executionContext repository.RunAgentExecutionContext) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executionContexts[runAgentID] = executionContext
}

func evaluationSpecKey(challengePackVersionID uuid.UUID, name string, versionNumber int32) string {
	return fmt.Sprintf("%s/%s/%d", challengePackVersionID, name, versionNumber)
}

func runAgentTransitionKey(runAgentID uuid.UUID, toStatus domain.RunAgentStatus) string {
	return fmt.Sprintf("%s/%s", runAgentID, toStatus)
}

func (r *fakeRunRepository) SetRunTemporalIDs(_ context.Context, params repository.SetRunTemporalIDsParams) (domain.Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callLog = append(r.callLog, fmt.Sprintf("SetRunTemporalIDs:%s", params.RunID))
	r.setTemporalIDsCalls = append(r.setTemporalIDsCalls, params)

	if r.run.ID != params.RunID {
		return domain.Run{}, repository.ErrRunNotFound
	}
	if r.run.TemporalWorkflowID != nil || r.run.TemporalRunID != nil {
		if equalStringPtrs(r.run.TemporalWorkflowID, &params.TemporalWorkflowID) &&
			equalStringPtrs(r.run.TemporalRunID, &params.TemporalRunID) {
			return cloneRun(r.run), nil
		}

		return domain.Run{}, repository.TemporalIDConflictError{
			RunID:                params.RunID,
			ExistingWorkflowID:   cloneStringPtr(r.run.TemporalWorkflowID),
			ExistingTemporalRun:  cloneStringPtr(r.run.TemporalRunID),
			RequestedWorkflowID:  params.TemporalWorkflowID,
			RequestedTemporalRun: params.TemporalRunID,
		}
	}

	r.run.TemporalWorkflowID = &params.TemporalWorkflowID
	r.run.TemporalRunID = &params.TemporalRunID

	return cloneRun(r.run), nil
}

func (r *fakeRunRepository) TransitionRunStatus(_ context.Context, params repository.TransitionRunStatusParams) (domain.Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callLog = append(r.callLog, fmt.Sprintf("TransitionRunStatus:%s", params.ToStatus))

	if r.run.ID != params.RunID {
		return domain.Run{}, repository.ErrRunNotFound
	}
	if !r.run.Status.CanTransitionTo(params.ToStatus) {
		return domain.Run{}, repository.InvalidTransitionError{
			Entity: "run",
			From:   string(r.run.Status),
			To:     string(params.ToStatus),
		}
	}

	r.run.Status = params.ToStatus
	now := time.Now().UTC()
	switch params.ToStatus {
	case domain.RunStatusProvisioning:
		if r.run.StartedAt == nil {
			r.run.StartedAt = &now
		}
	case domain.RunStatusCompleted:
		if r.run.FinishedAt == nil {
			r.run.FinishedAt = &now
		}
	case domain.RunStatusFailed:
		if r.run.FailedAt == nil {
			r.run.FailedAt = &now
		}
		if r.run.FinishedAt == nil {
			r.run.FinishedAt = &now
		}
	case domain.RunStatusCancelled:
		if r.run.CancelledAt == nil {
			r.run.CancelledAt = &now
		}
		if r.run.FinishedAt == nil {
			r.run.FinishedAt = &now
		}
	}
	r.runStatusCalls = append(r.runStatusCalls, params)

	return cloneRun(r.run), nil
}

func (r *fakeRunRepository) TransitionRunAgentStatus(_ context.Context, params repository.TransitionRunAgentStatusParams) (domain.RunAgent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callLog = append(r.callLog, fmt.Sprintf("TransitionRunAgentStatus:%s:%s", params.RunAgentID, params.ToStatus))

	runAgent, ok := r.runAgents[params.RunAgentID]
	if !ok {
		return domain.RunAgent{}, repository.ErrRunAgentNotFound
	}
	if err, ok := r.runAgentStatusErrs[runAgentTransitionKey(params.RunAgentID, params.ToStatus)]; ok {
		return domain.RunAgent{}, err
	}
	if !runAgent.Status.CanTransitionTo(params.ToStatus) {
		return domain.RunAgent{}, repository.InvalidTransitionError{
			Entity: "run_agent",
			From:   string(runAgent.Status),
			To:     string(params.ToStatus),
		}
	}

	runAgent.Status = params.ToStatus
	if params.ToStatus == domain.RunAgentStatusFailed && params.FailureReason != nil {
		reason := *params.FailureReason
		runAgent.FailureReason = &reason
	}
	r.runAgents[params.RunAgentID] = runAgent
	r.runAgentStatusCalls = append(r.runAgentStatusCalls, params)

	return cloneRunAgent(runAgent), nil
}

func (r *fakeRunRepository) CreateHostedRunExecution(_ context.Context, params repository.CreateHostedRunExecutionParams) (repository.HostedRunExecution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	execution := repository.HostedRunExecution{
		ID:          uuid.New(),
		RunID:       params.RunID,
		RunAgentID:  params.RunAgentID,
		EndpointURL: params.EndpointURL,
		TraceLevel:  params.TraceLevel,
		Status:      "starting",
		DeadlineAt:  params.DeadlineAt,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	r.hostedExecutions[params.RunAgentID] = execution
	return execution, nil
}

func (r *fakeRunRepository) MarkHostedRunExecutionAccepted(_ context.Context, params repository.MarkHostedRunExecutionAcceptedParams) (repository.HostedRunExecution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	execution, ok := r.hostedExecutions[params.RunAgentID]
	if !ok {
		return repository.HostedRunExecution{}, repository.ErrHostedRunExecutionNotFound
	}
	execution.Status = "accepted"
	execution.ExternalRunID = &params.ExternalRunID
	execution.AcceptedResponse = append([]byte(nil), params.AcceptedResponse...)
	r.hostedExecutions[params.RunAgentID] = execution
	return execution, nil
}

func (r *fakeRunRepository) MarkHostedRunExecutionFailed(_ context.Context, params repository.MarkHostedRunExecutionFailedParams) (repository.HostedRunExecution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	execution, ok := r.hostedExecutions[params.RunAgentID]
	if !ok {
		return repository.HostedRunExecution{}, repository.ErrHostedRunExecutionNotFound
	}
	execution.Status = "failed"
	execution.ErrorMessage = stringPtr(params.ErrorMessage)
	r.hostedExecutions[params.RunAgentID] = execution
	return execution, nil
}

func (r *fakeRunRepository) MarkHostedRunExecutionTimedOut(_ context.Context, params repository.MarkHostedRunExecutionTimedOutParams) (repository.HostedRunExecution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	execution, ok := r.hostedExecutions[params.RunAgentID]
	if !ok {
		return repository.HostedRunExecution{}, repository.ErrHostedRunExecutionNotFound
	}
	execution.Status = "timed_out"
	execution.ErrorMessage = stringPtr(params.ErrorMessage)
	r.hostedExecutions[params.RunAgentID] = execution
	return execution, nil
}

func (r *fakeRunRepository) currentRun() domain.Run {
	r.mu.Lock()
	defer r.mu.Unlock()

	return cloneRun(r.run)
}

func (r *fakeRunRepository) currentEvalSession(id uuid.UUID) domain.EvalSession {
	r.mu.Lock()
	defer r.mu.Unlock()

	return cloneEvalSession(r.evalSessions[id])
}

func (r *fakeRunRepository) currentRunAgent(id uuid.UUID) domain.RunAgent {
	r.mu.Lock()
	defer r.mu.Unlock()

	return cloneRunAgent(r.runAgents[id])
}

func (r *fakeRunRepository) runStatusSequence() []domain.RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	statuses := make([]domain.RunStatus, 0, len(r.runStatusCalls))
	for _, call := range r.runStatusCalls {
		statuses = append(statuses, call.ToStatus)
	}

	return statuses
}

func (r *fakeRunRepository) runAgentStatusSequence(runAgentID uuid.UUID) []domain.RunAgentStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	statuses := make([]domain.RunAgentStatus, 0, len(r.runAgentStatusCalls))
	for _, call := range r.runAgentStatusCalls {
		if call.RunAgentID == runAgentID {
			statuses = append(statuses, call.ToStatus)
		}
	}

	return statuses
}

func (r *fakeRunRepository) runStatusTransitionCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.runStatusCalls)
}

func (r *fakeRunRepository) evalSessionStatusSequence(id uuid.UUID) []domain.EvalSessionStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	statuses := make([]domain.EvalSessionStatus, 0, len(r.evalSessionStatusCalls))
	for _, call := range r.evalSessionStatusCalls {
		if call.EvalSessionID == id {
			statuses = append(statuses, call.ToStatus)
		}
	}

	return statuses
}

func (r *fakeRunRepository) setTemporalIDsCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.setTemporalIDsCalls)
}

func (r *fakeRunRepository) hasCallPrefix(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, call := range r.callLog {
		if strings.HasPrefix(call, prefix) {
			return true
		}
	}

	return false
}

func (r *fakeRunRepository) callCountWithPrefix(prefix string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	var count int
	for _, call := range r.callLog {
		if strings.HasPrefix(call, prefix) {
			count++
		}
	}

	return count
}

func fixtureRun(runID uuid.UUID, status domain.RunStatus) domain.Run {
	createdAt := time.Now().UTC()

	return domain.Run{
		ID:            runID,
		Status:        status,
		Name:          "fixture-run",
		ExecutionMode: "comparison",
		ExecutionPlan: []byte(`{}`),
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}
}

func fixtureEvalSession(sessionID uuid.UUID, status domain.EvalSessionStatus) domain.EvalSession {
	now := time.Now().UTC()

	return domain.EvalSession{
		ID:            sessionID,
		Status:        status,
		Repetitions:   1,
		SchemaVersion: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func fixtureChildRun(runID uuid.UUID, sessionID uuid.UUID) domain.Run {
	run := fixtureRun(runID, domain.RunStatusQueued)
	run.EvalSessionID = &sessionID
	return run
}

func hostedExecutionContext(runID uuid.UUID, runAgentID uuid.UUID) repository.RunAgentExecutionContext {
	endpointURL := "https://example.com"
	return repository.RunAgentExecutionContext{
		Run: domain.Run{
			ID: runID,
		},
		RunAgent: domain.RunAgent{
			ID:        runAgentID,
			RunID:     runID,
			Status:    domain.RunAgentStatusQueued,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			ID:       uuid.New(),
			Manifest: fixtureEvaluationManifest(),
		},
		Deployment: repository.AgentDeploymentExecutionContext{
			AgentDeploymentID:         uuid.New(),
			AgentDeploymentSnapshotID: uuid.New(),
			DeploymentType:            "hosted_external",
			EndpointURL:               &endpointURL,
			SnapshotConfig:            []byte(`{"mode":"black_box"}`),
			RuntimeProfile: repository.RuntimeProfileExecutionContext{
				ExecutionTarget:   "hosted_external",
				RunTimeoutSeconds: 5,
			},
		},
	}
}

func nativeExecutionContext(runID uuid.UUID, runAgentID uuid.UUID) repository.RunAgentExecutionContext {
	return repository.RunAgentExecutionContext{
		Run: domain.Run{
			ID: runID,
		},
		RunAgent: domain.RunAgent{
			ID:        runAgentID,
			RunID:     runID,
			Status:    domain.RunAgentStatusQueued,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			ID:       uuid.New(),
			Manifest: fixtureEvaluationManifest(),
		},
		Deployment: repository.AgentDeploymentExecutionContext{
			AgentDeploymentID:         uuid.New(),
			AgentDeploymentSnapshotID: uuid.New(),
			DeploymentType:            "native",
			SnapshotConfig:            []byte(`{"temperature":0.1}`),
			RuntimeProfile: repository.RuntimeProfileExecutionContext{
				ExecutionTarget:    "native",
				TraceMode:          "full",
				StepTimeoutSeconds: 5,
				RunTimeoutSeconds:  5,
			},
			ProviderAccount: &repository.ProviderAccountExecutionContext{
				ID:                  uuid.New(),
				ProviderKey:         "openai",
				CredentialReference: "env://OPENAI_API_KEY",
			},
			ModelAlias: &repository.ModelAliasExecutionContext{
				ID:          uuid.New(),
				AliasKey:    "primary-model",
				DisplayName: "Primary Model",
				ModelCatalogEntry: repository.ModelCatalogEntryExecutionContext{
					ID:              uuid.New(),
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					DisplayName:     "GPT-4.1",
				},
			},
		},
	}
}

type fakeHostedRunStarter struct {
	response hostedruns.StartResponse
	err      error
}

func (f fakeHostedRunStarter) Start(context.Context, HostedRunStartInput) (hostedruns.StartResponse, error) {
	if f.err != nil {
		return hostedruns.StartResponse{}, f.err
	}
	return f.response, nil
}

func fixtureEvaluationManifest() []byte {
	return []byte(`{
		"evaluation_spec": {
			"name": "fixture-eval",
			"version_number": 1,
			"judge_mode": "deterministic",
			"validators": [
				{
					"key": "final-output-match",
					"type": "exact_match",
					"target": "final_output",
					"expected_from": "challenge_input"
				}
			],
			"metrics": [
				{
					"key": "completion",
					"type": "boolean",
					"collector": "run_completed_successfully"
				}
			],
			"scorecard": {
				"dimensions": ["correctness", "reliability"]
			}
		}
	}`)
}

type fakeNativeModelInvoker struct {
	mu               sync.Mutex
	result           engine.Result
	err              error
	callCount        int
	executionContext repository.RunAgentExecutionContext
}

func (f *fakeNativeModelInvoker) InvokeNativeModel(_ context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	f.executionContext = executionContext
	if f.err != nil {
		return engine.Result{}, f.err
	}
	return f.result, nil
}

type perAgentNativeModelInvoker struct {
	results map[uuid.UUID]engine.Result
	errs    map[uuid.UUID]error
}

func (f *perAgentNativeModelInvoker) InvokeNativeModel(_ context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error) {
	agentID := executionContext.RunAgent.ID
	if err, ok := f.errs[agentID]; ok {
		return engine.Result{}, err
	}
	if result, ok := f.results[agentID]; ok {
		return result, nil
	}
	return engine.Result{FinalOutput: "default", StopReason: engine.StopReasonCompleted}, nil
}

func fixtureRunAgent(runID uuid.UUID, runAgentID uuid.UUID, laneIndex int32) domain.RunAgent {
	createdAt := time.Now().UTC()

	return domain.RunAgent{
		ID:        runAgentID,
		RunID:     runID,
		LaneIndex: laneIndex,
		Label:     fmt.Sprintf("lane-%d", laneIndex),
		Status:    domain.RunAgentStatusQueued,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

func cloneRun(run domain.Run) domain.Run {
	cloned := run
	cloned.TemporalWorkflowID = cloneStringPtr(run.TemporalWorkflowID)
	cloned.TemporalRunID = cloneStringPtr(run.TemporalRunID)
	cloned.ExecutionPlan = append([]byte(nil), run.ExecutionPlan...)

	return cloned
}

func cloneEvalSession(session domain.EvalSession) domain.EvalSession {
	cloned := session
	cloned.AggregationConfig.Document = cloneJSON(session.AggregationConfig.Document)
	cloned.SuccessThresholdConfig.Document = cloneJSON(session.SuccessThresholdConfig.Document)
	cloned.RoutingTaskSnapshot.Document = cloneJSON(session.RoutingTaskSnapshot.Document)
	if session.StartedAt != nil {
		startedAt := *session.StartedAt
		cloned.StartedAt = &startedAt
	}
	if session.FinishedAt != nil {
		finishedAt := *session.FinishedAt
		cloned.FinishedAt = &finishedAt
	}

	return cloned
}

func cloneRunAgent(runAgent domain.RunAgent) domain.RunAgent {
	cloned := runAgent
	cloned.FailureReason = cloneStringPtr(runAgent.FailureReason)

	return cloned
}

func equalStringPtrs(left *string, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}

func equalRunStatuses(left []domain.RunStatus, right []domain.RunStatus) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}

func equalEvalSessionStatuses(left []domain.EvalSessionStatus, right []domain.EvalSessionStatus) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}

func equalRunAgentStatuses(left []domain.RunAgentStatus, right []domain.RunAgentStatus) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}

func TestDefaultActivityOptionsNoRetry(t *testing.T) {
	if defaultActivityOptions.RetryPolicy == nil {
		t.Fatalf("defaultActivityOptions should have a retry policy")
	}
	if defaultActivityOptions.RetryPolicy.MaximumAttempts != 1 {
		t.Fatalf("default maximum attempts = %d, want 1", defaultActivityOptions.RetryPolicy.MaximumAttempts)
	}
}

func TestNativeModelActivityOptionsNonRetryableTypes(t *testing.T) {
	executionContext := nativeExecutionContext(uuid.New(), uuid.New())
	options := nativeModelActivityOptions(executionContext)

	nonRetryable := options.RetryPolicy.NonRetryableErrorTypes
	expected := []string{
		repositoryRunNotFoundErrorType,
		repositoryRunAgentNotFoundErrorType,
		repositoryFrozenExecutionContextType,
		repositoryInvalidTransitionType,
		repositoryTransitionConflictType,
		engineFailureErrorTypePrefix + string(engine.StopReasonStepLimit),
		engineFailureErrorTypePrefix + string(engine.StopReasonToolLimit),
		engineFailureErrorTypePrefix + string(engine.StopReasonTimeout),
		engineFailureErrorTypePrefix + string(engine.StopReasonProviderError),
		engineFailureErrorTypePrefix + string(engine.StopReasonObserverError),
		providerFailureErrorTypePrefix + string(provider.FailureCodeAuth),
		providerFailureErrorTypePrefix + string(provider.FailureCodeInvalidRequest),
		providerFailureErrorTypePrefix + string(provider.FailureCodeUnsupportedProvider),
		providerFailureErrorTypePrefix + string(provider.FailureCodeCredentialUnavailable),
	}
	if len(nonRetryable) != len(expected) {
		t.Fatalf("non-retryable types count = %d, want %d", len(nonRetryable), len(expected))
	}
	for i, want := range expected {
		if nonRetryable[i] != want {
			t.Fatalf("non-retryable[%d] = %q, want %q", i, nonRetryable[i], want)
		}
	}
}

func TestExecuteNativeModelStepRetryableRateLimitIsRetryable(t *testing.T) {
	runAgentID := uuid.New()
	executionContext := nativeExecutionContext(uuid.New(), runAgentID)
	repo := newFakeRunRepository(
		fixtureRun(executionContext.Run.ID, domain.RunStatusRunning),
		fixtureRunAgent(executionContext.Run.ID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, executionContext)

	activities := NewActivities(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			err: provider.NewFailure("openai", provider.FailureCodeRateLimit, "rate limited", true, nil),
		},
	})

	err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
		RunID:      executionContext.Run.ID,
		RunAgentID: runAgentID,
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected temporal application error, got %T", err)
	}
	if appErr.NonRetryable() {
		t.Fatalf("rate limit error should be retryable")
	}
}

func TestExecuteNativeModelStepSandboxErrorIsRetryable(t *testing.T) {
	runAgentID := uuid.New()
	executionContext := nativeExecutionContext(uuid.New(), runAgentID)
	repo := newFakeRunRepository(
		fixtureRun(executionContext.Run.ID, domain.RunStatusRunning),
		fixtureRunAgent(executionContext.Run.ID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, executionContext)

	activities := NewActivities(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			err: engine.NewFailure(engine.StopReasonSandboxError, "sandbox creation failed", nil),
		},
	})

	err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
		RunID:      executionContext.Run.ID,
		RunAgentID: runAgentID,
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected temporal application error, got %T", err)
	}
	if appErr.NonRetryable() {
		t.Fatalf("sandbox error should be retryable")
	}
}

func TestExecuteNativeModelStepStepLimitIsNonRetryable(t *testing.T) {
	runAgentID := uuid.New()
	executionContext := nativeExecutionContext(uuid.New(), runAgentID)
	repo := newFakeRunRepository(
		fixtureRun(executionContext.Run.ID, domain.RunStatusRunning),
		fixtureRunAgent(executionContext.Run.ID, runAgentID, 0),
	)
	repo.setExecutionContext(runAgentID, executionContext)

	activities := NewActivities(repo, FakeWorkHooks{
		NativeModelInvoker: &fakeNativeModelInvoker{
			err: engine.NewFailure(engine.StopReasonStepLimit, "step limit reached", nil),
		},
	})

	err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
		RunID:      executionContext.Run.ID,
		RunAgentID: runAgentID,
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected temporal application error, got %T", err)
	}
	if !appErr.NonRetryable() {
		t.Fatalf("step limit error should be non-retryable")
	}
}
