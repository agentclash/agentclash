package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/pubsub"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var runEventStreamPollInterval = 750 * time.Millisecond

// registerEventStreamRoute adds the SSE endpoint for live run event streaming.
// Browsers using EventSource cannot set custom headers, so the endpoint keeps
// query-token fallback while preferring normal Authorization header auth.
func registerEventStreamRoute(
	router chi.Router,
	logger *slog.Logger,
	authenticator Authenticator,
	runReadService RunReadService,
	subscriber pubsub.EventSubscriber,
) {
	router.Get("/v1/runs/{runID}/events/stream", streamRunEventsHandler(logger, authenticator, runReadService, subscriber))
}

func streamRunEventsHandler(
	logger *slog.Logger,
	authenticator Authenticator,
	runReadService RunReadService,
	subscriber pubsub.EventSubscriber,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		streamService, ok := runReadService.(RunEventStreamService)
		if !ok {
			logger.Error("run read service does not implement run event streaming")
			writeError(w, http.StatusInternalServerError, "internal_error", "run event streaming is unavailable")
			return
		}

		// 1. Parse run ID from URL.
		rawRunID := chi.URLParam(r, "runID")
		runID, err := uuid.Parse(rawRunID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", "run ID must be a valid UUID")
			return
		}

		// 2. Authenticate via Authorization header, with query-token fallback for
		// browser EventSource clients that cannot set custom headers.
		authReq := r.Clone(r.Context())
		if authReq.Header.Get("Authorization") == "" {
			token := r.URL.Query().Get("token")
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing_token", "Authorization header or token query parameter is required")
				return
			}
			authReq.Header.Set("Authorization", "Bearer "+token)
		}
		caller, err := authenticator.Authenticate(authReq)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired credentials")
			return
		}

		// 3. Load the initial persisted snapshot. This both authorizes access and
		// gives us a catch-up source even when Redis pub/sub is unavailable.
		snapshot, err := streamService.ListRunEventStream(r.Context(), caller, runID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunNotFound):
				writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("failed to load run event stream snapshot",
					"run_id", runID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		// 4. Verify the response writer supports flushing.
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "internal_error", "streaming not supported")
			return
		}

		// 5. Set SSE headers.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // nginx compatibility
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		lastEventID := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
		startIndex := replayStartIndex(snapshot.Events, lastEventID)
		if lastEventID != "" && startIndex == 0 {
			logger.Warn("last event id not found in persisted snapshot; replaying full run stream",
				"run_id", runID,
				"last_event_id", lastEventID,
			)
		}

		delivered := make(map[string]struct{}, len(snapshot.Events))
		if err := emitPersistedRunEvents(w, flusher, snapshot.Events[startIndex:], delivered); err != nil {
			return
		}
		if isTerminalRunStatus(snapshot.Run.Status) {
			return
		}

		// 6. Subscribe to live events when available. Polling the persisted event
		// store remains the source of truth and catches up any missed pub/sub
		// messages.
		var eventCh <-chan []byte
		if _, noop := subscriber.(pubsub.NoopSubscriber); !noop {
			ch, err := subscriber.Subscribe(r.Context(), runID)
			if err != nil {
				logger.Warn("failed to subscribe to live run events; falling back to persisted polling",
					"run_id", runID,
					"error", err,
				)
			} else {
				eventCh = ch
			}
		}

		ticker := time.NewTicker(runEventStreamPollInterval)
		defer ticker.Stop()

		// 7. Stream live events and periodically catch up from persisted storage.
		for {
			select {
			case data, ok := <-eventCh:
				if !ok {
					eventCh = nil
					continue
				}
				if err := emitLiveRunEvent(w, flusher, data, delivered); err != nil {
					return
				}
			case <-ticker.C:
				snapshot, err := streamService.ListRunEventStream(r.Context(), caller, runID)
				if err != nil {
					switch {
					case errors.Is(err, repository.ErrRunNotFound), errors.Is(err, ErrForbidden):
						return
					default:
						logger.Warn("failed to refresh persisted run events",
							"run_id", runID,
							"error", err,
						)
						continue
					}
				}
				if err := emitPersistedRunEvents(w, flusher, snapshot.Events, delivered); err != nil {
					return
				}
				if isTerminalRunStatus(snapshot.Run.Status) {
					return
				}
			case <-r.Context().Done():
				return // client disconnected
			}
		}
	}
}

func emitPersistedRunEvents(
	w http.ResponseWriter,
	flusher http.Flusher,
	events []repository.RunEvent,
	delivered map[string]struct{},
) error {
	for _, event := range events {
		data, streamEventID, err := marshalPersistedRunEvent(event)
		if err != nil {
			return err
		}
		if _, seen := delivered[streamEventID]; seen {
			continue
		}
		delivered[streamEventID] = struct{}{}
		writeSSEFrame(w, flusher, streamEventID, data)
	}
	return nil
}

func emitLiveRunEvent(
	w http.ResponseWriter,
	flusher http.Flusher,
	data []byte,
	delivered map[string]struct{},
) error {
	streamEventID := extractStreamEventID(data)
	if _, seen := delivered[streamEventID]; seen {
		return nil
	}
	delivered[streamEventID] = struct{}{}
	writeSSEFrame(w, flusher, streamEventID, data)
	return nil
}

func marshalPersistedRunEvent(event repository.RunEvent) ([]byte, string, error) {
	streamEventID := persistedStreamEventID(event.RunAgentID, event.SequenceNumber)
	envelope := runevents.Envelope{
		EventID:        streamEventID,
		SchemaVersion:  runevents.SchemaVersionV1,
		RunID:          event.RunID,
		RunAgentID:     event.RunAgentID,
		SequenceNumber: event.SequenceNumber,
		EventType:      event.EventType,
		Source:         event.Source,
		OccurredAt:     event.OccurredAt.UTC(),
		Payload:        append([]byte(nil), event.Payload...),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, "", fmt.Errorf("marshal persisted run event: %w", err)
	}
	return data, streamEventID, nil
}

func replayStartIndex(events []repository.RunEvent, lastEventID string) int {
	if lastEventID == "" {
		return 0
	}
	for i, event := range events {
		if persistedStreamEventID(event.RunAgentID, event.SequenceNumber) == lastEventID {
			return i + 1
		}
	}
	return 0
}

func writeSSEFrame(w http.ResponseWriter, flusher http.Flusher, id string, data []byte) {
	fmt.Fprintf(w, "id: %s\n", id)
	fmt.Fprintf(w, "event: run_event\n")
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func persistedStreamEventID(runAgentID uuid.UUID, sequenceNumber int64) string {
	if runAgentID == uuid.Nil || sequenceNumber <= 0 {
		return "0"
	}
	return fmt.Sprintf("persisted:%s:%d", runAgentID.String(), sequenceNumber)
}

func isTerminalRunStatus(status domain.RunStatus) bool {
	switch status {
	case domain.RunStatusCompleted, domain.RunStatusFailed, domain.RunStatusCancelled:
		return true
	default:
		return false
	}
}

// extractStreamEventID derives a stable SSE event ID from the persisted wire
// identity of a run event. Sequence numbers are per run-agent, so the stream
// ID must include both the run-agent ID and its sequence number.
func extractStreamEventID(data []byte) string {
	var envelope struct {
		RunAgentID     uuid.UUID `json:"run_agent_id"`
		SequenceNumber int64     `json:"sequence_number"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "0"
	}
	return persistedStreamEventID(envelope.RunAgentID, envelope.SequenceNumber)
}
