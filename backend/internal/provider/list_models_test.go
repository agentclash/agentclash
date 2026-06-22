package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleListModelsEnrichesStaticPricing(t *testing.T) {
	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4.1-mini-2025-04-14"},{"id":"gpt-4o"}]}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(server.Client(), server.URL+"/v1", staticCredentialResolver{value: "sk-test"})
	models, err := client.ListModels(context.Background(), ListModelsRequest{ProviderKey: "openai", CredentialReference: "ref"})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("path = %q", gotPath)
	}
	if len(models) != 2 {
		t.Fatalf("want 2 models, got %d", len(models))
	}
	// sorted by id: gpt-4.1-mini-... before gpt-4o
	if models[0].ID != "gpt-4.1-mini-2025-04-14" {
		t.Fatalf("first model = %q", models[0].ID)
	}
	// resolved via "gpt-4.1-mini" prefix
	if models[0].PricingSource != PricingSourceStatic || models[0].InputCostPerMTok != 0.40 {
		t.Fatalf("static pricing not applied: %+v", models[0])
	}
}

func TestOpenRouterListModelsUsesLivePricing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"anthropic/claude-sonnet-4","name":"Claude Sonnet 4","pricing":{"prompt":"0.000003","completion":"0.000015"}}]}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(server.Client(), server.URL+"/api/v1", staticCredentialResolver{value: "k"})
	// baseURL has no openrouter.ai host in test, but live pricing is keyed off the
	// presence of a pricing object in the response, so it still resolves to live.
	models, err := client.ListModels(context.Background(), ListModelsRequest{ProviderKey: "openrouter", CredentialReference: "ref"})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("want 1 model, got %d", len(models))
	}
	m := models[0]
	if m.PricingSource != PricingSourceLive {
		t.Fatalf("pricing source = %q", m.PricingSource)
	}
	if m.InputCostPerMTok != 3.0 || m.OutputCostPerMTok != 15.0 {
		t.Fatalf("USD/token not scaled to per-MTok: %+v", m)
	}
	if m.DisplayName != "Claude Sonnet 4" {
		t.Fatalf("display name = %q", m.DisplayName)
	}
}

func TestOpenRouterListModelsFallsBackWhenLivePricingIsPartial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4.1-mini","pricing":{"prompt":"0.000003","completion":""}}]}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(server.Client(), server.URL+"/api/v1", staticCredentialResolver{value: "k"})
	models, err := client.ListModels(context.Background(), ListModelsRequest{ProviderKey: "openai", CredentialReference: "ref"})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("want 1 model, got %d", len(models))
	}
	model := models[0]
	if model.PricingSource != PricingSourceStatic || model.InputCostPerMTok != 0.40 || model.OutputCostPerMTok != 1.60 {
		t.Fatalf("partial live pricing should use complete static fallback: %+v", model)
	}
}

func TestParseUSDPerTokenRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "-Inf"} {
		if parsed, ok := parseUSDPerToken(value); ok {
			t.Fatalf("parseUSDPerToken(%q) = %v, true; want rejected", value, parsed)
		}
	}
}

func TestAnthropicListModels(t *testing.T) {
	var gotKey, gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-4-20250514","display_name":"Claude Sonnet 4"}]}`))
	}))
	defer server.Close()

	client := NewAnthropicClient(server.Client(), server.URL, "", staticCredentialResolver{value: "ak"})
	models, err := client.ListModels(context.Background(), ListModelsRequest{ProviderKey: "anthropic", CredentialReference: "ref"})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotKey != "ak" || gotVersion == "" {
		t.Fatalf("headers: key=%q version=%q", gotKey, gotVersion)
	}
	if len(models) != 1 || models[0].DisplayName != "Claude Sonnet 4" {
		t.Fatalf("models = %+v", models)
	}
	if models[0].PricingSource != PricingSourceStatic || models[0].InputCostPerMTok != 3.0 {
		t.Fatalf("static pricing via claude-sonnet-4 prefix not applied: %+v", models[0])
	}
}

func TestGeminiListModelsStripsPrefixAndFiltersMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "" {
			t.Errorf("missing key query param")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[
			{"name":"models/gemini-2.5-pro","displayName":"Gemini 2.5 Pro","supportedGenerationMethods":["generateContent"]},
			{"name":"models/embedding-001","displayName":"Embedding","supportedGenerationMethods":["embedContent"]}
		]}`))
	}))
	defer server.Close()

	client := NewGeminiClient(server.Client(), server.URL, staticCredentialResolver{value: "gk"})
	models, err := client.ListModels(context.Background(), ListModelsRequest{ProviderKey: "gemini", CredentialReference: "ref"})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("want 1 generation model, got %d: %+v", len(models), models)
	}
	if models[0].ID != "gemini-2.5-pro" {
		t.Fatalf("prefix not stripped: %q", models[0].ID)
	}
}

func TestListModelsPropagatesAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(server.Client(), server.URL+"/v1", staticCredentialResolver{value: "x"})
	_, err := client.ListModels(context.Background(), ListModelsRequest{ProviderKey: "openai", CredentialReference: "ref"})
	failure, ok := AsFailure(err)
	if !ok || failure.Code != FailureCodeAuth {
		t.Fatalf("want auth failure, got %v", err)
	}
}

func TestStaticModelPriceLongestPrefix(t *testing.T) {
	in, out, ok := StaticModelPrice("openai", "gpt-4.1-mini-2025-04-14")
	if !ok || in != 0.40 || out != 1.60 {
		t.Fatalf("gpt-4.1-mini prefix: in=%v out=%v ok=%v", in, out, ok)
	}
	if _, _, ok := StaticModelPrice("openai", "unknown-model"); ok {
		t.Fatalf("unknown model should not match")
	}
	if _, _, ok := StaticModelPrice("nonprovider", "x"); ok {
		t.Fatalf("unknown provider should not match")
	}
}

func TestRouterListModelsUnsupportedProvider(t *testing.T) {
	router := NewRouter(map[string]Client{})
	_, err := router.ListModels(context.Background(), ListModelsRequest{ProviderKey: "nope"})
	failure, ok := AsFailure(err)
	if !ok || failure.Code != FailureCodeUnsupportedProvider {
		t.Fatalf("want unsupported_provider, got %v", err)
	}
}

// fakeNonLister implements Client but not ModelLister.
type fakeNonLister struct{}

func (fakeNonLister) InvokeModel(context.Context, Request) (Response, error) {
	return Response{}, nil
}

func TestRouterListModelsUnsupportedCapability(t *testing.T) {
	router := NewRouter(map[string]Client{"x": fakeNonLister{}})
	_, err := router.ListModels(context.Background(), ListModelsRequest{ProviderKey: "x"})
	failure, ok := AsFailure(err)
	if !ok || failure.Code != FailureCodeUnsupportedCapability {
		t.Fatalf("want unsupported_capability, got %v", err)
	}
}
