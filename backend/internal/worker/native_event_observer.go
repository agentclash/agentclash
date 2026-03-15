package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
)

type NativeObserverFactory func(executionContext repository.RunAgentExecutionContext) (engine.Observer, error)

type RunEventRecorder interface {
	RecordRunEvent(ctx context.Context, params repository.RecordRunEventParams) (repository.RunEvent, error)
}

type NativeRunEventObserver struct {
	recorder         RunEventRecorder
	executionContext repository.RunAgentExecutionContext

	mu              sync.Mutex
	stepIndex       int
	eventIDSequence int64
	runStarted      bool
}

func NewNativeRunEventObserverFactory(recorder RunEventRecorder) NativeObserverFactory {
	return func(executionContext repository.RunAgentExecutionContext) (engine.Observer, error) {
		if recorder == nil {
			return engine.NoopObserver{}, nil
		}
		return &NativeRunEventObserver{
			recorder:         recorder,
			executionContext: executionContext,
		}, nil
	}
}

func (o *NativeRunEventObserver) OnStepStart(ctx context.Context, step int) error {
	o.mu.Lock()
	o.stepIndex = step
	o.mu.Unlock()

	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	return o.recordEvent(ctx, runevents.EventTypeSystemStepStarted, map[string]any{
		"step_index": step,
	}, runevents.SummaryMetadata{
		Status:        "running",
		StepIndex:     step,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *NativeRunEventObserver) OnProviderCall(ctx context.Context, request provider.Request) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	return o.recordEvent(ctx, runevents.EventTypeModelCallStarted, map[string]any{
		"provider_key":          request.ProviderKey,
		"provider_account_id":   request.ProviderAccountID,
		"model":                 request.Model,
		"trace_mode":            request.TraceMode,
		"step_timeout_ms":       request.StepTimeout.Milliseconds(),
		"message_count":         len(request.Messages),
		"tool_definition_count": len(request.Tools),
		"metadata":              normalizeJSON(request.Metadata),
	}, runevents.SummaryMetadata{
		Status:          "running",
		StepIndex:       o.currentStep(),
		ProviderKey:     request.ProviderKey,
		ProviderModelID: request.Model,
		EvidenceLevel:   runevents.EvidenceLevelNativeStructured,
	})
}

func (o *NativeRunEventObserver) OnProviderResponse(ctx context.Context, response provider.Response) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	return o.recordEvent(ctx, runevents.EventTypeModelCallCompleted, map[string]any{
		"provider_key":      response.ProviderKey,
		"provider_model_id": response.ProviderModelID,
		"finish_reason":     response.FinishReason,
		"output_text":       response.OutputText,
		"tool_calls":        cloneToolCalls(response.ToolCalls),
		"usage": map[string]int64{
			"input_tokens":  response.Usage.InputTokens,
			"output_tokens": response.Usage.OutputTokens,
			"total_tokens":  response.Usage.TotalTokens,
		},
		"raw_response": normalizeJSON(response.RawResponse),
	}, runevents.SummaryMetadata{
		Status:          "running",
		StepIndex:       o.currentStep(),
		ProviderKey:     response.ProviderKey,
		ProviderModelID: response.ProviderModelID,
		EvidenceLevel:   runevents.EvidenceLevelNativeStructured,
	})
}

func (o *NativeRunEventObserver) OnToolExecution(ctx context.Context, toolCall provider.ToolCall, result provider.ToolResult) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}

	eventType := runevents.EventTypeToolCallCompleted
	status := "completed"
	if result.IsError {
		eventType = runevents.EventTypeToolCallFailed
		status = "failed"
	}

	return o.recordEvent(ctx, eventType, map[string]any{
		"tool_call_id": toolCall.ID,
		"tool_name":    toolCall.Name,
		"arguments":    normalizeJSON(toolCall.Arguments),
		"result": map[string]any{
			"tool_call_id": result.ToolCallID,
			"content":      result.Content,
			"is_error":     result.IsError,
		},
	}, runevents.SummaryMetadata{
		Status:        status,
		StepIndex:     o.currentStep(),
		ToolName:      toolCall.Name,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *NativeRunEventObserver) OnStepEnd(ctx context.Context, step int) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	return o.recordEvent(ctx, runevents.EventTypeSystemStepCompleted, map[string]any{
		"step_index": step,
	}, runevents.SummaryMetadata{
		Status:        "running",
		StepIndex:     step,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *NativeRunEventObserver) OnRunComplete(ctx context.Context, result engine.Result) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	return o.recordEvent(ctx, runevents.EventTypeSystemRunCompleted, map[string]any{
		"final_output":    result.FinalOutput,
		"stop_reason":     result.StopReason,
		"step_count":      result.StepCount,
		"tool_call_count": result.ToolCallCount,
		"input_tokens":    result.Usage.InputTokens,
		"output_tokens":   result.Usage.OutputTokens,
		"total_tokens":    result.Usage.TotalTokens,
	}, runevents.SummaryMetadata{
		Status:        "completed",
		StepIndex:     o.currentStep(),
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *NativeRunEventObserver) OnRunFailure(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if startErr := o.ensureRunStarted(ctx); startErr != nil {
		return startErr
	}

	payload := map[string]any{
		"error":       err.Error(),
		"step_index":  o.currentStep(),
		"stop_reason": "",
	}
	if failure, ok := engine.AsFailure(err); ok {
		payload["stop_reason"] = failure.StopReason
	}
	if failure, ok := provider.AsFailure(err); ok {
		payload["provider_failure"] = map[string]any{
			"provider_key": failure.ProviderKey,
			"code":         failure.Code,
			"retryable":    failure.Retryable,
			"message":      failure.Message,
		}
	}

	return o.recordEvent(ctx, runevents.EventTypeSystemRunFailed, payload, runevents.SummaryMetadata{
		Status:        "failed",
		StepIndex:     o.currentStep(),
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *NativeRunEventObserver) ensureRunStarted(ctx context.Context) error {
	o.mu.Lock()
	if o.runStarted {
		o.mu.Unlock()
		return nil
	}
	o.mu.Unlock()

	if err := o.recordEvent(ctx, runevents.EventTypeSystemRunStarted, map[string]any{
		"deployment_type":  o.executionContext.Deployment.DeploymentType,
		"execution_target": o.executionContext.Deployment.RuntimeProfile.ExecutionTarget,
		"trace_mode":       o.executionContext.Deployment.RuntimeProfile.TraceMode,
		"started_at":       time.Now().UTC(),
	}, runevents.SummaryMetadata{
		Status:        "running",
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	}); err != nil {
		return err
	}

	o.mu.Lock()
	o.runStarted = true
	o.mu.Unlock()
	return nil
}

func (o *NativeRunEventObserver) currentStep() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.stepIndex
}

func (o *NativeRunEventObserver) nextEventID(eventType runevents.Type) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.eventIDSequence++
	return fmt.Sprintf("native:%s:%s:%d", o.executionContext.RunAgent.ID.String(), eventType, o.eventIDSequence)
}

func (o *NativeRunEventObserver) recordEvent(ctx context.Context, eventType runevents.Type, payload map[string]any, summary runevents.SummaryMetadata) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal native event payload: %w", err)
	}

	summary.IdempotencyKey = o.nextEventID(eventType)
	_, err = o.recorder.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       summary.IdempotencyKey,
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         o.executionContext.Run.ID,
			RunAgentID:    o.executionContext.RunAgent.ID,
			EventType:     eventType,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Now().UTC(),
			Payload:       payloadJSON,
			Summary:       summary,
		},
	})
	if err != nil {
		return fmt.Errorf("record native run event: %w", err)
	}
	return nil
}

func normalizeJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func cloneToolCalls(toolCalls []provider.ToolCall) []map[string]any {
	cloned := make([]map[string]any, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		cloned = append(cloned, map[string]any{
			"id":        toolCall.ID,
			"name":      toolCall.Name,
			"arguments": normalizeJSON(toolCall.Arguments),
		})
	}
	return cloned
}
