package vibe

import (
	"context"
	"encoding/json"
)

// Regrading creates an independently funded run of the exact saved replies and
// criteria. Neither the source run nor its artifact is rewritten. It never
// calls the target or the author, including when the old target is unavailable.
func (s *Service) prepareRegrade(ctx context.Context, actor string, v Session, sub Submission, p Plan) (Operation, error) {
	if !s.Config.GroundedJudging {
		return Operation{}, fault("regrade_unavailable", "Rechecking saved grades is unavailable with the current settings.")
	}
	original, err := s.Store.Operation(ctx, *sub.BaselineID)
	if err != nil || original.Actor != actor || original.SessionID != v.ID || !original.State.Terminal() || (original.Kind != "check" && original.Kind != "retest") {
		return Operation{}, fault("baseline_required", "Choose a saved result from this conversation to recheck its grades.")
	}
	var old Plan
	if err = json.Unmarshal(original.Input, &old); err != nil {
		return Operation{}, err
	}
	if old.Artifact == nil || sub.ArtifactID == nil || *sub.ArtifactID != old.Artifact.ID {
		return Operation{}, fault("baseline_required", "Choose the tests used for this saved result.")
	}
	p.Artifact = old.Artifact
	p.Evidence = old.Evidence
	p.RegradeOf = &original.ID
	p.TargetConfig = old.TargetConfig
	if old.Source != nil {
		copy := *old.Source
		p.Source = &copy
	} else {
		p.Source = &EvaluationSource{Kind: "prompt", ArtifactID: old.Artifact.ID, Label: "Saved text replies"}
	}
	p.Source.Comparison = "regraded"
	p.Document = Document{}
	p.AuthoringVersion = 0
	l := p.limits()
	if p.Evidence != nil {
		if !p.Artifact.IsConversationEvaluation() {
			return Operation{}, fault("invalid_evaluation", "The saved expectations are unavailable.")
		}
		if err = p.Evidence.ValidateReady(); err != nil {
			return Operation{}, err
		}
		p.ConversationJudgeVersion = 1
		p.ChecksPerCase = len(p.Artifact.ConversationEvaluation.Expectations)
		for _, c := range p.Evidence.Conversations {
			p.Cases = append(p.Cases, c.Key)
			p.CasePreviews = append(p.CasePreviews, conversationResult(p, c))
		}
		p.Calls = len(p.Cases)
	} else {
		compiled, err := s.Compiler.Compile(p.Artifact.Blueprint, sub.Models.Evaluator, p.Artifact.ID, l)
		if err != nil {
			return Operation{}, err
		}
		judges := len(compiled.Bundle.Version.EvaluationSpec.LLMJudges)
		p.ChecksPerCase = judges + len(compiled.Bundle.Version.EvaluationSpec.Validators)
		for _, c := range compiled.Cases {
			p.Cases = append(p.Cases, c.CaseKey)
			p.CasePreviews = append(p.CasePreviews, CaseResult{CaseKey: c.CaseKey, Input: raw(c.Payload), Expected: ExpectedBehavior(c)})
			result, err := s.Store.GetCase(ctx, actor, original.ID, c.CaseKey)
			if err != nil {
				return Operation{}, fault("evidence_unavailable", "Some saved replies are unavailable. Reload the original result before rechecking.")
			}
			if result.Version != p.Artifact.ID.String() {
				return Operation{}, fault("evidence_unavailable", "The saved reply belongs to different tests.")
			}
			if result.Output != "" && (result.Error == nil || result.Error.Code == "invalid_judge_output") {
				p.SavedResults = append(p.SavedResults, CaseResult{CaseKey: result.CaseKey, Version: result.Version, Output: result.Output})
				p.Calls += judges
			}
		}
		if len(p.SavedResults) == 0 {
			return Operation{}, fault("evidence_unavailable", "No complete saved replies are available. Run the tests first.")
		}
	}
	if p.Calls == 0 {
		return Operation{}, fault("regrade_unavailable", "These replies have no AI grades to recheck. Your earlier results are unchanged.")
	}
	if len(p.Cases) > l.Cases || p.Calls > l.ModelCalls {
		return Operation{}, fault("operation_limit", "These saved results exceed the current check allowance.")
	}
	profile, err := s.Config.Profile(sub.Models.Evaluator)
	if err != nil {
		return Operation{}, err
	}
	cost, err := profile.BoundCost(profile.inputLimit(l), l.OutputTokens)
	if err != nil {
		return Operation{}, err
	}
	p.MaxCost = cost * int64(p.Calls)
	if err = s.freezeGrading(&p); err != nil {
		return Operation{}, err
	}
	return s.Store.Submit(ctx, actor, v.ID, sub, p, s.Config)
}
