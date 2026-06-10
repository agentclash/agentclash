package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/securitystress"
)

func TestRenderSummaryWritesToProvidedWriter(t *testing.T) {
	r := &securitystress.Result{
		Provider:       "openai",
		Model:          "gpt-4o-mini",
		Iterations:     10,
		LeakedIters:    2,
		Posture:        0.8,
		TotalIncidents: 3,
		BySeverity:     map[string]int{"high": 2, "low": 1},
		ByStrategy: map[string]securitystress.StrategyOutcome{
			"roleplay": {Strategy: "roleplay", Refused: 8, Accepted: 2},
		},
		Errors: []string{"boom"},
	}

	var buf bytes.Buffer
	renderSummary(&buf, r)
	out := buf.String()

	for _, want := range []string{
		"=== openai / gpt-4o-mini ===",
		"iterations         : 10",
		"posture            : 0.80",
		"total incidents    : 3",
		"high=2",
		"refusal by strategy:",
		"roleplay",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("renderSummary output missing %q\n%s", want, out)
		}
	}
}

// stress-run fires real LLM-provider calls, so the JSON success path is not
// unit-testable in -short mode. The failure path (and its structured-error
// rendering under --json) is, and is exactly what an agent driving the command
// relies on: a missing pack must surface as a parseable file_not_found
// envelope on stderr, not as human prose on stdout.
func TestSecurityStressRunMissingPackRendersStructuredError(t *testing.T) {
	err := executeCommand(t, []string{"security", "stress-run", "/no/such/pack.yaml", "--json"}, "http://unused")
	if err == nil {
		t.Fatal("expected an error for a missing pack file")
	}

	var stderr bytes.Buffer
	_, rendered := RenderError(err, &stderr)
	if !rendered {
		t.Fatal("expected structured error to render in --json mode")
	}

	envelope := decodeStructuredError(t, stderr.String())
	if envelope.Error.Code != "file_not_found" {
		t.Fatalf("code = %q, want file_not_found\n%s", envelope.Error.Code, stderr.String())
	}
}
