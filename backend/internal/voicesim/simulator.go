package voicesim

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

const ScriptSchemaVersionV1 = "2026-05-13"

var (
	ErrInvalidScript           = errors.New("invalid scripted voice simulator script")
	ErrMaxTurnsExceeded        = errors.New("scripted voice simulator max_turns exceeded")
	ErrUnexpectedAgentResponse = errors.New("unexpected scripted voice agent response")
	ErrResponseLatencyExceeded = errors.New("scripted voice agent response latency exceeded")
)

type Script struct {
	SchemaVersion string    `json:"schema_version"`
	TraceID       string    `json:"trace_id"`
	RunID         uuid.UUID `json:"run_id"`
	RunAgentID    uuid.UUID `json:"run_agent_id"`
	ScenarioKey   string    `json:"scenario_key"`
	MaxTurns      int       `json:"max_turns"`
	BaseTime      time.Time `json:"base_time"`
	Steps         []Step    `json:"steps"`
}

type Step struct {
	TurnID               string        `json:"turn_id"`
	UserText             string        `json:"user_text"`
	Language             string        `json:"language,omitempty"`
	OccurredAtOffsetMS   int64         `json:"occurred_at_offset_ms"`
	ExpectedAgentText    string        `json:"expected_agent_text"`
	MaxResponseLatencyMS int64         `json:"max_response_latency_ms,omitempty"`
	Interruption         *Interruption `json:"interruption,omitempty"`
}

type Interruption struct {
	OccurredAtOffsetMS int64 `json:"occurred_at_offset_ms"`
	ClearBuffer        bool  `json:"clear_buffer,omitempty"`
}

type Agent interface {
	Respond(step Step) (AgentResponse, error)
}

type AgentResponse struct {
	Text      string
	Language  string
	LatencyMS int64
}

type Result struct {
	Trace      multimodaltrace.Trace
	Events     []runevents.Envelope
	TraceJSON  []byte
	EventsJSON []byte
}

type Simulator struct {
	script Script
}

func LoadScript(path string) (Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Script{}, err
	}
	var script Script
	if err := json.Unmarshal(data, &script); err != nil {
		return Script{}, fmt.Errorf("decode scripted voice simulator script: %w", err)
	}
	if err := script.Validate(); err != nil {
		return Script{}, err
	}
	return script, nil
}

func New(script Script) (*Simulator, error) {
	if err := script.Validate(); err != nil {
		return nil, err
	}
	return &Simulator{script: script}, nil
}

func (s Script) Validate() error {
	if s.SchemaVersion != ScriptSchemaVersionV1 {
		return fmt.Errorf("%w: schema_version must be %q", ErrInvalidScript, ScriptSchemaVersionV1)
	}
	if strings.TrimSpace(s.TraceID) == "" {
		return fmt.Errorf("%w: trace_id is required", ErrInvalidScript)
	}
	if s.RunID == uuid.Nil {
		return fmt.Errorf("%w: run_id is required", ErrInvalidScript)
	}
	if s.RunAgentID == uuid.Nil {
		return fmt.Errorf("%w: run_agent_id is required", ErrInvalidScript)
	}
	if strings.TrimSpace(s.ScenarioKey) == "" {
		return fmt.Errorf("%w: scenario_key is required", ErrInvalidScript)
	}
	if s.MaxTurns <= 0 {
		return fmt.Errorf("%w: max_turns must be greater than zero", ErrInvalidScript)
	}
	if s.BaseTime.IsZero() {
		return fmt.Errorf("%w: base_time is required", ErrInvalidScript)
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("%w: steps must contain at least one step", ErrInvalidScript)
	}
	if len(s.Steps) > s.MaxTurns {
		return fmt.Errorf("%w: steps=%d max_turns=%d", ErrMaxTurnsExceeded, len(s.Steps), s.MaxTurns)
	}
	var previousOffset int64 = -1
	for idx, step := range s.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("steps[%d]: %w", idx, err)
		}
		if step.OccurredAtOffsetMS <= previousOffset {
			return fmt.Errorf("steps[%d]: %w: occurred_at_offset_ms must increase", idx, ErrInvalidScript)
		}
		previousOffset = step.OccurredAtOffsetMS
	}
	return nil
}

func (s Step) Validate() error {
	if strings.TrimSpace(s.TurnID) == "" {
		return fmt.Errorf("%w: turn_id is required", ErrInvalidScript)
	}
	if strings.TrimSpace(s.UserText) == "" {
		return fmt.Errorf("%w: user_text is required", ErrInvalidScript)
	}
	if strings.TrimSpace(s.ExpectedAgentText) == "" {
		return fmt.Errorf("%w: expected_agent_text is required", ErrInvalidScript)
	}
	if s.OccurredAtOffsetMS < 0 {
		return fmt.Errorf("%w: occurred_at_offset_ms must be non-negative", ErrInvalidScript)
	}
	if s.MaxResponseLatencyMS < 0 {
		return fmt.Errorf("%w: max_response_latency_ms must be non-negative", ErrInvalidScript)
	}
	if s.Interruption != nil && s.Interruption.OccurredAtOffsetMS < s.OccurredAtOffsetMS {
		return fmt.Errorf("%w: interruption must not occur before the user turn", ErrInvalidScript)
	}
	return nil
}

func (s *Simulator) Run(agent Agent) (Result, error) {
	if agent == nil {
		return Result{}, fmt.Errorf("%w: agent is required", ErrInvalidScript)
	}

	segments := make([]multimodaltrace.Segment, 0, len(s.script.Steps)*4)
	events := make([]runevents.Envelope, 0, len(s.script.Steps)*5+2)
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
			summary.IdempotencyKey = fmt.Sprintf("%s:event:%03d", s.script.TraceID, eventSeq)
		}
		envelope := runevents.Envelope{
			EventID:        fmt.Sprintf("voice-sim:%s:%03d", s.script.TraceID, eventSeq),
			SchemaVersion:  runevents.SchemaVersionV1,
			RunID:          s.script.RunID,
			RunAgentID:     s.script.RunAgentID,
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

	if err := appendEvent(runevents.EventTypeMediaSessionStarted, s.script.BaseTime, map[string]any{
		"scenario_key":     s.script.ScenarioKey,
		"voice_session_id": s.script.TraceID,
		"mode":             "text-sim",
	}, runevents.SummaryMetadata{
		Status:         "running",
		IdempotencyKey: s.script.TraceID + ":session_started",
		EvidenceLevel:  runevents.EvidenceLevelVoiceStructured,
	}); err != nil {
		return Result{}, err
	}

	nextSegmentNumber := int64(1)
	for idx, step := range s.script.Steps {
		turnIndex := idx + 1
		userOccurredAt := s.script.BaseTime.Add(time.Duration(step.OccurredAtOffsetMS) * time.Millisecond).UTC()
		userSegmentID := fmt.Sprintf("%s:user-text", step.TurnID)
		segments = append(segments, multimodaltrace.Segment{
			SegmentID:      userSegmentID,
			SequenceNumber: nextSegmentNumber,
			Kind:           multimodaltrace.SegmentKindTextInput,
			Actor:          multimodaltrace.ActorUser,
			OccurredAt:     userOccurredAt,
			Text: &multimodaltrace.TextPayload{
				Text:     step.UserText,
				Language: step.Language,
			},
		})
		nextSegmentNumber++
		if err := appendEvent(runevents.EventTypeTranscriptFinal, userOccurredAt, map[string]any{
			"turn_id":    step.TurnID,
			"text":       step.UserText,
			"language":   step.Language,
			"segment_id": userSegmentID,
		}, turnSummary(turnIndex, "caller", "text")); err != nil {
			return Result{}, err
		}

		response, err := agent.Respond(step)
		if err != nil {
			return Result{}, err
		}
		if response.LatencyMS < 0 {
			return Result{}, fmt.Errorf("%w: latency_ms must be non-negative", ErrInvalidScript)
		}
		if step.MaxResponseLatencyMS > 0 && response.LatencyMS > step.MaxResponseLatencyMS {
			return Result{}, fmt.Errorf("%w: turn_id=%s latency_ms=%d max_response_latency_ms=%d", ErrResponseLatencyExceeded, step.TurnID, response.LatencyMS, step.MaxResponseLatencyMS)
		}
		if response.Text != step.ExpectedAgentText {
			if err := appendEvent(runevents.EventTypeSystemRunFailed, userOccurredAt, map[string]any{
				"turn_id":  step.TurnID,
				"expected": step.ExpectedAgentText,
				"actual":   response.Text,
			}, runevents.SummaryMetadata{
				Status:         "failed",
				IdempotencyKey: s.script.TraceID + ":" + step.TurnID + ":failed",
			}); err != nil {
				return Result{}, err
			}
			return Result{}, fmt.Errorf("%w: turn_id=%s expected %q got %q", ErrUnexpectedAgentResponse, step.TurnID, step.ExpectedAgentText, response.Text)
		}

		agentOccurredAt := userOccurredAt.Add(time.Duration(response.LatencyMS) * time.Millisecond).UTC()
		agentSegmentID := fmt.Sprintf("%s:agent-text", step.TurnID)
		responseLanguage := firstNonEmpty(response.Language, step.Language)
		segments = append(segments, multimodaltrace.Segment{
			SegmentID:      agentSegmentID,
			SequenceNumber: nextSegmentNumber,
			Kind:           multimodaltrace.SegmentKindTextOutput,
			Actor:          multimodaltrace.ActorAgent,
			OccurredAt:     agentOccurredAt,
			Text: &multimodaltrace.TextPayload{
				Text:     response.Text,
				Language: responseLanguage,
			},
		})
		nextSegmentNumber++

		metricSegmentID := fmt.Sprintf("%s:latency", step.TurnID)
		segments = append(segments, multimodaltrace.Segment{
			SegmentID:      metricSegmentID,
			SequenceNumber: nextSegmentNumber,
			Kind:           multimodaltrace.SegmentKindTimingMarker,
			Actor:          multimodaltrace.ActorEvaluator,
			OccurredAt:     agentOccurredAt,
			TimingMarker: &multimodaltrace.TimingMarkerPayload{
				Key:     "end_of_user_text_to_agent_text",
				ValueMS: response.LatencyMS,
			},
		})
		nextSegmentNumber++

		if err := appendEvent(runevents.EventTypeVoiceMetricRecorded, agentOccurredAt, map[string]any{
			"turn_id":    step.TurnID,
			"metric_key": "end_of_user_text_to_agent_text_ms",
			"value_ms":   response.LatencyMS,
		}, metricSummary(turnIndex, "end_of_user_text_to_agent_text_ms")); err != nil {
			return Result{}, err
		}
		if err := appendEvent(runevents.EventTypeTurnCompleted, agentOccurredAt, map[string]any{
			"turn_id":          step.TurnID,
			"user_segment_id":  userSegmentID,
			"agent_segment_id": agentSegmentID,
		}, turnSummary(turnIndex, "agent", "text")); err != nil {
			return Result{}, err
		}

		if step.Interruption != nil {
			interruptionOccurredAt := s.script.BaseTime.Add(time.Duration(step.Interruption.OccurredAtOffsetMS) * time.Millisecond).UTC()
			controlSegmentID := fmt.Sprintf("%s:interruption", step.TurnID)
			segments = append(segments, multimodaltrace.Segment{
				SegmentID:      controlSegmentID,
				SequenceNumber: nextSegmentNumber,
				Kind:           multimodaltrace.SegmentKindMediaControl,
				Actor:          multimodaltrace.ActorSystem,
				OccurredAt:     interruptionOccurredAt,
				MediaControl: &multimodaltrace.MediaControlPayload{
					Action:          "barge_in_detected",
					TargetSegmentID: agentSegmentID,
				},
			})
			nextSegmentNumber++
			if err := appendEvent(runevents.EventTypeBargeInDetected, interruptionOccurredAt, map[string]any{
				"turn_id":           step.TurnID,
				"target_segment_id": agentSegmentID,
			}, turnSummary(turnIndex, "caller", "text")); err != nil {
				return Result{}, err
			}
			if step.Interruption.ClearBuffer {
				if err := appendEvent(runevents.EventTypeAudioBufferCleared, interruptionOccurredAt, map[string]any{
					"turn_id":           step.TurnID,
					"target_segment_id": agentSegmentID,
				}, turnSummary(turnIndex, "system", "text")); err != nil {
					return Result{}, err
				}
			}
		}
	}

	completedAt := s.script.BaseTime
	if len(segments) > 0 {
		completedAt = segments[len(segments)-1].OccurredAt
	}
	if err := appendEvent(runevents.EventTypeSystemRunCompleted, completedAt, map[string]any{
		"scenario_key": s.script.ScenarioKey,
		"turn_count":   len(s.script.Steps),
	}, runevents.SummaryMetadata{
		Status:         "completed",
		IdempotencyKey: s.script.TraceID + ":completed",
	}); err != nil {
		return Result{}, err
	}

	trace := multimodaltrace.Trace{
		TraceID:       s.script.TraceID,
		SchemaVersion: multimodaltrace.SchemaVersionV1,
		RunID:         s.script.RunID,
		RunAgentID:    s.script.RunAgentID,
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
		Trace:      trace,
		Events:     events,
		TraceJSON:  traceJSON,
		EventsJSON: eventsJSON,
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

func metricSummary(turnIndex int, metricKey string) runevents.SummaryMetadata {
	summary := turnSummary(turnIndex, "evaluator", "metric")
	summary.MetricKey = metricKey
	return summary
}

func marshalStable(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
