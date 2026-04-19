package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/pubsub"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

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

		// 3. Authorize: verify caller can read this run.
		if _, err := runReadService.GetRun(r.Context(), caller, runID); err != nil {
			writeError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}

		// 4. Check if streaming is available (Redis configured).
		if _, ok := subscriber.(pubsub.NoopSubscriber); ok {
			writeError(w, http.StatusServiceUnavailable, "streaming_unavailable", "real-time streaming is not configured")
			return
		}

		// 5. Verify the response writer supports flushing.
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "internal_error", "streaming not supported")
			return
		}

		// 6. Set SSE headers.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // nginx compatibility
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		// 7. Subscribe to live events.
		eventCh, err := subscriber.Subscribe(r.Context(), runID)
		if err != nil {
			logger.Error("failed to subscribe to run events",
				"run_id", runID,
				"error", err,
			)
			// Headers already sent, just close.
			return
		}

		// 8. Stream events as SSE frames.
		for {
			select {
			case data, ok := <-eventCh:
				if !ok {
					return // channel closed
				}
				seqID := extractSequenceNumber(data)
				fmt.Fprintf(w, "id: %s\n", seqID)
				fmt.Fprintf(w, "event: run_event\n")
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return // client disconnected
			}
		}
	}
}

// extractSequenceNumber pulls the sequence_number from a JSON-serialized
// Envelope for use as the SSE event ID. Falls back to "0" on parse failure.
func extractSequenceNumber(data []byte) string {
	var envelope struct {
		SequenceNumber int64 `json:"SequenceNumber"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "0"
	}
	return fmt.Sprintf("%d", envelope.SequenceNumber)
}
