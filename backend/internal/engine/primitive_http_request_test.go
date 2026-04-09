package engine

import (
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
