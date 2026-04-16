package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

const runAgentReplaySummarySchemaVersion = "2026-03-16"

type runAgentReplaySummaryDocument struct {
	SchemaVersion string                       `json:"schema_version"`
	Status        string                       `json:"status"`
	Headline      string                       `json:"headline"`
	Counts        runAgentReplayCounts         `json:"counts"`
	ArtifactIDs   []uuid.UUID                  `json:"artifact_ids,omitempty"`
	TerminalState *runAgentReplayTerminalState `json:"terminal_state,omitempty"`
	Steps         []runAgentReplayStepDocument `json:"steps"`
}

type runAgentReplayCounts struct {
	Events            int64 `json:"events"`
	ReplaySteps       int   `json:"replay_steps"`
	AgentSteps        int   `json:"agent_steps"`
	ModelCalls        int   `json:"model_calls"`
	ToolCalls         int   `json:"tool_calls"`
	SandboxCommands   int   `json:"sandbox_commands"`
	SandboxFileEvents int   `json:"sandbox_file_events"`
	Outputs           int   `json:"outputs"`
	ScoringEvents     int   `json:"scoring_events"`
}

type runAgentReplayTerminalState struct {
	Status         string           `json:"status"`
	EventType      runevents.Type   `json:"event_type"`
	Source         runevents.Source `json:"source"`
	SequenceNumber int64            `json:"sequence_number"`
	OccurredAt     time.Time        `json:"occurred_at"`
	Headline       string           `json:"headline"`
	ErrorMessage   string           `json:"error_message,omitempty"`
}

type runAgentReplayStepDocument struct {
	Type              string           `json:"type"`
	Status            string           `json:"status"`
	Headline          string           `json:"headline"`
	Source            runevents.Source `json:"source"`
	StartedSequence   int64            `json:"started_sequence"`
	CompletedSequence *int64           `json:"completed_sequence,omitempty"`
	OccurredAt        time.Time        `json:"occurred_at"`
	CompletedAt       *time.Time       `json:"completed_at,omitempty"`
	EventCount        int              `json:"event_count"`
	EventTypes        []runevents.Type `json:"event_types"`
	ArtifactIDs       []uuid.UUID      `json:"artifact_ids,omitempty"`
	StepIndex         *int             `json:"step_index,omitempty"`
	ProviderKey       string           `json:"provider_key,omitempty"`
	ProviderModelID   string           `json:"provider_model_id,omitempty"`
	ToolName          string           `json:"tool_name,omitempty"`
	SubagentKey       string           `json:"subagent_key,omitempty"`
	SubagentLabel     string           `json:"subagent_label,omitempty"`
	SandboxAction     string           `json:"sandbox_action,omitempty"`
	MetricKey         string           `json:"metric_key,omitempty"`
	FinalOutput       string           `json:"final_output,omitempty"`
	ErrorMessage      string           `json:"error_message,omitempty"`
}

type replayOpenStep struct {
	category string
	index    int
}

func (r *Repository) BuildRunAgentReplay(ctx context.Context, runAgentID uuid.UUID) (RunAgentReplay, error) {
	events, err := r.ListRunEventsByRunAgentID(ctx, runAgentID)
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("list run events for replay build: %w", err)
	}

	summary, artifactID, latestSequenceNumber, err := buildRunAgentReplaySummary(events)
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("build run-agent replay summary: %w", err)
	}

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("marshal run-agent replay summary: %w", err)
	}

	row, err := r.queries.UpsertRunAgentReplayIndex(ctx, repositorysqlc.UpsertRunAgentReplayIndexParams{
		RunAgentID:           runAgentID,
		ArtifactID:           cloneUUIDPtr(artifactID),
		Summary:              summaryJSON,
		LatestSequenceNumber: cloneInt64Ptr(latestSequenceNumber),
		EventCount:           int64(len(events)),
	})
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("upsert run-agent replay index: %w", err)
	}

	replay, err := mapRunAgentReplay(row)
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("map run-agent replay: %w", err)
	}

	return replay, nil
}

func buildRunAgentReplaySummary(events []RunEvent) (runAgentReplaySummaryDocument, *uuid.UUID, *int64, error) {
	summary := runAgentReplaySummaryDocument{
		SchemaVersion: runAgentReplaySummarySchemaVersion,
		Status:        "not_started",
		Headline:      "Replay not started",
		Steps:         make([]runAgentReplayStepDocument, 0, len(events)),
	}
	if len(events) == 0 {
		return summary, nil, nil, nil
	}

	openSteps := make(map[string][]replayOpenStep)
	artifactSeen := make(map[uuid.UUID]struct{})
	artifactIDs := make([]uuid.UUID, 0)
	var latestArtifactID *uuid.UUID

	for _, event := range events {
		payload, err := decodeReplayPayload(event.Payload)
		if err != nil {
			return runAgentReplaySummaryDocument{}, nil, nil, fmt.Errorf("decode payload for sequence %d: %w", event.SequenceNumber, err)
		}

		if event.ArtifactID != nil {
			latestArtifactID = cloneUUIDPtr(event.ArtifactID)
			if _, ok := artifactSeen[*event.ArtifactID]; !ok {
				artifactSeen[*event.ArtifactID] = struct{}{}
				artifactIDs = append(artifactIDs, *event.ArtifactID)
			}
		}

		replayStepType := replayStepTypeForEvent(event.EventType)
		category := replayStepCategory(event.EventType, payload)
		headline := replayStepHeadline(event.EventType, payload)
		status := replayStatusForEvent(event.EventType)

		switch {
		case isReplayStartEvent(event.EventType):
			step := newReplayStepDocument(event, payload, replayStepType, headline, status)
			summary.Steps = append(summary.Steps, step)
			openSteps[category] = append(openSteps[category], replayOpenStep{
				category: category,
				index:    len(summary.Steps) - 1,
			})
		case isReplayTerminalPairEvent(event.EventType):
			stack := openSteps[category]
			if len(stack) == 0 {
				summary.Steps = append(summary.Steps, newReplayStepDocument(event, payload, replayStepType, headline, status))
				break
			}

			openStep := stack[len(stack)-1]
			openSteps[category] = stack[:len(stack)-1]
			step := summary.Steps[openStep.index]
			step.Status = status
			step.Headline = headline
			step.CompletedSequence = int64Ptr(event.SequenceNumber)
			completedAt := event.OccurredAt.UTC()
			step.CompletedAt = &completedAt
			step.EventCount++
			step.EventTypes = append(step.EventTypes, event.EventType)
			enrichReplayStep(&step, payload)
			addReplayArtifactIDs(&step, event.ArtifactID)
			summary.Steps[openStep.index] = step
		default:
			summary.Steps = append(summary.Steps, newReplayStepDocument(event, payload, replayStepType, headline, status))
		}

		if terminalState := replayTerminalStateForEvent(event, payload, headline, status); terminalState != nil {
			summary.TerminalState = terminalState
		}
	}

	for _, stack := range openSteps {
		for _, openStep := range stack {
			step := summary.Steps[openStep.index]
			if step.Status == "running" {
				step.Headline = incompleteReplayHeadline(step)
				summary.Steps[openStep.index] = step
			}
		}
	}

	summary.ArtifactIDs = artifactIDs
	summary.Counts = replayCounts(summary.Steps, int64(len(events)))
	if summary.TerminalState != nil {
		summary.Status = summary.TerminalState.Status
		summary.Headline = summary.TerminalState.Headline
	} else {
		lastStep := summary.Steps[len(summary.Steps)-1]
		summary.Status = "running"
		summary.Headline = lastStep.Headline
	}

	latestSequenceNumber := events[len(events)-1].SequenceNumber
	return summary, latestArtifactID, &latestSequenceNumber, nil
}

func newReplayStepDocument(event RunEvent, payload map[string]any, stepType string, headline string, status string) runAgentReplayStepDocument {
	step := runAgentReplayStepDocument{
		Type:            stepType,
		Status:          status,
		Headline:        headline,
		Source:          event.Source,
		StartedSequence: event.SequenceNumber,
		OccurredAt:      event.OccurredAt.UTC(),
		EventCount:      1,
		EventTypes:      []runevents.Type{event.EventType},
	}
	enrichReplayStep(&step, payload)
	addReplayArtifactIDs(&step, event.ArtifactID)
	if !isReplayStartEvent(event.EventType) {
		step.CompletedSequence = int64Ptr(event.SequenceNumber)
		completedAt := event.OccurredAt.UTC()
		step.CompletedAt = &completedAt
	}
	return step
}

func enrichReplayStep(step *runAgentReplayStepDocument, payload map[string]any) {
	if stepIndex, ok := replayInt(payload, "step_index"); ok {
		step.StepIndex = &stepIndex
	}
	if providerKey := replayString(payload, "provider_key"); providerKey != "" {
		step.ProviderKey = providerKey
	}
	if providerModelID := replayString(payload, "provider_model_id"); providerModelID != "" {
		step.ProviderModelID = providerModelID
	}
	if step.ProviderModelID == "" {
		if providerModelID := replayString(payload, "model"); providerModelID != "" {
			step.ProviderModelID = providerModelID
		}
	}
	if toolName := replayString(payload, "tool_name"); toolName != "" {
		step.ToolName = toolName
	}
	if subagentKey := replayString(payload, "subagent_key"); subagentKey != "" {
		step.SubagentKey = subagentKey
	}
	if subagentLabel := replayString(payload, "subagent_label"); subagentLabel != "" {
		step.SubagentLabel = subagentLabel
	}
	if sandboxAction := replaySandboxAction(payload); sandboxAction != "" {
		step.SandboxAction = sandboxAction
	}
	if metricKey := replayString(payload, "metric_key"); metricKey != "" {
		step.MetricKey = metricKey
	}
	if finalOutput := replayString(payload, "final_output"); finalOutput != "" {
		step.FinalOutput = finalOutput
	}
	if errMsg := replayErrorMessage(payload); errMsg != "" {
		step.ErrorMessage = errMsg
	}
}

func addReplayArtifactIDs(step *runAgentReplayStepDocument, artifactID *uuid.UUID) {
	if artifactID == nil {
		return
	}
	for _, existing := range step.ArtifactIDs {
		if existing == *artifactID {
			return
		}
	}
	step.ArtifactIDs = append(step.ArtifactIDs, *artifactID)
}

func replayCounts(steps []runAgentReplayStepDocument, eventCount int64) runAgentReplayCounts {
	counts := runAgentReplayCounts{
		Events:      eventCount,
		ReplaySteps: len(steps),
	}
	for _, step := range steps {
		switch step.Type {
		case "agent_step":
			counts.AgentSteps++
		case "model_call":
			counts.ModelCalls++
		case "tool_call":
			counts.ToolCalls++
		case "sandbox_command":
			counts.SandboxCommands++
		case "sandbox_file":
			counts.SandboxFileEvents++
		case "output":
			counts.Outputs++
		case "scoring", "scoring_metric":
			counts.ScoringEvents++
		}
	}
	return counts
}

func replayTerminalStateForEvent(event RunEvent, payload map[string]any, headline string, status string) *runAgentReplayTerminalState {
	if status != "completed" && status != "failed" {
		return nil
	}
	if event.EventType != runevents.EventTypeSystemRunCompleted &&
		event.EventType != runevents.EventTypeSystemRunFailed &&
		event.EventType != runevents.EventTypeScoringCompleted &&
		event.EventType != runevents.EventTypeScoringFailed {
		return nil
	}
	return &runAgentReplayTerminalState{
		Status:         status,
		EventType:      event.EventType,
		Source:         event.Source,
		SequenceNumber: event.SequenceNumber,
		OccurredAt:     event.OccurredAt.UTC(),
		Headline:       headline,
		ErrorMessage:   replayErrorMessage(payload),
	}
}

func replayStepTypeForEvent(eventType runevents.Type) string {
	switch eventType {
	case runevents.EventTypeSystemRunStarted,
		runevents.EventTypeSystemRunCompleted,
		runevents.EventTypeSystemRunFailed:
		return "run"
	case runevents.EventTypeSystemStepStarted,
		runevents.EventTypeSystemStepCompleted:
		return "agent_step"
	case runevents.EventTypeModelCallStarted,
		runevents.EventTypeModelCallCompleted:
		return "model_call"
	case runevents.EventTypeToolCallStarted,
		runevents.EventTypeToolCallCompleted,
		runevents.EventTypeToolCallFailed:
		return "tool_call"
	case runevents.EventTypeSandboxCommandStarted,
		runevents.EventTypeSandboxCommandCompleted,
		runevents.EventTypeSandboxCommandFailed:
		return "sandbox_command"
	case runevents.EventTypeSandboxFileRead,
		runevents.EventTypeSandboxFileWritten,
		runevents.EventTypeSandboxFileListed:
		return "sandbox_file"
	case runevents.EventTypeSystemOutputFinalized:
		return "output"
	case runevents.EventTypeScoringMetricRecorded:
		return "scoring_metric"
	case runevents.EventTypeScoringStarted,
		runevents.EventTypeScoringCompleted,
		runevents.EventTypeScoringFailed:
		return "scoring"
	default:
		return "event"
	}
}

func replayStepCategory(eventType runevents.Type, payload map[string]any) string {
	switch eventType {
	case runevents.EventTypeSystemRunStarted,
		runevents.EventTypeSystemRunCompleted,
		runevents.EventTypeSystemRunFailed:
		return "run"
	case runevents.EventTypeSystemStepStarted,
		runevents.EventTypeSystemStepCompleted:
		if stepIndex, ok := replayInt(payload, "step_index"); ok {
			return fmt.Sprintf("agent_step:%d", stepIndex)
		}
		return "agent_step"
	case runevents.EventTypeModelCallStarted,
		runevents.EventTypeModelCallCompleted:
		return "model_call"
	case runevents.EventTypeToolCallStarted,
		runevents.EventTypeToolCallCompleted,
		runevents.EventTypeToolCallFailed:
		return "tool_call"
	case runevents.EventTypeSandboxCommandStarted,
		runevents.EventTypeSandboxCommandCompleted,
		runevents.EventTypeSandboxCommandFailed:
		return "sandbox_command"
	case runevents.EventTypeScoringStarted,
		runevents.EventTypeScoringCompleted,
		runevents.EventTypeScoringFailed:
		return "scoring"
	default:
		return string(eventType)
	}
}

func replayStepHeadline(eventType runevents.Type, payload map[string]any) string {
	switch eventType {
	case runevents.EventTypeSystemRunStarted:
		return "Run started"
	case runevents.EventTypeSystemRunCompleted:
		return "Run completed"
	case runevents.EventTypeSystemRunFailed:
		return "Run failed"
	case runevents.EventTypeSystemOutputFinalized:
		return "Final output finalized"
	case runevents.EventTypeSystemStepStarted, runevents.EventTypeSystemStepCompleted:
		subagentLabel := replayString(payload, "subagent_label")
		if stepIndex, ok := replayInt(payload, "step_index"); ok {
			if subagentLabel != "" {
				return fmt.Sprintf("%s step %d", subagentLabel, stepIndex)
			}
			return fmt.Sprintf("Agent step %d", stepIndex)
		}
		if subagentLabel != "" {
			return fmt.Sprintf("%s step", subagentLabel)
		}
		return "Agent step"
	case runevents.EventTypeModelCallStarted, runevents.EventTypeModelCallCompleted:
		model := replayString(payload, "provider_model_id")
		if model == "" {
			model = replayString(payload, "model")
		}
		subagentLabel := replayString(payload, "subagent_label")
		if model != "" {
			if subagentLabel != "" {
				return fmt.Sprintf("%s model call to %s", subagentLabel, model)
			}
			return fmt.Sprintf("Model call to %s", model)
		}
		if subagentLabel != "" {
			return fmt.Sprintf("%s model call", subagentLabel)
		}
		return "Model call"
	case runevents.EventTypeToolCallStarted, runevents.EventTypeToolCallCompleted, runevents.EventTypeToolCallFailed:
		subagentLabel := replayString(payload, "subagent_label")
		if toolName := replayString(payload, "tool_name"); toolName != "" {
			if subagentLabel != "" {
				return fmt.Sprintf("%s tool call: %s", subagentLabel, toolName)
			}
			return fmt.Sprintf("Tool call: %s", toolName)
		}
		if subagentLabel != "" {
			return fmt.Sprintf("%s tool call", subagentLabel)
		}
		return "Tool call"
	case runevents.EventTypeSandboxCommandStarted, runevents.EventTypeSandboxCommandCompleted, runevents.EventTypeSandboxCommandFailed:
		return "Sandbox command"
	case runevents.EventTypeSandboxFileRead:
		return "Sandbox file read"
	case runevents.EventTypeSandboxFileWritten:
		return "Sandbox file write"
	case runevents.EventTypeSandboxFileListed:
		return "Sandbox file listing"
	case runevents.EventTypeScoringStarted:
		return "Scoring started"
	case runevents.EventTypeScoringCompleted:
		return "Scoring completed"
	case runevents.EventTypeScoringFailed:
		return "Scoring failed"
	case runevents.EventTypeScoringMetricRecorded:
		if metricKey := replayString(payload, "metric_key"); metricKey != "" {
			return fmt.Sprintf("Metric recorded: %s", metricKey)
		}
		return "Metric recorded"
	default:
		return string(eventType)
	}
}

func incompleteReplayHeadline(step runAgentReplayStepDocument) string {
	switch step.Type {
	case "run":
		return "Run interrupted"
	case "agent_step":
		if step.StepIndex != nil {
			return fmt.Sprintf("Agent step %d interrupted", *step.StepIndex)
		}
		return "Agent step interrupted"
	case "model_call":
		return "Model call interrupted"
	case "tool_call":
		return "Tool call interrupted"
	case "sandbox_command":
		return "Sandbox command interrupted"
	case "scoring":
		return "Scoring interrupted"
	default:
		return step.Headline
	}
}

func replayStatusForEvent(eventType runevents.Type) string {
	switch eventType {
	case runevents.EventTypeSystemRunCompleted,
		runevents.EventTypeSystemStepCompleted,
		runevents.EventTypeModelCallCompleted,
		runevents.EventTypeToolCallCompleted,
		runevents.EventTypeSandboxCommandCompleted,
		runevents.EventTypeScoringCompleted:
		return "completed"
	case runevents.EventTypeSystemRunFailed,
		runevents.EventTypeToolCallFailed,
		runevents.EventTypeSandboxCommandFailed,
		runevents.EventTypeScoringFailed:
		return "failed"
	default:
		return "running"
	}
}

func isReplayStartEvent(eventType runevents.Type) bool {
	switch eventType {
	case runevents.EventTypeSystemRunStarted,
		runevents.EventTypeSystemStepStarted,
		runevents.EventTypeModelCallStarted,
		runevents.EventTypeToolCallStarted,
		runevents.EventTypeSandboxCommandStarted,
		runevents.EventTypeScoringStarted:
		return true
	default:
		return false
	}
}

func isReplayTerminalPairEvent(eventType runevents.Type) bool {
	switch eventType {
	case runevents.EventTypeSystemRunCompleted,
		runevents.EventTypeSystemRunFailed,
		runevents.EventTypeSystemStepCompleted,
		runevents.EventTypeModelCallCompleted,
		runevents.EventTypeToolCallCompleted,
		runevents.EventTypeToolCallFailed,
		runevents.EventTypeSandboxCommandCompleted,
		runevents.EventTypeSandboxCommandFailed,
		runevents.EventTypeScoringCompleted,
		runevents.EventTypeScoringFailed:
		return true
	default:
		return false
	}
}

func decodeReplayPayload(payload json.RawMessage) (map[string]any, error) {
	if len(payload) == 0 {
		return map[string]any{}, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func replayString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return str
}

func replayInt(payload map[string]any, key string) (int, bool) {
	value, ok := payload[key]
	if !ok {
		return 0, false
	}
	number, ok := value.(float64)
	if !ok {
		return 0, false
	}
	return int(number), true
}

func replayErrorMessage(payload map[string]any) string {
	if errMsg := replayString(payload, "error"); errMsg != "" {
		return errMsg
	}
	if errMsg := replayString(payload, "error_message"); errMsg != "" {
		return errMsg
	}
	return ""
}

func replaySandboxAction(payload map[string]any) string {
	if action := replayString(payload, "action"); action != "" {
		return action
	}
	if action := replayString(payload, "sandbox_action"); action != "" {
		return action
	}
	return ""
}
