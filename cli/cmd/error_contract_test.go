package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"testing"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

// WI-2/WI-6 lockstep: the envelope's retryable bit and the process exit code
// are two representations of one classification — exit 75 iff retryable:true.
func TestExitCodeBandMatchesEnvelopeRetryable(t *testing.T) {
	intp := func(v int) *int { return &v }
	cases := []struct {
		name          string
		err           error
		wantExit      int
		wantRetryable bool
		wantCode      string
	}{
		{
			name:          "429 rate limited",
			err:           &cliapi.APIError{StatusCode: 429, Code: "rate_limited", Message: "slow down", RetryAfterSeconds: intp(7)},
			wantExit:      exitRetryableFailure,
			wantRetryable: true,
			wantCode:      "rate_limited",
		},
		{
			name:          "503 server error",
			err:           &cliapi.APIError{StatusCode: 503, Code: "unavailable", Message: "down"},
			wantExit:      exitRetryableFailure,
			wantRetryable: true,
			wantCode:      "unavailable",
		},
		{
			name:          "transport error",
			err:           &url.Error{Op: "Get", URL: "http://127.0.0.1:1/v1/runs", Err: errors.New("connection refused")},
			wantExit:      exitRetryableFailure,
			wantRetryable: true,
			wantCode:      "request_failed",
		},
		{
			name:          "402 quota gate is not retryable",
			err:           &cliapi.APIError{StatusCode: 402, Code: "quota_exceeded", Message: "quota", PlanKey: "free", Limit: intp(100), Used: intp(100)},
			wantExit:      1,
			wantRetryable: false,
			wantCode:      "quota_exceeded",
		},
		{
			name:          "401 auth",
			err:           &cliapi.APIError{StatusCode: 401, Code: "unauthorized", Message: "no"},
			wantExit:      exitAuthDenied,
			wantRetryable: false,
			wantCode:      "unauthorized",
		},
		{
			name:          "403 forbidden",
			err:           &cliapi.APIError{StatusCode: 403, Code: "forbidden", Message: "no"},
			wantExit:      exitAuthDenied,
			wantRetryable: false,
			wantCode:      "forbidden",
		},
		{
			name:          "404 not found",
			err:           &cliapi.APIError{StatusCode: 404, Code: "not_found", Message: "gone"},
			wantExit:      exitNotFound,
			wantRetryable: false,
			wantCode:      "not_found",
		},
		{
			name:          "local validation error",
			err:           &cliError{Code: "invalid_argument", Message: "bad flag"},
			wantExit:      exitValidationError,
			wantRetryable: false,
			wantCode:      "invalid_argument",
		},
		{
			name:          "missing workspace keeps documented code 2",
			err:           &cliError{Code: "missing_workspace", Message: "no workspace"},
			wantExit:      2,
			wantRetryable: false,
			wantCode:      "missing_workspace",
		},
		{
			name:          "missing local file",
			err:           fmt.Errorf("opening pack: %w", fs.ErrNotExist),
			wantExit:      exitNotFound,
			wantRetryable: false,
			wantCode:      "file_not_found",
		},
		{
			name:          "unclassified error stays generic 1",
			err:           errors.New("something odd"),
			wantExit:      1,
			wantRetryable: false,
			wantCode:      "invalid_argument",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCodeForError(tc.err); got != tc.wantExit {
				t.Errorf("exitCodeForError = %d, want %d", got, tc.wantExit)
			}
			se := classifyStructuredError(tc.err)
			if se.Retryable != tc.wantRetryable {
				t.Errorf("retryable = %v, want %v", se.Retryable, tc.wantRetryable)
			}
			if se.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", se.Code, tc.wantCode)
			}
			// The invariant itself, independent of the per-case expectations.
			if (tc.wantExit == exitRetryableFailure) != se.Retryable {
				t.Errorf("invariant broken: exit %d with retryable=%v", tc.wantExit, se.Retryable)
			}
		})
	}
}

// Command-specific ExitCodeErrors must keep precedence over the global band.
func TestExitCodeErrorPrecedenceOverBand(t *testing.T) {
	err := &ExitCodeError{Code: ciRunExitTimeout, Message: "timed out"}
	if got := exitCodeForError(err); got != ciRunExitTimeout {
		t.Fatalf("exitCodeForError = %d, want command-specific %d", got, ciRunExitTimeout)
	}
}

// WI-2: a 429 envelope carries retryable:true plus the parsed Retry-After.
func TestRenderError429CarriesRetryAfterDetails(t *testing.T) {
	// flagJSON is package-global; cmdMu is the suite's serialization
	// mechanism for global flag state (same contract as executeCommand).
	cmdMu.Lock()
	defer cmdMu.Unlock()
	flagJSON = true
	defer func() { flagJSON = false }()

	seven := 7
	err := &cliapi.APIError{StatusCode: 429, Code: "rate_limited", Message: "slow down", RetryAfterSeconds: &seven}

	var stderr bytes.Buffer
	exitCode, rendered := RenderError(err, &stderr)
	if !rendered {
		t.Fatal("expected a rendered JSON envelope")
	}
	if exitCode != exitRetryableFailure {
		t.Fatalf("exit = %d, want %d", exitCode, exitRetryableFailure)
	}

	var envelope structuredErrorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON envelope: %v", err)
	}
	if !envelope.Error.Retryable {
		t.Fatal("retryable = false, want true")
	}
	if got, ok := envelope.Error.Details["retry_after_seconds"].(float64); !ok || int(got) != 7 {
		t.Fatalf("details.retry_after_seconds = %v, want 7", envelope.Error.Details["retry_after_seconds"])
	}
}

// The `retryable` key must always be present in the envelope — agents branch
// on its value without an existence check.
func TestRenderErrorAlwaysEmitsRetryableKey(t *testing.T) {
	cmdMu.Lock()
	defer cmdMu.Unlock()
	flagJSON = true
	defer func() { flagJSON = false }()

	var stderr bytes.Buffer
	_, rendered := RenderError(&cliapi.APIError{StatusCode: 402, Code: "quota_exceeded", Message: "quota"}, &stderr)
	if !rendered {
		t.Fatal("expected a rendered JSON envelope")
	}
	var raw map[string]map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON envelope: %v", err)
	}
	if _, present := raw["error"]["retryable"]; !present {
		t.Fatal("envelope must always contain error.retryable")
	}
	if raw["error"]["retryable"] != false {
		t.Fatalf("retryable = %v, want false for a 402 quota gate", raw["error"]["retryable"])
	}
}
