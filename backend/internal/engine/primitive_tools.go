package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
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

	content, err := request.Session.ReadFile(ctx, args.Path)
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
	if err := request.Session.WriteFile(ctx, args.Path, []byte(args.Content)); err != nil {
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
	files, err := request.Session.ListFiles(ctx, args.Prefix)
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
				Lines struct {
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
