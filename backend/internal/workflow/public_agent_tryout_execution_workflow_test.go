package workflow

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

// publicTryoutWorkflowProbe records the calls of the two tryout activities in
// a workflow test.
type publicTryoutWorkflowProbe struct {
	mu              sync.Mutex
	cleanupInputs   []FailPublicAgentTryoutInput
	cleanupAttempts []int32
	cleanupTimeouts []time.Duration
	executeTimeouts []time.Duration
	// cleanupErr, when set, decides the result of each cleanup attempt.
	cleanupErr func(attempt int32) error
}

func newPublicTryoutWorkflowTestEnvironment(t *testing.T, execute func(ctx context.Context) error) (*testsuite.TestWorkflowEnvironment, *publicTryoutWorkflowProbe) {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	probe := &publicTryoutWorkflowProbe{}
	env.RegisterWorkflowWithOptions(PublicAgentTryoutExecutionWorkflow, sdkworkflow.RegisterOptions{Name: PublicAgentTryoutExecutionWorkflowName})
	env.RegisterActivityWithOptions(func(ctx context.Context, _ ExecutePublicAgentTryoutInput) error {
		info := sdkactivity.GetInfo(ctx)
		probe.mu.Lock()
		probe.executeTimeouts = append(probe.executeTimeouts, info.HeartbeatTimeout)
		probe.mu.Unlock()
		return execute(ctx)
	}, sdkactivity.RegisterOptions{Name: executePublicAgentTryoutActivityName})
	env.RegisterActivityWithOptions(func(ctx context.Context, input FailPublicAgentTryoutInput) error {
		info := sdkactivity.GetInfo(ctx)
		probe.mu.Lock()
		probe.cleanupInputs = append(probe.cleanupInputs, input)
		probe.cleanupAttempts = append(probe.cleanupAttempts, info.Attempt)
		probe.cleanupTimeouts = append(probe.cleanupTimeouts, info.StartToCloseTimeout)
		cleanupErr := probe.cleanupErr
		probe.mu.Unlock()
		if cleanupErr != nil {
			return cleanupErr(info.Attempt)
		}
		return nil
	}, sdkactivity.RegisterOptions{Name: failPublicAgentTryoutActivityName})
	return env, probe
}

func (p *publicTryoutWorkflowProbe) cleanups() []FailPublicAgentTryoutInput {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]FailPublicAgentTryoutInput(nil), p.cleanupInputs...)
}

func TestPublicAgentTryoutWorkflowFailsTryoutWhenActivityFails(t *testing.T) {
	tryoutID := uuid.New()
	env, probe := newPublicTryoutWorkflowTestEnvironment(t, func(context.Context) error {
		return errors.New("sandbox host stopped")
	})

	env.ExecuteWorkflow(PublicAgentTryoutExecutionWorkflow, PublicAgentTryoutExecutionWorkflowInput{TryoutID: tryoutID})

	err := env.GetWorkflowError()
	if err == nil || !strings.Contains(err.Error(), "sandbox host stopped") {
		t.Fatalf("workflow error = %v, want the activity error", err)
	}
	cleanups := probe.cleanups()
	if len(cleanups) != 1 || cleanups[0].TryoutID != tryoutID {
		t.Fatalf("cleanup calls = %#v, want one for tryout %s", cleanups, tryoutID)
	}
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if got := probe.cleanupTimeouts[0]; got != 30*time.Second {
		t.Fatalf("cleanup StartToCloseTimeout = %s, want 30s", got)
	}
	if got := probe.executeTimeouts[0]; got != publicTryoutHeartbeatTimeout {
		t.Fatalf("execute HeartbeatTimeout = %s, want %s", got, publicTryoutHeartbeatTimeout)
	}
}

func TestPublicAgentTryoutWorkflowRetriesFailureCleanup(t *testing.T) {
	env, probe := newPublicTryoutWorkflowTestEnvironment(t, func(context.Context) error {
		return errors.New("sandbox host stopped")
	})
	probe.cleanupErr = func(attempt int32) error {
		if attempt < 3 {
			// During a deploy, an old worker refuses the task.
			return errors.New("activity is not registered")
		}
		return nil
	}

	env.ExecuteWorkflow(PublicAgentTryoutExecutionWorkflow, PublicAgentTryoutExecutionWorkflowInput{TryoutID: uuid.New()})

	if err := env.GetWorkflowError(); err == nil || !strings.Contains(err.Error(), "sandbox host stopped") {
		t.Fatalf("workflow error = %v, want the activity error", err)
	}
	probe.mu.Lock()
	attempts := append([]int32(nil), probe.cleanupAttempts...)
	probe.mu.Unlock()
	if !slices.Equal(attempts, []int32{1, 2, 3}) {
		t.Fatalf("cleanup attempts = %v, want [1 2 3]", attempts)
	}
}

func TestPublicAgentTryoutWorkflowReturnsActivityErrorWhenCleanupFails(t *testing.T) {
	env, probe := newPublicTryoutWorkflowTestEnvironment(t, func(context.Context) error {
		return errors.New("sandbox host stopped")
	})
	probe.cleanupErr = func(int32) error {
		return temporal.NewNonRetryableApplicationError("database unavailable", "test", nil)
	}

	env.ExecuteWorkflow(PublicAgentTryoutExecutionWorkflow, PublicAgentTryoutExecutionWorkflowInput{TryoutID: uuid.New()})

	err := env.GetWorkflowError()
	if err == nil || !strings.Contains(err.Error(), "sandbox host stopped") {
		t.Fatalf("workflow error = %v, want the first activity error", err)
	}
}

func TestPublicAgentTryoutWorkflowDefaultVersionSkipsCleanup(t *testing.T) {
	env, probe := newPublicTryoutWorkflowTestEnvironment(t, func(context.Context) error {
		return errors.New("sandbox host stopped")
	})
	env.OnGetVersion(publicTryoutFailOnActivityErrorVersionChangeID, sdkworkflow.DefaultVersion, sdkworkflow.Version(1)).
		Return(sdkworkflow.DefaultVersion)

	env.ExecuteWorkflow(PublicAgentTryoutExecutionWorkflow, PublicAgentTryoutExecutionWorkflowInput{TryoutID: uuid.New()})

	if err := env.GetWorkflowError(); err == nil || !strings.Contains(err.Error(), "sandbox host stopped") {
		t.Fatalf("workflow error = %v, want the activity error", err)
	}
	if cleanups := probe.cleanups(); len(cleanups) != 0 {
		t.Fatalf("cleanup calls = %#v, want none for the old history", cleanups)
	}
}

func TestPublicAgentTryoutWorkflowCancelSkipsCleanup(t *testing.T) {
	env, probe := newPublicTryoutWorkflowTestEnvironment(t, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	env.RegisterDelayedCallback(func() {
		env.CancelWorkflow()
	}, time.Second)

	env.ExecuteWorkflow(PublicAgentTryoutExecutionWorkflow, PublicAgentTryoutExecutionWorkflowInput{TryoutID: uuid.New()})

	if err := env.GetWorkflowError(); !temporal.IsCanceledError(err) {
		t.Fatalf("workflow error = %v, want canceled error", err)
	}
	if cleanups := probe.cleanups(); len(cleanups) != 0 {
		t.Fatalf("cleanup calls = %#v, want none after a cancel", cleanups)
	}
}

func TestPublicAgentTryoutWorkflowSuccessSkipsCleanup(t *testing.T) {
	env, probe := newPublicTryoutWorkflowTestEnvironment(t, func(context.Context) error {
		return nil
	})

	env.ExecuteWorkflow(PublicAgentTryoutExecutionWorkflow, PublicAgentTryoutExecutionWorkflowInput{TryoutID: uuid.New()})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error = %v, want nil", err)
	}
	if cleanups := probe.cleanups(); len(cleanups) != 0 {
		t.Fatalf("cleanup calls = %#v, want none after a success", cleanups)
	}
}

// registrationRecorder records the names that a Register function uses.
type registrationRecorder struct {
	workflows  []string
	activities []string
}

func (r *registrationRecorder) RegisterWorkflowWithOptions(_ interface{}, options sdkworkflow.RegisterOptions) {
	r.workflows = append(r.workflows, options.Name)
}

func (r *registrationRecorder) RegisterActivityWithOptions(_ interface{}, options sdkactivity.RegisterOptions) {
	r.activities = append(r.activities, options.Name)
}

func TestPublicTryoutCleanupActivityIsRegisteredWithItsWorkflow(t *testing.T) {
	for _, queue := range []string{TaskQueueBackground, LegacyTaskQueue} {
		t.Run(queue, func(t *testing.T) {
			recorder := &registrationRecorder{}
			RegisterForTaskQueue(recorder, &Activities{}, queue)
			if !slices.Contains(recorder.workflows, PublicAgentTryoutExecutionWorkflowName) {
				t.Fatalf("workflows = %v, want %s", recorder.workflows, PublicAgentTryoutExecutionWorkflowName)
			}
			if !slices.Contains(recorder.activities, failPublicAgentTryoutActivityName) {
				t.Fatalf("activities = %v, want %s", recorder.activities, failPublicAgentTryoutActivityName)
			}
		})
	}
}

func TestPublicTryoutHeartbeatReachesTemporal(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var mu sync.Mutex
	beats := 0
	env.SetOnActivityHeartbeatListener(func(*sdkactivity.Info, converter.EncodedValues) {
		mu.Lock()
		beats++
		mu.Unlock()
	})
	env.RegisterActivityWithOptions(func(ctx context.Context) error {
		stop := startPublicTryoutHeartbeat(ctx, time.Millisecond, func() { recordPublicTryoutHeartbeat(ctx) })
		defer stop()
		time.Sleep(50 * time.Millisecond)
		return nil
	}, sdkactivity.RegisterOptions{Name: "workflow.test_public_tryout_heartbeat"})
	env.RegisterWorkflowWithOptions(func(ctx sdkworkflow.Context) error {
		ctx = sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{StartToCloseTimeout: time.Minute})
		return sdkworkflow.ExecuteActivity(ctx, "workflow.test_public_tryout_heartbeat").Get(ctx, nil)
	}, sdkworkflow.RegisterOptions{Name: "test-public-tryout-heartbeat"})

	env.ExecuteWorkflow("test-public-tryout-heartbeat")

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if beats == 0 {
		t.Fatal("heartbeats = 0, want at least one")
	}
}
