package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/runtime/scoring"
)

const gradingParserVersion = "grounded-judge-v1"
const gradingNormalizationVersion = "scoring-spec-v1+conversation-crlf-trim-v1"

// Resolved from the operator-approved model profile at admission. Pricing,
// display names and metadata expiry affect availability, not grading identity.
type ModelExecution struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Route            string  `json:"route"`
	Temperature      float64 `json:"temperature"`
	MaxOutput        int     `json:"max_output"`
	DisableReasoning bool    `json:"disable_reasoning"`
}
type GradingContract struct {
	Version       int            `json:"version"`
	Hash          string         `json:"hash"`
	CriteriaHash  string         `json:"criteria_hash"`
	TestsHash     string         `json:"tests_hash"`
	PromptHash    string         `json:"prompt_hash"`
	Schema        string         `json:"schema"`
	Parser        string         `json:"parser"`
	Normalization string         `json:"normalization"`
	Aggregation   string         `json:"aggregation"`
	Evaluator     ModelExecution `json:"evaluator"`
}
type TargetConfiguration struct {
	ModelExecution
	InstructionsHash string `json:"instructions_hash"`
}

func modelExecution(profile ModelProfile, l Limits) ModelExecution {
	return ModelExecution{Provider: "openrouter", Model: profile.ID, Route: profile.Route, Temperature: 0, MaxOutput: l.OutputTokens, DisableReasoning: profile.DisableReasoning}
}
func (g GradingContract) fingerprint() string {
	g.Hash = ""
	return Hash(raw(g))
}
func gradingSupported(g *GradingContract) bool {
	return g != nil && g.Version == 1 && g.Hash == g.fingerprint() && g.Parser == gradingParserVersion && g.Schema == "judge-finding-json-v1" && g.Normalization == gradingNormalizationVersion && g.Aggregation == "all-checks-v1" && g.PromptHash == Hash([]byte(groundedJudgeInstruction+groundedSingleInstruction+groundedConversationInstruction))
}

func (s *Service) freezeGrading(p *Plan) error {
	if !s.Config.GroundedJudging {
		return nil
	}
	profile, err := s.Config.Profile(p.Submission.Models.Evaluator)
	if err != nil {
		return err
	}
	g := &GradingContract{Version: 1, Schema: "judge-finding-json-v1", Parser: gradingParserVersion, Normalization: gradingNormalizationVersion, Aggregation: "all-checks-v1", PromptHash: Hash([]byte(groundedJudgeInstruction + groundedSingleInstruction + groundedConversationInstruction)), Evaluator: modelExecution(profile, p.limits())}
	if p.Evidence != nil {
		g.CriteriaHash = Hash(raw(p.Artifact.ConversationEvaluation.Expectations))
		g.TestsHash = Hash(raw(evidenceComparisonInputs(*p.Evidence)))
	} else {
		compiled, err := s.Compiler.Compile(p.Artifact.Blueprint, p.Submission.Models.Evaluator, p.Artifact.ID, p.limits())
		if err != nil {
			return err
		}
		var normalized scoring.EvaluationSpec
		normalized, err = scoring.DecodeDefinition(raw(compiled.Bundle.Version.EvaluationSpec))
		if err != nil {
			return err
		}
		g.CriteriaHash, err = gradingCriteriaHash(normalized)
		if err != nil {
			return err
		}
		g.TestsHash, err = CanonicalJSONHash(p.Artifact.Blueprint)
		if err != nil {
			return err
		}
		if p.RegradeOf == nil {
			target, err := s.Config.Profile(p.Submission.Models.Target)
			if err != nil {
				return err
			}
			instructions := p.Artifact.AgentPrompt
			if !p.Artifact.IsTestSuite() {
				instructions = PreviewPrompt(instructions)
			}
			p.TargetConfig = &TargetConfiguration{ModelExecution: modelExecution(target, p.limits()), InstructionsHash: Hash([]byte(instructions))}
		}
	}
	g.Hash = g.fingerprint()
	p.Grading = g
	return nil
}

func (s *Service) verifyComparison(ctx context.Context, p Plan) error {
	if p.Submission.Kind != "retest" || p.Submission.BaselineID == nil {
		return nil
	}
	original, err := s.Store.Operation(ctx, *p.Submission.BaselineID)
	if err != nil {
		return err
	}
	var old Plan
	if err = json.Unmarshal(original.Input, &old); err != nil {
		return err
	}
	if p.Grading == nil && old.Grading == nil {
		return nil
	} // Historical contract; the UI labels comparability as unverified.
	if !gradingSupported(p.Grading) || !gradingSupported(old.Grading) || old.Grading.Hash != p.Grading.Hash {
		return fault("comparison_changed", "The grading settings changed. Start a new check, or recheck the saved grades. Your earlier results are preserved.")
	}
	return nil
}

func validateGradingDispatch(p Plan, role Role, profile ModelProfile) error {
	if p.Grading == nil {
		return nil
	}
	if !gradingSupported(p.Grading) {
		return fault("grading_changed", "This saved check needs its original grading version. Start a new check; earlier results are unchanged.")
	}
	var expected *ModelExecution
	if role == Evaluator {
		expected = &p.Grading.Evaluator
	}
	if role == Target && p.TargetConfig != nil {
		expected = &p.TargetConfig.ModelExecution
	}
	if expected != nil && *expected != modelExecution(profile, p.limits()) {
		return fault("model_policy_changed", "The model or provider settings changed after this check was prepared. Start a new check with the current settings.")
	}
	return nil
}

func evidenceComparisonInputs(e EvidenceSet) []any {
	list := []any{}
	for _, c := range e.Conversations {
		turns := []string{}
		for _, m := range c.Messages {
			if m.Role == "assistant" {
				turns = append(turns, "assistant")
			} else {
				turns = append(turns, m.Role+":"+strings.TrimSpace(strings.ReplaceAll(m.Content, "\r\n", "\n")))
			}
		}
		list = append(list, turns)
	}
	return list
}

func (p Plan) savedOutput(key string) (string, error) {
	for _, r := range p.SavedResults {
		if r.CaseKey == key && r.Version == p.Artifact.ID.String() && r.Output != "" {
			return r.Output, nil
		}
	}
	return "", fmt.Errorf("saved reply unavailable")
}

// A generated spec name contains the artifact UUID. It labels the run, not its
// grading rules; fixing agent instructions must not manufacture a new grader.
func gradingCriteriaHash(spec scoring.EvaluationSpec) (string, error) {
	spec.Name = ""
	return CanonicalJSONHash(raw(spec))
}
