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

type PromptEvalObserverFactory func(executionContext repository.RunAgentExecutionContext) (engine.Observer, error)

// PromptEvalRunEventObserver records the minimal set of run events required by
// the scoring pipeline for a prompt_eval execution: run.started,
// model.call.completed, output.finalized, run.completed (or run.failed).
type PromptEvalRunEventObserver struct {
	recorder         RunEventRecorder
	executionContext repository.RunAgentExecutionContext

	mu              sync.Mutex
	eventIDSequence int64
	outputRecorded  bool

	started    sync.Once
	startedErr error
}

func NewPromptEvalRunEventObserverFactory(recorder RunEventRecorder) PromptEvalObserverFactory {
	return func(executionContext repository.RunAgentExecutionContext) (engine.Observer, error) {
		if recorder == nil {
			return engine.NoopObserver{}, nil
		}
		return &PromptEvalRunEventObserver{
			recorder:         recorder,
			executionContext: executionContext,
		}, nil
	}
}

func (o *PromptEvalRunEventObserver) OnStepStart(ctx context.Context, _ int) error {
	return o.ensureRunStarted(ctx)
}

func (o *PromptEvalRunEventObserver) OnProviderCall(ctx context.Context, request provider.Request) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	return o.recordEvent(ctx, runevents.EventTypeModelCallStarted, map[string]any{
		"provider_key":        request.ProviderKey,
		"provider_account_id": request.ProviderAccountID,
		"model":               request.Model,
		"trace_mode":          request.TraceMode,
		"step_timeout_ms":     request.StepTimeout.Milliseconds(),
		"message_count":       len(request.Messages),
		"metadata":            normalizeJSON(request.Metadata),
	}, runevents.SummaryMetadata{
		Status:          "running",
		StepIndex:       1,
		ProviderKey:     request.ProviderKey,
		ProviderModelID: request.Model,
		EvidenceLevel:   runevents.EvidenceLevelNativeStructured,
	})
}

func (o *PromptEvalRunEventObserver) OnProviderOutput(context.Context, provider.Request, provider.StreamDelta) error {
	return nil
}

func (o *PromptEvalRunEventObserver) OnProviderResponse(ctx context.Context, response provider.Response) error {
	if err := o.ensureRunStarted(ctx); err != nil {
		return err
	}
	o.mu.Lock()
	alreadyRecorded := o.outputRecorded
	o.outputRecorded = true
	o.mu.Unlock()

	if err := o.recordEvent(ctx, runevents.EventTypeModelCallCompleted, map[string]any{
		"provider_key":      response.ProviderKey,
		"provider_model_id": response.ProviderModelID,
		"finish_reason":     response.FinishReason,
		"output_text":       response.OutputText,
		"tool_calls":        []any{},
		"usage": map[string]int64{
			"input_tokens":  response.Usage.InputTokens,
			"output_tokens": response.Usage.OutputTokens,
			"total_tokens":  response.Usage.TotalTokens,
		},
		"timing": map[string]int64{
			"ttft_ms":          response.Timing.TTFT.Milliseconds(),
			"total_latency_ms": response.Timing.TotalLatency.Milliseconds(),
		},
		"raw_response": normalizeJSON(response.RawResponse),
	}, runevents.SummaryMetadata{
		Status:          "running",
		StepIndex:       1,
		ProviderKey:     response.ProviderKey,
		ProviderModelID: response.ProviderModelID,
		EvidenceLevel:   runevents.EvidenceLevelNativeStructured,
	}); err != nil {
		return err
	}
	if alreadyRecorded {
		return nil
	}
	return o.recordEvent(ctx, runevents.EventTypeSystemOutputFinalized, map[string]any{
		"final_output": response.OutputText,
		"source":       "prompt_eval_engine",
	}, runevents.SummaryMetadata{
		Status:        "running",
		StepIndex:     1,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *PromptEvalRunEventObserver) OnToolExecution(context.Context, engine.ToolExecutionRecord) error {
	return nil
}

func (o *PromptEvalRunEventObserver) OnStepEnd(context.Context, int) error {
	return nil
}

func (o *PromptEvalRunEventObserver) OnPostExecutionVerification(context.Context, []engine.PostExecutionVerificationResult) error {
	return nil
}

func (o *PromptEvalRunEventObserver) OnRunComplete(ctx context.Context, result engine.Result) error {
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
		StepIndex:     1,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *PromptEvalRunEventObserver) OnRunFailure(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if startErr := o.ensureRunStarted(ctx); startErr != nil {
		return startErr
	}

	payload := map[string]any{
		"error":       err.Error(),
		"step_index":  1,
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
		StepIndex:     1,
		EvidenceLevel: runevents.EvidenceLevelNativeStructured,
	})
}

func (o *PromptEvalRunEventObserver) ensureRunStarted(ctx context.Context) error {
	o.started.Do(func() {
		o.startedErr = o.recordEvent(ctx, runevents.EventTypeSystemRunStarted, map[string]any{
			"deployment_type":  o.executionContext.Deployment.DeploymentType,
			"execution_mode":   "prompt_eval",
			"execution_target": o.executionContext.Deployment.RuntimeProfile.ExecutionTarget,
			"trace_mode":       o.executionContext.Deployment.RuntimeProfile.TraceMode,
			"started_at":       time.Now().UTC(),
		}, runevents.SummaryMetadata{
			Status:        "running",
			EvidenceLevel: runevents.EvidenceLevelNativeStructured,
		})
	})
	return o.startedErr
}

func (o *PromptEvalRunEventObserver) nextEventID(eventType runevents.Type) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.eventIDSequence++
	return fmt.Sprintf("prompt_eval:%s:%s:%d", o.executionContext.RunAgent.ID.String(), eventType, o.eventIDSequence)
}

func (o *PromptEvalRunEventObserver) recordEvent(ctx context.Context, eventType runevents.Type, payload map[string]any, summary runevents.SummaryMetadata) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal prompt_eval event payload: %w", err)
	}

	summary.IdempotencyKey = o.nextEventID(eventType)
	_, err = o.recorder.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       summary.IdempotencyKey,
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         o.executionContext.Run.ID,
			RunAgentID:    o.executionContext.RunAgent.ID,
			EventType:     eventType,
			Source:        runevents.SourcePromptEvalEngine,
			OccurredAt:    time.Now().UTC(),
			Payload:       payloadJSON,
			Summary:       summary,
		},
	})
	if err != nil {
		return fmt.Errorf("record prompt_eval run event: %w", err)
	}
	return nil
}
