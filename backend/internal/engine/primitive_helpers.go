package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

const (
	toolKindFile    = "file"
	toolKindData    = "data"
	toolKindNetwork = "network"
	toolKindBuild   = "build"

	toolOutputInlineLimitBytes       = 32 * 1024
	toolOutputPreviewLineCount       = 50
	toolSpillDirectory               = "/workspace/.agentclash/spill"
	toolInputDirectory               = "/workspace/.agentclash/tool-inputs"
	httpRequestTimeoutSecondsDefault = 30
	httpRequestTimeoutSecondsMax     = 60
	buildToolTimeoutSecondsDefault   = 120
	buildToolTimeoutSecondsMax       = 300
	httpRequestBodyLimitBytes        = 1 * 1024 * 1024
	httpResponseLimitBytes           = 5 * 1024 * 1024
)

type commandBehavior struct {
	SuccessExitCodes     []int
	EmptyResultExitCodes []int
}

type commandExecutionResult struct {
	ExecResult     sandbox.ExecResult
	IsError        bool
	IsEmpty        bool
	Classification string
}

func executeInternalCommand(ctx context.Context, request ToolExecutionRequest, toolName string, execRequest sandbox.ExecRequest, behavior commandBehavior) (commandExecutionResult, error) {
	result, err := request.Session.Exec(ctx, execRequest)
	if err != nil {
		if errorsAreShellPolicy(err) {
			return commandExecutionResult{IsError: true, Classification: "policy"}, nil
		}
		return commandExecutionResult{}, NewFailure(StopReasonSandboxError, "execute sandbox command", err)
	}

	successCodes := behavior.SuccessExitCodes
	if len(successCodes) == 0 {
		successCodes = []int{0}
	}
	if containsExitCode(successCodes, result.ExitCode) {
		return commandExecutionResult{ExecResult: result, Classification: "success"}, nil
	}
	if containsExitCode(behavior.EmptyResultExitCodes, result.ExitCode) {
		return commandExecutionResult{ExecResult: result, IsEmpty: true, Classification: "empty"}, nil
	}

	return commandExecutionResult{
		ExecResult:     result,
		IsError:        true,
		Classification: fmt.Sprintf("%s_exit_%d", strings.TrimSpace(toolName), result.ExitCode),
	}, nil
}

func toolTextOutput(ctx context.Context, request ToolExecutionRequest, toolName string, text string) (map[string]any, error) {
	totalBytes := len(text)
	totalLines := countLines(text)
	if totalBytes <= toolOutputInlineLimitBytes {
		return map[string]any{
			"truncated":   false,
			"content":     text,
			"total_bytes": totalBytes,
			"total_lines": totalLines,
		}, nil
	}

	if err := ensureToolSpillDirectory(ctx, request); err != nil {
		return nil, err
	}

	spillPath := path.Join(toolSpillDirectory, fmt.Sprintf("%s_%s.txt", strings.TrimSpace(toolName), uuid.NewString()))
	if err := request.Session.WriteFile(ctx, spillPath, []byte(text)); err != nil {
		return nil, NewFailure(StopReasonSandboxError, "write spilled tool output", err)
	}

	preview := previewLines(text, toolOutputPreviewLineCount)
	return map[string]any{
		"truncated":        true,
		"preview":          preview,
		"full_output_path": spillPath,
		"total_bytes":      totalBytes,
		"total_lines":      totalLines,
		"preview_lines":    countLines(preview),
	}, nil
}

func toolJSONOutput(ctx context.Context, request ToolExecutionRequest, toolName string, payload any) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", NewFailure(StopReasonSandboxError, "marshal tool result", err)
	}
	if len(encoded) <= toolOutputInlineLimitBytes {
		inline := map[string]any{
			"truncated":   false,
			"content":     payload,
			"total_bytes": len(encoded),
			"total_lines": countLines(string(encoded)),
		}
		encodedInline, err := json.Marshal(inline)
		if err != nil {
			return "", NewFailure(StopReasonSandboxError, "marshal inline tool output envelope", err)
		}
		return string(encodedInline), nil
	}

	output, err := toolTextOutput(ctx, request, toolName, string(encoded))
	if err != nil {
		return "", err
	}
	encodedOutput, err := json.Marshal(output)
	if err != nil {
		return "", NewFailure(StopReasonSandboxError, "marshal tool output envelope", err)
	}
	return string(encodedOutput), nil
}

func ensureToolSpillDirectory(ctx context.Context, request ToolExecutionRequest) error {
	return ensureToolDirectory(ctx, request, toolSpillDirectory)
}

func ensureToolDirectory(ctx context.Context, request ToolExecutionRequest, directory string) error {
	_, err := request.Session.Exec(ctx, sandbox.ExecRequest{
		Command: []string{"mkdir", "-p", directory},
	})
	if err != nil {
		return NewFailure(StopReasonSandboxError, "create tool directory", err)
	}
	return nil
}

func allowsToolKind(toolPolicy sandbox.ToolPolicy, kind string) bool {
	if strings.TrimSpace(kind) == "" {
		return false
	}
	if len(toolPolicy.AllowedToolKinds) == 0 {
		return true
	}
	for _, allowed := range toolPolicy.AllowedToolKinds {
		if strings.EqualFold(strings.TrimSpace(allowed), strings.TrimSpace(kind)) {
			return true
		}
	}
	return false
}

func allowsFileTools(toolPolicy sandbox.ToolPolicy) bool {
	return allowsToolKind(toolPolicy, toolKindFile)
}

func allowsDataTools(toolPolicy sandbox.ToolPolicy) bool {
	return allowsToolKind(toolPolicy, toolKindData)
}

func allowsNetworkTools(toolPolicy sandbox.ToolPolicy) bool {
	return allowsToolKind(toolPolicy, toolKindNetwork)
}

func allowsBuildTools(toolPolicy sandbox.ToolPolicy) bool {
	return allowsToolKind(toolPolicy, toolKindBuild)
}

func containsExitCode(codes []int, want int) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

func countLines(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func previewLines(text string, maxLines int) string {
	if maxLines <= 0 || text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}
	return strings.Join(lines[:maxLines], "\n")
}

func errorsAreShellPolicy(err error) bool {
	return err != nil && strings.Contains(err.Error(), sandbox.ErrShellNotAllowed.Error())
}
