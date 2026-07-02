package scoring

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestRunTotalTokensUnchangedWithoutRaceContext guards the backwards-compat
// promise: runs without race_context inject no tokens, so `run_total_tokens`
// keeps its pre-#400 value. Break this and every existing dashboard or
// validator keyed on `run_total_tokens` goes wrong.
func TestRunTotalTokensUnchangedWithoutRaceContext(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "tokens-no-race",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "present", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "literal:ok"},
		},
		Metrics: []MetricDeclaration{
			{Key: "total", Type: MetricTypeNumeric, Collector: "run_total_tokens"},
			{Key: "agent", Type: MetricTypeNumeric, Collector: "run_agent_tokens"},
			{Key: "race", Type: MetricTypeNumeric, Collector: "run_race_context_tokens"},
		},
		Scorecard: ScorecardDeclaration{Dimensions: []DimensionDeclaration{{Key: "correctness"}}},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "system.output.finalized", OccurredAt: time.Date(2026, 4, 25, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"output":"ok"}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 4, 25, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"total_tokens":1200}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent: %v", err)
	}

	total := evaluation.MetricResults[0].NumericValue
	agent := evaluation.MetricResults[1].NumericValue
	race := evaluation.MetricResults[2].NumericValue
	if total == nil || *total != 1200 {
		t.Errorf("run_total_tokens = %v, want 1200 (unchanged for race_context=false)", total)
	}
	if agent == nil || *agent != 1200 {
		t.Errorf("run_agent_tokens = %v, want 1200", agent)
	}
	if race == nil || *race != 0 {
		t.Errorf("run_race_context_tokens = %v, want 0", race)
	}
}

// TestRaceContextTokenSplitUsesProviderTotalOnce exercises a race-context
// run: provider total usage already includes the injected prompt bytes, so
// run_total_tokens must not add them a second time. The estimated injected
// tokens are exposed separately and subtracted from run_agent_tokens.
func TestRaceContextTokenSplitUsesProviderTotalOnce(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "tokens-with-race",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "present", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "literal:ok"},
		},
		Metrics: []MetricDeclaration{
			{Key: "total", Type: MetricTypeNumeric, Collector: "run_total_tokens"},
			{Key: "agent", Type: MetricTypeNumeric, Collector: "run_agent_tokens"},
			{Key: "race", Type: MetricTypeNumeric, Collector: "run_race_context_tokens"},
		},
		Scorecard: ScorecardDeclaration{Dimensions: []DimensionDeclaration{{Key: "correctness"}}},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "race.standings.injected", OccurredAt: time.Date(2026, 4, 25, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"tokens_added":120,"triggered_by":"cadence"}`)},
			{Type: "race.standings.injected", OccurredAt: time.Date(2026, 4, 25, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"tokens_added":135,"triggered_by":"peer_submitted"}`)},
			{Type: "system.output.finalized", OccurredAt: time.Date(2026, 4, 25, 9, 0, 3, 0, time.UTC), Payload: []byte(`{"output":"ok"}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 4, 25, 9, 0, 4, 0, time.UTC), Payload: []byte(`{"total_tokens":2000}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent: %v", err)
	}

	total := evaluation.MetricResults[0].NumericValue
	agent := evaluation.MetricResults[1].NumericValue
	race := evaluation.MetricResults[2].NumericValue
	if total == nil || *total != 2000 {
		t.Errorf("run_total_tokens = %v, want 2000 (provider total as-is)", total)
	}
	if agent == nil || *agent != 1745 {
		t.Errorf("run_agent_tokens = %v, want 1745 (2000 total - 255 race)", agent)
	}
	if race == nil || *race != 255 {
		t.Errorf("run_race_context_tokens = %v, want 255 (120 + 135)", race)
	}
}

// TestRunRaceContextTokensAvailableWhenNoModelUsage ensures the race
// collector doesn't error out just because the run has no model.call
// events (rare edge case for early failures).
func TestRunRaceContextTokensAvailableWhenNoModelUsage(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "tokens-race-only",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "present", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "literal:ok"},
		},
		Metrics: []MetricDeclaration{
			{Key: "race", Type: MetricTypeNumeric, Collector: "run_race_context_tokens"},
		},
		Scorecard: ScorecardDeclaration{Dimensions: []DimensionDeclaration{{Key: "correctness"}}},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "race.standings.injected", OccurredAt: time.Date(2026, 4, 25, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"tokens_added":42}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent: %v", err)
	}

	race := evaluation.MetricResults[0].NumericValue
	if race == nil || *race != 42 {
		t.Errorf("run_race_context_tokens = %v, want 42", race)
	}
}
