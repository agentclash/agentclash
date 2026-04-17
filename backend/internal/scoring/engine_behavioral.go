package scoring

import (
	"encoding/json"
	"fmt"
	"strings"
)

type toolCallTraceEntry struct {
	ToolName      string          `json:"tool_name"`
	Arguments     json.RawMessage `json:"arguments"`
	Failed        bool            `json:"failed"`
	FailureOrigin string          `json:"failure_origin,omitempty"`
	ErrorMessage  string          `json:"error_message,omitempty"`
}

func behavioralDimensionScore(spec EvaluationSpec, evidence extractedEvidence, validators []ValidatorResult) (*float64, string, OutputState) {
	if spec.Behavioral == nil || len(spec.Behavioral.Signals) == 0 {
		return nil, "behavioral config is unavailable", OutputStateUnavailable
	}

	var (
		totalWeight  float64
		weightedSum  float64
		gateFailures []string
	)

	for _, signal := range spec.Behavioral.Signals {
		score, reason, state := behavioralSignalScore(signal.Key, evidence.toolCallTrace, validators)
		if state != OutputStateAvailable || score == nil {
			return nil, fmt.Sprintf("behavioral signal %q is unavailable: %s", signal.Key, firstNonEmpty(reason, "signal is unavailable")), OutputStateUnavailable
		}
		totalWeight += signal.Weight
		weightedSum += signal.Weight * *score
		if signal.Gate && signal.PassThreshold != nil && *score < *signal.PassThreshold {
			gateFailures = append(gateFailures, fmt.Sprintf("%s=%.3f < %.3f", signal.Key, *score, *signal.PassThreshold))
		}
	}

	if totalWeight <= 0 {
		return nil, "behavioral weights are unavailable", OutputStateUnavailable
	}

	score := weightedSum / totalWeight
	if len(gateFailures) > 0 {
		score = 0
		return &score, "behavioral gate failed: " + strings.Join(gateFailures, ", "), OutputStateAvailable
	}
	return &score, "", OutputStateAvailable
}

func behavioralSignalScore(key BehavioralSignalKey, trace []toolCallTraceEntry, validators []ValidatorResult) (*float64, string, OutputState) {
	switch key {
	case BehavioralSignalRecoveryBehavior:
		return recoveryBehaviorScore(trace)
	case BehavioralSignalExplorationEfficiency:
		return explorationEfficiencyScore(trace)
	case BehavioralSignalErrorCascade:
		return errorCascadeScore(trace)
	case BehavioralSignalScopeAdherence:
		return scopeAdherenceScore(trace)
	case BehavioralSignalConfidenceCalibration:
		return confidenceCalibrationScore(trace, validators)
	default:
		return nil, fmt.Sprintf("unsupported behavioral signal %q", key), OutputStateError
	}
}

func recoveryBehaviorScore(trace []toolCallTraceEntry) (*float64, string, OutputState) {
	failureCount := 0
	recoveryAttempts := 0
	adaptiveRecoveries := 0
	for i := range trace {
		if !trace[i].Failed {
			continue
		}
		failureCount++
		if i+1 >= len(trace) {
			continue
		}
		recoveryAttempts++
		if !sameToolCall(trace[i], trace[i+1]) {
			adaptiveRecoveries++
		}
	}
	if failureCount == 0 {
		return floatPtr(1), "", OutputStateAvailable
	}
	if recoveryAttempts == 0 {
		return floatPtr(0), "no recovery attempts followed the observed failures", OutputStateAvailable
	}
	score := float64(adaptiveRecoveries) / float64(recoveryAttempts)
	return &score, "", OutputStateAvailable
}

func explorationEfficiencyScore(trace []toolCallTraceEntry) (*float64, string, OutputState) {
	if len(trace) == 0 {
		return floatPtr(1), "", OutputStateAvailable
	}
	seen := make(map[string]struct{}, len(trace))
	duplicateCalls := 0
	for _, entry := range trace {
		signature := toolCallSignature(entry)
		if _, ok := seen[signature]; ok {
			duplicateCalls++
			continue
		}
		seen[signature] = struct{}{}
	}
	score := 1 - (float64(duplicateCalls) / float64(len(trace)))
	return &score, "", OutputStateAvailable
}

func errorCascadeScore(trace []toolCallTraceEntry) (*float64, string, OutputState) {
	maxConsecutiveFailures := 0
	currentStreak := 0
	for _, entry := range trace {
		if entry.Failed {
			currentStreak++
			if currentStreak > maxConsecutiveFailures {
				maxConsecutiveFailures = currentStreak
			}
			continue
		}
		currentStreak = 0
	}
	if maxConsecutiveFailures == 0 {
		return floatPtr(1), "", OutputStateAvailable
	}
	score := 1 / float64(maxConsecutiveFailures)
	return &score, "", OutputStateAvailable
}

func scopeAdherenceScore(trace []toolCallTraceEntry) (*float64, string, OutputState) {
	if len(trace) == 0 {
		return floatPtr(1), "", OutputStateAvailable
	}
	outOfScopeCalls := 0
	for _, entry := range trace {
		if isOutOfScopeToolCall(entry) {
			outOfScopeCalls++
		}
	}
	score := 1 - (float64(outOfScopeCalls) / float64(len(trace)))
	return &score, "", OutputStateAvailable
}

func confidenceCalibrationScore(trace []toolCallTraceEntry, validators []ValidatorResult) (*float64, string, OutputState) {
	confidence, ok := latestConfidenceReport(trace)
	if !ok {
		return nil, "report_confidence evidence is unavailable", OutputStateUnavailable
	}
	outcome, reason, ok := validatorBinaryOutcome(validators)
	if !ok {
		return nil, firstNonEmpty(reason, "validator outcome is unavailable"), OutputStateUnavailable
	}
	brier := (confidence - outcome) * (confidence - outcome)
	score := 1 - brier
	if score < 0 {
		score = 0
	}
	return &score, "", OutputStateAvailable
}

func latestConfidenceReport(trace []toolCallTraceEntry) (float64, bool) {
	for i := len(trace) - 1; i >= 0; i-- {
		entry := trace[i]
		if entry.Failed || strings.TrimSpace(entry.ToolName) != "report_confidence" {
			continue
		}
		payload := decodePayload(entry.Arguments)
		for _, key := range []string{"confidence", "score"} {
			if value, ok := numericValue(payload, key); ok && value >= 0 && value <= 1 {
				return value, true
			}
		}
	}
	return 0, false
}

func validatorBinaryOutcome(validators []ValidatorResult) (float64, string, bool) {
	if len(validators) == 0 {
		return 0, "validator outcome is unavailable", false
	}
	for _, validator := range validators {
		if validator.State != OutputStateAvailable || validator.NormalizedScore == nil {
			return 0, "validator outcome requires all validators to be available", false
		}
		if *validator.NormalizedScore < 1 {
			return 0, "", true
		}
	}
	return 1, "", true
}

func isOutOfScopeToolCall(entry toolCallTraceEntry) bool {
	if !entry.Failed {
		return false
	}
	errorMessage := strings.ToLower(strings.TrimSpace(entry.ErrorMessage))
	return strings.Contains(errorMessage, "tool is not allowed in this runtime") ||
		strings.Contains(errorMessage, "tool is not available in this runtime")
}

func sameToolCall(left toolCallTraceEntry, right toolCallTraceEntry) bool {
	return toolCallSignature(left) == toolCallSignature(right)
}

func toolCallSignature(entry toolCallTraceEntry) string {
	return strings.TrimSpace(entry.ToolName) + "\x00" + strings.TrimSpace(string(normalizeToolCallArguments(entry.Arguments)))
}

func normalizeToolCallArguments(raw json.RawMessage) json.RawMessage {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return json.RawMessage(`{}`)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return json.RawMessage(`{}`)
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return normalized
}

func buildToolCallTraceEntry(payload map[string]any, eventType string) (toolCallTraceEntry, bool) {
	toolName, ok := stringValue(payload, "tool_name")
	if !ok || strings.TrimSpace(toolName) == "" {
		return toolCallTraceEntry{}, false
	}

	var args json.RawMessage
	if raw, exists := payload["arguments"]; exists {
		args = mustMarshalJSON(raw)
	}
	args = normalizeToolCallArguments(args)

	entry := toolCallTraceEntry{
		ToolName:  strings.TrimSpace(toolName),
		Arguments: args,
		Failed:    eventType == "tool.call.failed",
	}
	if failureOrigin, ok := stringValue(payload, "failure_origin"); ok {
		entry.FailureOrigin = strings.TrimSpace(failureOrigin)
	}
	if result, ok := payload["result"].(map[string]any); ok {
		if isError, ok := result["is_error"].(bool); ok {
			entry.Failed = isError || entry.Failed
		}
		if content, ok := extractLooseString(result["content"]); ok {
			entry.ErrorMessage = decodeBehavioralToolError(content)
		}
	}
	return entry, true
}

func decodeBehavioralToolError(content string) string {
	payload := decodePayload(json.RawMessage(content))
	if errorMessage, ok := stringValue(payload, "error"); ok {
		return strings.TrimSpace(errorMessage)
	}
	return strings.TrimSpace(content)
}
