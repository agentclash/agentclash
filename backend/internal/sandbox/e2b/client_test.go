package e2b

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/sandbox"
)

func TestCreateSandboxRequestUsesCamelCaseInternetField(t *testing.T) {
	payload, err := json.Marshal(createSandboxRequest{
		TemplateID:          "template",
		Timeout:             300,
		Secure:              true,
		AllowInternetAccess: false,
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if _, ok := decoded["allowInternetAccess"]; !ok {
		t.Fatalf("payload missing allowInternetAccess field: %s", string(payload))
	}
	if _, ok := decoded["allow_internet_access"]; ok {
		t.Fatalf("payload unexpectedly contains allow_internet_access field: %s", string(payload))
	}
}

func TestCreateSandboxRequestEnvVars(t *testing.T) {
	payload, err := json.Marshal(createSandboxRequest{
		TemplateID: "template",
		Timeout:    300,
		Secure:     true,
		EnvVars:    map[string]string{"FOO": "bar", "DB_URL": "postgres://localhost"},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	envVars, ok := decoded["envVars"]
	if !ok {
		t.Fatalf("payload missing envVars field: %s", string(payload))
	}
	envMap, ok := envVars.(map[string]any)
	if !ok {
		t.Fatalf("envVars is not a map: %T", envVars)
	}
	if envMap["FOO"] != "bar" {
		t.Errorf("envVars[FOO] = %v, want bar", envMap["FOO"])
	}
}

func TestCreateSandboxRequestNetwork(t *testing.T) {
	payload, err := json.Marshal(createSandboxRequest{
		TemplateID: "template",
		Timeout:    300,
		Secure:     true,
		Network:    &networkConfig{AllowOut: []string{"10.0.0.0/8", "192.168.0.0/16"}},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	network, ok := decoded["network"]
	if !ok {
		t.Fatalf("payload missing network field: %s", string(payload))
	}
	netMap, ok := network.(map[string]any)
	if !ok {
		t.Fatalf("network is not a map: %T", network)
	}
	allowOut, ok := netMap["allowOut"]
	if !ok {
		t.Fatalf("network missing allowOut field: %s", string(payload))
	}
	arr, ok := allowOut.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("allowOut = %v, want 2-element array", allowOut)
	}
}

func TestCreateSandboxRequestNoOptionalFields(t *testing.T) {
	payload, err := json.Marshal(createSandboxRequest{
		TemplateID: "template",
		Timeout:    300,
		Secure:     true,
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if _, ok := decoded["envVars"]; ok {
		t.Errorf("payload should not contain envVars when empty: %s", string(payload))
	}
	if _, ok := decoded["network"]; ok {
		t.Errorf("payload should not contain network when nil: %s", string(payload))
	}
	if _, ok := decoded["metadata"]; ok {
		t.Errorf("payload should not contain metadata when nil: %s", string(payload))
	}
}

func TestNormalizeHTTPErrorUsesOperationSpecificNotFoundError(t *testing.T) {
	if err := normalizeHTTPError(404, "missing file", sandbox.ErrFileNotFound); !errors.Is(err, sandbox.ErrFileNotFound) {
		t.Fatalf("404 error = %v, want sandbox.ErrFileNotFound", err)
	}
	if err := normalizeHTTPError(404, "missing sandbox", sandbox.ErrSandboxNotFound); !errors.Is(err, sandbox.ErrSandboxNotFound) {
		t.Fatalf("404 error = %v, want sandbox.ErrSandboxNotFound", err)
	}
}
