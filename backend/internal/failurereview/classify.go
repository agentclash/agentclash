package failurereview

import "strings"

type FailureClass string

const (
	FailureClassIncorrectFinalOutput  FailureClass = "incorrect_final_output"
	FailureClassToolSelectionError    FailureClass = "tool_selection_error"
	FailureClassToolArgumentError     FailureClass = "tool_argument_error"
	FailureClassRetrievalGrounding    FailureClass = "retrieval_grounding_failure"
	FailureClassPolicyViolation       FailureClass = "policy_violation"
	FailureClassTimeoutOrBudget       FailureClass = "timeout_or_budget_exhaustion"
	FailureClassSandboxFailure        FailureClass = "sandbox_failure"
	FailureClassMalformedOutput       FailureClass = "malformed_output"
	FailureClassFlakyNonDeterministic FailureClass = "flaky_non_deterministic"
	FailureClassInsufficientEvidence  FailureClass = "insufficient_evidence"
	FailureClassOther                 FailureClass = "other"
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

// TaxonomyForFailureClass keeps the triage family separate from whether the
// failure should normally count against the evaluated agent.
func TaxonomyForFailureClass(class FailureClass) FailureTaxonomy {
	switch class {
	case FailureClassIncorrectFinalOutput:
		return FailureTaxonomy{Family: "agent", Code: "agent.output_incorrect", Label: "Incorrect final output", AgentFault: true}
	case FailureClassToolSelectionError:
		return FailureTaxonomy{Family: "agent", Code: "agent.tool_selection", Label: "Tool selection error", AgentFault: true}
	case FailureClassToolArgumentError:
		return FailureTaxonomy{Family: "agent", Code: "agent.tool_arguments", Label: "Tool argument error", AgentFault: true}
	case FailureClassRetrievalGrounding:
		return FailureTaxonomy{Family: "agent", Code: "agent.retrieval_grounding", Label: "Retrieval grounding failure", AgentFault: true}
	case FailureClassPolicyViolation:
		return FailureTaxonomy{Family: "agent", Code: "agent.policy_violation", Label: "Policy violation", AgentFault: true}
	case FailureClassTimeoutOrBudget:
		return FailureTaxonomy{Family: "workflow", Code: "workflow.timeout_budget", Label: "Timeout or budget exhaustion", AgentFault: true}
	case FailureClassSandboxFailure:
		return FailureTaxonomy{Family: "platform", Code: "platform.sandbox_failure", Label: "Sandbox failure", AgentFault: false}
	case FailureClassMalformedOutput:
		return FailureTaxonomy{Family: "agent", Code: "agent.malformed_output", Label: "Malformed output", AgentFault: true}
	case FailureClassFlakyNonDeterministic:
		return FailureTaxonomy{Family: "evidence", Code: "evidence.flaky_nondeterministic", Label: "Flaky or nondeterministic", AgentFault: false}
	case FailureClassInsufficientEvidence:
		return FailureTaxonomy{Family: "evidence", Code: "evidence.insufficient", Label: "Insufficient evidence", AgentFault: false}
	default:
		return FailureTaxonomy{Family: "agent", Code: "agent.other", Label: "Other agent failure", AgentFault: true}
	}
}

type ClassificationInput struct {
	EvidenceTier               EvidenceTier
	FailedChecks               []string
	HasStructuredFailureSignal bool
	HasTimeoutOrBudgetSignal   bool
	HasSandboxFailure          bool
	HasMalformedOutput         bool
	HasLLMFinalAnswerFailure   bool
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
