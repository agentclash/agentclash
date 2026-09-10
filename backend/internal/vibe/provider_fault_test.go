package vibe

import (
	"fmt"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
)

func TestVibeProviderFaultsKeepSafeCategories(t *testing.T) {
	for code, expected := range map[provider.FailureCode]string{
		provider.FailureCodeRateLimit:             "provider_rate_limit",
		provider.FailureCodeAuth:                  "provider_auth",
		provider.FailureCodeCredentialUnavailable: "provider_auth",
		provider.FailureCodeInvalidRequest:        "provider_request_rejected",
		provider.FailureCodeUnsupportedCapability: "provider_request_rejected",
		provider.FailureCodeTimeout:               "provider_timeout",
		provider.FailureCodeUnavailable:           "provider_unavailable",
		provider.FailureCodeMalformedResponse:     "provider_response_invalid",
		provider.FailureCodeUnknown:               "provider_error",
	} {
		err := fmt.Errorf("wrapped: %w", provider.NewFailure("openrouter", code, "SECRET credential and untrusted provider body", true, nil))
		f := issueFrom(err)
		if f.Code != expected || strings.Contains(f.Message, "SECRET") || strings.Contains(f.Message, "provider body") {
			t.Fatalf("provider classification or safe message failed: %+v", f)
		}
	}
	want := &Fault{Code: "context_limit", Message: "Bounded context."}
	if issueFrom(want) != want || issueFrom(nil) != nil {
		t.Fatal("existing domain faults changed")
	}
}
