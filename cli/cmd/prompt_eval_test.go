package cmd

import (
	"encoding/json"
	"errors"
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
