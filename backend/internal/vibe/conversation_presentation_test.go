package vibe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestVibeGuidanceContractAndIsolation(t *testing.T) {
	s := newConversationState(uuid.New())
	p := Plan{AuthoringVersion: 14, Conversation: &ConversationContext{State: s}}
	id := uuid.New()
	example := GuidanceExample{Input: "PDF table with Product and Price", Expected: "Markdown keeps each price beside its product."}
	card := exampleCard(example, s.Brief.ScopeID, id)
	if err := validateMessageCards(p, []json.RawMessage{card}, s, id); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"html", "progress", "evidence", "recovery"} {
		var bad map[string]any
		_ = json.Unmarshal(card, &bad)
		bad["kind"] = kind
		if validateMessageCards(p, []json.RawMessage{raw(bad)}, s, id) == nil {
			t.Fatal("model issued a non-illustrative card", kind)
		}
	}
	for _, key := range []string{"origin_message_id", "scope_id", "id", "run", "html"} {
		var bad map[string]any
		_ = json.Unmarshal(card, &bad)
		bad[key] = uuid.NewString()
		if validateMessageCards(p, []json.RawMessage{raw(bad)}, s, id) == nil {
			t.Fatal("invalid card binding accepted", key)
		}
	}
	legacy := p
	legacy.AuthoringVersion = 13
	if validateGuidanceExample(legacy, &example) == nil || validateMessageCards(legacy, []json.RawMessage{card}, s, id) == nil {
		t.Fatal("old contract gained new effects")
	}
	if validateGuidanceExample(p, &GuidanceExample{Input: strings.Repeat("a", 601), Expected: "x"}) == nil {
		t.Fatal("unbounded guidance")
	}
	p.Document.Messages = []Message{{ID: id, Role: "assistant", Content: "Which formatting matters?", Cards: []json.RawMessage{card}}}
	for _, task := range []conversationTask{taskRoute, taskAuthor, taskExplain, taskSignals} {
		input, err := buildTaskInput(p, task, "prepare_tests", nil)
		if err != nil || strings.Contains(string(raw(input)), example.Expected) {
			t.Fatal("illustration entered model context", task, err)
		}
	}
	for _, version := range []int{11, 12, 13} {
		rollback := p
		rollback.AuthoringVersion = version
		if strings.Contains(string(raw(taskMessages(rollback, taskRoute, "", nil))), example.Expected) {
			t.Fatal("rollback exposed a display-only illustration", version)
		}
	}
	before := completionCommandHash("Which formatting matters?", nil, nil, []AuthoringCompletion{{Cards: []json.RawMessage{card}}})
	example.Expected = "Changed output"
	if before == completionCommandHash("Which formatting matters?", nil, nil, []AuthoringCompletion{{Cards: []json.RawMessage{exampleCard(example, s.Brief.ScopeID, id)}}}) {
		t.Fatal("card change escaped replay hashing")
	}
}

func TestIntegrationVibeGuidanceDoesNotBecomePolicy(t *testing.T) {
	s, v, _ := memoryService(t)
	s.Config.PreciseActions, s.Config.ContextGuidance = true, true
	v, _ = memoryExecute(t, s, v, "My agent converts PDF to Markdown.", func(provider.Request) any {
		route := questionRoute()
		route.Example = &GuidanceExample{Input: "PDF heading: Pricing", Expected: "# Pricing"}
		return route
	})
	message := v.Document.Messages[len(v.Document.Messages)-1]
	if len(message.Cards) != 1 || len(v.Document.Artifacts) != 0 || len(v.Document.Policies) != 0 || len(v.Document.ConversationState.Guidance.Events) != 1 {
		t.Fatal("showing guidance adopted or lost it")
	}
	for _, fact := range v.Document.ConversationState.Brief.Facts {
		if fact.Kind == "rule" {
			t.Fatal("illustration became a rule")
		}
	}
	question := v.Document.ConversationState.PendingQuestion.ID
	v, _ = memoryExecute(t, s, v, "give me vodka", func(req provider.Request) any {
		if strings.Contains(req.Messages[1].Content, "# Pricing") {
			t.Fatal("example leaked into router")
		}
		return reliableRoute{Intent: "chat", Reply: "What matters for your converter?", Memory: &memoryUpdate{}}
	})
	if v.Document.ConversationState.PendingQuestion.ID != question || len(v.Document.Messages[1].Cards) != 1 || len(v.Document.Policies) != 0 {
		t.Fatal("chat lost the question/illustration or added policy")
	}
}

func TestIntegrationVibeExplanationCanAccompanyPreparation(t *testing.T) {
	s, v, _ := memoryService(t)
	s.Config.PreciseActions, s.Config.ContextGuidance = true, true
	rule := "My agent returns the text it receives unchanged."
	stage := 0
	v, op := memoryExecute(t, s, v, rule+" Explain what a test is and prepare exactly one test.", func(req provider.Request) any {
		stage++
		switch stage {
		case 1:
			return reliableRoute{Intent: "prepare_tests", Count: 1, Reply: "I'll prepare one test.", Memory: &memoryUpdate{Facts: []memoryFact{{Kind: "job", Quote: rule}}}, Example: &GuidanceExample{Input: "Hello", Expected: "Hello"}}
		case 2:
			var input taskInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			if strings.Contains(req.Messages[1].Content, "Hello") {
				t.Fatal("illustration became an authoring requirement")
			}
			id := input.CurrentRequest.ID
			return createSuiteCommand{Rules: []PolicyRule{{ID: "echo", Statement: rule, SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: rule, Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Echo", Summary: "Returns the input unchanged.", Scenarios: []TestScenario{{Input: "Good morning", Expected: "Good morning"}}}}
		case 3:
			var input SuiteReviewInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			if strings.Contains(req.Messages[1].Content, "Hello") {
				t.Fatal("illustration contaminated independent review")
			}
			return supportedSuiteReview(input)
		default:
			t.Fatal("guidance added an inference pass")
		}
		return nil
	})
	if stage != 3 || len(v.Document.Artifacts) != 1 || len(v.Document.Messages[len(v.Document.Messages)-1].Cards) != 1 || op.Completion.CaseCount != 1 {
		t.Fatal("help blocked preparation or changed its count")
	}
}
