package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/hostedruns"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrRunMustBeQueued      = errors.New("run must already be queued")
	ErrRunHasNoAgents       = errors.New("run must have at least one run agent")
	ErrRunAgentMustBeQueued = errors.New("run agent must already be queued")
	ErrRunAgentRunMismatch  = errors.New("run agent does not belong to run")
)

type RunRepository interface {
	GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error)
	ListRunAgentsByRunID(ctx context.Context, runID uuid.UUID) ([]domain.RunAgent, error)
	GetRunAgentByID(ctx context.Context, id uuid.UUID) (domain.RunAgent, error)
	GetRunAgentExecutionContextByID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentExecutionContext, error)
	BuildRunAgentReplay(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentReplay, error)
	SetRunTemporalIDs(ctx context.Context, params repository.SetRunTemporalIDsParams) (domain.Run, error)
	TransitionRunStatus(ctx context.Context, params repository.TransitionRunStatusParams) (domain.Run, error)
	TransitionRunAgentStatus(ctx context.Context, params repository.TransitionRunAgentStatusParams) (domain.RunAgent, error)
	CreateHostedRunExecution(ctx context.Context, params repository.CreateHostedRunExecutionParams) (repository.HostedRunExecution, error)
	MarkHostedRunExecutionAccepted(ctx context.Context, params repository.MarkHostedRunExecutionAcceptedParams) (repository.HostedRunExecution, error)
	MarkHostedRunExecutionFailed(ctx context.Context, params repository.MarkHostedRunExecutionFailedParams) (repository.HostedRunExecution, error)
	MarkHostedRunExecutionTimedOut(ctx context.Context, params repository.MarkHostedRunExecutionTimedOutParams) (repository.HostedRunExecution, error)
}

type HostedRunStarter interface {
	Start(ctx context.Context, input HostedRunStartInput) (hostedruns.StartResponse, error)
}

type HostedRunStartInput struct {
	ExecutionContext repository.RunAgentExecutionContext
	TraceLevel       string
	TaskPayload      json.RawMessage
	DeadlineAt       time.Time
}

func validateRunQueued(run domain.Run) error {
	if run.Status == domain.RunStatusQueued {
		return nil
	}

	return fmt.Errorf("%w: run %s is %s", ErrRunMustBeQueued, run.ID, run.Status)
}

func validateRunAgentQueued(runAgent domain.RunAgent, runID uuid.UUID) error {
	if runAgent.RunID != runID {
		return fmt.Errorf("%w: run_agent=%s run=%s expected_run=%s", ErrRunAgentRunMismatch, runAgent.ID, runAgent.RunID, runID)
	}
	if runAgent.Status == domain.RunAgentStatusQueued {
		return nil
	}

	return fmt.Errorf("%w: run_agent %s is %s", ErrRunAgentMustBeQueued, runAgent.ID, runAgent.Status)
}
