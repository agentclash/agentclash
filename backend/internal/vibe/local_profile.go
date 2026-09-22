package vibe

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"
)

// Local development should not stop overnight because a manually copied
// catalog snapshot expired. Recheck public metadata on first use and hourly.
// This does NOT run inference, approve a new route, increase a price ceiling,
// extend conformance to another model, or modify an operation's frozen profile.
// The paid gateway continues to enforce provider routing and max_price.
type localProfileVerifier struct {
	mu      sync.Mutex
	client  *http.Client
	baseURL string
	now     func() time.Time
	entries map[string]localProfileCheck
}

type localProfileCheck struct {
	next time.Time
	err  error
}

func newLocalProfileVerifier() *localProfileVerifier {
	return &localProfileVerifier{
		client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("catalog redirect refused")
		}},
		baseURL: "https://openrouter.ai/api/v1/models/",
		now:     time.Now,
		entries: make(map[string]localProfileCheck),
	}
}

func (v *localProfileVerifier) verify(p ModelProfile) error {
	// Config values are copied across services. One shared verifier serializes
	// refreshes so concurrent config/admission/gateway calls do not stampede.
	v.mu.Lock()
	defer v.mu.Unlock()
	key := Hash(raw(p))
	if cached, ok := v.entries[key]; ok && v.now().Before(cached.next) {
		return cached.err
	}
	err := v.fetch(p)
	ttl := time.Hour
	if err != nil {
		ttl = time.Minute
		slog.Warn("vibe local model metadata verification failed", "model", p.ID, "route", p.Route, "reason", err.Error())
	} else {
		slog.Info("vibe local model metadata verified", "model", p.ID, "route", p.Route, "refresh_after", v.now().Add(ttl))
	}
	v.entries[key] = localProfileCheck{next: v.now().Add(ttl), err: err}
	return err
}

type localEndpoint struct {
	ModelID       string                     `json:"model_id"`
	Tag           string                     `json:"tag"`
	Status        *int                       `json:"status"`
	Context       int                        `json:"context_length"`
	MaxPrompt     *int                       `json:"max_prompt_tokens"`
	MaxCompletion *int                       `json:"max_completion_tokens"`
	Parameters    []string                   `json:"supported_parameters"`
	Pricing       map[string]json.RawMessage `json:"pricing"`
}

func (v *localProfileVerifier) fetch(p ModelProfile) error {
	// Only called after Config.Profile has checked the approved model and route.
	res, err := v.client.Get(v.baseURL + p.ID + "/endpoints")
	if err != nil {
		return fmt.Errorf("catalog request failed")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("catalog returned HTTP %d", res.StatusCode)
	}
	const maxBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil || len(data) > maxBytes {
		return fmt.Errorf("catalog response unreadable or too large")
	}
	var catalog struct {
		Data struct {
			ID        string          `json:"id"`
			Endpoints []localEndpoint `json:"endpoints"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &catalog) != nil || catalog.Data.ID != p.ID {
		return fmt.Errorf("catalog model mismatch or invalid response")
	}
	var matches []localEndpoint
	for _, e := range catalog.Data.Endpoints {
		if e.Tag == p.Route {
			matches = append(matches, e)
		}
	}
	if len(matches) != 1 {
		return fmt.Errorf("approved route missing or ambiguous")
	}
	e := matches[0]
	if e.ModelID != p.ID || e.Status == nil || *e.Status != 0 {
		return fmt.Errorf("approved endpoint unavailable or model mismatch")
	}
	if e.Context < p.Context || (e.MaxPrompt != nil && *e.MaxPrompt < p.Context) || (e.MaxCompletion != nil && *e.MaxCompletion < 4096) {
		return fmt.Errorf("endpoint context or output allowance decreased")
	}
	for _, parameter := range []string{"structured_outputs", "response_format", "max_tokens", "temperature", "reasoning"} {
		if !slices.Contains(e.Parameters, parameter) {
			return fmt.Errorf("endpoint no longer supports %s", parameter)
		}
	}
	for field, ceiling := range map[string]int64{"prompt": p.InputNanoPerToken, "completion": p.OutputNanoPerToken} {
		price, err := catalogPrice(e.Pricing[field])
		if err != nil || price <= 0 || price > ceiling {
			return fmt.Errorf("endpoint %s price missing, invalid, or above ceiling", field)
		}
	}
	// Zero/omitted ancillary prices are allowed. A cache read cannot cost more
	// than the reserved uncached input. Unknown nonzero charges need review.
	for field, raw := range e.Pricing {
		if field == "prompt" || field == "completion" || field == "discount" {
			continue
		}
		ceiling := int64(0)
		if field == "input_cache_read" {
			ceiling = p.InputNanoPerToken
		}
		price, err := catalogPrice(raw)
		if err != nil || price > ceiling {
			return fmt.Errorf("endpoint %s price requires review", field)
		}
	}
	return nil
}

func catalogPrice(raw json.RawMessage) (int64, error) {
	var amount string
	if json.Unmarshal(raw, &amount) != nil {
		return 0, fmt.Errorf("invalid catalog price")
	}
	return ParseUSD(amount)
}
