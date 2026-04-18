package provider

import (
	"net/http"
	"strings"
)

const defaultXAIBaseURL = "https://api.x.ai/v1"

// XAIClient uses xAI's OpenAI-compatible chat completions surface while
// keeping a distinct constructor and default endpoint for provider wiring.
type XAIClient struct {
	OpenAICompatibleClient
}

func NewXAIClient(httpClient *http.Client, baseURL string, credentialResolver CredentialResolver) XAIClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultXAIBaseURL
	}
	return XAIClient{
		OpenAICompatibleClient: NewOpenAICompatibleClient(httpClient, baseURL, credentialResolver),
	}
}
