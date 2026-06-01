package generation_test

import (
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/datasets/generation"
)

func TestBuildSelfInstructPromptIncludesSeeds(t *testing.T) {
	prompt := generation.BuildSelfInstructPrompt([]generation.SeedExample{
		{Input: json.RawMessage(`{"question":"hello"}`), Expected: json.RawMessage(`{"answer":"hi"}`)},
	}, 3)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsAll(prompt, "Seed examples:", `"question":"hello"`, `"answer":"hi"`) {
		t.Fatalf("prompt missing seed content: %q", prompt)
	}
}

func TestParseSelfInstructResponse(t *testing.T) {
	candidate, err := generation.ParseSelfInstructResponse(`{"input":{"q":"x"},"expected":{"a":"y"}}`)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if string(candidate.Input) != `{"q":"x"}` {
		t.Fatalf("unexpected input: %s", candidate.Input)
	}
}

func TestParseSelfInstructResponseRejectsInvalidJSON(t *testing.T) {
	_, err := generation.ParseSelfInstructResponse("not json")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestCanonicalInputHashDedupesKeyOrder(t *testing.T) {
	a, err := generation.CanonicalInputHash(json.RawMessage(`{"b":1,"a":2}`))
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	b, err := generation.CanonicalInputHash(json.RawMessage(`{"a":2,"b":1}`))
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if a != b {
		t.Fatalf("expected equal hashes, got %s and %s", a, b)
	}
}

func TestComputeCostUSD(t *testing.T) {
	cost := generation.ComputeCostUSD(1_000_000, 500_000, generation.ModelPricing{
		InputCostPerMillionTokens:  3,
		OutputCostPerMillionTokens: 6,
	})
	if cost != 6 {
		t.Fatalf("expected cost 6, got %v", cost)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !contains(text, part) {
			return false
		}
	}
	return true
}

func contains(text, part string) bool {
	return len(text) >= len(part) && (text == part || len(part) == 0 || indexOf(text, part) >= 0)
}

func indexOf(text, part string) int {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
