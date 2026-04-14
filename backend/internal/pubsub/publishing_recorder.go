package pubsub

import (
	"context"
	"log/slog"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/worker"
)

// PublishingRecorder wraps a RunEventRecorder and publishes each
// successfully persisted event to the EventPublisher. If publishing
// fails, the error is logged but not propagated -- the database
// remains the source of truth and publish failures must not break
// event recording.
type PublishingRecorder struct {
	inner     worker.RunEventRecorder
	publisher EventPublisher
	logger    *slog.Logger
}

var _ worker.RunEventRecorder = (*PublishingRecorder)(nil)

func NewPublishingRecorder(inner worker.RunEventRecorder, publisher EventPublisher, logger *slog.Logger) *PublishingRecorder {
	return &PublishingRecorder{inner: inner, publisher: publisher, logger: logger}
}

func (r *PublishingRecorder) RecordRunEvent(ctx context.Context, params repository.RecordRunEventParams) (repository.RunEvent, error) {
	event, err := r.inner.RecordRunEvent(ctx, params)
	if err != nil {
		return event, err
	}

	// Publish the persisted event with the DB-assigned sequence number.
	publishEnvelope := params.Event.WithSequenceNumber(event.SequenceNumber)
	if pubErr := r.publisher.PublishRunEvent(ctx, params.Event.RunID, publishEnvelope); pubErr != nil {
		r.logger.Warn("failed to publish run event to redis",
			"run_id", params.Event.RunID,
			"event_type", string(params.Event.EventType),
			"sequence_number", event.SequenceNumber,
			"error", pubErr,
		)
	}

	return event, nil
}
