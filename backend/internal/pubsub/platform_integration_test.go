//go:build awsplatform

package pubsub

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider/throttle"
)

func TestAWSValkeyTransport(t *testing.T) {
	if os.Getenv("PLATFORM_REDIS_URL") == "" {
		t.Skip("isolated harness required")
	}
	cfg := RedisConfig{URL: os.Getenv("PLATFORM_REDIS_URL"), TLSCAFile: os.Getenv("PLATFORM_REDIS_CA"), DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, PoolSize: 2}
	c, err := NewRedisClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	subscription := c.Subscribe(ctx, "run:fixture:events")
	defer subscription.Close()
	if _, err := subscription.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Publish(ctx, "run:fixture:events", "fixture").Err(); err != nil {
		t.Fatal(err)
	}
	if msg, err := subscription.ReceiveMessage(ctx); err != nil || msg.Payload != "fixture" {
		t.Fatal("pub/sub failed", err)
	}
	pipe := c.TxPipeline()
	pipe.Set(ctx, "agentclash:test:counter", 1, time.Minute)
	pipe.Incr(ctx, "agentclash:test:counter")
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if result, err := c.Eval(ctx, "return redis.call('GET', KEYS[1])", []string{"agentclash:test:counter"}).Text(); err != nil || result != "2" {
		t.Fatal("Lua/transaction failed", err)
	}
	if err := c.FlushAll(ctx).Err(); err == nil {
		t.Fatal("application may flush durable state")
	}
	limiter := throttle.NewRedisLimiter(c, throttle.Config{LimitsByProvider: map[string]throttle.Limits{"fixture": {RPM: 10, TPM: 1000, MaxConcurrent: 1}}, AcquireTimeout: time.Second})
	lease, err := limiter.Acquire(ctx, throttle.Key{Provider: "fixture", Credential: "probe"}, 10)
	if err != nil {
		t.Fatal("provider throttle ACL failed", err)
	}
	lease.Reconcile(5)
	lease.Release()
	cfg.TLSCAFile = os.Getenv("PLATFORM_UNTRUSTED_CA")
	if wrong, err := NewRedisClient(cfg); err == nil {
		wrong.Close()
		t.Fatal("untrusted cache CA accepted")
	}
}
