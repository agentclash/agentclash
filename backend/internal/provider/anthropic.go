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

const (
	defaultAnthropicBaseURL    = "https://api.anthropic.com"
	defaultAnthropicAPIVersion = "2023-06-01"
	defaultAnthropicMaxTokens  = 4096
)

type AnthropicClient struct {
	httpClient         *http.Client
	baseURL            string
	apiVersion         string
	credentialResolver CredentialResolver
}

func NewAnthropicClient(httpClient *http.Client, baseURL string, apiVersion string, credentialResolver CredentialResolver) AnthropicClient {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultAnthropicBaseURL
	}
	if strings.TrimSpace(apiVersion) == "" {
		apiVersion = defaultAnthropicAPIVersion
	}
	return AnthropicClient{
		httpClient:         httpClient,
		baseURL:            strings.TrimRight(baseURL, "/"),
		apiVersion:         apiVersion,
		credentialResolver: credentialResolver,
	}
}

func (c AnthropicClient) InvokeModel(ctx context.Context, request Request) (Response, error) {
	return c.StreamModel(ctx, request, nil)
}

func (c AnthropicClient) StreamModel(ctx context.Context, request Request, onDelta func(StreamDelta) error) (Response, error) {
	apiKey, err := c.credentialResolver.Resolve(ctx, request.CredentialReference)
	if err != nil {
		return Response{}, normalizeCredentialError(request.ProviderKey, err)
	}

	body, err := buildAnthropicRequestBody(request)
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

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return Response{}, NewFailure(request.ProviderKey, FailureCodeInvalidRequest, "build provider request", false, err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", c.apiVersion)
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
		return Response{}, normalizeAnthropicErrorResponse(request.ProviderKey, resp.StatusCode, resp.Header, raw)
	}

	accumulator := NewStreamAccumulator(request.ProviderKey, startedAt)
	if err := consumeAnthropicStream(resp.Body, request.ProviderKey, accumulator, onDelta); err != nil {
		return Response{}, err
	}

	return accumulator.Finalize(time.Now().UTC())
}

func buildAnthropicRequestBody(request Request) (anthropicRequest, error) {
	var system string
	messages := make([]anthropicMessage, 0, len(request.Messages))

	for _, msg := range request.Messages {
		if msg.Role == "system" {
			system = msg.Content
			continue
		}

		converted, err := normalizeAnthropicRequestMessage(request.ProviderKey, msg)
		if err != nil {
			return anthropicRequest{}, err
		}
		messages = append(messages, converted)
	}

	tools := make([]anthropicTool, 0, len(request.Tools))
	for _, tool := range request.Tools {
		normalized, err := normalizeAnthropicRequestTool(request.ProviderKey, tool)
		if err != nil {
			return anthropicRequest{}, err
		}
		tools = append(tools, normalized)
	}

	return anthropicRequest{
		Model:     request.Model,
		MaxTokens: defaultAnthropicMaxTokens,
		System:    system,
		Messages:  messages,
		Tools:     tools,
		Stream:    true,
	}, nil
}

func normalizeAnthropicRequestMessage(providerKey string, msg Message) (anthropicMessage, error) {
	switch msg.Role {
	case "user":
		return anthropicMessage{
			Role:    "user",
			Content: anthropicContentFromString(msg.Content),
		}, nil

	case "assistant":
		if len(msg.ToolCalls) > 0 {
			blocks := make([]anthropicContentBlock, 0, len(msg.ToolCalls)+1)
			if msg.Content != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				input := tc.Arguments
				if len(input) == 0 {
					input = json.RawMessage(`{}`)
				}
				if !json.Valid(input) {
					return anthropicMessage{}, NewFailure(providerKey, FailureCodeInvalidRequest, fmt.Sprintf("tool call %q arguments must be valid JSON", tc.Name), false, nil)
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: append(json.RawMessage(nil), input...),
				})
			}
			return anthropicMessage{
				Role:          "assistant",
				ContentBlocks: blocks,
			}, nil
		}
		return anthropicMessage{
			Role:    "assistant",
			Content: anthropicContentFromString(msg.Content),
		}, nil

	case "tool":
		return anthropicMessage{
			Role: "user",
			ContentBlocks: []anthropicContentBlock{
				{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
					IsError:   msg.IsError,
				},
			},
		}, nil

	default:
		return anthropicMessage{}, NewFailure(providerKey, FailureCodeInvalidRequest, fmt.Sprintf("unsupported message role %q", msg.Role), false, nil)
	}
}

func anthropicContentFromString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func normalizeAnthropicRequestTool(providerKey string, tool ToolDefinition) (anthropicTool, error) {
	schema := tool.Parameters
	if len(schema) == 0 {
		schema = json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	}
	if !json.Valid(schema) {
		return anthropicTool{}, NewFailure(providerKey, FailureCodeInvalidRequest, fmt.Sprintf("tool %q parameters must be valid JSON", tool.Name), false, nil)
	}

	return anthropicTool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: append(json.RawMessage(nil), schema...),
	}, nil
}

// --- Anthropic request/response types ---

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream"`
}

type anthropicMessage struct {
	Role          string                 `json:"role"`
	Content       json.RawMessage        `json:"content,omitempty"`
	ContentBlocks []anthropicContentBlock `json:"-"`
}

func (m anthropicMessage) MarshalJSON() ([]byte, error) {
	if len(m.ContentBlocks) > 0 {
		type alias struct {
			Role    string                 `json:"role"`
			Content []anthropicContentBlock `json:"content"`
		}
		return json.Marshal(alias{Role: m.Role, Content: m.ContentBlocks})
	}
	type alias struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	return json.Marshal(alias{Role: m.Role, Content: m.Content})
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// --- Anthropic SSE event types ---

type anthropicMessageStart struct {
	Message struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type anthropicContentBlockStart struct {
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
}

type anthropicContentBlockDelta struct {
	Index int `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
}

type anthropicMessageDelta struct {
	Delta struct {
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicErrorEnvelope struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// --- Anthropic stream consumption ---

func consumeAnthropicStream(body io.Reader, providerKey string, accumulator *StreamAccumulator, onDelta func(StreamDelta) error) error {
	reader := bufio.NewReader(body)
	var currentEvent string
	dataLines := make([]string, 0, 1)
	eventsProcessed := false

	// Track tool call indices for mapping content blocks to ToolCallFragment.Index.
	toolCallIndex := 0
	lastBlockWasToolUse := false
	var inputTokens int64

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		eventType := currentEvent
		currentEvent = ""

		if strings.TrimSpace(data) == "" {
			return nil
		}

		eventsProcessed = true
		return processAnthropicStreamEvent(providerKey, eventType, []byte(data), accumulator, onDelta, &toolCallIndex, &lastBlockWasToolUse, &inputTokens)
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
			// SSE comment / keepalive
		default:
			field, value, found := strings.Cut(trimmed, ":")
			if !found {
				continue
			}
			value = strings.TrimPrefix(value, " ")
			switch field {
			case "event":
				currentEvent = value
			case "data":
				dataLines = append(dataLines, value)
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

func processAnthropicStreamEvent(providerKey string, eventType string, raw []byte, accumulator *StreamAccumulator, onDelta func(StreamDelta) error, toolCallIndex *int, lastBlockWasToolUse *bool, inputTokens *int64) error {
	timestamp := time.Now().UTC()

	switch eventType {
	case "message_start":
		var msg anthropicMessageStart
		if err := json.Unmarshal(raw, &msg); err != nil {
			return NewFailure(providerKey, FailureCodeMalformedResponse, "decode message_start", false, err)
		}
		*inputTokens = msg.Message.Usage.InputTokens
		return emitAnthropicDelta(accumulator, onDelta, StreamDelta{
			Kind:      StreamDeltaKindTerminal,
			Timestamp: timestamp,
			Terminal: StreamTerminal{
				ProviderModelID: msg.Message.Model,
				RawResponse:     raw,
			},
		})

	case "content_block_start":
		var block anthropicContentBlockStart
		if err := json.Unmarshal(raw, &block); err != nil {
			return NewFailure(providerKey, FailureCodeMalformedResponse, "decode content_block_start", false, err)
		}
		*lastBlockWasToolUse = block.ContentBlock.Type == "tool_use"
		if *lastBlockWasToolUse {
			return emitAnthropicDelta(accumulator, onDelta, StreamDelta{
				Kind:      StreamDeltaKindToolCall,
				Timestamp: timestamp,
				ToolCall: ToolCallFragment{
					Index:        *toolCallIndex,
					IDFragment:   block.ContentBlock.ID,
					NameFragment: block.ContentBlock.Name,
				},
			})
		}
		return nil

	case "content_block_delta":
		var delta anthropicContentBlockDelta
		if err := json.Unmarshal(raw, &delta); err != nil {
			return NewFailure(providerKey, FailureCodeMalformedResponse, "decode content_block_delta", false, err)
		}
		switch delta.Delta.Type {
		case "text_delta":
			return emitAnthropicDelta(accumulator, onDelta, StreamDelta{
				Kind:      StreamDeltaKindText,
				Timestamp: timestamp,
				Text:      delta.Delta.Text,
			})
		case "input_json_delta":
			return emitAnthropicDelta(accumulator, onDelta, StreamDelta{
				Kind:      StreamDeltaKindToolCall,
				Timestamp: timestamp,
				ToolCall: ToolCallFragment{
					Index:             *toolCallIndex,
					ArgumentsFragment: delta.Delta.PartialJSON,
				},
			})
		}
		return nil

	case "content_block_stop":
		if *lastBlockWasToolUse {
			*toolCallIndex++
			*lastBlockWasToolUse = false
		}
		return nil

	case "message_delta":
		var msgDelta anthropicMessageDelta
		if err := json.Unmarshal(raw, &msgDelta); err != nil {
			return NewFailure(providerKey, FailureCodeMalformedResponse, "decode message_delta", false, err)
		}
		outputTokens := msgDelta.Usage.OutputTokens
		return emitAnthropicDelta(accumulator, onDelta, StreamDelta{
			Kind:      StreamDeltaKindTerminal,
			Timestamp: timestamp,
			Terminal: StreamTerminal{
				FinishReason: NormalizeAnthropicStopReason(msgDelta.Delta.StopReason),
				Usage: &Usage{
					InputTokens:  *inputTokens,
					OutputTokens: outputTokens,
					TotalTokens:  *inputTokens + outputTokens,
				},
				RawResponse: raw,
			},
		})

	case "message_stop":
		return nil

	case "error":
		var envelope anthropicErrorEnvelope
		if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Error.Message != "" {
			if envelope.Error.Type == "overloaded_error" {
				return NewFailure(providerKey, FailureCodeRateLimit, envelope.Error.Message, true, nil)
			}
			return NewFailure(providerKey, FailureCodeUnknown, envelope.Error.Message, false, nil)
		}
		return NewFailure(providerKey, FailureCodeUnknown, "provider stream error", false, nil)

	case "ping":
		return nil

	default:
		// Ignore unknown event types for forward compatibility.
		return nil
	}
}

func emitAnthropicDelta(accumulator *StreamAccumulator, onDelta func(StreamDelta) error, delta StreamDelta) error {
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

func normalizeAnthropicErrorResponse(providerKey string, statusCode int, header http.Header, raw []byte) error {
	var envelope anthropicErrorEnvelope
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
		f := Failure{ProviderKey: providerKey, Code: FailureCodeRateLimit, Message: message, Retryable: true, RetryAfter: parseRetryAfter(header)}
		return f
	case 529: // Anthropic overloaded
		f := Failure{ProviderKey: providerKey, Code: FailureCodeRateLimit, Message: message, Retryable: true, RetryAfter: parseRetryAfter(header)}
		return f
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return NewFailure(providerKey, FailureCodeInvalidRequest, message, false, nil)
	case http.StatusGatewayTimeout, http.StatusBadGateway, http.StatusServiceUnavailable:
		return NewFailure(providerKey, FailureCodeUnavailable, message, true, nil)
	default:
		return NewFailure(providerKey, FailureCodeUnknown, message, statusCode >= 500, nil)
	}
}
