package generation

import (
	"encoding/json"
	"fmt"
	"strings"
)

func BuildSelfInstructPrompt(seeds []SeedExample, count int) string {
	var b strings.Builder
	b.WriteString("You are generating synthetic evaluation dataset examples.\n")
	b.WriteString("Study the seed examples below and invent ONE new example that follows the same task shape but uses different inputs.\n")
	b.WriteString("Respond with ONLY valid JSON in this exact shape:\n")
	b.WriteString(`{"input": <json value>, "expected": <json value or null>}` + "\n")
	b.WriteString("Do not wrap the JSON in markdown fences.\n\n")
	b.WriteString("Seed examples:\n")
	for i, seed := range seeds {
		b.WriteString(fmt.Sprintf("%d. input: %s\n", i+1, compactJSON(seed.Input)))
		if len(seed.Expected) > 0 && string(seed.Expected) != "null" {
			b.WriteString(fmt.Sprintf("   expected: %s\n", compactJSON(seed.Expected)))
		}
	}
	b.WriteString(fmt.Sprintf("\nGenerate exactly 1 new example. Target batch size for this job: %d.\n", count))
	return b.String()
}

func ParseSelfInstructResponse(raw string) (Candidate, error) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var parsed struct {
		Input    json.RawMessage `json:"input"`
		Expected json.RawMessage `json:"expected"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return Candidate{}, fmt.Errorf("decode self-instruct response: %w", err)
	}
	if len(parsed.Input) == 0 || string(parsed.Input) == "null" {
		return Candidate{}, fmt.Errorf("self-instruct response missing input")
	}
	return Candidate{Input: parsed.Input, Expected: parsed.Expected}, nil
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "null"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return string(raw)
	}
	return string(encoded)
}
