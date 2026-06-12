package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Public-tryout LLM-as-judge selection. Anonymous visitors pick a judge model
// from a small hosted allowlist (platform keys only — never user keys) and a
// strictness level. The selection is baked into the tryout's
// evaluation_spec_snapshot at create time as standard llm_judges declarations,
// so the worker can reuse the existing judge machinery unchanged.

const (
	defaultPublicJudgeModel      = "gpt-5-mini"
	defaultPublicJudgeStrictness = "standard"
	publicJudgeTimeoutMS         = 45_000

	publicJudgeKeyOverall     = "overall_quality"
	publicJudgeKeyInstantFail = "instant_fail"
	publicJudgeKeyReviewer    = "reviewer_bar"
)

// defaultPublicJudgeModels is the hosted judge allowlist. Models must be cheap
// (they run on platform keys for anonymous visitors) and their names must be
// inferable to a provider by the worker's judge credential resolution
// (claude-* -> anthropic, gpt-* -> openai, gemini-* -> gemini).
func defaultPublicJudgeModels() []string {
	return []string{"gpt-5-mini", "claude-haiku-4-5", "gemini-2.5-flash"}
}

type AgentTryoutJudgeSelection struct {
	Model      string `json:"model"`
	Strictness string `json:"strictness,omitempty"`
}

// normalizePublicTryoutJudge validates a judge selection against the hosted
// allowlist. A nil selection enables the default judge, so every public tryout
// ends with a verdict. Returns ErrInvalidAgentTryoutInput on unknown models or
// strictness values.
func normalizePublicTryoutJudge(selection *AgentTryoutJudgeSelection, allowedModels []string) (AgentTryoutJudgeSelection, error) {
	normalized := AgentTryoutJudgeSelection{
		Model:      defaultPublicJudgeModel,
		Strictness: defaultPublicJudgeStrictness,
	}
	if len(allowedModels) > 0 {
		normalized.Model = allowedModels[0]
	}
	if selection == nil {
		return normalized, nil
	}

	if model := strings.TrimSpace(selection.Model); model != "" {
		allowed := false
		for _, candidate := range allowedModels {
			if strings.EqualFold(candidate, model) {
				normalized.Model = candidate
				allowed = true
				break
			}
		}
		if !allowed {
			return AgentTryoutJudgeSelection{}, fmt.Errorf(
				"%w: judge model %q is not available; choose one of %s",
				ErrInvalidAgentTryoutInput, model, strings.Join(allowedModels, ", "),
			)
		}
	}

	switch strictness := strings.ToLower(strings.TrimSpace(selection.Strictness)); strictness {
	case "":
		// keep default
	case "lenient", "standard", "harsh":
		normalized.Strictness = strictness
	default:
		return AgentTryoutJudgeSelection{}, fmt.Errorf(
			"%w: judge strictness %q is not supported; use lenient, standard, or harsh",
			ErrInvalidAgentTryoutInput, selection.Strictness,
		)
	}
	return normalized, nil
}

// publicTryoutEvalSetup is the non-engineer questionnaire the playground embeds
// in the tryout input. Every field is optional; the judge rubric degrades to a
// sensible generic bar when answers are missing.
type publicTryoutEvalSetup struct {
	UnacceptableMistakes string `json:"unacceptable_mistakes"`
	HumanReviewer        string `json:"human_reviewer"`
	BusinessPriority     string `json:"business_priority"`
	OutputStyle          string `json:"output_style"`
	MonthlyVolume        string `json:"monthly_volume"`
}

func decodePublicTryoutEvalSetup(input json.RawMessage) publicTryoutEvalSetup {
	var wrapper struct {
		EvalSetup publicTryoutEvalSetup `json:"eval_setup"`
	}
	_ = json.Unmarshal(input, &wrapper)
	return wrapper.EvalSetup
}

func publicJudgeStrictnessClause(strictness string) string {
	switch strictness {
	case "lenient":
		return "Grade generously: only clear violations of the operator's bar should fail the work; do not penalize minor stylistic issues."
	case "harsh":
		return "Grade ruthlessly: any violation of the operator's bar, sloppiness, placeholder content, or unsupported claim fails the work."
	default:
		return "Grade like a careful professional reviewer: real violations of the operator's bar fail the work, nitpicks do not."
	}
}

func publicJudgePriorityClause(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "polish":
		return "Top priority: the result must look client-ready with clear structure and an appropriate tone."
	case "speed":
		return "Top priority: the result must be complete and immediately usable without extra back-and-forth."
	case "cost":
		return "Top priority: the result must justify automation, with no wasteful filler or padding."
	case "compliance":
		return "Top priority: the result must stay inside stated policies and flag risky or missing information."
	default:
		return "Top priority: the facts must be correct, grounded in the provided input, with nothing fabricated."
	}
}

func publicJudgeStyleClause(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "creative":
		return "Creative variation is welcome as long as the core requirements are met."
	case "balanced":
		return "A balance of consistency and judgment is expected."
	default:
		return "Output is expected to be consistent and repeatable, in a form that could be produced the same way every time."
	}
}

// publicTryoutJudgeDeclarations derives the llm_judges declarations from the
// operator's own answers: one overall rubric judge plus assertion judges for
// the named instant-fail mistake and the sign-off reviewer when provided. The
// declarations use the exact JSON shape scoring.LLMJudgeDeclaration decodes.
func publicTryoutJudgeDeclarations(judge AgentTryoutJudgeSelection, setup publicTryoutEvalSetup, taskName string) ([]map[string]any, map[string]string) {
	mistake := strings.TrimSpace(setup.UnacceptableMistakes)
	reviewer := strings.TrimSpace(setup.HumanReviewer)
	task := strings.TrimSpace(taskName)
	if task == "" {
		task = "an office-work task"
	}

	base := func(key string) map[string]any {
		return map[string]any{
			"key":          key,
			"model":        judge.Model,
			"context_from": []string{"final_output"},
			"samples":      1,
			"timeout_ms":   publicJudgeTimeoutMS,
		}
	}

	rubricLines := []string{
		"You are grading the output of an AI agent that completed office work for a non-technical operator.",
		fmt.Sprintf("Task: %s.", task),
		publicJudgeStrictnessClause(judge.Strictness),
		"The operator's bar:",
	}
	if mistake != "" {
		rubricLines = append(rubricLines, fmt.Sprintf("- Unforgivable mistake (instant fail): %s.", mistake))
	} else {
		rubricLines = append(rubricLines, "- Unforgivable mistake (instant fail): fabricated facts presented as real.")
	}
	if reviewer != "" {
		rubricLines = append(rubricLines, fmt.Sprintf("- The person who must be able to sign off without embarrassment: %s.", reviewer))
	}
	rubricLines = append(rubricLines,
		fmt.Sprintf("- %s", publicJudgePriorityClause(setup.BusinessPriority)),
		fmt.Sprintf("- %s", publicJudgeStyleClause(setup.OutputStyle)),
		"Score 1-5: 5 = approvable without edits; 3 = usable after minor edits; 1 = commits the unforgivable mistake or misses the task.",
	)

	overall := base(publicJudgeKeyOverall)
	overall["mode"] = "rubric"
	overall["rubric"] = strings.Join(rubricLines, "\n")
	overall["score_scale"] = map[string]float64{"min": 1, "max": 5}

	declarations := []map[string]any{overall}
	labels := map[string]string{publicJudgeKeyOverall: "Overall, against your bar"}

	if mistake != "" {
		instantFail := base(publicJudgeKeyInstantFail)
		instantFail["mode"] = "assertion"
		instantFail["assertion"] = fmt.Sprintf(
			"%s The output does not commit the operator's unforgivable mistake: %s.",
			publicJudgeStrictnessClause(judge.Strictness), mistake,
		)
		declarations = append(declarations, instantFail)
		labels[publicJudgeKeyInstantFail] = fmt.Sprintf("Avoids your instant fail: %s", truncatePublicJudgeLabel(mistake, 60))
	}
	if reviewer != "" {
		reviewerBar := base(publicJudgeKeyReviewer)
		reviewerBar["mode"] = "assertion"
		reviewerBar["assertion"] = fmt.Sprintf(
			"%s A reviewer in the role of %q would accept this output for the task %q with at most minor edits.",
			publicJudgeStrictnessClause(judge.Strictness), reviewer, task,
		)
		declarations = append(declarations, reviewerBar)
		labels[publicJudgeKeyReviewer] = fmt.Sprintf("%s would sign off", reviewer)
	}
	return declarations, labels
}

func truncatePublicJudgeLabel(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

// evaluationSpecWithPublicJudge merges the judge declarations into the
// template's evaluation spec snapshot. The result keeps the deterministic
// validators untouched and adds llm_judges + judge_meta, which the public
// tryout workflow decodes with the same helpers as harness runs.
func evaluationSpecWithPublicJudge(spec json.RawMessage, judge AgentTryoutJudgeSelection, input json.RawMessage, taskName string) json.RawMessage {
	merged := map[string]any{}
	if len(spec) > 0 {
		if err := json.Unmarshal(spec, &merged); err != nil {
			merged = map[string]any{}
		}
	}

	declarations, labels := publicTryoutJudgeDeclarations(judge, decodePublicTryoutEvalSetup(input), taskName)
	merged["judge_mode"] = "hybrid"
	merged["llm_judges"] = declarations
	merged["judge_meta"] = map[string]any{
		"model":      judge.Model,
		"strictness": judge.Strictness,
		"labels":     labels,
	}

	encoded, err := json.Marshal(merged)
	if err != nil {
		return spec
	}
	return encoded
}
