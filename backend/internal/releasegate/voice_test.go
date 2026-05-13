package releasegate

import (
	"testing"

	"github.com/agentclash/agentclash/backend/internal/voiceeval"
	"github.com/agentclash/agentclash/backend/internal/voicescorecard"
)

func TestEvaluateVoiceComparisonNoRegressionPasses(t *testing.T) {
	evaluation := evaluateVoice(t, voiceScorecard(), voiceScorecard(), DefaultVoiceComparisonConfig())

	if evaluation.Verdict != VerdictPass {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictPass)
	}
	if evaluation.ReasonCode != "within_thresholds" {
		t.Fatalf("reason code = %q, want within_thresholds", evaluation.ReasonCode)
	}
}

func TestEvaluateVoiceComparisonLatencyRegressionFails(t *testing.T) {
	candidate := voiceScorecard()
	setVoiceLatencyMS(&candidate, 1600)

	evaluation := evaluateVoice(t, voiceScorecard(), candidate, DefaultVoiceComparisonConfig())

	if evaluation.Verdict != VerdictFail {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictFail)
	}
	if evaluation.ReasonCode != "threshold_fail_latency_ms" {
		t.Fatalf("reason code = %q, want threshold_fail_latency_ms", evaluation.ReasonCode)
	}
}

func TestEvaluateVoiceComparisonTaskSuccessRegressionFails(t *testing.T) {
	candidate := voiceScorecard()
	setVoiceDimensionScore(&candidate, VoiceDimensionTaskBusinessSuccess, 0.98)

	evaluation := evaluateVoice(t, voiceScorecard(), candidate, DefaultVoiceComparisonConfig())

	if evaluation.Verdict != VerdictFail {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictFail)
	}
	if evaluation.ReasonCode != "threshold_fail_task_business_success" {
		t.Fatalf("reason code = %q, want threshold_fail_task_business_success", evaluation.ReasonCode)
	}
}

func TestEvaluateVoiceComparisonPolicyHardGateFails(t *testing.T) {
	candidate := voiceScorecard()
	candidate.Passed = false
	candidate.HardGateFailed = true
	setVoiceDimensionState(&candidate, VoiceDimensionInteractionQuality, voiceeval.StateFailed)

	evaluation := evaluateVoice(t, voiceScorecard(), candidate, DefaultVoiceComparisonConfig())

	if evaluation.Verdict != VerdictFail {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictFail)
	}
	if evaluation.ReasonCode != "scorecard_not_passed" {
		t.Fatalf("reason code = %q, want scorecard_not_passed", evaluation.ReasonCode)
	}
}

func TestEvaluateVoiceComparisonMissingMetricCanFailOrWarnByPolicy(t *testing.T) {
	candidate := voiceScorecard()
	candidate.Passed = false
	candidate.DegradedKeys = []string{"end_of_user_turn_to_first_agent_output_ms"}
	removeVoiceLatencyMetric(&candidate)

	failEvaluation := evaluateVoice(t, voiceScorecard(), candidate, VoiceComparisonConfig{MissingEvidenceOutcome: VerdictFail})
	if failEvaluation.Verdict != VerdictFail {
		t.Fatalf("fail policy verdict = %q, want %q", failEvaluation.Verdict, VerdictFail)
	}
	if failEvaluation.ReasonCode != "threshold_fail_voice_missing_evidence" {
		t.Fatalf("fail policy reason = %q, want threshold_fail_voice_missing_evidence", failEvaluation.ReasonCode)
	}

	warnEvaluation := evaluateVoice(t, voiceScorecard(), candidate, VoiceComparisonConfig{MissingEvidenceOutcome: VerdictWarn})
	if warnEvaluation.Verdict != VerdictWarn {
		t.Fatalf("warn policy verdict = %q, want %q", warnEvaluation.Verdict, VerdictWarn)
	}
	if warnEvaluation.ReasonCode != "threshold_warn_voice_missing_evidence" {
		t.Fatalf("warn policy reason = %q, want threshold_warn_voice_missing_evidence", warnEvaluation.ReasonCode)
	}
	if len(warnEvaluation.Details.Warnings) == 0 {
		t.Fatalf("warnings = %v, want degraded evidence warning", warnEvaluation.Details.Warnings)
	}
}

func TestExistingNonVoiceCompareFixtureStillPasses(t *testing.T) {
	evaluation, err := Evaluate(testSummary(t, `{
		"status":"comparable",
		"dimension_deltas":{
			"correctness":{"delta":-0.01,"better_direction":"higher","state":"available"},
			"reliability":{"delta":-0.01,"better_direction":"higher","state":"available"},
			"latency":{"delta":0.02,"better_direction":"lower","state":"available"},
			"cost":{"delta":0.03,"better_direction":"lower","state":"available"}
		},
		"failure_divergence":{"candidate_failed_baseline_succeeded":false,"both_failed_differently":false},
		"replay_summary_divergence":{"state":"available"},
		"evidence_quality":{}
	}`), DefaultPolicy())
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if evaluation.Verdict != VerdictPass {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictPass)
	}
}

func evaluateVoice(t *testing.T, baseline voicescorecard.Scorecard, candidate voicescorecard.Scorecard, config VoiceComparisonConfig) Evaluation {
	t.Helper()
	evaluation, err := Evaluate(BuildVoiceComparisonSummary(baseline, candidate, config), VoicePolicy(config))
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	return evaluation
}

func voiceScorecard() voicescorecard.Scorecard {
	return voicescorecard.Scorecard{
		SchemaVersion:  "2026-05-13",
		Type:           "voice",
		OverallScore:   1,
		Passed:         true,
		HardGateFailed: false,
		Dimensions: []voicescorecard.Dimension{
			voiceDimension(VoiceDimensionTaskBusinessSuccess, 1, voiceeval.StatePassed, true),
			voiceDimension(VoiceDimensionInteractionQuality, 1, voiceeval.StatePassed, true),
			{
				Key:      "latency",
				Name:     "Latency",
				Score:    1,
				State:    voiceeval.StatePassed,
				HardGate: false,
				Metrics: []voicescorecard.MetricDetail{
					{
						Key:     "end_of_user_turn_to_first_agent_output_ms",
						State:   voiceeval.StatePassed,
						ValueMS: int64PtrForVoiceTest(1200),
					},
				},
			},
			voiceDimension("robustness", 1, voiceeval.StatePassed, false),
			voiceDimension(VoiceDimensionToolDataCorrectness, 1, voiceeval.StatePassed, true),
			voiceDimension("cost", 1, voiceeval.StatePassed, false),
		},
	}
}

func voiceDimension(key string, score float64, state voiceeval.State, hardGate bool) voicescorecard.Dimension {
	return voicescorecard.Dimension{
		Key:      key,
		Name:     key,
		Score:    score,
		State:    state,
		HardGate: hardGate,
	}
}

func setVoiceLatencyMS(scorecard *voicescorecard.Scorecard, value int64) {
	for dimIdx := range scorecard.Dimensions {
		if scorecard.Dimensions[dimIdx].Key != "latency" {
			continue
		}
		for metricIdx := range scorecard.Dimensions[dimIdx].Metrics {
			if scorecard.Dimensions[dimIdx].Metrics[metricIdx].Key == "end_of_user_turn_to_first_agent_output_ms" {
				scorecard.Dimensions[dimIdx].Metrics[metricIdx].ValueMS = int64PtrForVoiceTest(value)
				return
			}
		}
	}
}

func removeVoiceLatencyMetric(scorecard *voicescorecard.Scorecard) {
	for dimIdx := range scorecard.Dimensions {
		if scorecard.Dimensions[dimIdx].Key == "latency" {
			scorecard.Dimensions[dimIdx].Metrics = nil
			scorecard.Dimensions[dimIdx].State = voiceeval.StateUnavailable
			scorecard.Dimensions[dimIdx].Score = 0.5
		}
	}
}

func setVoiceDimensionScore(scorecard *voicescorecard.Scorecard, key string, score float64) {
	for idx := range scorecard.Dimensions {
		if scorecard.Dimensions[idx].Key == key {
			scorecard.Dimensions[idx].Score = score
			return
		}
	}
}

func setVoiceDimensionState(scorecard *voicescorecard.Scorecard, key string, state voiceeval.State) {
	for idx := range scorecard.Dimensions {
		if scorecard.Dimensions[idx].Key == key {
			scorecard.Dimensions[idx].State = state
			scorecard.Dimensions[idx].Score = 0
			return
		}
	}
}

func int64PtrForVoiceTest(value int64) *int64 {
	return &value
}
