package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/hostedruns"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/pubsub"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

type HostedRunIngestionService interface {
	IngestEvent(ctx context.Context, runID uuid.UUID, token string, event hostedruns.Event) error
}

type HostedRunWorkflowSignaler interface {
	SignalRunAgentWorkflow(ctx context.Context, runID uuid.UUID, runAgentID uuid.UUID, event hostedruns.Event) error
}

type HostedRunExecutionRepository interface {
	GetHostedRunExecutionByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (repository.HostedRunExecution, error)
	ApplyHostedRunEvent(ctx context.Context, params repository.ApplyHostedRunEventParams) (repository.HostedRunExecution, error)
	RecordHostedRunEvent(ctx context.Context, params repository.RecordHostedRunEventParams) (repository.RunAgentReplay, error)
}

type HostedRunIngestionManager struct {
	repo      HostedRunExecutionRepository
	signer    hostedruns.CallbackTokenSigner
	signaler  HostedRunWorkflowSignaler
	publisher pubsub.EventPublisher
	logger    *slog.Logger
}

type noopHostedRunIngestionService struct{}

func NewHostedRunIngestionManager(repo HostedRunExecutionRepository, secret string, signaler HostedRunWorkflowSignaler, publisher pubsub.EventPublisher, logger *slog.Logger) *HostedRunIngestionManager {
	if publisher == nil {
		publisher = pubsub.NoopPublisher{}
	}
	return &HostedRunIngestionManager{
		repo:      repo,
		signer:    hostedruns.NewCallbackTokenSigner(secret),
		signaler:  signaler,
		publisher: publisher,
		logger:    logger,
	}
}

func (noopHostedRunIngestionService) IngestEvent(context.Context, uuid.UUID, string, hostedruns.Event) error {
	return errors.New("hosted run ingestion is not configured")
}

func (m *HostedRunIngestionManager) IngestEvent(ctx context.Context, runID uuid.UUID, token string, event hostedruns.Event) error {
	claims, err := m.signer.Verify(token)
	if err != nil {
		return err
	}
	if claims.RunID != runID {
		return errors.New("callback run_id does not match token")
	}
	if claims.RunAgentID != event.RunAgentID {
		return errors.New("callback run_agent_id does not match token")
	}
	if err := event.Validate(); err != nil {
		return err
	}

	execution, err := m.repo.GetHostedRunExecutionByRunAgentID(ctx, event.RunAgentID)
	if err != nil {
		return err
	}
	if execution.RunID != runID {
		return errors.New("callback run_id does not match hosted execution")
	}
	if execution.ExternalRunID == nil || *execution.ExternalRunID != event.ExternalRunID {
		return errors.New("callback external_run_id does not match hosted execution")
	}

	status, resultPayload, errorMessage := hostedExecutionStateForEvent(event)
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	normalizedEvent, err := runevents.NormalizeHostedEvent(runID, event)
	if err != nil {
		return err
	}
	traceEvents, err := runevents.NormalizeHostedTraceEvents(runID, event)
	if err != nil {
		return err
	}
	replaySummary, err := hostedReplaySummary(normalizedEvent, event)
	if err != nil {
		return err
	}
	if _, err := m.repo.ApplyHostedRunEvent(ctx, repository.ApplyHostedRunEventParams{
		RunAgentID:       event.RunAgentID,
		Status:           status,
		ExternalRunID:    &event.ExternalRunID,
		LastEventType:    event.EventType,
		LastEventPayload: payload,
		ResultPayload:    resultPayload,
		ErrorMessage:     errorMessage,
		OccurredAt:       event.OccurredAt,
	}); err != nil {
		return err
	}
	replayRecord, err := m.repo.RecordHostedRunEvent(ctx, repository.RecordHostedRunEventParams{
		Event:            normalizedEvent,
		AdditionalEvents: traceEvents,
		Summary:          replaySummary,
	})
	if err != nil {
		return err
	}

	// Publish the persisted event to Redis for live streaming.
	var seqNum int64
	if replayRecord.LatestSequenceNumber != nil {
		seqNum = *replayRecord.LatestSequenceNumber
	}
	publishEnvelope := normalizedEvent.WithSequenceNumber(seqNum)
	if pubErr := m.publisher.PublishRunEvent(ctx, runID, publishEnvelope); pubErr != nil {
		m.logger.Warn("failed to publish hosted run event to redis",
			"run_id", runID,
			"run_agent_id", event.RunAgentID,
			"error", pubErr,
		)
	}

	if isHostedRunTerminalEvent(event) {
		return m.signaler.SignalRunAgentWorkflow(ctx, runID, event.RunAgentID, event)
	}
	return nil
}

func ingestHostedRunEventHandler(logger *slog.Logger, service HostedRunIngestionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		runID, err := runIDFromURLParam("runID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
			return
		}

		token, err := hostedruns.BearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_callback_token", "authorization bearer token is required")
			return
		}

		var event hostedruns.Event
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		if decoder.More() {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain exactly one JSON object")
			return
		}

		if err := service.IngestEvent(r.Context(), runID, token, event); err != nil {
			switch {
			case errors.Is(err, hostedruns.ErrInvalidCallbackToken):
				writeError(w, http.StatusUnauthorized, "invalid_callback_token", "callback token is invalid")
			case errors.Is(err, repository.ErrHostedRunExecutionNotFound):
				writeError(w, http.StatusNotFound, "hosted_run_not_found", "hosted run execution not found")
			default:
				if errors.Is(err, hostedruns.ErrInvalidEventType) || errors.Is(err, hostedruns.ErrInvalidFinalStatus) || errors.Is(err, hostedruns.ErrInvalidTraceLevel) {
					writeError(w, http.StatusBadRequest, "invalid_hosted_event", err.Error())
					return
				}
				if eventErr := event.Validate(); eventErr != nil {
					writeError(w, http.StatusBadRequest, "invalid_hosted_event", eventErr.Error())
					return
				}
				logger.Error("ingest hosted run event failed",
					"method", r.Method,
					"path", r.URL.Path,
					"run_id", runID,
					"run_agent_id", event.RunAgentID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
	}
}

func isHostedRunTerminalEvent(event hostedruns.Event) bool {
	return event.EventType == hostedruns.EventTypeError || event.EventType == hostedruns.EventTypeRunFinished
}

func hostedExecutionStateForEvent(event hostedruns.Event) (string, json.RawMessage, *string) {
	switch event.EventType {
	case hostedruns.EventTypeRunStarted:
		return "running", nil, nil
	case hostedruns.EventTypeFinalAnswer:
		return "running", cloneJSON(event.Output), nil
	case hostedruns.EventTypeError:
		return "failed", cloneJSON(event.Output), cloneStringPtr(event.ErrorMessage)
	case hostedruns.EventTypeRunFinished:
		if event.FinalStatus != nil && *event.FinalStatus == hostedruns.FinalStatusCompleted {
			return "completed", cloneJSON(event.Output), nil
		}
		return "failed", cloneJSON(event.Output), cloneStringPtr(event.ErrorMessage)
	default:
		return "failed", nil, nil
	}
}

func cloneJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func hostedReplaySummary(normalizedEvent runevents.Envelope, event hostedruns.Event) (json.RawMessage, error) {
	summary, err := json.Marshal(map[string]any{
		"mode":            normalizedEvent.Summary.EvidenceLevel,
		"source":          normalizedEvent.Source,
		"schema_version":  normalizedEvent.SchemaVersion,
		"last_event_type": normalizedEvent.EventType,
		"status":          normalizedEvent.Summary.Status,
		"external_run_id": normalizedEvent.Summary.ExternalRunID,
		"idempotency_key": normalizedEvent.Summary.IdempotencyKey,
		"raw_event_type":  event.EventType,
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}
