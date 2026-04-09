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

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"

type GeminiClient struct {
	httpClient         *http.Client
	baseURL            string
	credentialResolver CredentialResolver
}

func NewGeminiClient(httpClient *http.Client, baseURL string, credentialResolver CredentialResolver) GeminiClient {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultGeminiBaseURL
	}
	return GeminiClient{
		httpClient:         httpClient,
		baseURL:            strings.TrimRight(baseURL, "/"),
		credentialResolver: credentialResolver,
	}
}

func (c GeminiClient) InvokeModel(ctx context.Context, request Request) (Response, error) {
	return c.StreamModel(ctx, request, nil)
}

func (c GeminiClient) StreamModel(ctx context.Context, request Request, onDelta func(StreamDelta) error) (Response, error) {
	apiKey, err := c.credentialResolver.Resolve(ctx, request.CredentialReference)
	if err != nil {
		return Response{}, normalizeCredentialError(request.ProviderKey, err)
	}

	body, err := buildGeminiRequestBody(request)
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

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s&alt=sse", c.baseURL, request.Model, apiKey)
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Response{}, NewFailure(request.ProviderKey, FailureCodeInvalidRequest, "build provider request", false, err)
	}
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
		return Response{}, normalizeGeminiErrorResponse(request.ProviderKey, resp.StatusCode, raw)
	}

	accumulator := NewStreamAccumulator(request.ProviderKey, startedAt)
	if err := consumeGeminiStream(resp.Body, request.ProviderKey, request.Model, accumulator, onDelta); err != nil {
		return Response{}, err
	}

	return accumulator.Finalize(time.Now().UTC())
}

func buildGeminiRequestBody(request Request) (geminiRequest, error) {
	var systemInstruction *geminiContent
	contents := make([]geminiContent, 0, len(request.Messages))

	for _, msg := range request.Messages {
		if msg.Role == "system" {
			systemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: msg.Content}},
			}
			continue
		}

		converted, err := normalizeGeminiRequestMessage(request.ProviderKey, msg)
		if err != nil {
			return geminiRequest{}, err
		}
		contents = append(contents, converted)
	}

	var tools []geminiToolDeclarationWrapper
	if len(request.Tools) > 0 {
		declarations := make([]geminiFunctionDeclaration, 0, len(request.Tools))
		for _, tool := range request.Tools {
			normalized, err := normalizeGeminiRequestTool(request.ProviderKey, tool)
			if err != nil {
				return geminiRequest{}, err
			}
			declarations = append(declarations, normalized)
		}
		tools = []geminiToolDeclarationWrapper{{FunctionDeclarations: declarations}}
	}

	return geminiRequest{
		Contents:          contents,
		Tools:             tools,
		SystemInstruction: systemInstruction,
	}, nil
}

func normalizeGeminiRequestMessage(providerKey string, msg Message) (geminiContent, error) {
	switch msg.Role {
	case "user":
		return geminiContent{
			Role:  "user",
			Parts: []geminiPart{{Text: msg.Content}},
		}, nil

	case "assistant":
		if len(msg.ToolCalls) > 0 {
			parts := make([]geminiPart, 0, len(msg.ToolCalls)+1)
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				args := tc.Arguments
				if len(args) == 0 {
					args = json.RawMessage(`{}`)
				}
				if !json.Valid(args) {
					return geminiContent{}, NewFailure(providerKey, FailureCodeInvalidRequest, fmt.Sprintf("tool call %q arguments must be valid JSON", tc.Name), false, nil)
				}
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Name,
						Args: append(json.RawMessage(nil), args...),
					},
				})
			}
			return geminiContent{Role: "model", Parts: parts}, nil
		}
		return geminiContent{
			Role:  "model",
			Parts: []geminiPart{{Text: msg.Content}},
		}, nil

	case "tool":
		return geminiContent{
			Role: "user",
			Parts: []geminiPart{
				{
					FunctionResponse: &geminiFunctionResponse{
						Name: msg.ToolCallID,
						Response: geminiResponsePayload{
							Content: msg.Content,
						},
					},
				},
			},
		}, nil

	default:
		return geminiContent{}, NewFailure(providerKey, FailureCodeInvalidRequest, fmt.Sprintf("unsupported message role %q", msg.Role), false, nil)
	}
}

func normalizeGeminiRequestTool(providerKey string, tool ToolDefinition) (geminiFunctionDeclaration, error) {
	parameters := tool.Parameters
	if len(parameters) == 0 {
		parameters = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	if !json.Valid(parameters) {
		return geminiFunctionDeclaration{}, NewFailure(providerKey, FailureCodeInvalidRequest, fmt.Sprintf("tool %q parameters must be valid JSON", tool.Name), false, nil)
	}

	// Gemini uses a protobuf-defined subset of OpenAPI schema and rejects
	// unknown fields like additionalProperties, $schema, $ref, oneOf, etc.
	parameters = stripUnsupportedGeminiSchemaFields(parameters)

	return geminiFunctionDeclaration{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  append(json.RawMessage(nil), parameters...),
	}, nil
}

// geminiUnsupportedSchemaFields lists JSON Schema keywords that the Gemini API
// does not recognise. The API accepts only the fields defined in its protobuf
// Schema message: type, format, title, description, nullable, enum, items,
// maxItems, minItems, properties, required, minProperties, maxProperties,
// minimum, maximum, minLength, maxLength, pattern, example, anyOf,
// propertyOrdering, default. Everything else must be stripped.
var geminiUnsupportedSchemaFields = map[string]bool{
	"additionalProperties":  true,
	"$schema":               true,
	"$ref":                  true,
	"$defs":                 true,
	"$id":                   true,
	"$comment":              true,
	"$anchor":               true,
	"definitions":           true,
	"oneOf":                 true,
	"allOf":                 true,
	"not":                   true,
	"if":                    true,
	"then":                  true,
	"else":                  true,
	"const":                 true,
	"patternProperties":     true,
	"propertyNames":         true,
	"dependentRequired":     true,
	"dependentSchemas":      true,
	"unevaluatedProperties": true,
	"unevaluatedItems":      true,
	"prefixItems":           true,
	"contains":              true,
	"uniqueItems":           true,
	"multipleOf":            true,
	"exclusiveMinimum":      true,
	"exclusiveMaximum":      true,
	"examples":              true,
	"readOnly":              true,
	"writeOnly":             true,
	"deprecated":            true,
	"contentMediaType":      true,
	"contentEncoding":       true,
}

func stripUnsupportedGeminiSchemaFields(schema json.RawMessage) json.RawMessage {
	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		return schema
	}

	stripped := stripGeminiFieldsRecursive(parsed)

	cleaned, err := json.Marshal(stripped)
	if err != nil {
		return schema
	}
	return cleaned
}

func stripGeminiFieldsRecursive(obj map[string]any) map[string]any {
	for key := range geminiUnsupportedSchemaFields {
		delete(obj, key)
	}

	// Recurse into properties (each value is a sub-schema).
	if props, ok := obj["properties"].(map[string]any); ok {
		for propKey, propVal := range props {
			if propSchema, ok := propVal.(map[string]any); ok {
				props[propKey] = stripGeminiFieldsRecursive(propSchema)
			}
		}
	}

	// Recurse into items (array element schema).
	if items, ok := obj["items"].(map[string]any); ok {
		obj["items"] = stripGeminiFieldsRecursive(items)
	}

	// Recurse into anyOf (the only composition keyword Gemini supports).
	if anyOf, ok := obj["anyOf"].([]any); ok {
		for i, branch := range anyOf {
			if branchSchema, ok := branch.(map[string]any); ok {
				anyOf[i] = stripGeminiFieldsRecursive(branchSchema)
			}
		}
	}

	return obj
}

// --- Gemini request/response types ---

type geminiRequest struct {
	Contents          []geminiContent                `json:"contents"`
	Tools             []geminiToolDeclarationWrapper `json:"tools,omitempty"`
	SystemInstruction *geminiContent                 `json:"system_instruction,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type geminiFunctionResponse struct {
	Name     string               `json:"name"`
	Response geminiResponsePayload `json:"response"`
}

type geminiResponsePayload struct {
	Content string `json:"content"`
}

type geminiToolDeclarationWrapper struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations"`
}

type geminiFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// --- Gemini streaming response types ---

type geminiStreamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string              `json:"text,omitempty"`
				FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason,omitempty"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int64 `json:"promptTokenCount"`
		CandidatesTokenCount int64 `json:"candidatesTokenCount"`
		TotalTokenCount      int64 `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
	ModelVersion string `json:"modelVersion,omitempty"`
}

type geminiErrorEnvelope struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// --- Gemini stream consumption ---

func consumeGeminiStream(body io.Reader, providerKey string, model string, accumulator *StreamAccumulator, onDelta func(StreamDelta) error) error {
	reader := bufio.NewReader(body)
	dataLines := make([]string, 0, 1)
	eventsProcessed := false
	toolCallIndex := 0

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]

		if strings.TrimSpace(data) == "" {
			return nil
		}

		eventsProcessed = true
		return processGeminiStreamEvent(providerKey, model, []byte(data), accumulator, onDelta, &toolCallIndex)
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

func processGeminiStreamEvent(providerKey string, model string, raw []byte, accumulator *StreamAccumulator, onDelta func(StreamDelta) error, toolCallIndex *int) error {
	// Check for error response first.
	var errEnvelope geminiErrorEnvelope
	if err := json.Unmarshal(raw, &errEnvelope); err == nil && errEnvelope.Error.Message != "" {
		return NewFailure(providerKey, FailureCodeUnknown, errEnvelope.Error.Message, false, nil)
	}

	var chunk geminiStreamChunk
	if err := json.Unmarshal(raw, &chunk); err != nil {
		return NewFailure(providerKey, FailureCodeMalformedResponse, "decode provider stream chunk", false, err)
	}

	timestamp := time.Now().UTC()

	if len(chunk.Candidates) > 0 {
		candidate := chunk.Candidates[0]

		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				if err := emitGeminiDelta(accumulator, onDelta, StreamDelta{
					Kind:      StreamDeltaKindText,
					Timestamp: timestamp,
					Text:      part.Text,
				}); err != nil {
					return err
				}
			}
			if part.FunctionCall != nil {
				args := part.FunctionCall.Args
				if len(args) == 0 {
					args = json.RawMessage(`{}`)
				}
				if err := emitGeminiDelta(accumulator, onDelta, StreamDelta{
					Kind:      StreamDeltaKindToolCall,
					Timestamp: timestamp,
					ToolCall: ToolCallFragment{
						Index:             *toolCallIndex,
						IDFragment:        fmt.Sprintf("gemini-call-%d", *toolCallIndex),
						NameFragment:      part.FunctionCall.Name,
						ArgumentsFragment: string(args),
					},
				}); err != nil {
					return err
				}
				*toolCallIndex++
			}
		}

		if candidate.FinishReason != "" {
			modelVersion := model
			if chunk.ModelVersion != "" {
				modelVersion = chunk.ModelVersion
			}
			terminal := StreamTerminal{
				FinishReason:    NormalizeGeminiFinishReason(candidate.FinishReason),
				ProviderModelID: modelVersion,
				RawResponse:     raw,
			}
			if err := emitGeminiDelta(accumulator, onDelta, StreamDelta{
				Kind:      StreamDeltaKindTerminal,
				Timestamp: timestamp,
				Terminal:  terminal,
			}); err != nil {
				return err
			}
		}
	}

	if chunk.UsageMetadata != nil {
		if err := emitGeminiDelta(accumulator, onDelta, StreamDelta{
			Kind:      StreamDeltaKindTerminal,
			Timestamp: timestamp,
			Terminal: StreamTerminal{
				Usage: &Usage{
					InputTokens:  chunk.UsageMetadata.PromptTokenCount,
					OutputTokens: chunk.UsageMetadata.CandidatesTokenCount,
					TotalTokens:  chunk.UsageMetadata.TotalTokenCount,
				},
				RawResponse: raw,
			},
		}); err != nil {
			return err
		}
	}

	return nil
}

func emitGeminiDelta(accumulator *StreamAccumulator, onDelta func(StreamDelta) error, delta StreamDelta) error {
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

func normalizeGeminiErrorResponse(providerKey string, statusCode int, raw []byte) error {
	var envelope geminiErrorEnvelope
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
