package telemetry

import "time"

// Trace is the complete record of one agent's run in a race.
type Trace struct {
	RaceID    string `json:"race_id"`
	AgentName string `json:"agent_name"`
	Model     string `json:"model"`

	Steps     []Step    `json:"steps"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	// Outcome
	Submitted         bool   `json:"submitted"`
	SubmitExplanation string `json:"submit_explanation,omitempty"`
	Error             string `json:"error,omitempty"`

	// Aggregated metrics (computed by Finish)
	TotalTokens     int `json:"total_tokens"`
	TotalLLMCalls   int `json:"total_llm_calls"`
	TotalToolCalls  int `json:"total_tool_calls"`
	UniqueToolsUsed int `json:"unique_tools_used"`
	FailedToolCalls int `json:"failed_tool_calls"`
}

func NewTrace(raceID, agentName, model string) *Trace {
	return &Trace{
		RaceID:    raceID,
		AgentName: agentName,
		Model:     model,
		StartedAt: time.Now(),
	}
}

func (t *Trace) AddStep(s Step) {
	s.Number = len(t.Steps) + 1
	t.Steps = append(t.Steps, s)
}

// Finish computes aggregate metrics. Call after the agent's run is done.
func (t *Trace) Finish() {
	t.EndedAt = time.Now()

	tools := make(map[string]bool)
	for _, s := range t.Steps {
		t.TotalTokens += s.TokensUsed
		switch s.Type {
		case StepThink:
			t.TotalLLMCalls++
		case StepAct:
			t.TotalToolCalls++
			if s.ToolName != "" {
				tools[s.ToolName] = true
			}
			if !s.Success {
				t.FailedToolCalls++
			}
		}
	}
	t.UniqueToolsUsed = len(tools)
}
