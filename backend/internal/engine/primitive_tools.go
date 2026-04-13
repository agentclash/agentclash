package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

func nativePrimitiveTools(toolPolicy sandbox.ToolPolicy) map[string]Tool {
	tools := map[string]Tool{
		submitToolName: primitiveTool{
			name:        submitToolName,
			description: "Submit your final answer for the benchmark when you are finished.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`),
			execute:     executeSubmitTool,
		},
	}

	if allowsFileTools(toolPolicy) {
		tools[readFileToolName] = primitiveTool{
			name:        readFileToolName,
			description: "Read a file from the sandbox workspace.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`),
			execute:     executeReadFileTool,
		}
		tools[writeFileToolName] = primitiveTool{
			name:        writeFileToolName,
			description: "Write text content to a file in the sandbox workspace.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"],"additionalProperties":false}`),
			execute:     executeWriteFileTool,
		}
		tools[listFilesToolName] = primitiveTool{
			name:        listFilesToolName,
			description: "List files in the sandbox workspace under an optional path prefix.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"prefix":{"type":"string"}},"additionalProperties":false}`),
			execute:     executeListFilesTool,
		}
		tools[searchFilesToolName] = primitiveTool{
			name:        searchFilesToolName,
			description: "Search for files in the sandbox workspace by name or glob pattern.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"},"max_results":{"type":"integer","minimum":1}},"required":["pattern"],"additionalProperties":false}`),
			execute:     executeSearchFilesTool,
		}
		tools[searchTextToolName] = primitiveTool{
			name:        searchTextToolName,
			description: "Search file contents in the sandbox workspace using a regex pattern.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"},"include":{"type":"string"},"case_sensitive":{"type":"boolean"},"max_results":{"type":"integer","minimum":1}},"required":["pattern"],"additionalProperties":false}`),
			execute:     executeSearchTextTool,
		}
	}

	if allowsDataTools(toolPolicy) {
		tools[queryJSONToolName] = primitiveTool{
			name:        queryJSONToolName,
			description: "Query JSON from a file or inline JSON string using jq.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"file_path":{"type":"string"},"json":{"type":"string"},"output_path":{"type":"string"}},"required":["query"],"additionalProperties":false}`),
			execute:     executeQueryJSONTool,
		}
		tools[querySQLToolName] = primitiveTool{
			name:        querySQLToolName,
			description: "Run a SQL query against a supported database engine. Day-1 support is SQLite only.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"engine":{"type":"string"},"query":{"type":"string"},"database_path":{"type":"string"},"output_path":{"type":"string"}},"required":["engine","query"],"additionalProperties":false}`),
			execute:     executeQuerySQLTool,
		}
	}

	if allowsNetworkTools(toolPolicy) {
		tools[httpRequestToolName] = primitiveTool{
			name:        httpRequestToolName,
			description: "Make an HTTP request from inside the sandbox with structured response output.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"method":{"type":"string"},"url":{"type":"string"},"headers":{"type":"object","additionalProperties":{"type":"string"}},"body":{"type":"string"},"timeout_seconds":{"type":"integer","minimum":1},"output_path":{"type":"string"}},"required":["method","url"],"additionalProperties":false}`),
			execute:     executeHTTPRequestTool,
		}
	}

	if allowsBuildTools(toolPolicy) {
		tools[runTestsToolName] = primitiveTool{
			name:        runTestsToolName,
			description: "Run project tests in the sandbox workspace using an explicit or auto-detected command.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"},"minItems":1}]},"working_directory":{"type":"string"},"environment":{"type":"object","additionalProperties":{"type":"string"}},"timeout_seconds":{"type":"integer","minimum":1}},"additionalProperties":false}`),
			execute:     executeRunTestsTool,
		}
		tools[buildToolName] = primitiveTool{
			name:        buildToolName,
			description: "Build the project in the sandbox workspace using an explicit or auto-detected command.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"},"minItems":1}]},"working_directory":{"type":"string"},"environment":{"type":"object","additionalProperties":{"type":"string"}},"timeout_seconds":{"type":"integer","minimum":1}},"additionalProperties":false}`),
			execute:     executeBuildTool,
		}
	}

	if toolPolicy.AllowShell {
		tools[execToolName] = primitiveTool{
			name:        execToolName,
			description: "Execute a shell command inside the sandbox workspace.",
			parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"array","items":{"type":"string"},"minItems":1},"working_directory":{"type":"string"},"environment":{"type":"object","additionalProperties":{"type":"string"}}},"required":["command"],"additionalProperties":false}`),
			execute:     executeExecTool,
		}
	}

	return tools
}

type primitiveTool struct {
	name        string
	description string
	parameters  json.RawMessage
	execute     func(context.Context, ToolExecutionRequest) (ToolExecutionResult, error)
}

func (t primitiveTool) Name() string {
	return t.name
}

func (t primitiveTool) Description() string {
	return t.description
}

func (t primitiveTool) Parameters() json.RawMessage {
	return cloneJSON(t.parameters)
}

func (t primitiveTool) Category() ToolCategory {
	return ToolCategoryPrimitive
}

func (t primitiveTool) Execute(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	return t.execute(ctx, request)
}

func executeSubmitTool(_ context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	var args struct {
		Answer string `json:"answer"`
	}
	if err := decodeToolArguments(submitToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	if strings.TrimSpace(args.Answer) == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("answer is required"), IsError: true}, nil
	}
	return ToolExecutionResult{
		Content:     `{"submitted":true}`,
		Completed:   true,
		FinalOutput: args.Answer,
	}, nil
}

// validateSandboxPath ensures the given path resolves to a location within the
// sandbox workspace root. Relative paths are resolved against the root. Returns
// the cleaned absolute path or an error if the path escapes the workspace.
func validateSandboxPath(rawPath string) (string, error) {
	cleaned := rawPath
	if !path.IsAbs(cleaned) {
		cleaned = path.Join(defaultSandboxWorkingDirectory, cleaned)
	}
	cleaned = path.Clean(cleaned)
	// The cleaned path must be exactly the root or start with root + "/".
	if cleaned != defaultSandboxWorkingDirectory && !strings.HasPrefix(cleaned, defaultSandboxWorkingDirectory+"/") {
		return "", fmt.Errorf("path %q is outside the sandbox workspace", rawPath)
	}
	return cleaned, nil
}

func executeReadFileTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if !allowsFileTools(request.ToolPolicy) {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Path string `json:"path"`
	}
	if err := decodeToolArguments(readFileToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	safePath, err := validateSandboxPath(args.Path)
	if err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}

	content, err := request.Session.ReadFile(ctx, safePath)
	if err != nil {
		if errors.Is(err, sandbox.ErrFileNotFound) {
			return ToolExecutionResult{Content: encodeToolErrorMessage(fmt.Sprintf("file %q was not found", strings.TrimSpace(args.Path))), IsError: true}, nil
		}
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "read sandbox file", err)
	}

	payload, err := json.Marshal(map[string]any{
		"path":    strings.TrimSpace(args.Path),
		"content": string(content),
	})
	if err != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "marshal read_file result", err)
	}

	return ToolExecutionResult{Content: string(payload)}, nil
}

func executeWriteFileTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if !allowsFileTools(request.ToolPolicy) {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeToolArguments(writeFileToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	safePath, err := validateSandboxPath(args.Path)
	if err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	if err := request.Session.WriteFile(ctx, safePath, []byte(args.Content)); err != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "write sandbox file", err)
	}

	payload, err := json.Marshal(map[string]any{
		"path":    strings.TrimSpace(args.Path),
		"written": true,
		"bytes":   len(args.Content),
	})
	if err != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "marshal write_file result", err)
	}

	return ToolExecutionResult{Content: string(payload)}, nil
}

func executeListFilesTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if !allowsFileTools(request.ToolPolicy) {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Prefix string `json:"prefix"`
	}
	if err := decodeToolArguments(listFilesToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	safePrefix, err := validateSandboxPath(args.Prefix)
	if err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	files, err := request.Session.ListFiles(ctx, safePrefix)
	if err != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "list sandbox files", err)
	}

	payload, err := json.Marshal(map[string]any{
		"prefix": strings.TrimSpace(args.Prefix),
		"files":  files,
	})
	if err != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "marshal list_files result", err)
	}

	return ToolExecutionResult{Content: string(payload)}, nil
}

func executeSearchFilesTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if !allowsFileTools(request.ToolPolicy) {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Pattern    string `json:"pattern"`
		Path       string `json:"path"`
		MaxResults int    `json:"max_results"`
	}
	if err := decodeToolArguments(searchFilesToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("pattern is required"), IsError: true}, nil
	}
	searchPath := strings.TrimSpace(args.Path)
	if searchPath == "" {
		searchPath = defaultSandboxWorkingDirectory
	}
	if safePath, err := validateSandboxPath(searchPath); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	} else {
		searchPath = safePath
	}
	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	commandResult, err := executeInternalCommand(ctx, request, searchFilesToolName, sandbox.ExecRequest{
		Command: []string{
			"sh", "-lc",
			"find \"$1\" -type f -name \"$2\" | head -n \"$3\"",
			"sh",
			searchPath,
			strings.TrimSpace(args.Pattern),
			fmt.Sprintf("%d", maxResults),
		},
	}, commandBehavior{})
	if err != nil {
		return ToolExecutionResult{}, err
	}

	files := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(commandResult.ExecResult.Stdout), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		files = append(files, trimmed)
	}

	content, err := toolJSONOutput(ctx, request, searchFilesToolName, map[string]any{
		"pattern":     strings.TrimSpace(args.Pattern),
		"path":        searchPath,
		"max_results": maxResults,
		"files":       files,
	})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{Content: content}, nil
}

func executeSearchTextTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if !allowsFileTools(request.ToolPolicy) {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Pattern       string `json:"pattern"`
		Path          string `json:"path"`
		Include       string `json:"include"`
		CaseSensitive *bool  `json:"case_sensitive"`
		MaxResults    int    `json:"max_results"`
	}
	if err := decodeToolArguments(searchTextToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("pattern is required"), IsError: true}, nil
	}

	searchPath := strings.TrimSpace(args.Path)
	if searchPath == "" {
		searchPath = defaultSandboxWorkingDirectory
	}
	safePath, err := validateSandboxPath(searchPath)
	if err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	searchPath = safePath
	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = 200
	}

	command := []string{
		"rg",
		"--json",
		"--line-number",
		"--color", "never",
		"--max-count", fmt.Sprintf("%d", maxResults),
	}
	if args.CaseSensitive == nil || *args.CaseSensitive {
		command = append(command, "--case-sensitive")
	} else {
		command = append(command, "-i")
	}
	if include := strings.TrimSpace(args.Include); include != "" {
		command = append(command, "-g", include)
	}
	command = append(command, strings.TrimSpace(args.Pattern), searchPath)

	commandResult, err := executeInternalCommand(ctx, request, searchTextToolName, sandbox.ExecRequest{
		Command: command,
	}, commandBehavior{EmptyResultExitCodes: []int{1}})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	if commandResult.IsError {
		return ToolExecutionResult{Content: encodeToolErrorMessage(strings.TrimSpace(commandResult.ExecResult.Stderr)), IsError: true}, nil
	}

	matches, parseErr := parseRipgrepMatches(commandResult.ExecResult.Stdout)
	if parseErr != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "parse ripgrep output", parseErr)
	}

	caseSensitive := true
	if args.CaseSensitive != nil {
		caseSensitive = *args.CaseSensitive
	}
	content, err := toolJSONOutput(ctx, request, searchTextToolName, map[string]any{
		"pattern":        strings.TrimSpace(args.Pattern),
		"path":           searchPath,
		"include":        strings.TrimSpace(args.Include),
		"case_sensitive": caseSensitive,
		"max_results":    maxResults,
		"matches":        matches,
	})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{Content: content}, nil
}

type ripgrepMatch struct {
	Path       string `json:"path"`
	LineNumber int64  `json:"line_number"`
	LineText   string `json:"line_text"`
}

type detectedCommand struct {
	Framework string
	Command   []string
	Label     string
}

func parseRipgrepMatches(stdout string) ([]ripgrepMatch, error) {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	matches := make([]ripgrepMatch, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var event struct {
			Type string `json:"type"`
			Data struct {
				Path struct {
					Text string `json:"text"`
				} `json:"path"`
				LineNumber int64 `json:"line_number"`
				Lines      struct {
					Text string `json:"text"`
				} `json:"lines"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(trimmed), &event); err != nil {
			return nil, err
		}
		if event.Type != "match" {
			continue
		}
		matches = append(matches, ripgrepMatch{
			Path:       strings.TrimSpace(event.Data.Path.Text),
			LineNumber: event.Data.LineNumber,
			LineText:   strings.TrimRight(event.Data.Lines.Text, "\n"),
		})
	}
	return matches, nil
}

func executeQueryJSONTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if !allowsDataTools(request.ToolPolicy) {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Query      string `json:"query"`
		FilePath   string `json:"file_path"`
		JSON       string `json:"json"`
		OutputPath string `json:"output_path"`
	}
	if err := decodeToolArguments(queryJSONToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("query is required"), IsError: true}, nil
	}
	filePath := strings.TrimSpace(args.FilePath)
	inlineJSON := strings.TrimSpace(args.JSON)
	if (filePath == "" && inlineJSON == "") || (filePath != "" && inlineJSON != "") {
		return ToolExecutionResult{Content: encodeToolErrorMessage("provide exactly one of file_path or json"), IsError: true}, nil
	}

	var execRequest sandbox.ExecRequest
	if filePath != "" {
		execRequest = sandbox.ExecRequest{
			Command: []string{"jq", "-c", query, filePath},
		}
	} else {
		execRequest = sandbox.ExecRequest{
			Command: []string{"sh", "-lc", "printf '%s' \"$1\" | jq -c \"$2\"", "sh", inlineJSON, query},
		}
	}

	commandResult, err := executeInternalCommand(ctx, request, queryJSONToolName, execRequest, commandBehavior{})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	if commandResult.IsError {
		message := strings.TrimSpace(commandResult.ExecResult.Stderr)
		if message == "" {
			message = "jq query failed"
		}
		return ToolExecutionResult{Content: encodeToolErrorMessage(message), IsError: true}, nil
	}

	outputPath := strings.TrimSpace(args.OutputPath)
	if outputPath != "" {
		if err := request.Session.WriteFile(ctx, outputPath, []byte(commandResult.ExecResult.Stdout)); err != nil {
			return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "write query_json output file", err)
		}
		content, err := toolJSONOutput(ctx, request, queryJSONToolName, map[string]any{
			"query":       query,
			"output_path": outputPath,
			"written":     true,
			"total_bytes": len(commandResult.ExecResult.Stdout),
		})
		if err != nil {
			return ToolExecutionResult{}, err
		}
		return ToolExecutionResult{Content: content}, nil
	}

	resultValue := parseJQOutput(commandResult.ExecResult.Stdout)
	content, err := toolJSONOutput(ctx, request, queryJSONToolName, map[string]any{
		"query":  query,
		"result": resultValue,
	})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{Content: content}, nil
}

func parseJQOutput(stdout string) any {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	values := make([]any, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			values = append(values, trimmed)
			continue
		}
		values = append(values, decoded)
	}
	if len(values) == 0 {
		return nil
	}
	if len(values) == 1 {
		return values[0]
	}
	return values
}

func executeHTTPRequestTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if !allowsNetworkTools(request.ToolPolicy) || !request.ToolPolicy.AllowNetwork {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Method         string            `json:"method"`
		URL            string            `json:"url"`
		Headers        map[string]string `json:"headers"`
		Body           string            `json:"body"`
		TimeoutSeconds int               `json:"timeout_seconds"`
		OutputPath     string            `json:"output_path"`
	}
	if err := decodeToolArguments(httpRequestToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	method := strings.ToUpper(strings.TrimSpace(args.Method))
	if method == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("method is required"), IsError: true}, nil
	}
	url := strings.TrimSpace(args.URL)
	if url == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("url is required"), IsError: true}, nil
	}
	if len(args.Body) > httpRequestBodyLimitBytes {
		return ToolExecutionResult{Content: encodeToolErrorMessage("request body exceeds size limit"), IsError: true}, nil
	}
	timeoutSeconds := args.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = httpRequestTimeoutSecondsDefault
	}
	if timeoutSeconds > httpRequestTimeoutSecondsMax {
		timeoutSeconds = httpRequestTimeoutSecondsMax
	}

	if err := ensureToolDirectory(ctx, request, toolInputDirectory); err != nil {
		return ToolExecutionResult{}, err
	}

	requestPath := path.Join(toolInputDirectory, fmt.Sprintf("%s_%s.json", httpRequestToolName, uuid.NewString()))
	inputPayload, err := json.Marshal(map[string]any{
		"method":                  method,
		"url":                     url,
		"headers":                 cloneStringMap(args.Headers),
		"body":                    args.Body,
		"timeout_seconds":         timeoutSeconds,
		"output_path":             strings.TrimSpace(args.OutputPath),
		"network_allowlist":       append([]string(nil), request.NetworkAllowlist...),
		"max_request_body_bytes":  httpRequestBodyLimitBytes,
		"max_response_body_bytes": httpResponseLimitBytes,
	})
	if err != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "marshal http_request input", err)
	}
	if err := request.Session.WriteFile(ctx, requestPath, inputPayload); err != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "write http_request input", err)
	}
	// Scrub the request file as soon as the python helper has consumed
	// it. The file carries resolved ${secrets.*} headers and lives
	// under /workspace, which the agent's read_file primitive can
	// reach. The engine loop is strictly serial (a single tool call
	// completes before the agent sees the result and can issue the
	// next call), so overwriting after executeInternalCommand returns
	// closes the window before the agent ever has a chance to look.
	// A detached context keeps the cleanup running even when the
	// request context was cancelled — the session is still alive
	// until prepareSandbox tears it down. See issue #186.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if scrubErr := request.Session.WriteFile(cleanupCtx, requestPath, nil); scrubErr != nil {
			slog.Default().Warn("failed to scrub http_request input file after exec; plaintext secret may persist in sandbox",
				"path", requestPath,
				"error", scrubErr,
			)
		}
	}()

	commandResult, err := executeInternalCommand(ctx, request, httpRequestToolName, sandbox.ExecRequest{
		Command: []string{"python3", "/tools/http_request.py", requestPath},
	}, commandBehavior{})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	if commandResult.IsError {
		// Defense-in-depth: even though http_request.py #186 step 5
		// wraps its httpx call in try/except with type-only error
		// formatting, a pack pinned to an older sandbox template may
		// still ship the pre-#186 script and emit a raw traceback
		// that includes resolved request headers. Scrub any
		// auth-shaped stderr fragment before it becomes a tool error.
		message := strings.TrimSpace(scrubStderrSecrets(commandResult.ExecResult.Stderr))
		if message == "" {
			message = "http request failed"
		}
		return ToolExecutionResult{Content: encodeToolErrorMessage(message), IsError: true}, nil
	}

	var responsePayload any
	if err := json.Unmarshal([]byte(strings.TrimSpace(commandResult.ExecResult.Stdout)), &responsePayload); err != nil {
		// Deliberately drop the json.Unmarshal error: its message
		// quotes a slice of the malformed input, which could include
		// unscrubbed response headers (Authorization echoed by a
		// misbehaving server). A generic error avoids leaking any
		// stdout bytes into the NewFailure cause chain. See #186.
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "decode http_request output", errors.New("malformed response payload"))
	}
	// Strip well-known authentication headers from the response before
	// it flows into the LLM context and the run_events table. Some APIs
	// echo the Authorization / Cookie header back verbatim (for debug or
	// by accident); without scrubbing, a composed tool that authenticates
	// with ${secrets.X} would leak the plaintext back to the agent.
	// NOTE: response BODY scrubbing is deliberately out of scope — body
	// content is structured, pack-specific, and can't be safely stripped
	// without domain knowledge. See issue #186 for the full threat model.
	scrubSensitiveResponseHeaders(responsePayload)
	content, err := toolJSONOutput(ctx, request, httpRequestToolName, responsePayload)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{Content: content}, nil
}

func executeRunTestsTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	return executeBuildLikeTool(ctx, request, runTestsToolName, "test")
}

func executeBuildTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	return executeBuildLikeTool(ctx, request, buildToolName, "build")
}

func executeBuildLikeTool(ctx context.Context, request ToolExecutionRequest, toolName string, mode string) (ToolExecutionResult, error) {
	if !allowsBuildTools(request.ToolPolicy) {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Command          json.RawMessage   `json:"command"`
		WorkingDirectory string            `json:"working_directory"`
		Environment      map[string]string `json:"environment"`
		TimeoutSeconds   int               `json:"timeout_seconds"`
	}
	if err := decodeToolArguments(toolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}

	command, commandLabel, framework, err := resolveBuildLikeCommand(ctx, request, mode, args.Command)
	if err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}

	workingDirectory := strings.TrimSpace(args.WorkingDirectory)
	if workingDirectory == "" {
		workingDirectory = defaultSandboxWorkingDirectory
	}
	timeoutSeconds := args.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = buildToolTimeoutSecondsDefault
	}
	if timeoutSeconds > buildToolTimeoutSecondsMax {
		timeoutSeconds = buildToolTimeoutSecondsMax
	}

	commandResult, err := executeInternalCommand(ctx, request, toolName, sandbox.ExecRequest{
		Command:          command,
		WorkingDirectory: workingDirectory,
		Environment:      cloneStringMap(args.Environment),
		Timeout:          time.Duration(timeoutSeconds) * time.Second,
	}, commandBehavior{})
	if err != nil {
		return ToolExecutionResult{}, err
	}

	content, marshalErr := toolJSONOutput(ctx, request, toolName, map[string]any{
		"framework":         framework,
		"mode":              mode,
		"command":           commandLabel,
		"working_directory": workingDirectory,
		"exit_code":         commandResult.ExecResult.ExitCode,
		"stdout":            commandResult.ExecResult.Stdout,
		"stderr":            commandResult.ExecResult.Stderr,
		"summary": map[string]any{
			"status": map[bool]string{true: "failed", false: "completed"}[commandResult.IsError],
		},
	})
	if marshalErr != nil {
		return ToolExecutionResult{}, marshalErr
	}
	return ToolExecutionResult{Content: content, IsError: commandResult.IsError}, nil
}

func resolveBuildLikeCommand(ctx context.Context, request ToolExecutionRequest, mode string, raw json.RawMessage) ([]string, string, string, error) {
	if command, label, ok, err := parseCommandOverride(raw); err != nil {
		return nil, "", "", err
	} else if ok {
		return command, label, "custom", nil
	}

	detectors := []func(context.Context, ToolExecutionRequest, string) (detectedCommand, bool, error){
		detectPackageJSONCommand,
		detectGoCommand,
		detectCargoCommand,
		detectPyProjectCommand,
		detectMakeCommand,
	}
	for _, detector := range detectors {
		detected, ok, err := detector(ctx, request, mode)
		if err != nil {
			return nil, "", "", err
		}
		if ok {
			return detected.Command, detected.Label, detected.Framework, nil
		}
	}

	return nil, "", "", fmt.Errorf("could not auto-detect a %s command; provide an explicit command", mode)
}

func parseCommandOverride(raw json.RawMessage) ([]string, string, bool, error) {
	if len(raw) == 0 {
		return nil, "", false, nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		trimmed := strings.TrimSpace(asString)
		if trimmed == "" {
			return nil, "", false, nil
		}
		return []string{"sh", "-lc", trimmed}, trimmed, true, nil
	}

	var asArray []string
	if err := json.Unmarshal(raw, &asArray); err == nil {
		normalized := normalizeStrings(asArray)
		if len(normalized) == 0 {
			return nil, "", false, nil
		}
		return normalized, strings.Join(normalized, " "), true, nil
	}
	return nil, "", false, fmt.Errorf("command override must be a string or string array")
}

func detectPackageJSONCommand(ctx context.Context, request ToolExecutionRequest, mode string) (detectedCommand, bool, error) {
	content, err := request.Session.ReadFile(ctx, path.Join(defaultSandboxWorkingDirectory, "package.json"))
	if err != nil {
		if errors.Is(err, sandbox.ErrFileNotFound) {
			return detectedCommand{}, false, nil
		}
		return detectedCommand{}, false, err
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return detectedCommand{}, false, nil
	}
	script, ok := pkg.Scripts[mode]
	if !ok || strings.TrimSpace(script) == "" {
		return detectedCommand{}, false, nil
	}
	return detectedCommand{
		Framework: "npm",
		Command:   []string{"npm", "run", mode},
		Label:     "npm run " + mode,
	}, true, nil
}

func detectGoCommand(ctx context.Context, request ToolExecutionRequest, mode string) (detectedCommand, bool, error) {
	_, err := request.Session.ReadFile(ctx, path.Join(defaultSandboxWorkingDirectory, "go.mod"))
	if err != nil {
		if errors.Is(err, sandbox.ErrFileNotFound) {
			return detectedCommand{}, false, nil
		}
		return detectedCommand{}, false, err
	}
	if mode == "test" {
		return detectedCommand{Framework: "go", Command: []string{"go", "test", "./..."}, Label: "go test ./..."}, true, nil
	}
	return detectedCommand{Framework: "go", Command: []string{"go", "build", "./..."}, Label: "go build ./..."}, true, nil
}

func detectCargoCommand(ctx context.Context, request ToolExecutionRequest, mode string) (detectedCommand, bool, error) {
	_, err := request.Session.ReadFile(ctx, path.Join(defaultSandboxWorkingDirectory, "Cargo.toml"))
	if err != nil {
		if errors.Is(err, sandbox.ErrFileNotFound) {
			return detectedCommand{}, false, nil
		}
		return detectedCommand{}, false, err
	}
	if mode == "test" {
		return detectedCommand{Framework: "cargo", Command: []string{"cargo", "test"}, Label: "cargo test"}, true, nil
	}
	return detectedCommand{Framework: "cargo", Command: []string{"cargo", "build"}, Label: "cargo build"}, true, nil
}

func detectPyProjectCommand(ctx context.Context, request ToolExecutionRequest, mode string) (detectedCommand, bool, error) {
	_, err := request.Session.ReadFile(ctx, path.Join(defaultSandboxWorkingDirectory, "pyproject.toml"))
	if err != nil {
		if errors.Is(err, sandbox.ErrFileNotFound) {
			return detectedCommand{}, false, nil
		}
		return detectedCommand{}, false, err
	}
	if mode == "test" {
		return detectedCommand{Framework: "python", Command: []string{"python3", "-m", "unittest", "discover"}, Label: "python3 -m unittest discover"}, true, nil
	}
	return detectedCommand{Framework: "python", Command: []string{"python3", "-m", "compileall", "."}, Label: "python3 -m compileall ."}, true, nil
}

func detectMakeCommand(ctx context.Context, request ToolExecutionRequest, mode string) (detectedCommand, bool, error) {
	_, err := request.Session.ReadFile(ctx, path.Join(defaultSandboxWorkingDirectory, "Makefile"))
	if err != nil {
		if errors.Is(err, sandbox.ErrFileNotFound) {
			return detectedCommand{}, false, nil
		}
		return detectedCommand{}, false, err
	}
	return detectedCommand{
		Framework: "make",
		Command:   []string{"make", mode},
		Label:     "make " + mode,
	}, true, nil
}

func executeQuerySQLTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if !allowsDataTools(request.ToolPolicy) {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Engine       string `json:"engine"`
		Query        string `json:"query"`
		DatabasePath string `json:"database_path"`
		OutputPath   string `json:"output_path"`
	}
	if err := decodeToolArguments(querySQLToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}

	engine := strings.TrimSpace(args.Engine)
	if !strings.EqualFold(engine, "sqlite") {
		return ToolExecutionResult{Content: encodeToolErrorMessage(fmt.Sprintf("engine %q is not supported yet; day-1 support is sqlite only", engine)), IsError: true}, nil
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("query is required"), IsError: true}, nil
	}
	databasePath := strings.TrimSpace(args.DatabasePath)
	if databasePath == "" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("database_path is required for sqlite"), IsError: true}, nil
	}

	commandResult, err := executeInternalCommand(ctx, request, querySQLToolName, sandbox.ExecRequest{
		Command: []string{"sqlite3", "-json", databasePath, query},
	}, commandBehavior{})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	if commandResult.IsError {
		message := strings.TrimSpace(commandResult.ExecResult.Stderr)
		if message == "" {
			message = "sql query failed"
		}
		return ToolExecutionResult{Content: encodeToolErrorMessage(message), IsError: true}, nil
	}

	outputPath := strings.TrimSpace(args.OutputPath)
	if outputPath != "" {
		if err := request.Session.WriteFile(ctx, outputPath, []byte(commandResult.ExecResult.Stdout)); err != nil {
			return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "write query_sql output file", err)
		}
		content, err := toolJSONOutput(ctx, request, querySQLToolName, map[string]any{
			"engine":      "sqlite",
			"query":       query,
			"output_path": outputPath,
			"written":     true,
			"total_bytes": len(commandResult.ExecResult.Stdout),
		})
		if err != nil {
			return ToolExecutionResult{}, err
		}
		return ToolExecutionResult{Content: content}, nil
	}

	var rows any
	trimmed := strings.TrimSpace(commandResult.ExecResult.Stdout)
	if trimmed == "" {
		rows = []any{}
	} else if err := json.Unmarshal([]byte(trimmed), &rows); err != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "decode sqlite json output", err)
	}

	content, err := toolJSONOutput(ctx, request, querySQLToolName, map[string]any{
		"engine": "sqlite",
		"query":  query,
		"rows":   rows,
	})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{Content: content}, nil
}

func executeExecTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if !request.ToolPolicy.AllowShell {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	var args struct {
		Command          []string          `json:"command"`
		WorkingDirectory string            `json:"working_directory,omitempty"`
		Environment      map[string]string `json:"environment,omitempty"`
	}
	if err := decodeToolArguments(execToolName, request.Args, &args); err != nil {
		return ToolExecutionResult{Content: encodeToolErrorMessage(err.Error()), IsError: true}, nil
	}
	if len(args.Command) == 0 {
		return ToolExecutionResult{Content: encodeToolErrorMessage("command must contain at least one element"), IsError: true}, nil
	}

	commandResult, err := executeInternalCommand(ctx, request, execToolName, sandbox.ExecRequest{
		Command:          append([]string(nil), args.Command...),
		WorkingDirectory: strings.TrimSpace(args.WorkingDirectory),
		Environment:      cloneStringMap(args.Environment),
	}, commandBehavior{})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	if commandResult.Classification == "policy" {
		return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
	}

	payload, marshalErr := json.Marshal(commandResult.ExecResult)
	if marshalErr != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "marshal exec result", marshalErr)
	}
	if commandResult.IsError {
		return ToolExecutionResult{Content: string(payload), IsError: true}, nil
	}

	return ToolExecutionResult{Content: string(payload)}, nil
}
