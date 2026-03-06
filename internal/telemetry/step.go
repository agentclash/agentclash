package telemetry

import "time"

// StepType classifies what happened in a step.
type StepType string

const (
	StepThink   StepType = "think"   // LLM call — the agent reasons
	StepAct     StepType = "act"     // tool execution — the agent does something
	StepObserve StepType = "observe" // race state injected — the agent sees standings
)

// Step is a single atomic event in an agent's run.
type Step struct {
	Number    int           `json:"number"`
	Type      StepType      `json:"type"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration_ms"`

	// Think steps
	LLMResponse string `json:"llm_response,omitempty"`
	TokensUsed  int    `json:"tokens_used,omitempty"`

	// Act steps
	ToolName   string `json:"tool_name,omitempty"`
	ToolInput  string `json:"tool_input,omitempty"`
	ToolOutput string `json:"tool_output,omitempty"`
	ToolError  string `json:"tool_error,omitempty"`
	Success    bool   `json:"success,omitempty"`

	// Observe steps
	Observation string `json:"observation,omitempty"`

	// Race awareness
	RaceStateInjected bool `json:"race_state_injected,omitempty"`
	PositionAtStep    int  `json:"position_at_step,omitempty"`
}
