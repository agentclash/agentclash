package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

type OpenAICompatibleClient struct {
	httpClient         *http.Client
	baseURL            string
	credentialResolver CredentialResolver
}

func NewOpenAICompatibleClient(httpClient *http.Client, baseURL string, credentialResolver CredentialResolver) OpenAICompatibleClient {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return OpenAICompatibleClient{
		httpClient:         httpClient,
		baseURL:            strings.TrimRight(baseURL, "/"),
		credentialResolver: credentialResolver,
	}
}

func (c OpenAICompatibleClient) InvokeModel(ctx context.Context, request Request) (Response, error) {
	apiKey, err := c.credentialResolver.Resolve(ctx, request.CredentialReference)
	if err != nil {
		return Response{}, normalizeCredentialError(request.ProviderKey, err)
	}

	body, err := buildOpenAIRequestBody(request)
	if err != nil {
		return Response{}, err
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, NewFailure(request.ProviderKey, FailureCodeInvalidRequest, "marshal provider request", false, err)
	}

	callCtx := ctx
	if request.StepTimeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, request.StepTimeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Response{}, NewFailure(request.ProviderKey, FailureCodeInvalidRequest, "build provider request", false, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Response{}, classifyTransportError(request.ProviderKey, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, NewFailure(request.ProviderKey, FailureCodeUnavailable, "read provider response", true, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Response{}, normalizeOpenAIErrorResponse(request.ProviderKey, resp.StatusCode, raw)
	}

	var completion openAICompletionResponse
	if err := json.Unmarshal(raw, &completion); err != nil {
		return Response{}, NewFailure(request.ProviderKey, FailureCodeMalformedResponse, "decode provider response", false, err)
	}
	if len(completion.Choices) != 1 {
		return Response{}, NewFailure(request.ProviderKey, FailureCodeMalformedResponse, "provider response must contain exactly one choice", false, nil)
	}

	toolCalls, err := normalizeOpenAIToolCalls(request.ProviderKey, completion.Choices[0].Message.ToolCalls)
	if err != nil {
		return Response{}, err
	}

	outputText := ""
	if completion.Choices[0].Message.Content != nil {
		outputText = *completion.Choices[0].Message.Content
	}

	return Response{
		ProviderKey:     request.ProviderKey,
		ProviderModelID: completion.Model,
		FinishReason:    completion.Choices[0].FinishReason,
		OutputText:      outputText,
		ToolCalls:       toolCalls,
		Usage: Usage{
			InputTokens:  completion.Usage.PromptTokens,
			OutputTokens: completion.Usage.CompletionTokens,
			TotalTokens:  completion.Usage.TotalTokens,
		},
		RawResponse: append([]byte(nil), raw...),
	}, nil
}

func buildOpenAIRequestBody(request Request) (openAICompletionRequest, error) {
	messages := make([]openAIRequestMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		normalized, err := normalizeOpenAIRequestMessage(request.ProviderKey, message)
		if err != nil {
			return openAICompletionRequest{}, err
		}
		messages = append(messages, normalized)
	}

	tools := make([]openAIRequestTool, 0, len(request.Tools))
	for _, tool := range request.Tools {
		normalized, err := normalizeOpenAIRequestTool(request.ProviderKey, tool)
		if err != nil {
			return openAICompletionRequest{}, err
		}
		tools = append(tools, normalized)
	}

	return openAICompletionRequest{
		Model:    request.Model,
		Messages: messages,
		Tools:    tools,
		Metadata: request.Metadata,
	}, nil
}

func normalizeOpenAIRequestMessage(providerKey string, message Message) (openAIRequestMessage, error) {
	toolCalls := make([]openAIResponseToolCall, 0, len(message.ToolCalls))
	for _, toolCall := range message.ToolCalls {
		normalized, err := normalizeOpenAIResponseToolCall(providerKey, toolCall)
		if err != nil {
			return openAIRequestMessage{}, err
		}
		toolCalls = append(toolCalls, normalized)
	}

	return openAIRequestMessage{
		Role:       message.Role,
		Content:    normalizeOpenAIContent(message),
		ToolCalls:  toolCalls,
		ToolCallID: message.ToolCallID,
	}, nil
}

func normalizeOpenAIContent(message Message) *string {
	if message.Content != "" {
		content := message.Content
		return &content
	}
	if message.Role == "assistant" && len(message.ToolCalls) > 0 {
		return nil
	}
	content := ""
	return &content
}

func normalizeOpenAIRequestTool(providerKey string, tool ToolDefinition) (openAIRequestTool, error) {
	parameters := tool.Parameters
	if len(parameters) == 0 {
		parameters = json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	}
	if !json.Valid(parameters) {
		return openAIRequestTool{}, NewFailure(providerKey, FailureCodeInvalidRequest, fmt.Sprintf("tool %q parameters must be valid JSON", tool.Name), false, nil)
	}

	return openAIRequestTool{
		Type: "function",
		Function: openAIFunctionDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  append(json.RawMessage(nil), parameters...),
		},
	}, nil
}

func normalizeOpenAIResponseToolCall(providerKey string, toolCall ToolCall) (openAIResponseToolCall, error) {
	arguments := toolCall.Arguments
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	if !json.Valid(arguments) {
		return openAIResponseToolCall{}, NewFailure(providerKey, FailureCodeInvalidRequest, fmt.Sprintf("tool call %q arguments must be valid JSON", toolCall.Name), false, nil)
	}

	return openAIResponseToolCall{
		ID:   toolCall.ID,
		Type: "function",
		Function: openAIFunctionCall{
			Name:      toolCall.Name,
			Arguments: string(arguments),
		},
	}, nil
}

func normalizeOpenAIToolCalls(providerKey string, toolCalls []openAIResponseToolCall) ([]ToolCall, error) {
	normalized := make([]ToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		if toolCall.Type != "" && toolCall.Type != "function" {
			return nil, NewFailure(providerKey, FailureCodeMalformedResponse, fmt.Sprintf("unsupported OpenAI tool call type %q", toolCall.Type), false, nil)
		}

		arguments := strings.TrimSpace(toolCall.Function.Arguments)
		if arguments == "" {
			arguments = `{}`
		}
		rawArguments := json.RawMessage(arguments)
		if !json.Valid(rawArguments) {
			return nil, NewFailure(providerKey, FailureCodeMalformedResponse, fmt.Sprintf("tool call %q returned invalid JSON arguments", toolCall.Function.Name), false, nil)
		}

		normalized = append(normalized, ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: append(json.RawMessage(nil), rawArguments...),
		})
	}
	return normalized, nil
}

type openAICompletionRequest struct {
	Model    string                 `json:"model"`
	Messages []openAIRequestMessage `json:"messages"`
	Tools    []openAIRequestTool    `json:"tools,omitempty"`
	Metadata json.RawMessage        `json:"metadata,omitempty"`
}

type openAIRequestMessage struct {
	Role       string                   `json:"role"`
	Content    *string                  `json:"content"`
	ToolCalls  []openAIResponseToolCall `json:"tool_calls,omitempty"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
}

type openAIRequestTool struct {
	Type     string                   `json:"type"`
	Function openAIFunctionDefinition `json:"function"`
}

type openAIFunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openAIResponseToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAICompletionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string                   `json:"role"`
			Content   *string                  `json:"content"`
			ToolCalls []openAIResponseToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		TotalTokens      int64 `json:"total_tokens"`
	} `json:"usage"`
}

type openAIErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func normalizeCredentialError(providerKey string, err error) error {
	if failure, ok := AsFailure(err); ok {
		failure.ProviderKey = providerKey
		return failure
	}
	return NewFailure(providerKey, FailureCodeCredentialUnavailable, err.Error(), false, err)
}

func classifyTransportError(providerKey string, err error) error {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return NewFailure(providerKey, FailureCodeTimeout, "provider request timed out", true, err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
		return NewFailure(providerKey, FailureCodeTimeout, "provider request timed out", true, err)
	}
	return NewFailure(providerKey, FailureCodeUnavailable, "provider request failed", true, err)
}

func normalizeOpenAIErrorResponse(providerKey string, statusCode int, raw []byte) error {
	var envelope openAIErrorEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return NewFailure(
			providerKey,
			FailureCodeMalformedResponse,
			fmt.Sprintf("provider returned HTTP %d with invalid error payload", statusCode),
			false,
			err,
		)
	}

	message := envelope.Error.Message
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("provider returned HTTP %d", statusCode)
	}

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return NewFailure(providerKey, FailureCodeAuth, message, false, nil)
	case http.StatusTooManyRequests:
		return NewFailure(providerKey, FailureCodeRateLimit, message, true, nil)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return NewFailure(providerKey, FailureCodeInvalidRequest, message, false, nil)
	case http.StatusGatewayTimeout, http.StatusBadGateway, http.StatusServiceUnavailable:
		return NewFailure(providerKey, FailureCodeUnavailable, message, true, nil)
	default:
		return NewFailure(providerKey, FailureCodeUnknown, message, statusCode >= 500, nil)
	}
}
