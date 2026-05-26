package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	responsesPollInterval = 3 * time.Second
)

type openAIResponsesCreateRequest struct {
	Model      string                    `json:"model"`
	Input      []openAIResponsesMessage  `json:"input"`
	Tools      []openAIResponsesTool     `json:"tools,omitempty"`
	Background bool                      `json:"background,omitempty"`
	Reasoning  *openAIResponsesReasoning `json:"reasoning,omitempty"`
}

type openAIResponsesReasoning struct {
	Summary string `json:"summary,omitempty"`
}

type openAIResponsesMessage struct {
	Role    string                        `json:"role"`
	Content []openAIResponsesContentBlock `json:"content"`
}

type openAIResponsesContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type openAIResponsesTool struct {
	Type string `json:"type"`
}

type openAIResponsesResponse struct {
	ID         string          `json:"id"`
	Status     string          `json:"status"`
	Model      string          `json:"model"`
	Output     json.RawMessage `json:"output"`
	OutputText string          `json:"output_text,omitempty"`
	Usage      *struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		TotalTokens      int64 `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type openAIResponsesOutputItem struct {
	Type    string                        `json:"type"`
	Role    string                        `json:"role,omitempty"`
	Content []openAIResponsesContentBlock `json:"content,omitempty"`
}

func (c OpenAICompatibleClient) InvokeResearch(ctx context.Context, request ResearchRequest) (Response, error) {
	apiKey, err := c.credentialResolver.Resolve(ctx, request.CredentialReference)
	if err != nil {
		return Response{}, normalizeCredentialError(request.ProviderKey, err)
	}

	instructions := strings.TrimSpace(request.Instructions)
	if schema := strings.TrimSpace(string(request.OutputSchema)); schema != "" && schema != "{}" && schema != "null" {
		if instructions != "" {
			instructions += "\n\n"
		}
		instructions += "Final answer contract (respond with JSON matching this schema when finished):\n" + schema
	}

	input := make([]openAIResponsesMessage, 0, 2)
	if instructions != "" {
		input = append(input, openAIResponsesMessage{
			Role: "developer",
			Content: []openAIResponsesContentBlock{
				{Type: "input_text", Text: instructions},
			},
		})
	}
	userText := strings.TrimSpace(request.Input)
	if userText == "" {
		return Response{}, NewFailure(request.ProviderKey, FailureCodeInvalidRequest, "responses input is empty", false, nil)
	}
	input = append(input, openAIResponsesMessage{
		Role: "user",
		Content: []openAIResponsesContentBlock{
			{Type: "input_text", Text: userText},
		},
	})

	body := openAIResponsesCreateRequest{
		Model:      request.Model,
		Input:      input,
		Tools:      []openAIResponsesTool{{Type: "web_search_preview"}},
		Background: true,
		Reasoning:  &openAIResponsesReasoning{Summary: "auto"},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, NewFailure(request.ProviderKey, FailureCodeInvalidRequest, "marshal responses request", false, err)
	}

	callCtx := ctx
	if request.RunTimeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, request.RunTimeout)
		defer cancel()
	}

	startedAt := time.Now().UTC()
	createResp, rawCreate, err := c.postResponses(callCtx, apiKey, "/responses", payload)
	if err != nil {
		return Response{}, err
	}

	finalResp := createResp
	finalRaw := rawCreate
	if createResp.Status != "completed" && createResp.Status != "failed" && createResp.Status != "cancelled" && createResp.Status != "incomplete" {
		finalResp, finalRaw, err = c.pollResponsesUntilDone(callCtx, apiKey, createResp.ID)
		if err != nil {
			return Response{}, err
		}
	}

	if finalResp.Status == "failed" || finalResp.Status == "cancelled" || finalResp.Status == "incomplete" {
		message := fmt.Sprintf("responses job ended with status %q", finalResp.Status)
		if finalResp.Error != nil && strings.TrimSpace(finalResp.Error.Message) != "" {
			message = finalResp.Error.Message
		}
		return Response{}, NewFailure(request.ProviderKey, FailureCodeUnavailable, message, false, nil)
	}

	outputText, err := extractResponsesOutputText(request.ProviderKey, finalResp)
	if err != nil {
		return Response{}, err
	}

	completedAt := time.Now().UTC()
	usage := Usage{}
	if finalResp.Usage != nil {
		usage = Usage{
			InputTokens:  int64(finalResp.Usage.PromptTokens),
			OutputTokens: int64(finalResp.Usage.CompletionTokens),
			TotalTokens:  int64(finalResp.Usage.TotalTokens),
		}
	}

	modelID := strings.TrimSpace(finalResp.Model)
	if modelID == "" {
		modelID = request.Model
	}

	return Response{
		ProviderKey:     request.ProviderKey,
		ProviderModelID: modelID,
		FinishReason:    finalResp.Status,
		OutputText:      outputText,
		Usage:           usage,
		Timing: Timing{
			StartedAt:    startedAt,
			FirstTokenAt: startedAt,
			CompletedAt:  completedAt,
			TTFT:         0,
			TotalLatency: completedAt.Sub(startedAt),
		},
		RawResponse: finalRaw,
	}, nil
}

func (c OpenAICompatibleClient) postResponses(ctx context.Context, apiKey, path string, payload []byte) (openAIResponsesResponse, json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return openAIResponsesResponse{}, nil, NewFailure("openai", FailureCodeInvalidRequest, "build responses request", false, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return openAIResponsesResponse{}, nil, classifyTransportError("openai", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return openAIResponsesResponse{}, nil, NewFailure("openai", FailureCodeUnavailable, "read responses body", true, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return openAIResponsesResponse{}, nil, normalizeOpenAIErrorResponse("openai", resp.StatusCode, resp.Header, raw)
	}

	var decoded openAIResponsesResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return openAIResponsesResponse{}, nil, NewFailure("openai", FailureCodeMalformedResponse, "decode responses body", false, err)
	}
	return decoded, json.RawMessage(raw), nil
}

func (c OpenAICompatibleClient) getResponses(ctx context.Context, apiKey, responseID string) (openAIResponsesResponse, json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/responses/"+responseID, nil)
	if err != nil {
		return openAIResponsesResponse{}, nil, NewFailure("openai", FailureCodeInvalidRequest, "build responses poll request", false, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return openAIResponsesResponse{}, nil, classifyTransportError("openai", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return openAIResponsesResponse{}, nil, NewFailure("openai", FailureCodeUnavailable, "read responses poll body", true, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return openAIResponsesResponse{}, nil, normalizeOpenAIErrorResponse("openai", resp.StatusCode, resp.Header, raw)
	}

	var decoded openAIResponsesResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return openAIResponsesResponse{}, nil, NewFailure("openai", FailureCodeMalformedResponse, "decode responses poll body", false, err)
	}
	return decoded, json.RawMessage(raw), nil
}

func (c OpenAICompatibleClient) pollResponsesUntilDone(ctx context.Context, apiKey, responseID string) (openAIResponsesResponse, json.RawMessage, error) {
	if strings.TrimSpace(responseID) == "" {
		return openAIResponsesResponse{}, nil, NewFailure("openai", FailureCodeMalformedResponse, "responses create response missing id", false, nil)
	}

	ticker := time.NewTicker(responsesPollInterval)
	defer ticker.Stop()

	for {
		decoded, raw, err := c.getResponses(ctx, apiKey, responseID)
		if err != nil {
			return openAIResponsesResponse{}, nil, err
		}
		switch decoded.Status {
		case "completed", "failed", "cancelled", "incomplete":
			return decoded, raw, nil
		}

		select {
		case <-ctx.Done():
			return openAIResponsesResponse{}, nil, NewFailure("openai", FailureCodeTimeout, "responses job polling exceeded runtime budget", false, ctx.Err())
		case <-ticker.C:
		}
	}
}

func extractResponsesOutputText(providerKey string, response openAIResponsesResponse) (string, error) {
	if text := strings.TrimSpace(response.OutputText); text != "" {
		return text, nil
	}
	if len(response.Output) == 0 {
		return "", NewFailure(providerKey, FailureCodeMalformedResponse, "responses output is empty", false, nil)
	}

	var items []openAIResponsesOutputItem
	if err := json.Unmarshal(response.Output, &items); err != nil {
		return "", NewFailure(providerKey, FailureCodeMalformedResponse, "decode responses output array", false, err)
	}

	parts := make([]string, 0, 2)
	for _, item := range items {
		if item.Type != "message" {
			continue
		}
		for _, block := range item.Content {
			if block.Type == "output_text" && strings.TrimSpace(block.Text) != "" {
				parts = append(parts, block.Text)
			}
		}
	}
	if len(parts) == 0 {
		return "", NewFailure(providerKey, FailureCodeMalformedResponse, "responses output did not contain a message", false, nil)
	}
	return strings.Join(parts, "\n\n"), nil
}
