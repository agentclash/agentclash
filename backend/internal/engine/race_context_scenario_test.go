package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/racecontext"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

// programmableStandingsStore lets tests change the snapshot between
// iterations, simulating peers that advance, submit, or fail. All reads
// return the current snapshot; Update is a no-op (reads are the only
// thing the executor's injection path uses).
type programmableStandingsStore struct {
	mu       sync.Mutex
	snapshot map[uuid.UUID]racecontext.StandingsEntry
}

func (p *programmableStandingsStore) set(snapshot map[uuid.UUID]racecontext.StandingsEntry) {
	p.mu.Lock()
	p.snapshot = snapshot
	p.mu.Unlock()
}

func (p *programmableStandingsStore) Update(context.Context, uuid.UUID, racecontext.StandingsEntry) error {
	return nil
}

func (p *programmableStandingsStore) Snapshot(context.Context, uuid.UUID) (map[uuid.UUID]racecontext.StandingsEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Return a copy so callers can't mutate our state.
	out := make(map[uuid.UUID]racecontext.StandingsEntry, len(p.snapshot))
	for k, v := range p.snapshot {
		out[k] = v
	}
	return out, nil
}

func (p *programmableStandingsStore) Close() error { return nil }

// simulateLoop advances the executor's injection predicate across
// totalSteps iterations. Between each step the caller can mutate the
// store snapshot. Returns the captured injections in order.
func simulateLoop(t *testing.T, executor NativeExecutor, store *programmableStandingsStore, execCtx repository.RunAgentExecutionContext, totalSteps int, perStep func(step int, snapshot *programmableStandingsStore)) []StandingsInjection {
	t.Helper()
	state := &loopState{}
	var captured []StandingsInjection
	obs, ok := executor.observer.(*captureObserver)
	if !ok {
		t.Fatalf("simulateLoop requires captureObserver; got %T", executor.observer)
	}

	for step := 1; step <= totalSteps; step++ {
		state.stepCount = step
		if perStep != nil {
			perStep(step, store)
		}
		if err := executor.maybeInjectRaceStandings(context.Background(), execCtx, state); err != nil {
			t.Fatalf("step %d: injection returned error: %v", step, err)
		}
	}
	captured = append(captured, obs.injections...)
	return captured
}

// TestRaceContextScenarioAcrossManySteps drives the executor's injection
// path through a realistic 12-step race with 3 peers. Verifies the full
// sequence: cadence fires at step 3, step 6, and so on; a peer
// submission triggers an out-of-cadence injection; failed peers don't
// fire additional events once rendered.
func TestRaceContextScenarioAcrossManySteps(t *testing.T) {
	self := uuid.New()
	peerA := uuid.New()
	peerB := uuid.New()
	peerC := uuid.New()

	store := &programmableStandingsStore{}
	// Initial snapshot: all 4 agents running.
	store.set(map[uuid.UUID]racecontext.StandingsEntry{
		self:  {RunAgentID: self, Model: "grok-4", Step: 1, State: racecontext.StandingsStateRunning},
		peerA: {RunAgentID: peerA, Model: "claude-sonnet-4-6", Step: 1, State: racecontext.StandingsStateRunning},
		peerB: {RunAgentID: peerB, Model: "gpt-5", Step: 1, State: racecontext.StandingsStateRunning},
		peerC: {RunAgentID: peerC, Model: "gemini-2.5-pro", Step: 1, State: racecontext.StandingsStateRunning},
	})

	obs := &captureObserver{}
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(store)

	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = true
	execCtx.RunAgent.ID = self

	injections := simulateLoop(t, executor, store, execCtx, 12, func(step int, s *programmableStandingsStore) {
		switch step {
		case 5:
			// Peer A submits between step 4 and step 5.
			s.set(map[uuid.UUID]racecontext.StandingsEntry{
				self:  {RunAgentID: self, Model: "grok-4", Step: step, State: racecontext.StandingsStateRunning},
				peerA: {RunAgentID: peerA, Model: "claude-sonnet-4-6", Step: 8, State: racecontext.StandingsStateSubmitted},
				peerB: {RunAgentID: peerB, Model: "gpt-5", Step: 4, State: racecontext.StandingsStateRunning},
				peerC: {RunAgentID: peerC, Model: "gemini-2.5-pro", Step: 4, State: racecontext.StandingsStateRunning},
			})
		case 9:
			// Peer B fails at step 9.
			s.set(map[uuid.UUID]racecontext.StandingsEntry{
				self:  {RunAgentID: self, Model: "grok-4", Step: step, State: racecontext.StandingsStateRunning},
				peerA: {RunAgentID: peerA, Model: "claude-sonnet-4-6", Step: 8, State: racecontext.StandingsStateSubmitted},
				peerB: {RunAgentID: peerB, Model: "gpt-5", Step: 7, State: racecontext.StandingsStateFailed},
				peerC: {RunAgentID: peerC, Model: "gemini-2.5-pro", Step: 6, State: racecontext.StandingsStateRunning},
			})
		}
	})

	// Expected fire sequence:
	//  step 3 → cadence (first-fire)
	//  step 5 → peer_submitted (A) — overrides cadence gap
	//  step 8 → cadence (5 + 3 = 8)
	//  step 9 → peer_failed (B) — overrides cadence gap
	//  step 12 → cadence (9 + 3 = 12)
	if got := len(injections); got != 5 {
		t.Fatalf("injection count = %d, want 5; injections: %+v", got, injections)
	}

	type want struct {
		step    int
		trigger runevents.RaceStandingsTrigger
	}
	expected := []want{
		{3, runevents.RaceStandingsTriggerCadence},
		{5, runevents.RaceStandingsTriggerPeerSubmitted},
		{8, runevents.RaceStandingsTriggerCadence},
		{9, runevents.RaceStandingsTriggerPeerFailed},
		{12, runevents.RaceStandingsTriggerCadence},
	}
	for i, exp := range expected {
		if injections[i].StepIndex != exp.step {
			t.Errorf("injection[%d] step = %d, want %d", i, injections[i].StepIndex, exp.step)
		}
		if injections[i].TriggeredBy != exp.trigger {
			t.Errorf("injection[%d] trigger = %q, want %q", i, injections[i].TriggeredBy, exp.trigger)
		}
	}
}

// TestRaceContextScenarioByteIdenticalWhenDisabled guards the most
// important backwards-compat property: with race_context=false, the
// injection path never mutates the message list nor emits any event,
// even when a store is attached and full of peer data. Breaking this
// would regress every pre-#400 run.
func TestRaceContextScenarioByteIdenticalWhenDisabled(t *testing.T) {
	self := uuid.New()
	peer := uuid.New()
	store := &programmableStandingsStore{
		snapshot: map[uuid.UUID]racecontext.StandingsEntry{
			self: {RunAgentID: self, Step: 5, State: racecontext.StandingsStateRunning},
			peer: {RunAgentID: peer, Step: 7, State: racecontext.StandingsStateSubmitted},
		},
	}

	obs := &captureObserver{}
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(store)

	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = false // ← the switch
	execCtx.RunAgent.ID = self

	state := &loopState{}
	for step := 1; step <= 15; step++ {
		state.stepCount = step
		if err := executor.maybeInjectRaceStandings(context.Background(), execCtx, state); err != nil {
			t.Fatalf("step %d: unexpected error: %v", step, err)
		}
	}

	if len(state.messages) != 0 {
		t.Errorf("message list mutated while race_context=false; appended %d", len(state.messages))
	}
	if len(obs.injections) != 0 {
		t.Errorf("events emitted while race_context=false; got %d", len(obs.injections))
	}
	if state.lastInjectionStep != 0 {
		t.Errorf("lastInjectionStep advanced while race_context=false; got %d", state.lastInjectionStep)
	}
	if state.lastPeerStates != nil {
		t.Errorf("peer state tracking happened while race_context=false")
	}
}

// TestRaceContextScenarioCustomCadencePerRun verifies the runtime-config
// path end to end: overriding race_context_min_step_gap changes the
// cadence without any code changes, and the override surfaces on every
// emitted event so replayers can explain what cadence was active.
func TestRaceContextScenarioCustomCadencePerRun(t *testing.T) {
	self := uuid.New()
	peer := uuid.New()
	store := &programmableStandingsStore{
		snapshot: map[uuid.UUID]racecontext.StandingsEntry{
			self: {RunAgentID: self, Step: 1, State: racecontext.StandingsStateRunning},
			peer: {RunAgentID: peer, Step: 1, State: racecontext.StandingsStateRunning},
		},
	}

	obs := &captureObserver{}
	executor := NewNativeExecutor(nil, nil, obs).WithStandingsStore(store)

	// Custom cadence = 5 steps (wider than the default 3).
	custom := int32(5)
	execCtx := repository.RunAgentExecutionContext{}
	execCtx.Run.ID = uuid.New()
	execCtx.Run.RaceContext = true
	execCtx.Run.RaceContextMinStepGap = &custom
	execCtx.RunAgent.ID = self

	injections := simulateLoop(t, executor, store, execCtx, 15, nil)

	// With cadence=5 and first-fire at step 3, expect: 3, 8, 13.
	if got := len(injections); got != 3 {
		t.Fatalf("injection count = %d, want 3 (cadence=5 over 15 steps); injections: %+v", got, injections)
	}
	expectedSteps := []int{3, 8, 13}
	for i, want := range expectedSteps {
		if injections[i].StepIndex != want {
			t.Errorf("injection[%d] step = %d, want %d", i, injections[i].StepIndex, want)
		}
		if injections[i].MinStepGap != 5 {
			t.Errorf("injection[%d] min_step_gap = %d, want 5", i, injections[i].MinStepGap)
		}
	}
}
