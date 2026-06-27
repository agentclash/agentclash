package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	AcceptanceModeJudge     = "judge"
	AcceptanceModeThreshold = "threshold"

	JudgeVerdictAccept  = "accept"
	JudgeVerdictImprove = "improve"
	JudgeVerdictReject  = "reject"
)

type AgenticJudgeInput struct {
	Seeds     []SeedExample
	Candidate Candidate
}

type AgenticJudgeVerdict struct {
	Verdict                 string          `json:"verdict"`
	QualityVerdict          string          `json:"quality_verdict,omitempty"`
	WeakScore               *float64        `json:"weak_score,omitempty"`
	StrongScore             *float64        `json:"strong_score,omitempty"`
	Gap                     *float64        `json:"gap,omitempty"`
	WeakPattern             string          `json:"weak_pattern,omitempty"`
	StrongPattern           string          `json:"strong_pattern,omitempty"`
	GapInterpretation       string          `json:"gap_interpretation,omitempty"`
	RubricConcerns          []string        `json:"rubric_concerns,omitempty"`
	CapabilityTags          []string        `json:"capability_tags,omitempty"`
	GRPOSuitability         string          `json:"grpo_suitability,omitempty"`
	SuggestionForChallenger string          `json:"suggestion_for_challenger,omitempty"`
	Raw                     json.RawMessage `json:"-"`
}

type AgenticAcceptanceConfig struct {
	Mode           string
	MinGap         *float64
	MaxWeakScore   *float64
	MinStrongScore *float64
}

func BuildAgenticJudgePrompt(input AgenticJudgeInput) string {
	var b strings.Builder
	b.WriteString("You are judging a synthetic AgentClash dataset example before it is accepted.\n")
	b.WriteString("Decide whether the candidate is high-quality, realistic, non-duplicative in spirit, and useful for evaluating an agent.\n")
	b.WriteString("This phase is judge-only: no weak or strong solver rollouts are available yet. Prefer rejecting vague, trivial, answer-leaking, impossible, or schema-inconsistent examples.\n")
	b.WriteString("Respond with ONLY valid JSON in this exact shape:\n")
	b.WriteString(`{"verdict":"accept|improve|reject","quality_verdict":"high|medium|low","weak_score":0.0,"strong_score":1.0,"gap":0.0,"weak_pattern":"","strong_pattern":"","gap_interpretation":"","rubric_concerns":[],"capability_tags":[],"grpo_suitability":"high|medium|low","suggestion_for_challenger":null}` + "\n")
	b.WriteString("Use null or omit scores when solver scores are not available. If verdict is improve or reject, include a concrete suggestion_for_challenger.\n\n")
	b.WriteString("Seed examples:\n")
	for i, seed := range input.Seeds {
		b.WriteString(fmt.Sprintf("%d. input: %s\n", i+1, compactJSON(seed.Input)))
		if len(seed.Expected) > 0 && string(seed.Expected) != "null" {
			b.WriteString(fmt.Sprintf("   expected: %s\n", compactJSON(seed.Expected)))
		}
	}
	b.WriteString("\nCandidate:\n")
	b.WriteString(fmt.Sprintf("input: %s\n", compactJSON(input.Candidate.Input)))
	if len(input.Candidate.Expected) > 0 && string(input.Candidate.Expected) != "null" {
		b.WriteString(fmt.Sprintf("expected: %s\n", compactJSON(input.Candidate.Expected)))
	}
	return b.String()
}

func ParseAgenticJudgeResponse(raw string) (AgenticJudgeVerdict, error) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var verdict AgenticJudgeVerdict
	if err := json.Unmarshal([]byte(text), &verdict); err != nil {
		return AgenticJudgeVerdict{}, fmt.Errorf("decode agentic judge response: %w", err)
	}
	verdict.Raw = json.RawMessage(text)
	switch verdict.Verdict {
	case JudgeVerdictAccept, JudgeVerdictImprove, JudgeVerdictReject:
	default:
		return AgenticJudgeVerdict{}, errors.New("agentic judge response has invalid verdict")
	}
	if err := validateOptionalScore("weak_score", verdict.WeakScore); err != nil {
		return AgenticJudgeVerdict{}, err
	}
	if err := validateOptionalScore("strong_score", verdict.StrongScore); err != nil {
		return AgenticJudgeVerdict{}, err
	}
	if err := validateOptionalScore("gap", verdict.Gap); err != nil {
		return AgenticJudgeVerdict{}, err
	}
	return verdict, nil
}

func ShouldAcceptJudgeVerdict(verdict AgenticJudgeVerdict, cfg AgenticAcceptanceConfig) bool {
	if verdict.Verdict != JudgeVerdictAccept {
		return false
	}
	if cfg.Mode == AcceptanceModeThreshold {
		if cfg.MinGap == nil || cfg.MaxWeakScore == nil || cfg.MinStrongScore == nil {
			return false
		}
	}
	if cfg.MinGap != nil {
		if verdict.Gap == nil || *verdict.Gap < *cfg.MinGap {
			return false
		}
	}
	if cfg.MaxWeakScore != nil {
		if verdict.WeakScore == nil || *verdict.WeakScore > *cfg.MaxWeakScore {
			return false
		}
	}
	if cfg.MinStrongScore != nil {
		if verdict.StrongScore == nil || *verdict.StrongScore < *cfg.MinStrongScore {
			return false
		}
	}
	return true
}

func AgenticJudgeRejectionDetail(verdict AgenticJudgeVerdict) string {
	if verdict.SuggestionForChallenger != "" {
		return verdict.SuggestionForChallenger
	}
	if verdict.GapInterpretation != "" {
		return verdict.GapInterpretation
	}
	if verdict.QualityVerdict != "" {
		return "judge quality verdict: " + verdict.QualityVerdict
	}
	return "agentic judge did not accept candidate"
}

func AgenticJudgeMetadata(verdict AgenticJudgeVerdict) json.RawMessage {
	metadata := map[string]any{
		"verdict":            verdict.Verdict,
		"quality_verdict":    verdict.QualityVerdict,
		"weak_score":         verdict.WeakScore,
		"strong_score":       verdict.StrongScore,
		"gap":                verdict.Gap,
		"weak_pattern":       verdict.WeakPattern,
		"strong_pattern":     verdict.StrongPattern,
		"gap_interpretation": verdict.GapInterpretation,
		"rubric_concerns":    verdict.RubricConcerns,
		"capability_tags":    verdict.CapabilityTags,
		"grpo_suitability":   verdict.GRPOSuitability,
	}
	if verdict.SuggestionForChallenger != "" {
		metadata["suggestion_for_challenger"] = verdict.SuggestionForChallenger
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

func validateOptionalScore(name string, value *float64) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > 1 {
		return fmt.Errorf("%s must be between 0 and 1", name)
	}
	return nil
}
