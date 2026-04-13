package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

const (
	submitToolName      = "submit"
	readFileToolName    = "read_file"
	writeFileToolName   = "write_file"
	listFilesToolName   = "list_files"
	searchFilesToolName = "search_files"
	searchTextToolName  = "search_text"
	queryJSONToolName   = "query_json"
	querySQLToolName    = "query_sql"
	httpRequestToolName = "http_request"
	runTestsToolName    = "run_tests"
	buildToolName       = "build"
	execToolName        = "exec"
)

func toolMessage(result provider.ToolResult) provider.Message {
	return provider.Message{
		Role:       "tool",
		Content:    result.Content,
		ToolCallID: result.ToolCallID,
		IsError:    result.IsError,
	}
}

func successToolResult(toolCallID string, content string) provider.ToolResult {
	return provider.ToolResult{
		ToolCallID: toolCallID,
		Content:    content,
	}
}

func errorToolResult(toolCallID string, message string) provider.ToolResult {
	return provider.ToolResult{
		ToolCallID: toolCallID,
		Content:    encodeToolErrorMessage(message),
		IsError:    true,
	}
}

func encodeToolErrorMessage(message string) string {
	payload, err := json.Marshal(map[string]any{
		"error": message,
	})
	if err != nil {
		return `{"error":"tool execution failed"}`
	}
	return string(payload)
}

func decodeToolArguments(toolName string, arguments json.RawMessage, target interface{}) error {
	if len(arguments) == 0 {
		arguments = []byte(`{}`)
	}
	if err := json.Unmarshal(arguments, target); err != nil {
		return fmt.Errorf("tool %q arguments must be valid JSON", toolName)
	}
	return nil
}

func buildInitialMessages(executionContext repository.RunAgentExecutionContext) ([]provider.Message, error) {
	payload, err := buildTaskPromptPayload(executionContext)
	if err != nil {
		return nil, err
	}

	return []provider.Message{
		{
			Role:    "system",
			Content: buildSystemPrompt(executionContext),
		},
		{
			Role:    "user",
			Content: payload,
		},
	}, nil
}

func buildSystemPrompt(executionContext repository.RunAgentExecutionContext) string {
	sections := make([]string, 0, 4)

	if policyInstructions := strings.TrimSpace(extractPolicyInstructions(executionContext.Deployment.AgentBuildVersion.PolicySpec)); policyInstructions != "" {
		sections = append(sections, policyInstructions)
	}

	sections = append(sections,
		"You are executing a native AgentClash benchmark run inside an isolated sandbox.",
		"Use the available tools to inspect and modify the workspace. Tool failures are recoverable; adapt and continue if you still have budget.",
		"When you are finished, call the submit tool with your final answer. Plain assistant text does not end the run.",
	)

	if contract := strings.TrimSpace(string(executionContext.Deployment.AgentBuildVersion.OutputSchema)); contract != "" && contract != "{}" {
		sections = append(sections, "Final answer contract:\n"+contract)
	}

	return strings.Join(sections, "\n\n")
}

func buildTaskPromptPayload(executionContext repository.RunAgentExecutionContext) (string, error) {
	type taskPayload struct {
		RunID                string                                           `json:"run_id"`
		RunAgentID           string                                           `json:"run_agent_id"`
		RunName              string                                           `json:"run_name,omitempty"`
		ChallengePackVersion json.RawMessage                                  `json:"challenge_pack_version"`
		Challenges           []repository.ChallengeDefinitionExecutionContext `json:"challenges,omitempty"`
		ChallengeInputSet    *repository.ChallengeInputSetExecutionContext    `json:"challenge_input_set,omitempty"`
		AgentSpec            json.RawMessage                                  `json:"agent_spec,omitempty"`
		DeploymentConfig     json.RawMessage                                  `json:"deployment_config,omitempty"`
		RuntimeProfile       json.RawMessage                                  `json:"runtime_profile,omitempty"`
	}

	payload, err := json.MarshalIndent(taskPayload{
		RunID:                executionContext.Run.ID.String(),
		RunAgentID:           executionContext.RunAgent.ID.String(),
		RunName:              executionContext.Run.Name,
		ChallengePackVersion: cloneJSON(executionContext.ChallengePackVersion.Manifest),
		Challenges:           cloneChallengeDefinitions(executionContext.ChallengePackVersion.Challenges),
		ChallengeInputSet:    cloneChallengeInputSet(executionContext.ChallengeInputSet),
		AgentSpec:            cloneJSON(executionContext.Deployment.AgentBuildVersion.AgentSpec),
		DeploymentConfig:     cloneJSON(executionContext.Deployment.SnapshotConfig),
		RuntimeProfile:       cloneJSON(executionContext.Deployment.RuntimeProfile.ProfileConfig),
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal task prompt payload: %w", err)
	}
	return "Benchmark context:\n" + string(payload), nil
}

func buildProviderMetadata(executionContext repository.RunAgentExecutionContext) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]string{
		"run_id":                    executionContext.Run.ID.String(),
		"run_agent_id":              executionContext.RunAgent.ID.String(),
		"challenge_pack_version_id": executionContext.ChallengePackVersion.ID.String(),
		"agent_build_version_id":    executionContext.Deployment.AgentBuildVersion.ID.String(),
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func stepTimeout(executionContext repository.RunAgentExecutionContext) time.Duration {
	if executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds) * time.Second
}

func runTimeout(executionContext repository.RunAgentExecutionContext) time.Duration {
	if executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds) * time.Second
}

func extractPolicyInstructions(policySpec json.RawMessage) string {
	var decoded struct {
		Instructions      string `json:"instructions"`
		Role              string `json:"role"`
		SystemPrompt      string `json:"system_prompt"`
		SuccessConditions string `json:"success_conditions"`
	}
	if err := json.Unmarshal(policySpec, &decoded); err != nil {
		return ""
	}

	sections := make([]string, 0, 4)
	for _, value := range []string{
		decoded.Role,
		decoded.SystemPrompt,
		decoded.Instructions,
		decoded.SuccessConditions,
	} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			sections = append(sections, trimmed)
		}
	}

	return strings.Join(sections, "\n\n")
}

func (e NativeExecutor) executeToolCalls(
	ctx context.Context,
	session sandbox.Session,
	registry *Registry,
	toolPolicy sandbox.ToolPolicy,
	networkAllowlist []string,
	toolCallsUsedSoFar int,
	toolCalls []provider.ToolCall,
) ([]provider.Message, string, bool, int, error) {
	toolMessages := make([]provider.Message, 0, len(toolCalls))
	toolCallsUsed := 0

	for _, toolCall := range toolCalls {
		tool, ok := registry.Resolve(toolCall.Name)
		if !ok {
			result := errorToolResult(toolCall.ID, fmt.Sprintf("tool %q is not available in this runtime", toolCall.Name))
			if observerErr := e.observer.OnToolExecution(ctx, ToolExecutionRecord{
				ToolCall: toolCall,
				Result:   result,
			}); observerErr != nil {
				return nil, "", false, toolCallsUsed, NewFailure(StopReasonObserverError, "record native tool event", observerErr)
			}
			toolMessages = append(toolMessages, toolMessage(result))
			continue
		}

		if tool.Name() == submitToolName && len(toolCalls) != 1 {
			result := errorToolResult(toolCall.ID, "submit must be called by itself")
			if observerErr := e.observer.OnToolExecution(ctx, ToolExecutionRecord{
				ToolCall:     toolCall,
				Result:       result,
				ToolCategory: tool.Category(),
			}); observerErr != nil {
				return nil, "", false, toolCallsUsed, NewFailure(StopReasonObserverError, "record native tool event", observerErr)
			}
			toolMessages = append(toolMessages, toolMessage(result))
			continue
		}

		if limit := int(toolPolicy.MaxToolCalls); limit > 0 && toolCallsUsedSoFar+toolCallsUsed >= limit {
			totalUsed := toolCallsUsedSoFar + toolCallsUsed
			return nil, "", false, toolCallsUsed, NewFailure(StopReasonToolLimit, fmt.Sprintf("native execution exhausted tool-call budget after %d tool calls", totalUsed), nil)
		}

		executionResult, hardErr := tool.Execute(ctx, ToolExecutionRequest{
			Args:             toolCall.Arguments,
			Session:          session,
			ToolPolicy:       toolPolicy,
			NetworkAllowlist: append([]string(nil), networkAllowlist...),
			Registry:         registry,
		})
		if hardErr != nil {
			return nil, "", false, toolCallsUsed, hardErr
		}

		result := provider.ToolResult{
			ToolCallID: toolCall.ID,
			Content:    executionResult.Content,
			IsError:    executionResult.IsError,
		}
		record := ToolExecutionRecord{
			ToolCall:             toolCall,
			Result:               result,
			ToolCategory:         tool.Category(),
			ResolvedToolName:     executionResult.ResolvedToolName,
			ResolvedToolCategory: executionResult.ResolvedToolCategory,
			FailureOrigin:        executionResult.FailureOrigin,
			ResolutionChain:      executionResult.ResolutionChain,
			FailureDepth:         executionResult.FailureDepth,
		}
		if observerErr := e.observer.OnToolExecution(ctx, record); observerErr != nil {
			return nil, "", false, toolCallsUsed, NewFailure(StopReasonObserverError, "record native tool event", observerErr)
		}

		if tool.Name() != submitToolName {
			toolCallsUsed++
		}
		toolMessages = append(toolMessages, toolMessage(result))
		if executionResult.Completed {
			if executionResult.IsError {
				return toolMessages, "", false, toolCallsUsed, nil
			}
			return toolMessages[:len(toolMessages)-1], executionResult.FinalOutput, true, toolCallsUsed, nil
		}
	}

	return toolMessages, "", false, toolCallsUsed, nil
}
