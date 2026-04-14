package pubsub

import (
	"context"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

// EventPublisher publishes a persisted run event to a pub/sub channel.
// Implementations must be safe for concurrent use.
type EventPublisher interface {
	PublishRunEvent(ctx context.Context, runID uuid.UUID, event runevents.Envelope) error
	Close() error
}

// NoopPublisher silently discards all publish calls.
// Used when Redis is not configured.
type NoopPublisher struct{}

func (NoopPublisher) PublishRunEvent(context.Context, uuid.UUID, runevents.Envelope) error {
	return nil
}

func (NoopPublisher) Close() error { return nil }
