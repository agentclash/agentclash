package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestApplySandboxConfig_WithSandboxBlock(t *testing.T) {
	manifest := json.RawMessage(`{
		"sandbox": {
			"network_access": true,
			"network_allowlist": ["10.0.0.0/8"],
			"env_vars": {"DB_URL": "postgres://localhost"},
			"additional_packages": ["ffmpeg"],
			"sandbox_template_id": "custom-template"
		},
		"version": {
			"number": 1,
			"sandbox_template_id": "pinned-template"
		}
	}`)

	request := &sandbox.CreateRequest{}
	if err := applySandboxConfig(request, manifest, nil); err != nil {
		t.Fatalf("applySandboxConfig returned error: %v", err)
	}

	if !request.ToolPolicy.AllowNetwork {
		t.Error("expected AllowNetwork=true from sandbox.network_access")
	}
	if len(request.NetworkAllowlist) != 1 || request.NetworkAllowlist[0] != "10.0.0.0/8" {
		t.Errorf("expected network_allowlist=[10.0.0.0/8], got %v", request.NetworkAllowlist)
	}
	if request.EnvVars["DB_URL"] != "postgres://localhost" {
		t.Errorf("expected DB_URL env var, got %v", request.EnvVars)
	}
	if len(request.AdditionalPackages) != 1 || request.AdditionalPackages[0] != "ffmpeg" {
		t.Errorf("expected additional_packages=[ffmpeg], got %v", request.AdditionalPackages)
	}
	// version.sandbox_template_id takes precedence over sandbox.sandbox_template_id
	if request.TemplateID != "pinned-template" {
		t.Errorf("expected TemplateID=pinned-template (from version block), got %q", request.TemplateID)
	}
}

func TestApplySandboxConfig_WithoutSandboxBlock(t *testing.T) {
	manifest := json.RawMessage(`{
		"tool_policy": {"allow_shell": true},
		"version": {"number": 1}
	}`)

	request := &sandbox.CreateRequest{
		ToolPolicy: sandbox.ToolPolicy{AllowNetwork: false},
	}
	if err := applySandboxConfig(request, manifest, nil); err != nil {
		t.Fatalf("applySandboxConfig returned error: %v", err)
	}

	if request.ToolPolicy.AllowNetwork {
		t.Error("expected AllowNetwork to remain false when no sandbox block")
	}
	if len(request.NetworkAllowlist) != 0 {
		t.Error("expected empty NetworkAllowlist when no sandbox block")
	}
	if len(request.EnvVars) != 0 {
		t.Error("expected empty EnvVars when no sandbox block")
	}
	if len(request.AdditionalPackages) != 0 {
		t.Error("expected empty AdditionalPackages when no sandbox block")
	}
	if request.TemplateID != "" {
		t.Errorf("expected empty TemplateID when no sandbox block, got %q", request.TemplateID)
	}
}

func TestApplySandboxConfig_TemplateIDFromSandboxOnly(t *testing.T) {
	manifest := json.RawMessage(`{
		"sandbox": {
			"sandbox_template_id": "from-sandbox"
		},
		"version": {"number": 1}
	}`)

	request := &sandbox.CreateRequest{}
	if err := applySandboxConfig(request, manifest, nil); err != nil {
		t.Fatalf("applySandboxConfig returned error: %v", err)
	}

	if request.TemplateID != "from-sandbox" {
		t.Errorf("expected TemplateID=from-sandbox, got %q", request.TemplateID)
	}
}

func TestApplySandboxConfig_InvalidJSON(t *testing.T) {
	request := &sandbox.CreateRequest{}
	if err := applySandboxConfig(request, json.RawMessage(`{invalid`), nil); err != nil {
		t.Fatalf("applySandboxConfig on invalid JSON returned error: %v", err)
	}

	// Should not panic, should leave request unchanged
	if request.TemplateID != "" {
		t.Errorf("expected empty TemplateID on invalid JSON, got %q", request.TemplateID)
	}
}

func TestApplySandboxConfig_ResolvesSecretsInEnvVars(t *testing.T) {
	manifest := json.RawMessage(`{
		"sandbox": {
			"env_vars": {
				"DB_URL": "${secrets.DB_URL}",
				"COMBINED": "prefix-${secrets.TOKEN}-suffix",
				"LITERAL": "plain-value"
			}
		}
	}`)
	secrets := map[string]string{
		"DB_URL": "postgres://user:pass@host/db",
		"TOKEN":  "abc123",
	}

	request := &sandbox.CreateRequest{}
	if err := applySandboxConfig(request, manifest, secrets); err != nil {
		t.Fatalf("applySandboxConfig returned error: %v", err)
	}
	if got, want := request.EnvVars["DB_URL"], "postgres://user:pass@host/db"; got != want {
		t.Errorf("DB_URL = %q, want %q", got, want)
	}
	if got, want := request.EnvVars["COMBINED"], "prefix-abc123-suffix"; got != want {
		t.Errorf("COMBINED = %q, want %q", got, want)
	}
	if got, want := request.EnvVars["LITERAL"], "plain-value"; got != want {
		t.Errorf("LITERAL = %q, want %q", got, want)
	}
}

func TestApplySandboxConfig_MissingSecretIsError(t *testing.T) {
	manifest := json.RawMessage(`{
		"sandbox": {
			"env_vars": {"DB_URL": "${secrets.DB_URL}"}
		}
	}`)

	request := &sandbox.CreateRequest{}
	err := applySandboxConfig(request, manifest, map[string]string{})
	if err == nil {
		t.Fatalf("expected error for missing secret, got nil")
	}
	if !strings.Contains(err.Error(), "DB_URL") {
		t.Fatalf("error should name the missing secret: %v", err)
	}
}

func TestApplySandboxConfig_RejectsNonSecretsNamespaceInEnvVars(t *testing.T) {
	manifest := json.RawMessage(`{
		"sandbox": {
			"env_vars": {"BAD": "${parameters.url}"}
		}
	}`)

	request := &sandbox.CreateRequest{}
	err := applySandboxConfig(request, manifest, nil)
	if err == nil {
		t.Fatalf("expected error for parameters namespace, got nil")
	}
	if !strings.Contains(err.Error(), "only ${secrets.X}") {
		t.Fatalf("error should reject non-secrets namespace: %v", err)
	}
}

func TestApplySandboxConfig_RejectsUnclosedPlaceholder(t *testing.T) {
	manifest := json.RawMessage(`{
		"sandbox": {
			"env_vars": {"BAD": "${secrets.DB_URL"}
		}
	}`)

	request := &sandbox.CreateRequest{}
	err := applySandboxConfig(request, manifest, map[string]string{"DB_URL": "x"})
	if err == nil {
		t.Fatalf("expected error for unclosed placeholder, got nil")
	}
	if !strings.Contains(err.Error(), "unclosed placeholder") {
		t.Fatalf("error should mention unclosed placeholder: %v", err)
	}
}
