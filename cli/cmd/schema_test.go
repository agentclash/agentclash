package cmd

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool(
	"update",
	false,
	"regenerate the schema golden snapshot (testdata/schema_snapshot.json)",
)

func findCommandByPath(cmds []commandSchema, path string) *commandSchema {
	for i := range cmds {
		if cmds[i].Path == path {
			return &cmds[i]
		}
		if found := findCommandByPath(cmds[i].Subcommands, path); found != nil {
			return found
		}
	}
	return nil
}

// TestSchemaJSONMatchesGoldenSnapshot is the Cobra-grep contract test: it walks
// the live command tree and fails if it drifts from the committed golden file.
// Adding/renaming/removing a command or flag without updating the golden fails
// here. Regenerate with: go test ./cmd -run TestSchemaJSONMatchesGoldenSnapshot -update
func TestSchemaJSONMatchesGoldenSnapshot(t *testing.T) {
	doc := buildCLISchema(rootCmd, "test")
	got, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	got = append(got, '\n')

	path := filepath.Join("testdata", "schema_snapshot.json")

	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (regenerate with `go test ./cmd -run TestSchemaJSONMatchesGoldenSnapshot -update`): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("CLI command tree drifted from testdata/schema_snapshot.json.\n" +
			"If this change is intentional, regenerate and review:\n" +
			"  go test ./cmd -run TestSchemaJSONMatchesGoldenSnapshot -update")
	}
}

func TestSchemaCommandEmitsParseableJSON(t *testing.T) {
	cap := captureStdout(t)
	err := executeCommand(t, []string{"schema", "--json"}, "http://unused")
	out := cap.finish()
	if err != nil {
		t.Fatalf("schema --json: %v", err)
	}

	var doc cliSchema
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("schema output is not valid JSON: %v\n%s", err, out)
	}
	if doc.SchemaVersion != schemaDocVersion {
		t.Errorf("schema_version = %d, want %d", doc.SchemaVersion, schemaDocVersion)
	}
	if len(doc.Commands) == 0 {
		t.Error("schema has no commands")
	}
	if len(doc.GlobalFlags) == 0 {
		t.Error("schema has no global flags")
	}
	if len(doc.ExitCodes) == 0 {
		t.Error("schema has no exit codes")
	}
	// Spot-check a known nested command survives the walk.
	if findCommandByPath(doc.Commands, "agentclash compare gate") == nil {
		t.Error("expected 'agentclash compare gate' in the schema")
	}
}

func TestSchemaUnknownSubcommandFails(t *testing.T) {
	err := executeCommand(t, []string{"schema", "bogus"}, "http://unused")
	if err == nil {
		t.Fatal("expected an error for `schema bogus` (Args: NoArgs), got nil")
	}
}

// TestSchemaExitCodesMatchConsts keeps the documented exit-code registry in
// lockstep with the real consts the commands return.
func TestSchemaExitCodesMatchConsts(t *testing.T) {
	want := map[string]int{
		"compare_gate_fail":                  gateExitFail,
		"compare_gate_warn":                  gateExitWarn,
		"compare_gate_insufficient_evidence": gateExitInsufficientEvidence,
		"prompt_eval_gate_failed":            promptEvalExitGate,
		"prompt_eval_execution_error":        promptEvalExitExecution,
		"prompt_eval_invalid_input":          promptEvalExitInvalid,
		"ci_run_invalid_manifest":            ciRunExitInvalidManifest,
		"ci_run_api_error":                   ciRunExitAPI,
		"ci_run_timeout":                     ciRunExitTimeout,
		"ci_run_run_failed":                  ciRunExitRunFailed,
		"validation_error":                   exitValidationError,
		"not_found":                          exitNotFound,
		"retryable_failure":                  exitRetryableFailure,
		"auth_denied":                        exitAuthDenied,
	}

	got := make(map[string]int, len(documentedExitCodes))
	for _, ec := range documentedExitCodes {
		got[ec.Name] = ec.Code
	}
	for name, code := range want {
		actual, ok := got[name]
		if !ok {
			t.Errorf("documentedExitCodes missing %q", name)
			continue
		}
		if actual != code {
			t.Errorf("exit code %q: registry has %d, const is %d", name, actual, code)
		}
	}
}

// TestNoCommandExitCodeCollidesWithGlobalBand enforces the precondition the
// "exit 75 ⟺ retryable:true" invariant silently relies on: no command-scoped
// exit code may reuse a value in the global failure-class band. If a future
// command returned ExitCodeError{Code: 75}, exitCodeForError would emit 75
// (command codes win) while classifyStructuredError reported retryable:false —
// breaking the invariant. The bands are sysexits values (64/66/75/77),
// deliberately above the 1–31 command range; this test makes "deliberately"
// a guarantee instead of a convention.
func TestNoCommandExitCodeCollidesWithGlobalBand(t *testing.T) {
	band := map[int]string{
		exitValidationError:  "validation_error",
		exitNotFound:         "not_found",
		exitRetryableFailure: "retryable_failure",
		exitAuthDenied:       "auth_denied",
	}
	for _, ec := range documentedExitCodes {
		if len(ec.Commands) == 0 {
			continue // global band entry itself
		}
		if name, clash := band[ec.Code]; clash {
			t.Errorf("command-scoped code %q (%d, used by %v) collides with global band %q; "+
				"a command returning this code would break the exit-75-iff-retryable invariant",
				ec.Name, ec.Code, ec.Commands, name)
		}
	}
}
