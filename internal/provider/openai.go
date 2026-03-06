package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAIConfig configures the OpenAI-compatible client.
// Works with OpenAI, OpenRouter, Groq, Together, DeepSeek, and any
// provider that implements the /v1/chat/completions endpoint.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string // defaults to "https://api.openai.com/v1"
}

type openaiProvider struct {
	client  *http.Client
	apiKey  string
	baseURL string
}

func NewOpenAI(cfg OpenAIConfig) Provider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &openaiProvider{
		client:  &http.Client{},
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
	}
}

func (p *openaiProvider) Name() string { return "openai-compatible" }

func (p *openaiProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	oaiReq := p.buildRequest(req)

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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

// --- OpenAI API types (internal) ---

type oaiRequest struct {
	Model               string       `json:"model"`
	Messages            []oaiMessage `json:"messages"`
	Tools               []oaiTool    `json:"tools,omitempty"`
	MaxTokens           int          `json:"max_tokens,omitempty"`
	MaxCompletionTokens int          `json:"max_completion_tokens,omitempty"`
	Temperature         *float64     `json:"temperature,omitempty"`
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiResponse struct {
	Choices []struct {
		Message      oaiMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *openaiProvider) buildRequest(req *ChatRequest) oaiRequest {
	msgs := make([]oaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = oaiMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			msgs[i].ToolCalls = append(msgs[i].ToolCalls, oaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
	}

	var tools []oaiTool
	for _, t := range req.Tools {
		tools = append(tools, oaiTool{
			Type: "function",
			Function: oaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	oReq := oaiRequest{
		Model:    req.Model,
		Messages: msgs,
		Tools:    tools,
	}

	// o-series models (o1, o3, o4-mini, etc.) use max_completion_tokens
	// and don't support the temperature parameter
	if isOSeries(req.Model) {
		oReq.MaxCompletionTokens = req.MaxTokens
	} else {
		oReq.MaxTokens = req.MaxTokens
		temp := req.Temperature
		oReq.Temperature = &temp
	}

	return oReq
}

func (p *openaiProvider) parseResponse(body []byte) (*ChatResponse, error) {
	var oaiResp oaiResponse
	if err := json.Unmarshal(body, &oaiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if oaiResp.Error != nil {
		return nil, fmt.Errorf("api error: %s", oaiResp.Error.Message)
	}
	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := oaiResp.Choices[0]

	var toolCalls []ToolCall
	for _, tc := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	stopReason := choice.FinishReason
	if stopReason == "tool_calls" {
		stopReason = "tool_calls"
	}

	return &ChatResponse{
		Content:    choice.Message.Content,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		Usage: Usage{
			PromptTokens:     oaiResp.Usage.PromptTokens,
			CompletionTokens: oaiResp.Usage.CompletionTokens,
			TotalTokens:      oaiResp.Usage.TotalTokens,
		},
	}, nil
}

// isOSeries detects OpenAI o-series reasoning models that need
// max_completion_tokens instead of max_tokens and don't support temperature.
func isOSeries(model string) bool {
	m := strings.ToLower(model)
	for _, prefix := range []string{"o1", "o3", "o4"} {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}
