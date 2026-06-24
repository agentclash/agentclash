package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/config"
)

func TestRenderErrorJSONPreservesAPIError(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/not-a-uuid": jsonHandler(400, map[string]any{
			"error": map[string]any{"code": "invalid_run_id", "message": "run id must be a valid UUID"},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "get", "not-a-uuid", "--json"}, srv.URL)
	if err == nil {
		t.Fatal("expected API error")
	}

	var stderr bytes.Buffer
	exitCode, rendered := RenderError(err, &stderr)
	if !rendered {
		t.Fatal("expected structured error to render")
	}
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code")
	}

	envelope := decodeStructuredError(t, stderr.String())
	if envelope.Error.Code != "invalid_run_id" {
		t.Fatalf("code = %q, want invalid_run_id", envelope.Error.Code)
	}
	if envelope.Error.Message != "run id must be a valid UUID" {
		t.Fatalf("message = %q", envelope.Error.Message)
	}
	if envelope.Error.Status != 400 {
		t.Fatalf("status = %d, want 400", envelope.Error.Status)
	}
	if envelope.Error.Details == nil {
		t.Fatal("details must be present")
	}
}

func TestRenderErrorJSONIncludesBillingDetailsAndNextStep(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(402, map[string]any{
			"error": map[string]any{
				"code":           "quota_exceeded",
				"message":        "monthly run quota exhausted",
				"plan_key":       "free",
				"upgrade_target": "pro",
				"limit":          100,
				"used":           100,
				"remaining":      0,
				"reset_at":       "2026-07-01T00:00:00Z",
			},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "list", "-w", "ws-1", "--json"}, srv.URL)
	if err == nil {
		t.Fatal("expected quota error")
	}

	var stderr bytes.Buffer
	_, rendered := RenderError(err, &stderr)
	if !rendered {
		t.Fatal("expected structured error to render")
	}

	envelope := decodeStructuredError(t, stderr.String())
	if envelope.Error.Code != "quota_exceeded" || envelope.Error.Status != 402 {
		t.Fatalf("error = %#v, want quota_exceeded 402", envelope.Error)
	}
	if got := envelope.Error.Details["plan_key"]; got != "free" {
		t.Fatalf("details.plan_key = %v, want free", got)
	}
	if got := envelope.Error.Details["upgrade_target"]; got != "pro" {
		t.Fatalf("details.upgrade_target = %v, want pro", got)
	}
	if got := envelope.Error.Details["limit"]; got != float64(100) {
		t.Fatalf("details.limit = %v, want 100", got)
	}
	if got := envelope.Error.Details["used"]; got != float64(100) {
		t.Fatalf("details.used = %v, want 100", got)
	}
	if got := envelope.Error.Details["reset_at"]; got != "2026-07-01T00:00:00Z" {
		t.Fatalf("details.reset_at = %v, want 2026-07-01T00:00:00Z", got)
	}
	if envelope.Error.NextStep == "" {
		t.Fatal("expected a next_step hint for a billing gate")
	}
}

func TestRenderErrorJSONQuotaResetHintWithoutUpgrade(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(402, map[string]any{
			"error": map[string]any{
				"code":      "quota_exceeded",
				"message":   "monthly run quota exhausted",
				"plan_key":  "enterprise",
				"limit":     100,
				"used":      100,
				"remaining": 0,
				"reset_at":  "2026-07-01T00:00:00Z",
			},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "list", "-w", "ws-1", "--json"}, srv.URL)
	if err == nil {
		t.Fatal("expected quota error")
	}

	var stderr bytes.Buffer
	_, rendered := RenderError(err, &stderr)
	if !rendered {
		t.Fatal("expected structured error to render")
	}

	envelope := decodeStructuredError(t, stderr.String())
	if envelope.Error.Code != "quota_exceeded" {
		t.Fatalf("code = %q, want quota_exceeded", envelope.Error.Code)
	}
	if got := envelope.Error.Details["reset_at"]; got != "2026-07-01T00:00:00Z" {
		t.Fatalf("details.reset_at = %v, want 2026-07-01T00:00:00Z", got)
	}
	if _, ok := envelope.Error.Details["upgrade_target"]; ok {
		t.Fatalf("details.upgrade_target should be absent, got %v", envelope.Error.Details["upgrade_target"])
	}
	if !strings.Contains(envelope.Error.NextStep, "reset") {
		t.Fatalf("next_step = %q, want a wait-for-reset hint mentioning \"reset\"", envelope.Error.NextStep)
	}
	if strings.Contains(envelope.Error.NextStep, "update billing") {
		t.Fatalf("next_step = %q, should not advise \"update billing\" when a reset_at is present", envelope.Error.NextStep)
	}
}

func TestRenderErrorOutputJSONPreservesAPIError(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-denied/runs": jsonHandler(403, map[string]any{
			"error": map[string]any{"code": "forbidden", "message": "workspace access denied"},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "list", "-w", "ws-denied", "--output", "json"}, srv.URL)
	if err == nil {
		t.Fatal("expected API error")
	}

	var stderr bytes.Buffer
	_, rendered := RenderError(err, &stderr)
	if !rendered {
		t.Fatal("expected structured error to render for --output json")
	}

	envelope := decodeStructuredError(t, stderr.String())
	if envelope.Error.Code != "forbidden" || envelope.Error.Status != 403 {
		t.Fatalf("error = %#v, want forbidden 403", envelope.Error)
	}
}

func TestRenderErrorUserConfigJSONPreservesAPIError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.Save(config.UserConfig{Output: "json"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-denied/runs": jsonHandler(403, map[string]any{
			"error": map[string]any{"code": "forbidden", "message": "workspace access denied"},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "list", "-w", "ws-denied"}, srv.URL)
	if err == nil {
		t.Fatal("expected API error")
	}

	var stderr bytes.Buffer
	_, rendered := RenderError(err, &stderr)
	if !rendered {
		t.Fatal("expected structured error to render for config output json")
	}

	envelope := decodeStructuredError(t, stderr.String())
	if envelope.Error.Code != "forbidden" || envelope.Error.Status != 403 {
		t.Fatalf("error = %#v, want forbidden 403", envelope.Error)
	}
}

func TestRenderErrorUserConfigTableDoesNotRender(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.Save(config.UserConfig{Output: "table"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-denied/runs": jsonHandler(403, map[string]any{
			"error": map[string]any{"code": "forbidden", "message": "workspace access denied"},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "list", "-w", "ws-denied"}, srv.URL)
	if err == nil {
		t.Fatal("expected API error")
	}

	var stderr bytes.Buffer
	_, rendered := RenderError(err, &stderr)
	if rendered {
		t.Fatalf("expected table config to skip structured renderer, got %q", stderr.String())
	}
}

func TestRenderErrorJSONClassifiesMissingFile(t *testing.T) {
	err := executeCommand(t, []string{"eval-pack", "validate", "missing.yaml", "-w", "ws-1", "--json"}, "http://unused")
	if err == nil {
		t.Fatal("expected missing file error")
	}

	var stderr bytes.Buffer
	_, rendered := RenderError(err, &stderr)
	if !rendered {
		t.Fatal("expected structured error to render")
	}

	envelope := decodeStructuredError(t, stderr.String())
	if envelope.Error.Code != "file_not_found" {
		t.Fatalf("code = %q, want file_not_found", envelope.Error.Code)
	}
	if !strings.Contains(envelope.Error.Message, "missing.yaml") {
		t.Fatalf("message = %q, want missing filename", envelope.Error.Message)
	}
}

func TestRenderErrorJSONClassifiesLocalValidation(t *testing.T) {
	err := executeCommand(t, []string{
		"run", "create",
		"-w", "ws-1",
		"--eval-pack-version", "cpv-1",
		"--deployments", "dep-1",
		"--scope", "bogus",
		"--json",
	}, "http://unused")
	if err == nil {
		t.Fatal("expected local validation error")
	}

	var stderr bytes.Buffer
	_, rendered := RenderError(err, &stderr)
	if !rendered {
		t.Fatal("expected structured error to render")
	}

	envelope := decodeStructuredError(t, stderr.String())
	if envelope.Error.Code != "invalid_argument" {
		t.Fatalf("code = %q, want invalid_argument", envelope.Error.Code)
	}
	if !strings.Contains(envelope.Error.Message, "invalid --scope") {
		t.Fatalf("message = %q, want invalid scope", envelope.Error.Message)
	}
}

func TestRenderErrorJSONClassifiesRequestFailure(t *testing.T) {
	err := fmt.Errorf("request failed: %w", &url.Error{Op: "Get", URL: "http://127.0.0.1:1", Err: errors.New("connection refused")})
	withJSONFlag(t, func() {
		var stderr bytes.Buffer
		_, rendered := RenderError(err, &stderr)
		if !rendered {
			t.Fatal("expected structured error to render")
		}

		envelope := decodeStructuredError(t, stderr.String())
		if envelope.Error.Code != "request_failed" {
			t.Fatalf("code = %q, want request_failed", envelope.Error.Code)
		}
	})
}

func TestRenderErrorJSONClassifiesInvalidConfig(t *testing.T) {
	err := fmt.Errorf("loading config: %w", errors.New("invalid yaml"))
	withJSONFlag(t, func() {
		var stderr bytes.Buffer
		_, rendered := RenderError(err, &stderr)
		if !rendered {
			t.Fatal("expected structured error to render")
		}

		envelope := decodeStructuredError(t, stderr.String())
		if envelope.Error.Code != "invalid_config" {
			t.Fatalf("code = %q, want invalid_config", envelope.Error.Code)
		}
	})
}

func withJSONFlag(t *testing.T, fn func()) {
	t.Helper()
	oldJSON := flagJSON
	oldOutput := flagOutput
	flagJSON = true
	flagOutput = ""
	t.Cleanup(func() {
		flagJSON = oldJSON
		flagOutput = oldOutput
	})
	fn()
}

func TestRenderErrorDefaultOutputDoesNotRender(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/organizations": jsonHandler(401, map[string]any{
			"error": map[string]any{"code": "unauthorized", "message": "invalid token"},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "bad-token")
	err := executeCommand(t, []string{"org", "list"}, srv.URL)
	if err == nil {
		t.Fatal("expected API error")
	}

	var stderr bytes.Buffer
	_, rendered := RenderError(err, &stderr)
	if rendered {
		t.Fatalf("expected default output to skip structured renderer, got %q", stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func decodeStructuredError(t *testing.T, raw string) structuredErrorEnvelope {
	t.Helper()

	var envelope structuredErrorEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("structured error is not valid JSON: %v\n%s", err, raw)
	}
	return envelope
}
