package vibe

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func localCatalog(p ModelProfile) map[string]any {
	return map[string]any{"data": map[string]any{"id": p.ID, "endpoints": []any{map[string]any{
		"model_id": p.ID, "tag": p.Route, "status": 0, "context_length": p.Context,
		"max_completion_tokens": 4096,
		"supported_parameters":  []string{"structured_outputs", "response_format", "max_tokens", "temperature", "reasoning"},
		"pricing":               map[string]any{"prompt": "0.00000003", "completion": "0.00000013", "input_cache_read": "0.00000001", "discount": 0},
	}}}}
}

func TestVibeLocalProfileRenewal(t *testing.T) {
	c := deepseekConfig()
	p := c.Profiles[c.DefaultModel]
	p.ExpiresAt = time.Now().Add(-24 * time.Hour)
	c.Profiles[p.ID] = p
	before := Hash(raw(p))
	var calls atomic.Int32
	var unavailable atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.URL.Path != "/"+p.ID+"/endpoints" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected metadata request %s %s", r.Method, r.URL.Path)
		}
		if unavailable.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(localCatalog(p))
	}))
	defer srv.Close()
	clock := time.Now()
	v := newLocalProfileVerifier()
	v.baseURL, v.now = srv.URL+"/", func() time.Time { return clock }
	c.localProfiles = v
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := c.Profile(p.ID)
			if err != nil || Hash(raw(got)) != before {
				t.Errorf("renewal changed the frozen contract or failed: %v", err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent callers made %d metadata calls", calls.Load())
	}
	// Simulate another day without changing .env or restarting either process.
	clock = clock.Add(24 * time.Hour)
	if _, err := c.Profile(p.ID); err != nil || calls.Load() != 2 {
		t.Fatalf("next-day automatic renewal failed: calls=%d err=%v", calls.Load(), err)
	}
	unavailable.Store(true)
	clock = clock.Add(time.Hour)
	for range 3 {
		if _, err := c.Profile(p.ID); err == nil {
			t.Fatal("expired cache authorized dispatch after catalog failure")
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("failed refresh was not backed off: %d", calls.Load())
	}
	unavailable.Store(false)
	clock = clock.Add(time.Minute)
	if _, err := c.Profile(p.ID); err != nil || calls.Load() != 4 {
		t.Fatalf("recovery after outage failed: calls=%d err=%v", calls.Load(), err)
	}
	// A local cache cannot authorize hosted execution or an unconformed profile.
	c.LocalTesting = false
	if _, err := c.Profile(p.ID); err == nil {
		t.Fatal("hosted mode accepted expired local profile")
	}
	c.LocalTesting = true
	p.Conformed = false
	c.Profiles[p.ID] = p
	if _, err := c.Profile(p.ID); err == nil || calls.Load() != 4 {
		t.Fatal("metadata approved an unconformed model")
	}
}

func TestVibeLocalProfileRejectsCatalogChanges(t *testing.T) {
	c := deepseekConfig()
	p := c.Profiles[c.DefaultModel]
	for name, mutate := range map[string]func(map[string]any, map[string]any){
		"wrong model":                func(d, e map[string]any) { d["id"] = "other/model" },
		"wrong endpoint model":       func(d, e map[string]any) { e["model_id"] = "other/model" },
		"wrong route":                func(d, e map[string]any) { e["tag"] = "other/provider" },
		"duplicate route":            func(d, e map[string]any) { d["endpoints"] = []any{e, e} },
		"unavailable":                func(d, e map[string]any) { e["status"] = 1 },
		"missing status":             func(d, e map[string]any) { delete(e, "status") },
		"smaller context":            func(d, e map[string]any) { e["context_length"] = p.Context - 1 },
		"smaller prompt":             func(d, e map[string]any) { e["max_prompt_tokens"] = 1000 },
		"smaller output":             func(d, e map[string]any) { e["max_completion_tokens"] = 2048 },
		"missing structured outputs": func(d, e map[string]any) { e["supported_parameters"] = []string{"max_tokens"} },
		"price increase":             func(d, e map[string]any) { e["pricing"].(map[string]any)["prompt"] = "0.000000101" },
		"missing completion price":   func(d, e map[string]any) { delete(e["pricing"].(map[string]any), "completion") },
		"negative price":             func(d, e map[string]any) { e["pricing"].(map[string]any)["prompt"] = "-0.1" },
		"malformed price":            func(d, e map[string]any) { e["pricing"].(map[string]any)["completion"] = "NaN" },
		"request fee":                func(d, e map[string]any) { e["pricing"].(map[string]any)["request"] = "0.001" },
		"cache price increase":       func(d, e map[string]any) { e["pricing"].(map[string]any)["input_cache_read"] = "0.1" },
		"new charge":                 func(d, e map[string]any) { e["pricing"].(map[string]any)["new_charge"] = "0.1" },
	} {
		t.Run(name, func(t *testing.T) {
			body := localCatalog(p)
			d := body["data"].(map[string]any)
			mutate(d, d["endpoints"].([]any)[0].(map[string]any))
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer srv.Close()
			v := newLocalProfileVerifier()
			v.baseURL = srv.URL + "/"
			if err := v.verify(p); err == nil {
				t.Fatal("unsafe catalog approved")
			}
		})
	}
}

func TestVibeLocalProfileCacheIsBoundToContract(t *testing.T) {
	c := deepseekConfig()
	p := c.Profiles[c.DefaultModel]
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(localCatalog(p))
	}))
	defer srv.Close()
	v := newLocalProfileVerifier()
	v.baseURL = srv.URL + "/"
	if err := v.verify(p); err != nil {
		t.Fatal(err)
	}
	changed := p
	changed.InputNanoPerToken = 1
	if err := v.verify(changed); err == nil || calls.Load() != 2 {
		t.Fatal("changed ceiling reused a prior profile's verification")
	}
}

func TestVibeLocalProfileBadTransport(t *testing.T) {
	p := deepseekConfig().Profiles["deepseek/deepseek-v4-flash-0731"]
	for name, body := range map[string]string{"malformed": "{", "oversized": strings.Repeat(" ", (1<<20)+1), "null": "null"} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			v := newLocalProfileVerifier()
			v.baseURL = srv.URL + "/"
			if err := v.verify(p); err == nil {
				t.Fatal("bad catalog transport approved")
			}
		})
	}
}
