package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/racecontext"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

// The run agent workflow writes the race standings entry itself for the
// paths whose events do not pass through the StandingsRecorder of the
// worker:
//   - A hosted agent sends its events to the API, which stores them
//     without the recorder.
//   - A connector agent stores its events in one database transaction.
//   - A run agent with case fan-out records no system.run.* event, because
//     replay and analytics read those events as the state of the run agent.
//
// The write changes no persisted run event.
const (
	updateRaceStandingsActivityName = "workflow.update_race_standings"

	// raceStandingsVersionChangeID gates the standings writes of the run
	// agent workflow. Histories from before the change do not get the new
	// activity.
	raceStandingsVersionChangeID = "race-standings-final-state"

	raceStandingsActivityTimeout = 5 * time.Second
	// raceStandingsStoreTimeout is shorter than the activity timeout. So a
	// slow store gives a store error, not an activity timeout and a retry.
	raceStandingsStoreTimeout = 3 * time.Second
	// A store error gets no retry, so a Redis failure costs one store call.
	// The retries are for a failure before the store call: in a rolling
	// deploy, an old worker does not know the activity, and a retry then
	// goes to a new worker.
	raceStandingsActivityMaxAttempts = 5

	invalidRaceStandingsStateErrorType = "workflow.invalid_race_standings_state"
	raceStandingsWriteFailedErrorType  = "workflow.race_standings_write_failed"
)

type UpdateRaceStandingsInput struct {
	RunID      uuid.UUID                  `json:"run_id"`
	RunAgentID uuid.UUID                  `json:"run_agent_id"`
	State      racecontext.StandingsState `json:"state"`
	OccurredAt time.Time                  `json:"occurred_at"`
}

// UpdateRaceStandings merges one state into the standings entry of a run
// agent. The store keeps a final state final, so a second write is safe.
func (a *Activities) UpdateRaceStandings(ctx context.Context, input UpdateRaceStandingsInput) error {
	store := a.hooks.StandingsStore
	if store == nil {
		return nil
	}
	if !input.State.Valid() {
		err := fmt.Errorf("race standings state %q is not valid", input.State)
		return temporal.NewNonRetryableApplicationError(err.Error(), invalidRaceStandingsStateErrorType, err)
	}
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	entry := racecontext.StandingsEntry{
		RunAgentID: input.RunAgentID,
		State:      input.State,
	}
	switch input.State {
	case racecontext.StandingsStateRunning:
		entry.StartedAt = &occurredAt
	case racecontext.StandingsStateSubmitted:
		entry.SubmittedAt = &occurredAt
	case racecontext.StandingsStateFailed, racecontext.StandingsStateTimedOut:
		entry.FailedAt = &occurredAt
	}
	updateCtx, cancel := context.WithTimeout(ctx, raceStandingsStoreTimeout)
	defer cancel()
	if err := store.Update(updateCtx, input.RunID, entry); err != nil {
		return temporal.NewNonRetryableApplicationError(err.Error(), raceStandingsWriteFailedErrorType, err)
	}
	return nil
}

// writeRaceStandings writes one standings state for the run agent when the
// run has race context. The write is best effort: a failure only logs, the
// same as the StandingsRecorder. A run without race context gets no command,
// so its history does not change.
func writeRaceStandings(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext, state racecontext.StandingsState) {
	if !executionContext.Run.RaceContext {
		return
	}
	if sdkworkflow.GetVersion(ctx, raceStandingsVersionChangeID, sdkworkflow.DefaultVersion, 1) == sdkworkflow.DefaultVersion {
		return
	}
	options := defaultActivityOptions
	options.StartToCloseTimeout = raceStandingsActivityTimeout
	options.RetryPolicy = &temporal.RetryPolicy{MaximumAttempts: raceStandingsActivityMaxAttempts}
	err := sdkworkflow.ExecuteActivity(sdkworkflow.WithActivityOptions(ctx, options), updateRaceStandingsActivityName, UpdateRaceStandingsInput{
		RunID:      input.RunID,
		RunAgentID: input.RunAgentID,
		State:      state,
		OccurredAt: sdkworkflow.Now(ctx).UTC(),
	}).Get(ctx, nil)
	if err != nil {
		sdkworkflow.GetLogger(ctx).Warn("race standings write failed",
			"run_agent_id", input.RunAgentID.String(),
			"state", string(state),
			"error", err,
		)
	}
}

// writeRaceStandingsForError writes the final state for a failed run agent.
// A cancel writes nothing: the activity cannot run on a canceled context.
func writeRaceStandingsForError(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext, err error) {
	if isWorkflowCanceled(err) {
		return
	}
	writeRaceStandings(ctx, input, executionContext, raceStandingsStateForError(err))
}

// raceStandingsStateForError gives timed_out for a timeout and failed for
// each other error. The newswire shows a timeout as "TIMED OUT", and peers
// get the peer_timed_out trigger.
func raceStandingsStateForError(err error) racecontext.StandingsState {
	if isRaceTimeoutError(err) {
		return racecontext.StandingsStateTimedOut
	}
	return racecontext.StandingsStateFailed
}

func isRaceTimeoutError(err error) bool {
	var hostedTimeout hostedRunTimeoutError
	if errors.As(err, &hostedTimeout) {
		return true
	}
	var activityTimeout *temporal.TimeoutError
	if errors.As(err, &activityTimeout) {
		return true
	}
	return hasApplicationErrorType(err, engineFailureErrorTypePrefix+string(engine.StopReasonTimeout))
}

// caseFanoutStandingsState gives the state of a run agent after all of its
// cases finish. One passed case is enough for submitted, because the run
// agent then goes on to evaluation with a partial result.
func caseFanoutStandingsState(succeeded, failed, timedOut int) racecontext.StandingsState {
	switch {
	case succeeded > 0:
		return racecontext.StandingsStateSubmitted
	case failed > 0 && timedOut == failed:
		return racecontext.StandingsStateTimedOut
	default:
		return racecontext.StandingsStateFailed
	}
}

// hostedRunTimeoutError is the error of a hosted run that got no callback
// before its deadline. Error gives the same text as before, so the failure
// reason of the run agent does not change.
type hostedRunTimeoutError struct {
	reason string
}

func (e hostedRunTimeoutError) Error() string { return e.reason }
