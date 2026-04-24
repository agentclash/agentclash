package cmd

import (
	"testing"

	"github.com/agentclash/agentclash/cli/internal/output"
)

// buildRC constructs a minimal RunContext with the requested output format
// so IsNonInteractive can be exercised without the full PersistentPreRunE
// pipeline.
func buildRC(format string) *RunContext {
	return &RunContext{
		Output: output.NewFormatter(format, false, true),
	}
}

func TestIsNonInteractiveNilReceiverIsNonInteractive(t *testing.T) {
	var rc *RunContext
	if !rc.IsNonInteractive() {
		t.Fatal("nil RunContext must report non-interactive")
	}
}

func TestIsNonInteractiveAllSignalsFireIndependently(t *testing.T) {
	// Force TTY attached so only the signal under test drives the result.
	orig := ttyAttached
	ttyAttached = func() bool { return true }
	t.Cleanup(func() { ttyAttached = orig })

	// Clear all env-level signals at the start.
	t.Setenv("AGENTCLASH_NON_INTERACTIVE", "")
	t.Setenv("CI", "")
	flagNonInteractive = false
	t.Cleanup(func() { flagNonInteractive = false })

	t.Run("baseline interactive when TTY and no signals", func(t *testing.T) {
		rc := buildRC("table")
		if rc.IsNonInteractive() {
			t.Fatal("expected interactive when TTY + no signals set")
		}
	})

	t.Run("--non-interactive flag", func(t *testing.T) {
		flagNonInteractive = true
		defer func() { flagNonInteractive = false }()
		rc := buildRC("table")
		if !rc.IsNonInteractive() {
			t.Fatal("--non-interactive flag should force non-interactive")
		}
	})

	t.Run("structured output (json)", func(t *testing.T) {
		rc := buildRC("json")
		if !rc.IsNonInteractive() {
			t.Fatal("json output format should force non-interactive")
		}
	})

	t.Run("structured output (yaml)", func(t *testing.T) {
		rc := buildRC("yaml")
		if !rc.IsNonInteractive() {
			t.Fatal("yaml output format should force non-interactive")
		}
	})

	t.Run("AGENTCLASH_NON_INTERACTIVE=1", func(t *testing.T) {
		t.Setenv("AGENTCLASH_NON_INTERACTIVE", "1")
		rc := buildRC("table")
		if !rc.IsNonInteractive() {
			t.Fatal("AGENTCLASH_NON_INTERACTIVE=1 should force non-interactive")
		}
	})

	t.Run("AGENTCLASH_NON_INTERACTIVE only fires on literal 1", func(t *testing.T) {
		for _, v := range []string{"", "0", "true", "yes"} {
			t.Setenv("AGENTCLASH_NON_INTERACTIVE", v)
			rc := buildRC("table")
			if rc.IsNonInteractive() {
				t.Fatalf("AGENTCLASH_NON_INTERACTIVE=%q should not trigger", v)
			}
		}
	})

	t.Run("CI=true", func(t *testing.T) {
		t.Setenv("CI", "true")
		rc := buildRC("table")
		if !rc.IsNonInteractive() {
			t.Fatal("CI=true should force non-interactive")
		}
	})

	t.Run("CI only fires on literal true", func(t *testing.T) {
		for _, v := range []string{"", "false", "1", "yes"} {
			t.Setenv("CI", v)
			rc := buildRC("table")
			if rc.IsNonInteractive() {
				t.Fatalf("CI=%q should not trigger — only literal 'true'", v)
			}
		}
	})
}

func TestIsNonInteractiveNoTTYIsNonInteractive(t *testing.T) {
	orig := ttyAttached
	ttyAttached = func() bool { return false }
	t.Cleanup(func() { ttyAttached = orig })

	t.Setenv("AGENTCLASH_NON_INTERACTIVE", "")
	t.Setenv("CI", "")
	flagNonInteractive = false

	rc := buildRC("table")
	if !rc.IsNonInteractive() {
		t.Fatal("no TTY should force non-interactive regardless of other flags")
	}
}
