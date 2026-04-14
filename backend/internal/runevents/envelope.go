package runevents

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const SchemaVersionV1 = "2026-03-15"

type Type string

const (
	EventTypeSystemRunStarted      Type = "system.run.started"
	EventTypeSystemRunCompleted    Type = "system.run.completed"
	EventTypeSystemRunFailed       Type = "system.run.failed"
	EventTypeSystemOutputFinalized Type = "system.output.finalized"
	EventTypeSystemStepStarted     Type = "system.step.started"
	EventTypeSystemStepCompleted   Type = "system.step.completed"

	EventTypeModelCallStarted       Type = "model.call.started"
	EventTypeModelCallCompleted     Type = "model.call.completed"
	EventTypeModelOutputDelta       Type = "model.output.delta"
	EventTypeModelToolCallsProposed Type = "model.tool_calls.proposed"

	EventTypeToolCallStarted   Type = "tool.call.started"
	EventTypeToolCallCompleted Type = "tool.call.completed"
	EventTypeToolCallFailed    Type = "tool.call.failed"

	EventTypeSandboxCommandStarted   Type = "sandbox.command.started"
	EventTypeSandboxCommandCompleted Type = "sandbox.command.completed"
	EventTypeSandboxCommandFailed    Type = "sandbox.command.failed"
	EventTypeSandboxFileRead         Type = "sandbox.file.read"
	EventTypeSandboxFileWritten      Type = "sandbox.file.written"
	EventTypeSandboxFileListed       Type = "sandbox.file.listed"

	EventTypeGraderVerificationFileCaptured    Type = "grader.verification.file_captured"
	EventTypeGraderVerificationDirectoryListed Type = "grader.verification.directory_listed"
	EventTypeGraderVerificationCodeExecuted    Type = "grader.verification.code_executed"

	EventTypeScoringStarted        Type = "scoring.started"
	EventTypeScoringMetricRecorded Type = "scoring.metric.recorded"
	EventTypeScoringCompleted      Type = "scoring.completed"
	EventTypeScoringFailed         Type = "scoring.failed"
)

type Source string

const (
	SourceNativeEngine       Source = "native_engine"
	SourcePromptEvalEngine   Source = "prompt_eval_engine"
	SourceHostedExternal     Source = "hosted_external"
	SourceHostedCallback     Source = "hosted_callback"
	SourceWorkerScoring      Source = "worker_scoring"
	SourceGraderVerification Source = "grader_verification"
)

type EvidenceLevel string

const (
	EvidenceLevelNativeStructured EvidenceLevel = "native_structured"
	EvidenceLevelHostedStructured EvidenceLevel = "hosted_structured"
	EvidenceLevelHostedBlackBox   EvidenceLevel = "hosted_black_box"
	EvidenceLevelDerivedSummary   EvidenceLevel = "derived_summary"
)

var (
	ErrInvalidSchemaVersion = errors.New("invalid run event schema version")
	ErrInvalidEventType     = errors.New("invalid run event type")
	ErrInvalidEventSource   = errors.New("invalid run event source")
)

type SummaryMetadata struct {
	Status          string        `json:"status,omitempty"`
	StepIndex       int           `json:"step_index,omitempty"`
	ProviderKey     string        `json:"provider_key,omitempty"`
	ProviderModelID string        `json:"provider_model_id,omitempty"`
	ToolName        string        `json:"tool_name,omitempty"`
	ToolCategory    string        `json:"tool_category,omitempty"`
	SandboxAction   string        `json:"sandbox_action,omitempty"`
	MetricKey       string        `json:"metric_key,omitempty"`
	ExternalRunID   string        `json:"external_run_id,omitempty"`
	EvidenceLevel   EvidenceLevel `json:"evidence_level,omitempty"`
	IdempotencyKey  string        `json:"idempotency_key,omitempty"`
}

type Envelope struct {
	EventID        string          `json:"event_id"`
	SchemaVersion  string          `json:"schema_version"`
	RunID          uuid.UUID       `json:"run_id"`
	RunAgentID     uuid.UUID       `json:"run_agent_id"`
	SequenceNumber int64           `json:"sequence_number"`
	EventType      Type            `json:"event_type"`
	Source         Source          `json:"source"`
	OccurredAt     time.Time       `json:"occurred_at"`
	Payload        json.RawMessage `json:"payload"`
	Summary        SummaryMetadata `json:"summary,omitempty"`
}

func (e Envelope) ValidatePending() error {
	if e.EventID == "" {
		return errors.New("event_id is required")
	}
	if e.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("%w: %q", ErrInvalidSchemaVersion, e.SchemaVersion)
	}
	if e.RunID == uuid.Nil {
		return errors.New("run_id is required")
	}
	if e.RunAgentID == uuid.Nil {
		return errors.New("run_agent_id is required")
	}
	if !isValidType(e.EventType) {
		return fmt.Errorf("%w: %q", ErrInvalidEventType, e.EventType)
	}
	if !isValidSource(e.Source) {
		return fmt.Errorf("%w: %q", ErrInvalidEventSource, e.Source)
	}
	if e.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if len(e.Payload) > 0 && !json.Valid(e.Payload) {
		return errors.New("payload must be valid JSON")
	}
	return nil
}

func (e Envelope) ValidatePersisted() error {
	if err := e.ValidatePending(); err != nil {
		return err
	}
	if e.SequenceNumber <= 0 {
		return errors.New("sequence_number must be greater than zero")
	}
	return nil
}

func (e Envelope) WithSequenceNumber(sequenceNumber int64) Envelope {
	e.SequenceNumber = sequenceNumber
	return e
}

func isValidType(eventType Type) bool {
	switch eventType {
	case EventTypeSystemRunStarted,
		EventTypeSystemRunCompleted,
		EventTypeSystemRunFailed,
		EventTypeSystemOutputFinalized,
		EventTypeSystemStepStarted,
		EventTypeSystemStepCompleted,
		EventTypeModelCallStarted,
		EventTypeModelCallCompleted,
		EventTypeModelOutputDelta,
		EventTypeModelToolCallsProposed,
		EventTypeToolCallStarted,
		EventTypeToolCallCompleted,
		EventTypeToolCallFailed,
		EventTypeSandboxCommandStarted,
		EventTypeSandboxCommandCompleted,
		EventTypeSandboxCommandFailed,
		EventTypeSandboxFileRead,
		EventTypeSandboxFileWritten,
		EventTypeSandboxFileListed,
		EventTypeGraderVerificationFileCaptured,
		EventTypeGraderVerificationDirectoryListed,
		EventTypeGraderVerificationCodeExecuted,
		EventTypeScoringStarted,
		EventTypeScoringMetricRecorded,
		EventTypeScoringCompleted,
		EventTypeScoringFailed:
		return true
	default:
		return false
	}
}

func isValidSource(source Source) bool {
	switch source {
	case SourceNativeEngine, SourcePromptEvalEngine, SourceHostedExternal, SourceHostedCallback, SourceWorkerScoring, SourceGraderVerification:
		return true
	default:
		return false
	}
}
