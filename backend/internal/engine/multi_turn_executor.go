package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/simulator"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/agentclash/agentclash/runtime/runner"
	"github.com/google/uuid"
)

const StopReasonMaxTurns StopReason = "max_turns"
const StopReasonSimulatorError StopReason = "simulator_error"
const StopReasonHumanTurnTimeout StopReason = "human_turn_timeout"

// MultiTurnConversationSummary is emitted on conversation.completed.
type MultiTurnConversationSummary struct {
	TurnCount      int      `json:"turn_count"`
	ActorsUsed     []string `json:"actors_used"`
	MismatchTurns  []int    `json:"mismatch_turns"`
	HumanTurnCount int      `json:"human_turn_count"`
	StopReason     string   `json:"stop_reason,omitempty"`
	FinalOutput    string   `json:"final_output,omitempty"`
}

// MultiTurnEventRecorder records outer-loop turn events.
type MultiTurnEventRecorder interface {
	RecordTurnUserMessage(ctx context.Context, turnIndex int, phaseID, actor, content string) error
	RecordTurnUserSimulated(ctx context.Context, turnIndex int, phaseID string, metadata simulator.Metadata) error
	RecordTurnAssistantMessage(ctx context.Context, turnIndex int, phaseID, content string) error
	RecordTurnCompleted(ctx context.Context, turnIndex int, phaseID, actor string, mismatch bool) error
	RecordTurnAwaitingHuman(ctx context.Context, turnIndex int, phaseID, promptHint string) error
	RecordConversationCompleted(ctx context.Context, summary MultiTurnConversationSummary) error
}

type noopMultiTurnEventRecorder struct{}

func (noopMultiTurnEventRecorder) RecordTurnUserMessage(context.Context, int, string, string, string) error {
	return nil
}
func (noopMultiTurnEventRecorder) RecordTurnUserSimulated(context.Context, int, string, simulator.Metadata) error {
	return nil
}
func (noopMultiTurnEventRecorder) RecordTurnAssistantMessage(context.Context, int, string, string) error {
	return nil
}
func (noopMultiTurnEventRecorder) RecordTurnCompleted(context.Context, int, string, string, bool) error {
	return nil
}
func (noopMultiTurnEventRecorder) RecordTurnAwaitingHuman(context.Context, int, string, string) error {
	return nil
}
func (noopMultiTurnEventRecorder) RecordConversationCompleted(context.Context, MultiTurnConversationSummary) error {
	return nil
}

func multiTurnRecorder(observer Observer) MultiTurnEventRecorder {
	if recorder, ok := observer.(MultiTurnEventRecorder); ok {
		return recorder
	}
	return noopMultiTurnEventRecorder{}
}

// MultiTurnExecutor runs hybrid user-simulator phases with a native inner loop per turn.
type MultiTurnExecutor struct {
	nativeExecutor NativeExecutor
	observer       Observer
	simulator      TurnMessageGenerator
	humanGate      HumanTurnGate
}

func NewMultiTurnExecutor(nativeExecutor NativeExecutor, observer Observer) MultiTurnExecutor {
	if observer == nil {
		observer = NoopObserver{}
	}
	return MultiTurnExecutor{
		nativeExecutor: nativeExecutor,
		observer:       observer,
		humanGate:      NoopHumanTurnGate(),
	}
}

func (e MultiTurnExecutor) WithSecretsLookup(lookup SecretsLookup) MultiTurnExecutor {
	e.nativeExecutor = e.nativeExecutor.WithSecretsLookup(lookup)
	return e
}

func (e MultiTurnExecutor) WithAssetLoader(loader AssetLoader) MultiTurnExecutor {
	e.nativeExecutor = e.nativeExecutor.WithAssetLoader(loader)
	return e
}

func (e MultiTurnExecutor) WithSimulator(generator TurnMessageGenerator) MultiTurnExecutor {
	e.simulator = generator
	return e
}

func (e MultiTurnExecutor) WithHumanTurnGate(gate HumanTurnGate) MultiTurnExecutor {
	if gate != nil {
		e.humanGate = gate
	}
	return e
}

func (e MultiTurnExecutor) Execute(ctx context.Context, executionContext runner.ExecutionContext) (result Result, err error) {
	recorder := multiTurnRecorder(e.observer)
	defer runner.FinishWithObserver(ctx, e.observer, &result, &err, runner.TerminalObserverMessages{
		Failure:    "record multi_turn terminal failure event",
		Completion: "record multi_turn terminal completion event",
	})

	spec, err := userSimulatorForExecution(executionContext)
	if err != nil {
		return Result{}, err
	}

	conversation, cleanup, err := e.nativeExecutor.BeginConversation(ctx, executionContext)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()

	summary, stopReason, execErr := e.runHybridConversation(ctx, executionContext, spec, conversation, recorder)
	if execErr != nil {
		return Result{}, execErr
	}
	if finalizeErr := conversation.Finalize(ctx); finalizeErr != nil {
		return Result{}, finalizeErr
	}
	if err := recorder.RecordConversationCompleted(ctx, summary); err != nil {
		return Result{}, NewFailure(StopReasonObserverError, "record conversation.completed event", err)
	}

	return Result{
		FinalOutput:   summary.FinalOutput,
		StopReason:    stopReason,
		StepCount:     conversation.AggregateSteps(),
		ToolCallCount: conversation.AggregateToolCalls(),
		Usage:         conversation.AggregateUsage(),
	}, nil
}

func (e MultiTurnExecutor) runHybridConversation(
	ctx context.Context,
	executionContext runner.ExecutionContext,
	spec *challengepack.UserSimulatorSpec,
	conversation *NativeConversation,
	recorder MultiTurnEventRecorder,
) (MultiTurnConversationSummary, StopReason, error) {
	maxTurns := int(spec.MaxTurns)
	if maxTurns <= 0 {
		maxTurns = 50
	}

	state := hybridConversationState{
		turnIndex:      0,
		lastMismatch:   false,
		finalOutput:    "",
		mismatchTurns:  []int{},
		seenActors:     map[string]struct{}{},
		humanTurnCount: 0,
		transcript:     []simulator.TranscriptTurn{},
	}

	for _, phase := range spec.Phases {
		if !shouldRunPhase(phase, state.lastMismatch) {
			continue
		}
		state.seenActors[phase.Actor] = struct{}{}

		switch phase.Actor {
		case challengepack.UserSimulatorActorScripted:
			if err := e.runScriptedPhase(ctx, executionContext, phase, maxTurns, conversation, recorder, &state); err != nil {
				return MultiTurnConversationSummary{}, "", err
			}
		case challengepack.UserSimulatorActorLLM:
			if err := e.runLLMPhase(ctx, executionContext, phase, maxTurns, conversation, recorder, &state); err != nil {
				return MultiTurnConversationSummary{}, "", err
			}
		case challengepack.UserSimulatorActorHuman:
			if err := e.runHumanPhase(ctx, executionContext, phase, maxTurns, conversation, recorder, &state); err != nil {
				return MultiTurnConversationSummary{}, "", err
			}
		}

		if state.stopReason != "" {
			summary := buildConversationSummary(state.turnIndex, actorKeys(state.seenActors), state.mismatchTurns, state.finalOutput, state.humanTurnCount, state.stopReason)
			return summary, state.stopReason, nil
		}
	}

	summary := buildConversationSummary(state.turnIndex, actorKeys(state.seenActors), state.mismatchTurns, state.finalOutput, state.humanTurnCount, StopReasonCompleted)
	return summary, StopReasonCompleted, nil
}

type hybridConversationState struct {
	turnIndex      int
	lastMismatch   bool
	finalOutput    string
	mismatchTurns  []int
	seenActors     map[string]struct{}
	humanTurnCount int
	transcript     []simulator.TranscriptTurn
	stopReason     StopReason
}

func (e MultiTurnExecutor) runScriptedPhase(
	ctx context.Context,
	executionContext runner.ExecutionContext,
	phase challengepack.UserSimulatorPhase,
	maxTurns int,
	conversation *NativeConversation,
	recorder MultiTurnEventRecorder,
	state *hybridConversationState,
) error {
	for _, turn := range phase.Turns {
		if state.turnIndex >= maxTurns {
			state.stopReason = StopReasonMaxTurns
			return nil
		}
		message, err := renderScriptedTurnMessage(turn.Message, executionContext)
		if err != nil {
			return err
		}
		mismatch, err := e.executeUserTurn(ctx, phase.ID, challengepack.UserSimulatorActorScripted, message, turn.Expects, conversation, recorder, state)
		if err != nil {
			return err
		}
		state.lastMismatch = mismatch
	}
	return nil
}

func (e MultiTurnExecutor) runLLMPhase(
	ctx context.Context,
	executionContext runner.ExecutionContext,
	phase challengepack.UserSimulatorPhase,
	maxTurns int,
	conversation *NativeConversation,
	recorder MultiTurnEventRecorder,
	state *hybridConversationState,
) error {
	if e.simulator == nil {
		return provider.NewFailure("", provider.FailureCodeInvalidRequest, "llm simulator is not configured", false, nil)
	}
	providerKey, providerAccountID, credentialRef, model, err := simulator.ResolveTarget(executionContext)
	if err != nil {
		return err
	}
	// Pack-author override: an LLM phase may pin a chat-compatible model id
	// when the deployment runs on a model that only supports /v1/responses
	// (e.g. o-series reasoning models). Provider and credentials still come
	// from the deployment, so the override must name a model the deployment's
	// provider serves.
	if override := strings.TrimSpace(phase.Model); override != "" {
		model = override
	}

	phaseMax := int(phase.MaxTurns)
	if phaseMax <= 0 {
		phaseMax = maxTurns
	}
	phaseTurns := 0

	// The simulator issues its own provider call (separate from the agent
	// loop in conversation.RunTurn) and therefore needs the conversation's
	// run context — which has workspace secrets injected. The outer ctx
	// here does NOT carry those secrets, so passing it would cause
	// workspace-secret:// credential references to fail to resolve.
	simulatorCtx := conversation.Context()

	for phaseTurns < phaseMax && state.turnIndex < maxTurns {
		message, metadata, err := e.simulator.GenerateUserMessage(simulatorCtx, simulator.Input{
			Persona:           phase.Persona,
			Transcript:        append([]simulator.TranscriptTurn(nil), state.transcript...),
			CasePayload:       casePayloadMap(executionContext),
			PhaseID:           phase.ID,
			ProviderKey:       providerKey,
			ProviderAccountID: providerAccountID,
			CredentialRef:     credentialRef,
			Model:             model,
		})
		if err != nil {
			return NewFailure(StopReasonSimulatorError, "generate llm user message", err)
		}

		if err := recorder.RecordTurnUserSimulated(ctx, state.turnIndex, phase.ID, metadata); err != nil {
			return NewFailure(StopReasonObserverError, "record turn.user.simulated event", err)
		}

		mismatch, err := e.executeUserTurn(ctx, phase.ID, challengepack.UserSimulatorActorLLM, message, nil, conversation, recorder, state)
		if err != nil {
			return err
		}
		state.lastMismatch = mismatch
		phaseTurns++

		if phaseUntilSatisfied(phase.Until, untilEvalContext{
			AssistantText: state.finalOutput,
			UserMessage:   message,
			TurnIndex:     state.turnIndex - 1,
			PhaseTurns:    phaseTurns,
			MaxPhaseTurns: phaseMax,
		}) {
			break
		}
	}
	return nil
}

func (e MultiTurnExecutor) runHumanPhase(
	ctx context.Context,
	executionContext runner.ExecutionContext,
	phase challengepack.UserSimulatorPhase,
	maxTurns int,
	conversation *NativeConversation,
	recorder MultiTurnEventRecorder,
	state *hybridConversationState,
) error {
	if state.turnIndex >= maxTurns {
		state.stopReason = StopReasonMaxTurns
		return nil
	}

	promptHint := strings.TrimSpace(phase.Persona)
	if err := recorder.RecordTurnAwaitingHuman(ctx, state.turnIndex, phase.ID, promptHint); err != nil {
		return NewFailure(StopReasonObserverError, "record turn.awaiting_human event", err)
	}

	timeout := time.Duration(phase.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}

	message, err := e.humanGate.WaitForHumanTurn(ctx, HumanTurnRequest{
		RunAgentID: executionContext.RunAgent.ID,
		TurnIndex:  state.turnIndex,
		PhaseID:    phase.ID,
		PromptHint: promptHint,
		Timeout:    timeout,
	})
	if err != nil {
		if errors.Is(err, ErrHumanTurnTimeout) {
			switch challengepack.NormalizeHumanOnTimeout(phase.OnTimeout) {
			case challengepack.UserSimulatorHumanOnTimeoutFail:
				return NewFailure(StopReasonHumanTurnTimeout, "human turn timed out", err)
			default:
				state.stopReason = StopReasonHumanTurnTimeout
				return nil
			}
		}
		return err
	}

	state.humanTurnCount++
	_, err = e.executeUserTurn(ctx, phase.ID, challengepack.UserSimulatorActorHuman, message, nil, conversation, recorder, state)
	return err
}

func (e MultiTurnExecutor) executeUserTurn(
	ctx context.Context,
	phaseID string,
	actor string,
	message string,
	expects []challengepack.CaseExpectation,
	conversation *NativeConversation,
	recorder MultiTurnEventRecorder,
	state *hybridConversationState,
) (bool, error) {
	if err := recorder.RecordTurnUserMessage(ctx, state.turnIndex, phaseID, actor, message); err != nil {
		return false, NewFailure(StopReasonObserverError, "record turn.user.message event", err)
	}

	turnResult, err := conversation.RunTurn(ctx, message)
	if err != nil {
		return false, err
	}
	state.finalOutput = turnResult.AssistantText

	if err := recorder.RecordTurnAssistantMessage(ctx, state.turnIndex, phaseID, turnResult.AssistantText); err != nil {
		return false, NewFailure(StopReasonObserverError, "record turn.assistant.message event", err)
	}

	mismatch := evaluateTurnExpects(turnResult.AssistantText, expects)
	if mismatch {
		state.mismatchTurns = append(state.mismatchTurns, state.turnIndex)
	}
	if err := recorder.RecordTurnCompleted(ctx, state.turnIndex, phaseID, actor, mismatch); err != nil {
		return false, NewFailure(StopReasonObserverError, "record turn.completed event", err)
	}

	state.transcript = append(state.transcript, simulator.TranscriptTurn{Actor: actor, Content: message, PhaseID: phaseID})
	state.transcript = append(state.transcript, simulator.TranscriptTurn{Actor: "assistant", Content: turnResult.AssistantText, PhaseID: phaseID})
	state.turnIndex++
	return mismatch, nil
}

func userSimulatorForExecution(executionContext runner.ExecutionContext) (*challengepack.UserSimulatorSpec, error) {
	if executionContext.ChallengeInputSet == nil || len(executionContext.ChallengeInputSet.Cases) == 0 {
		return nil, provider.NewFailure("", provider.FailureCodeInvalidRequest, "multi_turn execution requires a challenge case", false, nil)
	}
	first := executionContext.ChallengeInputSet.Cases[0]
	if first.UserSimulator == nil {
		return nil, provider.NewFailure("", provider.FailureCodeInvalidRequest, "multi_turn execution requires user_simulator on the case", false, nil)
	}
	return challengepack.CloneUserSimulatorSpec(first.UserSimulator), nil
}

func renderScriptedTurnMessage(template string, executionContext runner.ExecutionContext) (string, error) {
	ctx := caseTemplateContextForExecution(executionContext)
	rendered, err := challengepack.RenderCaseTemplate(template, ctx)
	if err != nil {
		rendered = challengepack.RenderCaseTemplateLenient(template, ctx)
	}
	if strings.TrimSpace(rendered) == "" {
		return "", provider.NewFailure("", provider.FailureCodeInvalidRequest, "scripted turn message rendered empty", false, fmt.Errorf("empty scripted turn message"))
	}
	return rendered, nil
}

func buildConversationSummary(turnCount int, actorsUsed []string, mismatchTurns []int, finalOutput string, humanTurnCount int, stopReason StopReason) MultiTurnConversationSummary {
	return MultiTurnConversationSummary{
		TurnCount:      turnCount,
		ActorsUsed:     append([]string(nil), actorsUsed...),
		MismatchTurns:  append([]int(nil), mismatchTurns...),
		HumanTurnCount: humanTurnCount,
		StopReason:     string(stopReason),
		FinalOutput:    finalOutput,
	}
}

func actorKeys(seen map[string]struct{}) []string {
	out := make([]string, 0, len(seen))
	for actor := range seen {
		out = append(out, actor)
	}
	return out
}

func casePayloadMap(executionContext runner.ExecutionContext) map[string]any {
	if executionContext.ChallengeInputSet == nil || len(executionContext.ChallengeInputSet.Cases) == 0 {
		return nil
	}
	payload := executionContext.ChallengeInputSet.Cases[0].Payload
	if len(payload) == 0 {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return map[string]any{"raw": string(payload)}
	}
	return decoded
}

// RunAgentIDForHumanTurn returns the run agent id used for human turn gates.
func RunAgentIDForHumanTurn(executionContext runner.ExecutionContext) uuid.UUID {
	return executionContext.RunAgent.ID
}

// ActorForRecorder maps pack actor strings to run event actors.
func ActorForRecorder(actor string) string {
	switch strings.TrimSpace(actor) {
	case challengepack.UserSimulatorActorScripted:
		return runevents.ConversationActorScripted
	case challengepack.UserSimulatorActorLLM:
		return runevents.ConversationActorLLM
	case challengepack.UserSimulatorActorHuman:
		return runevents.ConversationActorHuman
	default:
		return strings.TrimSpace(actor)
	}
}
