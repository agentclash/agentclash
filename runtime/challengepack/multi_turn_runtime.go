package challengepack

import (
	"hash/fnv"

	"github.com/google/uuid"
)

// FirstCaseUserSimulator returns the user_simulator on the first case, if any.
func FirstCaseUserSimulator(cases []StoredCaseDocument) *UserSimulatorSpec {
	if len(cases) == 0 || cases[0].UserSimulator == nil {
		return nil
	}
	return CloneUserSimulatorSpec(cases[0].UserSimulator)
}

// ShouldSampleCalibration deterministically samples run agents for H2 calibration review.
func ShouldSampleCalibration(runAgentID uuid.UUID, sampleRate float64) bool {
	if sampleRate <= 0 {
		return false
	}
	if sampleRate >= 1 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write(runAgentID[:])
	bucket := h.Sum32() % 10000
	return float64(bucket) < sampleRate*10000
}

// NormalizeHumanOnTimeout returns the effective human-phase timeout policy.
func NormalizeHumanOnTimeout(raw string) string {
	switch raw {
	case UserSimulatorHumanOnTimeoutFail:
		return UserSimulatorHumanOnTimeoutFail
	default:
		return UserSimulatorHumanOnTimeoutStop
	}
}

// ArenaEligibleFromSpec reports whether post-run arena comparison is enabled.
func ArenaEligibleFromSpec(spec *UserSimulatorSpec) bool {
	if spec == nil || spec.PostRun == nil || spec.PostRun.Arena == nil {
		return false
	}
	return spec.PostRun.Arena.Enabled
}

// ArenaEligibleAgent is a run agent queued for pairwise arena comparison.
type ArenaEligibleAgent struct {
	RunAgentID uuid.UUID
	CaseKey    string
	LaneIndex  int32
}

// PairArenaAgents groups eligible agents by case_key and returns adjacent lane pairs.
func PairArenaAgents(agents []ArenaEligibleAgent) [][2]uuid.UUID {
	if len(agents) < 2 {
		return nil
	}
	byCase := map[string][]ArenaEligibleAgent{}
	for _, agent := range agents {
		key := agent.CaseKey
		if key == "" {
			key = "_default"
		}
		byCase[key] = append(byCase[key], agent)
	}

	var pairs [][2]uuid.UUID
	for _, group := range byCase {
		if len(group) < 2 {
			continue
		}
		sortArenaAgents(group)
		for i := 0; i+1 < len(group); i += 2 {
			pairs = append(pairs, [2]uuid.UUID{group[i].RunAgentID, group[i+1].RunAgentID})
		}
	}
	return pairs
}

func sortArenaAgents(agents []ArenaEligibleAgent) {
	for i := 0; i < len(agents); i++ {
		for j := i + 1; j < len(agents); j++ {
			if agents[j].LaneIndex < agents[i].LaneIndex ||
				(agents[j].LaneIndex == agents[i].LaneIndex && agents[j].RunAgentID.String() < agents[i].RunAgentID.String()) {
				agents[i], agents[j] = agents[j], agents[i]
			}
		}
	}
}
