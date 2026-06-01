package gate

import (
	"math"

	"github.com/google/uuid"
)

type ExampleOutcome struct {
	DatasetExampleID uuid.UUID  `json:"dataset_example_id"`
	Verdict          *string    `json:"verdict,omitempty"`
	NormalizedScore  *float64   `json:"normalized_score,omitempty"`
	RunID            *uuid.UUID `json:"run_id,omitempty"`
	RunAgentID       *uuid.UUID `json:"run_agent_id,omitempty"`
}

type Thresholds struct {
	MinPassRate    *float64
	MaxRegressions *int
}

type Regression struct {
	DatasetExampleID uuid.UUID `json:"dataset_example_id"`
	Reason           string    `json:"reason"`
	BaselineVerdict  *string   `json:"baseline_verdict,omitempty"`
	CandidateVerdict *string   `json:"candidate_verdict,omitempty"`
	BaselineScore    *float64  `json:"baseline_score,omitempty"`
	CandidateScore   *float64  `json:"candidate_score,omitempty"`
}

type Result struct {
	Pass               bool         `json:"pass"`
	PassRate           float64      `json:"pass_rate"`
	BaselinePassRate   float64      `json:"baseline_pass_rate"`
	RegressionCount    int          `json:"regression_count"`
	Regressions        []Regression `json:"regressions"`
	EvaluatedExamples  int          `json:"evaluated_examples"`
	FailedThresholds   []string     `json:"failed_thresholds,omitempty"`
}

func IsPassing(verdict *string) bool {
	return verdict != nil && *verdict == "pass"
}

func PassRate(outcomes []ExampleOutcome) float64 {
	if len(outcomes) == 0 {
		return 0
	}
	passed := 0
	for _, item := range outcomes {
		if IsPassing(item.Verdict) {
			passed++
		}
	}
	return float64(passed) / float64(len(outcomes))
}

func Evaluate(baseline, candidate []ExampleOutcome, thresholds Thresholds) Result {
	baselineByExample := make(map[uuid.UUID]ExampleOutcome, len(baseline))
	for _, item := range baseline {
		baselineByExample[item.DatasetExampleID] = item
	}

	regressions := make([]Regression, 0)
	for _, item := range candidate {
		base, ok := baselineByExample[item.DatasetExampleID]
		if !ok {
			if !IsPassing(item.Verdict) {
				regressions = append(regressions, Regression{
					DatasetExampleID: item.DatasetExampleID,
					Reason:           "new_failure",
					CandidateVerdict: item.Verdict,
					CandidateScore:   item.NormalizedScore,
				})
			}
			continue
		}
		if IsPassing(base.Verdict) && !IsPassing(item.Verdict) {
			regressions = append(regressions, Regression{
				DatasetExampleID: item.DatasetExampleID,
				Reason:           "newly_failing",
				BaselineVerdict:  base.Verdict,
				CandidateVerdict: item.Verdict,
				BaselineScore:    base.NormalizedScore,
				CandidateScore:   item.NormalizedScore,
			})
			continue
		}
		if !IsPassing(base.Verdict) {
			continue
		}
		if base.NormalizedScore != nil && item.NormalizedScore != nil && *item.NormalizedScore+1e-9 < *base.NormalizedScore {
			regressions = append(regressions, Regression{
				DatasetExampleID: item.DatasetExampleID,
				Reason:           "score_regression",
				BaselineVerdict:  base.Verdict,
				CandidateVerdict: item.Verdict,
				BaselineScore:    base.NormalizedScore,
				CandidateScore:   item.NormalizedScore,
			})
		}
	}

	candidateByExample := make(map[uuid.UUID]struct{}, len(candidate))
	for _, item := range candidate {
		candidateByExample[item.DatasetExampleID] = struct{}{}
	}
	for _, base := range baseline {
		if _, ok := candidateByExample[base.DatasetExampleID]; ok {
			continue
		}
		regressions = append(regressions, Regression{
			DatasetExampleID: base.DatasetExampleID,
			Reason:           "missing_example",
			BaselineVerdict:  base.Verdict,
			BaselineScore:    base.NormalizedScore,
		})
	}

	if len(candidate) == 0 {
		return Result{
			Pass:              false,
			BaselinePassRate:  PassRate(baseline),
			FailedThresholds:  []string{"no_candidate_outcomes"},
		}
	}

	result := Result{
		Pass:              true,
		PassRate:          PassRate(candidate),
		BaselinePassRate:  PassRate(baseline),
		RegressionCount:   len(regressions),
		Regressions:       regressions,
		EvaluatedExamples: len(candidate),
	}

	if thresholds.MinPassRate != nil && result.PassRate+1e-9 < *thresholds.MinPassRate {
		result.Pass = false
		result.FailedThresholds = append(result.FailedThresholds, "min_pass_rate")
	}
	if thresholds.MaxRegressions != nil && result.RegressionCount > *thresholds.MaxRegressions {
		result.Pass = false
		result.FailedThresholds = append(result.FailedThresholds, "max_regressions")
	}
	if len(regressions) > 0 && thresholds.MaxRegressions == nil && thresholds.MinPassRate == nil {
		result.Pass = false
		result.FailedThresholds = append(result.FailedThresholds, "regressions_detected")
	}

	return result
}

func RoundPassRate(value float64) float64 {
	return math.Round(value*10000) / 10000
}
