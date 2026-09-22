package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
)

func TestRedisPingHonorsReadinessDeadline(t *testing.T) {
	r := miniredis.RunT(t)
	c, err := NewRedisClient(RedisConfig{URL: "redis://" + r.Addr(), DialTimeout: time.Second, ReadTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	release := make(chan struct{})
	defer close(release)
	r.Server().SetPreHook(func(_ *server.Peer, command string, _ ...string) bool {
		if command == "PING" {
			<-release
		}
		return false
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := c.Ping(ctx).Err(); err == nil {
		t.Fatal("stalled Redis unexpectedly answered PING")
	}
	if time.Since(start) > time.Second {
		t.Fatal("Redis ignored request deadline and used its longer socket timeout")
	}
}
