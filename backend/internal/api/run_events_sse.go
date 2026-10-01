package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/pubsub"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var runEventStreamPollInterval = 750 * time.Millisecond

const runEventStreamWriteTimeout = 10 * time.Second

type runEventStreamOptions struct {
	heartbeatInterval time.Duration
	shutdownContext   context.Context
}

// SSEConnectionGate limits concurrent SSE streams (Fleet 14). nil = unlimited.
type SSEConnectionGate interface {
	TryAcquire(ctx context.Context) bool
	Release(ctx context.Context)
}

// registerEventStreamRoute adds the SSE endpoint for live run event streaming.
// Browsers using EventSource cannot set custom headers, so the endpoint keeps
// query-token fallback while preferring normal Authorization header auth.
// RunEventPayloadResolverProvider optionally exposes a claim-check resolver for
// hydrating offloaded payloads on the live Redis SSE path.
// sseGate is injected via Server Config / routerOptions (not a package global).
type RunEventPayloadResolverProvider interface {
	RunEventPayloadResolver() *runevents.Resolver
}

func registerEventStreamRoute(
	router chi.Router,
	logger *slog.Logger,
	authenticator Authenticator,
	runReadService RunReadService,
	subscriber pubsub.EventSubscriber,
	sseGate SSEConnectionGate,
	options ...runEventStreamOptions,
) {
	router.Get("/v1/runs/{runID}/events/stream", streamRunEventsHandler(logger, authenticator, runReadService, subscriber, sseGate, options...))
}

func streamRunEventsHandler(
	logger *slog.Logger,
	authenticator Authenticator,
	runReadService RunReadService,
	subscriber pubsub.EventSubscriber,
	sseGate SSEConnectionGate,
	options ...runEventStreamOptions,
) http.HandlerFunc {
	var opts runEventStreamOptions
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.heartbeatInterval <= 0 {
		opts.heartbeatInterval = defaultSSEHeartbeatInterval
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		if opts.shutdownContext != nil {
			stop := context.AfterFunc(opts.shutdownContext, cancel)
			defer stop()
		}
		r = r.WithContext(ctx)
		streamService, ok := runReadService.(RunEventStreamService)
		if !ok {
			logger.Error("run read service does not implement run event streaming")
			writeError(w, http.StatusInternalServerError, "internal_error", "run event streaming is unavailable")
			return
		}

		if sseGate != nil {
			if !sseGate.TryAcquire(r.Context()) {
				writeError(w, http.StatusServiceUnavailable, "sse_capacity_exceeded", "too many active event streams")
				return
			}
			defer sseGate.Release(r.Context())
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
		if err := writeSSEBytes(w, nil); err != nil {
			return
		}

		lastEventID := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
		startIndex := replayStartIndex(snapshot.Events, lastEventID)
		if lastEventID != "" && startIndex == 0 {
			logger.Warn("last event id not found in persisted snapshot; replaying full run stream",
				"run_id", runID,
				"last_event_id", lastEventID,
			)
		}

		delivered := make(map[string]struct{}, len(snapshot.Events))
		// Polling reads the full snapshot again. Seed the skipped prefix as
		// delivered so a reconnect cursor remains effective on later polls.
		for _, event := range snapshot.Events[:startIndex] {
			delivered[persistedStreamEventID(event.RunAgentID, event.SequenceNumber)] = struct{}{}
		}
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
		if _, noop := subscriber.(pubsub.NoopSubscriber); subscriber != nil && !noop {
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
		heartbeat := time.NewTicker(opts.heartbeatInterval)
		defer heartbeat.Stop()

		// 7. Stream live events and periodically catch up from persisted storage.
		for {
			select {
			case <-heartbeat.C:
				if err := writeSSEBytes(w, []byte(": keepalive\n\n")); err != nil {
					return
				}
			case data, ok := <-eventCh:
				if !ok {
					eventCh = nil
					continue
				}
				var payloadResolver *runevents.Resolver
				if provider, ok := runReadService.(RunEventPayloadResolverProvider); ok {
					payloadResolver = provider.RunEventPayloadResolver()
				}
				liveCtx, liveCancel := context.WithTimeout(r.Context(), min(5*time.Second, opts.heartbeatInterval/2))
				err := emitLiveRunEvent(liveCtx, w, flusher, data, delivered, payloadResolver)
				liveCancel()
				if err != nil {
					return
				}
			case <-ticker.C:
				refreshCtx, refreshCancel := context.WithTimeout(r.Context(), min(5*time.Second, opts.heartbeatInterval/2))
				snapshot, err := streamService.ListRunEventStream(refreshCtx, caller, runID)
				refreshCancel()
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

func exportRunEventsJSONLHandler(logger *slog.Logger, runReadService RunReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		streamService, ok := runReadService.(RunEventStreamService)
		if !ok {
			logger.Error("run read service does not implement run event export")
			writeError(w, http.StatusInternalServerError, "internal_error", "run event export is unavailable")
			return
		}

		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		runID, err := runIDFromURLParam("runID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
			return
		}

		snapshot, err := streamService.ListRunEventStream(r.Context(), caller, runID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunNotFound):
				writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("failed to export run events",
					"run_id", runID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		var body bytes.Buffer
		if err := writeRunEventsJSONL(&body, snapshot.Events); err != nil {
			logger.Error("failed to write run events export",
				"run_id", runID,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="agentclash-run-%s-events.jsonl"`, runID.String()))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", body.Len()))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(body.Bytes()); err != nil {
			logger.Error("failed to send run events export",
				"run_id", runID,
				"error", err,
			)
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
		if err := writeSSEFrame(w, flusher, streamEventID, data); err != nil {
			return err
		}
		delivered[streamEventID] = struct{}{}
	}
	return nil
}

func writeRunEventsJSONL(w io.Writer, events []repository.RunEvent) error {
	for _, event := range events {
		data, _, err := marshalPersistedRunEvent(event)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}

func emitLiveRunEvent(
	ctx context.Context,
	w http.ResponseWriter,
	flusher http.Flusher,
	data []byte,
	delivered map[string]struct{},
	payloadResolver *runevents.Resolver,
) error {
	hydrated, err := hydrateLiveRunEventData(ctx, payloadResolver, data)
	if err != nil {
		return err
	}
	streamEventID := extractStreamEventID(hydrated)
	if _, seen := delivered[streamEventID]; seen {
		return nil
	}
	if err := writeSSEFrame(w, flusher, streamEventID, hydrated); err != nil {
		return err
	}
	delivered[streamEventID] = struct{}{}
	return nil
}

func hydrateLiveRunEventData(ctx context.Context, resolver *runevents.Resolver, data []byte) ([]byte, error) {
	if resolver == nil || len(data) == 0 {
		return data, nil
	}
	var envelope runevents.Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return data, nil
	}
	if _, ok := runevents.ParsePayloadRef(envelope.Payload); !ok {
		return data, nil
	}
	resolved, err := resolver.Resolve(ctx, envelope.Payload)
	if err != nil {
		return nil, fmt.Errorf("hydrate live run event payload: %w", err)
	}
	envelope.Payload = resolved
	out, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal hydrated live run event: %w", err)
	}
	return out, nil
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

func writeSSEFrame(w http.ResponseWriter, _ http.Flusher, id string, data []byte) error {
	return writeSSEBytes(w, []byte(fmt.Sprintf("id: %s\nevent: run_event\ndata: %s\n\n", id, data)))
}

func writeSSEBytes(w http.ResponseWriter, data []byte) error {
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Now().Add(runEventStreamWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	defer rc.SetWriteDeadline(time.Time{})
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return rc.Flush()
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
