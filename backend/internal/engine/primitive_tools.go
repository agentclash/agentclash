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

	result, err := request.Session.Exec(ctx, sandbox.ExecRequest{
		Command:          append([]string(nil), args.Command...),
		WorkingDirectory: strings.TrimSpace(args.WorkingDirectory),
		Environment:      cloneStringMap(args.Environment),
	})
	if err != nil {
		if errors.Is(err, sandbox.ErrShellNotAllowed) {
			return ToolExecutionResult{Content: encodeToolErrorMessage("tool is not allowed in this runtime"), IsError: true}, nil
		}
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "execute sandbox command", err)
	}

	payload, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return ToolExecutionResult{}, NewFailure(StopReasonSandboxError, "marshal exec result", marshalErr)
	}
	if result.ExitCode != 0 {
		return ToolExecutionResult{Content: string(payload), IsError: true}, nil
	}

	return ToolExecutionResult{Content: string(payload)}, nil
}
