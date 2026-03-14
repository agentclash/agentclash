package e2b

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
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

func TestNormalizeHTTPErrorUsesOperationSpecificNotFoundError(t *testing.T) {
	if err := normalizeHTTPError(404, "missing file", sandbox.ErrFileNotFound); !errors.Is(err, sandbox.ErrFileNotFound) {
		t.Fatalf("404 error = %v, want sandbox.ErrFileNotFound", err)
	}
	if err := normalizeHTTPError(404, "missing sandbox", sandbox.ErrSandboxNotFound); !errors.Is(err, sandbox.ErrSandboxNotFound) {
		t.Fatalf("404 error = %v, want sandbox.ErrSandboxNotFound", err)
	}
}
