package vibe

import (
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/tiktoken-go/tokenizer"
	"math"
	"math/big"
	"os"
	"strings"
	"time"
)

// Profiles are operator-approved price ceilings, never model-generated metadata.
// Expired, absent or unverified profiles cannot authorize hosted execution.
type ModelProfile struct {
	Free               bool      `json:"free,omitempty"`
	DisableReasoning   bool      `json:"disable_reasoning,omitempty"`
	StructuredOutputs  bool      `json:"structured_outputs,omitempty"`
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Route              string    `json:"route"`
	InputNanoPerToken  int64     `json:"input_nano_per_token"`
	OutputNanoPerToken int64     `json:"output_nano_per_token"`
	Context            int       `json:"context_tokens"`
	FramingAllowance   int       `json:"framing_allowance"`
	Conformed          bool      `json:"conformed"`
	ExpiresAt          time.Time `json:"expires_at"`
}
type Config struct {
	FreeOnly          bool
	DefaultModel      string
	Enabled           bool
	Credential        string
	Profiles          map[string]ModelProfile
	AnonymousDaily    int64
	AnonymousCampaign int64
	Campaign          string
}

func LoadConfig() (Config, error) {
	c := Config{Enabled: os.Getenv("VIBE_ENABLED") == "true", Credential: os.Getenv("VIBE_OPENROUTER_KEY"), Profiles: map[string]ModelProfile{}, Campaign: os.Getenv("VIBE_CAMPAIGN")}
	c.FreeOnly = os.Getenv("VIBE_FREE_ONLY") == "true"
	c.DefaultModel = os.Getenv("VIBE_DEFAULT_MODEL")
	var profiles []ModelProfile
	if raw := os.Getenv("VIBE_MODELS_JSON"); raw != "" {
		if err := Decode([]byte(raw), LimitsFor(false), &profiles); err != nil {
			return c, fmt.Errorf("VIBE_MODELS_JSON: %w", err)
		}
	}
	for _, p := range profiles {
		if _, ok := c.Profiles[p.ID]; ok {
			return c, fmt.Errorf("duplicate model profile")
		}
		c.Profiles[p.ID] = p
	}
	for key, dst := range map[string]*int64{"VIBE_ANON_DAILY_USD": &c.AnonymousDaily, "VIBE_ANON_CAMPAIGN_USD": &c.AnonymousCampaign} {
		if s := os.Getenv(key); s != "" {
			n, err := ParseUSD(s)
			if err != nil {
				return c, fmt.Errorf("%s: %w", key, err)
			}
			*dst = n
		}
	}
	if c.Enabled && (c.FreeOnly || c.DefaultModel != "") {
		if err := c.ValidateModels(c.DefaultModels(), true); err != nil {
			return c, fmt.Errorf("Vibe default models: %w", err)
		}
	}
	return c, nil
}

func (c Config) DefaultModels() Models {
	if c.DefaultModel == "" {
		return DefaultModels()
	}
	return Models{Assistant: c.DefaultModel, Target: c.DefaultModel, Evaluator: c.DefaultModel}
}

// Free routes are explicit pilot profiles, not an inference from missing prices.
// Exact endpoint slugs avoid routing an unpaid key to a paid model/provider.
func (p ModelProfile) validFreeRoute() bool {
	return p.Free && p.InputNanoPerToken == 0 && p.OutputNanoPerToken == 0 &&
		((p.ID == "liquid/lfm-2.5-2.6b:free" && p.Route == "liquid/fp8") ||
			(p.ID == "google/gemma-4-31b-it:free" && p.Route == "google-ai-studio") ||
			(p.ID == "dots-studio/dots-3-note-preview:free" && p.Route == "atlas-cloud/fp8"))
}
func (c Config) Profile(id string) (ModelProfile, error) {
	p, ok := c.Profiles[id]
	if !ok || p.ID != id || !p.Conformed || p.ExpiresAt.Before(time.Now()) || p.FramingAllowance < 2048 || p.Context < 32768 {
		return p, fault("pricing_unavailable", "This model is unavailable until its price and context profile is verified.")
	}
	if c.FreeOnly {
		if !p.validFreeRoute() {
			return p, fault("free_model_required", "This server only allows its verified free model routes.")
		}
		return p, nil
	}
	if p.Free || p.InputNanoPerToken <= 0 || p.OutputNanoPerToken <= 0 || p.InputNanoPerToken > 100_000 || p.OutputNanoPerToken > 100_000 || p.Route != "openai" {
		return p, fault("pricing_unavailable", "This model has no approved paid price profile.")
	}
	switch p.ID {
	case "openai/gpt-4o-mini", "openai/gpt-4.1-mini", "openai/gpt-4.1":
	default:
		return p, fault("unsupported_model", "This model has no verified text token profile.")
	}
	return p, nil
}
func (c Config) ValidateModels(m Models, anon bool) error {
	for _, id := range []string{m.Assistant, m.Target, m.Evaluator} {
		if _, err := c.Profile(id); err != nil {
			return err
		}
	}
	if anon && m.Evaluator != c.DefaultModels().Evaluator {
		return fault("evaluator_pinned", "The free trial uses a fixed evaluator for comparable results.")
	}
	return nil
}
func ParseUSD(s string) (int64, error) {
	if len(s) == 0 || len(s) > 64 || strings.ContainsAny(s, "eE/ ") {
		return 0, fmt.Errorf("invalid USD amount")
	}
	n, ok := new(big.Rat).SetString(s)
	if !ok || n.Sign() < 0 {
		return 0, fmt.Errorf("invalid USD amount")
	}
	n.Mul(n, new(big.Rat).SetInt64(NanoUSD))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n.Num(), n.Denom(), r)
	if r.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("USD amount overflow")
	}
	return q.Int64(), nil
}
func (p ModelProfile) BoundCost(in, out int) (int64, error) {
	if in >= 0 && out >= 0 && p.validFreeRoute() {
		return 0, nil
	}
	if in < 0 || out < 0 || p.InputNanoPerToken <= 0 || p.OutputNanoPerToken <= 0 {
		return 0, fault("pricing_unavailable", "Cannot safely price this request.")
	}
	n := new(big.Int).Mul(big.NewInt(int64(in)), big.NewInt(p.InputNanoPerToken))
	n.Add(n, new(big.Int).Mul(big.NewInt(int64(out)), big.NewInt(p.OutputNanoPerToken)))
	if !n.IsInt64() || n.Int64() > MaxOperationCost {
		return 0, fault("budget_limit", "Request exceeds the maximum operation budget.")
	}
	return n.Int64(), nil
}
func AddCost(a, b int64) (int64, error) {
	if a < 0 || b < 0 || a > math.MaxInt64-b {
		return 0, fmt.Errorf("cost overflow")
	}
	return a + b, nil
}

type ContextCount struct {
	Method     string `json:"method,omitempty"`
	Estimate   int    `json:"estimate"`
	UpperBound int    `json:"upper_bound"`
}

// CountContext is applied to each complete assembled invocation, independently
// for every role. The tokenizer estimate is diagnostic, not an accounting bound.
// Byte-level BPE cannot require more content tokens than UTF-8 bytes; an explicitly
// conformed, pinned provider profile supplies the additional framing allowance.
func CountContext(req provider.Request, p ModelProfile, l Limits) (ContextCount, error) {
	b, err := json.Marshal(struct {
		Messages []provider.Message        `json:"messages"`
		Tools    []provider.ToolDefinition `json:"tools"`
		Schema   json.RawMessage           `json:"response_format"`
	}{req.Messages, req.Tools, req.ResponseFormat})
	if err != nil {
		return ContextCount{}, err
	}
	n := ContextCount{UpperBound: len(b) + p.FramingAllowance}
	if n.UpperBound > l.ContextTokens || req.MaxOutputTokens <= 0 || req.MaxOutputTokens > l.OutputTokens || n.UpperBound+req.MaxOutputTokens > p.Context {
		return n, contextFault(req, n, p, l)
	}
	if p.Free {
		// These tokenizers are not o200k. Report only the conservative byte bound;
		// do not present an OpenAI tokenizer estimate as this model's token count.
		n.Method = "utf8_bytes_plus_verified_framing"
		return n, nil
	}
	enc, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		return n, err
	}
	n.Estimate, err = enc.Count(string(b))
	return n, err
}
