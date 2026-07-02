package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/simulator"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/runevents"
)

type MultiTurnObserverFactory func(executionContext repository.RunAgentExecutionContext) (engine.Observer, error)

// MultiTurnRunEventObserver delegates native inner-loop events to a native
// observer and records outer-loop multi-turn conversation events.
type MultiTurnRunEventObserver struct {
	native           *NativeRunEventObserver
	recorder         RunEventRecorder
	executionContext repository.RunAgentExecutionContext

	mu           sync.Mutex
	turnEventSeq int64
}

func NewMultiTurnRunEventObserverFactory(recorder RunEventRecorder) MultiTurnObserverFactory {
	nativeFactory := NewNativeRunEventObserverFactory(recorder)
	return func(executionContext repository.RunAgentExecutionContext) (engine.Observer, error) {
		if recorder == nil {
			return engine.NoopObserver{}, nil
		}
		native, err := nativeFactory(executionContext)
		if err != nil {
			return nil, err
		}
		nativeObserver, ok := native.(*NativeRunEventObserver)
		if !ok {
			return native, nil
		}
		return &MultiTurnRunEventObserver{
			native:           nativeObserver,
			recorder:         recorder,
			executionContext: executionContext,
		}, nil
	}
}

func (o *MultiTurnRunEventObserver) OnStepStart(ctx context.Context, step int) error {
	return o.native.OnStepStart(ctx, step)
}

func (o *MultiTurnRunEventObserver) OnProviderCall(ctx context.Context, request provider.Request) error {
	return o.native.OnProviderCall(ctx, request)
}

func (o *MultiTurnRunEventObserver) OnProviderOutput(ctx context.Context, request provider.Request, delta provider.StreamDelta) error {
	return o.native.OnProviderOutput(ctx, request, delta)
}

func (o *MultiTurnRunEventObserver) OnProviderResponse(ctx context.Context, response provider.Response) error {
	return o.native.OnProviderResponse(ctx, response)
}

func (o *MultiTurnRunEventObserver) OnToolExecution(ctx context.Context, record engine.ToolExecutionRecord) error {
	return o.native.OnToolExecution(ctx, record)
}

func (o *MultiTurnRunEventObserver) OnStepEnd(ctx context.Context, step int) error {
	return o.native.OnStepEnd(ctx, step)
}

func (o *MultiTurnRunEventObserver) OnPostExecutionVerification(ctx context.Context, results []engine.PostExecutionVerificationResult) error {
	return o.native.OnPostExecutionVerification(ctx, results)
}

func (o *MultiTurnRunEventObserver) OnStandingsInjected(ctx context.Context, injection engine.StandingsInjection) error {
	return o.native.OnStandingsInjected(ctx, injection)
}

func (o *MultiTurnRunEventObserver) OnRunComplete(ctx context.Context, result engine.Result) error {
	return o.native.OnRunComplete(ctx, result)
}

func (o *MultiTurnRunEventObserver) OnRunFailure(ctx context.Context, err error) error {
	return o.native.OnRunFailure(ctx, err)
}

func (o *MultiTurnRunEventObserver) RecordTurnUserMessage(ctx context.Context, turnIndex int, phaseID, actor, content string) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	turn := turnIndex
	eventActor := engine.ActorForRecorder(actor)
	return o.recordTurnEvent(ctx, runevents.EventTypeTurnUserMessage, map[string]any{
		"content":    content,
		"actor":      eventActor,
		"phase_id":   phaseID,
		"turn_index": turnIndex,
	}, runevents.SummaryMetadata{
		Status:        "running",
		TurnIndex:     &turn,
		PhaseID:       phaseID,
		Actor:         eventActor,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *MultiTurnRunEventObserver) RecordTurnUserSimulated(ctx context.Context, turnIndex int, phaseID string, metadata simulator.Metadata) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	turn := turnIndex
	return o.recordTurnEvent(ctx, runevents.EventTypeTurnUserSimulated, map[string]any{
		"provider_key":      metadata.ProviderKey,
		"provider_model_id": metadata.ProviderModelID,
		"phase_id":          metadata.PhaseID,
		"turn_index":        turnIndex,
	}, runevents.SummaryMetadata{
		Status:        "running",
		TurnIndex:     &turn,
		PhaseID:       firstNonEmpty(metadata.PhaseID, phaseID),
		Actor:         runevents.ConversationActorLLM,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *MultiTurnRunEventObserver) RecordTurnAwaitingHuman(ctx context.Context, turnIndex int, phaseID, promptHint string) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	turn := turnIndex
	return o.recordTurnEvent(ctx, runevents.EventTypeTurnAwaitingHuman, map[string]any{
		"phase_id":    phaseID,
		"prompt_hint": promptHint,
		"turn_index":  turnIndex,
	}, runevents.SummaryMetadata{
		Status:        "running",
		TurnIndex:     &turn,
		PhaseID:       phaseID,
		Actor:         runevents.ConversationActorHuman,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (o *MultiTurnRunEventObserver) RecordTurnAssistantMessage(ctx context.Context, turnIndex int, phaseID, content string) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	turn := turnIndex
	return o.recordTurnEvent(ctx, runevents.EventTypeTurnAssistantMessage, map[string]any{
		"content":    content,
		"phase_id":   phaseID,
		"turn_index": turnIndex,
	}, runevents.SummaryMetadata{
		Status:        "running",
		TurnIndex:     &turn,
		PhaseID:       phaseID,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *MultiTurnRunEventObserver) RecordTurnCompleted(ctx context.Context, turnIndex int, phaseID, actor string, mismatch bool) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	turn := turnIndex
	mismatchCopy := mismatch
	return o.recordTurnEvent(ctx, runevents.EventTypeTurnCompleted, map[string]any{
		"turn_index": turnIndex,
		"mismatch":   mismatch,
	}, runevents.SummaryMetadata{
		Status:        "running",
		TurnIndex:     &turn,
		PhaseID:       phaseID,
		Actor:         actor,
		Mismatch:      &mismatchCopy,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *MultiTurnRunEventObserver) RecordConversationCompleted(ctx context.Context, summary engine.MultiTurnConversationSummary) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	payload, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal conversation.completed payload: %w", err)
	}
	return o.recordTurnEvent(ctx, runevents.EventTypeConversationCompleted, jsonRawToMap(payload), runevents.SummaryMetadata{
		Status:        "running",
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *MultiTurnRunEventObserver) ensureRunStarted(ctx context.Context) error {
	return o.native.ensureRunStarted(ctx)
}

func (o *MultiTurnRunEventObserver) recordTurnEvent(ctx context.Context, eventType runevents.Type, payload map[string]any, summary runevents.SummaryMetadata) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal multi_turn event payload: %w", err)
	}

	summary.IdempotencyKey = o.nextTurnEventID(eventType)
	_, err = o.recorder.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       summary.IdempotencyKey,
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         o.executionContext.Run.ID,
			RunAgentID:    o.executionContext.RunAgent.ID,
			EventType:     eventType,
			Source:        runevents.SourceMultiTurnEngine,
			OccurredAt:    time.Now().UTC(),
			Payload:       payloadJSON,
			Summary:       summary,
		},
	})
	if err != nil {
		return fmt.Errorf("record multi_turn run event: %w", err)
	}
	return nil
}

func (o *MultiTurnRunEventObserver) nextTurnEventID(eventType runevents.Type) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.turnEventSeq++
	return fmt.Sprintf("multi_turn:%s:%s:%d", o.executionContext.RunAgent.ID.String(), eventType, o.turnEventSeq)
}

func jsonRawToMap(payload json.RawMessage) map[string]any {
	if len(payload) == 0 {
		return map[string]any{}
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return map[string]any{"raw": string(payload)}
	}
	return decoded
}

// NewBufferedMultiTurnObserverFactory wraps the multi-turn observer factory with buffering.
func NewBufferedMultiTurnObserverFactory(recorder RunEventRecorder) MultiTurnObserverFactory {
	innerFactory := NewMultiTurnRunEventObserverFactory(recorder)
	return func(executionContext repository.RunAgentExecutionContext) (engine.Observer, error) {
		inner, err := innerFactory(executionContext)
		if err != nil {
			return nil, err
		}
		if _, ok := inner.(engine.NoopObserver); ok {
			return inner, nil
		}
		return NewBufferedObserver(inner), nil
	}
}
