package vibe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func alternativeProfiles() []ModelProfile {
	return []ModelProfile{
		{ID: "deepseek/deepseek-v4-pro", Route: "baidu/fp8", DisableReasoning: true, StructuredOutputs: true, InputNanoPerToken: 1700, OutputNanoPerToken: 3400, Context: 128000, FramingAllowance: 2048, Conformed: true, ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "openai/gpt-5.4-mini", Route: "openai", OmitTemperature: true, DisableReasoning: true, StructuredOutputs: true, InputNanoPerToken: 750, OutputNanoPerToken: 4500, Context: 128000, FramingAllowance: 2048, Conformed: true, ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "deepseek/deepseek-v4.1-flash", Route: "coreweave/fp8", DisableReasoning: true, StructuredOutputs: true, InputNanoPerToken: 200, OutputNanoPerToken: 650, Context: 1048576, FramingAllowance: 2048, Conformed: true, ExpiresAt: time.Now().Add(time.Hour)},
	}
}

func TestIntegrationVibeAlternativeGatewayUsesConformedParameters(t *testing.T) {
	for _, profile := range alternativeProfiles() {
		t.Run(profile.ID, func(t *testing.T) {
			base := integrationStore(t)
			cfg := deepseekConfig()
			cfg.DefaultModel = profile.ID
			cfg.Profiles[profile.ID] = profile
			store := NewStore(base.DB, cfg)
			v := anonSession(t, store)
			sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Models: cfg.DefaultModels()}
			op, err := store.Submit(context.Background(), v.Actor, v.ID, sub, Plan{LocalTesting: true, Anonymous: true, Submission: sub, Calls: 1, MaxCost: NanoUSD / 10}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = store.Start(context.Background(), op.ID); err != nil {
				t.Fatal(err)
			}
			gateway := Gateway{Store: store, Config: cfg, Gate: testGate(t), Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
				if (req.Temperature == nil) != profile.OmitTemperature || string(req.Reasoning) != `{"enabled":false}` || len(req.Tools) != 0 || req.Model != profile.ID {
					t.Fatalf("unconformed settings: %s", profile.ID)
				}
				var policy struct {
					Only     []string `json:"only"`
					Fallback bool     `json:"allow_fallbacks"`
				}
				if json.Unmarshal(req.OpenRouterPolicy, &policy) != nil || len(policy.Only) != 1 || policy.Only[0] != profile.Route || policy.Fallback {
					t.Fatal("route not pinned")
				}
				cost := json.Number("0.000001")
				return provider.Response{OutputText: "OK", Usage: provider.Usage{InputTokens: 10, OutputTokens: 1, CostUSD: &cost}}, nil
			})}
			if _, err = gateway.Call(context.Background(), op, "route", Assistant, []provider.Message{{Role: "user", Content: "hello"}}, nil); err != nil {
				t.Fatal(err)
			}
			if err = store.Finish(context.Background(), op.ID, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVibeAlternativeOpenAIMetadataWithoutTemperature(t *testing.T) {
	var profile ModelProfile
	for _, candidate := range alternativeProfiles() {
		if candidate.ID == "openai/gpt-5.4-mini" {
			profile = candidate
		}
	}
	for _, unknownCharge := range []bool{false, true} {
		catalog := localCatalog(profile)
		endpoint := catalog["data"].(map[string]any)["endpoints"].([]any)[0].(map[string]any)
		endpoint["supported_parameters"] = []string{"structured_outputs", "response_format", "max_tokens", "reasoning"}
		pricing := endpoint["pricing"].(map[string]any)
		pricing["web_search"] = "0.01"
		if unknownCharge {
			pricing["request"] = "0.01"
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(catalog) }))
		v := newLocalProfileVerifier()
		v.baseURL = server.URL + "/"
		err := v.fetch(profile)
		server.Close()
		if (err != nil) != unknownCharge {
			t.Fatalf("unknownCharge=%v err=%v", unknownCharge, err)
		}
	}
}

func TestVibeAlternativeModelProfiles(t *testing.T) {
	for _, p := range alternativeProfiles() {
		t.Run(p.ID, func(t *testing.T) {
			cfg := deepseekConfig()
			cfg.Profiles[p.ID] = p
			if _, err := cfg.Profile(p.ID); err != nil {
				t.Fatal(err)
			}
			req := provider.Request{MaxOutputTokens: 100, Messages: []provider.Message{{Role: "user", Content: "Prepare a test"}}}
			count, err := CountContext(req, p, cfg.Limits(true))
			if err != nil || count.Estimate != 0 || count.Method != "utf8_bytes_plus_verified_framing" {
				t.Fatalf("model received an unverified tokenizer estimate: %+v %v", count, err)
			}
			for name, change := range map[string]func(*ModelProfile){
				"unapproved route": func(p *ModelProfile) { p.Route = "other-provider" },
				"missing schema":   func(p *ModelProfile) { p.StructuredOutputs = false },
				"unverified":       func(p *ModelProfile) { p.Conformed = false },
				"expired":          func(p *ModelProfile) { p.ExpiresAt = time.Now().Add(-time.Hour) },
				"zero price":       func(p *ModelProfile) { p.InputNanoPerToken = 0 },
			} {
				t.Run(name, func(t *testing.T) {
					bad := p
					change(&bad)
					cfg.Profiles[p.ID] = bad
					if _, err := cfg.Profile(p.ID); err == nil {
						t.Fatal("invalid profile accepted")
					}
				})
			}
			cfg.Profiles[p.ID] = p
			cfg.FreeOnly = true
			if _, err := cfg.Profile(p.ID); err == nil {
				t.Fatal("paid model entered free-only mode")
			}
		})
	}
}

func TestVibeAlternativeProfileMetadataRenewal(t *testing.T) {
	for _, p := range alternativeProfiles() {
		t.Run(p.ID, func(t *testing.T) {
			p.ExpiresAt = time.Now().Add(-24 * time.Hour)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/"+p.ID+"/endpoints" {
					t.Error("wrong model metadata requested")
				}
				_ = json.NewEncoder(w).Encode(localCatalog(p))
			}))
			defer server.Close()
			cfg := deepseekConfig()
			cfg.Profiles[p.ID] = p
			cfg.localProfiles = newLocalProfileVerifier()
			cfg.localProfiles.baseURL = server.URL + "/"
			got, err := cfg.Profile(p.ID)
			if err != nil || Hash(raw(got)) != Hash(raw(p)) {
				t.Fatalf("renewal changed the contract: %v", err)
			}
		})
	}
}

func TestVibePaidLocalEvaluatorChoice(t *testing.T) {
	cfg := deepseekConfig()
	p := alternativeProfiles()[0]
	cfg.Profiles[p.ID] = p
	models := cfg.DefaultModels()
	models.Evaluator = p.ID
	if err := cfg.ValidateModels(models, true); err != nil {
		t.Fatal(err)
	}
	if err := validateRoleModels(cfg, models, true, Evaluator); err != nil {
		t.Fatal(err)
	}
	cfg.LocalTesting = false
	if err := cfg.ValidateModels(models, true); err == nil {
		t.Fatal("hosted anonymous grader was not pinned")
	}
	if err := validateRoleModels(cfg, models, true, Evaluator); err == nil {
		t.Fatal("recorded-reply hosted grader was not pinned")
	}
	if err := cfg.ValidateModels(models, false); err != nil {
		t.Fatal(err)
	}
	cfg.LocalTesting, cfg.FreeOnly = true, true
	if !cfg.EvaluatorPinned(true) {
		t.Fatal("free-only grader was not pinned")
	}
}
