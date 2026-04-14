package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisPublisher publishes run events to Redis pub/sub channels.
type RedisPublisher struct {
	client *redis.Client
}

// NewRedisPublisher creates a publisher backed by the given Redis client.
func NewRedisPublisher(client *redis.Client) *RedisPublisher {
	return &RedisPublisher{client: client}
}

func (p *RedisPublisher) PublishRunEvent(ctx context.Context, runID uuid.UUID, event runevents.Envelope) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event for publish: %w", err)
	}
	return p.client.Publish(ctx, ChannelName(runID), data).Err()
}

func (p *RedisPublisher) Close() error {
	return p.client.Close()
}
