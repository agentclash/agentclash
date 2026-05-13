package voicetextsim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/voicedeployment"
	"github.com/agentclash/agentclash/backend/internal/voicesim"
)

var (
	ErrInvalidInput = errors.New("invalid text-sim voice execution input")
	ErrRunFailed    = errors.New("text-sim voice execution failed")
)

type Status string

const (
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Input struct {
	Bundle     challengepack.Bundle
	Script     voicesim.Script
	Deployment voicedeployment.Deployment
}

type Result struct {
	Status        Status
	FailureReason string
	Trace         multimodaltrace.Trace
	Events        []runevents.Envelope
	TraceJSON     []byte
	EventsJSON    []byte
}

func Run(ctx context.Context, input Input) (Result, error) {
	if err := validateInput(input); err != nil {
		return Result{}, err
	}
	segments := make([]multimodaltrace.Segment, 0, len(input.Script.Steps)*8)
	events := make([]runevents.Envelope, 0, len(input.Script.Steps)*8+2)
	eventSeq := int64(0)
	appendEvent := func(eventType runevents.Type, occurredAt time.Time, payload any, summary runevents.SummaryMetadata) error {
		eventSeq++
		rawPayload, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal event payload: %w", err)
		}
		if summary.EvidenceLevel == "" {
			summary.EvidenceLevel = runevents.EvidenceLevelVoiceStructured
		}
		if summary.IdempotencyKey == "" {
			summary.IdempotencyKey = fmt.Sprintf("%s:text-sim:event:%03d", input.Script.TraceID, eventSeq)
		}
		envelope := runevents.Envelope{
			EventID:        fmt.Sprintf("voice-text-sim:%s:%03d", input.Script.TraceID, eventSeq),
			SchemaVersion:  runevents.SchemaVersionV1,
			RunID:          input.Script.RunID,
			RunAgentID:     input.Script.RunAgentID,
			SequenceNumber: eventSeq,
			EventType:      eventType,
			Source:         runevents.SourceVoiceAdapter,
			OccurredAt:     occurredAt.UTC(),
			Payload:        rawPayload,
			Summary:        summary,
		}
		if err := envelope.ValidatePersisted(); err != nil {
			return err
		}
		events = append(events, envelope)
		return nil
	}

	if err := appendEvent(runevents.EventTypeMediaSessionStarted, input.Script.BaseTime, map[string]any{
		"mode":             "text-sim",
		"scenario_key":     input.Script.ScenarioKey,
		"voice_session_id": input.Script.TraceID,
		"pack_slug":        input.Bundle.Pack.Slug,
	}, runevents.SummaryMetadata{
		Status:         string(StatusCompleted),
		IdempotencyKey: input.Script.TraceID + ":text-sim:session-started",
	}); err != nil {
		return Result{}, err
	}

	nextSequence := int64(1)
	for idx, step := range input.Script.Steps {
		turnIndex := idx + 1
		userOccurredAt := input.Script.BaseTime.Add(time.Duration(step.OccurredAtOffsetMS) * time.Millisecond).UTC()
		userSegmentID := step.TurnID + ":user-text"
		userSegment := multimodaltrace.Segment{
			SegmentID:      userSegmentID,
			SequenceNumber: nextSequence,
			Kind:           multimodaltrace.SegmentKindTextInput,
			Actor:          multimodaltrace.ActorUser,
			OccurredAt:     userOccurredAt,
			Text: &multimodaltrace.TextPayload{
				Text:     step.UserText,
				Language: step.Language,
			},
		}
		segments = append(segments, userSegment)
		nextSequence++
		if err := appendEvent(runevents.EventTypeTranscriptFinal, userOccurredAt, map[string]any{
			"turn_id":    step.TurnID,
			"text":       step.UserText,
			"language":   step.Language,
			"segment_id": userSegmentID,
		}, turnSummary(turnIndex, "caller", "text")); err != nil {
			return Result{}, err
		}

		deploymentResult, err := input.Deployment.Invoke(ctx, voicedeployment.Invocation{
			TraceID:       input.Script.TraceID,
			RunID:         input.Script.RunID,
			RunAgentID:    input.Script.RunAgentID,
			TurnID:        step.TurnID,
			InputSegments: append([]multimodaltrace.Segment(nil), segments...),
		})
		if err != nil {
			failureReason := err.Error()
			if eventErr := appendEvent(runevents.EventTypeSystemRunFailed, userOccurredAt, map[string]any{
				"turn_id": step.TurnID,
				"reason":  failureReason,
			}, runevents.SummaryMetadata{
				Status:         string(StatusFailed),
				IdempotencyKey: input.Script.TraceID + ":" + step.TurnID + ":failed",
			}); eventErr != nil {
				return Result{}, eventErr
			}
			result, buildErr := buildResult(input, StatusFailed, failureReason, segments, events)
			if buildErr != nil {
				return Result{}, buildErr
			}
			return result, nil
		}

		segments = append(segments, deploymentResult.Segments...)
		for _, segment := range deploymentResult.Segments {
			if err := appendSegmentEvent(appendEvent, turnIndex, step.TurnID, segment); err != nil {
				return Result{}, err
			}
		}
		if deploymentResult.Outcome == voicedeployment.OutcomeFail {
			failureReason := fmt.Sprintf("turn_id=%s deployment outcome failed", step.TurnID)
			if err := appendEvent(runevents.EventTypeSystemRunFailed, latestSegmentTime(deploymentResult.Segments, userOccurredAt), map[string]any{
				"turn_id": step.TurnID,
				"reason":  failureReason,
			}, runevents.SummaryMetadata{
				Status:         string(StatusFailed),
				IdempotencyKey: input.Script.TraceID + ":" + step.TurnID + ":failed",
			}); err != nil {
				return Result{}, err
			}
			result, err := buildResult(input, StatusFailed, failureReason, segments, events)
			if err != nil {
				return Result{}, err
			}
			return result, nil
		}
		if err := appendEvent(runevents.EventTypeTurnCompleted, latestSegmentTime(deploymentResult.Segments, userOccurredAt), map[string]any{
			"turn_id":         step.TurnID,
			"user_segment_id": userSegmentID,
			"segment_count":   len(deploymentResult.Segments) + 1,
		}, turnSummary(turnIndex, "agent", "text")); err != nil {
			return Result{}, err
		}
	}

	completedAt := input.Script.BaseTime
	if len(segments) > 0 {
		completedAt = segments[len(segments)-1].OccurredAt
	}
	if err := appendEvent(runevents.EventTypeSystemRunCompleted, completedAt, map[string]any{
		"mode":         "text-sim",
		"scenario_key": input.Script.ScenarioKey,
		"turn_count":   len(input.Script.Steps),
	}, runevents.SummaryMetadata{
		Status:         string(StatusCompleted),
		IdempotencyKey: input.Script.TraceID + ":text-sim:completed",
	}); err != nil {
		return Result{}, err
	}
	return buildResult(input, StatusCompleted, "", segments, events)
}

func validateInput(input Input) error {
	if input.Deployment == nil {
		return fmt.Errorf("%w: deployment is required", ErrInvalidInput)
	}
	if input.Bundle.Modality != challengepack.ModalityVoice {
		return fmt.Errorf("%w: bundle.modality must be %q", ErrInvalidInput, challengepack.ModalityVoice)
	}
	if input.Bundle.InterfaceSpec == nil {
		return fmt.Errorf("%w: bundle.interface_spec is required", ErrInvalidInput)
	}
	if !contains(input.Bundle.InterfaceSpec.Transports, "text_sim") {
		return fmt.Errorf("%w: bundle.interface_spec.transports must include text_sim", ErrInvalidInput)
	}
	if err := input.Script.Validate(); err != nil {
		return fmt.Errorf("%w: script: %w", ErrInvalidInput, err)
	}
	if input.Bundle.Scenario != nil && input.Bundle.Scenario.MaxTurns > 0 && len(input.Script.Steps) > int(input.Bundle.Scenario.MaxTurns) {
		return fmt.Errorf("%w: script has %d steps but scenario.max_turns=%d", ErrInvalidInput, len(input.Script.Steps), input.Bundle.Scenario.MaxTurns)
	}
	return nil
}

func appendSegmentEvent(appendEvent func(runevents.Type, time.Time, any, runevents.SummaryMetadata) error, turnIndex int, turnID string, segment multimodaltrace.Segment) error {
	switch segment.Kind {
	case multimodaltrace.SegmentKindToolCall:
		return appendEvent(runevents.EventTypeToolCallStarted, segment.OccurredAt, map[string]any{
			"turn_id":    turnID,
			"segment_id": segment.SegmentID,
			"call_id":    segment.ToolCall.CallID,
			"tool_name":  segment.ToolCall.ToolName,
			"arguments":  json.RawMessage(segment.ToolCall.Arguments),
		}, toolSummary(turnIndex, segment.ToolCall.ToolName))
	case multimodaltrace.SegmentKindTextOutput:
		return nil
	case multimodaltrace.SegmentKindAudioOutput:
		if err := appendEvent(runevents.EventTypeAgentAudioStarted, segment.OccurredAt, map[string]any{
			"turn_id":      turnID,
			"segment_id":   segment.SegmentID,
			"artifact_ref": segment.Audio.ArtifactRef,
		}, turnSummary(turnIndex, "agent", "audio")); err != nil {
			return err
		}
		completedAt := segment.OccurredAt.Add(time.Duration(segment.Audio.DurationMS) * time.Millisecond).UTC()
		return appendEvent(runevents.EventTypeAgentAudioCompleted, completedAt, map[string]any{
			"turn_id":      turnID,
			"segment_id":   segment.SegmentID,
			"artifact_ref": segment.Audio.ArtifactRef,
			"duration_ms":  segment.Audio.DurationMS,
		}, turnSummary(turnIndex, "agent", "audio"))
	case multimodaltrace.SegmentKindTranscriptFinal:
		return appendEvent(runevents.EventTypeTranscriptFinal, segment.OccurredAt, map[string]any{
			"turn_id":    turnID,
			"text":       segment.Transcript.Text,
			"language":   segment.Transcript.Language,
			"segment_id": segment.SegmentID,
		}, turnSummary(turnIndex, "agent", "text"))
	case multimodaltrace.SegmentKindTimingMarker:
		return appendEvent(runevents.EventTypeVoiceMetricRecorded, segment.OccurredAt, map[string]any{
			"turn_id":    turnID,
			"metric_key": segment.TimingMarker.Key,
			"value_ms":   segment.TimingMarker.ValueMS,
		}, metricSummary(turnIndex, segment.TimingMarker.Key))
	default:
		return nil
	}
}

func buildResult(input Input, status Status, failureReason string, segments []multimodaltrace.Segment, events []runevents.Envelope) (Result, error) {
	trace := multimodaltrace.Trace{
		TraceID:       input.Script.TraceID,
		SchemaVersion: multimodaltrace.SchemaVersionV1,
		RunID:         input.Script.RunID,
		RunAgentID:    input.Script.RunAgentID,
		Segments:      segments,
	}
	if err := trace.Validate(); err != nil {
		return Result{}, err
	}
	traceJSON, err := marshalStable(trace)
	if err != nil {
		return Result{}, err
	}
	eventsJSON, err := marshalStable(events)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Status:        status,
		FailureReason: failureReason,
		Trace:         trace,
		Events:        events,
		TraceJSON:     traceJSON,
		EventsJSON:    eventsJSON,
	}, nil
}

func turnSummary(turnIndex int, speaker string, channel string) runevents.SummaryMetadata {
	return runevents.SummaryMetadata{
		TurnIndex:     &turnIndex,
		Speaker:       speaker,
		Channel:       channel,
		EvidenceLevel: runevents.EvidenceLevelVoiceStructured,
	}
}

func toolSummary(turnIndex int, toolName string) runevents.SummaryMetadata {
	summary := turnSummary(turnIndex, "agent", "tool")
	summary.ToolName = toolName
	return summary
}

func metricSummary(turnIndex int, metricKey string) runevents.SummaryMetadata {
	summary := turnSummary(turnIndex, "evaluator", "metric")
	summary.MetricKey = metricKey
	return summary
}

func latestSegmentTime(segments []multimodaltrace.Segment, fallback time.Time) time.Time {
	latest := fallback
	for _, segment := range segments {
		if segment.OccurredAt.After(latest) {
			latest = segment.OccurredAt
		}
	}
	return latest.UTC()
}

func marshalStable(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == needle {
			return true
		}
	}
	return false
}
