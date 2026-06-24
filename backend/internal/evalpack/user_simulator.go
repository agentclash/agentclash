package evalpack

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

	UserSimulatorHumanOnTimeoutStop = "stop"
	UserSimulatorHumanOnTimeoutFail = "fail"
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
	OnTimeout string              `yaml:"on_timeout,omitempty" json:"on_timeout,omitempty"`
	// Model optionally overrides the simulator's LLM model id for `actor: llm`
	// phases. When empty, the simulator inherits the deployment's model — but
	// some deployments use models that only support /v1/responses (e.g.
	// o4-mini, o3, o4-mini-deep-research) while the simulator's provider
	// client uses /v1/chat/completions. Setting Model to a chat-compatible
	// id (e.g. gpt-4o-mini, claude-haiku-4-5-20251001) avoids that mismatch.
	// Provider and credentials are still inherited from the deployment, so
	// the override must name a model that the deployment's provider serves.
	Model string `yaml:"model,omitempty" json:"model,omitempty"`
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

func CloneUserSimulatorSpec(spec *UserSimulatorSpec) *UserSimulatorSpec {
	return cloneUserSimulatorSpec(spec)
}

func cloneUserSimulatorSpec(spec *UserSimulatorSpec) *UserSimulatorSpec {
	if spec == nil {
		return nil
	}
	cloned := *spec
	if len(spec.Phases) > 0 {
		cloned.Phases = append([]UserSimulatorPhase(nil), spec.Phases...)
		for i, phase := range spec.Phases {
			if len(phase.Turns) > 0 {
				cloned.Phases[i].Turns = append([]UserSimulatorTurn(nil), phase.Turns...)
			}
			if len(phase.Until) > 0 {
				cloned.Phases[i].Until = append([]string(nil), phase.Until...)
			}
		}
	}
	if spec.Calibration != nil {
		calibration := *spec.Calibration
		cloned.Calibration = &calibration
	}
	if spec.PostRun != nil {
		postRun := *spec.PostRun
		if spec.PostRun.Arena != nil {
			arena := *spec.PostRun.Arena
			postRun.Arena = &arena
		}
		cloned.PostRun = &postRun
	}
	return &cloned
}
