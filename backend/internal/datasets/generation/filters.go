package generation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/agentclash/agentclash/backend/internal/domain"
)

func CanonicalInputHash(input json.RawMessage) (string, error) {
	canonical, err := canonicalizeJSONValue(input)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func ValidateCandidateInput(schema json.RawMessage, enforced bool, input json.RawMessage) error {
	if !enforced {
		return nil
	}
	return domain.ValidateDatasetInputAgainstSchema(schema, input)
}

func ContainsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func canonicalizeJSONValue(raw json.RawMessage) ([]byte, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("canonicalize input: %w", err)
	}
	sorted := sortJSONValue(value)
	return json.Marshal(sorted)
}

func sortJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(typed))
		for _, key := range keys {
			out[key] = sortJSONValue(typed[key])
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = sortJSONValue(item)
		}
		return out
	default:
		return typed
	}
}
