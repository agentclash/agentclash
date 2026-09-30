package vibe

import (
	"context"
	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/google/uuid"
	"testing"
)

func TestBuildRegenerationPreservesExecutionContext(t *testing.T) {
	job := "Return the supplied project codename."
	p, o := memoryPlan(t, Document{}, job)
	p.AuthoringVersion = contextualBuildAuthoringVersion
	p.Document.Evaluation = &EvaluationContext{Door: "build"}
	p.Conversation.RequiredCount = 1
	rule := PolicyRule{ID: "codename", Statement: job, SourceBlockIDs: []string{p.Conversation.CurrentRequest.ID}, Evidence: []RuleEvidence{{SourceBlockID: p.Conversation.CurrentRequest.ID, Quote: job, Kind: "requirement"}}}
	p.Conversation.Policy = &PolicySnapshot{ID: uuid.New(), ScopeID: uuid.MustParse(effectiveConversationState(p).Brief.ScopeID), SourceVersion: SourcePolicyVersion, Rules: []PolicyRule{rule}}
	original := Artifact{ID: uuid.New(), Kind: "test_suite", AgentPrompt: "Look up the project codename in the reference.", InputContract: materialContract(), RequiredCapabilities: []string{"text_generation"}, ReferenceInputs: []inputs.Binding{{ID: uuid.New(), Hash: "reference-hash", Usage: "reference"}}, ScopeNote: "Uses the supplied reference."}
	p.Artifact = &original
	cmd := createSuiteCommand{Rules: []PolicyRule{rule}, Tests: testSuiteProposal{Title: "Codename", Summary: "Checks the supplied project codename.", SuccessCriteria: job, Scenarios: []TestScenario{{Input: "What is the codename?", Expected: "KESTREL"}}}}
	r := Runner{Service: &Service{Compiler: memoryCompiler{}}}
	candidate, policy, _, e := r.buildReliableCandidate(raw(cmd), "prepare_tests", o, p)
	if e != nil {
		t.Fatal(e)
	}
	if candidate.AgentPrompt != original.AgentPrompt {
		t.Fatal("probe did not reach retained-instructions path")
	}
	used := false
	if e = r.preparePrototype(context.Background(), o, p, candidate, policy, &used); e != nil {
		t.Fatal(e)
	}
	if !sameJSON(raw(candidate.ReferenceInputs), raw(original.ReferenceInputs)) || !sameJSON(raw(candidate.InputContract), raw(original.InputContract)) || !sameJSON(raw(candidate.RequiredCapabilities), raw(original.RequiredCapabilities)) || candidate.ScopeNote != original.ScopeNote {
		t.Fatal("execution context was dropped")
	}
	candidate.RequiredCapabilities[0] = "changed"
	candidate.InputContract.Formats[0] = "changed"
	candidate.ReferenceInputs[0].Hash = "changed"
	if original.RequiredCapabilities[0] != "text_generation" || original.InputContract.Formats[0] != "text" || original.ReferenceInputs[0].Hash != "reference-hash" {
		t.Fatal("historical execution context was mutated")
	}

}
