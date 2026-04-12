package api

import (
	"strings"
	"testing"
)

func TestLoadConfigFromEnv_DefaultAuthModeDev(t *testing.T) {
	unsetEnv(t, "AUTH_MODE")
	unsetEnv(t, "WORKOS_CLIENT_ID")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthMode != "dev" {
		t.Errorf("AuthMode = %q, want %q", cfg.AuthMode, "dev")
	}
}

func TestLoadConfigFromEnv_WorkOSModeRequiresClientID(t *testing.T) {
	t.Setenv("AUTH_MODE", "workos")
	unsetEnv(t, "WORKOS_CLIENT_ID")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error when AUTH_MODE=workos but WORKOS_CLIENT_ID is empty")
	}
	if !strings.Contains(err.Error(), "WORKOS_CLIENT_ID") {
		t.Errorf("error = %v, want mention of WORKOS_CLIENT_ID", err)
	}
}

func TestLoadConfigFromEnv_WorkOSModeWithClientID(t *testing.T) {
	t.Setenv("AUTH_MODE", "workos")
	t.Setenv("WORKOS_CLIENT_ID", "client_01TEST")
	t.Setenv("APP_ENV", "production")
	t.Setenv("ARTIFACT_SIGNING_SECRET", strings.Repeat("a", 64))

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthMode != "workos" {
		t.Errorf("AuthMode = %q, want %q", cfg.AuthMode, "workos")
	}
	if cfg.WorkOSClientID != "client_01TEST" {
		t.Errorf("WorkOSClientID = %q, want %q", cfg.WorkOSClientID, "client_01TEST")
	}
}

func TestLoadConfigFromEnv_InvalidAuthMode(t *testing.T) {
	t.Setenv("AUTH_MODE", "oauth2")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid AUTH_MODE")
	}
	if !strings.Contains(err.Error(), "AUTH_MODE") {
		t.Errorf("error = %v, want mention of AUTH_MODE", err)
	}
}
