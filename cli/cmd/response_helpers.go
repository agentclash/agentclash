package cmd

import (
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/api"
)

func mapValue(m map[string]any, keys ...string) any {
	if m == nil {
		return nil
	}
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value
		}
	}
	return nil
}

func mapString(m map[string]any, keys ...string) string {
	return str(mapValue(m, keys...))
}

func mapObject(m map[string]any, keys ...string) map[string]any {
	value := mapValue(m, keys...)
	if out, ok := value.(map[string]any); ok {
		return out
	}
	return nil
}

func mapSlice(m map[string]any, keys ...string) []any {
	value := mapValue(m, keys...)
	if out, ok := value.([]any); ok {
		return out
	}
	if values, ok := value.([]string); ok {
		out := make([]any, 0, len(values))
		for _, value := range values {
			out = append(out, value)
		}
		return out
	}
	if objects, ok := value.([]map[string]any); ok {
		out := make([]any, 0, len(objects))
		for _, value := range objects {
			out = append(out, value)
		}
		return out
	}
	return nil
}

func mapStringSlice(m map[string]any, keys ...string) []string {
	raw := mapSlice(m, keys...)
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		if text := strings.TrimSpace(str(value)); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func joinMapStrings(m map[string]any, key string) string {
	return strings.Join(mapStringSlice(m, key), ", ")
}

func handleStatefulReadResponse(rc *RunContext, resp *api.Response, resource string) (bool, error) {
	switch resp.StatusCode {
	case 202, 409:
		var payload map[string]any
		if err := resp.DecodeJSON(&payload); err != nil {
			return true, fmt.Errorf("decoding %s response: %w", resource, err)
		}

		if rc.Output.IsStructured() {
			if err := rc.Output.PrintRaw(payload); err != nil {
				return true, err
			}
		} else {
			state := mapString(payload, "state", "status")
			message := mapString(payload, "message")
			rendered := formatStatefulReadMessage(resource, state, message)
			if resp.StatusCode == 202 {
				rc.Output.PrintWarning(rendered)
			} else {
				rc.Output.PrintError(rendered)
			}
		}

		if resp.StatusCode == 409 {
			return true, &ExitCodeError{Code: 1}
		}
		return true, nil
	default:
		return false, nil
	}
}

func formatStatefulReadMessage(resource, state, message string) string {
	base := resource
	if state != "" {
		base = fmt.Sprintf("%s %s", resource, state)
	}
	if message == "" {
		return base
	}
	return fmt.Sprintf("%s — %s", base, message)
}
