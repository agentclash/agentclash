package workflow

import (
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/hostedruns"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

// newHostedTimeoutRace starts a hosted run agent. After the start, a callback
// commits the given status, but its signal does not arrive before the
// deadline timer. This is the race between the callback and the timer.
func newHostedTimeoutRace(t *testing.T, status repository.HostedRunExecutionStatus, message *string) (*fakeRunRepository, *testsuite.TestWorkflowEnvironment, RunAgentWorkflowInput) {
	t.Helper()
	runID, runAgentID := uuid.New(), uuid.New()
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusRunning), fixtureRunAgent(runID, runAgentID, 0))
	executionContext := hostedExecutionContext(runID, runAgentID)
	executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds = 5
	repo.setExecutionContext(runAgentID, executionContext)
	env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
		HostedRunStarter: fakeHostedRunStarter{response: hostedruns.StartResponse{Accepted: true, ExternalRunID: "ext-race"}},
	})
	// The fake start writes the status, so the callback commits after it.
	env.RegisterDelayedCallback(func() {
		if got := repo.hostedExecution(runAgentID).Status; got != repository.HostedRunExecutionStatusAccepted {
			t.Errorf("callback ran before the start: status=%s", got)
		}
		repo.setHostedExecutionStatus(runAgentID, status, message)
	}, 2*time.Second)
	return repo, env, RunAgentWorkflowInput{RunID: runID, RunAgentID: runAgentID}
}

// The callback committed completed before the timer fired. The run agent
// must agree with the execution: no failure, and the run agent goes to
// evaluating, the same as on the signal path.
func TestRunAgentWorkflowHostedTimeoutHonorsCommittedCompletion(t *testing.T) {
	repo, env, input := newHostedTimeoutRace(t, repository.HostedRunExecutionStatusCompleted, nil)

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("committed completion failed the run agent: %v", err)
	}
	if got := repo.currentRunAgent(input.RunAgentID).Status; got != domain.RunAgentStatusEvaluating {
		t.Fatalf("run agent status=%s, want %s", got, domain.RunAgentStatusEvaluating)
	}
	if got := repo.hostedExecution(input.RunAgentID).Status; got != repository.HostedRunExecutionStatusCompleted {
		t.Fatalf("execution status=%s, want completed", got)
	}
}

// A history from before the change replays with DefaultVersion. It keeps the
// old behavior: the timer fails the run agent, also after a committed
// completion.
func TestRunAgentWorkflowHostedTimeoutDefaultVersionKeepsTimeout(t *testing.T) {
	repo, env, input := newHostedTimeoutRace(t, repository.HostedRunExecutionStatusCompleted, nil)
	env.OnGetVersion(hostedTimeoutHonorsCallbackVersionChangeID, sdkworkflow.DefaultVersion, sdkworkflow.Version(1)).
		Return(sdkworkflow.DefaultVersion)

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	err := env.GetWorkflowError()
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for callback") {
		t.Fatalf("workflow error=%v, want the old timeout failure", err)
	}
	if got := repo.currentRunAgent(input.RunAgentID).Status; got != domain.RunAgentStatusFailed {
		t.Fatalf("run agent status=%s, want failed", got)
	}
}

// During a deploy, an old worker can run the timeout activity. It returns no
// result, so the status is empty, and the timeout path applies.
func TestRunAgentWorkflowHostedTimeoutEmptyStatusKeepsTimeout(t *testing.T) {
	repo, env, input := newHostedTimeoutRace(t, repository.HostedRunExecutionStatusCompleted, nil)
	env.OnActivity(markHostedRunTimedOutActivityName, mock.Anything, mock.Anything).Return(MarkHostedRunTimedOutResult{}, nil).Once()

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	err := env.GetWorkflowError()
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for callback") {
		t.Fatalf("workflow error=%v, want the timeout failure", err)
	}
	if got := repo.currentRunAgent(input.RunAgentID).Status; got != domain.RunAgentStatusFailed {
		t.Fatalf("run agent status=%s, want failed", got)
	}
	env.AssertExpectations(t)
}

func TestHostedTimeoutOutcome(t *testing.T) {
	stored, empty := "remote failed", ""
	for _, tc := range []struct {
		marked MarkHostedRunTimedOutResult
		want   string
	}{
		{marked: MarkHostedRunTimedOutResult{Status: repository.HostedRunExecutionStatusCompleted}},
		{marked: MarkHostedRunTimedOutResult{Status: repository.HostedRunExecutionStatusFailed, ErrorMessage: &stored}, want: stored},
		{marked: MarkHostedRunTimedOutResult{Status: repository.HostedRunExecutionStatusFailed}, want: "hosted run failed"},
		{marked: MarkHostedRunTimedOutResult{Status: repository.HostedRunExecutionStatusFailed, ErrorMessage: &empty}, want: "hosted run failed"},
		{marked: MarkHostedRunTimedOutResult{Status: repository.HostedRunExecutionStatusUnknown}, want: "timeout"},
		{marked: MarkHostedRunTimedOutResult{Status: repository.HostedRunExecutionStatusTimedOut}, want: "timeout"},
		{marked: MarkHostedRunTimedOutResult{}, want: "timeout"},
	} {
		err := hostedTimeoutOutcome(tc.marked, "timeout")
		if tc.want == "" {
			if err != nil {
				t.Errorf("%+v: err=%v, want nil", tc.marked, err)
			}
			continue
		}
		if err == nil || err.Error() != tc.want {
			t.Errorf("%+v: err=%v, want %q", tc.marked, err, tc.want)
		}
	}
}

// The callback committed failed before the timer fired. The run agent fails
// with the stored message, not with the timeout.
func TestRunAgentWorkflowHostedTimeoutHonorsCommittedFailure(t *testing.T) {
	remoteMessage := "remote tool crashed"
	for _, tc := range []struct {
		name    string
		message *string
		want    string
	}{
		{name: "stored_message", message: &remoteMessage, want: remoteMessage},
		{name: "no_message", want: "hosted run failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, env, input := newHostedTimeoutRace(t, repository.HostedRunExecutionStatusFailed, tc.message)

			env.ExecuteWorkflow(RunAgentWorkflow, input)

			err := env.GetWorkflowError()
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "timed out") {
				t.Fatalf("workflow error=%v, want %q", err, tc.want)
			}
			runAgent := repo.currentRunAgent(input.RunAgentID)
			if runAgent.Status != domain.RunAgentStatusFailed || runAgent.FailureReason == nil || !strings.Contains(*runAgent.FailureReason, tc.want) {
				t.Fatalf("run agent status=%s reason=%v, want failed with %q", runAgent.Status, runAgent.FailureReason, tc.want)
			}
		})
	}
}
