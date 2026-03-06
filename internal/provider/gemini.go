package provider

// Gemini supports OpenAI-compatible API via Google's endpoint.
// This is a thin wrapper that sets the right base URL.

type GeminiConfig struct {
	APIKey string
}

func NewGemini(cfg GeminiConfig) Provider {
	return NewOpenAI(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
	})
}
