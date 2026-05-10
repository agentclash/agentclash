package repository

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildEvalSessionAggregatePayloadIsDeterministicAcrossRunOrder(t *testing.T) {
	first := evalSessionTestSource(
		"11111111-1111-1111-1111-111111111111",
		evalSessionTestParticipant(
			"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			0,
			"Primary",
			0.80,
			map[string]float64{"custom": 0.60},
			evalSessionTestTask("task-a", true),
		),
	)
	second := evalSessionTestSource(
		"22222222-2222-2222-2222-222222222222",
		evalSessionTestParticipant(
			"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			0,
			"Primary",
			0.90,
			map[string]float64{"custom": 0.70},
			evalSessionTestTask("task-a", false),
		),
	)

	leftAggregate, leftEvidence, _, err := buildEvalSessionAggregatePayload(
		2,
		[]evalSessionAggregateSource{first, second},
		nil,
		evalSessionTestBehavior(),
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}
	rightAggregate, rightEvidence, _, err := buildEvalSessionAggregatePayload(
		2,
		[]evalSessionAggregateSource{second, first},
		nil,
		evalSessionTestBehavior(),
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}

	if string(leftAggregate) != string(rightAggregate) {
		t.Fatalf("aggregate payload differs by run order:\nleft=%s\nright=%s", leftAggregate, rightAggregate)
	}
	if string(leftEvidence) != string(rightEvidence) {
		t.Fatalf("evidence payload differs by run order:\nleft=%s\nright=%s", leftEvidence, rightEvidence)
	}
}

func TestBuildEvalSessionAggregatePayloadCapturesMissingScorecardsAndPartialCoverage(t *testing.T) {
	aggregateJSON, evidenceJSON, scoredChildCount, err := buildEvalSessionAggregatePayload(
		2,
		[]evalSessionAggregateSource{
			evalSessionTestSource(
				"33333333-3333-3333-3333-333333333333",
				evalSessionTestParticipant(
					"cccccccc-cccc-cccc-cccc-cccccccccccc",
					0,
					"Primary",
					0.88,
					map[string]float64{"correctness": 0.81},
					evalSessionTestTask("task-a", true),
				),
			),
		},
		[]uuid.UUID{uuid.MustParse("44444444-4444-4444-4444-444444444444")},
		evalSessionTestBehavior(),
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}
	if scoredChildCount != 1 {
		t.Fatalf("scored child count = %d, want 1", scoredChildCount)
	}

	var aggregate evalSessionAggregateDocument
	if err := json.Unmarshal(aggregateJSON, &aggregate); err != nil {
		t.Fatalf("unmarshal aggregate: %v", err)
	}
	if aggregate.ScoredChildCount != 1 {
		t.Fatalf("aggregate scored child count = %d, want 1", aggregate.ScoredChildCount)
	}
	if aggregate.Overall == nil || aggregate.Overall.Interval != nil {
		t.Fatalf("overall aggregate = %#v, want no interval for n=1", aggregate.Overall)
	}

	var evidence evalSessionAggregateEvidence
	if err := json.Unmarshal(evidenceJSON, &evidence); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	if !slices.Contains(evidence.MissingScorecardRunIDs, uuid.MustParse("44444444-4444-4444-4444-444444444444")) {
		t.Fatalf("missing scorecard ids = %v, want included run", evidence.MissingScorecardRunIDs)
	}
	if !slices.Contains(evidence.Warnings, "confidence intervals require at least 2 scored child runs") {
		t.Fatalf("warnings = %v, want insufficient evidence warning", evidence.Warnings)
	}
	if !slices.Contains(evidence.Warnings, "1 child run scorecards are missing from aggregation evidence") {
		t.Fatalf("warnings = %v, want missing scorecard warning", evidence.Warnings)
	}
}

func TestBuildEvalSessionAggregatePayloadRejectsMissingScoredChildren(t *testing.T) {
	_, _, _, err := buildEvalSessionAggregatePayload(2, nil, nil, evalSessionTestBehavior(), nil)
	if !errors.Is(err, ErrEvalSessionAggregateUnavailable) {
		t.Fatalf("error = %v, want ErrEvalSessionAggregateUnavailable", err)
	}
}

func TestBuildEvalSessionAggregatePayloadComputesPassMetricsAndK1Equivalence(t *testing.T) {
	aggregateJSON, _, _, err := buildEvalSessionAggregatePayload(
		3,
		[]evalSessionAggregateSource{
			evalSessionTestSource(
				"55555555-5555-5555-5555-555555555551",
				evalSessionTestParticipant(
					"dddddddd-dddd-dddd-dddd-dddddddddddd",
					0,
					"Primary",
					0.80,
					map[string]float64{"correctness": 0.80},
					evalSessionTestTask("task-a", true),
					evalSessionTestTask("task-b", false),
				),
			),
			evalSessionTestSource(
				"55555555-5555-5555-5555-555555555552",
				evalSessionTestParticipant(
					"eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
					0,
					"Primary",
					0.90,
					map[string]float64{"correctness": 0.90},
					evalSessionTestTask("task-a", true),
					evalSessionTestTask("task-b", false),
				),
			),
			evalSessionTestSource(
				"55555555-5555-5555-5555-555555555553",
				evalSessionTestParticipant(
					"ffffffff-ffff-ffff-ffff-ffffffffffff",
					0,
					"Primary",
					0.70,
					map[string]float64{"correctness": 0.70},
					evalSessionTestTask("task-a", false),
					evalSessionTestTask("task-b", false),
				),
			),
		},
		nil,
		evalSessionTestBehavior(),
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}

	var aggregate evalSessionAggregateDocument
	if err := json.Unmarshal(aggregateJSON, &aggregate); err != nil {
		t.Fatalf("unmarshal aggregate: %v", err)
	}
	if aggregate.PassAtK == nil || aggregate.PassPowK == nil || aggregate.MetricRouting == nil {
		t.Fatalf("top-level pass metrics/routing missing: %#v", aggregate)
	}
	if len(aggregate.TaskSuccess) != 2 {
		t.Fatalf("task_success length = %d, want 2", len(aggregate.TaskSuccess))
	}

	taskA := aggregate.TaskSuccess[0]
	taskB := aggregate.TaskSuccess[1]
	if taskA.TaskKey != "task-a" || taskB.TaskKey != "task-b" {
		t.Fatalf("task ordering = %#v, want task-a/task-b", aggregate.TaskSuccess)
	}

	if math.Abs(taskA.SuccessRate-(2.0/3.0)) > 1e-9 {
		t.Fatalf("task-a success rate = %f, want %f", taskA.SuccessRate, 2.0/3.0)
	}
	if taskA.PassAtK["1"] != taskA.SuccessRate || taskA.PassPowK["1"] != taskA.SuccessRate {
		t.Fatalf("task-a k=1 equivalence failed: %#v", taskA)
	}
	if taskB.SuccessRate != 0 || taskB.PassAtK["1"] != 0 || taskB.PassPowK["1"] != 0 {
		t.Fatalf("task-b rates = %#v, want zeros", taskB)
	}

	passAt1 := aggregate.PassAtK.ByK["1"].Mean
	passAt3 := aggregate.PassAtK.ByK["3"].Mean
	passAt5 := aggregate.PassAtK.ByK["5"].Mean
	passAt10 := aggregate.PassAtK.ByK["10"].Mean
	passPow1 := aggregate.PassPowK.ByK["1"].Mean
	passPow3 := aggregate.PassPowK.ByK["3"].Mean
	passPow5 := aggregate.PassPowK.ByK["5"].Mean
	passPow10 := aggregate.PassPowK.ByK["10"].Mean

	if math.Abs(passAt1-passPow1) > 1e-9 {
		t.Fatalf("suite k=1 mismatch: pass@1=%f pass^1=%f", passAt1, passPow1)
	}
	if !(passAt1 <= passAt3 && passAt3 <= passAt5 && passAt5 <= passAt10) {
		t.Fatalf("pass@k should be monotonic increasing: %f %f %f %f", passAt1, passAt3, passAt5, passAt10)
	}
	if !(passPow1 >= passPow3 && passPow3 >= passPow5 && passPow5 >= passPow10) {
		t.Fatalf("pass^k should be monotonic decreasing: %f %f %f %f", passPow1, passPow3, passPow5, passPow10)
	}
	if aggregate.MetricRouting.PrimaryMetric != "pass_at_k" {
		t.Fatalf("primary metric = %q, want pass_at_k", aggregate.MetricRouting.PrimaryMetric)
	}
}

func TestResolveEvalSessionReliabilityWeightInfersFromTaskProperties(t *testing.T) {
	source, weight, reasoning := resolveEvalSessionReliabilityWeight(evalSessionAggregateBehavior{
		TaskProperties: evalSessionRoutingTaskProperties{
			HasSideEffects: true,
			Autonomy:       "full",
			StepCount:      4,
			OutputType:     "action",
		},
	})

	if source != "task_properties" {
		t.Fatalf("source = %q, want task_properties", source)
	}
	if math.Abs(weight-1.0) > 1e-9 {
		t.Fatalf("weight = %f, want 1.0", weight)
	}
	for _, fragment := range []string{"side effects", "fully autonomous", "4 steps", "action"} {
		if !strings.Contains(reasoning, fragment) {
			t.Fatalf("reasoning = %q, want fragment %q", reasoning, fragment)
		}
	}
}

func TestBuildEvalSessionMetricRoutingUsesManualOverride(t *testing.T) {
	manualWeight := 0.85
	routing := buildEvalSessionMetricRouting(
		evalSessionAggregateBehavior{
			EffectiveK:              3,
			ManualReliabilityWeight: &manualWeight,
			TaskProperties:          evalSessionRoutingTaskProperties{HasSideEffects: true, Autonomy: "full", StepCount: 5, OutputType: "action"},
		},
		&evalSessionPassMetricSeries{
			EffectiveK: 3,
			ByK: map[string]evalSessionMetricAggregate{
				"3": buildEvalSessionMetricAggregate([]float64{0.9, 0.8}),
			},
		},
		&evalSessionPassMetricSeries{
			EffectiveK: 3,
			ByK: map[string]evalSessionMetricAggregate{
				"3": buildEvalSessionMetricAggregate([]float64{0.6, 0.5}),
			},
		},
	)
	if routing == nil {
		t.Fatal("routing = nil, want value")
	}
	if routing.Source != "manual_override" {
		t.Fatalf("source = %q, want manual_override", routing.Source)
	}
	if routing.ReliabilityWeight != manualWeight {
		t.Fatalf("weight = %f, want %f", routing.ReliabilityWeight, manualWeight)
	}
	if routing.PrimaryMetric != "pass_pow_k" {
		t.Fatalf("primary metric = %q, want pass_pow_k", routing.PrimaryMetric)
	}
	if !strings.Contains(routing.Reasoning, "manual override") {
		t.Fatalf("reasoning = %q, want manual override explanation", routing.Reasoning)
	}
}

func TestDeriveEvalSessionChallengeSuccessSupportsBinaryAndContinuousModes(t *testing.T) {
	failVerdict := "fail"
	passVerdict := "pass"
	trueResult, source, ok := deriveEvalSessionChallengeSuccess([]JudgeResultRecord{
		{Verdict: &passVerdict},
		{Verdict: &passVerdict},
	}, 0.8)
	if !ok || !trueResult || source != "judge_results_verdict" {
		t.Fatalf("binary success = (%v,%q,%v), want (true,judge_results_verdict,true)", trueResult, source, ok)
	}

	falseResult, source, ok := deriveEvalSessionChallengeSuccess([]JudgeResultRecord{
		{Verdict: &passVerdict},
		{Verdict: &failVerdict},
	}, 0.8)
	if !ok || falseResult || source != "judge_results_verdict" {
		t.Fatalf("binary failure = (%v,%q,%v), want (false,judge_results_verdict,true)", falseResult, source, ok)
	}

	verdictWithScore, source, ok := deriveEvalSessionChallengeSuccess([]JudgeResultRecord{
		{Verdict: &passVerdict, NormalizedScore: float64Ptr(0.1)},
		{Verdict: &passVerdict, NormalizedScore: float64Ptr(0.2)},
	}, 0.8)
	if !ok || !verdictWithScore || source != "judge_results_verdict" {
		t.Fatalf("verdict precedence = (%v,%q,%v), want (true,judge_results_verdict,true)", verdictWithScore, source, ok)
	}

	continuous, source, ok := deriveEvalSessionChallengeSuccess([]JudgeResultRecord{
		{NormalizedScore: float64Ptr(0.7)},
		{NormalizedScore: float64Ptr(0.9)},
	}, 0.8)
	if !ok || !continuous || source != "judge_results_threshold" {
		t.Fatalf("continuous success = (%v,%q,%v), want (true,judge_results_threshold,true)", continuous, source, ok)
	}
}

func TestDeriveEvalSessionSuiteFallbackOutcomeEnforcesRequiredDimensions(t *testing.T) {
	outcome, ok := deriveEvalSessionSuiteFallbackOutcome(runScorecardAgentSummary{
		OverallScore:     float64Ptr(0.90),
		CorrectnessScore: float64Ptr(0.90),
		ReliabilityScore: float64Ptr(0.70),
	}, evalSessionAggregateBehavior{
		SuccessThreshold:     0.8,
		RequireAllDimensions: []string{"correctness", "reliability"},
	})
	if !ok {
		t.Fatal("fallback outcome unresolved, want resolved")
	}
	if outcome.Success {
		t.Fatalf("fallback success = true, want false because reliability dimension misses threshold")
	}

	passedOutcome, ok := deriveEvalSessionSuiteFallbackOutcome(runScorecardAgentSummary{
		Passed: boolPtr(true),
	}, evalSessionAggregateBehavior{SuccessThreshold: 0.8})
	if !ok || !passedOutcome.Success || passedOutcome.Source != "scorecard_passed" {
		t.Fatalf("passed fallback = %#v, want scorecard_passed success", passedOutcome)
	}
}

func TestBuildEvalSessionAggregatePayloadComparisonClearWinner(t *testing.T) {
	aggregateJSON, evidenceJSON, _, err := buildEvalSessionAggregatePayload(
		3,
		[]evalSessionAggregateSource{
			evalSessionTestComparisonSource("66666666-6666-6666-6666-666666666661", true, false),
			evalSessionTestComparisonSource("66666666-6666-6666-6666-666666666662", true, false),
			evalSessionTestComparisonSource("66666666-6666-6666-6666-666666666663", true, false),
		},
		nil,
		evalSessionAggregateBehavior{KValues: []int{1, 3, 5, 10}, EffectiveK: 3, SuccessThreshold: 0.8},
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}

	var aggregate evalSessionAggregateDocument
	if err := json.Unmarshal(aggregateJSON, &aggregate); err != nil {
		t.Fatalf("unmarshal aggregate: %v", err)
	}
	if aggregate.Comparison == nil {
		t.Fatal("comparison = nil, want clear winner result")
	}
	if aggregate.Comparison.Status != "clear_winner" {
		t.Fatalf("comparison status = %q, want clear_winner", aggregate.Comparison.Status)
	}
	if aggregate.Comparison.WinnerLaneIndex == nil || *aggregate.Comparison.WinnerLaneIndex != 0 {
		t.Fatalf("winner lane index = %v, want 0", aggregate.Comparison.WinnerLaneIndex)
	}
	if aggregate.TopLevelSource != "repeated_clear_winner" {
		t.Fatalf("top_level_source = %q, want repeated_clear_winner", aggregate.TopLevelSource)
	}
	if aggregate.Overall == nil {
		t.Fatal("top-level overall missing for clear winner comparison")
	}

	var evidence evalSessionAggregateEvidence
	if err := json.Unmarshal(evidenceJSON, &evidence); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	for _, warning := range evidence.Warnings {
		if strings.Contains(warning, "top-level winner aggregate omitted") {
			t.Fatalf("warnings = %v, want no omission warning for clear winner", evidence.Warnings)
		}
	}
}

func TestBuildEvalSessionAggregatePayloadComparisonUsesWinnerLabel(t *testing.T) {
	aggregateJSON, _, _, err := buildEvalSessionAggregatePayload(
		3,
		[]evalSessionAggregateSource{
			evalSessionTestSameLaneComparisonSource("67676767-6767-6767-6767-676767676761"),
			evalSessionTestSameLaneComparisonSource("67676767-6767-6767-6767-676767676762"),
			evalSessionTestSameLaneComparisonSource("67676767-6767-6767-6767-676767676763"),
		},
		nil,
		evalSessionAggregateBehavior{KValues: []int{1, 3, 5, 10}, EffectiveK: 3, SuccessThreshold: 0.8},
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}

	var aggregate evalSessionAggregateDocument
	if err := json.Unmarshal(aggregateJSON, &aggregate); err != nil {
		t.Fatalf("unmarshal aggregate: %v", err)
	}
	if aggregate.Comparison == nil || aggregate.Comparison.Status != "clear_winner" {
		t.Fatalf("comparison = %#v, want clear_winner", aggregate.Comparison)
	}
	if aggregate.Comparison.WinnerLaneIndex == nil || *aggregate.Comparison.WinnerLaneIndex != 0 {
		t.Fatalf("winner lane index = %v, want 0", aggregate.Comparison.WinnerLaneIndex)
	}
	if aggregate.Comparison.WinnerLabel != "smoke / Primary" {
		t.Fatalf("winner label = %q, want smoke / Primary", aggregate.Comparison.WinnerLabel)
	}
	if aggregate.Overall == nil || math.Abs(aggregate.Overall.Mean-0.95) > 1e-9 {
		t.Fatalf("top-level overall = %#v, want smoke winner aggregate", aggregate.Overall)
	}
}

func TestBuildEvalSessionAggregatePayloadSeriesEffectiveKUsesRunsPerLineup(t *testing.T) {
	aggregateJSON, _, _, err := buildEvalSessionAggregatePayload(
		6,
		[]evalSessionAggregateSource{
			evalSessionTestSeriesSource("68686868-6868-6868-6868-686868686861", "default", false, 0.30),
			evalSessionTestSeriesSource("68686868-6868-6868-6868-686868686862", "default", false, 0.30),
			evalSessionTestSeriesSource("68686868-6868-6868-6868-686868686863", "default", false, 0.30),
			evalSessionTestSeriesSource("68686868-6868-6868-6868-686868686864", "smoke", true, 0.95),
			evalSessionTestSeriesSource("68686868-6868-6868-6868-686868686865", "smoke", true, 0.95),
			evalSessionTestSeriesSource("68686868-6868-6868-6868-686868686866", "smoke", true, 0.95),
		},
		nil,
		evalSessionAggregateBehavior{KValues: []int{1, 3, 5, 6, 10}, EffectiveK: 6, SuccessThreshold: 0.8},
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}

	var aggregate evalSessionAggregateDocument
	if err := json.Unmarshal(aggregateJSON, &aggregate); err != nil {
		t.Fatalf("unmarshal aggregate: %v", err)
	}
	if aggregate.Comparison == nil || aggregate.Comparison.EffectiveK != 3 {
		t.Fatalf("comparison = %#v, want effective_k 3 from per-lineup repetitions", aggregate.Comparison)
	}
	if aggregate.Comparison.WinnerLabel != "smoke / Primary" {
		t.Fatalf("winner label = %q, want smoke / Primary", aggregate.Comparison.WinnerLabel)
	}
	for _, participant := range aggregate.Participants {
		if participant.MetricRouting == nil || participant.MetricRouting.EffectiveK != 3 {
			t.Fatalf("participant = %#v, want metric routing effective_k 3", participant)
		}
	}
}

func TestBuildEvalSessionAggregatePayloadSeriesLabelsDisambiguateLineups(t *testing.T) {
	aggregateJSON, _, _, err := buildEvalSessionAggregatePayload(
		2,
		[]evalSessionAggregateSource{
			evalSessionTestSeriesDocumentSource("69696969-6969-6969-6969-696969696961", "default", 0.30),
			evalSessionTestSeriesDocumentSource("69696969-6969-6969-6969-696969696962", "smoke", 0.95),
		},
		nil,
		evalSessionAggregateBehavior{KValues: []int{1, 2, 3, 5, 10}, EffectiveK: 2, SuccessThreshold: 0.8},
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}

	var aggregate evalSessionAggregateDocument
	if err := json.Unmarshal(aggregateJSON, &aggregate); err != nil {
		t.Fatalf("unmarshal aggregate: %v", err)
	}
	if len(aggregate.Participants) != 2 {
		t.Fatalf("participants = %#v, want separate lineup participants", aggregate.Participants)
	}
	labels := []string{aggregate.Participants[0].Label, aggregate.Participants[1].Label}
	if !slices.Contains(labels, "default / Primary") || !slices.Contains(labels, "smoke / Primary") {
		t.Fatalf("participant labels = %v, want lineup-qualified labels", labels)
	}
}

func TestEvalSessionSeriesParticipantLabelAvoidsDuplicateLineup(t *testing.T) {
	if got := evalSessionSeriesParticipantLabel("smoke", "smoke"); got != "smoke" {
		t.Fatalf("label = %q, want smoke", got)
	}
	if got := evalSessionSeriesParticipantLabel("premium", "Primary"); got != "premium / Primary" {
		t.Fatalf("label = %q, want premium / Primary", got)
	}
}

func TestEvalSessionParticipantByComparisonWinnerRejectsMissingLabel(t *testing.T) {
	laneIndex := int32(0)
	_, ok := evalSessionParticipantByComparisonWinner(
		[]evalSessionParticipantAggregate{
			{LaneIndex: 0, Label: "default / Primary"},
			{LaneIndex: 0, Label: "smoke / Primary"},
		},
		&evalSessionRepeatedComparison{
			WinnerLaneIndex: &laneIndex,
			WinnerLabel:     "premium / Primary",
		},
	)
	if ok {
		t.Fatal("winner lookup succeeded with mismatched non-empty label; want fail closed")
	}
}

func TestBuildEvalSessionAggregatePayloadComparisonNoClearWinner(t *testing.T) {
	aggregateJSON, evidenceJSON, _, err := buildEvalSessionAggregatePayload(
		3,
		[]evalSessionAggregateSource{
			evalSessionTestOverlapSource("77777777-7777-7777-7777-777777777771",
				map[string]bool{"task-a": true, "task-b": true, "task-c": false},
				map[string]bool{"task-a": true, "task-b": false, "task-c": false},
			),
			evalSessionTestOverlapSource("77777777-7777-7777-7777-777777777772",
				map[string]bool{"task-a": true, "task-b": false, "task-c": false},
				map[string]bool{"task-a": true, "task-b": true, "task-c": false},
			),
			evalSessionTestOverlapSource("77777777-7777-7777-7777-777777777773",
				map[string]bool{"task-a": true, "task-b": false, "task-c": false},
				map[string]bool{"task-a": false, "task-b": false, "task-c": false},
			),
		},
		nil,
		evalSessionAggregateBehavior{KValues: []int{1, 3, 5, 10}, EffectiveK: 3, SuccessThreshold: 0.8},
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}

	var aggregate evalSessionAggregateDocument
	if err := json.Unmarshal(aggregateJSON, &aggregate); err != nil {
		t.Fatalf("unmarshal aggregate: %v", err)
	}
	if aggregate.Comparison == nil || aggregate.Comparison.Status != "no_clear_winner" {
		t.Fatalf("comparison = %#v, want no_clear_winner", aggregate.Comparison)
	}
	if aggregate.TopLevelSource != "" || aggregate.Overall != nil {
		t.Fatalf("top-level winner summary should be omitted for noisy comparison: %#v", aggregate)
	}

	var evidence evalSessionAggregateEvidence
	if err := json.Unmarshal(evidenceJSON, &evidence); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	if !slices.Contains(evidence.Warnings, "comparison session top-level winner aggregate omitted because repeated-session evidence overlaps and no clear winner exists") {
		t.Fatalf("warnings = %v, want noisy-comparison omission warning", evidence.Warnings)
	}
}

func TestBuildEvalSessionAggregatePayloadComparisonInsufficientEvidence(t *testing.T) {
	aggregateJSON, evidenceJSON, _, err := buildEvalSessionAggregatePayload(
		1,
		[]evalSessionAggregateSource{
			evalSessionTestComparisonSource("88888888-8888-8888-8888-888888888881", true, false),
		},
		nil,
		evalSessionAggregateBehavior{KValues: []int{1, 3, 5, 10}, EffectiveK: 1, SuccessThreshold: 0.8},
		nil,
	)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}

	var aggregate evalSessionAggregateDocument
	if err := json.Unmarshal(aggregateJSON, &aggregate); err != nil {
		t.Fatalf("unmarshal aggregate: %v", err)
	}
	if aggregate.Comparison == nil || aggregate.Comparison.Status != "insufficient_evidence" {
		t.Fatalf("comparison = %#v, want insufficient_evidence", aggregate.Comparison)
	}
	if aggregate.Comparison.ReasonCode != "scored_child_runs_insufficient" {
		t.Fatalf("reason code = %q, want scored_child_runs_insufficient", aggregate.Comparison.ReasonCode)
	}
	if aggregate.TopLevelSource != "" || aggregate.Overall != nil {
		t.Fatalf("top-level winner summary should be omitted: %#v", aggregate)
	}

	var evidence evalSessionAggregateEvidence
	if err := json.Unmarshal(evidenceJSON, &evidence); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	if !slices.Contains(evidence.Warnings, "comparison session top-level winner aggregate omitted because repeated-session evidence is insufficient") {
		t.Fatalf("warnings = %v, want insufficient-evidence omission warning", evidence.Warnings)
	}
}

func TestBuildEvalSessionRepeatedComparisonHandlesMetricRoutingMismatchDefensively(t *testing.T) {
	comparison := buildEvalSessionRepeatedComparison([]evalSessionParticipantAggregate{
		{
			LaneIndex: 0,
			Label:     "Alpha",
			PassAtK: &evalSessionPassMetricSeries{
				EffectiveK: 3,
				ByK: map[string]evalSessionMetricAggregate{
					"3": buildEvalSessionMetricAggregate([]float64{0.9, 0.8}),
				},
			},
			PassPowK: &evalSessionPassMetricSeries{
				EffectiveK: 3,
				ByK: map[string]evalSessionMetricAggregate{
					"3": buildEvalSessionMetricAggregate([]float64{0.7, 0.6}),
				},
			},
			MetricRouting: &evalSessionMetricRouting{
				PrimaryMetric:       "pass_at_k",
				EffectiveK:          3,
				CompositeAgentScore: 0.85,
			},
		},
		{
			LaneIndex: 1,
			Label:     "Beta",
			PassAtK: &evalSessionPassMetricSeries{
				EffectiveK: 3,
				ByK: map[string]evalSessionMetricAggregate{
					"3": buildEvalSessionMetricAggregate([]float64{0.8, 0.7}),
				},
			},
			PassPowK: &evalSessionPassMetricSeries{
				EffectiveK: 3,
				ByK: map[string]evalSessionMetricAggregate{
					"3": buildEvalSessionMetricAggregate([]float64{0.6, 0.5}),
				},
			},
			MetricRouting: &evalSessionMetricRouting{
				PrimaryMetric:       "pass_pow_k",
				EffectiveK:          3,
				CompositeAgentScore: 0.75,
			},
		},
	}, 3, 3)

	if comparison == nil {
		t.Fatal("comparison = nil, want defensive mismatch result")
	}
	if comparison.Status != "insufficient_evidence" {
		t.Fatalf("status = %q, want insufficient_evidence", comparison.Status)
	}
	if comparison.ReasonCode != "metric_routing_mismatch" {
		t.Fatalf("reason_code = %q, want metric_routing_mismatch", comparison.ReasonCode)
	}
}

func TestBuildEvalSessionMetricAggregateReturnsZeroValueForEmptyInput(t *testing.T) {
	aggregate := buildEvalSessionMetricAggregate(nil)

	if aggregate != (evalSessionMetricAggregate{}) {
		t.Fatalf("aggregate = %#v, want zero value", aggregate)
	}
}

func evalSessionTestBehavior() evalSessionAggregateBehavior {
	return evalSessionAggregateBehavior{
		KValues:          []int{1, 3, 5, 10},
		EffectiveK:       3,
		SuccessThreshold: 0.8,
	}
}

func evalSessionTestSource(runID string, participants ...evalSessionAggregateParticipantSource) evalSessionAggregateSource {
	agents := make([]runScorecardAgentSummary, 0, len(participants))
	for _, participant := range participants {
		agents = append(agents, participant.Agent)
	}
	return evalSessionAggregateSource{
		RunID:              uuid.MustParse(runID),
		Document:           runScorecardDocument{Agents: agents},
		ParticipantSources: participants,
	}
}

func evalSessionTestParticipant(
	runAgentID string,
	laneIndex int32,
	label string,
	overall float64,
	dimensions map[string]float64,
	tasks ...evalSessionAggregateTaskOutcome,
) evalSessionAggregateParticipantSource {
	agent := runScorecardAgentSummary{
		RunAgentID:   uuid.MustParse(runAgentID),
		LaneIndex:    laneIndex,
		Label:        label,
		HasScorecard: true,
		OverallScore: float64Ptr(overall),
		Dimensions:   map[string]comparisonScorecardDimensionInfo{},
	}
	for key, value := range dimensions {
		agent.Dimensions[key] = comparisonScorecardDimensionInfo{Score: float64Ptr(value)}
	}
	return evalSessionAggregateParticipantSource{
		Key: evalSessionParticipantKey{
			LaneIndex: laneIndex,
			Label:     label,
		},
		Agent:        agent,
		TaskOutcomes: tasks,
	}
}

func evalSessionTestTask(taskKey string, success bool) evalSessionAggregateTaskOutcome {
	return evalSessionAggregateTaskOutcome{
		TaskKey: taskKey,
		Success: success,
		Source:  "judge_results_verdict",
	}
}

func evalSessionTestComparisonSource(runID string, alphaSuccess bool, betaSuccess bool) evalSessionAggregateSource {
	return evalSessionTestSource(
		runID,
		evalSessionTestParticipant(
			runIDToRunAgentID(runID, "00000000-0000-0000-0000-000000000001"),
			0,
			"Alpha",
			0.95,
			map[string]float64{"correctness": 0.95},
			evalSessionTestTask("task-a", alphaSuccess),
			evalSessionTestTask("task-b", alphaSuccess),
			evalSessionTestTask("task-c", alphaSuccess),
		),
		evalSessionTestParticipant(
			runIDToRunAgentID(runID, "00000000-0000-0000-0000-000000000002"),
			1,
			"Beta",
			0.30,
			map[string]float64{"correctness": 0.30},
			evalSessionTestTask("task-a", betaSuccess),
			evalSessionTestTask("task-b", betaSuccess),
			evalSessionTestTask("task-c", betaSuccess),
		),
	)
}

func evalSessionTestSameLaneComparisonSource(runID string) evalSessionAggregateSource {
	return evalSessionTestSource(
		runID,
		evalSessionTestParticipant(
			runIDToRunAgentID(runID, "00000000-0000-0000-0000-000000000001"),
			0,
			"default / Primary",
			0.30,
			map[string]float64{"correctness": 0.30},
			evalSessionTestTask("task-a", false),
			evalSessionTestTask("task-b", false),
			evalSessionTestTask("task-c", false),
		),
		evalSessionTestParticipant(
			runIDToRunAgentID(runID, "00000000-0000-0000-0000-000000000002"),
			0,
			"smoke / Primary",
			0.95,
			map[string]float64{"correctness": 0.95},
			evalSessionTestTask("task-a", true),
			evalSessionTestTask("task-b", true),
			evalSessionTestTask("task-c", true),
		),
	)
}

func evalSessionTestSeriesSource(runID string, lineup string, success bool, overall float64) evalSessionAggregateSource {
	label := evalSessionSeriesParticipantLabel(lineup, "Primary")
	return evalSessionTestSource(
		runID,
		evalSessionTestParticipant(
			runIDToRunAgentID(runID, "00000000-0000-0000-0000-000000000001"),
			0,
			label,
			overall,
			map[string]float64{"correctness": overall},
			evalSessionTestTask("task-a", success),
			evalSessionTestTask("task-b", success),
			evalSessionTestTask("task-c", success),
		),
	)
}

func evalSessionTestSeriesDocumentSource(runID string, lineup string, overall float64) evalSessionAggregateSource {
	agentID := uuid.MustParse(runIDToRunAgentID(runID, "00000000-0000-0000-0000-000000000001"))
	return evalSessionAggregateSource{
		RunID:            uuid.MustParse(runID),
		DeploymentLineup: lineup,
		Document: runScorecardDocument{
			Agents: []runScorecardAgentSummary{
				{
					RunAgentID:   agentID,
					LaneIndex:    0,
					Label:        "Primary",
					HasScorecard: true,
					OverallScore: float64Ptr(overall),
					Dimensions: map[string]comparisonScorecardDimensionInfo{
						"correctness": {Score: float64Ptr(overall)},
					},
				},
			},
		},
	}
}

func evalSessionTestOverlapSource(runID string, alpha map[string]bool, beta map[string]bool) evalSessionAggregateSource {
	return evalSessionTestSource(
		runID,
		evalSessionTestParticipant(
			runIDToRunAgentID(runID, "00000000-0000-0000-0000-000000000011"),
			0,
			"Alpha",
			0.70,
			map[string]float64{"correctness": 0.70},
			evalSessionTestTask("task-a", alpha["task-a"]),
			evalSessionTestTask("task-b", alpha["task-b"]),
			evalSessionTestTask("task-c", alpha["task-c"]),
		),
		evalSessionTestParticipant(
			runIDToRunAgentID(runID, "00000000-0000-0000-0000-000000000022"),
			1,
			"Beta",
			0.68,
			map[string]float64{"correctness": 0.68},
			evalSessionTestTask("task-a", beta["task-a"]),
			evalSessionTestTask("task-b", beta["task-b"]),
			evalSessionTestTask("task-c", beta["task-c"]),
		),
	)
}

func runIDToRunAgentID(runID string, suffix string) string {
	return runID[:24] + suffix[24:]
}

func boolPtr(value bool) *bool {
	return &value
}
