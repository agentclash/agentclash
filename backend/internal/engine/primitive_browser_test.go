package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/sandbox"
)

func TestBrowserTools_VisibleOnlyWhenBrowserKindAllowed(t *testing.T) {
	withoutBrowser, err := buildToolRegistry(sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}}, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildToolRegistry without browser returned error: %v", err)
	}
	if _, ok := withoutBrowser.Resolve(browserOpenToolName); ok {
		t.Fatalf("browser_open should not be visible without browser tool kind")
	}

	withBrowser, err := buildToolRegistry(sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindBrowser}}, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildToolRegistry with browser returned error: %v", err)
	}
	for _, name := range []string{browserStartToolName, browserOpenToolName, browserInfoToolName, browserScreenshotToolName, browserClickToolName, browserTypeToolName, browserKeyToolName, browserEvalToolName, browserStopToolName} {
		if _, ok := withBrowser.Resolve(name); !ok {
			t.Fatalf("%s should be visible with browser tool kind", name)
		}
	}
}

func TestBrowserOpenTool_ExecutesHarnessWithRunAgentNamespace(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-id")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		if request.Command[0] != "sh" {
			t.Fatalf("expected shell command, got %#v", request.Command)
		}
		if got := request.Environment["BU_NAME"]; got != "run-agent-123" {
			t.Fatalf("BU_NAME = %q, want run-agent-123", got)
		}
		if got := request.Environment[browserAPIKeySecretName]; got != "secret-key" {
			t.Fatalf("BROWSER_USE_API_KEY = %q, want secret-key", got)
		}
		if !strings.Contains(request.Command[2], "start_remote_daemon(name") {
			t.Fatalf("browser_open should ensure remote daemon before harness, command=%s", request.Command[2])
		}
		if !strings.Contains(request.Command[2], "new_tab(url)") {
			t.Fatalf("browser_open should navigate via browser-harness helpers, command=%s", request.Command[2])
		}
		return sandbox.ExecResult{
			ExitCode: 0,
			Stdout:   "https://live.browser-use.example\n{\"ok\":true,\"loaded\":true,\"page\":{\"url\":\"https://example.com\",\"title\":\"Example\"}}\n",
		}, nil
	})

	result, err := executeBrowserOpenTool(t.Context(), ToolExecutionRequest{
		Args:             json.RawMessage(`{"url":"https://example.com"}`),
		Session:          session,
		ToolPolicy:       sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindBrowser}},
		RunAgentID:       "run-agent-123",
		WorkspaceSecrets: map[string]string{browserAPIKeySecretName: "secret-key"},
	})
	if err != nil {
		t.Fatalf("executeBrowserOpenTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}
	if !strings.Contains(result.Content, `"url":"https://example.com"`) {
		t.Fatalf("expected page URL in structured content, got %s", result.Content)
	}
}

func TestBrowserStartTool_OmitsProfileNameAsPythonNone(t *testing.T) {
	session := sandbox.NewFakeSession("browser-start")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: `{"ok":true}`})

	result, err := executeBrowserStartTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindBrowser}},
		RunAgentID: "run-agent-123",
	})
	if err != nil {
		t.Fatalf("executeBrowserStartTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}
	calls := session.ExecCalls()
	if len(calls) != 1 {
		t.Fatalf("exec call count = %d, want 1", len(calls))
	}
	if !strings.Contains(calls[0].Command[2], "profileName=None") {
		t.Fatalf("expected omitted profile_name to be Python None, command=%s", calls[0].Command[2])
	}
}

func TestBrowserScreenshotTool_DefaultPathIsUnique(t *testing.T) {
	session := sandbox.NewFakeSession("browser-screenshot")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: `{"ok":true}`})
	request := ToolExecutionRequest{
		Args:       json.RawMessage(`{}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindBrowser}},
		RunAgentID: "run-agent-123",
	}

	if _, err := executeBrowserScreenshotTool(t.Context(), request); err != nil {
		t.Fatalf("first executeBrowserScreenshotTool returned error: %v", err)
	}
	if _, err := executeBrowserScreenshotTool(t.Context(), request); err != nil {
		t.Fatalf("second executeBrowserScreenshotTool returned error: %v", err)
	}
	calls := session.ExecCalls()
	if len(calls) != 2 {
		t.Fatalf("exec call count = %d, want 2", len(calls))
	}
	if calls[0].Command[2] == calls[1].Command[2] {
		t.Fatalf("default screenshot commands reused the same path: %s", calls[0].Command[2])
	}
	if !strings.Contains(calls[0].Command[2], "/workspace/browser-screenshot-") || !strings.Contains(calls[1].Command[2], "/workspace/browser-screenshot-") {
		t.Fatalf("default screenshot commands should use unique browser-screenshot paths: %#v", calls)
	}
}

func TestBrowserOpenTool_DeniedWithoutBrowserKind(t *testing.T) {
	session := sandbox.NewFakeSession("browser-denied")
	session.SetExecFunc(func(sandbox.ExecRequest, map[string][]byte) (sandbox.ExecResult, error) {
		t.Fatal("browser tool should not execute when policy denies browser kind")
		return sandbox.ExecResult{}, nil
	})

	result, err := executeBrowserOpenTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"url":"https://example.com"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("executeBrowserOpenTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected policy denial, got %#v", result)
	}
	if !strings.Contains(result.Content, "browser tools are not allowed") {
		t.Fatalf("denial content = %s", result.Content)
	}
}

func TestBrowserTool_ScrubsRawBrowserAPIKeyFromStderr(t *testing.T) {
	session := sandbox.NewFakeSession("browser-secret-stderr")
	session.SetExecResult(sandbox.ExecResult{
		ExitCode: 1,
		Stderr:   "Traceback: request failed for key bu_secret_value",
	})

	result, err := executeBrowserInfoTool(t.Context(), ToolExecutionRequest{
		Session:          session,
		ToolPolicy:       sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindBrowser}},
		RunAgentID:       "run-agent-123",
		WorkspaceSecrets: map[string]string{browserAPIKeySecretName: "bu_secret_value"},
	})
	if err != nil {
		t.Fatalf("executeBrowserInfoTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error")
	}
	if strings.Contains(result.Content, "bu_secret_value") {
		t.Fatalf("raw browser API key leaked in content: %s", result.Content)
	}
	if !strings.Contains(result.Content, redactedHeaderMarker) {
		t.Fatalf("expected redaction marker in content: %s", result.Content)
	}
}

func TestBrowserEvalTool_ReturnsStructuredResult(t *testing.T) {
	session := sandbox.NewFakeSession("browser-eval")
	session.SetExecResult(sandbox.ExecResult{
		ExitCode: 0,
		Stdout:   "{\"ok\":true,\"value\":\"AgentClash\",\"page\":{\"url\":\"https://example.com\"}}\n",
	})

	result, err := executeBrowserEvalTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"expression":"document.title"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindBrowser}},
		RunAgentID: "run-agent-123",
	})
	if err != nil {
		t.Fatalf("executeBrowserEvalTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}
	if !strings.Contains(result.Content, `"value":"AgentClash"`) {
		t.Fatalf("expected eval value in result, got %s", result.Content)
	}
}
