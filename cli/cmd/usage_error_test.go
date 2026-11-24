package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/config"
)

func TestCobraUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		message string
	}{
		{"required flag", []string{"share", "create", "--resource-type", "run_scorecard"}, `required flag(s) "resource-id" not set`},
		{"argument count", []string{"run", "get"}, "accepts 1 arg(s), received 0"},
		{"unknown flag", []string{"version", "--not-a-flag"}, "unknown flag: --not-a-flag"},
		{"unknown root subcommand", []string{"not-a-command"}, `unknown command "not-a-command"`},
		{"unknown nested subcommand", []string{"connector", "list", "not-a-command"}, `unknown command "not-a-command"`},
		{"unparseable value", []string{"evalset", "finding", "list", "eval-set-id", "--limit", "nope"}, `invalid argument "nope" for "--limit" flag`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("table", func(t *testing.T) {
				assertUsageError(t, tc.args, tc.message, false)
			})
			t.Run("json", func(t *testing.T) {
				args := append(append([]string{}, tc.args...), "--json")
				assertUsageError(t, args, tc.message, true)
			})
		})
	}
}

func TestCobraUsageErrorOutputFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		json bool
	}{
		{"before failure", []string{"--json", "version", "--bogus"}, true},
		{"explicit true", []string{"version", "--bogus", "--json=true"}, true},
		{"explicit false", []string{"version", "--bogus", "--json=false"}, false},
		{"last json flag wins", []string{"--json", "version", "--bogus", "--json=false"}, false},
		{"long output", []string{"version", "--bogus", "--output", "json"}, true},
		{"long output equals", []string{"version", "--bogus", "--output=json"}, true},
		{"short output", []string{"version", "--bogus", "-o", "json"}, true},
		{"short output cluster", []string{"version", "--bogus", "-qoyaml"}, true},
		{"json overrides table", []string{"version", "--bogus", "--json", "--output=table"}, true},
		{"json as flag value", []string{"version", "--bogus", "--jq", "--json"}, false},
		{"json after delimiter", []string{"version", "--bogus", "--", "--json"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertUsageError(t, tc.args, "unknown flag: --bogus", tc.json)
		})
	}
}

func TestCobraUsageErrorAfterMalformedFlag(t *testing.T) {
	assertUsageError(t, []string{"version", "---bogus", "--json"}, "bad flag syntax", true)
}

func TestCobraUsageErrorInternalCompletion(t *testing.T) {
	assertUsageError(t, []string{"__complete"}, "requires at least 1 arg(s)", false)
}

func TestCobraUsageErrorSavedOutput(t *testing.T) {
	isolateCLIEnv(t)
	if err := config.Save(config.UserConfig{Output: "json"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		json bool
	}{
		{"saved json", []string{"version", "--bogus"}, true},
		{"explicit table", []string{"version", "--bogus", "--output=table"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkUsageError(t, tc.args, "unknown flag: --bogus", tc.json)
		})
	}
}

func TestCobraHelpSkipsUsageValidation(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, nil)
	stdout := captureStdout(t)
	err := executeCommand(t, []string{"share", "create", "--help", "--json"}, srv.URL)
	out := stdout.finish()
	assertExitCode(t, err, 0)
	if !strings.Contains(out, "Usage:") || !strings.Contains(out, "agentclash share create") {
		t.Fatalf("expected share create help, got %q", out)
	}
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func assertUsageError(t *testing.T, args []string, message string, wantJSON bool) {
	t.Helper()
	isolateCLIEnv(t)
	checkUsageError(t, args, message, wantJSON)
}

func checkUsageError(t *testing.T, args []string, message string, wantJSON bool) {
	t.Helper()
	srv, count := countingFakeAPI(t, nil)
	err := executeCommand(t, args, srv.URL)
	if err == nil || !strings.Contains(err.Error(), message) {
		t.Fatalf("error = %v, want message containing %q", err, message)
	}
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
	var stderr bytes.Buffer
	code, rendered := RenderError(err, &stderr)
	if code != exitValidationError {
		t.Errorf("exit code = %d, want %d", code, exitValidationError)
	}
	if rendered != wantJSON {
		t.Fatalf("rendered JSON = %v, want %v (stderr: %q)", rendered, wantJSON, stderr.String())
	}
	assertCLIErrorCode(t, err, "invalid_argument")
	if !wantJSON {
		return
	}
	got := decodeStructuredError(t, stderr.String()).Error
	if got.Code != "invalid_argument" || got.Message != err.Error() || got.Retryable || got.Details == nil {
		t.Errorf("invalid usage error envelope: %+v", got)
	}
}
