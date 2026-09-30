package vibe

import (
	"fmt"
	"strings"
)

// Journey is conversation context, never evidence of a live connection.
// PreviewConsent is writable only by the user's revision-checked edit action.
type Journey struct {
	Mode           string `json:"mode,omitempty"`
	Stack          string `json:"stack,omitempty"`
	Evidence       string `json:"evidence,omitempty"`
	PreviewConsent bool   `json:"preview_consent,omitempty"`
}

type JourneyProposal struct {
	Mode     string `json:"mode"`
	Stack    string `json:"stack"`
	Evidence string `json:"evidence"`
}

type TestScenario struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
}

type TestPlan struct {
	Title          string         `json:"title"`
	Objective      string         `json:"objective"`
	Scenarios      []TestScenario `json:"scenarios"`
	EvidenceNeeded []string       `json:"evidence_needed"`
	NextSteps      []string       `json:"next_steps"`
	LocalTestCode  string         `json:"local_test_code"`
}

// Wire representation of the model's single artifact union. IDs, provenance,
// acceptance and execution permissions are supplied only by the server.
type AuthoringArtifact struct {
	Summary             string         `json:"summary"`
	Kind                string         `json:"kind"`
	Title               string         `json:"title"`
	AgentPrompt         string         `json:"agent_prompt"`
	Examples            []string       `json:"examples"`
	PositiveExample     string         `json:"positive_example"`
	NegativeExample     string         `json:"negative_example"`
	InsufficientExample string         `json:"insufficient_example"`
	SuccessCriteria     string         `json:"success_criteria"`
	Objective           string         `json:"objective"`
	Scenarios           []TestScenario `json:"scenarios"`
	EvidenceNeeded      []string       `json:"evidence_needed"`
	NextSteps           []string       `json:"next_steps"`
	LocalTestCode       string         `json:"local_test_code"`
}

type Capability struct {
	CriteriaInstructions string   `json:"criteria_instructions,omitempty"`
	ID                   string   `json:"id"`
	Label                string   `json:"label"`
	Available            bool     `json:"available"`
	Description          string   `json:"description"`
	Instructions         string   `json:"instructions,omitempty"`
	URL                  string   `json:"url,omitempty"`
	Example              string   `json:"example,omitempty"`
	NextSteps            []string `json:"next_steps,omitempty"`
}

const LocalPythonHandoff = `from agentclash_eval import assert_agent
from agentclash_eval.metrics import Contains

def invoke_agent(case: dict) -> str:
    # Map case["input"] to YOUR function, workflow or HTTP request schema.
    # Configure credentials locally, connect/read timeouts and an overall deadline.
    # Return observed text (or serialize your JSON output); capture tool calls separately.
    # Raise on timeout or unavailable output. Never substitute a mock success.
    raise NotImplementedError("Configure your own authenticated, bounded invocation")

def test_observed_output():
    case = {"input": "[REPLACE with this scenario's input]"}
    output = invoke_agent(case)
    assert isinstance(output, str)
    assert_agent(output, metrics=[Contains("[REPLACE with expected text]")])

# Run: pytest tests/
# Contains checks text presence only. Use Python assertions on actual JSON/tool
# traces for ranking, sources, corrected arguments, retries and unknown outcomes.
`

func LocalHandoffSteps() []string {
	return []string{"Review the scenarios and expected behavior; describe any changes you want.", "Export this plan and use the linked pytest documentation in your own environment.", "Map inputs and observed outputs to your invocation. Configure local credentials, connect/read timeouts and an overall deadline.", "Run your own tests and capture outputs, sources and tool results. Unknown action outcomes require investigation before retrying; audio and telephony need their own harness."}
}

func Capabilities() []Capability {
	return []Capability{
		{ID: "text_preview", Label: "Text preview", Available: true, Description: "Analyzes supplied text. No company discovery, source fetching or business actions.", Instructions: previewActionPolicy, CriteriaInstructions: previewEvidencePolicy},
		{ID: "existing_agent", Label: "Existing agent testing", Description: "Your live agent is not connected. Review supplied examples or export a test plan for your own environment.", URL: "/docs/guides/vibe-evals-existing-agent"},
		{ID: "voice", Label: "Voice and telephony", Description: "Text/mock tests exclude STT/TTS, acoustic interruptions, real transfers and call latency."},
		{ID: "python_sdk", Label: "Local Python tests", Description: "Local pytest: assert_agent(output, metrics=[Contains(\"text\")]) from agentclash_eval and agentclash_eval.metrics. Text presence only; your invocation, auth and timeouts.", URL: "https://github.com/agentclash/agentclash-evals/blob/main/docs/evaltest/pytest.md", Example: LocalPythonHandoff, NextSteps: LocalHandoffSteps()},
	}
}

const previewActionPolicy = "Preview capabilities: this is a text-only simulation with no external tools. Never claim a booking, transfer, notification, escalation, or future follow-up has happened or will happen. Explain unavailable actions honestly; label hypothetical actions as simulated. A described human process is not a connected action. Use only supplied business facts and leave unknown policies unknown.\n\n"

func PreviewPrompt(prompt string) string {
	if strings.HasPrefix(prompt, previewActionPolicy) {
		return prompt
	}
	return previewActionPolicy + prompt
}

func (a Artifact) IsTestPlan() bool { return a.Kind == "test_plan" }

func (p *TestPlan) validate(l Limits) error {
	if p == nil {
		return nil
	}
	if strings.TrimSpace(p.Title) == "" || len(p.Title) > MaxKeyBytes || strings.TrimSpace(p.Objective) == "" || len(p.Objective) > 4000 || len(p.Scenarios) < 1 || len(p.Scenarios) > 3 || len(p.EvidenceNeeded) < 1 || len(p.EvidenceNeeded) > 5 || len(p.NextSteps) > 5 || len(p.LocalTestCode) > l.MessageBytes {
		return fmt.Errorf("test_plan needs a bounded title, objective, one to three scenarios and at most five evidence/next steps")
	}
	for _, s := range p.Scenarios {
		if strings.TrimSpace(s.Input) == "" || len(strings.Fields(s.Expected)) < 4 || len(s.Input) > 4000 || len(s.Expected) > 4000 {
			return fmt.Errorf("each scenario needs input and an observable expected behavior of at least four words, not just a failure category")
		}
	}
	for _, list := range [][]string{p.EvidenceNeeded, p.NextSteps} {
		for _, s := range list {
			if strings.TrimSpace(s) == "" || len(s) > 4000 {
				return fmt.Errorf("test plan steps must be nonempty bounded strings")
			}
		}
	}
	return nil
}

func (p *TestPlan) Validate(l Limits) error { return p.validate(l) }
