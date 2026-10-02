//go:build awsplatform

package worker_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/pubsub"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
)

func TestAWSVibeAdmissionACL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, role := range []string{"api", "worker", "terminal"} {
		t.Run(role, func(t *testing.T) {
			url := os.Getenv("PLATFORM_VIBE_" + strings.ToUpper(role) + "_REDIS_URL")
			if url == "" {
				t.Skip("isolated generated TLS ACL harness required")
			}
			client, err := pubsub.NewRedisClient(pubsub.RedisConfig{URL: url, TLSCAFile: os.Getenv("PLATFORM_REDIS_CA"), DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second, PoolSize: 1})
			if err != nil {
				t.Fatal("TLS cache connection failed")
			}
			defer client.Close()
			actor := "acl-rehearsal:" + uuid.NewString()
			minute := time.Now().UTC().Format("200601021504")
			gate := vibe.Gate{Redis: client}
			if role == "terminal" {
				if gate.Check(ctx, actor, vibe.Limits{Rate: 2}) == nil {
					t.Fatal("terminal admitted Vibe work")
				}
				return
			}
			for range 2 {
				if err = gate.Check(ctx, actor, vibe.Limits{Rate: 2}); err != nil {
					t.Fatal("generated application ACL rejected Vibe admission")
				}
			}
			var fault *vibe.Fault
			if err = gate.Check(ctx, actor, vibe.Limits{Rate: 2}); !errors.As(err, &fault) || fault.Code != "rate_limit" {
				t.Fatal("Vibe limiter did not reject the third request")
			}
			key := "vibe:rate:" + vibe.Hash([]byte(actor)) + ":" + minute
			ttl, err := client.TTL(ctx, key).Result()
			if err != nil || ttl <= 0 || ttl > 120*time.Second {
				t.Fatal("Vibe rate key lacks its bounded expiry")
			}
			if client.Set(ctx, "vibe:other:fixture", "denied", time.Minute).Err() == nil || client.Set(ctx, "trycli:fixture", "denied", time.Minute).Err() == nil {
				t.Fatal("application ACL escaped its key namespace")
			}
			if client.FlushDB(ctx).Err() == nil || client.FlushAll(ctx).Err() == nil {
				t.Fatal("application may flush persistent state")
			}
		})
	}
}
