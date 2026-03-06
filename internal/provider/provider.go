package provider

import "context"

// Provider is the common interface for all LLM backends.
type Provider interface {
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	Name() string
}

// Message represents a single message in the conversation.
type Message struct {
	Role       string     `json:"role"` // "system", "user", "assistant", "tool"
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"` // tool name for tool results
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // raw JSON string
}

// ToolDef describes a tool the model can call.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Usage tracks token consumption for a single LLM call.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatRequest is what we send to any provider.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Tools       []ToolDef
	MaxTokens   int
	Temperature float64
}

// ChatResponse is the normalized response from any provider.
type ChatResponse struct {
	Content    string
	ToolCalls  []ToolCall
	Usage      Usage
	StopReason string // "stop", "tool_calls", "length"
}
