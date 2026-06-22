package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// ListModels enumerates the models reachable with the resolved credential via
// the OpenAI-compatible GET {baseURL}/models endpoint. This covers openai, xai,
// openrouter, and mistral. OpenRouter additionally returns per-token pricing,
// which is surfaced as live pricing; every other provider is enriched from the
// static fallback map.
func (c OpenAICompatibleClient) ListModels(ctx context.Context, request ListModelsRequest) ([]ModelInfo, error) {
	apiKey, err := c.credentialResolver.Resolve(ctx, request.CredentialReference)
	if err != nil {
		return nil, normalizeCredentialError(request.ProviderKey, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, NewFailure(request.ProviderKey, FailureCodeInvalidRequest, "build models request", false, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classifyTransportError(request.ProviderKey, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewFailure(request.ProviderKey, FailureCodeUnavailable, "read provider response", true, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, normalizeOpenAIErrorResponse(request.ProviderKey, resp.StatusCode, resp.Header, raw)
	}

	var envelope struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Pricing *struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, NewFailure(request.ProviderKey, FailureCodeMalformedResponse, "decode models response", false, err)
	}

	models := make([]ModelInfo, 0, len(envelope.Data))
	for _, entry := range envelope.Data {
		if strings.TrimSpace(entry.ID) == "" {
			continue
		}
		info := ModelInfo{ID: entry.ID, DisplayName: entry.Name}
		if info.DisplayName == "" {
			info.DisplayName = entry.ID
		}
		// OpenRouter reports pricing as USD-per-token strings; scale to per-Mtok.
		if entry.Pricing != nil {
			if in, ok := parseUSDPerToken(entry.Pricing.Prompt); ok {
				info.InputCostPerMTok = in * 1_000_000
				if out, ok := parseUSDPerToken(entry.Pricing.Completion); ok {
					info.OutputCostPerMTok = out * 1_000_000
				}
				info.PricingSource = PricingSourceLive
			}
		}
		models = append(models, info)
	}

	return enrichStaticPricing(request.ProviderKey, sortModels(models)), nil
}

// ListModels enumerates Anthropic models via GET {baseURL}/v1/models.
func (c AnthropicClient) ListModels(ctx context.Context, request ListModelsRequest) ([]ModelInfo, error) {
	apiKey, err := c.credentialResolver.Resolve(ctx, request.CredentialReference)
	if err != nil {
		return nil, normalizeCredentialError(request.ProviderKey, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/models?limit=1000", nil)
	if err != nil {
		return nil, NewFailure(request.ProviderKey, FailureCodeInvalidRequest, "build models request", false, err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", c.apiVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classifyTransportError(request.ProviderKey, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewFailure(request.ProviderKey, FailureCodeUnavailable, "read provider response", true, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, normalizeAnthropicErrorResponse(request.ProviderKey, resp.StatusCode, resp.Header, raw)
	}

	var envelope struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, NewFailure(request.ProviderKey, FailureCodeMalformedResponse, "decode models response", false, err)
	}

	models := make([]ModelInfo, 0, len(envelope.Data))
	for _, entry := range envelope.Data {
		if strings.TrimSpace(entry.ID) == "" {
			continue
		}
		name := entry.DisplayName
		if name == "" {
			name = entry.ID
		}
		models = append(models, ModelInfo{ID: entry.ID, DisplayName: name})
	}

	return enrichStaticPricing(request.ProviderKey, sortModels(models)), nil
}

// ListModels enumerates Gemini models via GET {baseURL}/v1beta/models.
func (c GeminiClient) ListModels(ctx context.Context, request ListModelsRequest) ([]ModelInfo, error) {
	apiKey, err := c.credentialResolver.Resolve(ctx, request.CredentialReference)
	if err != nil {
		return nil, normalizeCredentialError(request.ProviderKey, err)
	}

	endpoint := fmt.Sprintf("%s/v1beta/models?pageSize=1000&key=%s", c.baseURL, apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, NewFailure(request.ProviderKey, FailureCodeInvalidRequest, "build models request", false, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classifyTransportError(request.ProviderKey, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewFailure(request.ProviderKey, FailureCodeUnavailable, "read provider response", true, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, normalizeGeminiErrorResponse(request.ProviderKey, resp.StatusCode, resp.Header, raw)
	}

	var envelope struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, NewFailure(request.ProviderKey, FailureCodeMalformedResponse, "decode models response", false, err)
	}

	models := make([]ModelInfo, 0, len(envelope.Models))
	for _, entry := range envelope.Models {
		id := strings.TrimPrefix(strings.TrimSpace(entry.Name), "models/")
		if id == "" {
			continue
		}
		// Only surface models that can actually generate content.
		if len(entry.SupportedGenerationMethods) > 0 && !containsAny(entry.SupportedGenerationMethods, "generateContent", "streamGenerateContent") {
			continue
		}
		name := entry.DisplayName
		if name == "" {
			name = id
		}
		models = append(models, ModelInfo{ID: id, DisplayName: name})
	}

	return enrichStaticPricing(request.ProviderKey, sortModels(models)), nil
}

func parseUSDPerToken(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

func sortModels(models []ModelInfo) []ModelInfo {
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

func containsAny(haystack []string, needles ...string) bool {
	for _, n := range needles {
		if slices.Contains(haystack, n) {
			return true
		}
	}
	return false
}
