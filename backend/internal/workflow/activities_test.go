package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
)

func TestWrapActivityError(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		wantNil         bool
		wantType        string
		wantNonRetryable bool
		wantPassthrough  bool // error returned as-is, not an ApplicationError
	}{
		{
			name:    "nil returns nil",
			err:     nil,
			wantNil: true,
		},
		{
			name:            "ErrRunNotFound",
			err:             repository.ErrRunNotFound,
			wantType:        repositoryRunNotFoundErrorType,
			wantNonRetryable: true,
		},
		{
			name:            "ErrRunAgentNotFound",
			err:             repository.ErrRunAgentNotFound,
			wantType:        repositoryRunAgentNotFoundErrorType,
			wantNonRetryable: true,
		},
		{
			name:            "ErrFrozenExecutionContext",
			err:             repository.ErrFrozenExecutionContext,
			wantType:        repositoryFrozenExecutionContextType,
			wantNonRetryable: true,
		},
		{
			name:            "ErrTemporalIDConflict",
			err:             repository.ErrTemporalIDConflict,
			wantType:        repositoryTemporalIDConflictType,
			wantNonRetryable: true,
		},
		{
			name:            "ErrInvalidTransition",
			err:             repository.ErrInvalidTransition,
			wantType:        repositoryInvalidTransitionType,
			wantNonRetryable: true,
		},
		{
			name:            "ErrTransitionConflict",
			err:             repository.ErrTransitionConflict,
			wantType:        repositoryTransitionConflictType,
			wantNonRetryable: true,
		},
		{
			name:            "engine failure timeout is non-retryable",
			err:             engine.NewFailure(engine.StopReasonTimeout, "timed out", nil),
			wantType:        "engine.timeout",
			wantNonRetryable: true,
		},
		{
			name:            "engine failure step_limit is non-retryable",
			err:             engine.NewFailure(engine.StopReasonStepLimit, "steps exceeded", nil),
			wantType:        "engine.step_limit",
			wantNonRetryable: true,
		},
		{
			name:            "engine failure provider_error is non-retryable",
			err:             engine.NewFailure(engine.StopReasonProviderError, "provider down", nil),
			wantType:        "engine.provider_error",
			wantNonRetryable: true,
		},
		{
			name:            "engine failure sandbox_error is retryable",
			err:             engine.NewFailure(engine.StopReasonSandboxError, "sandbox crashed", nil),
			wantType:        "engine.sandbox_error",
			wantNonRetryable: false,
		},
		{
			name:            "engine failure observer_error is non-retryable",
			err:             engine.NewFailure(engine.StopReasonObserverError, "observer failed", nil),
			wantType:        "engine.observer_error",
			wantNonRetryable: true,
		},
		{
			name:            "provider failure non-retryable auth",
			err:             provider.NewFailure("openai", provider.FailureCodeAuth, "bad key", false, nil),
			wantType:        "provider.auth",
			wantNonRetryable: true,
		},
		{
			name:            "provider failure retryable rate_limit",
			err:             provider.NewFailure("openai", provider.FailureCodeRateLimit, "rate limited", true, nil),
			wantType:        "provider.rate_limit",
			wantNonRetryable: false,
		},
		{
			name:            "provider failure non-retryable invalid_request",
			err:             provider.NewFailure("openai", provider.FailureCodeInvalidRequest, "bad request", false, nil),
			wantType:        "provider.invalid_request",
			wantNonRetryable: true,
		},
		{
			name:            "unknown error passthrough",
			err:             errors.New("something unexpected"),
			wantPassthrough: true,
		},
		{
			name:            "wrapped repository error still detected",
			err:             fmt.Errorf("wrap: %w", repository.ErrRunNotFound),
			wantType:        repositoryRunNotFoundErrorType,
			wantNonRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapActivityError(tt.err)

			if tt.wantNil {
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("expected error, got nil")
			}

			if tt.wantPassthrough {
				var appErr *temporal.ApplicationError
				if errors.As(result, &appErr) {
					t.Fatalf("expected non-ApplicationError, got ApplicationError with type %q", appErr.Type())
				}
				if result.Error() != tt.err.Error() {
					t.Fatalf("error = %q, want %q", result.Error(), tt.err.Error())
				}
				return
			}

			var appErr *temporal.ApplicationError
			if !errors.As(result, &appErr) {
				t.Fatalf("expected temporal.ApplicationError, got %T: %v", result, result)
			}
			if appErr.Type() != tt.wantType {
				t.Fatalf("error type = %q, want %q", appErr.Type(), tt.wantType)
			}
			if appErr.NonRetryable() != tt.wantNonRetryable {
				t.Fatalf("non-retryable = %v, want %v", appErr.NonRetryable(), tt.wantNonRetryable)
			}
		})
	}
}

func TestLoadRun(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	activities := NewActivities(repo, FakeWorkHooks{})

	t.Run("success", func(t *testing.T) {
		run, err := activities.LoadRun(context.Background(), LoadRunInput{RunID: runID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if run.ID != runID {
			t.Fatalf("run ID = %s, want %s", run.ID, runID)
		}
	})

	t.Run("not found wraps error", func(t *testing.T) {
		_, err := activities.LoadRun(context.Background(), LoadRunInput{RunID: uuid.New()})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !hasApplicationErrorType(err, repositoryRunNotFoundErrorType) {
			t.Fatalf("expected run not found error type, got %v", err)
		}
	})
}

func TestListRunAgents(t *testing.T) {
	runID := uuid.New()
	agent1 := uuid.New()
	agent2 := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, agent1, 0),
		fixtureRunAgent(runID, agent2, 1),
	)
	activities := NewActivities(repo, FakeWorkHooks{})

	t.Run("success returns agents", func(t *testing.T) {
		agents, err := activities.ListRunAgents(context.Background(), ListRunAgentsInput{RunID: runID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(agents) != 2 {
			t.Fatalf("agent count = %d, want 2", len(agents))
		}
	})

	t.Run("empty list for unknown run", func(t *testing.T) {
		agents, err := activities.ListRunAgents(context.Background(), ListRunAgentsInput{RunID: uuid.New()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(agents) != 0 {
			t.Fatalf("agent count = %d, want 0", len(agents))
		}
	})
}

func TestLoadRunAgent(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	activities := NewActivities(repo, FakeWorkHooks{})

	t.Run("success", func(t *testing.T) {
		agent, err := activities.LoadRunAgent(context.Background(), LoadRunAgentInput{RunAgentID: runAgentID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.ID != runAgentID {
			t.Fatalf("agent ID = %s, want %s", agent.ID, runAgentID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := activities.LoadRunAgent(context.Background(), LoadRunAgentInput{RunAgentID: uuid.New()})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !hasApplicationErrorType(err, repositoryRunAgentNotFoundErrorType) {
			t.Fatalf("expected run agent not found error type, got %v", err)
		}
	})
}

func TestTransitionRunStatus(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusQueued),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	activities := NewActivities(repo, FakeWorkHooks{})

	t.Run("success", func(t *testing.T) {
		run, err := activities.TransitionRunStatus(context.Background(), TransitionRunStatusInput{
			RunID:    runID,
			ToStatus: domain.RunStatusProvisioning,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if run.Status != domain.RunStatusProvisioning {
			t.Fatalf("status = %s, want %s", run.Status, domain.RunStatusProvisioning)
		}
	})

	t.Run("invalid transition wraps error", func(t *testing.T) {
		// repo run is now in Provisioning from previous test; try invalid transition
		_, err := activities.TransitionRunStatus(context.Background(), TransitionRunStatusInput{
			RunID:    runID,
			ToStatus: domain.RunStatusQueued,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !hasApplicationErrorType(err, repositoryInvalidTransitionType) {
			t.Fatalf("expected invalid transition error type, got %v", err)
		}
	})
}

func TestTransitionRunAgentStatus(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	activities := NewActivities(repo, FakeWorkHooks{})

	t.Run("success", func(t *testing.T) {
		agent, err := activities.TransitionRunAgentStatus(context.Background(), TransitionRunAgentStatusInput{
			RunAgentID: runAgentID,
			ToStatus:   domain.RunAgentStatusReady,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.Status != domain.RunAgentStatusReady {
			t.Fatalf("status = %s, want %s", agent.Status, domain.RunAgentStatusReady)
		}
	})

	t.Run("not found wraps error", func(t *testing.T) {
		_, err := activities.TransitionRunAgentStatus(context.Background(), TransitionRunAgentStatusInput{
			RunAgentID: uuid.New(),
			ToStatus:   domain.RunAgentStatusReady,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !hasApplicationErrorType(err, repositoryRunAgentNotFoundErrorType) {
			t.Fatalf("expected run agent not found error type, got %v", err)
		}
	})
}

func TestExecuteNativeModelStep(t *testing.T) {
	t.Run("nil invoker returns nil", func(t *testing.T) {
		runID := uuid.New()
		runAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		activities := NewActivities(repo, FakeWorkHooks{})

		err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: runAgentID,
		})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		runID := uuid.New()
		runAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

		activities := NewActivities(repo, FakeWorkHooks{
			NativeModelInvoker: &fakeNativeModelInvoker{
				result: engine.Result{FinalOutput: "ok", StopReason: engine.StopReasonCompleted},
			},
		})

		err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: runAgentID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("repo error wraps to non-retryable", func(t *testing.T) {
		runID := uuid.New()
		unknownAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
		)

		activities := NewActivities(repo, FakeWorkHooks{
			NativeModelInvoker: &fakeNativeModelInvoker{},
		})

		err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: unknownAgentID,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !hasApplicationErrorType(err, repositoryRunAgentNotFoundErrorType) {
			t.Fatalf("expected run agent not found, got %v", err)
		}
	})

	t.Run("engine failure timeout non-retryable", func(t *testing.T) {
		runID := uuid.New()
		runAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

		activities := NewActivities(repo, FakeWorkHooks{
			NativeModelInvoker: &fakeNativeModelInvoker{
				err: engine.NewFailure(engine.StopReasonTimeout, "timed out", nil),
			},
		})

		err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: runAgentID,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		var appErr *temporal.ApplicationError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected ApplicationError, got %T", err)
		}
		if !appErr.NonRetryable() {
			t.Fatalf("timeout error should be non-retryable")
		}
		if appErr.Type() != "engine.timeout" {
			t.Fatalf("type = %q, want engine.timeout", appErr.Type())
		}
	})

	t.Run("engine failure sandbox_error retryable", func(t *testing.T) {
		runID := uuid.New()
		runAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

		activities := NewActivities(repo, FakeWorkHooks{
			NativeModelInvoker: &fakeNativeModelInvoker{
				err: engine.NewFailure(engine.StopReasonSandboxError, "sandbox crashed", nil),
			},
		})

		err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: runAgentID,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		var appErr *temporal.ApplicationError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected ApplicationError, got %T", err)
		}
		if appErr.NonRetryable() {
			t.Fatalf("sandbox error should be retryable")
		}
	})

	t.Run("provider failure non-retryable auth", func(t *testing.T) {
		runID := uuid.New()
		runAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

		activities := NewActivities(repo, FakeWorkHooks{
			NativeModelInvoker: &fakeNativeModelInvoker{
				err: provider.NewFailure("openai", provider.FailureCodeAuth, "bad key", false, nil),
			},
		})

		err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: runAgentID,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		var appErr *temporal.ApplicationError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected ApplicationError, got %T", err)
		}
		if !appErr.NonRetryable() {
			t.Fatalf("auth error should be non-retryable")
		}
		if appErr.Type() != "provider.auth" {
			t.Fatalf("type = %q, want provider.auth", appErr.Type())
		}
	})

	t.Run("provider failure retryable rate_limit", func(t *testing.T) {
		runID := uuid.New()
		runAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

		activities := NewActivities(repo, FakeWorkHooks{
			NativeModelInvoker: &fakeNativeModelInvoker{
				err: provider.NewFailure("openai", provider.FailureCodeRateLimit, "rate limited", true, nil),
			},
		})

		err := activities.ExecuteNativeModelStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: runAgentID,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		var appErr *temporal.ApplicationError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected ApplicationError, got %T", err)
		}
		if appErr.NonRetryable() {
			t.Fatalf("rate limit error should be retryable")
		}
	})
}

func TestExecutePromptEvalStep(t *testing.T) {
	t.Run("nil invoker returns non-retryable error", func(t *testing.T) {
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
			t.Fatalf("expected ApplicationError, got %T", err)
		}
		if !appErr.NonRetryable() {
			t.Fatalf("nil invoker error should be non-retryable")
		}
		if appErr.Type() != "workflow.prompt_eval_invoker_missing" {
			t.Fatalf("type = %q, want workflow.prompt_eval_invoker_missing", appErr.Type())
		}
	})

	t.Run("success", func(t *testing.T) {
		runID := uuid.New()
		runAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

		activities := NewActivities(repo, FakeWorkHooks{
			PromptEvalInvoker: &fakePromptEvalInvoker{
				result: engine.Result{FinalOutput: "eval done", StopReason: engine.StopReasonCompleted},
			},
		})

		err := activities.ExecutePromptEvalStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: runAgentID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("repo error wraps", func(t *testing.T) {
		runID := uuid.New()
		unknownAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
		)

		activities := NewActivities(repo, FakeWorkHooks{
			PromptEvalInvoker: &fakePromptEvalInvoker{},
		})

		err := activities.ExecutePromptEvalStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: unknownAgentID,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !hasApplicationErrorType(err, repositoryRunAgentNotFoundErrorType) {
			t.Fatalf("expected run agent not found, got %v", err)
		}
	})

	t.Run("invoker failure wraps error", func(t *testing.T) {
		runID := uuid.New()
		runAgentID := uuid.New()
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		repo.setExecutionContext(runAgentID, nativeExecutionContext(runID, runAgentID))

		activities := NewActivities(repo, FakeWorkHooks{
			PromptEvalInvoker: &fakePromptEvalInvoker{
				err: engine.NewFailure(engine.StopReasonTimeout, "eval timed out", nil),
			},
		})

		err := activities.ExecutePromptEvalStep(context.Background(), RunAgentWorkflowInput{
			RunID:      runID,
			RunAgentID: runAgentID,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !hasApplicationErrorType(err, "engine.timeout") {
			t.Fatalf("expected engine.timeout error type, got %v", err)
		}
	})
}

func TestPrepareExecutionLane(t *testing.T) {
	input := RunAgentWorkflowInput{RunID: uuid.New(), RunAgentID: uuid.New()}

	t.Run("nil hook returns nil", func(t *testing.T) {
		activities := NewActivities(nil, FakeWorkHooks{})
		err := activities.PrepareExecutionLane(context.Background(), input)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("hook called and returns nil", func(t *testing.T) {
		called := false
		activities := NewActivities(nil, FakeWorkHooks{
			PrepareExecutionLane: func(_ context.Context, _ RunAgentWorkflowInput) error {
				called = true
				return nil
			},
		})
		err := activities.PrepareExecutionLane(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Fatalf("hook was not called")
		}
	})

	t.Run("hook error propagated", func(t *testing.T) {
		hookErr := errors.New("lane prep failed")
		activities := NewActivities(nil, FakeWorkHooks{
			PrepareExecutionLane: func(_ context.Context, _ RunAgentWorkflowInput) error {
				return hookErr
			},
		})
		err := activities.PrepareExecutionLane(context.Background(), input)
		if !errors.Is(err, hookErr) {
			t.Fatalf("expected hook error, got %v", err)
		}
	})
}

func TestSimulateExecution(t *testing.T) {
	input := RunAgentWorkflowInput{RunID: uuid.New(), RunAgentID: uuid.New()}

	t.Run("nil hook returns nil", func(t *testing.T) {
		activities := NewActivities(nil, FakeWorkHooks{})
		err := activities.SimulateExecution(context.Background(), input)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("hook error propagated", func(t *testing.T) {
		hookErr := errors.New("simulate failed")
		activities := NewActivities(nil, FakeWorkHooks{
			SimulateExecution: func(_ context.Context, _ RunAgentWorkflowInput) error {
				return hookErr
			},
		})
		err := activities.SimulateExecution(context.Background(), input)
		if !errors.Is(err, hookErr) {
			t.Fatalf("expected hook error, got %v", err)
		}
	})
}

func TestSimulateEvaluation(t *testing.T) {
	input := RunAgentWorkflowInput{RunID: uuid.New(), RunAgentID: uuid.New()}

	t.Run("nil hook returns nil", func(t *testing.T) {
		activities := NewActivities(nil, FakeWorkHooks{})
		err := activities.SimulateEvaluation(context.Background(), input)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("hook error propagated", func(t *testing.T) {
		hookErr := errors.New("eval simulate failed")
		activities := NewActivities(nil, FakeWorkHooks{
			SimulateEvaluation: func(_ context.Context, _ RunAgentWorkflowInput) error {
				return hookErr
			},
		})
		err := activities.SimulateEvaluation(context.Background(), input)
		if !errors.Is(err, hookErr) {
			t.Fatalf("expected hook error, got %v", err)
		}
	})
}

func TestBuildRunAgentReplay(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()

	t.Run("success", func(t *testing.T) {
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		activities := NewActivities(repo, FakeWorkHooks{})

		replay, err := activities.BuildRunAgentReplay(context.Background(), BuildRunAgentReplayInput{
			RunAgentID: runAgentID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if replay.RunAgentID != runAgentID {
			t.Fatalf("replay agent ID = %s, want %s", replay.RunAgentID, runAgentID)
		}
	})

	t.Run("not found wraps error", func(t *testing.T) {
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusRunning),
		)
		activities := NewActivities(repo, FakeWorkHooks{})

		_, err := activities.BuildRunAgentReplay(context.Background(), BuildRunAgentReplayInput{
			RunAgentID: uuid.New(),
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !hasApplicationErrorType(err, repositoryRunAgentNotFoundErrorType) {
			t.Fatalf("expected run agent not found, got %v", err)
		}
	})
}

func TestBuildRunScorecard(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusScoring),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	activities := NewActivities(repo, FakeWorkHooks{})

	scorecard, err := activities.BuildRunScorecard(context.Background(), BuildRunScorecardInput{
		RunID: runID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scorecard.RunID != runID {
		t.Fatalf("scorecard run ID = %s, want %s", scorecard.RunID, runID)
	}
}

func TestAttachRunTemporalIDs(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()

	t.Run("success", func(t *testing.T) {
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusQueued),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		activities := NewActivities(repo, FakeWorkHooks{})

		run, err := activities.AttachRunTemporalIDs(context.Background(), AttachRunTemporalIDsInput{
			RunID:              runID,
			TemporalWorkflowID: "wf-123",
			TemporalRunID:      "run-456",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if run.TemporalWorkflowID == nil || *run.TemporalWorkflowID != "wf-123" {
			t.Fatalf("temporal workflow id = %v, want wf-123", run.TemporalWorkflowID)
		}
	})

	t.Run("conflict wraps error", func(t *testing.T) {
		repo := newFakeRunRepository(
			fixtureRun(runID, domain.RunStatusQueued),
			fixtureRunAgent(runID, runAgentID, 0),
		)
		activities := NewActivities(repo, FakeWorkHooks{})

		// First attach succeeds
		_, _ = activities.AttachRunTemporalIDs(context.Background(), AttachRunTemporalIDsInput{
			RunID:              runID,
			TemporalWorkflowID: "wf-123",
			TemporalRunID:      "run-456",
		})

		// Second attach with different IDs should conflict
		_, err := activities.AttachRunTemporalIDs(context.Background(), AttachRunTemporalIDsInput{
			RunID:              runID,
			TemporalWorkflowID: "wf-different",
			TemporalRunID:      "run-different",
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !hasApplicationErrorType(err, repositoryTemporalIDConflictType) {
			t.Fatalf("expected temporal id conflict, got %v", err)
		}
	})
}

func TestMarkHostedRunTimedOut(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, runAgentID, 0),
	)
	// Pre-create a hosted execution so MarkHostedRunExecutionTimedOut can find it
	repo.hostedExecutions[runAgentID] = repository.HostedRunExecution{
		RunAgentID: runAgentID,
		Status:     "running",
	}
	activities := NewActivities(repo, FakeWorkHooks{})

	err := activities.MarkHostedRunTimedOut(context.Background(), MarkHostedRunTimedOutInput{
		RunAgentID:   runAgentID,
		ErrorMessage: "deadline exceeded",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// fakePromptEvalInvoker implements the PromptEvalInvoker interface for testing.
type fakePromptEvalInvoker struct {
	result engine.Result
	err    error
}

func (f *fakePromptEvalInvoker) InvokePromptEval(_ context.Context, _ repository.RunAgentExecutionContext) (engine.Result, error) {
	if f.err != nil {
		return engine.Result{}, f.err
	}
	return f.result, nil
}
