package failurereview

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/google/uuid"
)

func TestBuildRunAgentItemsComputesPromotionEligibilityAndRefs(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	runAgentID := uuid.New()
	challengeID := uuid.New()
	limitScore := 0.25
	metricValue := 42.0
	cursorSeq := int64(11)
	verdict := "fail"

	scorecard := mustJSON(t, map[string]any{
		"dimensions": map[string]any{
			"correctness": map[string]any{"state": "available", "score": limitScore},
		},
		"validator_details": []any{
			map[string]any{
				"key":     "policy.filesystem",
				"type":    "exact_match",
				"verdict": "fail",
				"state":   "available",
				"reason":  "policy rejected write",
				"source": map[string]any{
					"kind":       "final_output",
					"sequence":   cursorSeq,
					"event_type": "system.output.finalized",
				},
			},
		},
		"metric_details": []any{
			map[string]any{
				"key":           "total_tokens",
				"state":         "available",
				"numeric_value": metricValue,
			},
		},
	})

	items, err := BuildRunAgentItems(RunAgentInput{
		RunID:                runID,
		RunStatus:            domain.RunStatusCompleted,
		RunAgentID:           runAgentID,
		DeploymentType:       "native",
		ChallengePackStatus:  "runnable",
		HasChallengeInputSet: true,
		Cases: []CaseContext{
			{
				ChallengeIdentityID: challengeID,
				ChallengeKey:        "ticket-1",
				CaseKey:             "case-a",
				ItemKey:             "prompt.txt",
				Artifacts: []ArtifactContext{
					{Key: "workspace-zip", Kind: "archive", Path: "assets/workspace.zip"},
				},
			},
		},
		Scorecard: scorecard,
		JudgeResults: []JudgeResult{
			{
				ChallengeIdentityID: &challengeID,
				Key:                 "policy.filesystem",
				Verdict:             &verdict,
			},
		},
		MetricResults: []MetricResult{
			{
				ChallengeIdentityID: &challengeID,
				Key:                 "total_tokens",
				MetricType:          "numeric",
				NumericValue:        &metricValue,
			},
		},
		Events: []Event{
			{SequenceNumber: cursorSeq, EventType: "system.output.finalized", Payload: mustJSON(t, map[string]any{"final_output": "done"})},
		},
	})
	if err != nil {
		t.Fatalf("BuildRunAgentItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1", len(items))
	}

	item := items[0]
	if item.FailureClass != FailureClassPolicyViolation {
		t.Fatalf("failure class = %s, want %s", item.FailureClass, FailureClassPolicyViolation)
	}
	if item.FailureTaxonomy.Code != "agent.policy_violation" || !item.FailureTaxonomy.AgentFault {
		t.Fatalf("failure taxonomy = %+v, want agent.policy_violation agent fault", item.FailureTaxonomy)
	}
	if item.Severity != SeverityBlocking {
		t.Fatalf("severity = %s, want %s", item.Severity, SeverityBlocking)
	}
	if item.FailureState != FailureStateFailed {
		t.Fatalf("failure state = %s, want %s", item.FailureState, FailureStateFailed)
	}
	if item.EvidenceTier != EvidenceTierNativeStructured {
		t.Fatalf("evidence tier = %s, want %s", item.EvidenceTier, EvidenceTierNativeStructured)
	}
	if !item.Promotable {
		t.Fatal("promotable = false, want true")
	}
	if len(item.PromotionModeAvailable) != 2 {
		t.Fatalf("promotion modes = %#v, want full_executable + output_only", item.PromotionModeAvailable)
	}
	if len(item.ReplayStepRefs) != 1 || item.ReplayStepRefs[0].SequenceNumber != cursorSeq {
		t.Fatalf("replay refs = %#v, want one finalized-output ref", item.ReplayStepRefs)
	}
	if len(item.JudgeRefs) != 1 || item.JudgeRefs[0].Key != "policy.filesystem" {
		t.Fatalf("judge refs = %#v, want validator ref", item.JudgeRefs)
	}
	if len(item.MetricRefs) != 1 || item.MetricRefs[0].Key != "total_tokens" {
		t.Fatalf("metric refs = %#v, want metric ref", item.MetricRefs)
	}
	if len(item.FailedChecks) != 1 || item.FailedChecks[0] != "policy.filesystem" {
		t.Fatalf("failed checks = %#v, want policy.filesystem", item.FailedChecks)
	}
	if len(item.FailedDimensions) != 1 || item.FailedDimensions[0] != "correctness" {
		t.Fatalf("failed dimensions = %#v, want correctness", item.FailedDimensions)
	}
}

func TestBuildRunAgentItemsComputesStableFailureIdentity(t *testing.T) {
	t.Parallel()

	verdict := "fail"
	scorecard := mustJSON(t, map[string]any{
		"dimensions": map[string]any{
			"grounding":   map[string]any{"state": "available", "score": 0.4},
			"correctness": map[string]any{"state": "available", "score": 0.2},
		},
		"validator_details": []any{
			map[string]any{
				"key":     "tool_argument.schema",
				"type":    "json_schema",
				"verdict": "fail",
				"state":   "available",
				"source": map[string]any{
					"kind":       "tool_call",
					"sequence":   12,
					"event_type": "tool.call.completed",
				},
			},
			map[string]any{
				"key":     "policy.filesystem",
				"type":    "exact_match",
				"verdict": "fail",
				"state":   "available",
				"source": map[string]any{
					"kind":       "final_output",
					"sequence":   18,
					"event_type": "system.output.finalized",
				},
			},
		},
	})

	buildItem := func(runID, runAgentID, challengeID uuid.UUID, reversed bool) Item {
		t.Helper()
		judgeResults := []JudgeResult{
			{
				ChallengeIdentityID: &challengeID,
				Key:                 "tool_argument.schema",
				Verdict:             &verdict,
			},
			{
				ChallengeIdentityID: &challengeID,
				Key:                 "policy.filesystem",
				Verdict:             &verdict,
			},
		}
		if reversed {
			judgeResults[0], judgeResults[1] = judgeResults[1], judgeResults[0]
		}

		items, err := BuildRunAgentItems(RunAgentInput{
			RunID:               runID,
			RunStatus:           domain.RunStatusCompleted,
			RunAgentID:          runAgentID,
			DeploymentType:      "native",
			ChallengePackStatus: "runnable",
			Cases: []CaseContext{
				{
					ChallengeIdentityID: challengeID,
					ChallengeKey:        "ticket-stable",
					CaseKey:             "case-stable",
					ItemKey:             "prompt.txt",
				},
			},
			Scorecard:    scorecard,
			JudgeResults: judgeResults,
			Events: []Event{
				{SequenceNumber: 18, EventType: "system.output.finalized", Payload: mustJSON(t, map[string]any{"final_output": "oops"})},
			},
		})
		if err != nil {
			t.Fatalf("BuildRunAgentItems returned error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("item count = %d, want 1", len(items))
		}
		return items[0]
	}

	first := buildItem(uuid.New(), uuid.New(), uuid.New(), false)
	rerun := buildItem(uuid.New(), uuid.New(), uuid.New(), true)
	repeat := buildItem(first.RunID, first.RunAgentID, *first.ChallengeIdentityID, true)

	if !strings.HasPrefix(first.FailureFingerprint, "frf_") {
		t.Fatalf("failure fingerprint = %q, want frf_ prefix", first.FailureFingerprint)
	}
	if !strings.HasPrefix(first.FailureClusterKey, "frc_") {
		t.Fatalf("failure cluster key = %q, want frc_ prefix", first.FailureClusterKey)
	}
	if first.FailureFingerprint == rerun.FailureFingerprint {
		t.Fatalf("fingerprint stayed stable across run identity changes: %q", first.FailureFingerprint)
	}
	if first.FailureClusterKey != rerun.FailureClusterKey {
		t.Fatalf("cluster key = %q, want stable %q across reruns", rerun.FailureClusterKey, first.FailureClusterKey)
	}
	if first.FailureFingerprint != repeat.FailureFingerprint {
		t.Fatalf("fingerprint = %q, want deterministic %q for equivalent ordered inputs", repeat.FailureFingerprint, first.FailureFingerprint)
	}
	if first.FailureClusterKey != repeat.FailureClusterKey {
		t.Fatalf("cluster key = %q, want deterministic %q for equivalent ordered inputs", repeat.FailureClusterKey, first.FailureClusterKey)
	}
}

func TestFailureIdentityCanonicalizesSortedInputs(t *testing.T) {
	t.Parallel()

	challengeID := uuid.New()
	base := Item{
		RunID:               uuid.New(),
		RunAgentID:          uuid.New(),
		ChallengeIdentityID: &challengeID,
		ChallengeKey:        "ticket-canonical",
		CaseKey:             "case-canonical",
		ItemKey:             "prompt.txt",
		FailureState:        FailureStateFailed,
		FailureClass:        FailureClassToolArgumentError,
		FailedDimensions:    []string{"grounding", "correctness"},
		FailedChecks:        []string{"tool_argument.schema", "policy.filesystem"},
		EvidenceTier:        EvidenceTierNativeStructured,
		ReplayStepRefs: []ReplayStepRef{
			{SequenceNumber: 20, EventType: "tool.call.completed", Kind: "tool_call"},
			{SequenceNumber: 10, EventType: "system.output.finalized", Kind: "final_output"},
		},
	}
	reversed := base
	reversed.FailedDimensions = []string{"correctness", "grounding"}
	reversed.FailedChecks = []string{"policy.filesystem", "tool_argument.schema"}
	reversed.ReplayStepRefs = []ReplayStepRef{
		{SequenceNumber: 10, EventType: "system.output.finalized", Kind: "final_output"},
		{SequenceNumber: 20, EventType: "tool.call.completed", Kind: "tool_call"},
	}

	baseFingerprint, err := buildFailureFingerprint(base)
	if err != nil {
		t.Fatalf("buildFailureFingerprint(base) returned error: %v", err)
	}
	reversedFingerprint, err := buildFailureFingerprint(reversed)
	if err != nil {
		t.Fatalf("buildFailureFingerprint(reversed) returned error: %v", err)
	}
	if baseFingerprint != reversedFingerprint {
		t.Fatalf("fingerprint = %q, want canonical %q", reversedFingerprint, baseFingerprint)
	}

	baseClusterKey, err := buildFailureClusterKey(base)
	if err != nil {
		t.Fatalf("buildFailureClusterKey(base) returned error: %v", err)
	}
	reversedClusterKey, err := buildFailureClusterKey(reversed)
	if err != nil {
		t.Fatalf("buildFailureClusterKey(reversed) returned error: %v", err)
	}
	if baseClusterKey != reversedClusterKey {
		t.Fatalf("cluster key = %q, want canonical %q", reversedClusterKey, baseClusterKey)
	}
}

func TestBuildClusterSummariesGroupsAndSortsDeterministically(t *testing.T) {
	t.Parallel()

	runAgentA := uuid.New()
	runAgentB := uuid.New()
	items := []Item{
		{
			RunAgentID:         runAgentA,
			ChallengeKey:       "ticket-b",
			CaseKey:            "case-b",
			FailureFingerprint: "frf-b1",
			FailureClusterKey:  "frc-b",
			FailureState:       FailureStateFailed,
			FailureClass:       FailureClassToolArgumentError,
			FailureTaxonomy:    TaxonomyForFailureClass(FailureClassToolArgumentError),
			EvidenceTier:       EvidenceTierNativeStructured,
			Severity:           SeverityWarning,
			Headline:           "ticket-b triggered tool_argument_error",
			RecommendedAction:  "Inspect the replay around the failing tool call and correct the selection or arguments.",
			Promotable:         true,
		},
		{
			RunAgentID:         runAgentB,
			ChallengeKey:       "ticket-a",
			CaseKey:            "case-a",
			FailureFingerprint: "frf-a",
			FailureClusterKey:  "frc-a",
			FailureState:       FailureStateFailed,
			FailureClass:       FailureClassPolicyViolation,
			FailureTaxonomy:    TaxonomyForFailureClass(FailureClassPolicyViolation),
			EvidenceTier:       EvidenceTierNativeStructured,
			Severity:           SeverityBlocking,
			Headline:           "ticket-a triggered policy_violation",
			RecommendedAction:  "Tighten the agent or tool policy before promoting this failure.",
			Promotable:         true,
		},
		{
			RunAgentID:         runAgentB,
			ChallengeKey:       "ticket-b",
			CaseKey:            "case-c",
			FailureFingerprint: "frf-b2",
			FailureClusterKey:  "frc-b",
			FailureState:       FailureStateWarning,
			FailureClass:       FailureClassToolArgumentError,
			FailureTaxonomy:    TaxonomyForFailureClass(FailureClassToolArgumentError),
			EvidenceTier:       EvidenceTierNativeStructured,
			Severity:           SeverityWarning,
			Headline:           "ticket-b triggered tool_argument_error",
			RecommendedAction:  "Inspect the replay around the failing tool call and correct the selection or arguments.",
			Promotable:         false,
		},
	}

	summaries := BuildClusterSummaries(items)
	if len(summaries) != 2 {
		t.Fatalf("cluster count = %d, want 2: %#v", len(summaries), summaries)
	}
	if summaries[0].FailureClusterKey != "frc-a" {
		t.Fatalf("first cluster = %q, want blocking cluster frc-a before warning cluster", summaries[0].FailureClusterKey)
	}
	if summaries[0].FailureTaxonomy.Code != "agent.policy_violation" {
		t.Fatalf("first cluster taxonomy = %+v, want policy violation", summaries[0].FailureTaxonomy)
	}
	if summaries[1].Count != 2 || summaries[1].PromotableCount != 1 {
		t.Fatalf("frc-b counts = %d/%d, want count 2 promotable 1", summaries[1].Count, summaries[1].PromotableCount)
	}
	if !reflect.DeepEqual(summaries[1].CaseKeys, []string{"case-b", "case-c"}) {
		t.Fatalf("case keys = %#v, want sorted case-b/case-c", summaries[1].CaseKeys)
	}
	wantRunAgentIDs := []string{runAgentA.String(), runAgentB.String()}
	sort.Strings(wantRunAgentIDs)
	if !reflect.DeepEqual(summaries[1].RunAgentIDs, wantRunAgentIDs) {
		t.Fatalf("run agent ids = %#v, want sorted unique ids", summaries[1].RunAgentIDs)
	}
	if summaries[1].RepresentativeFailureFingerprint != "frf-b1" {
		t.Fatalf("representative fingerprint = %q, want first matching fingerprint", summaries[1].RepresentativeFailureFingerprint)
	}
}

func TestAttachClusterHistoryLabelsNewAndRecurringTrends(t *testing.T) {
	t.Parallel()

	mostRecent := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	older := mostRecent.Add(-24 * time.Hour)
	mostRecentRunID := uuid.New()
	olderRunID := uuid.New()

	testCases := []struct {
		name             string
		currentCount     int
		historicalCounts []int
		wantTrend        ClusterTrend
		wantPriorRuns    int
		wantPriorFails   int
		wantLastCount    int
	}{
		{
			name:         "new",
			currentCount: 1,
			wantTrend:    ClusterTrendNew,
		},
		{
			name:             "increasing",
			currentCount:     3,
			historicalCounts: []int{1, 2},
			wantTrend:        ClusterTrendIncreasing,
			wantPriorRuns:    2,
			wantPriorFails:   3,
			wantLastCount:    1,
		},
		{
			name:             "decreasing",
			currentCount:     1,
			historicalCounts: []int{3},
			wantTrend:        ClusterTrendDecreasing,
			wantPriorRuns:    1,
			wantPriorFails:   3,
			wantLastCount:    3,
		},
		{
			name:             "recurring",
			currentCount:     2,
			historicalCounts: []int{2},
			wantTrend:        ClusterTrendRecurring,
			wantPriorRuns:    1,
			wantPriorFails:   2,
			wantLastCount:    2,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			historicalRuns := []ClusterHistoryRun{
				{
					RunID:      mostRecentRunID,
					ObservedAt: mostRecent,
					Clusters:   historicalCluster("frc-target", tt.historicalCounts, 0),
				},
				{
					RunID:      olderRunID,
					ObservedAt: older,
					Clusters:   historicalCluster("frc-target", tt.historicalCounts, 1),
				},
			}
			summaries := AttachClusterHistory([]ClusterSummary{{
				FailureClusterKey: "frc-target",
				Count:             tt.currentCount,
			}}, historicalRuns)
			history := summaries[0].History
			if history == nil {
				t.Fatal("history = nil, want populated")
			}

			if history.Trend != tt.wantTrend {
				t.Fatalf("trend = %s, want %s", history.Trend, tt.wantTrend)
			}
			if history.WindowRunCount != 2 {
				t.Fatalf("window_run_count = %d, want 2", history.WindowRunCount)
			}
			if history.PriorRunCount != tt.wantPriorRuns || history.PriorFailureCount != tt.wantPriorFails {
				t.Fatalf("prior counts = %d/%d, want %d/%d", history.PriorRunCount, history.PriorFailureCount, tt.wantPriorRuns, tt.wantPriorFails)
			}
			if tt.wantPriorRuns == 0 {
				if history.LastSeenRunID != nil || history.LastSeenAt != nil {
					t.Fatalf("last seen = %v/%v, want nils for new cluster", history.LastSeenRunID, history.LastSeenAt)
				}
				return
			}
			if history.LastSeenRunID == nil || *history.LastSeenRunID != mostRecentRunID {
				t.Fatalf("last_seen_run_id = %v, want %s", history.LastSeenRunID, mostRecentRunID)
			}
			if history.LastSeenAt == nil || !history.LastSeenAt.Equal(mostRecent) {
				t.Fatalf("last_seen_at = %v, want %s", history.LastSeenAt, mostRecent)
			}
			if history.LastRunFailureCount != tt.wantLastCount {
				t.Fatalf("last_run_failure_count = %d, want %d", history.LastRunFailureCount, tt.wantLastCount)
			}
		})
	}
}

func historicalCluster(key string, counts []int, index int) []ClusterSummary {
	if index >= len(counts) {
		return nil
	}
	return []ClusterSummary{{
		FailureClusterKey: key,
		Count:             counts[index],
	}}
}

func TestBuildRunAgentItemsHandlesHostedBlackBoxEligibility(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	runAgentID := uuid.New()
	challengeID := uuid.New()
	verdict := "fail"

	items, err := BuildRunAgentItems(RunAgentInput{
		RunID:                runID,
		RunStatus:            domain.RunStatusCompleted,
		RunAgentID:           runAgentID,
		DeploymentType:       "hosted_external",
		ChallengePackStatus:  "archived",
		HasChallengeInputSet: true,
		Cases: []CaseContext{
			{
				ChallengeIdentityID: challengeID,
				ChallengeKey:        "ticket-2",
				CaseKey:             "case-b",
				ItemKey:             "prompt.txt",
			},
		},
		JudgeResults: []JudgeResult{
			{
				ChallengeIdentityID: &challengeID,
				Key:                 "tool_argument.schema",
				Verdict:             &verdict,
			},
		},
		Events: []Event{
			{SequenceNumber: 3, EventType: "system.run.completed", Payload: mustJSON(t, map[string]any{"final_output": "oops"})},
		},
	})
	if err != nil {
		t.Fatalf("BuildRunAgentItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1", len(items))
	}
	item := items[0]
	if item.EvidenceTier != EvidenceTierHostedBlackBox {
		t.Fatalf("evidence tier = %s, want %s", item.EvidenceTier, EvidenceTierHostedBlackBox)
	}
	if item.FailureClass != FailureClassToolArgumentError {
		t.Fatalf("failure class = %s, want %s", item.FailureClass, FailureClassToolArgumentError)
	}
	if !item.Promotable {
		t.Fatal("promotable = false, want true from completed run with challenge identity")
	}
	if len(item.PromotionModeAvailable) != 1 || item.PromotionModeAvailable[0] != PromotionModeOutputOnly {
		t.Fatalf("promotion modes = %#v, want output_only only", item.PromotionModeAvailable)
	}
	if item.Severity != SeverityWarning {
		t.Fatalf("severity = %s, want %s for hosted black-box evidence", item.Severity, SeverityWarning)
	}
}

func TestBuildRunAgentItemsSkipsOutputOnlyPromotionWithoutChallengeInputSet(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	runAgentID := uuid.New()
	challengeID := uuid.New()
	verdict := "fail"

	items, err := BuildRunAgentItems(RunAgentInput{
		RunID:                runID,
		RunStatus:            domain.RunStatusCompleted,
		RunAgentID:           runAgentID,
		DeploymentType:       "native",
		ChallengePackStatus:  "runnable",
		HasChallengeInputSet: false,
		Cases: []CaseContext{
			{
				ChallengeIdentityID: challengeID,
				ChallengeKey:        "ticket-no-input-set",
				CaseKey:             "case-z",
				ItemKey:             "prompt.txt",
			},
		},
		JudgeResults: []JudgeResult{
			{
				ChallengeIdentityID: &challengeID,
				Key:                 "policy.filesystem",
				Verdict:             &verdict,
			},
		},
		Events: []Event{
			{SequenceNumber: 4, EventType: "system.output.finalized", Payload: mustJSON(t, map[string]any{"final_output": "oops"})},
		},
	})
	if err != nil {
		t.Fatalf("BuildRunAgentItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1", len(items))
	}
	if len(items[0].PromotionModeAvailable) != 0 {
		t.Fatalf("promotion modes = %#v, want none when challenge input set is unavailable", items[0].PromotionModeAvailable)
	}
}

func TestBuildRunAgentItemsSkipsFullExecutablePromotionWithoutChallengeInputSet(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	runAgentID := uuid.New()
	challengeID := uuid.New()
	verdict := "fail"

	items, err := BuildRunAgentItems(RunAgentInput{
		RunID:                runID,
		RunStatus:            domain.RunStatusCompleted,
		RunAgentID:           runAgentID,
		DeploymentType:       "native",
		ChallengePackStatus:  "runnable",
		HasChallengeInputSet: false,
		Cases: []CaseContext{
			{
				ChallengeIdentityID: challengeID,
				ChallengeKey:        "ticket-native-no-input-set",
				CaseKey:             "case-y",
				ItemKey:             "prompt.txt",
			},
		},
		Scorecard: mustJSON(t, map[string]any{
			"validator_details": []any{
				map[string]any{
					"key":     "policy.filesystem",
					"type":    "exact_match",
					"verdict": "fail",
					"state":   "available",
				},
			},
		}),
		JudgeResults: []JudgeResult{
			{
				ChallengeIdentityID: &challengeID,
				Key:                 "policy.filesystem",
				Verdict:             &verdict,
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildRunAgentItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1", len(items))
	}
	if len(items[0].PromotionModeAvailable) != 0 {
		t.Fatalf("promotion modes = %#v, want none without a challenge input set", items[0].PromotionModeAvailable)
	}
}

func TestAssembleFailureReviewItemBuildsRefsAndFailedChecks(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	runAgentID := uuid.New()
	challengeID := uuid.New()
	verdict := "fail"

	items, err := BuildRunAgentItems(RunAgentInput{
		RunID:               runID,
		RunStatus:           domain.RunStatusCompleted,
		RunAgentID:          runAgentID,
		DeploymentType:      "native",
		ChallengePackStatus: "runnable",
		Cases: []CaseContext{
			{
				ChallengeIdentityID: challengeID,
				ChallengeKey:        "ticket-3",
				CaseKey:             "case-c",
				ItemKey:             "prompt.txt",
			},
		},
		Scorecard: mustJSON(t, map[string]any{
			"dimensions": map[string]any{
				"correctness": map[string]any{"state": "available", "score": 0.2},
			},
			"validator_details": []any{
				map[string]any{
					"key":     "tool_argument.schema",
					"type":    "json_schema",
					"verdict": "fail",
					"state":   "available",
					"source": map[string]any{
						"kind":       "final_output",
						"sequence":   7,
						"event_type": "system.output.finalized",
					},
				},
			},
		}),
		JudgeResults: []JudgeResult{
			{
				ChallengeIdentityID: &challengeID,
				Key:                 "tool_argument.schema",
				Verdict:             &verdict,
			},
		},
		Events: []Event{
			{SequenceNumber: 7, EventType: "system.output.finalized", Payload: mustJSON(t, map[string]any{"final_output": "oops"})},
		},
	})
	if err != nil {
		t.Fatalf("BuildRunAgentItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1", len(items))
	}

	item := items[0]
	if len(item.FailedChecks) != 1 || item.FailedChecks[0] != "tool_argument.schema" {
		t.Fatalf("failed checks = %#v, want tool_argument.schema", item.FailedChecks)
	}
	if len(item.ReplayStepRefs) != 1 || item.ReplayStepRefs[0].SequenceNumber != 7 {
		t.Fatalf("replay refs = %#v, want finalized-output ref", item.ReplayStepRefs)
	}
	if item.ArtifactRefs == nil {
		t.Fatal("artifact refs = nil, want empty slice for stable JSON arrays")
	}
	if item.MetricRefs == nil {
		t.Fatal("metric refs = nil, want empty slice for stable JSON arrays")
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	if !json.Valid(encoded) || !strings.Contains(string(encoded), `"severity":"`) {
		t.Fatalf("encoded item = %s, want severity on the wire", encoded)
	}
	if !strings.Contains(string(encoded), `"failure_fingerprint":"`) || !strings.Contains(string(encoded), `"failure_cluster_key":"`) {
		t.Fatalf("encoded item = %s, want failure identity fields on the wire", encoded)
	}
}

func TestBuildRunAgentItemsIgnoresPassingLLMJudgeReasons(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	runAgentID := uuid.New()
	challengeID := uuid.New()
	score := 1.0

	items, err := BuildRunAgentItems(RunAgentInput{
		RunID:               runID,
		RunStatus:           domain.RunStatusCompleted,
		RunAgentID:          runAgentID,
		DeploymentType:      "native",
		ChallengePackStatus: "runnable",
		Cases: []CaseContext{
			{
				ChallengeIdentityID: challengeID,
				ChallengeKey:        "ticket-4",
				CaseKey:             "case-d",
				ItemKey:             "prompt.txt",
			},
		},
		LLMJudgeResults: []LLMJudgeResult{
			{
				Key:             "final_answer",
				NormalizedScore: &score,
				Reason:          "answer matches expected",
			},
		},
		Events: []Event{
			{SequenceNumber: 9, EventType: "system.output.finalized", Payload: mustJSON(t, map[string]any{"final_output": "ok"})},
		},
	})
	if err != nil {
		t.Fatalf("BuildRunAgentItems returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("item count = %d, want 0 for a passing LLM judge with explanatory reason", len(items))
	}
}

func TestCursorEncodingRoundTripsAndAcceptsLegacyJSON(t *testing.T) {
	t.Parallel()

	key := CursorKey{
		RunAgentID:   uuid.NewString(),
		ChallengeID:  uuid.NewString(),
		ChallengeKey: "ticket-5",
		CaseKey:      "case-e",
		ItemKey:      "prompt.txt",
	}

	encoded, err := EncodeCursor(key)
	if err != nil {
		t.Fatalf("EncodeCursor returned error: %v", err)
	}
	if encoded == "" || encoded[0] == '{' {
		t.Fatalf("encoded cursor = %q, want opaque non-JSON value", encoded)
	}
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor(encoded) returned error: %v", err)
	}
	if decoded != key {
		t.Fatalf("decoded cursor = %#v, want %#v", decoded, key)
	}

	legacyJSON := `{"RunAgentID":"` + key.RunAgentID + `","ChallengeID":"` + key.ChallengeID + `","ChallengeKey":"` + key.ChallengeKey + `","CaseKey":"` + key.CaseKey + `","ItemKey":"` + key.ItemKey + `"}`
	decodedLegacy, err := DecodeCursor(legacyJSON)
	if err != nil {
		t.Fatalf("DecodeCursor(legacy) returned error: %v", err)
	}
	if decodedLegacy != key {
		t.Fatalf("decoded legacy cursor = %#v, want %#v", decodedLegacy, key)
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return encoded
}
