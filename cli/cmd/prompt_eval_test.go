package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestPromptEvalImportPromptfooBasicConversion(t *testing.T) {
	path := writePromptEvalFixture(t, `description: refund-import
prompts:
  - "Reply to {{input}}"
providers:
  - openai:gpt-5.5
tests:
  - description: greeting
    vars:
      input: Say hello
    assert:
      - type: contains
        value: hello
      - type: equals
        value: hello there
`)
	stdout := captureStdout(t)
	err := executeCommand(t, []string{"prompt-eval", "import-promptfoo", path, "--provider-account", "ci-openai"}, "http://unused")
	out := stdout.finish()
	if err != nil {
		t.Fatalf("import-promptfoo error: %v\n%s", err, out)
	}
	var cfg promptEvalConfig
	if err := yaml.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("decode imported yaml: %v\n%s", err, out)
	}
	if cfg.Name != "refund-import" || cfg.Prompt.Template != "Reply to {{input}}" {
		t.Fatalf("unexpected imported config: %+v", cfg)
	}
	if len(cfg.Models) != 1 || cfg.Models[0].Alias != "gpt-5.5" || cfg.Models[0].ProviderAccount != "ci-openai" {
		t.Fatalf("models = %+v", cfg.Models)
	}
	if len(cfg.Tests) != 1 || cfg.Tests[0].Key != "greeting" || len(cfg.Tests[0].Assert) != 2 {
		t.Fatalf("tests = %+v", cfg.Tests)
	}
	if result := validatePromptEvalConfigForTest(cfg); !result.Valid {
		t.Fatalf("imported config should validate: %v", result.Errors)
	}
}

func TestPromptEvalImportPromptfooWritesOutputWithForce(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptfooYAML())
	outPath := filepath.Join(t.TempDir(), "prompt-eval.yaml")
	err := executeCommand(t, []string{"prompt-eval", "import-promptfoo", path, "--out", outPath}, "http://unused")
	if err != nil {
		t.Fatalf("import-promptfoo --out error: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	err = executeCommand(t, []string{"prompt-eval", "import-promptfoo", path, "--out", outPath}, "http://unused")
	if err == nil || !strings.Contains(err.Error(), "exit code 5") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
	err = executeCommand(t, []string{"prompt-eval", "import-promptfoo", path, "--out", outPath, "--force"}, "http://unused")
	if err != nil {
		t.Fatalf("import-promptfoo --force error: %v", err)
	}
}

func TestPromptEvalImportPromptfooRejectsUnsupportedAssertions(t *testing.T) {
	path := writePromptEvalFixture(t, strings.Replace(validPromptfooYAML(), "type: contains", "type: llm-rubric", 1))
	result := runPromptfooImportJSON(t, path)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "llm-rubric") {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestPromptEvalImportPromptfooRejectsSchemalessIsJSON(t *testing.T) {
	path := writePromptEvalFixture(t, strings.Replace(validPromptfooYAML(), "type: contains\n        value: hello", "type: is-json", 1))
	result := runPromptfooImportJSON(t, path)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "is-json without a JSON schema") {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestPromptEvalImportPromptfooRejectsNunjucksControlFlow(t *testing.T) {
	path := writePromptEvalFixture(t, strings.Replace(validPromptfooYAML(), "Reply to {{input}}", "{% if input %}{{input}}{% endif %}", 1))
	result := runPromptfooImportJSON(t, path)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "Nunjucks control flow") {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestPromptEvalImportPromptfooRejectsFilePrompt(t *testing.T) {
	path := writePromptEvalFixture(t, strings.Replace(validPromptfooYAML(), `"Reply to {{input}}"`, `file://prompts/refund.txt`, 1))
	result := runPromptfooImportJSON(t, path)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "file:// prompts") {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestPromptEvalImportPromptfooRejectsPerTestUnsupportedFields(t *testing.T) {
	path := writePromptEvalFixture(t, strings.Replace(validPromptfooYAML(), "    vars:\n      input: hello", "    provider: openai:gpt-4\n    transform: output.toUpperCase()\n    vars:\n      input: hello", 1))
	result := runPromptfooImportJSON(t, path)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "unsupported promptfoo fields") {
		t.Fatalf("errors = %v", result.Errors)
	}
	if !containsPromptEvalMessage(result.Errors, "provider") || !containsPromptEvalMessage(result.Errors, "transform") {
		t.Fatalf("errors should name unsupported fields: %v", result.Errors)
	}
}

func TestPromptEvalImportPromptfooRejectsIncompatibleRegex(t *testing.T) {
	path := writePromptEvalFixture(t, strings.Replace(validPromptfooYAML(), "type: contains\n        value: hello", "type: regex\n        value: \"(?<=hello) world\"", 1))
	result := runPromptfooImportJSON(t, path)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "not RE2-compatible") {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestPromptEvalImportPromptfooIContainsLossyRules(t *testing.T) {
	alpha := writePromptEvalFixture(t, strings.Replace(validPromptfooYAML(), "type: contains", "type: icontains", 1))
	result := runPromptfooImportJSON(t, alpha)
	if result.Valid || !containsPromptEvalMessage(result.Errors, "icontains") {
		t.Fatalf("expected icontains refusal, got %+v", result)
	}
	result = runPromptfooImportJSONWithArgs(t, []string{"prompt-eval", "import-promptfoo", alpha, "--lossy", "--json"})
	if result.Valid || !containsPromptEvalMessage(result.Errors, "icontains") {
		t.Fatalf("lettered icontains should still be refused under --lossy, got %+v", result)
	}
	neutral := writePromptEvalFixture(t, strings.Replace(validPromptfooYAML(), "type: contains\n        value: hello", "type: icontains\n        value: \"123\"", 1))
	result = runPromptfooImportJSONWithArgs(t, []string{"prompt-eval", "import-promptfoo", neutral, "--lossy", "--json"})
	if !result.Valid {
		t.Fatalf("case-neutral icontains should import under --lossy: %v", result.Errors)
	}
}

func TestPromptEvalValidateRemoteRequiresWorkspace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_WORKSPACE", "")
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	stdout := captureStdout(t)
	err := executeCommand(t, []string{"prompt-eval", "validate", path, "--remote", "--json"}, "http://unused")
	out := stdout.finish()
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitInvalid {
		t.Fatalf("expected ExitCodeError{%d}, got %T %v", promptEvalExitInvalid, err, err)
	}
	var result promptEvalValidationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode json output: %v\n%s", err, out)
	}
	if result.Remote != nil {
		t.Fatalf("remote payload should be absent when workspace is missing: %+v", result.Remote)
	}
	if !containsPromptEvalMessage(result.Errors, "no workspace specified for --remote") {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestPromptEvalValidateCIRequiresRemote(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	result := runPromptEvalValidateJSON(t, []string{"prompt-eval", "validate", path, "--ci", "--json"}, "http://unused")
	if result.Valid || !containsPromptEvalMessage(result.Errors, "--ci requires --remote") {
		t.Fatalf("errors = %v", result.Errors)
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

func TestPromptEvalValidateRemoteFollowsPagination(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	srv := promptEvalRemoteFakeAPI(t, promptEvalRemoteFakeOptions{PaginateAliases: true})
	defer srv.Close()

	result := runPromptEvalRemoteValidateJSON(t, path, srv.URL, false)
	if !result.Valid {
		t.Fatalf("remote validate errors = %v", result.Errors)
	}
	if got := result.Remote.Models[0].ModelAliasID; got != "alias-1" {
		t.Fatalf("model_alias_id = %q, want alias-1", got)
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

func TestPromptEvalRunCreatesPlaygroundTestCasesAndExperiments(t *testing.T) {
	path := writePromptEvalFixture(t, allAssertionsPromptEvalYAML())
	fake := newPromptEvalRunFake(t, nil)
	defer fake.server.Close()

	result := runPromptEvalRunJSON(t, path, fake.server.URL)
	if result.ExitCode != 0 || len(result.Playgrounds) != 1 || len(result.Playgrounds[0].Experiments) != 1 {
		t.Fatalf("unexpected run result: %+v", result)
	}
	if len(fake.playgroundCreates) != 1 {
		t.Fatalf("playground creates = %d, want 1", len(fake.playgroundCreates))
	}
	spec := mapObject(fake.playgroundCreates[0], "evaluation_spec")
	validators := mapSlice(spec, "validators")
	if len(validators) != 7 {
		t.Fatalf("validators = %d, want 7: %#v", len(validators), validators)
	}
	for _, want := range []string{"exact_match", "contains", "regex_match", "json_schema", "json_path_match", "boolean_assert"} {
		if !promptEvalValidatorTypesContain(validators, want) {
			t.Fatalf("validators missing %s: %#v", want, validators)
		}
	}
	if len(fake.testCaseCreates) != 1 {
		t.Fatalf("test case creates = %d, want 1", len(fake.testCaseCreates))
	}
	expectations := mapObject(fake.testCaseCreates[0], "expectations")
	if got := len(mapSlice(expectations, "prompt_eval_assertions")); got != 7 {
		t.Fatalf("prompt_eval_assertions = %d, want 7", got)
	}
	if len(fake.experimentCreates) != 1 || fake.experimentCreates[0]["model_alias_id"] != "alias-1" || fake.experimentCreates[0]["provider_account_id"] != "pa-1" {
		t.Fatalf("experiment create payloads = %#v", fake.experimentCreates)
	}
}

func TestPromptEvalRunValidatorMappingCoversSupportedAssertions(t *testing.T) {
	path := writePromptEvalFixture(t, allAssertionsPromptEvalYAML())
	fake := newPromptEvalRunFake(t, nil)
	defer fake.server.Close()

	_ = runPromptEvalRunJSON(t, path, fake.server.URL)
	spec := mapObject(fake.playgroundCreates[0], "evaluation_spec")
	validators := mapSlice(spec, "validators")
	for _, want := range []string{"exact_match", "contains", "regex_match", "json_schema", "json_path_match", "boolean_assert"} {
		if !promptEvalValidatorTypesContain(validators, want) {
			t.Fatalf("validators missing %s: %#v", want, validators)
		}
	}
	for _, validator := range validators {
		asMap := validator.(map[string]any)
		if !strings.HasPrefix(str(asMap["expected_from"]), "case.expectations.prompt_eval_assertions.") {
			t.Fatalf("validator expected_from should read prompt_eval_assertions: %#v", asMap)
		}
	}
}

func TestPromptEvalRunJSONEnvelope(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	fake := newPromptEvalRunFake(t, nil)
	defer fake.server.Close()

	result := runPromptEvalRunJSON(t, path, fake.server.URL)
	if result.SchemaVersion != 1 || result.WorkspaceID != "ws-1" || result.ConfigHash == "" || result.ModelCount != 1 || result.TestCount != 1 || result.CaseCount != 1 {
		t.Fatalf("unexpected envelope: %+v", result)
	}
	if len(result.Playgrounds) != 1 || result.Playgrounds[0].PlaygroundID == "" || result.Playgrounds[0].PlaygroundURL == "" {
		t.Fatalf("missing playground envelope: %+v", result.Playgrounds)
	}
	if len(result.Playgrounds[0].Experiments) != 1 || result.Playgrounds[0].Experiments[0].ExperimentID == "" || result.Playgrounds[0].Experiments[0].ExperimentURL == "" {
		t.Fatalf("missing experiment envelope: %+v", result.Playgrounds[0].Experiments)
	}
}

func TestPromptEvalRunUpdatesExistingResourcesWithoutDeletingOrphans(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	fake := newPromptEvalRunFake(t, &promptEvalRunFakeState{
		playgrounds: []map[string]any{{"id": "pg-1", "name": "Prompt Eval: refund-bot-v1"}},
		testCases: []map[string]any{
			{"id": "tc-1", "case_key": "greeting", "variables": map[string]any{"input": "old"}, "expectations": map[string]any{}},
			{"id": "tc-orphan", "case_key": "orphan", "variables": map[string]any{}, "expectations": map[string]any{}},
		},
	})
	defer fake.server.Close()

	result := runPromptEvalRunJSON(t, path, fake.server.URL)
	if result.ExitCode != 0 || result.Playgrounds[0].TestsUpdated != 1 {
		t.Fatalf("unexpected run result: %+v", result)
	}
	if len(fake.playgroundUpdates) != 1 {
		t.Fatalf("playground updates = %d, want 1", len(fake.playgroundUpdates))
	}
	if len(fake.testCaseUpdates) != 1 {
		t.Fatalf("test case updates = %d, want 1", len(fake.testCaseUpdates))
	}
	if fake.deleteCalled {
		t.Fatal("run deleted an orphan test case")
	}
}

func TestPromptEvalRunGroupsByAssertionSignature(t *testing.T) {
	body := strings.Replace(validPromptEvalYAML(), "thresholds:", "  - key: regex-case\n    vars:\n      input: Say hello in French\n    assert:\n      - type: regex\n        value: \"Bonjour|Salut\"\n        metric: correctness\nthresholds:", 1)
	path := writePromptEvalFixture(t, body)
	fake := newPromptEvalRunFake(t, nil)
	defer fake.server.Close()

	result := runPromptEvalRunJSON(t, path, fake.server.URL)
	if len(result.Playgrounds) != 2 {
		t.Fatalf("playgrounds = %d, want 2: %+v", len(result.Playgrounds), result.Playgrounds)
	}
	for _, create := range fake.playgroundCreates {
		if !strings.Contains(str(create["name"]), "Prompt Eval: refund-bot-v1 [") {
			t.Fatalf("grouped playground name missing signature suffix: %#v", create["name"])
		}
	}
}

func TestPromptEvalRunRerunNoopsExistingTestCases(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	fake := newPromptEvalRunFake(t, nil)
	defer fake.server.Close()

	first := runPromptEvalRunJSON(t, path, fake.server.URL)
	second := runPromptEvalRunJSON(t, path, fake.server.URL)
	if first.Playgrounds[0].TestsCreated != 1 {
		t.Fatalf("first tests_created = %d, want 1", first.Playgrounds[0].TestsCreated)
	}
	if second.Playgrounds[0].TestsNoop != 1 || second.Playgrounds[0].TestsCreated != 0 || second.Playgrounds[0].TestsUpdated != 0 {
		t.Fatalf("second run should no-op existing test case: %+v", second.Playgrounds[0])
	}
}

func TestPromptEvalRunFollowPassesGate(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	fake := newPromptEvalRunFake(t, nil)
	defer fake.server.Close()

	result, err := runPromptEvalRunJSONWithArgs(t, []string{"prompt-eval", "run", path, "-w", "ws-1", "--json", "--follow", "--poll-interval", "1ms"}, fake.server.URL)
	if err != nil {
		t.Fatalf("follow pass error: %v", err)
	}
	if result.GateVerdict != "pass" || result.Summary.AssertionsPassed != 1 || result.ExitCode != 0 {
		t.Fatalf("unexpected follow result: %+v", result)
	}
	if got := result.Results[0].Telemetry["stability_checks"]; got == nil {
		t.Fatalf("missing stability telemetry: %+v", result.Results[0].Telemetry)
	}
}

func TestPromptEvalRunFollowFailsAssertionGate(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	fake := newPromptEvalRunFake(t, &promptEvalRunFakeState{resultVerdict: "fail"})
	defer fake.server.Close()

	result, err := runPromptEvalRunJSONWithArgs(t, []string{"prompt-eval", "run", path, "-w", "ws-1", "--json", "--follow", "--poll-interval", "1ms"}, fake.server.URL)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitGate {
		t.Fatalf("expected gate exit, got %T %v with result %+v", err, err, result)
	}
	if result.GateVerdict != "fail" || result.ExitCode != promptEvalExitGate {
		t.Fatalf("unexpected gate result: %+v", result)
	}
}

func TestPromptEvalRunFollowUsesConfigThreshold(t *testing.T) {
	body := strings.Replace(validPromptEvalYAML(), "assertion_pass_rate: 0.9", "assertion_pass_rate: 0.0", 1)
	path := writePromptEvalFixture(t, body)
	fake := newPromptEvalRunFake(t, &promptEvalRunFakeState{resultVerdict: "fail"})
	defer fake.server.Close()

	result, err := runPromptEvalRunJSONWithArgs(t, []string{"prompt-eval", "run", path, "-w", "ws-1", "--json", "--follow", "--poll-interval", "1ms"}, fake.server.URL)
	if err != nil {
		t.Fatalf("config threshold should allow zero pass rate: %v with result %+v", err, result)
	}
	if result.Results[0].Thresholds["assertion_pass_rate"] != 0 || result.GateVerdict != "pass" {
		t.Fatalf("config threshold was not applied: %+v", result)
	}
}

func TestPromptEvalRunFollowReportsExecutionError(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	fake := newPromptEvalRunFake(t, &promptEvalRunFakeState{caseStatus: "failed"})
	defer fake.server.Close()

	result, err := runPromptEvalRunJSONWithArgs(t, []string{"prompt-eval", "run", path, "-w", "ws-1", "--json", "--follow", "--poll-interval", "1ms"}, fake.server.URL)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitExecution {
		t.Fatalf("expected execution exit, got %T %v with result %+v", err, err, result)
	}
	if result.Summary.ExecutionErrors != 1 {
		t.Fatalf("execution errors = %d, want 1", result.Summary.ExecutionErrors)
	}
}

func TestPromptEvalRunFollowTimesOutWithPartialResults(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	fake := newPromptEvalRunFake(t, &promptEvalRunFakeState{experimentStatus: "running"})
	defer fake.server.Close()

	result, err := runPromptEvalRunJSONWithArgs(t, []string{"prompt-eval", "run", path, "-w", "ws-1", "--json", "--follow", "--poll-interval", "1ms", "--timeout", "1ms"}, fake.server.URL)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitExecution {
		t.Fatalf("expected timeout execution exit, got %T %v with result %+v", err, err, result)
	}
	if !containsPromptEvalMessage(result.Errors, "timed out") {
		t.Fatalf("errors = %v", result.Errors)
	}
	if len(result.Results) != 1 || result.Results[0].Summary.TotalCases != 1 {
		t.Fatalf("timeout did not include partial results: %+v", result)
	}
}

func TestPromptEvalRunFollowAuthFailureDuringPoll(t *testing.T) {
	path := writePromptEvalFixture(t, validPromptEvalYAML())
	fake := newPromptEvalRunFake(t, &promptEvalRunFakeState{authOnExperimentGet: true})
	defer fake.server.Close()

	result, err := runPromptEvalRunJSONWithArgs(t, []string{"prompt-eval", "run", path, "-w", "ws-1", "--json", "--follow", "--poll-interval", "1ms"}, fake.server.URL)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitInvalid {
		t.Fatalf("expected auth invalid exit, got %T %v with result %+v", err, err, result)
	}
}

func TestPromptEvalResultsCommandPrintsStableEnvelope(t *testing.T) {
	fake := newPromptEvalRunFake(t, nil)
	defer fake.server.Close()

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"prompt-eval", "results", "exp-1", "--json"}, fake.server.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("results command error: %v\n%s", err, out)
	}
	var result promptEvalResultsEnvelope
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode results json: %v\n%s", err, out)
	}
	if result.SchemaVersion != 1 || result.Summary.AssertionsPassed != 1 || result.GateVerdict != "pass" || result.Telemetry["row_count"] == nil {
		t.Fatalf("unexpected results envelope: %+v", result)
	}
}

func TestPromptEvalResultsCommandPrintsReadableTable(t *testing.T) {
	fake := newPromptEvalRunFake(t, &promptEvalRunFakeState{resultVerdict: "fail"})
	defer fake.server.Close()

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"prompt-eval", "results", "exp-1"}, fake.server.URL)
	out := stdout.finish()
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitGate {
		t.Fatalf("expected gate exit, got %T %v\n%s", err, err, out)
	}
	for _, want := range []string{
		"EXPERIMENT",
		"+",
		"|",
		"PASS RATE",
		"PASS/FAIL/ERR",
		"fail",
		"0%",
		"0/1/0",
		"DIMENSION",
		"correctness",
		"RESULT",
		"FAIL",
		"contains",
		"v1",
		"FAILURE CASE",
		"done",
		"Bonjour",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("table output missing %q:\n%s", want, out)
		}
	}
}

func TestPromptEvalResultsCommandWrapsBackendErrorExit(t *testing.T) {
	fake := newPromptEvalRunFake(t, &promptEvalRunFakeState{resultStatusCode: http.StatusBadGateway})
	defer fake.server.Close()

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"prompt-eval", "results", "exp-1", "--json"}, fake.server.URL)
	out := stdout.finish()
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != promptEvalExitExecution {
		t.Fatalf("expected execution exit, got %T %v\n%s", err, err, out)
	}
	var result promptEvalResultsEnvelope
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode results json: %v\n%s", err, out)
	}
	if result.ExitCode != promptEvalExitExecution {
		t.Fatalf("json exit_code = %d, want %d", result.ExitCode, promptEvalExitExecution)
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

func validPromptfooYAML() string {
	return `description: promptfoo-import
prompts:
  - "Reply to {{input}}"
providers:
  - openai:gpt-5.5
tests:
  - description: greeting
    vars:
      input: hello
    assert:
      - type: contains
        value: hello
`
}

func validatePromptEvalConfigForTest(cfg promptEvalConfig) promptEvalValidationResult {
	result := promptEvalValidationResult{SchemaVersion: promptEvalSchemaVersion, Valid: true, AssertionSignatures: []string{}, MaxCases: 100}
	result.ModelCount = len(cfg.Models)
	result.TestCount = len(cfg.Tests)
	result.CaseCount = len(cfg.Models) * len(cfg.Tests)
	validatePromptEvalConfig(cfg, 100, &result)
	result.Valid = len(result.Errors) == 0
	if !result.Valid {
		result.ExitCode = promptEvalExitInvalid
	}
	return result
}

func runPromptfooImportJSON(t *testing.T, path string) promptfooImportResult {
	t.Helper()
	return runPromptfooImportJSONWithArgs(t, []string{"prompt-eval", "import-promptfoo", path, "--json"})
}

func runPromptfooImportJSONWithArgs(t *testing.T, args []string) promptfooImportResult {
	t.Helper()
	stdout := captureStdout(t)
	_ = executeCommand(t, args, "http://unused")
	out := stdout.finish()
	var result promptfooImportResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode promptfoo import json: %v\n%s", err, out)
	}
	return result
}

func allAssertionsPromptEvalYAML() string {
	return `schemaVersion: 1
name: all-assertions
prompt:
  template: |
    Reply to: {{input}}
models:
  - alias: gpt-5.5
    provider_account: default
tests:
  - key: all
    vars:
      input: test
    assert:
      - type: exact_match
        value: done
        metric: correctness
      - type: equals
        value: done
        metric: correctness
      - type: contains
        value: done
        metric: correctness
      - type: regex
        value: "done|ok"
        metric: correctness
      - type: json_schema
        value: '{"type":"object"}'
        metric: correctness
      - type: json_path_match
        value: '{"path":"$.ok","value":true}'
        metric: correctness
      - type: boolean_assert
        value: true
        metric: correctness
`
}

type promptEvalRemoteFakeOptions struct {
	ModelAliases     []map[string]any
	ProviderAccounts []map[string]any
	Playgrounds      []map[string]any
	TestCases        []map[string]any
	OnWrite          func()
	PaginateAliases  bool
}

// promptEvalAuthEnv makes a prompt-eval test hermetic and authenticated:
// it isolates the config dir so the test can never pick up the developer's
// real stored credentials, and sets a dummy token so the client's
// no-credential short-circuit (ensureAuth) lets the request reach the fake
// server. The fakes do not check the token; its value is irrelevant.
func promptEvalAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
}

func promptEvalRemoteFakeAPI(t *testing.T, options promptEvalRemoteFakeOptions) *httptest.Server {
	t.Helper()
	promptEvalAuthEnv(t)
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
		if r.Method == http.MethodPost || r.Method == http.MethodPatch || r.Method == http.MethodPut || r.Method == http.MethodDelete {
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
			if options.PaginateAliases && r.URL.Query().Get("cursor") == "" {
				json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{}, "next_cursor": "page-2"})
				return
			}
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
	return runPromptEvalValidateJSON(t, args, apiURL)
}

func runPromptEvalValidateJSON(t *testing.T, args []string, apiURL string) promptEvalValidationResult {
	t.Helper()
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

type promptEvalRunFakeState struct {
	playgrounds         []map[string]any
	testCases           []map[string]any
	experimentStatus    string
	caseStatus          string
	resultVerdict       string
	resultStatusCode    int
	authOnExperimentGet bool
}

type promptEvalRunFake struct {
	server              *httptest.Server
	mu                  sync.Mutex
	playgrounds         []map[string]any
	testCases           []map[string]any
	playgroundCreates   []map[string]any
	playgroundUpdates   []map[string]any
	testCaseCreates     []map[string]any
	testCaseUpdates     []map[string]any
	experimentCreates   []map[string]any
	deleteCalled        bool
	nextPlaygroundID    int
	nextExperimentID    int
	experimentStatus    string
	caseStatus          string
	resultVerdict       string
	resultStatusCode    int
	authOnExperimentGet bool
}

func newPromptEvalRunFake(t *testing.T, state *promptEvalRunFakeState) *promptEvalRunFake {
	t.Helper()
	promptEvalAuthEnv(t)
	f := &promptEvalRunFake{nextPlaygroundID: 1, nextExperimentID: 1}
	if state != nil {
		f.playgrounds = append(f.playgrounds, state.playgrounds...)
		f.testCases = append(f.testCases, state.testCases...)
		f.experimentStatus = state.experimentStatus
		f.caseStatus = state.caseStatus
		f.resultVerdict = state.resultVerdict
		f.resultStatusCode = state.resultStatusCode
		f.authOnExperimentGet = state.authOnExperimentGet
	}
	if f.experimentStatus == "" {
		f.experimentStatus = "completed"
	}
	if f.caseStatus == "" {
		f.caseStatus = "completed"
	}
	if f.resultVerdict == "" {
		f.resultVerdict = "pass"
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/model-aliases":
			json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"id": "alias-1", "alias_key": "gpt-5.5", "provider_account_id": "pa-1"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/provider-accounts":
			json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"id": "pa-1", "provider_key": "openai", "status": "active"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/playgrounds":
			json.NewEncoder(w).Encode(map[string]any{"items": f.playgrounds})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces/ws-1/playgrounds":
			body := decodePromptEvalRequestBody(t, r)
			f.playgroundCreates = append(f.playgroundCreates, body)
			id := fmt.Sprintf("pg-%d", f.nextPlaygroundID)
			f.nextPlaygroundID++
			item := map[string]any{"id": id, "name": body["name"], "url": "https://agentclash.dev/playgrounds/" + id}
			f.playgrounds = append(f.playgrounds, item)
			json.NewEncoder(w).Encode(item)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/playgrounds/"):
			body := decodePromptEvalRequestBody(t, r)
			f.playgroundUpdates = append(f.playgroundUpdates, body)
			id := strings.TrimPrefix(r.URL.Path, "/v1/playgrounds/")
			json.NewEncoder(w).Encode(map[string]any{"id": id, "name": body["name"], "url": "https://agentclash.dev/playgrounds/" + id})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/playgrounds/") && strings.HasSuffix(r.URL.Path, "/test-cases"):
			json.NewEncoder(w).Encode(map[string]any{"items": f.testCases})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/playgrounds/") && strings.HasSuffix(r.URL.Path, "/test-cases"):
			body := decodePromptEvalRequestBody(t, r)
			f.testCaseCreates = append(f.testCaseCreates, body)
			id := fmt.Sprintf("tc-created-%d", len(f.testCaseCreates))
			f.testCases = append(f.testCases, map[string]any{
				"id":           id,
				"case_key":     body["case_key"],
				"variables":    body["variables"],
				"expectations": body["expectations"],
			})
			json.NewEncoder(w).Encode(map[string]any{"id": id, "case_key": body["case_key"]})
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/playground-test-cases/"):
			body := decodePromptEvalRequestBody(t, r)
			f.testCaseUpdates = append(f.testCaseUpdates, body)
			json.NewEncoder(w).Encode(map[string]any{"id": strings.TrimPrefix(r.URL.Path, "/v1/playground-test-cases/"), "case_key": body["case_key"]})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/playgrounds/") && strings.HasSuffix(r.URL.Path, "/experiments"):
			body := decodePromptEvalRequestBody(t, r)
			f.experimentCreates = append(f.experimentCreates, body)
			id := fmt.Sprintf("exp-%d", f.nextExperimentID)
			f.nextExperimentID++
			json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "queued", "url": "https://agentclash.dev/playground-experiments/" + id})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/playground-experiments/") && strings.HasSuffix(r.URL.Path, "/results"):
			if f.resultStatusCode != 0 {
				w.WriteHeader(f.resultStatusCode)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "bad_gateway", "message": "bad gateway"}})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{f.resultItem()}})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/playground-experiments/"):
			if f.authOnExperimentGet {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "unauthorized", "message": "unauthorized"}})
				return
			}
			id := strings.TrimPrefix(r.URL.Path, "/v1/playground-experiments/")
			json.NewEncoder(w).Encode(map[string]any{"id": id, "status": f.experimentStatus})
		case r.Method == http.MethodDelete:
			f.deleteCalled = true
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "delete not allowed"}})
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "not found"}})
		}
	}))
	return f
}

func (f *promptEvalRunFake) resultItem() map[string]any {
	validatorVerdict := f.resultVerdict
	score := 1.0
	if validatorVerdict == "fail" {
		score = 0
	}
	item := map[string]any{
		"id":               "res-1",
		"case_key":         "greeting",
		"status":           f.caseStatus,
		"actual_output":    "Bonjour",
		"latency_ms":       12,
		"total_tokens":     7,
		"dimension_scores": map[string]any{"correctness": score},
		"validator_results": []any{
			map[string]any{"key": "v1", "type": "contains", "verdict": validatorVerdict, "normalized_score": score, "expected_value": "done", "reason": ""},
		},
	}
	if f.caseStatus == "failed" {
		item["error_message"] = "provider failed"
	}
	return item
}

func decodePromptEvalRequestBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

func runPromptEvalRunJSON(t *testing.T, path, apiURL string) promptEvalRunResult {
	t.Helper()
	result, err := runPromptEvalRunJSONWithArgs(t, []string{"prompt-eval", "run", path, "-w", "ws-1", "--json"}, apiURL)
	if err != nil {
		t.Fatalf("prompt-eval run error: %v", err)
	}
	return result
}

func runPromptEvalRunJSONWithArgs(t *testing.T, args []string, apiURL string) (promptEvalRunResult, error) {
	t.Helper()
	stdout := captureStdout(t)
	err := executeCommand(t, args, apiURL)
	out := stdout.finish()
	var result promptEvalRunResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode run json: %v\n%s", err, out)
	}
	return result, err
}

func promptEvalValidatorTypesContain(validators []any, want string) bool {
	for _, item := range validators {
		if asMap, ok := item.(map[string]any); ok && asMap["type"] == want {
			return true
		}
	}
	return false
}
