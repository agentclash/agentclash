package cmd

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/agentclash/agentclash/runtime/domain"
)

func TestParseArgsFromUse(t *testing.T) {
	cases := []struct {
		use  string
		want []argSchema
	}{
		{"list", nil},
		{"get <runId>", []argSchema{{Name: "runId", Required: true}}},
		{"import <datasetId> <file>", []argSchema{{Name: "datasetId", Required: true}, {Name: "file", Required: true}}},
		{"logs [pod]", []argSchema{{Name: "pod", Required: false}}},
		{"add <files>...", []argSchema{{Name: "files", Required: true, Variadic: true}}},
		{"add [files...]", []argSchema{{Name: "files", Required: false, Variadic: true}}},
		{"schema [command ...]", []argSchema{{Name: "command", Required: false, Variadic: true}}},
	}
	for _, tc := range cases {
		if got := parseArgsFromUse(tc.use); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("parseArgsFromUse(%q) = %+v, want %+v", tc.use, got, tc.want)
		}
	}
}

// WI-3 acceptance: `run get` is invocable from the schema alone — the
// positional arg is visible and required.
func TestSchemaExposesPositionalArgs(t *testing.T) {
	doc := buildCLISchema(rootCmd, "test")
	runGet := findCommandByPath(doc.Commands, "agentclash run get")
	if runGet == nil {
		t.Fatal("agentclash run get missing from schema")
	}
	if len(runGet.Args) != 1 || !runGet.Args[0].Required {
		t.Fatalf("run get args = %+v, want one required positional", runGet.Args)
	}
}

// WI-3: --output exposes its closed value set; enum-less flags stay bare.
func TestSchemaExposesAllowedValues(t *testing.T) {
	doc := buildCLISchema(rootCmd, "test")

	var outputFlag *flagSchema
	for i := range doc.GlobalFlags {
		if doc.GlobalFlags[i].Name == "output" {
			outputFlag = &doc.GlobalFlags[i]
		}
	}
	if outputFlag == nil {
		t.Fatal("global --output missing from schema")
	}
	if !reflect.DeepEqual(outputFlag.AllowedValues, []string{"table", "json", "yaml"}) {
		t.Fatalf("--output allowed_values = %v, want [table json yaml]", outputFlag.AllowedValues)
	}

	runCreate := findCommandByPath(doc.Commands, "agentclash run create")
	if runCreate == nil {
		t.Fatal("agentclash run create missing from schema")
	}
	for _, f := range runCreate.Flags {
		if f.Name == "scope" && !reflect.DeepEqual(f.AllowedValues, []string{"full", "suite_only"}) {
			t.Fatalf("run create --scope allowed_values = %v, want [full suite_only]", f.AllowedValues)
		}
		if f.Name == "name" && f.AllowedValues != nil {
			t.Fatalf("free-form flag --name must not claim allowed_values; got %v", f.AllowedValues)
		}
	}
}

// WI-3: the error-code and status-enum registries are published, and the
// status registry stays in lockstep with isTerminalRunStatus — the same
// helper every --follow loop uses.
func TestSchemaPublishesErrorAndStatusRegistries(t *testing.T) {
	doc := buildCLISchema(rootCmd, "test")
	if len(doc.ErrorCodes) == 0 || len(doc.StatusEnums) == 0 {
		t.Fatalf("error_codes/status_enums missing: %d/%d entries", len(doc.ErrorCodes), len(doc.StatusEnums))
	}

	codes := map[string]bool{}
	for _, ec := range doc.ErrorCodes {
		codes[ec.Code] = true
	}
	for _, want := range []string{"invalid_argument", "request_failed", "follow_timeout", "stream_reconnect_exhausted", "missing_workspace"} {
		if !codes[want] {
			t.Errorf("error_codes missing %q", want)
		}
	}

	// The registry is the server's lifecycle tables, not a copy.
	if !reflect.DeepEqual(doc.StatusEnums, statusEnumsFromLifecycles(domain.Lifecycles())) {
		t.Fatal("status_enums drifted from domain.Lifecycles()")
	}

	// Each follow loop stops on exactly the terminal set that the registry
	// publishes for the resource that it polls.
	followPredicates := map[string]func(string) bool{
		"run":          isTerminalRunStatus,
		"eval_session": evalSessionStatusTerminal,
		"eval_set":     evalsetStatusTerminal,
	}
	byResource := map[string]StatusEnum{}
	for _, se := range doc.StatusEnums {
		byResource[se.Resource] = se
	}
	for resource, isTerminal := range followPredicates {
		se, ok := byResource[resource]
		if !ok {
			t.Fatalf("status_enums has no %q entry", resource)
		}
		terminal := map[string]bool{}
		for _, s := range se.Terminal {
			terminal[s] = true
		}
		for _, s := range se.Values {
			if isTerminal(s) != terminal[s] {
				t.Errorf("status_enums[%s]: %q terminal in registry = %v, but the follow loop says %v", resource, s, terminal[s], isTerminal(s))
			}
		}
	}
}

// budget_exceeded is a terminal eval-set status. Before the CLI read the
// server lifecycle, `eval-set submit --follow` polled forever on it and
// exited 0.
func TestEvalsetBudgetExceededEndsFollowWithFailure(t *testing.T) {
	if !evalsetStatusTerminal("budget_exceeded") {
		t.Fatal("budget_exceeded must end an eval-set follow loop")
	}
	if !evalsetStatusTerminal("canceled") {
		t.Fatal("the single-l canceled spelling must stay terminal")
	}
	if evalsetStatusTerminal("aggregating") {
		t.Fatal("aggregating is not terminal")
	}
	err := evalsetTerminalExit("budget_exceeded")
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != evalsetExitBudgetExceeded {
		t.Fatalf("evalsetTerminalExit(budget_exceeded) = %v, want exit code %d", err, evalsetExitBudgetExceeded)
	}
	if err := evalsetTerminalExit("completed"); err != nil {
		t.Fatalf("evalsetTerminalExit(completed) = %v, want nil", err)
	}
}

func TestCIRunTerminalStatusUsesTheRunLifecycle(t *testing.T) {
	cases := []struct {
		status            string
		terminal, success bool
	}{
		{"completed", true, true},
		{"failed", true, false},
		{"cancelled", true, false},
		{" Canceled ", true, false},
		{"scoring", false, false},
		{"queued", false, false},
	}
	for _, tc := range cases {
		terminal, success := ciRunTerminalStatus(tc.status)
		if terminal != tc.terminal || success != tc.success {
			t.Errorf("ciRunTerminalStatus(%q) = (%v, %v), want (%v, %v)", tc.status, terminal, success, tc.terminal, tc.success)
		}
	}
}

// WI-14: `schema <command-path>` returns just that subtree; aliases resolve;
// unknown paths fail with a stable code.
func TestSchemaSubtreeLookup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cap := captureStdout(t)
	err := executeCommand(t, []string{"schema", "run", "get", "--json"}, "http://unused.invalid")
	out := cap.finish()
	if err != nil {
		t.Fatalf("schema run get: %v", err)
	}
	var sub commandSchema
	if uerr := json.Unmarshal([]byte(out), &sub); uerr != nil {
		t.Fatalf("subtree output is not a single JSON doc: %v\n%s", uerr, out)
	}
	if sub.Path != "agentclash run get" || len(sub.Args) != 1 {
		t.Fatalf("subtree = path %q args %+v, want agentclash run get with one arg", sub.Path, sub.Args)
	}

	// Alias resolution: `harness` aliases `agent-harness`.
	cap = captureStdout(t)
	if err := executeCommand(t, []string{"schema", "harness", "run", "--json"}, "http://unused.invalid"); err != nil {
		t.Fatalf("schema harness run (alias): %v", err)
	}
	if uerr := json.Unmarshal([]byte(cap.finish()), &sub); uerr != nil || sub.Path != "agentclash agent-harness run" {
		t.Fatalf("alias lookup = %q (err %v), want agentclash agent-harness run", sub.Path, uerr)
	}

	err = executeCommand(t, []string{"schema", "bogus", "--json"}, "http://unused.invalid")
	var ce *cliError
	if !errors.As(err, &ce) || ce.Code != "invalid_argument" {
		t.Fatalf("unknown path error = %v, want cliError invalid_argument", err)
	}
}

// The error_codes registry's `retryable` flags must match what the CLI
// actually emits — a published `retryable:true` that the runtime contradicts
// (retryable:false + exit 64) is the exact contract lie schema v2 exists to
// prevent. Codes the CLI synthesizes as a cliError are checked end-to-end.
func TestErrorCodeRegistryRetryableMatchesRuntime(t *testing.T) {
	reg := map[string]bool{}
	for _, ec := range documentedErrorCodes {
		reg[ec.Code] = ec.Retryable
	}

	// cliError-synthesized codes: build the error, classify it, compare.
	for _, code := range []string{"follow_timeout", "stream_reconnect_exhausted", "missing_workspace", "invalid_argument"} {
		want, documented := reg[code]
		if !documented {
			t.Errorf("error code %q is emitted but absent from the registry", code)
			continue
		}
		se := classifyStructuredError(&cliError{Code: code, Message: "x"})
		if se.Retryable != want {
			t.Errorf("code %q: registry retryable=%v, runtime envelope retryable=%v", code, want, se.Retryable)
		}
		// The exit-75 ⟺ retryable invariant must also hold for these.
		gotExit := exitCodeForError(&cliError{Code: code, Message: "x"})
		if (gotExit == exitRetryableFailure) != want {
			t.Errorf("code %q: exit %d vs registry retryable=%v breaks the exit-75-iff-retryable invariant", code, gotExit, want)
		}
	}

	// Transport failures genuinely are retryable, and the registry must say so.
	if !reg["request_failed"] {
		t.Error("request_failed must be marked retryable in the registry")
	}
}
