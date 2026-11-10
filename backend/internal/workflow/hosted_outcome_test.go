package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/hostedruns"
	"github.com/google/uuid"
)

type hostedStarterFunc func(context.Context, HostedRunStartInput) (hostedruns.StartResponse, error)

func (f hostedStarterFunc) Start(ctx context.Context, input HostedRunStartInput) (hostedruns.StartResponse, error) {
	return f(ctx, input)
}

// This is the text that DoJSON gives for a 409 and for each other status
// with an unknown outcome.
const unknownStatusText = "connector returned a non-success response; execution outcome is unknown"

func (r *fakeRunRepository) hostedExecution(runAgentID uuid.UUID) repository.HostedRunExecution {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hostedExecutions[runAgentID]
}

func (r *fakeRunRepository) setHostedExecutionStatus(runAgentID uuid.UUID, status repository.HostedRunExecutionStatus, message *string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	execution := r.hostedExecutions[runAgentID]
	execution.Status = status
	execution.ErrorMessage = message
	r.hostedExecutions[runAgentID] = execution
}

// A start that was not sent, or that the remote refused, cannot have started
// the remote run. The activity marks the execution failed and returns the
// error, so the run agent does not wait for the deadline. Each other start
// failure keeps the outcome unknown and keeps the wait.
func TestStartHostedRunRejectionFailsExecution(t *testing.T) {
	for _, tc := range []struct {
		name        string
		startErr    error
		wantFailed  bool
		wantText    string
		wantSentinl error
	}{
		{name: "refused_401", startErr: &connectors.RefusedError{Status: 401}, wantFailed: true, wantText: "HTTP status 401", wantSentinl: connectors.ErrRefused},
		{name: "not_sent_revoked", startErr: &connectors.NotSentError{Err: repository.ErrConnectorUnapproved}, wantFailed: true, wantText: repository.ErrConnectorUnapproved.Error(), wantSentinl: repository.ErrConnectorUnapproved},
		{name: "unknown_409", startErr: errors.New(unknownStatusText)},
		{name: "unknown_transport", startErr: errors.New("connector outcome unknown; remote execution may have started")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runID, runAgentID := uuid.New(), uuid.New()
			repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusRunning), fixtureRunAgent(runID, runAgentID, 0))
			repo.setExecutionContext(runAgentID, hostedExecutionContext(runID, runAgentID))
			activities := NewActivities(repo, FakeWorkHooks{HostedRunStarter: fakeHostedRunStarter{err: tc.startErr}})

			result, err := activities.StartHostedRun(context.Background(), StartHostedRunInput{RunAgentID: runAgentID, TraceLevel: hostedruns.TraceLevelBlackBox})
			execution := repo.hostedExecution(runAgentID)
			if !tc.wantFailed {
				if err != nil {
					t.Fatalf("unknown outcome returned error %v; the wait must go on", err)
				}
				if execution.Status == repository.HostedRunExecutionStatusFailed || result.DeadlineAt.IsZero() {
					t.Fatalf("unknown outcome: status=%s deadline=%s", execution.Status, result.DeadlineAt)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("err=%v, want text %q", err, tc.wantText)
			}
			if !errors.Is(err, repository.ErrConnectorRejected) || !errors.Is(err, tc.wantSentinl) {
				t.Fatalf("err=%v lost its identity", err)
			}
			if execution.Status != repository.HostedRunExecutionStatusFailed || execution.ErrorMessage == nil || *execution.ErrorMessage != err.Error() {
				t.Fatalf("execution status=%s message=%v, want failed with %q", execution.Status, execution.ErrorMessage, err.Error())
			}
		})
	}
}

// A terminal callback can commit while the start request is open. A late
// refusal must not hide that result.
func TestStartHostedRunRejectionKeepsCommittedCallback(t *testing.T) {
	runID, runAgentID := uuid.New(), uuid.New()
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusRunning), fixtureRunAgent(runID, runAgentID, 0))
	repo.setExecutionContext(runAgentID, hostedExecutionContext(runID, runAgentID))
	starter := hostedStarterFunc(func(context.Context, HostedRunStartInput) (hostedruns.StartResponse, error) {
		repo.setHostedExecutionStatus(runAgentID, repository.HostedRunExecutionStatusCompleted, nil)
		return hostedruns.StartResponse{}, &connectors.RefusedError{Status: 401}
	})
	activities := NewActivities(repo, FakeWorkHooks{HostedRunStarter: starter})

	result, err := activities.StartHostedRun(context.Background(), StartHostedRunInput{RunAgentID: runAgentID, TraceLevel: hostedruns.TraceLevelBlackBox})
	if err != nil || !result.Completed {
		t.Fatalf("result=%+v err=%v, want the committed completion", result, err)
	}
	if got := repo.hostedExecution(runAgentID).Status; got != repository.HostedRunExecutionStatusCompleted {
		t.Fatalf("status=%s, want completed", got)
	}
}

// A refused start fails the run agent at once. A start with an unknown
// outcome (a 409) waits for the callback deadline.
func TestRunAgentWorkflowHostedRefusedStartFailsAtOnce(t *testing.T) {
	for _, tc := range []struct {
		name     string
		startErr error
		wantWait bool
		wantText string
	}{
		{name: "refused_401", startErr: &connectors.RefusedError{Status: 401}, wantText: "HTTP status 401"},
		{name: "unknown_409", startErr: errors.New(unknownStatusText), wantWait: true, wantText: "timed out waiting for callback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runID, runAgentID := uuid.New(), uuid.New()
			repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusRunning), fixtureRunAgent(runID, runAgentID, 0))
			executionContext := hostedExecutionContext(runID, runAgentID)
			executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds = 3600
			repo.setExecutionContext(runAgentID, executionContext)
			env := newTestWorkflowEnvironment(repo, FakeWorkHooks{HostedRunStarter: fakeHostedRunStarter{err: tc.startErr}})
			started := env.Now()

			env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{RunID: runID, RunAgentID: runAgentID})

			if err := env.GetWorkflowError(); err == nil {
				t.Fatal("expected workflow failure")
			}
			runAgent := repo.currentRunAgent(runAgentID)
			reason := ""
			if runAgent.FailureReason != nil {
				reason = *runAgent.FailureReason
			}
			if runAgent.Status != domain.RunAgentStatusFailed || !strings.Contains(reason, tc.wantText) {
				t.Fatalf("run agent status=%s reason=%q, want failed with %q", runAgent.Status, reason, tc.wantText)
			}
			waited := env.Now().Sub(started) >= time.Hour
			if waited != tc.wantWait {
				t.Fatalf("waited=%v (%s), want %v", waited, env.Now().Sub(started), tc.wantWait)
			}
		})
	}
}
