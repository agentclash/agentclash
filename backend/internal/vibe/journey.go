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

func (a *assistantReply) unpackArtifact(version int) error {
	if version >= 3 {
		if a.Draft != nil || a.TestPlan != nil {
			return fmt.Errorf("use the single artifact field; legacy draft/test_plan fields are not valid for this request")
		}
		if x := a.Artifact; x != nil && x.Kind == "agent_draft" {
			if len(x.Examples) != 0 || strings.TrimSpace(x.PositiveExample) == "" || strings.TrimSpace(x.NegativeExample) == "" || strings.TrimSpace(x.InsufficientExample) == "" {
				return fmt.Errorf("supply separate positive_example, negative_example and insufficient_example inputs")
			}
		}
	}
	if a.Artifact == nil {
		return nil
	} // Legacy queued replies remain readable.
	if a.Draft != nil || a.TestPlan != nil {
		return fmt.Errorf("use only the single artifact field")
	}
	x := a.Artifact
	switch x.Kind {
	case "agent_draft":
		if x.Objective != "" || len(x.Scenarios) > 0 || x.LocalTestCode != "" || len(x.EvidenceNeeded) > 0 || len(x.NextSteps) > 0 {
			return fmt.Errorf("an agent_draft cannot also contain a test_plan")
		}
		examples := x.Examples
		if x.PositiveExample != "" || x.NegativeExample != "" || x.InsufficientExample != "" {
			if strings.TrimSpace(x.PositiveExample) == "" || strings.TrimSpace(x.NegativeExample) == "" || strings.TrimSpace(x.InsufficientExample) == "" || len(examples) > 0 {
				return fmt.Errorf("supply three separate positive, negative and insufficient-evidence inputs")
			}
			examples = []string{x.PositiveExample, x.NegativeExample, x.InsufficientExample}
		}
		a.Draft = &DraftProposal{Title: x.Title, AgentPrompt: x.AgentPrompt, Examples: examples, SuccessCriteria: x.SuccessCriteria}
	case "test_plan":
		if x.AgentPrompt != "" || len(x.Examples) > 0 || x.SuccessCriteria != "" || x.PositiveExample != "" || x.NegativeExample != "" || x.InsufficientExample != "" {
			return fmt.Errorf("a test_plan cannot contain executable agent instructions or preview criteria")
		}
		a.TestPlan = &TestPlan{Title: x.Title, Objective: x.Objective, Scenarios: x.Scenarios, EvidenceNeeded: x.EvidenceNeeded, NextSteps: x.NextSteps, LocalTestCode: x.LocalTestCode}
	default:
		return fmt.Errorf("artifact.kind must be agent_draft or test_plan")
	}
	return nil
}

type Capability struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Available    bool     `json:"available"`
	Description  string   `json:"description"`
	Instructions string   `json:"instructions,omitempty"`
	URL          string   `json:"url,omitempty"`
	Example      string   `json:"example,omitempty"`
	NextSteps    []string `json:"next_steps,omitempty"`
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
	return []string{"Review the scenarios and expected behavior; request changes in Design.", "Export this plan and use the linked pytest documentation in your own environment.", "Map inputs and observed outputs to your invocation. Configure local credentials, connect/read timeouts and an overall deadline.", "Run your own tests and capture outputs, sources and tool results. Unknown action outcomes require investigation before retrying; audio and telephony need their own harness."}
}

func Capabilities() []Capability {
	return []Capability{
		{ID: "text_preview", Label: "Text preview", Available: true, Description: "Analyzes supplied text. No company discovery, source fetching or business actions.", Instructions: previewActionPolicy},
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

func (a assistantReply) validateJourney(p Plan, l Limits) error {
	if a.Journey == nil {
		return fmt.Errorf("journey is required: record mode, known stack and available evidence")
	}
	j := a.Journey
	if j.Mode != "idea" && j.Mode != "existing" && j.Mode != "exploring" {
		return fmt.Errorf("invalid journey mode")
	}
	if len(j.Stack) > 1000 || len(j.Evidence) > 1000 {
		return fmt.Errorf("keep stack and evidence descriptions under 1000 bytes each")
	}
	if a.ReplyKind != "design" && a.ReplyKind != "support" {
		return fmt.Errorf("reply_kind must be design or support")
	}
	if a.ReplyKind == "support" && (len(a.Changes) > 0 || len(a.Assumptions) > 0 || a.Draft != nil) {
		return fmt.Errorf("use reply_kind design for drafting/testing agents; support is only for Vibe product help, without an agent_draft, requirement changes or assumptions")
	}
	if len(a.Requirements) > 0 {
		return fmt.Errorf("use requirement_changes instead of proposed_requirements")
	}
	if len(a.Changes) > 5 {
		return fmt.Errorf("at most five requirement changes; preserve coverage")
	}
	if len(a.CriteriaRequirementIDs) > 5 || (a.Draft == nil && len(a.CriteriaRequirementIDs) > 0) {
		return fmt.Errorf("criteria links require a draft and at most five existing requirement IDs")
	}
	for _, id := range a.CriteriaRequirementIDs {
		found := false
		for _, q := range p.Document.Requirements {
			if q.ID.String() == id && (q.Status == "accepted" || q.Status == "proposed") && q.Change != "remove" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("criteria_requirement_ids must reference supplied active requirement IDs; use an empty list for new proposals")
		}
	}
	if a.Draft != nil && a.TestPlan != nil {
		return fmt.Errorf("choose a draft or a test plan")
	}
	if a.Draft != nil && (j.Mode == "existing" || p.Document.Journey.Mode == "existing") && !p.Document.Journey.PreviewConsent {
		return fmt.Errorf("existing agent: ask about stack/evidence or produce test_plan; replacement draft requires explicit user preview consent")
	}
	if err := a.TestPlan.validate(l); err != nil {
		return err
	}
	if err := validatePreviewProposal(a); err != nil {
		return err
	}
	copy := p.Document
	copy.Requirements = append([]Requirement{}, copy.Requirements...)
	return ReconcileRequirements(&copy, a.Changes, p.Submission.ClientID, p.Submission.ClientID)
}
