package vibe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func deepseekConfig() Config {
	c := testConfig()
	c.DefaultModel = "deepseek/deepseek-v4-flash-0731"
	c.Profiles[c.DefaultModel] = ModelProfile{ID: c.DefaultModel, Name: "DeepSeek V4 Flash 0731", Route: "open-inference/fp8", StructuredOutputs: true, DisableReasoning: true, InputNanoPerToken: 100, OutputNanoPerToken: 300, Context: 1048576, FramingAllowance: 2048, Conformed: true, ExpiresAt: timestamp().Add(time.Hour)}
	c.LocalTesting, c.LocalBudget = true, 10*NanoUSD
	return c
}

func TestVibeDeepSeekProfileAndContext(t *testing.T) {
	c := deepseekConfig()
	p, err := c.Profile(c.DefaultModel)
	if err != nil {
		t.Fatal(err)
	}
	if cost, err := p.BoundCost(1000, 100); err != nil || cost != 130000 {
		t.Fatalf("cost=%d err=%v", cost, err)
	}
	if p.inputLimit(c.Limits(true)) != p.Context-c.Limits(true).OutputTokens {
		t.Fatal("reservation exceeded endpoint context")
	}
	req := provider.Request{MaxOutputTokens: 2048, Messages: []provider.Message{{Role: "user", Content: strings.Repeat("x", 18698)}}}
	count, err := CountContext(req, p, c.Limits(true))
	if err != nil || count.Estimate != 0 || count.Method != "utf8_bytes_plus_verified_framing" {
		t.Fatalf("invalid DeepSeek token count: %+v %v", count, err)
	}
	for _, mutate := range []func(*ModelProfile){
		func(p *ModelProfile) { p.Route = "openai" },
		func(p *ModelProfile) { p.StructuredOutputs = false },
		func(p *ModelProfile) { p.Free = true },
		func(p *ModelProfile) { p.Conformed = false },
		func(p *ModelProfile) { p.InputNanoPerToken = 0 },
		func(p *ModelProfile) { p.ExpiresAt = timestamp().Add(-time.Second) },
	} {
		bad := p
		mutate(&bad)
		c.Profiles[p.ID] = bad
		if _, err := c.Profile(p.ID); err == nil {
			t.Fatal("invalid DeepSeek profile accepted")
		}
	}
	c.Profiles[p.ID] = p
	c.FreeOnly = true
	if _, err := c.Profile(p.ID); err == nil {
		t.Fatal("free-only mode accepted paid DeepSeek")
	}
}

func TestVibeDeepSeekVerifiedReplacementRoute(t *testing.T) {
	c := deepseekConfig()
	p := c.Profiles[c.DefaultModel]
	p.Route = "deepinfra/fp8"
	c.Profiles[p.ID] = p
	if _, err := c.Profile(p.ID); err != nil {
		t.Fatal(err)
	}
	p.Conformed = false
	c.Profiles[p.ID] = p
	if _, err := c.Profile(p.ID); err == nil {
		t.Fatal("replacement route bypassed conformance")
	}
}

func TestIntegrationLocalPaidBudgetAndRepeatedRuns(t *testing.T) {
	base := integrationStore(t)
	cfg := deepseekConfig()
	s := NewStore(base.DB, cfg)
	v := anonSession(t, s)
	ctx := context.Background()
	g := Gateway{Store: s, Config: cfg, Gate: testGate(t), Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		var policy struct {
			Only     []string           `json:"only"`
			Fallback bool               `json:"allow_fallbacks"`
			Required bool               `json:"require_parameters"`
			Price    map[string]float64 `json:"max_price"`
		}
		if err := json.Unmarshal(req.OpenRouterPolicy, &policy); err != nil {
			t.Fatal(err)
		}
		if req.Model != cfg.DefaultModel || len(policy.Only) != 1 || policy.Only[0] != "open-inference/fp8" || policy.Fallback || !policy.Required || policy.Price["prompt"] != .1 || policy.Price["completion"] != .3 || len(policy.Price) != 3 || policy.Price["request"] != 0 {
			t.Fatalf("wrong paid model/price policy: %+v", policy)
		}
		cost := json.Number("0.000001")
		return provider.Response{OutputText: "ok", Usage: provider.Usage{InputTokens: 10, OutputTokens: 1, CostUSD: &cost}}, nil
	})}
	for _, kind := range []string{"message", "check", "check", "retest", "retest"} {
		v, _ = s.GetSession(ctx, v.Actor, v.ID)
		sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Models: cfg.DefaultModels(), Kind: kind}
		plan := Plan{LocalTesting: true, Anonymous: true, Submission: sub, Calls: 1, MaxCost: NanoUSD / 100}
		o, err := s.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
		if err != nil {
			t.Fatal(kind, err)
		}
		if _, _, err = s.Start(ctx, o.ID); err != nil {
			t.Fatal(err)
		}
		if _, err = g.Call(ctx, o, "paid-local", Target, []provider.Message{{Role: "user", Content: strings.Repeat("x", 18698)}}, nil); err != nil {
			t.Fatal(err)
		}
		if err = s.Finish(ctx, o.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	account := "local:" + cfg.Campaign
	balance, held, err := s.Balance(ctx, account)
	if err != nil || balance != cfg.LocalBudget-5000 || held != 0 {
		t.Fatalf("budget balance=%d held=%d err=%v", balance, held, err)
	}
	// A new session/store shares the spent budget; it cannot refill it.
	s = NewStore(base.DB, cfg)
	v = anonSession(t, s)
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Models: cfg.DefaultModels(), Kind: "message"}
	plan := Plan{LocalTesting: true, Anonymous: true, Submission: sub, Calls: 1, MaxCost: cfg.LocalBudget}
	if _, err = s.Submit(ctx, v.Actor, v.ID, sub, plan, cfg); err == nil {
		t.Fatal("new session reset the paid budget")
	}
	// A plan alone cannot grant local privileges to the normal store.
	if _, err = base.Submit(ctx, v.Actor, v.ID, sub, plan, cfg); err == nil {
		t.Fatal("non-local store accepted local privileges")
	}
	var count int
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_grants WHERE account_id=$1", account).Scan(&count); err != nil || count != 1 {
		t.Fatalf("local budget granted %d times: %v", count, err)
	}
}
