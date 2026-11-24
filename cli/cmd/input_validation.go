package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// requireUUID rejects a value that is not a UUID before any HTTP call. The
// result is an invalid_argument error (exit 64), so an agent can tell a typo
// apart from a resource that the server cannot find (exit 66).
func requireUUID(label, value string) error {
	if _, err := uuid.Parse(value); err == nil {
		return nil
	}
	return &cliError{
		Code:    "invalid_argument",
		Message: fmt.Sprintf("%s must be a UUID, got %q", label, value),
		Details: map[string]any{"argument": label, "value": value},
	}
}

// requireOneOf rejects a flag value that is not in the closed set. Keep the
// allowed slice the same as the flagAllowedValues entry for the flag, so that
// the schema and the local check cannot drift.
func requireOneOf(flag, value string, allowed []string) error {
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return &cliError{
		Code:    "invalid_argument",
		Message: fmt.Sprintf("--%s must be one of %s, got %q", flag, strings.Join(allowed, ", "), value),
		Details: map[string]any{"flag": "--" + flag, "value": value, "allowed_values": allowed},
	}
}
