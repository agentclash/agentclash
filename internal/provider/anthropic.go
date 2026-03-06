package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AnthropicConfig configures the Anthropic API client.
type AnthropicConfig struct {
	APIKey string
}

type anthropicProvider struct {
	client *http.Client
	apiKey string
}

func NewAnthropic(cfg AnthropicConfig) Provider {
	return &anthropicProvider{
		client: &http.Client{},
		apiKey: cfg.APIKey,
	}
}

func (p *anthropicProvider) Name() string { return "anthropic" }

func (p *anthropicProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	antReq := p.buildRequest(req)

	body, err := json.Marshal(antReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return p.parseResponse(respBody)
}

// --- Anthropic API types (internal) ---

type antRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []antMessage `json:"messages"`
	Tools     []antTool    `json:"tools,omitempty"`
}

type antMessage struct {
	Role    string           `json:"role"`
	Content antMessageContent `json:"content"`
}

// antMessageContent can be a string or an array of content blocks.
// We always use the array form for consistency.
type antMessageContent []antContentBlock

type antContentBlock struct {
	Type string `json:"type"`

	// For type="text"
	Text string `json:"text,omitempty"`

	// For type="tool_use"
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`

	// For type="tool_result"
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type antTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type antResponse struct {
	Content    []antRespBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type antRespBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

func (p *anthropicProvider) buildRequest(req *ChatRequest) antRequest {
	var system string
	var messages []antMessage

	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			system = m.Content
		case "user":
			messages = append(messages, antMessage{
				Role:    "user",
				Content: antMessageContent{{Type: "text", Text: m.Content}},
			})
		case "assistant":
			var blocks antMessageContent
			if m.Content != "" {
				blocks = append(blocks, antContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				input := make(map[string]any)
				if tc.Arguments != "" {
					_ = json.Unmarshal([]byte(tc.Arguments), &input)
				}
				// Anthropic requires input to be non-nil even if empty
				if input == nil {
					input = make(map[string]any)
				}
				blocks = append(blocks, antContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			// Anthropic requires at least one content block per message
			if len(blocks) == 0 {
				blocks = append(blocks, antContentBlock{Type: "text", Text: ""})
			}
			messages = append(messages, antMessage{Role: "assistant", Content: blocks})
		case "tool":
			// Consecutive tool results must be merged into a single user message
			// Check if previous message is already a user message with tool_results
			if len(messages) > 0 && messages[len(messages)-1].Role == "user" &&
				len(messages[len(messages)-1].Content) > 0 &&
				messages[len(messages)-1].Content[0].Type == "tool_result" {
				messages[len(messages)-1].Content = append(
					messages[len(messages)-1].Content,
					antContentBlock{
						Type:      "tool_result",
						ToolUseID: m.ToolCallID,
						Content:   m.Content,
					},
				)
			} else {
				messages = append(messages, antMessage{
					Role: "user",
					Content: antMessageContent{{
						Type:      "tool_result",
						ToolUseID: m.ToolCallID,
						Content:   m.Content,
					}},
				})
			}
		}
	}

	var tools []antTool
	for _, t := range req.Tools {
		tools = append(tools, antTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	return antRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  messages,
		Tools:     tools,
	}
}

func (p *anthropicProvider) parseResponse(body []byte) (*ChatResponse, error) {
	var antResp antResponse
	if err := json.Unmarshal(body, &antResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if antResp.Error != nil {
		return nil, fmt.Errorf("api error: %s", antResp.Error.Message)
	}

	var content string
	var toolCalls []ToolCall

	for _, block := range antResp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(argsJSON),
			})
		}
	}

	stopReason := antResp.StopReason
	if stopReason == "tool_use" {
		stopReason = "tool_calls"
	} else if stopReason == "end_turn" {
		stopReason = "stop"
	}

	totalTokens := antResp.Usage.InputTokens + antResp.Usage.OutputTokens

	return &ChatResponse{
		Content:    content,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		Usage: Usage{
			PromptTokens:     antResp.Usage.InputTokens,
			CompletionTokens: antResp.Usage.OutputTokens,
			TotalTokens:      totalTokens,
		},
	}, nil
}
