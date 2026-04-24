package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/racecontext"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

type fakeStandingsStore struct {
	snapshot map[uuid.UUID]racecontext.StandingsEntry
	updates  []racecontext.StandingsEntry
	err      error
}

func (f *fakeStandingsStore) Update(_ context.Context, _ uuid.UUID, update racecontext.StandingsEntry) error {
	if f.snapshot == nil {
		f.snapshot = map[uuid.UUID]racecontext.StandingsEntry{}
	}
	f.updates = append(f.updates, update)
	f.snapshot[update.RunAgentID] = racecontext.MergeEntry(f.snapshot[update.RunAgentID], update)
	return nil
}

func (f *fakeStandingsStore) Snapshot(context.Context, uuid.UUID) (map[uuid.UUID]racecontext.StandingsEntry, error) {
	return f.snapshot, f.err
}

func (f *fakeStandingsStore) Close() error { return nil }

type captureObserver struct {
	NoopObserver
	injections []StandingsInjection
}

func (c *captureObserver) OnStandingsInjected(_ context.Context, inj StandingsInjection) error {
	c.injections = append(c.injections, inj)
	return nil
}

func TestEvaluateRaceContextCadenceFiresOnFirstEligibleStep(t *testing.T) {
	self := uuid.New()
	peer := uuid.New()
	snapshot := map[uuid.UUID]racecontext.StandingsEntry{
		self: {RunAgentID: self, Step: 3, State: racecontext.StandingsStateRunning},
		peer: {RunAgentID: peer, Step: 2, State: racecontext.StandingsStateRunning},
	}
	state := &loopState{stepCount: 3}

	trigger, fire := evaluateRaceContextCadence(state, snapshot, self, 3)
	if !fire {
		t.Fatalf("first eligible step must fire; trigger = %q", trigger)
	}
	if trigger != runevents.RaceStandingsTriggerCadence {
		t.Fatalf("first-fire trigger = %q, want cadence", trigger)
	}
}

func TestEvaluateRaceContextCadenceSuppressesWithinGap(t *testing.T) {
	self := uuid.New()
	peer := uuid.New()
	snapshot := map[uuid.UUID]racecontext.StandingsEntry{
		peer: {RunAgentID: peer, Step: 5, State: racecontext.StandingsStateRunning},
	}
	state := &loopState{
		stepCount:         4,
		lastInjectionStep: 3,
		lastPeerStates:    map[uuid.UUID]racecontext.StandingsState{peer: racecontext.StandingsStateRunning},
	}

	_, fire := evaluateRaceContextCadence(state, snapshot, self, 3)
	if fire {
		t.Fatalf("must not fire within min_step_gap (step=4, lastInj=3, gap=3)")
	}
}

func TestEvaluateRaceContextCadenceFiresAfterGap(t *testing.T) {
	self := uuid.New()
	peer := uuid.New()
	snapshot := map[uuid.UUID]racecontext.StandingsEntry{
		peer: {RunAgentID: peer, Step: 5, State: racecontext.StandingsStateRunning},
	}
	state := &loopState{
		stepCount:         6,
		lastInjectionStep: 3,
		lastPeerStates:    map[uuid.UUID]racecontext.StandingsState{peer: racecontext.StandingsStateRunning},
	}

	trigger, fire := evaluateRaceContextCadence(state, snapshot, self, 3)
	if !fire {
		t.Fatalf("cadence must fire at step=6 with lastInj=3 gap=3")
	}
	if trigger != runevents.RaceStandingsTriggerCadence {
		t.Fatalf("trigger = %q, want cadence", trigger)
	}
}

func TestEvaluateRaceContextCadenceFiresOnPeerSubmission(t *testing.T) {
	self := uuid.New()
	peer := uuid.New()
	snapshot := map[uuid.UUID]racecontext.StandingsEntry{
		peer: {RunAgentID: peer, Step: 5, State: racecontext.StandingsStateSubmitted},
	}
	state := &loopState{
		stepCount:         4,
		lastInjectionStep: 3,
		lastPeerStates:    map[uuid.UUID]racecontext.StandingsState{peer: racecontext.StandingsStateRunning},
	}

	trigger, fire := evaluateRaceContextCadence(state, snapshot, self, 3)
	if !fire {
		t.Fatalf("peer submission must trigger injection even within min_step_gap")
	}
	if trigger != runevents.RaceStandingsTriggerPeerSubmitted {
		t.Fatalf("trigger = %q, want peer_submitted", trigger)
	}
}

func TestEvaluateRaceContextCadenceFiresOnPeerFailure(t *testing.T) {
	self := uuid.New()
	peer := uuid.New()
	snapshot := map[uuid.UUID]racecontext.StandingsEntry{
		peer: {RunAgentID: peer, Step: 4, State: racecontext.StandingsStateFailed},
	}
	state := &loopState{
		stepCount:         4,
		lastInjectionStep: 3,
		lastPeerStates:    map[uuid.UUID]racecontext.StandingsState{peer: racecontext.StandingsStateRunning},
	}

	trigger, fire := evaluateRaceContextCadence(state, snapshot, self, 3)
	if !fire || trigger != runevents.RaceStandingsTriggerPeerFailed {
		t.Fatalf("peer failure trigger = (%q, %v), want (peer_failed, true)", trigger, fire)
	}
}

func TestEvaluateRaceContextCadenceFiresOnPeerTimeout(t *testing.T) {
	self := uuid.New()
	peer := uuid.New()
	snapshot := map[uuid.UUID]racecontext.StandingsEntry{
		peer: {RunAgentID: peer, Step: 5, State: racecontext.StandingsStateTimedOut},
	}
	state := &loopState{
		stepCount:         4,
		lastInjectionStep: 3,
		lastPeerStates:    map[uuid.UUID]racecontext.StandingsState{peer: racecontext.StandingsStateRunning},
	}

	trigger, fire := evaluateRaceContextCadence(state, snapshot, self, 3)
	if !fire || trigger != runevents.RaceStandingsTriggerPeerTimedOut {
		t.Fatalf("peer timeout trigger = (%q, %v), want (peer_timed_out, true)", trigger, fire)
	}
}

func TestMaybeInjectRaceStandingsSkippedWhenDisabled(t *testing.T) {
	obs := &captureObserver{}
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(&fakeStandingsStore{
		snapshot: map[uuid.UUID]racecontext.StandingsEntry{
			uuid.New(): {State: racecontext.StandingsStateRunning, Step: 2},
		},
	})
	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.RaceContext = false
	state := &loopState{stepCount: 5}

	if err := executor.maybeInjectRaceStandings(context.Background(), execCtx, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs.injections) != 0 {
		t.Fatalf("must not inject when race_context=false; got %d injections", len(obs.injections))
	}
	if len(state.messages) != 0 {
		t.Fatalf("must not mutate messages when race_context=false")
	}
}

func TestMaybeInjectRaceStandingsSkippedBeforeStep3(t *testing.T) {
	obs := &captureObserver{}
	self := uuid.New()
	peer := uuid.New()
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(&fakeStandingsStore{
		snapshot: map[uuid.UUID]racecontext.StandingsEntry{
			peer: {RunAgentID: peer, Step: 3, State: racecontext.StandingsStateRunning},
		},
	})
	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = true
	execCtx.RunAgent.ID = self
	state := &loopState{stepCount: 2}

	if err := executor.maybeInjectRaceStandings(context.Background(), execCtx, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs.injections) != 0 {
		t.Fatalf("must not inject before step 3; got %d", len(obs.injections))
	}
}

func TestMaybeInjectRaceStandingsAppendsUserMessageAndEmitsEvent(t *testing.T) {
	obs := &captureObserver{}
	self := uuid.New()
	peer := uuid.New()
	store := &fakeStandingsStore{
		snapshot: map[uuid.UUID]racecontext.StandingsEntry{
			self: {RunAgentID: self, Model: "grok-4", Step: 3, State: racecontext.StandingsStateRunning},
			peer: {RunAgentID: peer, Model: "claude-sonnet-4-6", Step: 4, State: racecontext.StandingsStateRunning},
		},
	}
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(store)

	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = true
	execCtx.RunAgent.ID = self
	state := &loopState{stepCount: 3, startedAt: time.Now()}

	if err := executor.maybeInjectRaceStandings(context.Background(), execCtx, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs.injections) != 1 {
		t.Fatalf("expected 1 injection event, got %d", len(obs.injections))
	}
	inj := obs.injections[0]
	if inj.StepIndex != 3 {
		t.Errorf("step = %d, want 3", inj.StepIndex)
	}
	if inj.TokensAdded <= 0 {
		t.Errorf("tokens_added = %d, want > 0", inj.TokensAdded)
	}
	if inj.TriggeredBy != runevents.RaceStandingsTriggerCadence {
		t.Errorf("trigger = %q, want cadence", inj.TriggeredBy)
	}
	if inj.MinStepGap != defaultRaceContextMinStepGap {
		t.Errorf("min_gap = %d, want default %d", inj.MinStepGap, defaultRaceContextMinStepGap)
	}

	if len(state.messages) != 1 {
		t.Fatalf("expected 1 user message appended, got %d", len(state.messages))
	}
	msg := state.messages[0]
	if msg.Role != "user" {
		t.Errorf("role = %q, want user", msg.Role)
	}
	if msg.Content == "" {
		t.Errorf("content empty")
	}
	if state.lastInjectionStep != 3 {
		t.Errorf("lastInjectionStep = %d, want 3", state.lastInjectionStep)
	}
}

func TestMaybeInjectRaceStandingsCustomMinStepGap(t *testing.T) {
	obs := &captureObserver{}
	self := uuid.New()
	peer := uuid.New()
	store := &fakeStandingsStore{
		snapshot: map[uuid.UUID]racecontext.StandingsEntry{
			peer: {RunAgentID: peer, Model: "claude-sonnet-4-6", Step: 4, State: racecontext.StandingsStateRunning},
		},
	}
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(store)

	custom := int32(5)
	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = true
	execCtx.Run.RaceContextMinStepGap = &custom
	execCtx.RunAgent.ID = self
	state := &loopState{stepCount: 3}

	if err := executor.maybeInjectRaceStandings(context.Background(), execCtx, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs.injections) != 1 {
		t.Fatalf("expected 1 injection (first-fire), got %d", len(obs.injections))
	}
	if obs.injections[0].MinStepGap != 5 {
		t.Errorf("min_gap = %d, want custom 5", obs.injections[0].MinStepGap)
	}
}

func TestMaybeInjectRaceStandingsSwallowsSnapshotError(t *testing.T) {
	obs := &captureObserver{}
	self := uuid.New()
	store := &fakeStandingsStore{err: errors.New("redis disconnected")}
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(store)

	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = true
	execCtx.RunAgent.ID = self
	state := &loopState{stepCount: 5}

	if err := executor.maybeInjectRaceStandings(context.Background(), execCtx, state); err != nil {
		t.Fatalf("snapshot error must be swallowed, got: %v", err)
	}
	if len(obs.injections) != 0 {
		t.Fatalf("must not inject when snapshot errors")
	}
}

func TestMaybeInjectRaceStandingsTracksPeerStatesEvenWhenNotInjecting(t *testing.T) {
	obs := &captureObserver{}
	self := uuid.New()
	peer := uuid.New()
	store := &fakeStandingsStore{
		snapshot: map[uuid.UUID]racecontext.StandingsEntry{
			peer: {RunAgentID: peer, Step: 5, State: racecontext.StandingsStateRunning},
		},
	}
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(store)

	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = true
	execCtx.RunAgent.ID = self
	state := &loopState{
		stepCount:         4,
		lastInjectionStep: 3,
		lastPeerStates:    map[uuid.UUID]racecontext.StandingsState{peer: racecontext.StandingsStateRunning},
	}

	// step=4, lastInj=3, default gap=3 → not due; no state transition.
	if err := executor.maybeInjectRaceStandings(context.Background(), execCtx, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs.injections) != 0 {
		t.Fatalf("should not inject on non-due step without state change")
	}
	if state.lastPeerStates[peer] != racecontext.StandingsStateRunning {
		t.Errorf("lastPeerStates should refresh even on no-inject; got %+v", state.lastPeerStates)
	}
}

func TestMaybeInjectRaceStandingsSeedsMissingPeersAsNotStarted(t *testing.T) {
	obs := &captureObserver{}
	self := uuid.New()
	peer := uuid.New()
	store := &fakeStandingsStore{
		snapshot: map[uuid.UUID]racecontext.StandingsEntry{
			self: {RunAgentID: self, Model: "grok-4", Step: 3, State: racecontext.StandingsStateRunning},
		},
	}
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(store)

	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = true
	execCtx.RunAgent.ID = self
	execCtx.RunAgents = []domain.RunAgent{
		{ID: self, RunID: execCtx.Run.ID},
		{ID: peer, RunID: execCtx.Run.ID},
	}
	state := &loopState{stepCount: 3}

	if err := executor.maybeInjectRaceStandings(context.Background(), execCtx, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs.injections) != 1 {
		t.Fatalf("expected 1 injection event, got %d", len(obs.injections))
	}
	text := obs.injections[0].StandingsSnapshot
	if !strings.Contains(text, "2 agents running, 0 submitted.") {
		t.Fatalf("standings header = %q, want missing peer counted as running/not_started", text)
	}
	if !strings.Contains(text, "agent-"+peer.String()[:8]+" — not started") {
		t.Fatalf("standings snapshot = %q, want missing peer rendered as not started", text)
	}
}

func TestSyncRaceContextStepStartUpdatesStoreBeforeSnapshot(t *testing.T) {
	self := uuid.New()
	store := &fakeStandingsStore{}
	executor := NewNativeExecutor(nil, nil, &captureObserver{}).WithStandingsStore(store)

	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = true
	execCtx.RunAgent.ID = self
	execCtx.Deployment.ModelAlias = &repository.ModelAliasExecutionContext{
		ModelCatalogEntry: repository.ModelCatalogEntryExecutionContext{ProviderModelID: "gpt-5"},
	}
	state := &loopState{stepCount: 3}

	executor.syncRaceContextStepStart(context.Background(), execCtx, state)

	entry := store.snapshot[self]
	if entry.RunAgentID != self {
		t.Fatalf("synced run_agent_id = %s, want %s", entry.RunAgentID, self)
	}
	if entry.Step != 3 || entry.State != racecontext.StandingsStateRunning {
		t.Fatalf("synced standings = step %d state %q, want step 3 running", entry.Step, entry.State)
	}
	if entry.Model != "gpt-5" {
		t.Fatalf("synced model = %q, want gpt-5", entry.Model)
	}
	if entry.StartedAt == nil {
		t.Fatalf("synced standings should set started_at")
	}
}

// Compile-time check that the captureObserver still satisfies the full
// Observer interface after slice 7. Guards against accidental interface
// drift in future refactors.
var _ Observer = (*captureObserver)(nil)
var _ = provider.Request{}
