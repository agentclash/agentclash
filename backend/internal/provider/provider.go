package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUnsupportedProvider   = errors.New("provider is not supported")
	ErrCredentialUnavailable = errors.New("provider credential is unavailable")
	ErrStreamingNotSupported = errors.New("provider streaming is not supported")
)

type Client interface {
	InvokeModel(ctx context.Context, request Request) (Response, error)
}

type Request struct {
	ProviderKey         string
	ProviderAccountID   string
	CredentialReference string
	Model               string
	TraceMode           string
	StepTimeout         time.Duration
	Messages            []Message
	Tools               []ToolDefinition
	Metadata            json.RawMessage
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	IsError    bool       `json:"is_error,omitempty"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

type Response struct {
	ProviderKey     string
	ProviderModelID string
	FinishReason    string
	OutputText      string
	ToolCalls       []ToolCall
	Usage           Usage
	Streamed        bool
	Timing          Timing
	RawResponse     json.RawMessage
}

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

type Failure struct {
	ProviderKey string
	Code        FailureCode
	Message     string
	Retryable   bool
	Cause       error
}

type FailureCode string

const (
	FailureCodeAuth                  FailureCode = "auth"
	FailureCodeRateLimit             FailureCode = "rate_limit"
	FailureCodeInvalidRequest        FailureCode = "invalid_request"
	FailureCodeTimeout               FailureCode = "timeout"
	FailureCodeUnavailable           FailureCode = "unavailable"
	FailureCodeMalformedResponse     FailureCode = "malformed_response"
	FailureCodeCredentialUnavailable FailureCode = "credential_unavailable"
	FailureCodeUnsupportedProvider   FailureCode = "unsupported_provider"
	FailureCodeUnsupportedCapability FailureCode = "unsupported_capability"
	FailureCodeUnknown               FailureCode = "unknown"
)

func (f Failure) Error() string {
	if f.Message == "" {
		return "provider invocation failed"
	}
	return f.Message
}

func (f Failure) Unwrap() error {
	return f.Cause
}

func NewFailure(providerKey string, code FailureCode, message string, retryable bool, cause error) error {
	return Failure{
		ProviderKey: providerKey,
		Code:        code,
		Message:     message,
		Retryable:   retryable,
		Cause:       cause,
	}
}

func AsFailure(err error) (Failure, bool) {
	var failure Failure
	if !errors.As(err, &failure) {
		return Failure{}, false
	}
	return failure, true
}

type CredentialResolver interface {
	Resolve(ctx context.Context, credentialReference string) (string, error)
}

type Router struct {
	adapters map[string]Client
}

func NewRouter(adapters map[string]Client) Router {
	cloned := make(map[string]Client, len(adapters))
	for key, adapter := range adapters {
		cloned[key] = adapter
	}
	return Router{adapters: cloned}
}

func (r Router) InvokeModel(ctx context.Context, request Request) (Response, error) {
	adapter, ok := r.adapters[request.ProviderKey]
	if !ok {
		return Response{}, NewFailure(
			request.ProviderKey,
			FailureCodeUnsupportedProvider,
			fmt.Sprintf("provider %q is not supported", request.ProviderKey),
			false,
			ErrUnsupportedProvider,
		)
	}

	return adapter.InvokeModel(ctx, request)
}

func (r Router) StreamModel(ctx context.Context, request Request, onDelta func(StreamDelta) error) (Response, error) {
	adapter, ok := r.adapters[request.ProviderKey]
	if !ok {
		return Response{}, NewFailure(
			request.ProviderKey,
			FailureCodeUnsupportedProvider,
			fmt.Sprintf("provider %q is not supported", request.ProviderKey),
			false,
			ErrUnsupportedProvider,
		)
	}

	streamingAdapter, ok := adapter.(StreamingClient)
	if !ok {
		return Response{}, NewFailure(
			request.ProviderKey,
			FailureCodeUnsupportedCapability,
			fmt.Sprintf("provider %q does not support streaming", request.ProviderKey),
			false,
			ErrStreamingNotSupported,
		)
	}

	return streamingAdapter.StreamModel(ctx, request, onDelta)
}
