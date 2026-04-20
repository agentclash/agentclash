package provider

import (
	"net/http"
	"time"
)

const DefaultHTTPTimeout = 60 * time.Second

func NewDefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: DefaultHTTPTimeout}
}

func NewDefaultRouter(httpClient *http.Client, resolver CredentialResolver) Router {
	if httpClient == nil {
		httpClient = NewDefaultHTTPClient()
	}
	if resolver == nil {
		resolver = EnvCredentialResolver{}
	}

	return NewRouter(map[string]Client{
		"openai":     NewOpenAICompatibleClient(httpClient, "", resolver),
		"anthropic":  NewAnthropicClient(httpClient, "", "", resolver),
		"gemini":     NewGeminiClient(httpClient, "", resolver),
		"xai":        NewOpenAICompatibleClient(httpClient, DefaultXAIBaseURL(), resolver),
		"openrouter": NewOpenAICompatibleClient(httpClient, "https://openrouter.ai/api/v1", resolver),
		"mistral":    NewOpenAICompatibleClient(httpClient, "https://api.mistral.ai/v1", resolver),
	})
}
