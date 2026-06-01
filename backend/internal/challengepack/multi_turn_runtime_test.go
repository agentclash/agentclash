package challengepack

import (
	"testing"

	"github.com/google/uuid"
)

func TestShouldSampleCalibration_IsDeterministic(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	first := ShouldSampleCalibration(id, 0.25)
	second := ShouldSampleCalibration(id, 0.25)
	if first != second {
		t.Fatal("expected deterministic sampling for the same run agent id")
	}
}

func TestShouldSampleCalibration_AlwaysWhenRateOne(t *testing.T) {
	t.Parallel()
	if !ShouldSampleCalibration(uuid.New(), 1) {
		t.Fatal("expected sample when rate is 1")
	}
}

func TestPairArenaAgents_AdjacentLanesPerCase(t *testing.T) {
	t.Parallel()
	left := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	right := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	otherCase := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	pairs := PairArenaAgents([]ArenaEligibleAgent{
		{RunAgentID: right, CaseKey: "case-a", LaneIndex: 1},
		{RunAgentID: left, CaseKey: "case-a", LaneIndex: 0},
		{RunAgentID: otherCase, CaseKey: "case-b", LaneIndex: 0},
	})
	if len(pairs) != 1 {
		t.Fatalf("len(pairs) = %d; want 1", len(pairs))
	}
	if pairs[0][0] != left || pairs[0][1] != right {
		t.Fatalf("unexpected pair order: %v", pairs[0])
	}
}
