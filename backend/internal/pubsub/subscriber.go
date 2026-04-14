package pubsub

import (
	"context"

	"github.com/google/uuid"
)

// EventSubscriber subscribes to run event channels and delivers raw JSON
// messages. Each call to Subscribe returns a channel that receives serialized
// Envelope JSON bytes. The channel is closed when the context is cancelled.
type EventSubscriber interface {
	Subscribe(ctx context.Context, runID uuid.UUID) (<-chan []byte, error)
	Close() error
}

// NoopSubscriber returns a closed channel immediately.
// Used when Redis is not configured.
type NoopSubscriber struct{}

func (NoopSubscriber) Subscribe(context.Context, uuid.UUID) (<-chan []byte, error) {
	ch := make(chan []byte)
	close(ch)
	return ch, nil
}

func (NoopSubscriber) Close() error { return nil }
