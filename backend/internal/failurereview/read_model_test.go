package failurereview

import (
	"encoding/json"
	"strings"
	"testing"

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
