package cmd

import (
	"strings"
	"testing"
)

func TestRequireUUID(t *testing.T) {
	if err := requireUUID("--run", "22222222-2222-4222-8222-222222222222"); err != nil {
		t.Fatalf("valid UUID rejected: %v", err)
	}
	for _, bad := range []string{"", "run-1", "22222222-2222-4222-8222", " 22222222-2222-4222-8222-222222222222"} {
		err := requireUUID("--run", bad)
		local := assertCLIErrorCode(t, err, "invalid_argument")
		assertExitCode(t, err, exitValidationError)
		if local.Details["argument"] != "--run" || local.Details["value"] != bad {
			t.Fatalf("details = %v, want argument and value", local.Details)
		}
	}
}

func TestRequireOneOf(t *testing.T) {
	allowed := []string{"left", "right"}
	if err := requireOneOf("winner", "left", allowed); err != nil {
		t.Fatalf("allowed value rejected: %v", err)
	}
	err := requireOneOf("winner", "middle", allowed)
	local := assertCLIErrorCode(t, err, "invalid_argument")
	assertExitCode(t, err, exitValidationError)
	if !strings.Contains(local.Message, "left, right") || local.Details["flag"] != "--winner" {
		t.Fatalf("error = %q details = %v, want the allowed values and the flag", local.Message, local.Details)
	}
}
