package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

type GetRunAgentTranscriptResult struct {
	RunAgent domain.RunAgent
	State    ReplayState
	Message  string
	Turns    []runevents.TranscriptTurn
}

func (m *ReplayReadManager) GetRunAgentTranscript(ctx context.Context, caller Caller, runAgentID uuid.UUID) (GetRunAgentTranscriptResult, error) {
	runAgent, err := m.repo.GetRunAgentByID(ctx, runAgentID)
	if err != nil {
		return GetRunAgentTranscriptResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, runAgent.WorkspaceID); err != nil {
		return GetRunAgentTranscriptResult{}, err
	}

	events, err := m.repo.ListRunEventsByRunAgentID(ctx, runAgentID)
	if err != nil {
		return GetRunAgentTranscriptResult{}, err
	}

	envelopes := transcriptEnvelopesFromRunEvents(events)
	turns, err := runevents.TranscriptFromEvents(envelopes)
	if err != nil {
		return GetRunAgentTranscriptResult{}, err
	}

	// A transcript is "ready" whenever the run-agent has reached a terminal
	// state, even if there are zero turns (a single-turn run simply has none).
	// While the run-agent is still progressing, surface pending so the client
	// can poll.
	state, message := transcriptState(runAgent.Status, len(turns))
	return GetRunAgentTranscriptResult{
		RunAgent: runAgent,
		State:    state,
		Message:  message,
		Turns:    turns,
	}, nil
}

// transcriptEnvelopesFromRunEvents adapts stored run events into the envelope
// shape TranscriptFromEvents consumes. The summary fields the reconstruction
// relies on (turn_index, phase_id, actor, mismatch) are written into each
// turn event's payload by the multi-turn observer, so we lift them back out
// here rather than depend on the separately-stored summary column.
func transcriptEnvelopesFromRunEvents(events []repository.RunEvent) []runevents.Envelope {
	envelopes := make([]runevents.Envelope, 0, len(events))
	for _, event := range events {
		if !isTranscriptEventType(event.EventType) {
			continue
		}
		envelopes = append(envelopes, runevents.Envelope{
			SchemaVersion:  runevents.SchemaVersionV1,
			RunID:          event.RunID,
			RunAgentID:     event.RunAgentID,
			SequenceNumber: event.SequenceNumber,
			EventType:      event.EventType,
			Source:         event.Source,
			OccurredAt:     event.OccurredAt,
			Payload:        event.Payload,
			Summary:        transcriptSummaryFromPayload(event.Payload),
		})
	}
	return envelopes
}

func isTranscriptEventType(eventType runevents.Type) bool {
	switch eventType {
	case runevents.EventTypeTurnUserMessage,
		runevents.EventTypeTurnUserSimulated,
		runevents.EventTypeTurnAssistantMessage,
		runevents.EventTypeTurnCompleted,
		runevents.EventTypeTurnAwaitingHuman,
		runevents.EventTypeTurnStateCaptured,
		runevents.EventTypeConversationCompleted:
		return true
	default:
		return false
	}
}

func transcriptSummaryFromPayload(payload json.RawMessage) runevents.SummaryMetadata {
	var fields struct {
		TurnIndex *int   `json:"turn_index"`
		PhaseID   string `json:"phase_id"`
		Actor     string `json:"actor"`
		Mismatch  *bool  `json:"mismatch"`
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &fields)
	}
	return runevents.SummaryMetadata{
		TurnIndex: fields.TurnIndex,
		PhaseID:   fields.PhaseID,
		Actor:     fields.Actor,
		Mismatch:  fields.Mismatch,
	}
}

func transcriptState(status domain.RunAgentStatus, turnCount int) (ReplayState, string) {
	switch status {
	case domain.RunAgentStatusCompleted:
		return ReplayStateReady, ""
	case domain.RunAgentStatusFailed:
		// Mirror the replay/scorecard endpoints: a failed run-agent surfaces
		// `errored` (HTTP 409) so a consumer checking `state == "ready"`
		// never mistakes an interrupted conversation for a clean finish. The
		// reconstructed turns are still returned in the body, so callers that
		// opt in (allowedStatuses: [409]) can render the partial transcript.
		return ReplayStateErrored, "the run-agent failed; the transcript may be partial"
	case domain.RunAgentStatusQueued,
		domain.RunAgentStatusReady,
		domain.RunAgentStatusExecuting,
		domain.RunAgentStatusEvaluating:
		if turnCount > 0 {
			// Turns are streaming in while the run is still active.
			return ReplayStateReady, ""
		}
		return ReplayStatePending, "transcript is being recorded"
	default:
		return ReplayStatePending, "transcript is being recorded"
	}
}

// --- HTTP layer ---

type transcriptTurnPayload struct {
	TurnIndex         int    `json:"turn_index"`
	PhaseID           string `json:"phase_id,omitempty"`
	Actor             string `json:"actor,omitempty"`
	UserMessage       string `json:"user_message,omitempty"`
	AssistantMessage  string `json:"assistant_message,omitempty"`
	Mismatch          bool   `json:"mismatch"`
	Completed         bool   `json:"completed"`
	AwaitingHuman     bool   `json:"awaiting_human"`
	AwaitingHumanHint string `json:"awaiting_human_hint,omitempty"`
	UserSimulated     bool   `json:"user_simulated"`
}

type getRunAgentTranscriptResponse struct {
	State          ReplayState             `json:"state"`
	Message        string                  `json:"message,omitempty"`
	RunAgentID     uuid.UUID               `json:"run_agent_id"`
	RunID          uuid.UUID               `json:"run_id"`
	RunAgentStatus domain.RunAgentStatus   `json:"run_agent_status"`
	TurnCount      int                     `json:"turn_count"`
	Turns          []transcriptTurnPayload `json:"turns"`
}

func getRunAgentTranscriptHandler(logger *slog.Logger, service ReplayReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		runAgentID, err := runAgentIDFromURLParam("runAgentID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_agent_id", err.Error())
			return
		}

		result, err := service.GetRunAgentTranscript(r.Context(), caller, runAgentID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunAgentNotFound):
				writeError(w, http.StatusNotFound, "run_agent_not_found", "run agent not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("get run-agent transcript request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"run_agent_id", runAgentID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		statusCode := http.StatusOK
		switch result.State {
		case ReplayStatePending:
			statusCode = http.StatusAccepted
		case ReplayStateErrored:
			statusCode = http.StatusConflict
		}
		writeJSON(w, statusCode, buildRunAgentTranscriptResponse(result))
	}
}

func buildRunAgentTranscriptResponse(result GetRunAgentTranscriptResult) getRunAgentTranscriptResponse {
	turns := make([]transcriptTurnPayload, 0, len(result.Turns))
	for _, turn := range result.Turns {
		turns = append(turns, transcriptTurnPayload{
			TurnIndex:         turn.TurnIndex,
			PhaseID:           turn.PhaseID,
			Actor:             turn.Actor,
			UserMessage:       turn.UserMessage,
			AssistantMessage:  turn.AssistantMessage,
			Mismatch:          turn.Mismatch,
			Completed:         turn.Completed,
			AwaitingHuman:     turn.AwaitingHuman,
			AwaitingHumanHint: turn.AwaitingHumanHint,
			UserSimulated:     turn.UserSimulated,
		})
	}
	return getRunAgentTranscriptResponse{
		State:          result.State,
		Message:        result.Message,
		RunAgentID:     result.RunAgent.ID,
		RunID:          result.RunAgent.RunID,
		RunAgentStatus: result.RunAgent.Status,
		TurnCount:      len(turns),
		Turns:          turns,
	}
}
