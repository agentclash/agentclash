package pubsub

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisConfig holds connection parameters for a Redis instance.
type RedisConfig struct {
	URL          string
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
	MinIdleConns int
}

// LoadRedisConfigFromEnv returns a RedisConfig if REDIS_URL is set.
// The second return value is false when REDIS_URL is empty or unset,
// signalling that Redis should not be used.
func LoadRedisConfigFromEnv() (RedisConfig, bool) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		return RedisConfig{}, false
	}
	return RedisConfig{
		URL:          url,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
	}, true
}

// NewRedisClient creates a Redis client from the given config and verifies
// connectivity with a PING.
func NewRedisClient(cfg RedisConfig) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	opts.MaxRetries = cfg.MaxRetries
	opts.DialTimeout = cfg.DialTimeout
	opts.ReadTimeout = cfg.ReadTimeout
	opts.WriteTimeout = cfg.WriteTimeout
	opts.PoolSize = cfg.PoolSize
	opts.MinIdleConns = cfg.MinIdleConns

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return client, nil
}

// ChannelName returns the Redis pub/sub channel for a given run.
// One channel per run; the Envelope payload carries RunAgentID so
// subscribers can filter client-side.
func ChannelName(runID uuid.UUID) string {
	return "run:" + runID.String() + ":events"
}
