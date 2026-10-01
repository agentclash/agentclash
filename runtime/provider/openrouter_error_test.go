package provider

import (
	"net/http"
	"testing"
)

func TestOpenRouterNumericErrorCodeKeepsHTTPClassification(t *testing.T) {
	for _, body := range []string{
		`{"error":{"code":429,"message":"Free provider rate limit"}}`,
		`{"error":{"code":"rate_limit","message":"Free provider rate limit"}}`,
	} {
		err := normalizeOpenAIErrorResponse("openrouter", http.StatusTooManyRequests, http.Header{}, []byte(body))
		failure, ok := AsFailure(err)
		if !ok || failure.Code != FailureCodeRateLimit {
			t.Fatalf("rate limit became malformed response: %v", err)
		}
	}
}
