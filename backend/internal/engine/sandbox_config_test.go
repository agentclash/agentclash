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
	if err := applySandboxConfig(request, manifest); err != nil {
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
	if err := applySandboxConfig(request, manifest); err != nil {
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
	if err := applySandboxConfig(request, manifest); err != nil {
		t.Fatalf("applySandboxConfig returned error: %v", err)
	}

	if request.TemplateID != "from-sandbox" {
		t.Errorf("expected TemplateID=from-sandbox, got %q", request.TemplateID)
	}
}

func TestApplySandboxConfig_InvalidJSON(t *testing.T) {
	request := &sandbox.CreateRequest{}
	if err := applySandboxConfig(request, json.RawMessage(`{invalid`)); err != nil {
		t.Fatalf("applySandboxConfig on invalid JSON returned error: %v", err)
	}

	// Should not panic, should leave request unchanged
	if request.TemplateID != "" {
		t.Errorf("expected empty TemplateID on invalid JSON, got %q", request.TemplateID)
	}
}

func TestApplySandboxConfig_RejectsSecretsInEnvVars(t *testing.T) {
	manifest := json.RawMessage(`{
		"sandbox": {
			"env_vars": {"DB_URL": "${secrets.DB_URL}"}
		}
	}`)

	request := &sandbox.CreateRequest{}
	err := applySandboxConfig(request, manifest)
	if err == nil {
		t.Fatalf("expected error for secrets in env_vars, got nil")
	}
	if !strings.Contains(err.Error(), "DB_URL") {
		t.Fatalf("error should name the offending env var: %v", err)
	}
	if !strings.Contains(err.Error(), "http_request") {
		t.Fatalf("error should point at the sanctioned path: %v", err)
	}
	if !strings.Contains(err.Error(), "#186") {
		t.Fatalf("error should reference the issue: %v", err)
	}
}

func TestApplySandboxConfig_RejectsAllPlaceholdersInEnvVars(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"parameters namespace", "${parameters.url}"},
		{"unknown namespace", "${something.foo}"},
		{"unclosed placeholder", "${secrets.DB_URL"},
		{"embedded in longer string", "prefix-${secrets.TOKEN}-suffix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := json.RawMessage(`{
				"sandbox": {"env_vars": {"BAD": "` + tc.value + `"}}
			}`)
			request := &sandbox.CreateRequest{}
			if err := applySandboxConfig(request, manifest); err == nil {
				t.Fatalf("expected error for placeholder %q, got nil", tc.value)
			}
		})
	}
}

func TestApplySandboxConfig_AcceptsLiteralEnvVars(t *testing.T) {
	manifest := json.RawMessage(`{
		"sandbox": {
			"env_vars": {
				"DB_URL": "postgres://localhost:5432/app",
				"SERVICE_URL": "https://api.example.com",
				"FEATURE_FLAGS": "a=1,b=2",
				"EMPTY": ""
			}
		}
	}`)
	request := &sandbox.CreateRequest{}
	if err := applySandboxConfig(request, manifest); err != nil {
		t.Fatalf("applySandboxConfig returned error: %v", err)
	}
	if got, want := request.EnvVars["DB_URL"], "postgres://localhost:5432/app"; got != want {
		t.Errorf("DB_URL = %q, want %q", got, want)
	}
	if got, want := request.EnvVars["SERVICE_URL"], "https://api.example.com"; got != want {
		t.Errorf("SERVICE_URL = %q, want %q", got, want)
	}
	if got, want := request.EnvVars["FEATURE_FLAGS"], "a=1,b=2"; got != want {
		t.Errorf("FEATURE_FLAGS = %q, want %q", got, want)
	}
}
