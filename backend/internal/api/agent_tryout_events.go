package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TryoutTimelineEventType is the stable, product-level category the frontend
// renders as a timeline row. It is derived from the lower-level run event type
// so the UI never has to know about the ~50 internal run event names.
type TryoutTimelineEventType string

const (
	TryoutTimelineStarted        TryoutTimelineEventType = "started"
	TryoutTimelinePlanning       TryoutTimelineEventType = "planning"
	TryoutTimelineToolCall       TryoutTimelineEventType = "tool_call"
	TryoutTimelineSandboxCommand TryoutTimelineEventType = "sandbox_command"
	TryoutTimelineFileWritten    TryoutTimelineEventType = "file_written"
	TryoutTimelineFileActivity   TryoutTimelineEventType = "file_activity"
	TryoutTimelineValidation     TryoutTimelineEventType = "validation"
	TryoutTimelineScoring        TryoutTimelineEventType = "scoring"
	TryoutTimelineFinished       TryoutTimelineEventType = "finished"
	TryoutTimelineActivity       TryoutTimelineEventType = "activity"
)

const (
	defaultTryoutEventLimit = int32(100)
	maxTryoutEventLimit     = int32(200)
)

// TryoutTimelineEvent is one product-level timeline row. It is the JSON shape
// returned by both the public and workspace event endpoints. Payload is the
// optional expandable structured detail — redacted to a safe allow-list for
// public/anonymous consumers and the full run-event payload for workspace
// members.
type TryoutTimelineEvent struct {
	Cursor     int64                   `json:"cursor"`
	Sequence   int64                   `json:"sequence"`
	Type       TryoutTimelineEventType `json:"type"`
	Summary    string                  `json:"summary"`
	OccurredAt time.Time               `json:"occurred_at"`
	Payload    json.RawMessage         `json:"payload,omitempty"`
}

// AgentTryoutEventsResult is what the manager returns for an events query: the
// (status-refreshed) tryout plus a page of mapped timeline events and the
// resumable cursor.
type AgentTryoutEventsResult struct {
	Tryout     repository.AgentTryout
	Events     []TryoutTimelineEvent
	NextCursor int64
	HasMore    bool
}

// TryoutEventsCursor is the pagination input: After is the last seen cursor (0
// to start from the beginning), Limit caps the page size.
type TryoutEventsCursor struct {
	After int64
	Limit int32
}

// safeTryoutPayloadKeys is the allow-list of run-event payload fields that are
// safe to surface in public/anonymous event responses. Anything not listed
// here (command text, stdout/stderr, file contents, model output, args, env,
// credentials, workspace/org ids, etc.) is dropped for public consumers.
var safeTryoutPayloadKeys = map[string]bool{
	"tool_name":         true,
	"tool_category":     true,
	"sandbox_action":    true,
	"exit_code":         true,
	"status":            true,
	"step_index":        true,
	"metric_key":        true,
	"metric_value":      true,
	"score":             true,
	"provider_key":      true,
	"provider_model_id": true,
	"duration_ms":       true,
	"latency_ms":        true,
	"path":              true,
	"file_path":         true,
	"relative_path":     true,
	"validator":         true,
	"verdict":           true,
	"outcome":           true,
	"passed":            true,
}

// classifyTryoutEvent maps an internal run event type to a stable product-level
// timeline category. The bool is false for events that should not appear in the
// tryout timeline at all (e.g. high-frequency streaming deltas).
func classifyTryoutEvent(t runevents.Type) (TryoutTimelineEventType, bool) {
	switch t {
	case runevents.EventTypeModelOutputDelta:
		// Token-level streaming noise; the timeline shows the call, not deltas.
		return "", false
	case runevents.EventTypeSystemRunStarted:
		return TryoutTimelineStarted, true
	case runevents.EventTypeSystemStepStarted,
		runevents.EventTypeSystemStepCompleted,
		runevents.EventTypeModelCallStarted,
		runevents.EventTypeModelCallCompleted,
		runevents.EventTypeModelToolCallsProposed:
		return TryoutTimelinePlanning, true
	case runevents.EventTypeToolCallStarted,
		runevents.EventTypeToolCallCompleted,
		runevents.EventTypeToolCallFailed:
		return TryoutTimelineToolCall, true
	case runevents.EventTypeSandboxCommandStarted,
		runevents.EventTypeSandboxCommandCompleted,
		runevents.EventTypeSandboxCommandFailed:
		return TryoutTimelineSandboxCommand, true
	case runevents.EventTypeSandboxFileWritten:
		return TryoutTimelineFileWritten, true
	case runevents.EventTypeSandboxFileRead,
		runevents.EventTypeSandboxFileListed:
		return TryoutTimelineFileActivity, true
	case runevents.EventTypeGraderVerificationFileCaptured,
		runevents.EventTypeGraderVerificationDirectoryListed,
		runevents.EventTypeGraderVerificationCodeExecuted:
		return TryoutTimelineValidation, true
	case runevents.EventTypeScoringStarted,
		runevents.EventTypeScoringMetricRecorded,
		runevents.EventTypeScoringCompleted,
		runevents.EventTypeScoringFailed:
		return TryoutTimelineScoring, true
	case runevents.EventTypeSystemOutputFinalized,
		runevents.EventTypeSystemRunCompleted,
		runevents.EventTypeSystemRunFailed:
		return TryoutTimelineFinished, true
	default:
		// Keep unknown-but-present events visible rather than silently dropping
		// them, so the timeline stays complete as new event types are added.
		return TryoutTimelineActivity, true
	}
}

// tryoutEventSummary builds a short, human-readable, secret-free summary line.
// It only reads allow-listed fields, so it never echoes command text, file
// contents, or model output.
func tryoutEventSummary(t runevents.Type, facts map[string]any) string {
	switch t {
	case runevents.EventTypeSystemRunStarted:
		return "Run started"
	case runevents.EventTypeSystemStepStarted:
		return withStep("Planning step", facts)
	case runevents.EventTypeSystemStepCompleted:
		return withStep("Completed step", facts)
	case runevents.EventTypeModelCallStarted:
		return joinSummary("Model planning", factString(facts, "provider_model_id"))
	case runevents.EventTypeModelCallCompleted:
		return joinSummary("Model responded", factString(facts, "provider_model_id"))
	case runevents.EventTypeModelToolCallsProposed:
		return "Model proposed tool calls"
	case runevents.EventTypeToolCallStarted:
		return joinSummary("Tool call started", factString(facts, "tool_name"))
	case runevents.EventTypeToolCallCompleted:
		return joinSummary("Tool call completed", factString(facts, "tool_name"))
	case runevents.EventTypeToolCallFailed:
		return joinSummary("Tool call failed", factString(facts, "tool_name"))
	case runevents.EventTypeSandboxCommandStarted:
		return "Sandbox command started"
	case runevents.EventTypeSandboxCommandCompleted:
		return joinSummary("Sandbox command completed", exitCodeNote(facts))
	case runevents.EventTypeSandboxCommandFailed:
		return joinSummary("Sandbox command failed", exitCodeNote(facts))
	case runevents.EventTypeSandboxFileWritten:
		return joinSummary("Wrote file", factPath(facts))
	case runevents.EventTypeSandboxFileRead:
		return joinSummary("Read file", factPath(facts))
	case runevents.EventTypeSandboxFileListed:
		return joinSummary("Listed files", factPath(facts))
	case runevents.EventTypeGraderVerificationFileCaptured,
		runevents.EventTypeGraderVerificationDirectoryListed,
		runevents.EventTypeGraderVerificationCodeExecuted:
		return "Validation"
	case runevents.EventTypeScoringStarted:
		return "Scoring started"
	case runevents.EventTypeScoringMetricRecorded:
		return joinSummary("Scored", factString(facts, "metric_key"))
	case runevents.EventTypeScoringCompleted:
		return "Scoring completed"
	case runevents.EventTypeScoringFailed:
		return "Scoring failed"
	case runevents.EventTypeSystemOutputFinalized:
		return "Output finalized"
	case runevents.EventTypeSystemRunCompleted:
		return "Run completed"
	case runevents.EventTypeSystemRunFailed:
		return "Run failed"
	default:
		return string(t)
	}
}

func mapTryoutTimelineEvent(event repository.RunEvent, redact bool) (TryoutTimelineEvent, bool) {
	eventType, include := classifyTryoutEvent(event.EventType)
	if !include {
		return TryoutTimelineEvent{}, false
	}
	facts := decodeTryoutPayloadFacts(event.Payload)
	var payload json.RawMessage
	if redact {
		payload = redactTryoutPayload(facts)
	} else {
		payload = nonEmptyJSON(event.Payload)
	}
	return TryoutTimelineEvent{
		Cursor:     event.ID,
		Sequence:   event.SequenceNumber,
		Type:       eventType,
		Summary:    tryoutEventSummary(event.EventType, facts),
		OccurredAt: event.OccurredAt.UTC(),
		Payload:    payload,
	}, true
}

func decodeTryoutPayloadFacts(payload json.RawMessage) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	var facts map[string]any
	if err := json.Unmarshal(payload, &facts); err != nil {
		return nil
	}
	return facts
}

// redactTryoutPayload returns a new JSON object containing only allow-listed,
// non-sensitive fields, or nil when nothing safe is present.
func redactTryoutPayload(facts map[string]any) json.RawMessage {
	if len(facts) == 0 {
		return nil
	}
	safe := make(map[string]any, len(facts))
	for key, value := range facts {
		if safeTryoutPayloadKeys[key] {
			safe[key] = value
		}
	}
	if len(safe) == 0 {
		return nil
	}
	encoded, err := json.Marshal(safe)
	if err != nil {
		return nil
	}
	return encoded
}

func nonEmptyJSON(payload json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return nil
	}
	return payload
}

func factString(facts map[string]any, key string) string {
	if facts == nil {
		return ""
	}
	if value, ok := facts[key].(string); ok {
		return value
	}
	return ""
}

func factPath(facts map[string]any) string {
	for _, key := range []string{"path", "file_path", "relative_path"} {
		if value := factString(facts, key); value != "" {
			return value
		}
	}
	return ""
}

func withStep(prefix string, facts map[string]any) string {
	if facts != nil {
		if value, ok := facts["step_index"].(float64); ok {
			return fmt.Sprintf("%s %d", prefix, int(value))
		}
	}
	return prefix
}

func exitCodeNote(facts map[string]any) string {
	if facts != nil {
		if value, ok := facts["exit_code"].(float64); ok {
			return fmt.Sprintf("exit %d", int(value))
		}
	}
	return ""
}

func joinSummary(prefix, detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return prefix
	}
	return prefix + ": " + detail
}

// GetPublicTryoutEvents returns the timeline for an anonymous (unclaimed)
// tryout. Payloads are redacted because the endpoint is unauthenticated and
// these tryouts are publicly shareable.
func (m *AgentTryoutManager) GetPublicTryoutEvents(ctx context.Context, id uuid.UUID, cursor TryoutEventsCursor) (AgentTryoutEventsResult, error) {
	tryout, err := m.repo.GetAgentTryoutByID(ctx, id)
	if err != nil {
		return AgentTryoutEventsResult{}, err
	}
	if tryout.WorkspaceID != nil {
		return AgentTryoutEventsResult{}, repository.ErrAgentTryoutNotFound
	}
	tryout = m.refreshTryoutFromExecution(ctx, tryout)
	return m.buildTryoutEvents(ctx, tryout, cursor, true)
}

// GetWorkspaceTryoutEvents returns the timeline for a workspace-owned tryout
// after authorizing the caller. Workspace members receive full event payloads.
func (m *AgentTryoutManager) GetWorkspaceTryoutEvents(ctx context.Context, caller Caller, id uuid.UUID, cursor TryoutEventsCursor) (AgentTryoutEventsResult, error) {
	tryout, err := m.repo.GetAgentTryoutByID(ctx, id)
	if err != nil {
		return AgentTryoutEventsResult{}, err
	}
	if tryout.WorkspaceID == nil {
		return AgentTryoutEventsResult{}, repository.ErrAgentTryoutNotFound
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, *tryout.WorkspaceID); err != nil {
		return AgentTryoutEventsResult{}, err
	}
	tryout = m.refreshTryoutFromExecution(ctx, tryout)
	return m.buildTryoutEvents(ctx, tryout, cursor, false)
}

func (m *AgentTryoutManager) buildTryoutEvents(ctx context.Context, tryout repository.AgentTryout, cursor TryoutEventsCursor, redact bool) (AgentTryoutEventsResult, error) {
	result := AgentTryoutEventsResult{
		Tryout:     tryout,
		Events:     []TryoutTimelineEvent{},
		NextCursor: cursor.After,
	}
	// No linked run yet (still queued): return an empty, resumable timeline plus
	// the current status snapshot rather than failing.
	if tryout.RunID == nil {
		return result, nil
	}

	limit := normalizeTryoutEventLimit(cursor.Limit)
	// Fetch one extra row to detect whether more pages remain.
	rows, err := m.repo.ListRunEventsByRunIDAfter(ctx, *tryout.RunID, cursor.After, limit+1)
	if err != nil {
		return AgentTryoutEventsResult{}, err
	}
	// HasMore reflects raw run-event rows scanned, not displayable events
	// produced. If every row in this batch is a filtered type, Events ends up
	// empty while HasMore stays true — clients must advance via NextCursor and
	// rate-limit their polling rather than treat HasMore as "more events to
	// show". See the AgentTryoutEventsResponse.has_more docs in the OpenAPI spec.
	if int32(len(rows)) > limit {
		result.HasMore = true
		rows = rows[:limit]
	}
	for _, row := range rows {
		// Advance the cursor for every scanned row (even skipped ones) so a page
		// full of filtered events still makes forward progress.
		result.NextCursor = row.ID
		item, include := mapTryoutTimelineEvent(row, redact)
		if !include {
			continue
		}
		result.Events = append(result.Events, item)
	}
	return result, nil
}

func normalizeTryoutEventLimit(limit int32) int32 {
	if limit <= 0 {
		return defaultTryoutEventLimit
	}
	if limit > maxTryoutEventLimit {
		return maxTryoutEventLimit
	}
	return limit
}

type agentTryoutEventsResponse struct {
	TryoutID      uuid.UUID                    `json:"tryout_id"`
	Status        repository.AgentTryoutStatus `json:"status"`
	RunID         *uuid.UUID                   `json:"run_id,omitempty"`
	CostLimitUSD  float64                      `json:"cost_limit_usd"`
	ActualCostUSD *float64                     `json:"actual_cost_usd,omitempty"`
	LatencyMS     *int64                       `json:"latency_ms,omitempty"`
	Events        []TryoutTimelineEvent        `json:"events"`
	NextCursor    int64                        `json:"next_cursor"`
	HasMore       bool                         `json:"has_more"`
}

func mapAgentTryoutEventsResponse(result AgentTryoutEventsResult) agentTryoutEventsResponse {
	events := result.Events
	if events == nil {
		events = []TryoutTimelineEvent{}
	}
	return agentTryoutEventsResponse{
		TryoutID:      result.Tryout.ID,
		Status:        result.Tryout.Status,
		RunID:         result.Tryout.RunID,
		CostLimitUSD:  result.Tryout.CostLimitUSD,
		ActualCostUSD: result.Tryout.ActualCostUSD,
		LatencyMS:     result.Tryout.LatencyMS,
		Events:        events,
		NextCursor:    result.NextCursor,
		HasMore:       result.HasMore,
	}
}

func parseTryoutEventsCursor(r *http.Request) TryoutEventsCursor {
	cursor := TryoutEventsCursor{}
	if raw := strings.TrimSpace(r.URL.Query().Get("after")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			cursor.After = value
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 32); err == nil {
			cursor.Limit = int32(value)
		}
	}
	return cursor
}

func getPublicAgentTryoutEventsHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		result, err := service.GetPublicTryoutEvents(r.Context(), id, parseTryoutEventsCursor(r))
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAgentTryoutEventsResponse(result))
	}
}

func getWorkspaceAgentTryoutEventsHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace_id must be a UUID")
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		result, err := service.GetWorkspaceTryoutEvents(r.Context(), caller, id, parseTryoutEventsCursor(r))
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		if result.Tryout.WorkspaceID == nil || *result.Tryout.WorkspaceID != workspaceID {
			writeError(w, http.StatusNotFound, "agent_tryout_not_found", "agent tryout not found")
			return
		}
		writeJSON(w, http.StatusOK, mapAgentTryoutEventsResponse(result))
	}
}
