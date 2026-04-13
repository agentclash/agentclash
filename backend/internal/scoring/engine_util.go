package scoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

func stringifyEvidenceJSON(raw []byte) string {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	stringified, err := stringifyEvidenceValue(decoded)
	if err != nil {
		return string(raw)
	}
	return stringified
}

func stringifyEvidenceValue(value any) (string, error) {
	if resolved, ok := extractLooseString(value); ok {
		return resolved, nil
	}
	switch typed := value.(type) {
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%v", typed)), nil
	case nil:
		return "null", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodePayload(payload json.RawMessage) map[string]any {
	if len(bytes.TrimSpace(payload)) == 0 {
		return map[string]any{}
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return map[string]any{}
	}
	return decoded
}

func stringValue(payload map[string]any, key string) (string, bool) {
	value, ok := payload[key]
	if !ok {
		return "", false
	}
	return extractLooseString(value)
}

func intValue(payload map[string]any, key string) (int, bool) {
	value, ok := numericValue(payload, key)
	if !ok {
		return 0, false
	}
	return int(value), true
}

func extractLooseString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case json.RawMessage:
		return string(bytes.TrimSpace(typed)), len(bytes.TrimSpace(typed)) > 0
	case map[string]any:
		for _, candidate := range []string{"value", "content", "text", "answer"} {
			if item, ok := typed[candidate]; ok {
				if resolved, ok := extractLooseString(item); ok {
					return resolved, true
				}
			}
		}
	case []any:
		if len(typed) == 1 {
			return extractLooseString(typed[0])
		}
	}
	return "", false
}

func numericValue(payload map[string]any, key string) (float64, bool) {
	value, ok := payload[key]
	if !ok {
		return 0, false
	}
	return anyNumber(value)
}

func usageValue(payload map[string]any, key string) (float64, bool) {
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return 0, false
	}
	return numericValue(usage, key)
}

func addModelUsage(usageByModel map[string]*pricedUsage, providerKey string, providerModelID string, field string, value float64) {
	key := providerKey + "\x00" + providerModelID
	usage, ok := usageByModel[key]
	if !ok {
		usage = &pricedUsage{
			ProviderKey:     providerKey,
			ProviderModelID: providerModelID,
		}
		usageByModel[key] = usage
	}
	switch field {
	case "input_tokens":
		usage.InputTokens += value
	case "output_tokens":
		usage.OutputTokens += value
	case "total_tokens":
		usage.TotalTokens += value
	}
}

func anyNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	}
	return 0, false
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func strconvBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("unsupported boolean value %q", value)
	}
}

func mustMarshalJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func floatPtr(value float64) *float64 {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func uuidPtrOrNil(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil {
		return nil
	}
	cloned := value
	return &cloned
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
