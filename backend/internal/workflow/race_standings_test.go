package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/racecontext"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/hostedruns"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

// recordingStandingsStore applies racecontext.MergeEntry, the same as the
// Redis store, and keeps the order of the written states.
type recordingStandingsStore struct {
	mu        sync.Mutex
	runIDs    []uuid.UUID
	states    []racecontext.StandingsState
	entries   map[uuid.UUID]racecontext.StandingsEntry
	deadlines []time.Time
	err       error
}

func newRecordingStandingsStore() *recordingStandingsStore {
	return &recordingStandingsStore{entries: map[uuid.UUID]racecontext.StandingsEntry{}}
}

func (s *recordingStandingsStore) Update(ctx context.Context, runID uuid.UUID, update racecontext.StandingsEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if deadline, ok := ctx.Deadline(); ok {
		s.deadlines = append(s.deadlines, deadline)
	}
	if s.err != nil {
		return s.err
	}
	s.runIDs = append(s.runIDs, runID)
	s.states = append(s.states, update.State)
	s.entries[update.RunAgentID] = racecontext.MergeEntry(s.entries[update.RunAgentID], update)
	return nil
}

func (s *recordingStandingsStore) Snapshot(context.Context, uuid.UUID) (map[uuid.UUID]racecontext.StandingsEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[uuid.UUID]racecontext.StandingsEntry, len(s.entries))
	for id, entry := range s.entries {
		out[id] = entry
	}
	return out, nil
}

func (s *recordingStandingsStore) Close() error { return nil }

func (s *recordingStandingsStore) writtenStates() []racecontext.StandingsState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]racecontext.StandingsState(nil), s.states...)
}

func (s *recordingStandingsStore) entry(runAgentID uuid.UUID) racecontext.StandingsEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entries[runAgentID]
}

func assertStandingsStates(t *testing.T, store *recordingStandingsStore, want ...racecontext.StandingsState) {
	t.Helper()
	got := store.writtenStates()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("written standings states = %v, want %v", got, want)
	}
}

func TestUpdateRaceStandingsSetsTheTimeOfTheState(t *testing.T) {
	occurredAt := time.Date(2026, 11, 11, 9, 30, 0, 0, time.UTC)
	for _, state := range []racecontext.StandingsState{
		racecontext.StandingsStateRunning,
		racecontext.StandingsStateSubmitted,
		racecontext.StandingsStateFailed,
		racecontext.StandingsStateTimedOut,
	} {
		t.Run(string(state), func(t *testing.T) {
			store := newRecordingStandingsStore()
			activities := NewActivities(newFakeRunRepository(domain.Run{}), FakeWorkHooks{StandingsStore: store})
			runID, runAgentID := uuid.New(), uuid.New()

			if err := activities.UpdateRaceStandings(context.Background(), UpdateRaceStandingsInput{
				RunID:      runID,
				RunAgentID: runAgentID,
				State:      state,
				OccurredAt: occurredAt,
			}); err != nil {
				t.Fatalf("UpdateRaceStandings returned error: %v", err)
			}

			if len(store.runIDs) != 1 || store.runIDs[0] != runID {
				t.Fatalf("run IDs = %v, want [%s]", store.runIDs, runID)
			}
			entry := store.entry(runAgentID)
			if entry.State != state || entry.RunAgentID != runAgentID {
				t.Fatalf("entry = %+v, want state %s for %s", entry, state, runAgentID)
			}
			var stamp *time.Time
			switch state {
			case racecontext.StandingsStateRunning:
				stamp = entry.StartedAt
			case racecontext.StandingsStateSubmitted:
				stamp = entry.SubmittedAt
			default:
				stamp = entry.FailedAt
			}
			if stamp == nil || !stamp.Equal(occurredAt) {
				t.Fatalf("time of state %s = %v, want %s", state, stamp, occurredAt)
			}
		})
	}
}

func TestUpdateRaceStandingsRejectsAnUnknownState(t *testing.T) {
	store := newRecordingStandingsStore()
	activities := NewActivities(newFakeRunRepository(domain.Run{}), FakeWorkHooks{StandingsStore: store})

	err := activities.UpdateRaceStandings(context.Background(), UpdateRaceStandingsInput{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		State:      "finished",
	})
	if !hasApplicationErrorType(err, invalidRaceStandingsStateErrorType) {
		t.Fatalf("error = %v, want %s", err, invalidRaceStandingsStateErrorType)
	}
	var applicationErr *temporal.ApplicationError
	if !errors.As(err, &applicationErr) || !applicationErr.NonRetryable() {
		t.Fatalf("error = %v, want a non-retryable application error", err)
	}
	if got := store.writtenStates(); len(got) != 0 {
		t.Fatalf("store got %v, want no write", got)
	}
}

// A store failure must not get a retry, and the store call must stop before
// the activity timeout. So a Redis failure costs one store call.
func TestUpdateRaceStandingsStoreFailureIsNotRetried(t *testing.T) {
	store := newRecordingStandingsStore()
	store.err = errors.New("redis unavailable")
	activities := NewActivities(newFakeRunRepository(domain.Run{}), FakeWorkHooks{StandingsStore: store})

	err := activities.UpdateRaceStandings(context.Background(), UpdateRaceStandingsInput{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		State:      racecontext.StandingsStateSubmitted,
	})
	end := time.Now()

	if !hasApplicationErrorType(err, raceStandingsWriteFailedErrorType) {
		t.Fatalf("error = %v, want %s", err, raceStandingsWriteFailedErrorType)
	}
	var applicationErr *temporal.ApplicationError
	if !errors.As(err, &applicationErr) || !applicationErr.NonRetryable() {
		t.Fatalf("error = %v, want a non-retryable application error", err)
	}
	if len(store.deadlines) != 1 {
		t.Fatalf("store calls with a deadline = %d, want 1", len(store.deadlines))
	}
	if limit := end.Add(raceStandingsStoreTimeout); store.deadlines[0].After(limit) {
		t.Fatalf("store deadline %s is after %s", store.deadlines[0], limit)
	}
	if raceStandingsStoreTimeout >= raceStandingsActivityTimeout {
		t.Fatalf("store timeout %s must be shorter than the activity timeout %s", raceStandingsStoreTimeout, raceStandingsActivityTimeout)
	}
}

func TestUpdateRaceStandingsWithoutAStoreDoesNothing(t *testing.T) {
	activities := NewActivities(newFakeRunRepository(domain.Run{}), FakeWorkHooks{})
	if err := activities.UpdateRaceStandings(context.Background(), UpdateRaceStandingsInput{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		State:      racecontext.StandingsStateSubmitted,
	}); err != nil {
		t.Fatalf("UpdateRaceStandings returned error: %v", err)
	}
}

func TestCaseFanoutStandingsState(t *testing.T) {
	for _, tc := range []struct {
		succeeded, failed, timedOut int
		want                        racecontext.StandingsState
	}{
		{succeeded: 3, want: racecontext.StandingsStateSubmitted},
		{succeeded: 1, failed: 2, timedOut: 1, want: racecontext.StandingsStateSubmitted},
		{succeeded: 1, failed: 2, timedOut: 2, want: racecontext.StandingsStateSubmitted},
		{failed: 3, want: racecontext.StandingsStateFailed},
		{failed: 3, timedOut: 2, want: racecontext.StandingsStateFailed},
		{failed: 3, timedOut: 3, want: racecontext.StandingsStateTimedOut},
		{want: racecontext.StandingsStateFailed},
	} {
		got := caseFanoutStandingsState(tc.succeeded, tc.failed, tc.timedOut)
		if got != tc.want {
			t.Errorf("caseFanoutStandingsState(%d, %d, %d) = %s, want %s", tc.succeeded, tc.failed, tc.timedOut, got, tc.want)
		}
	}
}

func TestRaceStandingsStateForError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want racecontext.StandingsState
	}{
		{name: "hosted timeout", err: hostedRunTimeoutError{reason: "late"}, want: racecontext.StandingsStateTimedOut},
		{name: "wrapped hosted timeout", err: fmt.Errorf("wrap: %w", hostedRunTimeoutError{reason: "late"}), want: racecontext.StandingsStateTimedOut},
		{name: "engine timeout", err: temporal.NewNonRetryableApplicationError("slow", engineFailureErrorTypePrefix+string(engine.StopReasonTimeout), nil), want: racecontext.StandingsStateTimedOut},
		{name: "activity timeout", err: temporal.NewTimeoutError(enumspb.TIMEOUT_TYPE_START_TO_CLOSE, nil), want: racecontext.StandingsStateTimedOut},
		{name: "other engine failure", err: temporal.NewNonRetryableApplicationError("bad", engineFailureErrorTypePrefix+"provider_error", nil), want: racecontext.StandingsStateFailed},
		{name: "plain error", err: errors.New("remote failed"), want: racecontext.StandingsStateFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := raceStandingsStateForError(tc.err); got != tc.want {
				t.Fatalf("state = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestHostedRunTimeoutErrorKeepsTheOldText(t *testing.T) {
	err := hostedTimeoutOutcome(MarkHostedRunTimedOutResult{}, "hosted run timed out waiting for callback after 5s")
	if err == nil || err.Error() != "hosted run timed out waiting for callback after 5s" {
		t.Fatalf("error = %v, want the old timeout text", err)
	}
	if raceStandingsStateForError(err) != racecontext.StandingsStateTimedOut {
		t.Fatalf("state of %v is not timed_out", err)
	}
}

// newHostedRaceEnv gives a hosted run agent in a run with race context.
func newHostedRaceEnv(t *testing.T, raceContext bool, hooks FakeWorkHooks) (*fakeRunRepository, *testsuite.TestWorkflowEnvironment, RunAgentWorkflowInput, *recordingStandingsStore) {
	t.Helper()
	runID, runAgentID := uuid.New(), uuid.New()
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusRunning), fixtureRunAgent(runID, runAgentID, 0))
	executionContext := hostedExecutionContext(runID, runAgentID)
	executionContext.Run.RaceContext = raceContext
	repo.setExecutionContext(runAgentID, executionContext)
	store := newRecordingStandingsStore()
	hooks.StandingsStore = store
	if hooks.HostedRunStarter == nil {
		hooks.HostedRunStarter = fakeHostedRunStarter{response: hostedruns.StartResponse{Accepted: true, ExternalRunID: "ext-race"}}
	}
	env := newTestWorkflowEnvironment(repo, hooks)
	return repo, env, RunAgentWorkflowInput{RunID: runID, RunAgentID: runAgentID}, store
}

func signalHostedEvent(env *testsuite.TestWorkflowEnvironment, event hostedruns.Event) {
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(HostedRunEventSignal, event)
	}, fakeStageDelay/2)
}

func TestRunAgentWorkflowHostedRaceStandingsSubmitted(t *testing.T) {
	repo, env, input, store := newHostedRaceEnv(t, true, FakeWorkHooks{})
	status := hostedruns.FinalStatusCompleted
	signalHostedEvent(env, hostedruns.Event{
		RunAgentID:    input.RunAgentID,
		ExternalRunID: "ext-race",
		EventType:     hostedruns.EventTypeRunFinished,
		OccurredAt:    time.Now().UTC(),
		FinalStatus:   &status,
		Output:        []byte(`{"answer":"done"}`),
	})

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunAgentWorkflow returned error: %v", err)
	}
	if got := repo.currentRunAgent(input.RunAgentID).Status; got != domain.RunAgentStatusEvaluating {
		t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusEvaluating)
	}
	assertStandingsStates(t, store, racecontext.StandingsStateRunning, racecontext.StandingsStateSubmitted)
	entry := store.entry(input.RunAgentID)
	if entry.State != racecontext.StandingsStateSubmitted || entry.StartedAt == nil || entry.SubmittedAt == nil {
		t.Fatalf("entry = %+v, want submitted with a start time and a submit time", entry)
	}
	if entry.SubmittedAt.Before(*entry.StartedAt) {
		t.Fatalf("submitted at %s is before started at %s", entry.SubmittedAt, entry.StartedAt)
	}
}

func TestRunAgentWorkflowHostedRaceStandingsFailed(t *testing.T) {
	repo, env, input, store := newHostedRaceEnv(t, true, FakeWorkHooks{})
	message := "remote agent crashed"
	signalHostedEvent(env, hostedruns.Event{
		RunAgentID:    input.RunAgentID,
		ExternalRunID: "ext-race",
		EventType:     hostedruns.EventTypeError,
		OccurredAt:    time.Now().UTC(),
		ErrorMessage:  &message,
	})

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	if err := env.GetWorkflowError(); err == nil || !strings.Contains(err.Error(), message) {
		t.Fatalf("workflow error = %v, want %q", err, message)
	}
	if got := repo.currentRunAgent(input.RunAgentID).Status; got != domain.RunAgentStatusFailed {
		t.Fatalf("run agent status = %s, want failed", got)
	}
	assertStandingsStates(t, store, racecontext.StandingsStateRunning, racecontext.StandingsStateFailed)
	if entry := store.entry(input.RunAgentID); entry.FailedAt == nil {
		t.Fatalf("entry = %+v, want a failed time", entry)
	}
}

func TestRunAgentWorkflowHostedRaceStandingsTimedOut(t *testing.T) {
	repo, env, input, store := newHostedRaceEnv(t, true, FakeWorkHooks{})

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	err := env.GetWorkflowError()
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for callback") {
		t.Fatalf("workflow error = %v, want the timeout failure", err)
	}
	runAgent := repo.currentRunAgent(input.RunAgentID)
	if runAgent.Status != domain.RunAgentStatusFailed || runAgent.FailureReason == nil || !strings.HasPrefix(*runAgent.FailureReason, "hosted run timed out waiting for callback after ") {
		t.Fatalf("run agent status = %s reason = %v, want failed with the old timeout text", runAgent.Status, runAgent.FailureReason)
	}
	assertStandingsStates(t, store, racecontext.StandingsStateRunning, racecontext.StandingsStateTimedOut)
}

// The callback came before the workflow waited, so the start activity
// reports the run as completed.
func TestRunAgentWorkflowHostedRaceStandingsCompletedBeforeTheWait(t *testing.T) {
	repo, env, input, store := newHostedRaceEnv(t, true, FakeWorkHooks{})
	env.OnActivity(startHostedRunActivityName, mock.Anything, mock.Anything).
		Return(StartHostedRunResult{Completed: true, ExternalRunID: "ext-race"}, nil).Once()

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunAgentWorkflow returned error: %v", err)
	}
	if got := repo.currentRunAgent(input.RunAgentID).Status; got != domain.RunAgentStatusEvaluating {
		t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusEvaluating)
	}
	assertStandingsStates(t, store, racecontext.StandingsStateRunning, racecontext.StandingsStateSubmitted)
	env.AssertExpectations(t)
}

func TestRunAgentWorkflowHostedRaceStandingsStartFailure(t *testing.T) {
	_, env, input, store := newHostedRaceEnv(t, true, FakeWorkHooks{})
	env.OnActivity(startHostedRunActivityName, mock.Anything, mock.Anything).
		Return(StartHostedRunResult{}, errors.New("dispatch persistence unavailable")).Once()

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("failed start was reported successful")
	}
	assertStandingsStates(t, store, racecontext.StandingsStateFailed)
}

// A cancel writes no final state. The API cancels the full run, so the peers
// stop too.
func TestRunAgentWorkflowHostedRaceStandingsCancelWritesNoFinalState(t *testing.T) {
	_, env, input, store := newHostedRaceEnv(t, true, FakeWorkHooks{})
	env.RegisterDelayedCallback(env.CancelWorkflow, fakeStageDelay/2)

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	if err := env.GetWorkflowError(); !isWorkflowCanceled(err) {
		t.Fatalf("workflow error = %v, want a cancel", err)
	}
	assertStandingsStates(t, store, racecontext.StandingsStateRunning)
}

func TestRunAgentWorkflowHostedWithoutRaceContextWritesNoStandings(t *testing.T) {
	_, env, input, store := newHostedRaceEnv(t, false, FakeWorkHooks{})
	status := hostedruns.FinalStatusCompleted
	signalHostedEvent(env, hostedruns.Event{
		RunAgentID:    input.RunAgentID,
		ExternalRunID: "ext-race",
		EventType:     hostedruns.EventTypeRunFinished,
		OccurredAt:    time.Now().UTC(),
		FinalStatus:   &status,
	})

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunAgentWorkflow returned error: %v", err)
	}
	assertStandingsStates(t, store)
}

// A history from before the change replays with DefaultVersion and gets no
// standings activity.
func TestRunAgentWorkflowHostedRaceStandingsDefaultVersionWritesNothing(t *testing.T) {
	_, env, input, store := newHostedRaceEnv(t, true, FakeWorkHooks{})
	env.OnGetVersion(raceStandingsVersionChangeID, sdkworkflow.DefaultVersion, sdkworkflow.Version(1)).
		Return(sdkworkflow.DefaultVersion)
	status := hostedruns.FinalStatusCompleted
	signalHostedEvent(env, hostedruns.Event{
		RunAgentID:    input.RunAgentID,
		ExternalRunID: "ext-race",
		EventType:     hostedruns.EventTypeRunFinished,
		OccurredAt:    time.Now().UTC(),
		FinalStatus:   &status,
	})

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunAgentWorkflow returned error: %v", err)
	}
	assertStandingsStates(t, store)
}

// A failed standings write must not fail the run agent.
func TestRunAgentWorkflowHostedRaceStandingsWriteFailureIsIgnored(t *testing.T) {
	repo, env, input, _ := newHostedRaceEnv(t, true, FakeWorkHooks{})
	env.OnActivity(updateRaceStandingsActivityName, mock.Anything, mock.Anything).
		Return(errors.New("redis unavailable"))
	status := hostedruns.FinalStatusCompleted
	signalHostedEvent(env, hostedruns.Event{
		RunAgentID:    input.RunAgentID,
		ExternalRunID: "ext-race",
		EventType:     hostedruns.EventTypeRunFinished,
		OccurredAt:    time.Now().UTC(),
		FinalStatus:   &status,
	})

	env.ExecuteWorkflow(RunAgentWorkflow, input)

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("RunAgentWorkflow returned error: %v", err)
	}
	if got := repo.currentRunAgent(input.RunAgentID).Status; got != domain.RunAgentStatusEvaluating {
		t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusEvaluating)
	}
}

type fakeConnectorInvoker struct {
	err error
}

func (f fakeConnectorInvoker) InvokeConnector(context.Context, repository.RunAgentExecutionContext) error {
	return f.err
}

func TestRunAgentWorkflowConnectorRaceStandings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		invokeErr error
		want      racecontext.StandingsState
	}{
		{name: "submitted", want: racecontext.StandingsStateSubmitted},
		{name: "failed", invokeErr: errors.New("connector refused the request"), want: racecontext.StandingsStateFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runID, runAgentID := uuid.New(), uuid.New()
			repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusRunning), fixtureRunAgent(runID, runAgentID, 0))
			executionContext := hostedExecutionContext(runID, runAgentID)
			executionContext.Run.RaceContext = true
			executionContext.Deployment.Connector = &connectors.Binding{Config: connectors.Config{Adapter: connectors.HTTPJSON}}
			repo.setExecutionContext(runAgentID, executionContext)
			store := newRecordingStandingsStore()
			env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
				ConnectorInvoker: fakeConnectorInvoker{err: tc.invokeErr},
				StandingsStore:   store,
			})

			env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{RunID: runID, RunAgentID: runAgentID})

			if err := env.GetWorkflowError(); (err != nil) != (tc.invokeErr != nil) {
				t.Fatalf("workflow error = %v, want error %v", err, tc.invokeErr != nil)
			}
			assertStandingsStates(t, store, racecontext.StandingsStateRunning, tc.want)
		})
	}
}

func TestRunAgentWorkflowCaseFanoutRaceStandings(t *testing.T) {
	timeoutErr := func() error {
		return temporal.NewNonRetryableApplicationError(
			"case timed out",
			engineFailureErrorTypePrefix+string(engine.StopReasonTimeout),
			engine.NewFailure(engine.StopReasonTimeout, "case timed out", nil),
		)
	}
	failErr := func() error {
		return temporal.NewNonRetryableApplicationError("case failed", engineFailureErrorTypePrefix+"provider_error", nil)
	}
	for _, tc := range []struct {
		name        string
		raceContext bool
		failures    map[string]error
		want        []racecontext.StandingsState
	}{
		{
			name:        "all cases pass",
			raceContext: true,
			want:        []racecontext.StandingsState{racecontext.StandingsStateSubmitted},
		},
		{
			name:        "one case fails",
			raceContext: true,
			failures:    map[string]error{"case-1": failErr()},
			want:        []racecontext.StandingsState{racecontext.StandingsStateSubmitted},
		},
		{
			name:        "all cases fail",
			raceContext: true,
			failures:    map[string]error{"case-0": failErr(), "case-1": timeoutErr(), "case-2": failErr()},
			want:        []racecontext.StandingsState{racecontext.StandingsStateFailed},
		},
		{
			name:        "all cases time out",
			raceContext: true,
			failures:    map[string]error{"case-0": timeoutErr(), "case-1": timeoutErr(), "case-2": timeoutErr()},
			want:        []racecontext.StandingsState{racecontext.StandingsStateTimedOut},
		},
		{
			name: "no race context",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runID, runAgentID := uuid.New(), uuid.New()
			repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusRunning), fixtureRunAgent(runID, runAgentID, 0))
			executionContext := nativeExecutionContextWithCases(runID, runAgentID, 3)
			executionContext.Run.RaceContext = tc.raceContext
			executionContext.Deployment.RuntimeProfile.ProfileConfig = json.RawMessage(`{"case_fanout":true,"max_in_flight_cases":2}`)
			repo.setExecutionContext(runAgentID, executionContext)
			store := newRecordingStandingsStore()
			env := newTestWorkflowEnvironment(repo, FakeWorkHooks{
				NativeModelInvoker: &trackingNativeModelInvoker{failCaseKeys: tc.failures},
				StandingsStore:     store,
			})

			env.ExecuteWorkflow(RunAgentWorkflow, RunAgentWorkflowInput{RunID: runID, RunAgentID: runAgentID})

			if err := env.GetWorkflowError(); err != nil {
				t.Fatalf("RunAgentWorkflow returned error: %v", err)
			}
			if got := repo.currentRunAgent(runAgentID).Status; got != domain.RunAgentStatusEvaluating {
				t.Fatalf("run agent status = %s, want %s", got, domain.RunAgentStatusEvaluating)
			}
			assertStandingsStates(t, store, tc.want...)
		})
	}
}
