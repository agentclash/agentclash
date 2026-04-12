package engine

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestHTTPRequestTool_RejectsWhenNetworkDisabled(t *testing.T) {
	session := sandbox.NewFakeSession("http-denied")
	result, err := executeHTTPRequestTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"method":"GET","url":"https://example.com"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindNetwork}, AllowNetwork: false},
	})
	if err != nil {
		t.Fatalf("executeHTTPRequestTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool denial, got %#v", result)
	}
}

func TestHTTPRequestTool_WritesStructuredRequestAndParsesResponse(t *testing.T) {
	session := sandbox.NewFakeSession("http-ok")
	session.SetExecFunc(func(request sandbox.ExecRequest, files map[string][]byte) (sandbox.ExecResult, error) {
		switch request.Command[0] {
		case "mkdir":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case "python3":
			var payload map[string]any
			if err := json.Unmarshal(files[request.Command[2]], &payload); err != nil {
				t.Fatalf("decode request payload: %v", err)
			}
			if payload["url"] != "https://example.com" {
				t.Fatalf("url payload = %#v", payload["url"])
			}
			allowlist := payload["network_allowlist"].([]any)
			if len(allowlist) != 1 || allowlist[0].(string) != "203.0.113.0/24" {
				t.Fatalf("allowlist = %#v", allowlist)
			}
			return sandbox.ExecResult{
				ExitCode: 0,
				Stdout:   `{"status_code":200,"headers":{"content-type":"application/json"},"url":"https://example.com","body":"ok","body_bytes":2}`,
			}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})

	result, err := executeHTTPRequestTool(t.Context(), ToolExecutionRequest{
		Args:             json.RawMessage(`{"method":"GET","url":"https://example.com","timeout_seconds":120}`),
		Session:          session,
		ToolPolicy:       sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindNetwork}, AllowNetwork: true},
		NetworkAllowlist: []string{"203.0.113.0/24"},
	})
	if err != nil {
		t.Fatalf("executeHTTPRequestTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}
	if !strings.Contains(result.Content, `"status_code":200`) {
		t.Fatalf("expected structured response payload, got %s", result.Content)
	}
	if !strings.Contains(result.Content, `"body":"ok"`) {
		t.Fatalf("expected body in structured response, got %s", result.Content)
	}
}

func TestHTTPRequestTool_ReturnsToolErrorOnScriptFailure(t *testing.T) {
	session := sandbox.NewFakeSession("http-fail")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch request.Command[0] {
		case "mkdir":
			return sandbox.ExecResult{ExitCode: 0}, nil
		default:
			return sandbox.ExecResult{ExitCode: 1, Stderr: "target host is blocked by network policy"}, nil
		}
	})

	result, err := executeHTTPRequestTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"method":"GET","url":"http://127.0.0.1"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindNetwork}, AllowNetwork: true},
	})
	if err != nil {
		t.Fatalf("executeHTTPRequestTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error, got %#v", result)
	}
}

func TestScrubSensitiveResponseHeaders_CaseInsensitive(t *testing.T) {
	// HTTP header names are case-insensitive (RFC 7230). Vendors
	// capitalize all over the place — AUTHORIZATION, authorization,
	// Authorization, X-API-KEY, x-api-key, X-Api-Key. The scrubber
	// must catch every casing.
	payload := map[string]any{
		"headers": map[string]any{
			"AUTHORIZATION":   "Bearer leaked1",
			"Proxy-Authorization": "Bearer leaked2",
			"SET-COOKIE":      "sid=leaked3",
			"X-Api-Key":       "leaked4",
			"x-access-token":  "leaked5",
			"X-CSRF-Token":    "leaked6",
			"X-Request-Id":    "safe-id",
		},
	}
	scrubSensitiveResponseHeaders(payload)

	headers := payload["headers"].(map[string]any)
	for _, key := range []string{"AUTHORIZATION", "Proxy-Authorization", "SET-COOKIE", "X-Api-Key", "x-access-token", "X-CSRF-Token"} {
		if got := headers[key]; got != redactedHeaderMarker {
			t.Errorf("header %q = %v, want %q", key, got, redactedHeaderMarker)
		}
	}
	if got := headers["X-Request-Id"]; got != "safe-id" {
		t.Errorf("non-sensitive X-Request-Id was scrubbed: %v", got)
	}
}

func TestScrubStderrSecrets_RemovesAuthPatterns(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		reject string // substring that must not survive
	}{
		{"authorization header in traceback", "httpx error at line 42: Authorization: Bearer super-secret-token", "super-secret-token"},
		{"cookie header", "Traceback: sent Cookie: session=abc123", "session=abc123"},
		{"bearer token in prose", "failed with token Bearer abc123def456", "abc123def456"},
		{"basic auth", "server rejected Basic YWRtaW46cGFzc3dvcmQ=", "YWRtaW46cGFzc3dvcmQ="},
		{"x-api-key with value", "x-api-key: leaked-value-here", "leaked-value-here"},
		{"proxy-authorization", "proxy-authorization: Digest leaked", "Digest leaked"},
		{"mixed case header", "AUTHORIZATION: Bearer shouty-secret", "shouty-secret"},
		{"bearer token with special chars", "Bearer abc:~def!123@456", "abc:~def!123@456"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scrubbed := scrubStderrSecrets(tc.input)
			if strings.Contains(scrubbed, tc.reject) {
				t.Fatalf("scrub left %q in output: %q", tc.reject, scrubbed)
			}
			if !strings.Contains(scrubbed, redactedHeaderMarker) {
				t.Fatalf("expected redaction marker in output: %q", scrubbed)
			}
		})
	}
}

func TestScrubStderrSecrets_PreservesNonSensitiveText(t *testing.T) {
	// A scrubber that over-matches is nearly as bad as no scrubber —
	// pack authors lose debuggability. Assert that normal error text
	// survives.
	cases := []string{
		"dns resolution failed: Name or service not known",
		"request timed out",
		"target host is blocked by network policy",
		"http error: TimeoutException",
		"connection refused by 203.0.113.42",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			if got := scrubStderrSecrets(input); got != input {
				t.Fatalf("legitimate error text was scrubbed: input=%q output=%q", input, got)
			}
		})
	}
}

func TestHTTPRequestTool_ScrubsStderrLeaksDefenseInDepth(t *testing.T) {
	// Simulate an older http_request.py version that did NOT wrap its
	// httpx call in try/except and instead printed a raw traceback
	// with the full Authorization header to stderr. The Go-side
	// scrubber must catch this even if the python sanitization is
	// absent.
	session := sandbox.NewFakeSession("http-legacy-stderr")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch request.Command[0] {
		case "mkdir":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case "python3":
			return sandbox.ExecResult{
				ExitCode: 1,
				Stderr: "Traceback (most recent call last):\n" +
					"  File \"/tools/http_request.py\", line 90, in main\n" +
					"    response = client.request(request[\"method\"], request[\"url\"], headers={'Authorization': 'Bearer LEAKED_LEGACY'})\n" +
					"httpx.ConnectError: connection refused\n",
			}, nil
		default:
			return sandbox.ExecResult{}, nil
		}
	})

	result, err := executeHTTPRequestTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"method":"GET","url":"https://api.example.com","headers":{"Authorization":"Bearer LEAKED_LEGACY"}}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindNetwork}, AllowNetwork: true},
	})
	if err != nil {
		t.Fatalf("executeHTTPRequestTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error result")
	}
	if strings.Contains(result.Content, "LEAKED_LEGACY") {
		t.Fatalf("stderr leak survived Go-side scrubber: %s", result.Content)
	}
}

func TestHTTPRequestTool_DecodeErrorDoesNotLeakStdoutBytes(t *testing.T) {
	// If python emits a malformed response, the json.Unmarshal error
	// used to include a slice of the unparsable bytes — which could
	// contain an Authorization header the server echoed back. The
	// decode error must be generic now; no stdout bytes in the cause.
	session := sandbox.NewFakeSession("http-bad-json")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch request.Command[0] {
		case "mkdir":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case "python3":
			// Not JSON — and crucially, it contains what looks like a
			// leaked secret. None of these bytes may surface in the
			// returned error.
			return sandbox.ExecResult{
				ExitCode: 0,
				Stdout:   `{"status_code":200,"headers":{"Authorization":"Bearer LEAKED_VALUE"`,
			}, nil
		default:
			return sandbox.ExecResult{}, nil
		}
	})

	_, err := executeHTTPRequestTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"method":"GET","url":"https://api.example.com"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindNetwork}, AllowNetwork: true},
	})
	if err == nil {
		t.Fatalf("expected decode error, got nil")
	}
	if strings.Contains(err.Error(), "LEAKED_VALUE") {
		t.Fatalf("decode error leaked raw stdout bytes: %v", err)
	}
	if strings.Contains(err.Error(), "Authorization") {
		t.Fatalf("decode error leaked header name from stdout: %v", err)
	}
}

func TestHTTPRequestTool_StripsSensitiveResponseHeaders(t *testing.T) {
	session := sandbox.NewFakeSession("http-echo-auth")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch request.Command[0] {
		case "mkdir":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case "python3":
			// Simulate a server that echoes every header back — this is
			// the exact failure mode step 4 must defend against.
			return sandbox.ExecResult{
				ExitCode: 0,
				Stdout: `{"status_code":200,"headers":{` +
					`"Content-Type":"application/json",` +
					`"Authorization":"Bearer super-secret",` +
					`"Set-Cookie":"sid=abc123",` +
					`"X-API-Key":"leaked",` +
					`"x-auth-token":"leaked2",` +
					`"WWW-Authenticate":"Basic realm=\"x\"",` +
					`"X-Request-Id":"safe-opaque"` +
					`},"url":"https://api.example.com","body":"ok","body_bytes":2}`,
			}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})

	result, err := executeHTTPRequestTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"method":"GET","url":"https://api.example.com","headers":{"Authorization":"Bearer super-secret"}}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindNetwork}, AllowNetwork: true},
	})
	if err != nil {
		t.Fatalf("executeHTTPRequestTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}

	// Sensitive header values must not appear anywhere in the result
	// content that flows back to the LLM and into run_events.
	sensitiveValues := []string{"Bearer super-secret", "sid=abc123", "leaked", "leaked2"}
	for _, value := range sensitiveValues {
		if strings.Contains(result.Content, value) {
			t.Fatalf("tool result leaked %q: %s", value, result.Content)
		}
	}

	// Redaction marker should appear — proves the scrubber ran.
	if !strings.Contains(result.Content, redactedHeaderMarker) {
		t.Fatalf("expected %q marker in scrubbed response, got %s", redactedHeaderMarker, result.Content)
	}

	// Safe headers must survive.
	if !strings.Contains(result.Content, "safe-opaque") {
		t.Fatalf("expected non-sensitive X-Request-Id header to survive, got %s", result.Content)
	}
	if !strings.Contains(result.Content, "application/json") {
		t.Fatalf("expected Content-Type to survive, got %s", result.Content)
	}
}

func TestHTTPRequestTool_ScrubsRequestFileAfterExec(t *testing.T) {
	session := sandbox.NewFakeSession("http-scrub")
	var requestSnapshot []byte
	session.SetExecFunc(func(request sandbox.ExecRequest, files map[string][]byte) (sandbox.ExecResult, error) {
		switch request.Command[0] {
		case "mkdir":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case "python3":
			// Capture what the tool-inputs file looked like at exec time —
			// the assertions below prove the scrub runs AFTER this.
			requestSnapshot = append([]byte(nil), files[request.Command[2]]...)
			return sandbox.ExecResult{
				ExitCode: 0,
				Stdout:   `{"status_code":200,"headers":{},"url":"https://api.example.com","body":"ok","body_bytes":2}`,
			}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})

	const secretValue = "Bearer super-secret-token"
	argsBytes, _ := json.Marshal(map[string]any{
		"method":  "GET",
		"url":     "https://api.example.com",
		"headers": map[string]string{"Authorization": secretValue},
	})

	result, err := executeHTTPRequestTool(t.Context(), ToolExecutionRequest{
		Args:       argsBytes,
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindNetwork}, AllowNetwork: true},
	})
	if err != nil {
		t.Fatalf("executeHTTPRequestTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}

	// Sanity check: the secret WAS present in the request file at
	// exec time. Without this, the assertion below would be vacuous.
	if !bytes.Contains(requestSnapshot, []byte(secretValue)) {
		t.Fatalf("expected request snapshot to contain secret before scrub, got %q", string(requestSnapshot))
	}

	// After http_request returns, no file in the sandbox session may
	// still carry the plaintext secret. The agent's read_file primitive
	// walks the same backing store as session.Files().
	for path, content := range session.Files() {
		if bytes.Contains(content, []byte(secretValue)) {
			t.Fatalf("file %q still contains the secret after http_request returned: %q", path, string(content))
		}
	}
}
