package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
	return c.StreamModel(ctx, request, nil)
}

func (c OpenAICompatibleClient) StreamModel(ctx context.Context, request Request, onDelta func(StreamDelta) error) (Response, error) {
	apiKey, err := c.credentialResolver.Resolve(ctx, request.CredentialReference)
	if err != nil {
		return Response{}, normalizeCredentialError(request.ProviderKey, err)
	}

	body, err := buildOpenAIRequestBody(request, true)
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

	startedAt := time.Now().UTC()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Response{}, classifyTransportError(request.ProviderKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return Response{}, NewFailure(request.ProviderKey, FailureCodeUnavailable, "read provider response", true, err)
		}
		return Response{}, normalizeOpenAIErrorResponse(request.ProviderKey, resp.StatusCode, raw)
	}

	accumulator := NewStreamAccumulator(request.ProviderKey, startedAt)
	if err := consumeOpenAIStream(resp.Body, request.ProviderKey, accumulator, onDelta); err != nil {
		return Response{}, err
	}

	return accumulator.Finalize(time.Now().UTC())
}

func buildOpenAIRequestBody(request Request, stream bool) (openAICompletionRequest, error) {
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
		Stream:   stream,
		StreamOptions: openAIStreamOptions{
			IncludeUsage: stream,
		},
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
	Model         string                 `json:"model"`
	Messages      []openAIRequestMessage `json:"messages"`
	Tools         []openAIRequestTool    `json:"tools,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
	StreamOptions openAIStreamOptions    `json:"stream_options,omitempty"`
}

type openAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
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

type openAICompletionChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason *string `json:"finish_reason"`
		Delta        struct {
			Role      string                     `json:"role"`
			Content   *string                    `json:"content"`
			ToolCalls []openAIStreamToolCallPart `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		TotalTokens      int64 `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

type openAIStreamToolCallPart struct {
	Index    int                `json:"index"`
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
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

func consumeOpenAIStream(body io.Reader, providerKey string, accumulator *StreamAccumulator, onDelta func(StreamDelta) error) error {
	reader := bufio.NewReader(body)
	dataLines := make([]string, 0, 1)
	eventsProcessed := false

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		eventsProcessed = true
		return processOpenAIStreamEvent(providerKey, []byte(data), accumulator, onDelta)
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return classifyTransportError(providerKey, err)
		}

		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case trimmed == "":
			if flushErr := flush(); flushErr != nil {
				return flushErr
			}
		case strings.HasPrefix(trimmed, ":"):
			// Ignore SSE comments/keepalive lines.
		default:
			field, value, found := strings.Cut(trimmed, ":")
			if found && field == "data" {
				dataLines = append(dataLines, strings.TrimPrefix(value, " "))
			}
		}

		if errors.Is(err, io.EOF) {
			if flushErr := flush(); flushErr != nil {
				return flushErr
			}
			if !eventsProcessed {
				return NewFailure(providerKey, FailureCodeMalformedResponse, "provider returned empty or non-streaming response", false, nil)
			}
			return nil
		}
	}
}

func processOpenAIStreamEvent(providerKey string, raw []byte, accumulator *StreamAccumulator, onDelta func(StreamDelta) error) error {
	if bytes.Equal(raw, []byte("[DONE]")) {
		return nil
	}

	var streamErr openAIErrorEnvelope
	if err := json.Unmarshal(raw, &streamErr); err == nil && strings.TrimSpace(streamErr.Error.Message) != "" {
		return NewFailure(providerKey, FailureCodeUnknown, streamErr.Error.Message, false, nil)
	}

	var chunk openAICompletionChunk
	if err := json.Unmarshal(raw, &chunk); err != nil {
		return NewFailure(providerKey, FailureCodeMalformedResponse, "decode provider stream chunk", false, err)
	}
	if len(chunk.Choices) > 1 {
		return NewFailure(providerKey, FailureCodeMalformedResponse, "provider stream chunk must contain at most one choice", false, nil)
	}

	timestamp := time.Now().UTC()
	if len(chunk.Choices) == 1 {
		choice := chunk.Choices[0]
		if choice.Delta.Content != nil {
			if err := emitOpenAIStreamDelta(accumulator, onDelta, StreamDelta{
				Kind:      StreamDeltaKindText,
				Timestamp: timestamp,
				Text:      *choice.Delta.Content,
			}); err != nil {
				return err
			}
		}
		for _, toolCall := range choice.Delta.ToolCalls {
			if toolCall.Type != "" && toolCall.Type != "function" {
				return NewFailure(providerKey, FailureCodeMalformedResponse, fmt.Sprintf("unsupported OpenAI tool call type %q", toolCall.Type), false, nil)
			}
			if err := emitOpenAIStreamDelta(accumulator, onDelta, StreamDelta{
				Kind:      StreamDeltaKindToolCall,
				Timestamp: timestamp,
				ToolCall: ToolCallFragment{
					Index:             toolCall.Index,
					IDFragment:        toolCall.ID,
					NameFragment:      toolCall.Function.Name,
					ArgumentsFragment: toolCall.Function.Arguments,
				},
			}); err != nil {
				return err
			}
		}

		if choice.FinishReason != nil {
			terminal := StreamTerminal{
				FinishReason:    *choice.FinishReason,
				ProviderModelID: chunk.Model,
				RawResponse:     raw,
			}
			if err := emitOpenAIStreamDelta(accumulator, onDelta, StreamDelta{
				Kind:      StreamDeltaKindTerminal,
				Timestamp: timestamp,
				Terminal:  terminal,
			}); err != nil {
				return err
			}
		}
	}

	if chunk.Usage != nil {
		if err := emitOpenAIStreamDelta(accumulator, onDelta, StreamDelta{
			Kind:      StreamDeltaKindTerminal,
			Timestamp: timestamp,
			Terminal: StreamTerminal{
				ProviderModelID: chunk.Model,
				Usage: &Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
					TotalTokens:  chunk.Usage.TotalTokens,
				},
				RawResponse: raw,
			},
		}); err != nil {
			return err
		}
	}

	if len(chunk.Choices) == 0 && chunk.Usage == nil {
		return NewFailure(providerKey, FailureCodeMalformedResponse, "provider stream chunk must contain a choice or usage data", false, nil)
	}

	return nil
}

func emitOpenAIStreamDelta(accumulator *StreamAccumulator, onDelta func(StreamDelta) error, delta StreamDelta) error {
	if err := accumulator.Consume(delta); err != nil {
		return err
	}
	if onDelta != nil {
		if err := onDelta(cloneStreamDelta(delta)); err != nil {
			return err
		}
	}
	return nil
}
