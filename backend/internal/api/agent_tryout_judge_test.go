package api

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNormalizePublicTryoutJudgeDefaults(t *testing.T) {
	judge, err := normalizePublicTryoutJudge(nil, defaultPublicJudgeModels())
	if err != nil {
		t.Fatalf("normalizePublicTryoutJudge() error = %v", err)
	}
	if judge.Model != defaultPublicJudgeModels()[0] {
		t.Fatalf("model = %q, want default %q", judge.Model, defaultPublicJudgeModels()[0])
	}
	if judge.Strictness != "standard" {
		t.Fatalf("strictness = %q, want standard", judge.Strictness)
	}
}

func TestNormalizePublicTryoutJudgeAcceptsAllowlistedModel(t *testing.T) {
	judge, err := normalizePublicTryoutJudge(&AgentTryoutJudgeSelection{
		Model:      "Claude-Haiku-4-5",
		Strictness: "harsh",
	}, defaultPublicJudgeModels())
	if err != nil {
		t.Fatalf("normalizePublicTryoutJudge() error = %v", err)
	}
	if judge.Model != "claude-haiku-4-5" {
		t.Fatalf("model = %q, want canonical claude-haiku-4-5", judge.Model)
	}
	if judge.Strictness != "harsh" {
		t.Fatalf("strictness = %q, want harsh", judge.Strictness)
	}
}

func TestNormalizePublicTryoutJudgeRejectsUnknownModel(t *testing.T) {
	_, err := normalizePublicTryoutJudge(&AgentTryoutJudgeSelection{Model: "gpt-5-pro-max"}, defaultPublicJudgeModels())
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("error = %v, want ErrInvalidAgentTryoutInput", err)
	}
}

func TestNormalizePublicTryoutJudgeRejectsUnknownStrictness(t *testing.T) {
	_, err := normalizePublicTryoutJudge(&AgentTryoutJudgeSelection{Strictness: "brutal"}, defaultPublicJudgeModels())
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("error = %v, want ErrInvalidAgentTryoutInput", err)
	}
}

func TestEvaluationSpecWithPublicJudgeDerivesJudgesFromEvalSetup(t *testing.T) {
	templateSpec := json.RawMessage(`{"validators":[{"key":"has_summary","type":"json_field","field":"summary"}],"scorecard":{"dimensions":["correctness"]}}`)
	input := json.RawMessage(`{"notes":"hello","eval_setup":{"unacceptable_mistakes":"Invented numbers","human_reviewer":"CFO","business_priority":"accuracy","output_style":"consistent"}}`)

	merged := evaluationSpecWithPublicJudge(templateSpec, AgentTryoutJudgeSelection{
		Model:      "gpt-5-mini",
		Strictness: "standard",
	}, input, "Meeting Minutes to Action Plan")

	var decoded struct {
		Validators []map[string]any `json:"validators"`
		JudgeMode  string           `json:"judge_mode"`
		LLMJudges  []struct {
			Key       string   `json:"key"`
			Mode      string   `json:"mode"`
			Model     string   `json:"model"`
			Rubric    string   `json:"rubric"`
			Assertion string   `json:"assertion"`
			Samples   int      `json:"samples"`
			Context   []string `json:"context_from"`
		} `json:"llm_judges"`
		JudgeMeta struct {
			Model      string            `json:"model"`
			Strictness string            `json:"strictness"`
			Labels     map[string]string `json:"labels"`
		} `json:"judge_meta"`
	}
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatalf("unmarshal merged spec: %v", err)
	}

	if len(decoded.Validators) != 1 {
		t.Fatalf("validators = %d, want untouched template validators", len(decoded.Validators))
	}
	if decoded.JudgeMode != "hybrid" {
		t.Fatalf("judge_mode = %q, want hybrid", decoded.JudgeMode)
	}
	if len(decoded.LLMJudges) != 3 {
		t.Fatalf("llm_judges = %d, want overall + instant_fail + reviewer_bar", len(decoded.LLMJudges))
	}
	for _, judge := range decoded.LLMJudges {
		if judge.Model != "gpt-5-mini" {
			t.Fatalf("judge %q model = %q, want gpt-5-mini", judge.Key, judge.Model)
		}
		if judge.Samples != 1 {
			t.Fatalf("judge %q samples = %d, want 1", judge.Key, judge.Samples)
		}
		if len(judge.Context) != 1 || judge.Context[0] != "final_output" {
			t.Fatalf("judge %q context_from = %v, want [final_output]", judge.Key, judge.Context)
		}
	}
	overall := decoded.LLMJudges[0]
	if overall.Key != publicJudgeKeyOverall || overall.Mode != "rubric" {
		t.Fatalf("first judge = %q/%q, want overall_quality rubric", overall.Key, overall.Mode)
	}
	if !strings.Contains(overall.Rubric, "Invented numbers") || !strings.Contains(overall.Rubric, "CFO") {
		t.Fatalf("overall rubric does not quote the operator's answers: %q", overall.Rubric)
	}
	if decoded.JudgeMeta.Model != "gpt-5-mini" || decoded.JudgeMeta.Strictness != "standard" {
		t.Fatalf("judge_meta = %+v, want model/strictness echoed", decoded.JudgeMeta)
	}
	if decoded.JudgeMeta.Labels[publicJudgeKeyReviewer] != "CFO would sign off" {
		t.Fatalf("reviewer label = %q", decoded.JudgeMeta.Labels[publicJudgeKeyReviewer])
	}
}

func TestEvaluationSpecWithPublicJudgeWithoutEvalSetupKeepsOverallJudgeOnly(t *testing.T) {
	merged := evaluationSpecWithPublicJudge(json.RawMessage(`{}`), AgentTryoutJudgeSelection{
		Model:      "gemini-2.5-flash",
		Strictness: "lenient",
	}, json.RawMessage(`{"notes":"hi"}`), "Status Report")

	var decoded struct {
		LLMJudges []struct {
			Key string `json:"key"`
		} `json:"llm_judges"`
	}
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatalf("unmarshal merged spec: %v", err)
	}
	if len(decoded.LLMJudges) != 1 || decoded.LLMJudges[0].Key != publicJudgeKeyOverall {
		t.Fatalf("llm_judges = %+v, want only the overall rubric judge", decoded.LLMJudges)
	}
}
