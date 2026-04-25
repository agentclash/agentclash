package pubsub

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/worker"
)

// StandingsRecorder wraps a worker.RunEventRecorder and mirrors a subset
// of the event stream into a per-run Redis hash (via StandingsStore). The
// hash backs the race-context feature from issue #400: when enabled on a
// run, the native executor reads the hash at step boundaries to build the
// newswire message injected into each agent's context.
//
// The recorder is intentionally observational. Errors from the store are
// logged and swallowed — the database remains the source of truth, and
// standings being stale or missing must never break event recording.
type StandingsRecorder struct {
	inner  worker.RunEventRecorder
	store  StandingsStore
	logger *slog.Logger
}

var _ worker.RunEventRecorder = (*StandingsRecorder)(nil)

// NewStandingsRecorder wraps `inner` so each persisted event also updates
// the standings store. Pass a NoopStandingsStore when Redis is unavailable
// so the overall pipeline stays uniform.
func NewStandingsRecorder(inner worker.RunEventRecorder, store StandingsStore, logger *slog.Logger) *StandingsRecorder {
	return &StandingsRecorder{inner: inner, store: store, logger: logger}
}

func (r *StandingsRecorder) RecordRunEvent(ctx context.Context, params repository.RecordRunEventParams) (repository.RunEvent, error) {
	event, err := r.inner.RecordRunEvent(ctx, params)
	if err != nil {
		return event, err
	}

	if r.store == nil {
		return event, nil
	}

	update, ok := standingsUpdateFor(params.Event)
	if !ok {
		return event, nil
	}
	update.RunAgentID = params.Event.RunAgentID

	if updateErr := r.store.Update(ctx, params.Event.RunID, update); updateErr != nil {
		r.logger.Warn("failed to update race-context standings",
			"run_id", params.Event.RunID,
			"run_agent_id", params.Event.RunAgentID,
			"event_type", string(params.Event.EventType),
			"error", updateErr,
		)
	}
	return event, nil
}

// standingsUpdateFor derives a partial StandingsEntry from an event.
// Returns (_, false) when the event is not relevant to standings — most
// events aren't. We only track lifecycle, step progression, tool counts,
// token spend, and terminal outcomes.
func standingsUpdateFor(env runevents.Envelope) (StandingsEntry, bool) {
	switch env.EventType {
	case runevents.EventTypeSystemRunStarted:
		now := env.OccurredAt
		if now.IsZero() {
			now = time.Now().UTC()
		}
		return StandingsEntry{
			State:     StandingsStateRunning,
			StartedAt: &now,
		}, true

	case runevents.EventTypeSystemStepStarted:
		return StandingsEntry{
			Step:  env.Summary.StepIndex,
			State: StandingsStateRunning,
		}, true

	case runevents.EventTypeToolCallCompleted:
		return StandingsEntry{ToolCalls: 1}, true

	case runevents.EventTypeModelCallCompleted:
		tokens, model := extractModelTokensAndName(env.Payload)
		return StandingsEntry{
			Model:      model,
			TokensUsed: tokens,
		}, true

	case runevents.EventTypeSystemOutputFinalized:
		now := env.OccurredAt
		if now.IsZero() {
			now = time.Now().UTC()
		}
		return StandingsEntry{
			State:       StandingsStateSubmitted,
			SubmittedAt: &now,
		}, true

	case runevents.EventTypeSystemRunFailed:
		now := env.OccurredAt
		if now.IsZero() {
			now = time.Now().UTC()
		}
		state := StandingsStateFailed
		// Distinguish timeouts from generic failures so the newswire
		// renders "TIMED OUT" and `peer_timed_out` triggers fire. The
		// native observer writes stop_reason="timeout" in the failure
		// payload when the run exceeds its runtime budget.
		if isTimeoutFailurePayload(env.Payload) {
			state = StandingsStateTimedOut
		}
		return StandingsEntry{
			State:    state,
			FailedAt: &now,
		}, true
	}

	return StandingsEntry{}, false
}

func isTimeoutFailurePayload(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var payload struct {
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	return payload.StopReason == "timeout"
}

func extractModelTokensAndName(raw json.RawMessage) (int64, string) {
	if len(raw) == 0 {
		return 0, ""
	}
	var payload struct {
		ProviderModelID string `json:"provider_model_id"`
		Usage           struct {
			TotalTokens int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, ""
	}
	return payload.Usage.TotalTokens, payload.ProviderModelID
}
