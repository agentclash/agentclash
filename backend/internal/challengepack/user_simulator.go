package challengepack

import "strings"

const (
	UserSimulatorSchemaVersionV1 = 1
	UserSimulatorKindHybrid      = "hybrid"

	UserSimulatorActorScripted = "scripted"
	UserSimulatorActorLLM      = "llm"
	UserSimulatorActorHuman    = "human"

	UserSimulatorTriggerAlways              = "always"
	UserSimulatorTriggerOnAssistantMismatch = "on_assistant_mismatch"
	UserSimulatorTriggerOnValidatorFail     = "on_validator_fail"
	UserSimulatorTriggerOnJudgeBelow        = "on_judge_below"
	UserSimulatorTriggerOnAgentLoop         = "on_agent_loop"
	UserSimulatorTriggerOnMaxLLMTurns       = "on_max_llm_turns"
	UserSimulatorTriggerManual              = "manual"
	UserSimulatorTriggerNever               = "never"

	UserSimulatorArenaComparisonPairwise = "pairwise"
)

// UserSimulatorSpec defines the per-case hybrid user actor manifest for multi_turn packs.
type UserSimulatorSpec struct {
	SchemaVersion int32                     `yaml:"schema_version" json:"schema_version"`
	Kind          string                    `yaml:"kind" json:"kind"`
	MaxTurns      int32                     `yaml:"max_turns,omitempty" json:"max_turns,omitempty"`
	Phases        []UserSimulatorPhase      `yaml:"phases" json:"phases"`
	Calibration   *UserSimulatorCalibration `yaml:"calibration,omitempty" json:"calibration,omitempty"`
	PostRun       *UserSimulatorPostRun     `yaml:"post_run,omitempty" json:"post_run,omitempty"`
}

type UserSimulatorPhase struct {
	ID        string              `yaml:"id" json:"id"`
	Actor     string              `yaml:"actor" json:"actor"`
	Trigger   string              `yaml:"trigger,omitempty" json:"trigger,omitempty"`
	Turns     []UserSimulatorTurn `yaml:"turns,omitempty" json:"turns,omitempty"`
	Persona   string              `yaml:"persona,omitempty" json:"persona,omitempty"`
	MaxTurns  int32               `yaml:"max_turns,omitempty" json:"max_turns,omitempty"`
	Until     []string            `yaml:"until,omitempty" json:"until,omitempty"`
	TimeoutMS int64               `yaml:"timeout_ms,omitempty" json:"timeout_ms,omitempty"`
}

type UserSimulatorTurn struct {
	Message string            `yaml:"message" json:"message"`
	Expects []CaseExpectation `yaml:"expects,omitempty" json:"expects,omitempty"`
}

type UserSimulatorCalibration struct {
	Enabled    bool    `yaml:"enabled" json:"enabled"`
	SampleRate float64 `yaml:"sample_rate,omitempty" json:"sample_rate,omitempty"`
}

type UserSimulatorPostRun struct {
	Arena *UserSimulatorArena `yaml:"arena,omitempty" json:"arena,omitempty"`
}

type UserSimulatorArena struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Comparison string `yaml:"comparison,omitempty" json:"comparison,omitempty"`
}

var supportedUserSimulatorActors = map[string]struct{}{
	UserSimulatorActorScripted: {},
	UserSimulatorActorLLM:      {},
	UserSimulatorActorHuman:    {},
}

var supportedUserSimulatorTriggers = map[string]struct{}{
	UserSimulatorTriggerAlways:              {},
	UserSimulatorTriggerOnAssistantMismatch: {},
	UserSimulatorTriggerOnValidatorFail:     {},
	UserSimulatorTriggerOnJudgeBelow:        {},
	UserSimulatorTriggerOnAgentLoop:         {},
	UserSimulatorTriggerOnMaxLLMTurns:       {},
	UserSimulatorTriggerManual:              {},
	UserSimulatorTriggerNever:               {},
}

var supportedUserSimulatorArenaComparisons = map[string]struct{}{
	UserSimulatorArenaComparisonPairwise: {},
}

func normalizeUserSimulatorTrigger(trigger string) string {
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		return UserSimulatorTriggerAlways
	}
	return trigger
}
