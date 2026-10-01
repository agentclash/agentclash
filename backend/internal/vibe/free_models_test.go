package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func freeConfig() Config {
	c := testConfig()
	c.FreeOnly = true
	c.DefaultModel = "liquid/lfm-2.5-2.6b:free"
	c.Profiles[c.DefaultModel] = ModelProfile{ID: c.DefaultModel, Route: "liquid/fp8", Free: true, Conformed: true, Context: 65536, FramingAllowance: 4096, ExpiresAt: timestamp().Add(time.Hour)}
	return c
}

func TestVibeFreeProfilesFailClosed(t *testing.T) {
	c := freeConfig()
	p, err := c.Profile(c.DefaultModel)
	if err != nil {
		t.Fatal(err)
	}
	if cost, err := p.BoundCost(16384, 2048); err != nil || cost != 0 {
		t.Fatalf("free price: %d %v", cost, err)
	}
	if _, err = c.Profile("openai/gpt-4.1-mini"); err == nil {
		t.Fatal("free-only server admitted paid model")
	}
	for _, mutate := range []func(*ModelProfile){
		func(p *ModelProfile) { p.Free = false },
		func(p *ModelProfile) { p.InputNanoPerToken = 1 },
		func(p *ModelProfile) { p.Route = "openai" },
		func(p *ModelProfile) { p.Conformed = false },
		func(p *ModelProfile) { p.ExpiresAt = timestamp().Add(-time.Second) },
	} {
		bad := p
		mutate(&bad)
		c.Profiles[c.DefaultModel] = bad
		if _, err = c.Profile(c.DefaultModel); err == nil {
			t.Fatal("invalid free price/profile accepted")
		}
	}
	c = freeConfig()
	if err = c.ValidateModels(c.DefaultModels(), true); err != nil {
		t.Fatal(err)
	}
	count, err := CountContext(provider.Request{MaxOutputTokens: 20, Messages: []provider.Message{{Role: "user", Content: "hello"}}}, p, LimitsFor(true))
	if err != nil || count.Estimate != 0 || count.UpperBound < 4096 || count.Method == "" {
		t.Fatalf("claimed an unsupported tokenizer: %+v %v", count, err)
	}
}

func TestIntegrationFreeJournalAndDailyLimit(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	cfg := freeConfig()
	ctx := context.Background()
	sub := Submission{ClientID: uuid.New(), Models: cfg.DefaultModels(), Kind: "playground"}
	plan := Plan{Free: true, Anonymous: true, Submission: sub, Calls: 2}
	o, err := s.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Start(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	g := Gateway{Store: s, Config: cfg, Gate: testGate(t), Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		calls++
		var policy struct {
			Only     []string       `json:"only"`
			Fallback bool           `json:"allow_fallbacks"`
			Required bool           `json:"require_parameters"`
			Price    map[string]int `json:"max_price"`
		}
		if err := json.Unmarshal(req.OpenRouterPolicy, &policy); err != nil {
			t.Fatal(err)
		}
		if req.Model != cfg.DefaultModel || len(policy.Only) != 1 || policy.Only[0] != "liquid/fp8" || policy.Fallback || !policy.Required || len(policy.Price) != 3 {
			t.Fatalf("unsafe free routing: %+v", policy)
		}
		for _, price := range policy.Price {
			if price != 0 {
				t.Fatal("nonzero price ceiling")
			}
		}
		zero := json.Number("0")
		return provider.Response{OutputText: "hello", Usage: provider.Usage{InputTokens: 10, OutputTokens: 1, CostUSD: &zero}}, nil
	})}
	if _, err = g.Call(ctx, o, "free:first", Target, []provider.Message{{Role: "user", Content: "hello"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	current, err := s.Operation(ctx, o.ID)
	if err != nil || current.ActualCost == nil || *current.ActualCost != 0 || current.MaxCost != 0 || current.Billing != Released {
		t.Fatalf("zero cost was lost: %+v %v", current, err)
	}
	// Seed completed, zero-price history to exercise the installation-wide cap.
	policy := raw(map[string]any{"profile": cfg.Profiles[cfg.DefaultModel]})
	for i := 0; i < MaxFreeDailyCalls; i++ {
		_, err = s.DB.Exec(ctx, `INSERT INTO vibe_attempts(id,operation_id,step_key,role,model,provider,policy,request_hash,input_bound,max_output,max_cost,actual_cost,state,completed_at) VALUES($1,$2,$3,'target',$4,'openrouter',$5,'fixture',1,1,0,0,'SUCCEEDED',now())`, uuid.New(), o.ID, fmt.Sprintf("fixture:%d", i), cfg.DefaultModel, policy)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = s.DB.Exec(context.Background(), "DELETE FROM vibe_attempts WHERE operation_id=$1", o.ID)
	})
	v, err = s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	sub.ClientID, sub.Revision = uuid.New(), v.Revision
	plan.Submission = sub
	next, err := s.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Start(ctx, next.ID); err != nil {
		t.Fatal(err)
	}
	_, err = g.Call(ctx, next, "free:blocked", Target, []provider.Message{{Role: "user", Content: "again"}}, nil)
	var f *Fault
	if !errors.As(err, &f) || f.Code != "free_capacity_reached" || calls != 1 {
		t.Fatalf("free daily cap did not stop I/O: %v calls=%d", err, calls)
	}
}
