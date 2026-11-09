package vibe

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/redis/go-redis/v9"
)

func TestVibeRedisReadinessRequiresBudgetOperations(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	ctx := context.Background()
	mr.Server().SetPreHook(func(p *server.Peer, cmd string, args ...string) bool {
		if strings.EqualFold(cmd, "EVAL") {
			p.WriteError("NOPERM budget script denied")
			return true
		}
		return false
	})
	if err := rc.Ping(ctx).Err(); err != nil {
		t.Fatal("fixture must remain reachable", err)
	}
	if err := (Gate{Redis: rc}).Healthy(ctx); err == nil {
		t.Error("Redis PING accepted despite unusable budget script")
	}
	mr.Server().SetPreHook(nil)
	if err := (Gate{Redis: rc}).Healthy(ctx); err != nil {
		t.Fatal(err)
	}
	if keys := mr.Keys(); len(keys) != 0 {
		t.Fatalf("readiness probe left persistent keys: %v", keys)
	}
}
