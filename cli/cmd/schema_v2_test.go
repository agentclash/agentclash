package cmd

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
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

	for _, se := range doc.StatusEnums {
		terminal := map[string]bool{}
		for _, s := range se.Terminal {
			if !isTerminalRunStatus(s) {
				t.Errorf("status_enums[%s] lists %q terminal, but isTerminalRunStatus disagrees", se.Resource, s)
			}
			terminal[s] = true
		}
		for _, s := range se.Values {
			if isTerminalRunStatus(s) && !terminal[s] {
				t.Errorf("status_enums[%s]: %q is terminal in code but not in the registry", se.Resource, s)
			}
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
