package vibe

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestVibeConfigRejectsInvalidStableSettings(t *testing.T) {
	for _, tc := range []struct{ key, value string }{{"VIBE_ENABLED", "yes"}, {"VIBE_MATERIAL_BUILD", "treu"}, {"VIBE_CAMPAIGN", " unstable campaign "}} {
		t.Run(tc.key, func(t *testing.T) {
			t.Setenv("APP_ENV", "production")
			t.Setenv("VIBE_ENABLED", "true")
			t.Setenv("VIBE_LOCAL_TESTING", "false")
			t.Setenv("VIBE_FREE_ONLY", "false")
			t.Setenv("VIBE_COOKIE_SECRET", strings.Repeat("s", 32))
			t.Setenv("VIBE_CAMPAIGN", "stable-fixture")
			t.Setenv("VIBE_MODELS_JSON", "")
			t.Setenv("VIBE_DEFAULT_MODEL", "")
			t.Setenv(tc.key, tc.value)
			if _, err := LoadConfig(); err == nil {
				t.Errorf("invalid %s accepted", tc.key)
			}
		})
	}
}

func TestVibeExpiredProfilesDoNotStopHistoricalService(t *testing.T) {
	cfg := freeConfig()
	p := cfg.Profiles[cfg.DefaultModel]
	p.ExpiresAt = time.Now().Add(-time.Hour)
	profiles, _ := json.Marshal([]ModelProfile{p})
	t.Setenv("APP_ENV", "production")
	t.Setenv("VIBE_ENABLED", "true")
	t.Setenv("VIBE_COOKIE_SECRET", strings.Repeat("s", 32))
	t.Setenv("VIBE_CAMPAIGN", "stable-fixture")
	t.Setenv("VIBE_FREE_ONLY", "true")
	t.Setenv("VIBE_LOCAL_TESTING", "false")
	t.Setenv("VIBE_DEFAULT_MODEL", cfg.DefaultModel)
	t.Setenv("VIBE_MODELS_JSON", string(profiles))
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("profile expiry must disable inference, not historical service/reconciliation: %v", err)
	}
	if _, err = loaded.Profile(cfg.DefaultModel); err == nil {
		t.Fatal("expired profile permitted new execution")
	}
}

func TestVibeMissingGuestCookiePreservesServiceStartup(t *testing.T) {
	t.Setenv("VIBE_ENABLED", "true")
	t.Setenv("VIBE_COOKIE_SECRET", "")
	t.Setenv("VIBE_LOCAL_TESTING", "false")
	t.Setenv("VIBE_MODELS_JSON", "")
	t.Setenv("VIBE_CAMPAIGN", "")
	if _, err := LoadConfig(); err != nil {
		t.Fatalf("missing guest cookie must leave signed-in history and reconciliation online: %v", err)
	}
}
