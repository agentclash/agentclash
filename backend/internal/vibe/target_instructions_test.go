package vibe

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestIntegrationSuppliedInstructionsRemainExactAcrossClarification(t *testing.T) {
	s, root := buildService(t)
	ctx := context.Background()
	v, err := s.Store.CreateAgent(ctx, root.Actor, nil, uuid.New(), "test", DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, s.Store, v.ID)
	instructions := "You answer return questions. Do not invent a shop policy.\nAsk when unsure."
	content := "Check these agent instructions:\n" + instructions
	o, err := s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: content, Instructions: instructions, Models: DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(askAction{Kind: "ask", Text: "Which returns should qualify?", Purpose: "clarify_rule", Options: []string{}}, factObservation{Kind: "job", Quote: "You answer return questions."})
	})
	if v.Document.TargetInstructions != instructions || len(v.Document.Artifacts) != 0 {
		t.Fatal("instructions were lost during clarification")
	}
	rule := "Only unopened items bought within 30 days qualify."
	o, err = s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: rule, Models: DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	v = executeBuildFixture(t, s, o, func(req provider.Request) any {
		calls++
		switch calls {
		case 1:
			value := interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "rule", Quote: rule})
			value.Answer = &answerObservation{Quote: rule}
			return value
		case 2:
			var input taskInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			id := input.CurrentRequest.ID
			return createSuiteCommand{Rules: []PolicyRule{{ID: "returns", Statement: rule, SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: rule, Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Returns", Summary: rule, SuccessCriteria: rule, Scenarios: []TestScenario{{Input: "Unopened, 10 days.", Expected: "Eligible."}, {Input: "Opened, 10 days.", Expected: "Not eligible."}, {Input: "Unopened, 45 days.", Expected: "Not eligible."}}}}
		case 3:
			var input SuiteReviewInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			return supportedSuiteReview(input)
		default:
			t.Fatal("unexpected target rewriting call")
			return nil
		}
	})
	if len(v.Document.Artifacts) != 1 || v.Document.Artifacts[0].AgentPrompt != instructions {
		t.Fatal("expected answers or policy rewrote the supplied target")
	}
	if len(v.Operations) != 2 || v.Operations[1].Kind != "message" {
		t.Fatal("instructions ran without authorization")
	}
	before := Hash(raw(v.Document))
	_, err = s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "New instructions", Instructions: "New instructions", Models: DefaultModels()})
	requireFault(t, err, "invalid_message")
	after, _ := s.Store.GetSession(ctx, v.Actor, v.ID)
	if Hash(raw(after.Document)) != before {
		t.Fatal("replacement intake silently changed the existing agent")
	}
}
