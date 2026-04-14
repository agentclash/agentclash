package pubsub

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisSubscriber subscribes to Redis pub/sub channels for run events.
type RedisSubscriber struct {
	client *redis.Client
	logger *slog.Logger
}

// NewRedisSubscriber creates a subscriber backed by the given Redis client.
func NewRedisSubscriber(client *redis.Client, logger *slog.Logger) *RedisSubscriber {
	return &RedisSubscriber{client: client, logger: logger}
}

func (s *RedisSubscriber) Subscribe(ctx context.Context, runID uuid.UUID) (<-chan []byte, error) {
	sub := s.client.Subscribe(ctx, ChannelName(runID))

	// Wait for subscription confirmation.
	if _, err := sub.Receive(ctx); err != nil {
		sub.Close()
		return nil, err
	}

	ch := make(chan []byte, 64) // buffered to handle bursts
	go func() {
		defer close(ch)
		defer sub.Close()
		msgCh := sub.Channel()
		for {
			select {
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				select {
				case ch <- []byte(msg.Payload):
				default:
					// Backpressure: drop event for slow consumer.
					// Client recovers via Last-Event-ID replay from Postgres.
					s.logger.Warn("dropping event for slow subscriber",
						"run_id", runID,
						"channel", ChannelName(runID),
					)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (s *RedisSubscriber) Close() error {
	return s.client.Close()
}
