package failurereview

import "strings"

type FailureClass string

const (
	FailureClassIncorrectFinalOutput     FailureClass = "incorrect_final_output"
	FailureClassToolSelectionError       FailureClass = "tool_selection_error"
	FailureClassToolArgumentError        FailureClass = "tool_argument_error"
	FailureClassRetrievalGrounding       FailureClass = "retrieval_grounding_failure"
	FailureClassPolicyViolation          FailureClass = "policy_violation"
	FailureClassTimeoutOrBudget          FailureClass = "timeout_or_budget_exhaustion"
	FailureClassSandboxFailure           FailureClass = "sandbox_failure"
	FailureClassMalformedOutput          FailureClass = "malformed_output"
	FailureClassFlakyNonDeterministic    FailureClass = "flaky_non_deterministic"
	FailureClassInsufficientEvidence     FailureClass = "insufficient_evidence"
	FailureClassOther                    FailureClass = "other"
)

func Classify(input ClassificationInput) FailureClass {
	if input.EvidenceTier == EvidenceTierHostedBlackBox && !input.HasStructuredFailureSignal {
		return FailureClassInsufficientEvidence
	}
	if input.HasTimeoutOrBudgetSignal {
		return FailureClassTimeoutOrBudget
	}
	if input.HasSandboxFailure {
		return FailureClassSandboxFailure
	}
	if input.HasMalformedOutput {
		return FailureClassMalformedOutput
	}
	if input.HasLLMFinalAnswerFailure {
		return FailureClassIncorrectFinalOutput
	}

	for _, check := range input.FailedChecks {
		normalized := strings.ToLower(strings.TrimSpace(check))
		switch {
		case strings.HasPrefix(normalized, "policy."):
			return FailureClassPolicyViolation
		case containsAny(normalized, "policy", "forbidden", "disallowed"):
			return FailureClassPolicyViolation
		case containsAny(normalized, "tool_argument", "tool-argument", "arguments", "argument", "schema"):
			return FailureClassToolArgumentError
		case containsAny(normalized, "tool_selection", "tool-selection", "wrong tool", "tool choice", "tool selection"):
			return FailureClassToolSelectionError
		case containsAny(normalized, "retrieval", "grounding"):
			return FailureClassRetrievalGrounding
		case containsAny(normalized, "flaky", "non-deterministic", "nondeterministic"):
			return FailureClassFlakyNonDeterministic
		}
	}

	return FailureClassOther
}

type ClassificationInput struct {
	EvidenceTier              EvidenceTier
	FailedChecks              []string
	HasStructuredFailureSignal bool
	HasTimeoutOrBudgetSignal  bool
	HasSandboxFailure         bool
	HasMalformedOutput        bool
	HasLLMFinalAnswerFailure  bool
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
