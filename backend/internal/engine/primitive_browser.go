package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/sandbox"
)

const (
	browserToolTimeoutDefault = 60 * time.Second
	browserToolTimeoutMax     = 180 * time.Second
	browserAPIKeySecretName   = "BROWSER_USE_API_KEY"
)

func browserPrimitiveTools() map[string]Tool {
	return map[string]Tool{
		browserStartToolName: primitiveTool{
			name:        browserStartToolName,
			description: "Start an isolated Browser Use cloud browser for this run agent.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"timeout_minutes":{"type":"integer","minimum":1,"maximum":240},"proxy_country_code":{"type":"string"},"profile_name":{"type":"string"}},"additionalProperties":false}`),
			execute:     executeBrowserStartTool,
		},
		browserOpenToolName: primitiveTool{
			name:        browserOpenToolName,
			description: "Open a URL in the run agent's isolated browser session and wait for the page to load.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"},"timeout_seconds":{"type":"integer","minimum":1,"maximum":60}},"required":["url"],"additionalProperties":false}`),
			execute:     executeBrowserOpenTool,
		},
		browserInfoToolName: primitiveTool{
			name:        browserInfoToolName,
			description: "Return the current browser page URL, title, viewport, scroll, and page dimensions.",
			parameters:  json.RawMessage(`{"type":"object","additionalProperties":false}`),
			execute:     executeBrowserInfoTool,
		},
		browserScreenshotToolName: primitiveTool{
			name:        browserScreenshotToolName,
			description: "Capture a browser screenshot to a sandbox file path.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"full_page":{"type":"boolean"}},"additionalProperties":false}`),
			execute:     executeBrowserScreenshotTool,
		},
		browserClickToolName: primitiveTool{
			name:        browserClickToolName,
			description: "Click browser viewport coordinates.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"x":{"type":"number"},"y":{"type":"number"},"button":{"type":"string"},"clicks":{"type":"integer","minimum":1}},"required":["x","y"],"additionalProperties":false}`),
			execute:     executeBrowserClickTool,
		},
		browserTypeToolName: primitiveTool{
			name:        browserTypeToolName,
			description: "Type text into the focused browser element.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"],"additionalProperties":false}`),
			execute:     executeBrowserTypeTool,
		},
		browserKeyToolName: primitiveTool{
			name:        browserKeyToolName,
			description: "Press a key in the browser, optionally with CDP modifier bits.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"},"modifiers":{"type":"integer","minimum":0}},"required":["key"],"additionalProperties":false}`),
			execute:     executeBrowserKeyTool,
		},
		browserEvalToolName: primitiveTool{
			name:        browserEvalToolName,
			description: "Evaluate JavaScript in the current browser tab and return the JSON-serializable value.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string"}},"required":["expression"],"additionalProperties":false}`),
			execute:     executeBrowserEvalTool,
		},
		browserStopToolName: primitiveTool{
			name:        browserStopToolName,
			description: "Stop the run agent's Browser Use cloud browser session.",
			parameters:  json.RawMessage(`{"type":"object","additionalProperties":false}`),
			execute:     executeBrowserStopTool,
		},
	}
}

func executeBrowserStartTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	var args struct {
		TimeoutMinutes   int    `json:"timeout_minutes"`
		ProxyCountryCode string `json:"proxy_country_code"`
		ProfileName      string `json:"profile_name"`
	}
	if err := decodeToolArguments(browserStartToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	timeout := args.TimeoutMinutes
	if timeout <= 0 {
		timeout = 60
	}
	code := fmt.Sprintf(`
import json, os
from admin import daemon_alive, start_remote_daemon
name = os.environ.get("BU_NAME", "default")
payload = {"ok": True, "name": name, "already_running": daemon_alive(name)}
if not payload["already_running"]:
    browser = start_remote_daemon(name, timeout=%d, profileName=%s, proxyCountryCode=%s)
    payload["browser_id"] = browser.get("id")
    payload["live_url"] = browser.get("liveUrl")
print(json.dumps(payload))
`, timeout, pyLiteral(strings.TrimSpace(args.ProfileName)), pyLiteralOrDefault(strings.TrimSpace(args.ProxyCountryCode), "us"))
	return executeBrowserPython(ctx, request, browserStartToolName, code, false)
}

func executeBrowserOpenTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	var args struct {
		URL            string `json:"url"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := decodeToolArguments(browserOpenToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	if strings.TrimSpace(args.URL) == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("url is required"), IsError: true}, nil
	}
	timeout := args.TimeoutSeconds
	if timeout <= 0 {
		timeout = 15
	}
	code := fmt.Sprintf(`
import json
url = %s
loaded = wait_for_load(0.1)
new_tab(url)
loaded = wait_for_load(%d)
print(json.dumps({"ok": True, "loaded": loaded, "page": page_info()}))
`, pyLiteral(args.URL), timeout)
	return executeBrowserHarness(ctx, request, browserOpenToolName, code, true)
}

func executeBrowserInfoTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if err := decodeToolArguments(browserInfoToolName, request.Args, &struct{}{}); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	return executeBrowserHarness(ctx, request, browserInfoToolName, `import json
print(json.dumps({"ok": True, "page": page_info()}))
`, true)
}

func executeBrowserScreenshotTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	var args struct {
		Path     string `json:"path"`
		FullPage bool   `json:"full_page"`
	}
	if err := decodeToolArguments(browserScreenshotToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	screenshotPath := strings.TrimSpace(args.Path)
	if screenshotPath == "" {
		screenshotPath = "/workspace/browser-screenshot.png"
	}
	if _, err := validateSandboxPath(screenshotPath); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	code := fmt.Sprintf(`import json
path = capture_screenshot(%s, full=%t)
print(json.dumps({"ok": True, "path": path, "page": page_info()}))
`, pyLiteral(screenshotPath), args.FullPage)
	return executeBrowserHarness(ctx, request, browserScreenshotToolName, code, true)
}

func executeBrowserClickTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	var args struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Button string  `json:"button"`
		Clicks int     `json:"clicks"`
	}
	if err := decodeToolArguments(browserClickToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	if args.Clicks <= 0 {
		args.Clicks = 1
	}
	if strings.TrimSpace(args.Button) == "" {
		args.Button = "left"
	}
	code := fmt.Sprintf(`import json, time
click_at_xy(%f, %f, button=%s, clicks=%d)
time.sleep(0.25)
print(json.dumps({"ok": True, "page": page_info()}))
`, args.X, args.Y, pyLiteral(args.Button), args.Clicks)
	return executeBrowserHarness(ctx, request, browserClickToolName, code, true)
}

func executeBrowserTypeTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	var args struct {
		Text string `json:"text"`
	}
	if err := decodeToolArguments(browserTypeToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	code := fmt.Sprintf(`import json
type_text(%s)
print(json.dumps({"ok": True, "page": page_info()}))
`, pyLiteral(args.Text))
	return executeBrowserHarness(ctx, request, browserTypeToolName, code, true)
}

func executeBrowserKeyTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	var args struct {
		Key       string `json:"key"`
		Modifiers int    `json:"modifiers"`
	}
	if err := decodeToolArguments(browserKeyToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	if strings.TrimSpace(args.Key) == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("key is required"), IsError: true}, nil
	}
	code := fmt.Sprintf(`import json
press_key(%s, modifiers=%d)
print(json.dumps({"ok": True, "page": page_info()}))
`, pyLiteral(args.Key), args.Modifiers)
	return executeBrowserHarness(ctx, request, browserKeyToolName, code, true)
}

func executeBrowserEvalTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	var args struct {
		Expression string `json:"expression"`
	}
	if err := decodeToolArguments(browserEvalToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	if strings.TrimSpace(args.Expression) == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("expression is required"), IsError: true}, nil
	}
	code := fmt.Sprintf(`import json
value = js(%s)
print(json.dumps({"ok": True, "value": value, "page": page_info()}))
`, pyLiteral(args.Expression))
	return executeBrowserHarness(ctx, request, browserEvalToolName, code, true)
}

func executeBrowserStopTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if err := decodeToolArguments(browserStopToolName, request.Args, &struct{}{}); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	code := `import json, os
from admin import stop_remote_daemon
name = os.environ.get("BU_NAME", "default")
stop_remote_daemon(name)
print(json.dumps({"ok": True, "stopped": True, "name": name}))
`
	return executeBrowserPython(ctx, request, browserStopToolName, code, false)
}

func executeBrowserHarness(ctx context.Context, request ToolExecutionRequest, toolName string, code string, ensureRemote bool) (ToolExecutionResult, error) {
	return executeBrowserPython(ctx, request, toolName, browserShellScript(code, ensureRemote), true)
}

func executeBrowserPython(ctx context.Context, request ToolExecutionRequest, toolName string, code string, isShell bool) (ToolExecutionResult, error) {
	if !allowsBrowserTools(request.ToolPolicy) {
		return policyDeniedToolResult("browser tools are not allowed in this runtime"), nil
	}
	command := []string{"python3", "-c", code}
	if isShell {
		command = []string{"sh", "-lc", code}
	}
	result, err := request.Session.Exec(ctx, sandbox.ExecRequest{
		Command:          command,
		WorkingDirectory: defaultSandboxWorkingDirectory,
		Environment:      browserEnvironment(request),
		Timeout:          browserToolTimeoutDefault,
	})
	if err != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "execute browser harness", err)
	}
	if result.ExitCode != 0 {
		message := strings.TrimSpace(scrubStderrSecrets(result.Stderr))
		if message == "" {
			message = fmt.Sprintf("browser harness exited with code %d", result.ExitCode)
		}
		return ToolExecutionResult{Content: encodeToolErrorMessage(message), IsError: true}, nil
	}
	payload, ok := lastJSONObject(result.Stdout)
	if !ok {
		payload = map[string]any{
			"ok":     true,
			"stdout": strings.TrimSpace(result.Stdout),
		}
	}
	if okValue, _ := payload["ok"].(bool); !okValue {
		content, err := toolJSONOutput(ctx, request, toolName, payload)
		if err != nil {
			return ToolExecutionResult{}, err
		}
		return ToolExecutionResult{Content: content, IsError: true}, nil
	}
	content, err := toolJSONOutput(ctx, request, toolName, payload)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{Content: content}, nil
}

func browserShellScript(code string, ensureRemote bool) string {
	var builder strings.Builder
	if ensureRemote {
		builder.WriteString(`python3 <<'PY'
import os
from admin import daemon_alive, start_remote_daemon
name = os.environ.get("BU_NAME", "default")
if not daemon_alive(name):
    start_remote_daemon(name, timeout=60)
PY
`)
	}
	builder.WriteString("browser-harness <<'PY'\n")
	builder.WriteString(code)
	if !strings.HasSuffix(code, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString("PY\n")
	return builder.String()
}

func browserEnvironment(request ToolExecutionRequest) map[string]string {
	env := map[string]string{
		"BU_NAME": browserSessionName(request),
	}
	if key := strings.TrimSpace(request.WorkspaceSecrets[browserAPIKeySecretName]); key != "" {
		env[browserAPIKeySecretName] = key
	}
	return env
}

func browserSessionName(request ToolExecutionRequest) string {
	name := strings.TrimSpace(request.RunAgentID)
	if name == "" && request.Session != nil {
		name = request.Session.ID()
	}
	if name == "" {
		name = "default"
	}
	var builder strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('-')
		}
	}
	cleaned := strings.Trim(builder.String(), "-_")
	if cleaned == "" {
		return "default"
	}
	return cleaned
}

func pyLiteral(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

func pyLiteralOrDefault(value string, fallback string) string {
	if value == "" {
		value = fallback
	}
	return pyLiteral(value)
}

func lastJSONObject(stdout string) (map[string]any, bool) {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err == nil {
			return payload, true
		}
	}
	return nil, false
}
