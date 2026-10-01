package api

import (
	"context"
	"errors"
	"fmt"

	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type vibeAPIHarness struct {
	t       *testing.T
	ctx     context.Context
	db      *pgxpool.Pool
	svc     *vibe.Service
	runner  *vibe.Runner
	session vibe.Session
	actor   string
	user    uuid.UUID
}

func newVibeAPIHarness(t *testing.T) *vibeAPIHarness {
	t.Helper()
	dsn := os.Getenv("VIBE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated migrated VIBE_TEST_DATABASE_URL")
	}
	if !strings.Contains(dsn, "vibe_test") {
		t.Fatal("refusing a non-test database")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	model := "liquid/lfm-2.5-2.6b:free"
	cfg := vibe.Config{Enabled: true, FreeOnly: true, LocalTesting: true, Credential: "fake-no-network", DefaultModel: model, Campaign: uuid.NewString(), AnonymousDaily: vibe.NanoUSD, AnonymousCampaign: 5 * vibe.NanoUSD, Profiles: map[string]vibe.ModelProfile{model: {ID: model, Route: "liquid/fp8", Free: true, Conformed: true, StructuredOutputs: true, Context: 65536, FramingAllowance: 4096, ExpiresAt: time.Now().Add(time.Hour)}}}
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	store := vibe.NewStore(db, cfg)
	svc := &vibe.Service{Store: store, Config: cfg, Gate: vibe.Gate{Redis: rc}, Compiler: VibePackCompiler{}}
	user := uuid.New()
	if _, err = db.Exec(ctx, "INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", user, user.String(), user.String()+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	actor := "user:" + user.String()
	session, err := store.CreateSession(ctx, actor, nil, uuid.New(), cfg.DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	h := &vibeAPIHarness{t: t, ctx: ctx, db: db, svc: svc, session: session, actor: actor, user: user}
	h.runner = &vibe.Runner{Service: svc, Gateway: &vibe.Gateway{Store: store, Config: cfg, Gate: svc.Gate, Client: groundingClient(func(context.Context, provider.Request) (provider.Response, error) {
		return provider.Response{}, fmt.Errorf("this API fixture has no inference configured")
	})}}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM vibe_attempts WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)", session.ID)
	})
	return h
}

func (h *vibeAPIHarness) reload() {
	h.t.Helper()
	var err error
	h.session, err = h.svc.Store.GetSession(h.ctx, h.actor, h.session.ID)
	if err != nil {
		h.t.Fatal(err)
	}
}

func (h *vibeAPIHarness) executeOperation(id uuid.UUID) (vibe.Operation, error) {
	h.t.Helper()
	runErr := h.runner.Execute(h.ctx, id)
	var issue *vibe.Fault
	if runErr != nil && !errors.As(runErr, &issue) {
		issue = &vibe.Fault{Code: "fixture_failure", Message: runErr.Error()}
	}
	if err := h.svc.Store.Finish(h.ctx, id, issue); err != nil {
		h.t.Fatal(err)
	}
	h.reload()
	for _, op := range h.session.Operations {
		if op.ID == id {
			return op, runErr
		}
	}
	h.t.Fatal("operation disappeared")
	return vibe.Operation{}, runErr
}
