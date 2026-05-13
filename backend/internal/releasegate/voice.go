package releasegate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/voicescorecard"
)

const (
	VoiceDimensionOverallScore        = "voice_overall_score"
	VoiceDimensionTaskBusinessSuccess = "task_business_success"
	VoiceDimensionInteractionQuality  = "interaction_quality"
	VoiceDimensionToolDataCorrectness = "tool_data_correctness"
	VoiceDimensionLatencyMS           = "latency_ms"
	VoiceDimensionMissingEvidence     = "voice_missing_evidence"
)

type VoiceComparisonConfig struct {
	MissingEvidenceOutcome Verdict
}

func DefaultVoiceComparisonConfig() VoiceComparisonConfig {
	return VoiceComparisonConfig{MissingEvidenceOutcome: VerdictFail}
}

func VoicePolicy(config VoiceComparisonConfig) Policy {
	missingThreshold := DimensionThreshold{FailDelta: floatPtr(0.01)}
	if config.MissingEvidenceOutcome == VerdictWarn {
		missingThreshold = DimensionThreshold{WarnDelta: floatPtr(0.01)}
	}
	return Policy{
		PolicyKey:              "voice-default",
		PolicyVersion:          1,
		RequireComparable:      true,
		RequireEvidenceQuality: false,
		RequireScorecardPass:   true,
		RequiredDimensions: []string{
			VoiceDimensionInteractionQuality,
			VoiceDimensionMissingEvidence,
			VoiceDimensionOverallScore,
			VoiceDimensionTaskBusinessSuccess,
			VoiceDimensionToolDataCorrectness,
		},
		Dimensions: map[string]DimensionThreshold{
			VoiceDimensionOverallScore:        {WarnDelta: floatPtr(0.02), FailDelta: floatPtr(0.05)},
			VoiceDimensionTaskBusinessSuccess: {FailDelta: floatPtr(0.01)},
			VoiceDimensionInteractionQuality:  {FailDelta: floatPtr(0.01)},
			VoiceDimensionToolDataCorrectness: {FailDelta: floatPtr(0.01)},
			VoiceDimensionLatencyMS:           {WarnDelta: floatPtr(100), FailDelta: floatPtr(250)},
			VoiceDimensionMissingEvidence:     missingThreshold,
		},
	}
}

func BuildVoiceComparisonSummary(baseline, candidate voicescorecard.Scorecard, config VoiceComparisonConfig) ComparisonSummary {
	warnings := voiceWarnings(baseline, candidate)
	return ComparisonSummary{
		SchemaVersion: "2026-05-13",
		Status:        "comparable",
		DimensionDeltas: map[string]DimensionDelta{
			VoiceDimensionOverallScore:        voiceDelta(baseline.OverallScore, candidate.OverallScore, "higher"),
			VoiceDimensionTaskBusinessSuccess: voiceDimensionDelta(baseline, candidate, VoiceDimensionTaskBusinessSuccess, "higher"),
			VoiceDimensionInteractionQuality:  voiceDimensionDelta(baseline, candidate, VoiceDimensionInteractionQuality, "higher"),
			VoiceDimensionToolDataCorrectness: voiceDimensionDelta(baseline, candidate, VoiceDimensionToolDataCorrectness, "higher"),
			VoiceDimensionLatencyMS:           voiceMetricDelta(baseline, candidate, "latency", "end_of_user_turn_to_first_agent_output_ms", "lower"),
			VoiceDimensionMissingEvidence:     voiceDelta(0, float64(newCandidateDegradedKeyCount(baseline.DegradedKeys, candidate.DegradedKeys)), "lower"),
		},
		ScorecardPass: &ScorecardPassSummary{
			Baseline:  boolPtr(!baseline.HardGateFailed),
			Candidate: boolPtr(!candidate.HardGateFailed),
		},
		FailureDivergence: FailureDivergence{
			CandidateFailedBaselineOK: candidate.HardGateFailed && !baseline.HardGateFailed,
			BothFailedDifferently:     candidate.HardGateFailed && baseline.HardGateFailed && voiceFailedHardGateKeys(candidate) != voiceFailedHardGateKeys(baseline),
		},
		ReplaySummaryDivergence: ReplayDivergence{State: "available"},
		EvidenceQuality: compareEvidenceQuality{
			Warnings: warnings,
		},
	}
}

func newCandidateDegradedKeyCount(baselineKeys []string, candidateKeys []string) int {
	baselineSet := make(map[string]struct{}, len(baselineKeys))
	for _, key := range baselineKeys {
		baselineSet[strings.TrimSpace(key)] = struct{}{}
	}
	count := 0
	for _, key := range candidateKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := baselineSet[key]; ok {
			continue
		}
		count++
	}
	return count
}

func voiceDimensionDelta(baseline, candidate voicescorecard.Scorecard, key string, direction string) DimensionDelta {
	baselineValue, baselineOK := voiceDimensionScore(baseline, key)
	candidateValue, candidateOK := voiceDimensionScore(candidate, key)
	if !baselineOK || !candidateOK {
		return DimensionDelta{BetterDirection: direction, State: "unavailable"}
	}
	return voiceDelta(baselineValue, candidateValue, direction)
}

func voiceMetricDelta(baseline, candidate voicescorecard.Scorecard, dimensionKey string, metricKey string, direction string) DimensionDelta {
	baselineValue, baselineOK := voiceMetricValueMS(baseline, dimensionKey, metricKey)
	candidateValue, candidateOK := voiceMetricValueMS(candidate, dimensionKey, metricKey)
	if !baselineOK || !candidateOK {
		return DimensionDelta{BetterDirection: direction, State: "unavailable"}
	}
	return voiceDelta(float64(baselineValue), float64(candidateValue), direction)
}

func voiceDelta(baseline float64, candidate float64, direction string) DimensionDelta {
	delta := candidate - baseline
	return DimensionDelta{
		BaselineValue:   floatPtr(baseline),
		CandidateValue:  floatPtr(candidate),
		Delta:           floatPtr(delta),
		BetterDirection: direction,
		State:           "available",
	}
}

func voiceDimensionScore(scorecard voicescorecard.Scorecard, key string) (float64, bool) {
	for _, dimension := range scorecard.Dimensions {
		if dimension.Key == key && dimension.State != "" {
			return dimension.Score, true
		}
	}
	return 0, false
}

func voiceMetricValueMS(scorecard voicescorecard.Scorecard, dimensionKey string, metricKey string) (int64, bool) {
	for _, dimension := range scorecard.Dimensions {
		if dimension.Key != dimensionKey {
			continue
		}
		for _, metric := range dimension.Metrics {
			if metric.Key == metricKey && metric.ValueMS != nil {
				return *metric.ValueMS, true
			}
		}
	}
	return 0, false
}

func voiceWarnings(baseline, candidate voicescorecard.Scorecard) []string {
	warnings := make([]string, 0, 2)
	if len(baseline.DegradedKeys) > 0 {
		warnings = append(warnings, fmt.Sprintf("baseline voice evidence degraded: %s", strings.Join(sortedStrings(baseline.DegradedKeys), ", ")))
	}
	if len(candidate.DegradedKeys) > 0 {
		warnings = append(warnings, fmt.Sprintf("candidate voice evidence degraded: %s", strings.Join(sortedStrings(candidate.DegradedKeys), ", ")))
	}
	return warnings
}

func voiceFailedHardGateKeys(scorecard voicescorecard.Scorecard) string {
	keys := make([]string, 0)
	for _, dimension := range scorecard.Dimensions {
		if dimension.HardGate && dimension.State == "failed" {
			keys = append(keys, dimension.Key)
		}
	}
	return strings.Join(sortedStrings(keys), ",")
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func boolPtr(value bool) *bool {
	return &value
}
