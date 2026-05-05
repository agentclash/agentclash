package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptEvalInitWritesScaffold(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".agentclash", "prompt-eval.yaml")

	err := executeCommand(t, []string{"prompt-eval", "init", target, "--name", "refund-bot"}, "http://unused")
	if err != nil {
		t.Fatalf("prompt-eval init error: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected scaffold at %s: %v", target, err)
	}
	result := validatePromptEvalFile(target, 100)
	if !result.Valid {
		t.Fatalf("scaffold should validate, errors=%v", result.Errors)
	}
	if result.ModelCount != 1 || result.TestCount != 1 || result.CaseCount != 1 {
		t.Fatalf("counts = models %d tests %d cases %d, want 1/1/1", result.ModelCount, result.TestCount, result.CaseCount)
	}
}

func TestPromptEvalInitRefusesExistingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "prompt-eval.yaml")
	if err := os.WriteFile(target, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	err := executeCommand(t, []string{"prompt-eval", "init", target}, "http://unused")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing-file error, got %v", err)
	}
	err = executeCommand(t, []string{"prompt-eval", "init", target, "--force"}, "http://unused")
	if err != nil {
		t.Fatalf("prompt-eval init --force error: %v", err)
	}
}

func TestPromptEvalValidateAcceptsValidConfigWithWarnings(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	result := validatePromptEvalFile(path, 100)
	if !result.Valid {
		t.Fatalf("valid config rejected: %v", result.Errors)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("warnings = %v, want provider_account default and single-test warnings", result.Warnings)
	}
	if len(result.AssertionSignatures) != 1 {
		t.Fatalf("assertion signatures = %v, want one", result.AssertionSignatures)
	}
}

func TestPromptEvalValidateRejectsLocalSemanticErrors(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr string
		max     int
	}{
		{
			name:    "duplicate test keys",
			body:    strings.Replace(validPromptEvalYAML(), "tests:\n  - key: greeting", "tests:\n  - key: greeting\n    vars: {input: hi}\n    assert: [{type: contains, value: hi}]\n  - key: greeting", 1),
			max:     100,
			wantErr: "duplicate test key",
		},
		{name: "missing variable", body: strings.Replace(validPromptEvalYAML(), "input: Say hello in French", "other: value", 1), max: 100, wantErr: "missing template variable"},
		{name: "unsupported template syntax", body: strings.Replace(validPromptEvalYAML(), "Reply to: {{input}}", "{% if input %}{{input}}{% endif %}", 1), max: 100, wantErr: "unsupported template control syntax"},
		{name: "unknown assertion", body: strings.Replace(validPromptEvalYAML(), "type: contains", "type: icontains", 1), max: 100, wantErr: "unsupported"},
		{name: "invalid regex", body: strings.Replace(validPromptEvalYAML(), "type: contains\n        value: Bonjour", "type: regex\n        value: \"(\"", 1), max: 100, wantErr: "invalid RE2 regex"},
		{name: "empty regex", body: strings.Replace(validPromptEvalYAML(), "type: contains\n        value: Bonjour", "type: regex\n        value: \"\"", 1), max: 100, wantErr: "regex_match requires a non-empty pattern"},
		{name: "empty json schema", body: strings.Replace(validPromptEvalYAML(), "type: contains\n        value: Bonjour", "type: json_schema\n        value: \"\"", 1), max: 100, wantErr: "json_schema requires"},
		{name: "no tests", body: strings.Replace(validPromptEvalYAML(), "tests:\n  - key: greeting\n    vars:\n      input: Say hello in French\n    expect:\n      output: Bonjour\n    assert:\n      - type: contains\n        value: Bonjour\n        metric: correctness", "tests: []", 1), max: 100, wantErr: "at least one test"},
		{name: "missing model selector", body: strings.Replace(validPromptEvalYAML(), "  - alias: gpt-5.5\n    provider_account: default", "  - provider_account: default", 1), max: 100, wantErr: "alias or model_alias_id"},
		{name: "max cases", body: validPromptEvalYAML(), max: 0, wantErr: "--max-cases must be greater than 0"},
		{name: "unknown yaml key", body: strings.Replace(validPromptEvalYAML(), "models:", "modls:", 1), max: 100, wantErr: "field modls not found"},
		{name: "bad assertion threshold", body: strings.Replace(validPromptEvalYAML(), "assertion_pass_rate: 0.9", "assertion_pass_rate: 1.2", 1), max: 100, wantErr: "thresholds.assertion_pass_rate must be between 0 and 1"},
		{name: "bad dimension threshold", body: strings.Replace(validPromptEvalYAML(), "correctness: 0.9", "correctness: -0.1", 1), max: 100, wantErr: "thresholds.dimensions.correctness must be between 0 and 1"},
		{name: "dotted template variable", body: strings.Replace(validPromptEvalYAML(), "{{input}}", "{{user.name}}", 1), max: 100, wantErr: "unsupported; use simple {{var}}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writePromptEvalFixture(t, tc.body)
			result := validatePromptEvalFile(path, tc.max)
			if result.Valid {
				t.Fatalf("expected invalid result")
			}
			if !containsPromptEvalMessage(result.Errors, tc.wantErr) {
				t.Fatalf("errors = %v, want substring %q", result.Errors, tc.wantErr)
			}
		})
	}
}

func TestPromptEvalValidateRejectsCaseCountOverMax(t *testing.T) {
	body := strings.Replace(validPromptEvalYAML(), "  - alias: gpt-5.5\n    provider_account: default", "  - alias: gpt-5.5\n    provider_account: default\n  - alias: gpt-5.5-mini\n    provider_account: default", 1)
	path := writePromptEvalFixture(t, body)
	result := validatePromptEvalFile(path, 1)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "case count 2 exceeds --max-cases 1") {
		t.Fatalf("max-cases errors = %v", result.Errors)
	}
	if result.ExitCode != promptEvalExitInvalid {
		t.Fatalf("exit code = %d, want %d", result.ExitCode, promptEvalExitInvalid)
	}
	if result.CaseCount != 2 {
		t.Fatalf("case count = %d, want 2", result.CaseCount)
	}
}

func TestPromptEvalValidateJSONEnvelope(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	stdout := captureStdout(t)
	err := executeCommand(t, []string{"prompt-eval", "validate", path, "--json"}, "http://unused")
	out := stdout.finish()
	if err != nil {
		t.Fatalf("prompt-eval validate --json error: %v", err)
	}
	var result promptEvalValidationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode json output: %v\n%s", err, out)
	}
	if result.SchemaVersion != 1 || !result.Valid || result.CaseCount != 1 || result.ExitCode != 0 {
		t.Fatalf("unexpected envelope: %+v", result)
	}
	if len(result.AssertionSignatures) != 1 {
		t.Fatalf("assertion signatures = %v, want one", result.AssertionSignatures)
	}
}

func TestPromptEvalValidateJSONEnvelopeForInvalidConfig(t *testing.T) {
	path := writePromptEvalFixture(t, strings.Replace(validPromptEvalYAML(), "type: contains", "type: nope", 1))
	stdout := captureStdout(t)
	err := executeCommand(t, []string{"prompt-eval", "validate", path, "--json"}, "http://unused")
	out := stdout.finish()
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitInvalid {
		t.Fatalf("expected ExitCodeError{%d}, got %T %v", promptEvalExitInvalid, err, err)
	}
	var result promptEvalValidationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode json output: %v\n%s", err, out)
	}
	if result.Valid || result.ExitCode != promptEvalExitInvalid || len(result.Errors) == 0 {
		t.Fatalf("unexpected invalid envelope: %+v", result)
	}
}

func TestPromptEvalValidateJSONEnvelopeForMissingFile(t *testing.T) {
	stdout := captureStdout(t)
	err := executeCommand(t, []string{"prompt-eval", "validate", filepath.Join(t.TempDir(), "missing.yaml"), "--json"}, "http://unused")
	out := stdout.finish()
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitInvalid {
		t.Fatalf("expected ExitCodeError{%d}, got %T %v", promptEvalExitInvalid, err, err)
	}
	var result promptEvalValidationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode json output: %v\n%s", err, out)
	}
	if result.AssertionSignatures == nil {
		t.Fatalf("assertion_signatures should be an empty array, got nil")
	}
	if result.MaxCases != 100 || result.ExitCode != promptEvalExitInvalid {
		t.Fatalf("unexpected missing-file envelope: %+v", result)
	}
}

func TestPromptEvalValidateRemoteRequiresWorkspace(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	err := executeCommand(t, []string{"prompt-eval", "validate", path, "--remote"}, "http://unused")
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitInvalid {
		t.Fatalf("expected ExitCodeError{%d}, got %T %v", promptEvalExitInvalid, err, err)
	}
}

func TestPromptEvalValidateRemoteResolvesAliasesAndProviderAccounts(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	srv := promptEvalRemoteFakeAPI(t, promptEvalRemoteFakeOptions{})
	defer srv.Close()

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"prompt-eval", "validate", path, "--remote", "-w", "ws-1", "--json"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("remote validate error: %v\n%s", err, out)
	}
	var result promptEvalValidationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode remote json: %v\n%s", err, out)
	}
	if result.Remote == nil || len(result.Remote.Models) != 1 {
		t.Fatalf("remote models missing: %+v", result.Remote)
	}
	if got := result.Remote.Models[0].ProviderAccountID; got != "pa-1" {
		t.Fatalf("provider_account_id = %q, want pa-1", got)
	}
}

func TestPromptEvalValidateRemoteRejectsUnknownOrAmbiguousModelAlias(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options promptEvalRemoteFakeOptions
		want    string
	}{
		{name: "unknown", options: promptEvalRemoteFakeOptions{ModelAliases: []map[string]any{}}, want: "model alias \"gpt-5.5\" was not found"},
		{name: "ambiguous", options: promptEvalRemoteFakeOptions{ModelAliases: []map[string]any{
			{"id": "alias-1", "alias_key": "gpt-5.5", "provider_key": "openai", "provider_account_id": "pa-1"},
			{"id": "alias-2", "alias_key": "gpt-5.5", "provider_key": "openai", "provider_account_id": "pa-1"},
		}}, want: "matched 2 workspace aliases"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writePromptEvalFixture(t, validPromptEvalYAML())
			srv := promptEvalRemoteFakeAPI(t, tc.options)
			defer srv.Close()
			result := runPromptEvalRemoteValidateJSON(t, path, srv.URL, false)
			if result.Valid || !containsPromptEvalMessage(result.Errors, tc.want) {
				t.Fatalf("errors = %v, want %q", result.Errors, tc.want)
			}
		})
	}
}

func TestPromptEvalValidateRemoteRejectsProviderAmbiguity(t *testing.T) {
	body := strings.Replace(validPromptEvalYAML(), "provider_account: default", "provider_account: openai", 1)
	path := writePromptEvalFixture(t, body)
	srv := promptEvalRemoteFakeAPI(t, promptEvalRemoteFakeOptions{ProviderAccounts: []map[string]any{
		{"id": "pa-1", "provider_key": "openai", "status": "active"},
		{"id": "pa-2", "provider_key": "openai", "status": "active"},
	}})
	defer srv.Close()

	result := runPromptEvalRemoteValidateJSON(t, path, srv.URL, false)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "matched 2 provider accounts") {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestPromptEvalValidateRemoteRejectsDefaultProviderInCI(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	srv := promptEvalRemoteFakeAPI(t, promptEvalRemoteFakeOptions{})
	defer srv.Close()

	result := runPromptEvalRemoteValidateJSON(t, path, srv.URL, true)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "provider_account: default is not allowed with --ci") {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestPromptEvalValidateRemoteDetectsDuplicatePlaygrounds(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	srv := promptEvalRemoteFakeAPI(t, promptEvalRemoteFakeOptions{Playgrounds: []map[string]any{
		{"id": "pg-1", "name": "Prompt Eval: refund-bot-v1"},
		{"id": "pg-2", "name": "Prompt Eval: refund-bot-v1"},
	}})
	defer srv.Close()

	result := runPromptEvalRemoteValidateJSON(t, path, srv.URL, false)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "multiple playgrounds named") {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestPromptEvalValidateRemoteReportsDryRunCounts(t *testing.T) {
	body := strings.Replace(validPromptEvalYAML(), "tests:\n  - key: greeting", "tests:\n  - key: salutation\n    vars:\n      input: Say hello in French\n    expect:\n      output: Bonjour\n    assert:\n      - type: contains\n        value: Salut\n        metric: correctness\n  - key: greeting", 1)
	body = strings.Replace(body, "thresholds:", "  - key: farewell\n    vars:\n      input: Say goodbye in French\n    expect:\n      output: Au revoir\n    assert:\n      - type: contains\n        value: Au revoir\n        metric: correctness\nthresholds:", 1)
	path := writePromptEvalFixture(t, body)
	srv := promptEvalRemoteFakeAPI(t, promptEvalRemoteFakeOptions{
		Playgrounds: []map[string]any{{"id": "pg-1", "name": "Prompt Eval: refund-bot-v1"}},
		TestCases: []map[string]any{
			{
				"id":        "tc-1",
				"case_key":  "greeting",
				"variables": map[string]any{"input": "Say hello in French"},
				"expectations": map[string]any{
					"output": "Bonjour",
					"prompt_eval_assertions": []any{
						map[string]any{"type": "contains", "expected": "Bonjour", "metric": "correctness"},
					},
				},
			},
			{
				"id":        "tc-3",
				"case_key":  "farewell",
				"variables": map[string]any{"input": "Say goodbye in French"},
				"expectations": map[string]any{
					"output": "Au revoir",
					"prompt_eval_assertions": []any{
						map[string]any{"type": "contains", "expected": "Bonjour", "metric": "correctness"},
					},
				},
			},
			{
				"id":           "tc-2",
				"case_key":     "orphan",
				"variables":    map[string]any{"input": "old"},
				"expectations": map[string]any{},
			},
		},
	})
	defer srv.Close()

	result := runPromptEvalRemoteValidateJSON(t, path, srv.URL, false)
	if !result.Valid {
		t.Fatalf("remote validate errors = %v", result.Errors)
	}
	if got := result.Remote.DryRun.TestsCreate; got != 1 {
		t.Fatalf("tests_create = %d, want 1", got)
	}
	if got := result.Remote.DryRun.TestsNoop; got != 1 {
		t.Fatalf("tests_noop = %d, want 1", got)
	}
	if got := result.Remote.DryRun.TestsUpdate; got != 1 {
		t.Fatalf("tests_update = %d, want 1", got)
	}
	if got := result.Remote.DryRun.TestsOrphan; got != 1 {
		t.Fatalf("tests_orphan = %d, want 1", got)
	}
}

func TestPromptEvalValidateRemoteIsReadOnly(t *testing.T) {
	var writeCalled bool
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	srv := promptEvalRemoteFakeAPI(t, promptEvalRemoteFakeOptions{OnWrite: func() { writeCalled = true }})
	defer srv.Close()

	result := runPromptEvalRemoteValidateJSON(t, path, srv.URL, false)
	if !result.Valid {
		t.Fatalf("remote validate errors = %v", result.Errors)
	}
	if writeCalled {
		t.Fatal("remote validate performed a write request")
	}
}

func TestChallengePackPromptEvalTemplateMentionsPromptEvalInit(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "pack.yaml")
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, []string{"challenge-pack", "init", target, "--template", "prompt_eval"}, "http://unused", false)
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("challenge-pack init error: %v", err)
	}
	if !strings.Contains(errOut, "prompt-eval init") {
		t.Fatalf("stderr missing prompt-eval init pointer:\n%s", errOut)
	}
}

func writePromptEvalFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prompt-eval.yaml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func containsPromptEvalMessage(messages []string, want string) bool {
	for _, msg := range messages {
		if strings.Contains(msg, want) {
			return true
		}
	}
	return false
}

func validPromptEvalYAML() string {
	return `schemaVersion: 1
name: refund-bot-v1
prompt:
  template: |
    You are a support assistant.
    Reply to: {{input}}
models:
  - alias: gpt-5.5
    provider_account: default
tests:
  - key: greeting
    vars:
      input: Say hello in French
    expect:
      output: Bonjour
    assert:
      - type: contains
        value: Bonjour
        metric: correctness
thresholds:
  assertion_pass_rate: 0.9
  dimensions:
    correctness: 0.9
`
}

type promptEvalRemoteFakeOptions struct {
	ModelAliases     []map[string]any
	ProviderAccounts []map[string]any
	Playgrounds      []map[string]any
	TestCases        []map[string]any
	OnWrite          func()
}

func promptEvalRemoteFakeAPI(t *testing.T, options promptEvalRemoteFakeOptions) *httptest.Server {
	t.Helper()
	modelAliases := options.ModelAliases
	if modelAliases == nil {
		modelAliases = []map[string]any{{"id": "alias-1", "alias_key": "gpt-5.5", "provider_key": "openai", "provider_account_id": "pa-1"}}
	}
	providerAccounts := options.ProviderAccounts
	if providerAccounts == nil {
		providerAccounts = []map[string]any{{"id": "pa-1", "provider_key": "openai", "name": "OpenAI", "status": "active"}}
	}
	playgrounds := options.Playgrounds
	if playgrounds == nil {
		playgrounds = []map[string]any{}
	}
	testCases := options.TestCases
	if testCases == nil {
		testCases = []map[string]any{}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
			if options.OnWrite != nil {
				options.OnWrite()
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "unexpected write request"}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/workspaces/ws-1/model-aliases":
			json.NewEncoder(w).Encode(map[string]any{"items": modelAliases})
		case "/v1/workspaces/ws-1/provider-accounts":
			json.NewEncoder(w).Encode(map[string]any{"items": providerAccounts})
		case "/v1/workspaces/ws-1/playgrounds":
			json.NewEncoder(w).Encode(map[string]any{"items": playgrounds})
		case "/v1/playgrounds/pg-1/test-cases":
			json.NewEncoder(w).Encode(map[string]any{"items": testCases})
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "not found"}})
		}
	}))
}

func runPromptEvalRemoteValidateJSON(t *testing.T, path, apiURL string, ciMode bool) promptEvalValidationResult {
	t.Helper()
	args := []string{"prompt-eval", "validate", path, "--remote", "-w", "ws-1", "--json"}
	if ciMode {
		args = append(args, "--ci")
	}
	stdout := captureStdout(t)
	err := executeCommand(t, args, apiURL)
	out := stdout.finish()
	var result promptEvalValidationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode remote json: %v\n%s", err, out)
	}
	if result.Valid && err != nil {
		t.Fatalf("remote validate returned error for valid result: %v\n%s", err, out)
	}
	return result
}
