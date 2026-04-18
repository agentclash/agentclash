package failurereview

import (
	"encoding/json"
	"testing"

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
		RunID:               runID,
		RunStatus:           "completed",
		RunAgentID:          runAgentID,
		DeploymentType:      "native",
		ChallengePackStatus: "runnable",
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

func TestBuildRunAgentItemsHandlesHostedBlackBoxEligibility(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	runAgentID := uuid.New()
	challengeID := uuid.New()
	verdict := "fail"

	items, err := BuildRunAgentItems(RunAgentInput{
		RunID:               runID,
		RunStatus:           "completed",
		RunAgentID:          runAgentID,
		DeploymentType:      "hosted_external",
		ChallengePackStatus: "archived",
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
}

func TestAssembleFailureReviewItemBuildsRefsAndFailedChecks(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	runAgentID := uuid.New()
	challengeID := uuid.New()
	verdict := "fail"

	items, err := BuildRunAgentItems(RunAgentInput{
		RunID:               runID,
		RunStatus:           "completed",
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
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return encoded
}
