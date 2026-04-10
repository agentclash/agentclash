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
