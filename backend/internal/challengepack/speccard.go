package challengepack

import (
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/scoring"
)

// SpecCard is a human-readable, structured summary of a composed pack — the
// "readable rubric" the builder renders in its preview pane instead of raw
// YAML. It is derived from a Bundle; the frontend computes an equivalent
// client-side for instant feedback, and this backend version is authoritative
// on the compile endpoint.
type SpecCard struct {
	PackName       string              `json:"pack_name"`
	Slug           string              `json:"slug"`
	Family         string              `json:"family"`
	Description    string              `json:"description,omitempty"`
	ExecutionMode  string              `json:"execution_mode"`
	ChallengeCount int                 `json:"challenge_count"`
	CaseCount      int                 `json:"case_count"`
	ValidatorCount int                 `json:"validator_count"`
	JudgeCount     int                 `json:"judge_count"`
	Strategy       string              `json:"strategy"`
	Dimensions     []SpecCardDimension `json:"dimensions"`
	PassCriteria   string              `json:"pass_criteria"`
}

// SpecCardDimension is one readable scoring axis on the card.
type SpecCardDimension struct {
	Key           string   `json:"key"`
	Source        string   `json:"source"`
	Weight        *float64 `json:"weight,omitempty"`
	Gate          bool     `json:"gate"`
	PassThreshold *float64 `json:"pass_threshold,omitempty"`
	References    []string `json:"references,omitempty"`
	Summary       string   `json:"summary"`
}

// SpecCardSummary derives a SpecCard from a composed Bundle.
func SpecCardSummary(bundle Bundle) SpecCard {
	spec := bundle.Version.EvaluationSpec

	caseCount := 0
	for _, inputSet := range bundle.InputSets {
		caseCount += len(inputSet.Cases)
	}

	strategy := spec.Scorecard.Strategy
	if strategy == "" {
		strategy = scoring.ScoringStrategyWeighted
	}

	dimensions := make([]SpecCardDimension, 0, len(spec.Scorecard.Dimensions))
	for _, d := range spec.Scorecard.Dimensions {
		dimensions = append(dimensions, SpecCardDimension{
			Key:           d.Key,
			Source:        string(d.Source),
			Weight:        d.Weight,
			Gate:          d.Gate || strategy == scoring.ScoringStrategyBinary,
			PassThreshold: d.PassThreshold,
			References:    dimensionReferences(d),
			Summary:       dimensionSummary(d, strategy),
		})
	}

	description := ""
	if bundle.Pack.Description != nil {
		description = *bundle.Pack.Description
	}

	return SpecCard{
		PackName:       bundle.Pack.Name,
		Slug:           bundle.Pack.Slug,
		Family:         bundle.Pack.Family,
		Description:    description,
		ExecutionMode:  bundle.Version.ExecutionMode,
		ChallengeCount: len(bundle.Challenges),
		CaseCount:      caseCount,
		ValidatorCount: len(spec.Validators),
		JudgeCount:     len(spec.LLMJudges),
		Strategy:       string(strategy),
		Dimensions:     dimensions,
		PassCriteria:   passCriteria(spec, strategy),
	}
}

func dimensionReferences(d scoring.DimensionDeclaration) []string {
	switch d.Source {
	case scoring.DimensionSourceValidators:
		return append([]string(nil), d.Validators...)
	case scoring.DimensionSourceLLMJudge:
		if d.JudgeKey != "" {
			return []string{d.JudgeKey}
		}
	case scoring.DimensionSourceMetric:
		if d.Metric != "" {
			return []string{d.Metric}
		}
	}
	return nil
}

func dimensionSummary(d scoring.DimensionDeclaration, strategy scoring.ScoringStrategy) string {
	gate := d.Gate || strategy == scoring.ScoringStrategyBinary
	summary := fmt.Sprintf("%q", d.Key)
	switch d.Source {
	case scoring.DimensionSourceValidators:
		summary += fmt.Sprintf(" — %d validator(s)", len(d.Validators))
	case scoring.DimensionSourceLLMJudge:
		summary += fmt.Sprintf(" — LLM judge %q", d.JudgeKey)
	case scoring.DimensionSourceMetric:
		summary += fmt.Sprintf(" — metric %q", d.Metric)
	default:
		if d.Source != "" {
			summary += fmt.Sprintf(" — %s", d.Source)
		}
	}
	switch {
	case gate && d.PassThreshold != nil:
		summary += fmt.Sprintf("; gate ≥ %.2f", *d.PassThreshold)
	case gate:
		summary += "; gate"
	case d.Weight != nil:
		summary += fmt.Sprintf("; weight %.2f", *d.Weight)
	}
	return summary
}

func passCriteria(spec scoring.EvaluationSpec, strategy scoring.ScoringStrategy) string {
	switch strategy {
	case scoring.ScoringStrategyBinary:
		return "Pass/fail: the agent passes only if every dimension clears its threshold."
	case scoring.ScoringStrategyHybrid:
		s := "Hybrid scoring: every gate dimension must pass"
		if spec.Scorecard.PassThreshold != nil {
			s += fmt.Sprintf(", and the weighted score must reach %.2f", *spec.Scorecard.PassThreshold)
		}
		return s + "."
	default:
		if spec.Scorecard.PassThreshold != nil {
			return fmt.Sprintf("Weighted score across %d dimension(s); the agent passes at ≥ %.2f overall.", len(spec.Scorecard.Dimensions), *spec.Scorecard.PassThreshold)
		}
		return fmt.Sprintf("Weighted score across %d dimension(s); passes when no gated dimension fails.", len(spec.Scorecard.Dimensions))
	}
}
