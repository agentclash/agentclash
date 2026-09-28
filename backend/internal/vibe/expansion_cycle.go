package vibe

import "context"

// Expansion uses the same durable authorization/Stop/resume mechanism as Build.
// Its frozen baseline determines both the preparation scope and execution bound.
func (s *Service) expansionQuote(ctx context.Context, v Session, request BuildQuoteRequest) (int, int, string, error) {
	l := s.Config.Limits(v.Anonymous)
	if v.Document.FormatVersion != 1 || request.ArtifactID == nil || request.AdditionalExamples < 1 || request.AdditionalExamples > l.Cases {
		return 0, 0, "", fault("invalid_request", "Choose an existing agent and a bounded batch.")
	}
	for _, operation := range v.Operations {
		if !operation.State.Terminal() {
			return 0, 0, "", fault("operation_active", "Wait for the current work to finish.")
		}
	}
	for _, a := range v.Document.Artifacts {
		if a.ID != *request.ArtifactID {
			continue
		}
		if !a.IsTestSuite() || a.AgentPrompt == "" || a.UnavailableReason != "" {
			return 0, 0, "", fault("agent_required", "Add supported agent instructions before trying tougher situations.")
		}
		policy := policyFor(v.Document, &a)
		if policy == nil {
			return 0, 0, "", sourceReviewRequired()
		}
		if _, err := verifiedPolicySources(v.Document, *policy); err != nil {
			return 0, 0, "", err
		}
		compiled, err := s.Compiler.Compile(a.Blueprint, request.Models.Evaluator, a.ID, l)
		if err != nil {
			return 0, 0, "", err
		}
		count := len(compiled.Cases) + request.AdditionalExamples
		if count > l.Cases {
			return 0, 0, "", fault("case_limit", "Choose a smaller additional batch. Your existing examples are preserved.")
		}
		return count, len(compiled.Bundle.Version.EvaluationSpec.LLMJudges), Hash(raw(a)), nil
	}
	return 0, 0, "", fault("artifact_required", "Choose the agent version to extend.")
}
