package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestVibeLocalTestingRequiresDevelopmentAndExplicitFunding(t *testing.T) {
	t.Setenv("VIBE_ENABLED", "false")
	t.Setenv("VIBE_MODELS_JSON", "")
	t.Setenv("VIBE_LOCAL_TESTING", "true")
	t.Setenv("VIBE_LOCAL_BUDGET_USD", "")
	t.Setenv("VIBE_CAMPAIGN", "local-test")
	for _, tc := range []struct {
		environment, free string
		valid             bool
	}{
		{"production", "true", false}, {"", "true", false},
		{"development", "false", false}, {"development", "true", true},
	} {
		t.Setenv("APP_ENV", tc.environment)
		t.Setenv("VIBE_FREE_ONLY", tc.free)
		cfg, err := LoadConfig()
		if (err == nil) != tc.valid || tc.valid && !cfg.TestingLocally() {
			t.Fatalf("env=%q free=%s valid=%t: %v", tc.environment, tc.free, tc.valid, err)
		}
	}
	t.Setenv("VIBE_FREE_ONLY", "false")
	t.Setenv("VIBE_LOCAL_BUDGET_USD", "10")
	if cfg, err := LoadConfig(); err != nil || !cfg.TestingLocally() || cfg.LocalBudget != 10*NanoUSD {
		t.Fatalf("explicit paid local budget was rejected: local=%t budget=%d err=%v", cfg.TestingLocally(), cfg.LocalBudget, err)
	}
	t.Setenv("APP_ENV", "production")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("paid local mode enabled in production")
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("VIBE_CAMPAIGN", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("paid local mode admitted without a stable budget identifier")
	}
}

func TestVibeLocalTestingUsesModelContextAndKeepsProviderBounds(t *testing.T) {
	cfg := freeConfig()
	profile := cfg.Profiles[cfg.DefaultModel]
	req := provider.Request{MaxOutputTokens: 2048, Messages: []provider.Message{{Role: "user", Content: strings.Repeat("a", 18698)}}}
	if _, err := CountContext(req, profile, cfg.Limits(true)); err == nil {
		t.Fatal("default trial cap disappeared")
	}
	cfg.LocalTesting = true
	if _, err := CountContext(req, profile, cfg.Limits(true)); err != nil {
		t.Fatal("local request still blocked by preview cap:", err)
	}
	req.Messages[0].Content = strings.Repeat("a", profile.Context)
	if _, err := CountContext(req, profile, cfg.Limits(true)); err == nil {
		t.Fatal("local request exceeded actual model context")
	}
	if _, err := cfg.Profile("openai/gpt-4.1-mini"); err == nil {
		t.Fatal("local testing admitted a paid route")
	}
	gate := testGate(t)
	for i := 0; i <= LimitsFor(true).Rate; i++ {
		if err := gate.Check(context.Background(), "local-tester", cfg.Limits(true)); err != nil {
			t.Fatal("local testing retained app rate limit:", err)
		}
	}
}

func TestIntegrationLocalTestingContinuesPastTrialQuotas(t *testing.T) {
	base := integrationStore(t)
	cfg := freeConfig()
	cfg.LocalTesting = true
	s := NewStore(base.DB, cfg)
	v := anonSession(t, s)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = s.DB.Exec(context.Background(), "DELETE FROM vibe_attempts WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)", v.ID)
	})
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Models: cfg.DefaultModels(), Kind: "playground"}
	plan := Plan{LocalTesting: true, Free: true, Anonymous: true, Submission: sub, Calls: 1}
	o, err := s.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Start(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	// Exceed all cumulative conversation/trial counters without real model I/O.
	_, err = s.DB.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline,model_calls)
	 SELECT gen_random_uuid(),session_id,actor,gen_random_uuid(),'local-fixture-'||i,
	 CASE WHEN i%3=0 THEN 'check' WHEN i%3=1 THEN 'retest' ELSE 'message' END,
	 'COMPLETED','RELEASED',models,input,0,deadline,$2 FROM vibe_operations CROSS JOIN generate_series(1,$3) AS i WHERE id=$1`, o.ID, TrialCalls, MaxConversationOperations)
	if err != nil {
		t.Fatal(err)
	}
	policy := raw(map[string]any{"profile": cfg.Profiles[cfg.DefaultModel]})
	for i := 0; i < MaxFreeDailyCalls; i++ {
		_, err = s.DB.Exec(ctx, `INSERT INTO vibe_attempts(id,operation_id,step_key,role,model,provider,policy,request_hash,input_bound,max_output,max_cost,actual_cost,state,completed_at) VALUES($1,$2,$3,'target',$4,'openrouter',$5,'fixture',1,1,0,0,'SUCCEEDED',now())`, uuid.New(), o.ID, fmt.Sprintf("fixture:%d", i), cfg.DefaultModel, policy)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Edit(ctx, v.Actor, v.ID, v.Revision+1, func(v *Session) error {
		for i := 0; i <= MaxConversationMessages; i++ {
			v.Document.Messages = append(v.Document.Messages, Message{ID: uuid.New(), Role: "user", Content: "Earlier test", CreatedAt: timestamp()})
		}
		return nil
	}); err != nil {
		t.Fatal("local history quota:", err)
	}
	calls := 0
	g := Gateway{Store: s, Config: cfg, Gate: testGate(t), Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		calls++
		var routing struct {
			Fallbacks bool           `json:"allow_fallbacks"`
			MaxPrice  map[string]int `json:"max_price"`
		}
		if err := json.Unmarshal(req.OpenRouterPolicy, &routing); err != nil {
			t.Fatal(err)
		}
		if routing.Fallbacks || len(routing.MaxPrice) != 3 {
			t.Fatal("free-only routing changed")
		}
		for _, price := range routing.MaxPrice {
			if price != 0 {
				t.Fatal("paid routing allowed")
			}
		}
		zero := json.Number("0")
		return provider.Response{OutputText: "ok", Usage: provider.Usage{InputTokens: 10, OutputTokens: 1, CostUSD: &zero}}, nil
	})}
	for _, kind := range []string{"message", "check", "retest", "check", "retest"} {
		v, err = s.GetSession(ctx, v.Actor, v.ID)
		if err != nil {
			t.Fatal(err)
		}
		sub.ClientID, sub.Revision, sub.Kind = uuid.New(), v.Revision, kind
		plan.Submission = sub
		next, err := s.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
		if err != nil {
			t.Fatal(kind, "admission:", err)
		}
		if _, _, err = s.Start(ctx, next.ID); err != nil {
			t.Fatal(err)
		}
		if _, err = g.Call(ctx, next, "local:continue", Target, []provider.Message{{Role: "user", Content: strings.Repeat("a", 18698)}}, nil); err != nil {
			t.Fatal(kind, "dispatch:", err)
		}
		if err = s.Finish(ctx, next.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 5 {
		t.Fatalf("calls=%d", calls)
	}
	// A client cannot grant itself the exception or carry it into normal mode.
	v, err = s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	sub.ClientID, sub.Revision = uuid.New(), v.Revision
	if _, err = base.Submit(ctx, v.Actor, v.ID, sub, plan, freeConfig()); err == nil {
		t.Fatal("normal server accepted local testing privileges")
	}
}
