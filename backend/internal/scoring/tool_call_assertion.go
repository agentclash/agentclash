package scoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type toolCallAssertionConfig struct {
	ToolName         string          `json:"tool_name,omitempty"`
	MustCall         *bool           `json:"must_call,omitempty"`
	Count            *int            `json:"count,omitempty"`
	MinCount         *int            `json:"min_count,omitempty"`
	MaxCount         *int            `json:"max_count,omitempty"`
	ArgumentsContain json.RawMessage `json:"arguments_contain,omitempty"`
	OrderedTools     []string        `json:"ordered_tools,omitempty"`
	OrderMode        string          `json:"order_mode,omitempty"`
}

const (
	toolCallOrderModeSubsequence = "subsequence"
	toolCallOrderModeExact       = "exact"
)

func ParseToolCallAssertionConfig(rawConfig json.RawMessage) (toolCallAssertionConfig, error) {
	var cfg toolCallAssertionConfig
	if len(strings.TrimSpace(string(rawConfig))) == 0 {
		return cfg, fmt.Errorf("tool_call_assertion config is required")
	}
	if err := decodeStrictJSON(rawConfig, &cfg); err != nil {
		return toolCallAssertionConfig{}, err
	}
	cfg.ToolName = strings.TrimSpace(cfg.ToolName)
	cfg.OrderMode = strings.TrimSpace(cfg.OrderMode)
	for i := range cfg.OrderedTools {
		cfg.OrderedTools[i] = strings.TrimSpace(cfg.OrderedTools[i])
	}
	if cfg.OrderMode == "" && len(cfg.OrderedTools) > 0 {
		cfg.OrderMode = toolCallOrderModeSubsequence
	}
	return cfg, nil
}

func validateToolCallAssertionConfig(cfg toolCallAssertionConfig, configPath string) ValidationErrors {
	var errs ValidationErrors

	if cfg.Count != nil && *cfg.Count < 0 {
		errs = append(errs, ValidationError{Field: configPath + ".count", Message: "must be greater than or equal to 0"})
	}
	if cfg.MinCount != nil && *cfg.MinCount < 0 {
		errs = append(errs, ValidationError{Field: configPath + ".min_count", Message: "must be greater than or equal to 0"})
	}
	if cfg.MaxCount != nil && *cfg.MaxCount < 0 {
		errs = append(errs, ValidationError{Field: configPath + ".max_count", Message: "must be greater than or equal to 0"})
	}
	if cfg.Count != nil && (cfg.MinCount != nil || cfg.MaxCount != nil) {
		errs = append(errs, ValidationError{Field: configPath + ".count", Message: "cannot be combined with min_count or max_count"})
	}
	if cfg.MinCount != nil && cfg.MaxCount != nil && *cfg.MinCount > *cfg.MaxCount {
		errs = append(errs, ValidationError{Field: configPath + ".min_count", Message: "must be less than or equal to max_count"})
	}
	if cfg.MustCall != nil && cfg.hasCountAssertion() {
		if *cfg.MustCall && cfg.Count != nil && *cfg.Count == 0 {
			errs = append(errs, ValidationError{Field: configPath + ".must_call", Message: "cannot be true when count is 0"})
		}
		if *cfg.MustCall && cfg.MaxCount != nil && *cfg.MaxCount == 0 {
			errs = append(errs, ValidationError{Field: configPath + ".must_call", Message: "cannot be true when max_count is 0"})
		}
		if !*cfg.MustCall && cfg.Count != nil && *cfg.Count > 0 {
			errs = append(errs, ValidationError{Field: configPath + ".must_call", Message: "cannot be false when count is greater than 0"})
		}
		if !*cfg.MustCall && cfg.MinCount != nil && *cfg.MinCount > 0 {
			errs = append(errs, ValidationError{Field: configPath + ".must_call", Message: "cannot be false when min_count is greater than 0"})
		}
	}
	if len(cfg.ArgumentsContain) > 0 {
		var fragment any
		if err := json.Unmarshal(cfg.ArgumentsContain, &fragment); err != nil {
			errs = append(errs, ValidationError{Field: configPath + ".arguments_contain", Message: fmt.Sprintf("must be valid JSON: %v", err)})
		} else if _, ok := fragment.(map[string]any); !ok {
			errs = append(errs, ValidationError{Field: configPath + ".arguments_contain", Message: "must be a JSON object"})
		}
	}
	if len(cfg.OrderedTools) > 0 {
		for i, tool := range cfg.OrderedTools {
			if tool == "" {
				errs = append(errs, ValidationError{Field: fmt.Sprintf("%s.ordered_tools[%d]", configPath, i), Message: "must not be empty"})
			}
		}
	}
	if cfg.OrderMode != "" {
		switch cfg.OrderMode {
		case toolCallOrderModeSubsequence, toolCallOrderModeExact:
		default:
			errs = append(errs, ValidationError{Field: configPath + ".order_mode", Message: `must be "subsequence" or "exact"`})
		}
		if len(cfg.OrderedTools) == 0 {
			errs = append(errs, ValidationError{Field: configPath + ".ordered_tools", Message: "is required when order_mode is set"})
		}
	}
	if !cfg.hasAnyAssertion() {
		errs = append(errs, ValidationError{Field: configPath, Message: "must declare at least one tool call assertion"})
	}
	return errs
}

func (cfg toolCallAssertionConfig) hasAnyAssertion() bool {
	return cfg.ToolName != "" ||
		cfg.MustCall != nil ||
		cfg.Count != nil ||
		cfg.MinCount != nil ||
		cfg.MaxCount != nil ||
		len(cfg.ArgumentsContain) > 0 ||
		len(cfg.OrderedTools) > 0
}

func evaluateToolCallAssertionValidator(result ValidatorResult, validator ValidatorDeclaration, evidence extractedEvidence) ValidatorResult {
	cfg, err := ParseToolCallAssertionConfig(validator.Config)
	if err != nil {
		result.State = OutputStateError
		result.Reason = fmt.Sprintf("parse tool_call_assertion config: %v", err)
		result.RawOutput = mustMarshalJSON(map[string]any{
			"state":  result.State,
			"reason": result.Reason,
		})
		return result
	}

	if errs := validateToolCallAssertionConfig(cfg, "config"); len(errs) > 0 {
		result.State = OutputStateError
		result.Reason = fmt.Sprintf("invalid tool_call_assertion config: %v", errs)
		result.RawOutput = mustMarshalJSON(map[string]any{
			"state":  result.State,
			"reason": result.Reason,
		})
		return result
	}

	outcome := applyToolCallAssertion(cfg, evidence.toolCallTrace)
	result.State = OutputStateAvailable
	result.Verdict = outcome.verdict
	result.NormalizedScore = outcome.normalizedScore
	result.Reason = outcome.reason
	if outcome.source != nil {
		result.Source = outcome.source
	}
	result.RawOutput = mustMarshalJSON(mergeEvidence(map[string]any{
		"state":            result.State,
		"verdict":          result.Verdict,
		"normalized_score": result.NormalizedScore,
		"reason":           result.Reason,
		"target":           validator.Target,
		"tool_name":        emptyNil(cfg.ToolName),
	}, outcome.evidence))
	return result
}

type toolCallAssertionOutcome struct {
	verdict         string
	normalizedScore *float64
	reason          string
	source          *Source
	evidence        map[string]any
}

func applyToolCallAssertion(cfg toolCallAssertionConfig, trace []toolCallTraceEntry) toolCallAssertionOutcome {
	matches := matchingToolCalls(cfg, trace)
	reasons := make([]string, 0)
	pass := true

	if cfg.hasPresenceAssertion() {
		mustCall := true
		if cfg.MustCall != nil {
			mustCall = *cfg.MustCall
		}
		if mustCall && len(matches) == 0 {
			pass = false
			reasons = append(reasons, "expected matching tool call was not observed")
		}
		if !mustCall && len(matches) > 0 {
			pass = false
			reasons = append(reasons, "forbidden matching tool call was observed")
		}
	}
	if cfg.Count != nil && len(matches) != *cfg.Count {
		pass = false
		reasons = append(reasons, fmt.Sprintf("matched tool call count %d != expected %d", len(matches), *cfg.Count))
	}
	if cfg.MinCount != nil && len(matches) < *cfg.MinCount {
		pass = false
		reasons = append(reasons, fmt.Sprintf("matched tool call count %d < min_count %d", len(matches), *cfg.MinCount))
	}
	if cfg.MaxCount != nil && len(matches) > *cfg.MaxCount {
		pass = false
		reasons = append(reasons, fmt.Sprintf("matched tool call count %d > max_count %d", len(matches), *cfg.MaxCount))
	}
	orderPass, orderReason := evaluateToolOrder(cfg, trace)
	if !orderPass {
		pass = false
		reasons = append(reasons, orderReason)
	}

	verdict := "pass"
	score := 1.0
	if !pass {
		verdict = "fail"
		score = 0
	}

	successfulToolNames := successfulObservedToolNames(trace)
	return toolCallAssertionOutcome{
		verdict:         verdict,
		normalizedScore: &score,
		reason:          strings.Join(reasons, "; "),
		source:          sourceForFirstMatchedToolCall(matches),
		evidence: map[string]any{
			"observed_count":        len(successfulToolNames),
			"failed_count":          len(trace) - len(successfulToolNames),
			"matched_count":         len(matches),
			"matched_indices":       matchedIndices(matches),
			"observed_tool_names":   successfulToolNames,
			"expected_count":        cfg.Count,
			"expected_min_count":    cfg.MinCount,
			"expected_max_count":    cfg.MaxCount,
			"expected_order":        nonEmptyStringsOrNil(cfg.OrderedTools),
			"expected_order_mode":   emptyNil(cfg.OrderMode),
			"arguments_contain_set": len(cfg.ArgumentsContain) > 0,
		},
	}
}

func (cfg toolCallAssertionConfig) hasPresenceAssertion() bool {
	if cfg.MustCall != nil {
		return true
	}
	if cfg.hasCountAssertion() {
		return false
	}
	return cfg.ToolName != "" || len(cfg.ArgumentsContain) > 0
}

func (cfg toolCallAssertionConfig) hasCountAssertion() bool {
	return cfg.Count != nil || cfg.MinCount != nil || cfg.MaxCount != nil
}

func matchingToolCalls(cfg toolCallAssertionConfig, trace []toolCallTraceEntry) []toolCallMatch {
	matches := make([]toolCallMatch, 0, len(trace))
	for i, entry := range trace {
		if entry.Failed {
			continue
		}
		if cfg.ToolName != "" && entry.ToolName != cfg.ToolName {
			continue
		}
		if len(cfg.ArgumentsContain) > 0 && !toolCallArgumentsContain(entry.Arguments, cfg.ArgumentsContain) {
			continue
		}
		matches = append(matches, toolCallMatch{Index: i, Entry: entry})
	}
	return matches
}

type toolCallMatch struct {
	Index int
	Entry toolCallTraceEntry
}

func toolCallArgumentsContain(arguments json.RawMessage, fragment json.RawMessage) bool {
	var actual any
	if err := json.Unmarshal(arguments, &actual); err != nil {
		return false
	}
	var expected any
	if err := json.Unmarshal(fragment, &expected); err != nil {
		return false
	}
	return jsonContains(actual, expected)
}

func jsonContains(actual any, expected any) bool {
	switch expectedTyped := expected.(type) {
	case map[string]any:
		actualTyped, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, expectedValue := range expectedTyped {
			actualValue, ok := actualTyped[key]
			if !ok || !jsonContains(actualValue, expectedValue) {
				return false
			}
		}
		return true
	case []any:
		actualTyped, ok := actual.([]any)
		if !ok || len(expectedTyped) > len(actualTyped) {
			return false
		}
		used := make([]bool, len(actualTyped))
		for _, expectedValue := range expectedTyped {
			found := false
			for i, actualValue := range actualTyped {
				if used[i] {
					continue
				}
				if jsonContains(actualValue, expectedValue) {
					used[i] = true
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(actual, expected)
	}
}

func evaluateToolOrder(cfg toolCallAssertionConfig, trace []toolCallTraceEntry) (bool, string) {
	if len(cfg.OrderedTools) == 0 {
		return true, ""
	}
	observed := successfulObservedToolNames(trace)
	switch cfg.OrderMode {
	case "", toolCallOrderModeSubsequence:
		if containsToolSubsequence(observed, cfg.OrderedTools) {
			return true, ""
		}
		return false, fmt.Sprintf("observed tool order %v does not contain expected subsequence %v", observed, cfg.OrderedTools)
	case toolCallOrderModeExact:
		if reflect.DeepEqual(observed, cfg.OrderedTools) {
			return true, ""
		}
		return false, fmt.Sprintf("observed tool order %v does not exactly match expected order %v", observed, cfg.OrderedTools)
	default:
		return false, fmt.Sprintf("unsupported order_mode %q", cfg.OrderMode)
	}
}

func containsToolSubsequence(observed []string, expected []string) bool {
	if len(expected) == 0 {
		return true
	}
	next := 0
	for _, tool := range observed {
		if tool == expected[next] {
			next++
			if next == len(expected) {
				return true
			}
		}
	}
	return false
}

func sourceForFirstMatchedToolCall(matches []toolCallMatch) *Source {
	if len(matches) == 0 {
		return nil
	}
	entry := matches[0].Entry
	if entry.Sequence <= 0 {
		return nil
	}
	return &Source{
		Kind:      SourceKindToolCall,
		Sequence:  int64Ptr(entry.Sequence),
		EventType: entry.EventType,
		FieldPath: "tool_calls",
	}
}

func matchedIndices(matches []toolCallMatch) []int {
	indices := make([]int, 0, len(matches))
	for _, match := range matches {
		indices = append(indices, match.Index)
	}
	return indices
}

func successfulObservedToolNames(trace []toolCallTraceEntry) []string {
	names := make([]string, 0, len(trace))
	for _, entry := range trace {
		if entry.Failed {
			continue
		}
		names = append(names, entry.ToolName)
	}
	return names
}

func emptyNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nonEmptyStringsOrNil(values []string) any {
	if len(values) == 0 {
		return nil
	}
	return values
}
