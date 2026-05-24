package engine

import (
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/scoring"
)

// evaluateTurnExpects returns true when any declared expectation fails (mismatch).
func evaluateTurnExpects(assistantText string, expects []challengepack.CaseExpectation) bool {
	if len(expects) == 0 {
		return false
	}
	for _, expectation := range expects {
		if !turnExpectationMet(assistantText, expectation) {
			return true
		}
	}
	return false
}

func turnExpectationMet(assistantText string, expectation challengepack.CaseExpectation) bool {
	expected, err := expectedTurnValue(expectation)
	if err != nil {
		return false
	}
	switch strings.TrimSpace(expectation.Kind) {
	case string(scoring.ValidatorTypeExactMatch):
		return strings.TrimSpace(assistantText) == expected
	case string(scoring.ValidatorTypeContains):
		return strings.Contains(assistantText, expected)
	default:
		return false
	}
}

func expectedTurnValue(expectation challengepack.CaseExpectation) (string, error) {
	if expectation.Value == nil {
		return "", fmt.Errorf("expectation value is required")
	}
	switch typed := expectation.Value.(type) {
	case string:
		return typed, nil
	case fmt.Stringer:
		return typed.String(), nil
	default:
		return fmt.Sprint(typed), nil
	}
}
