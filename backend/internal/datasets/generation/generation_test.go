package generation_test

import (
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/datasets/generation"
	"github.com/google/uuid"
)

func TestBuildSelfInstructPromptIncludesSeeds(t *testing.T) {
	prompt := generation.BuildSelfInstructPrompt([]generation.SeedExample{
		{Input: json.RawMessage(`{"question":"hello"}`), Expected: json.RawMessage(`{"answer":"hi"}`)},
	}, 3)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsAll(prompt, "Seed examples:", `"question":"hello"`, `"answer":"hi"`) {
		t.Fatalf("prompt missing seed content: %q", prompt)
	}
}

func TestParseSelfInstructResponse(t *testing.T) {
	candidate, err := generation.ParseSelfInstructResponse(`{"input":{"q":"x"},"expected":{"a":"y"}}`)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if string(candidate.Input) != `{"q":"x"}` {
		t.Fatalf("unexpected input: %s", candidate.Input)
	}
}

func TestParseSelfInstructResponseRejectsInvalidJSON(t *testing.T) {
	_, err := generation.ParseSelfInstructResponse("not json")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestCanonicalInputHashDedupesKeyOrder(t *testing.T) {
	a, err := generation.CanonicalInputHash(json.RawMessage(`{"b":1,"a":2}`))
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	b, err := generation.CanonicalInputHash(json.RawMessage(`{"a":2,"b":1}`))
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if a != b {
		t.Fatalf("expected equal hashes, got %s and %s", a, b)
	}
}

func TestComputeCostUSD(t *testing.T) {
	cost := generation.ComputeCostUSD(1_000_000, 500_000, generation.ModelPricing{
		InputCostPerMillionTokens:  3,
		OutputCostPerMillionTokens: 6,
	})
	if cost != 6 {
		t.Fatalf("expected cost 6, got %v", cost)
	}
}

func TestParseStrategyAcceptsAgenticSelfInstructAliases(t *testing.T) {
	for _, raw := range []string{"agentic_self_instruct", "agentic-self-instruct"} {
		strategy, err := generation.ParseStrategy(raw)
		if err != nil {
			t.Fatalf("parse strategy %q: %v", raw, err)
		}
		if strategy != generation.StrategyAgenticSelfInstruct {
			t.Fatalf("strategy = %q, want %q", strategy, generation.StrategyAgenticSelfInstruct)
		}
	}
}

func TestDecodeJobConfigForStrategyValidatesAgenticJudgeFields(t *testing.T) {
	providerID := uuid.New()
	raw, _ := json.Marshal(map[string]any{
		"provider_account_id": providerID,
		"model":               "gpt-4.1-mini",
	})
	if _, err := generation.DecodeJobConfigForStrategy(raw, generation.StrategyAgenticSelfInstruct); err == nil {
		t.Fatal("expected missing judge provider error")
	}

	judgeID := uuid.New()
	raw, _ = json.Marshal(map[string]any{
		"provider_account_id":       providerID,
		"model":                     "gpt-4.1-mini",
		"judge_provider_account_id": judgeID,
		"judge_model":               "gpt-4.1-mini",
		"max_rounds_per_example":    3,
		"acceptance_mode":           "judge",
	})
	cfg, err := generation.DecodeJobConfigForStrategy(raw, generation.StrategyAgenticSelfInstruct)
	if err != nil {
		t.Fatalf("decode agentic config: %v", err)
	}
	if cfg.JudgeProviderAccountID == nil || *cfg.JudgeProviderAccountID != judgeID {
		t.Fatalf("unexpected judge provider: %+v", cfg.JudgeProviderAccountID)
	}
}

func TestDecodeJobConfigForStrategyRequiresThresholdValues(t *testing.T) {
	providerID := uuid.New()
	judgeID := uuid.New()
	raw, _ := json.Marshal(map[string]any{
		"provider_account_id":       providerID,
		"model":                     "gpt-4.1-mini",
		"judge_provider_account_id": judgeID,
		"judge_model":               "gpt-4.1-mini",
		"acceptance_mode":           "threshold",
	})
	if _, err := generation.DecodeJobConfigForStrategy(raw, generation.StrategyAgenticSelfInstruct); err == nil {
		t.Fatal("expected missing threshold values error")
	}

	raw, _ = json.Marshal(map[string]any{
		"provider_account_id":       providerID,
		"model":                     "gpt-4.1-mini",
		"judge_provider_account_id": judgeID,
		"judge_model":               "gpt-4.1-mini",
		"acceptance_mode":           "threshold",
		"min_gap":                   0.2,
		"max_weak_score":            0.65,
		"min_strong_score":          0.75,
	})
	if _, err := generation.DecodeJobConfigForStrategy(raw, generation.StrategyAgenticSelfInstruct); err != nil {
		t.Fatalf("decode threshold config: %v", err)
	}
}

func TestDecodeJobConfigForStrategyValidatesDirectProviderSolvers(t *testing.T) {
	providerID := uuid.New()
	judgeID := uuid.New()
	weakID := uuid.New()
	strongID := uuid.New()
	raw, _ := json.Marshal(map[string]any{
		"provider_account_id":        providerID,
		"model":                      "gpt-4.1-mini",
		"judge_provider_account_id":  judgeID,
		"judge_model":                "gpt-4.1",
		"solver_mode":                "direct_provider",
		"weak_provider_account_id":   weakID,
		"weak_model":                 "gpt-4.1-nano",
		"strong_provider_account_id": strongID,
		"strong_model":               "gpt-4.1",
	})
	cfg, err := generation.DecodeJobConfigForStrategy(raw, generation.StrategyAgenticSelfInstruct)
	if err != nil {
		t.Fatalf("decode direct provider config: %v", err)
	}
	if cfg.SolverMode != generation.SolverModeDirectProvider {
		t.Fatalf("solver mode = %q", cfg.SolverMode)
	}
	if cfg.WeakRollouts != 1 || cfg.StrongRollouts != 1 {
		t.Fatalf("default rollouts = %d/%d, want 1/1", cfg.WeakRollouts, cfg.StrongRollouts)
	}

	raw, _ = json.Marshal(map[string]any{
		"provider_account_id":       providerID,
		"model":                     "gpt-4.1-mini",
		"judge_provider_account_id": judgeID,
		"judge_model":               "gpt-4.1",
		"solver_mode":               "direct_provider",
		"weak_provider_account_id":  weakID,
		"weak_model":                "gpt-4.1-nano",
		"strong_model":              "gpt-4.1",
	})
	if _, err := generation.DecodeJobConfigForStrategy(raw, generation.StrategyAgenticSelfInstruct); err == nil {
		t.Fatal("expected missing strong provider error")
	}
}

func TestDecodeJobConfigForStrategyValidatesDeploymentContext(t *testing.T) {
	providerID := uuid.New()
	judgeID := uuid.New()
	weakDeploymentID := uuid.New()
	strongDeploymentID := uuid.New()
	challengePackVersionID := uuid.New()
	raw, _ := json.Marshal(map[string]any{
		"provider_account_id":       providerID,
		"model":                     "gpt-4.1-mini",
		"judge_provider_account_id": judgeID,
		"judge_model":               "gpt-4.1",
		"weak_deployment_id":        weakDeploymentID,
		"strong_deployment_id":      strongDeploymentID,
		"challenge_pack_version_id": challengePackVersionID,
		"challenge_key":             "support-recovery",
		"field_mapping":             map[string]string{"input": "prompt"},
	})
	cfg, err := generation.DecodeJobConfigForStrategy(raw, generation.StrategyAgenticSelfInstruct)
	if err != nil {
		t.Fatalf("decode deployment context: %v", err)
	}
	if cfg.WeakDeploymentID == nil || *cfg.WeakDeploymentID != weakDeploymentID {
		t.Fatalf("weak deployment = %+v", cfg.WeakDeploymentID)
	}

	raw, _ = json.Marshal(map[string]any{
		"provider_account_id":       providerID,
		"model":                     "gpt-4.1-mini",
		"judge_provider_account_id": judgeID,
		"judge_model":               "gpt-4.1",
		"weak_deployment_id":        weakDeploymentID,
	})
	if _, err := generation.DecodeJobConfigForStrategy(raw, generation.StrategyAgenticSelfInstruct); err == nil {
		t.Fatal("expected incomplete deployment context error")
	}
}

func TestBuildAgenticSolverPromptOmitsExpectedAnswer(t *testing.T) {
	prompt := generation.BuildAgenticSolverPrompt("weak_solver", generation.Candidate{
		Input:    json.RawMessage(`{"prompt":"recover the account"}`),
		Expected: json.RawMessage(`{"answer":"secret expected answer"}`),
	})
	if !containsAll(prompt, "weak_solver", "recover the account") {
		t.Fatalf("solver prompt missing role/input: %q", prompt)
	}
	if contains(prompt, "secret expected answer") || contains(prompt, "expected") {
		t.Fatalf("solver prompt leaked expected answer: %q", prompt)
	}
}

func TestBuildAgenticJudgePromptIncludesSolverAttempts(t *testing.T) {
	prompt := generation.BuildAgenticJudgePrompt(generation.AgenticJudgeInput{
		Seeds:     []generation.SeedExample{{Input: json.RawMessage(`{"q":"seed"}`)}},
		Candidate: generation.Candidate{Input: json.RawMessage(`{"q":"candidate"}`)},
		WeakAttempts: []generation.AgenticSolverAttempt{{
			Role:    "weak_solver",
			Attempt: 1,
			Model:   "gpt-4.1-nano",
			Output:  "weak answer",
		}},
		StrongAttempts: []generation.AgenticSolverAttempt{{
			Role:    "strong_solver",
			Attempt: 1,
			Model:   "gpt-4.1",
			Output:  "strong answer",
		}},
	})
	if !containsAll(prompt, "Weak solver attempts", "weak answer", "Strong solver attempts", "strong answer") {
		t.Fatalf("judge prompt missing solver attempts: %q", prompt)
	}
}

func TestParseAgenticJudgeResponse(t *testing.T) {
	verdict, err := generation.ParseAgenticJudgeResponse(`{
		"verdict":"accept",
		"quality_verdict":"high",
		"weak_score":0.4,
		"strong_score":0.8,
		"gap":0.4,
		"capability_tags":["schema-following"]
	}`)
	if err != nil {
		t.Fatalf("parse judge response: %v", err)
	}
	if verdict.Verdict != generation.JudgeVerdictAccept {
		t.Fatalf("verdict = %q", verdict.Verdict)
	}
	if verdict.Gap == nil || *verdict.Gap != 0.4 {
		t.Fatalf("gap = %+v", verdict.Gap)
	}
}

func TestParseAgenticJudgeResponseRejectsInvalidResponses(t *testing.T) {
	tests := []string{
		`not json`,
		`{"quality_verdict":"high"}`,
		`{"verdict":"maybe"}`,
		`{"verdict":"accept","weak_score":1.2}`,
	}
	for _, raw := range tests {
		if _, err := generation.ParseAgenticJudgeResponse(raw); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
}

func TestShouldAcceptJudgeVerdict(t *testing.T) {
	weak := 0.4
	strong := 0.8
	gap := 0.4
	verdict := generation.AgenticJudgeVerdict{
		Verdict:     generation.JudgeVerdictAccept,
		WeakScore:   &weak,
		StrongScore: &strong,
		Gap:         &gap,
	}
	minGap := 0.2
	maxWeak := 0.65
	minStrong := 0.75
	if !generation.ShouldAcceptJudgeVerdict(verdict, generation.AgenticAcceptanceConfig{
		Mode:           generation.AcceptanceModeThreshold,
		MinGap:         &minGap,
		MaxWeakScore:   &maxWeak,
		MinStrongScore: &minStrong,
	}) {
		t.Fatal("expected threshold verdict to be accepted")
	}
	tooHighWeak := 0.9
	verdict.WeakScore = &tooHighWeak
	if generation.ShouldAcceptJudgeVerdict(verdict, generation.AgenticAcceptanceConfig{
		Mode:         generation.AcceptanceModeJudge,
		MaxWeakScore: &maxWeak,
	}) {
		t.Fatal("expected max weak guardrail to reject")
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !contains(text, part) {
			return false
		}
	}
	return true
}

func contains(text, part string) bool {
	return len(text) >= len(part) && (text == part || len(part) == 0 || indexOf(text, part) >= 0)
}

func indexOf(text, part string) int {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
