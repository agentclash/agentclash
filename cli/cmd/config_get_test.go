package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// WI-12 happy path: a set key in --json mode is a single {"key","value"} doc.
func TestConfigGetJSONSetKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := executeCommand(t, []string{"config", "set", "default_org", "org-42"}, "http://unused.invalid"); err != nil {
		t.Fatalf("config set: %v", err)
	}

	cap := captureStdout(t)
	err := executeCommand(t, []string{"config", "get", "default_org", "--json"}, "http://unused.invalid")
	out := cap.finish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc map[string]any
	if uerr := json.Unmarshal([]byte(out), &doc); uerr != nil {
		t.Fatalf("stdout is not a single JSON doc: %v\n%s", uerr, out)
	}
	if doc["key"] != "default_org" || doc["value"] != "org-42" {
		t.Fatalf("doc = %v, want key=default_org value=org-42", doc)
	}
}

// WI-12: an unset (but valid) key is data, not an error — value:null, exit 0.
func TestConfigGetJSONUnsetKeyIsNullNotError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cap := captureStdout(t)
	err := executeCommand(t, []string{"config", "get", "default_workspace", "--json"}, "http://unused.invalid")
	out := cap.finish()
	if err != nil {
		t.Fatalf("unset key must not error in --json mode; got %v", err)
	}
	var doc map[string]any
	if uerr := json.Unmarshal([]byte(out), &doc); uerr != nil {
		t.Fatalf("stdout is not a single JSON doc: %v\n%s", uerr, out)
	}
	value, present := doc["value"]
	if doc["key"] != "default_workspace" || !present || value != nil {
		t.Fatalf("doc = %v, want key=default_workspace value=null (present)", doc)
	}
}

// A typo'd key must not read as "valid but unset" — stable invalid_argument
// naming the valid keys, in both modes.
func TestConfigGetUnknownKeyFailsWithValidKeys(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for _, args := range [][]string{
		{"config", "get", "default_orgg", "--json"},
		{"config", "get", "default_orgg"},
	} {
		err := executeCommand(t, args, "http://unused.invalid")
		if err == nil {
			t.Fatalf("%v: expected an error for an unknown key, got nil", args)
		}
		var ce *cliError
		if !errors.As(err, &ce) || ce.Code != "invalid_argument" {
			t.Fatalf("%v: error = %v, want cliError invalid_argument", args, err)
		}
		if !strings.Contains(ce.Message, "default_org") {
			t.Fatalf("%v: message should name valid keys; got %q", args, ce.Message)
		}
	}
}

// Human mode is unchanged: set key prints the bare value; unset key errors.
func TestConfigGetHumanModeUnchanged(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := executeCommand(t, []string{"config", "set", "api_url", "https://api.example.dev"}, "http://unused.invalid"); err != nil {
		t.Fatalf("config set: %v", err)
	}

	cap := captureStdout(t)
	if err := executeCommand(t, []string{"config", "get", "api_url"}, "http://unused.invalid"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(cap.finish()); got != "https://api.example.dev" {
		t.Fatalf("stdout = %q, want the bare value", got)
	}

	if err := executeCommand(t, []string{"config", "get", "default_workspace"}, "http://unused.invalid"); err == nil {
		t.Fatal("human mode: unset key should still error (unchanged contract)")
	}
}
