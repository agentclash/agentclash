package vibe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func interpretationFixture(action any, facts ...factObservation) interpretation {
	if facts == nil {
		facts = []factObservation{}
	}
	return interpretation{Observations: facts, SourceMessageIDs: []string{}, Action: raw(action)}
}

func TestVibeV15FirstJobHasNoPlaceholderCorrections(t *testing.T) {
	job := "Build me a returns agent for Shopify"
	p, o := memoryPlan(t, Document{}, job)
	p.AuthoringVersion = interpretedAuthoringVersion
	unknownID := p.Conversation.State.Brief.Facts[0].ID
	for _, msg := range interpretationMessages(p, nil) {
		if strings.Contains(msg.Content, unknownID) {
			t.Fatal("unknown placeholder leaked to model")
		}
	}
	v := interpretationFixture(askAction{Kind: "ask", Text: "Which returns should qualify?", Purpose: "clarify_rule", Options: []string{}}, factObservation{Kind: "job", Quote: job})
	route, err := decodeInterpretation(raw(v), p)
	if err != nil {
		t.Fatal(err)
	}
	state, err := proposeConversationState(p, route, o)
	if err != nil {
		t.Fatal(err)
	}
	if route.NewAgent || state.Brief.ScopeID != p.Conversation.State.Brief.ScopeID || state.PendingQuestion == nil {
		t.Fatal("first job reset scope or lost question")
	}
	if state.Brief.Facts[len(state.Brief.Facts)-1].Text == nil {
		t.Fatal("job was not retained")
	}
	// Both real failed v14 outputs remain rejected under the legacy contract.
	for _, newAgent := range []bool{true, false} {
		legacy := reliableRoute{Intent: "prepare_tests", Count: 3, Reply: "Here are three starter tests", NewAgent: newAgent, Memory: &memoryUpdate{Facts: []memoryFact{{Kind: "job", Quote: job, SupersedesID: unknownID}}}}
		old := p
		old.AuthoringVersion = guidedAuthoringVersion
		if _, e := proposeConversationState(old, legacy, o); e == nil {
			t.Fatal("legacy replay behavior changed")
		}
	}
}

func TestVibeV15ActionSchemaAndAuthority(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action any
	}{
		{"reply", replyAction{Kind: "reply", Text: "Hi!"}},
		{"ask", askAction{Kind: "ask", Text: "Which returns qualify?", Purpose: "clarify_rule", Options: []string{"Unopened only", "Both"}}},
		{"prepare", prepareAction{Kind: "prepare_tests", Count: 3}},
		{"edit", mutationAction{Kind: "edit_tests"}},
		{"fix", mutationAction{Kind: "suggest_fix"}},
		{"propose", proposeAction{Kind: "propose", Text: "Example: unopened only. Use it?", Suggestions: []factObservation{{Kind: "rule", Quote: "unopened only"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := memoryPlan(t, Document{}, "hello")
			p.AuthoringVersion = interpretedAuthoringVersion
			v := interpretationFixture(tc.action)
			if _, err := decodeInterpretation(raw(v), p); err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			_ = json.Unmarshal(raw(v), &fields)
			fields["scope_id"] = uuid.New().String()
			if _, err := decodeInterpretation(raw(fields), p); err == nil {
				t.Fatal("model can supply internal scope")
			}
		})
	}
	p, o := memoryPlan(t, Document{}, "drin vodka hewhe")
	p.AuthoringVersion = interpretedAuthoringVersion
	v := interpretationFixture(replyAction{Kind: "reply", Text: "Tell me what your agent should do."}, factObservation{Kind: "rule", Quote: p.Submission.Content})
	route, err := decodeInterpretation(raw(v), p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = proposeConversationState(p, route, o); err == nil {
		t.Fatal("chat promoted to requirement")
	}
	v.Observations[0].CorrectionRef = 1
	if _, err = decodeInterpretation(raw(v), p); err == nil {
		t.Fatal("correction of placeholder accepted")
	}
}

func TestVibeV15ShortAnswersBindOnServer(t *testing.T) {
	d, _ := memoryTurn(t, Document{}, "My agent converts PDF to Markdown.", questionRoute())
	for _, text := range []string{"both", "dono, headings bhi preserve karna", "pata nahi"} {
		p, o := memoryPlan(t, d, text)
		p.AuthoringVersion = interpretedAuthoringVersion
		v := interpretationFixture(replyAction{Kind: "reply", Text: "We can work with that."})
		v.Answer = &answerObservation{Quote: text, Unknown: text == "pata nahi"}
		route, err := decodeInterpretation(raw(v), p)
		if err != nil {
			t.Fatal(err)
		}
		state, err := proposeConversationState(p, route, o)
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Answers) != 1 || state.Answers[0].Question.ID != d.ConversationState.PendingQuestion.ID {
			t.Fatal("answer not bound to original question")
		}
		v.Answer.Quote = "invented answer"
		route, err = decodeInterpretation(raw(v), p)
		if err == nil {
			_, err = proposeConversationState(p, route, o)
		}
		if err == nil {
			t.Fatal("unsupported answer accepted")
		}
	}
}

func TestVibeV15RecoveryFaultPolicy(t *testing.T) {
	for _, code := range []string{"usage_unknown", "provider_auth", "provider_request_rejected", "provider_timeout", "operation_stopped", "model_policy_changed", "budget_limit", "test_policy_conflict"} {
		if recoverableAssistantFault(fault(code, "test")) {
			t.Fatalf("unsafe recovery: %s", code)
		}
	}
	if !recoverableAssistantFault(fault("provider_rate_limit", "busy")) {
		t.Fatal("known busy failure not recoverable")
	}
	if interpretedStepAllowed("repair") || interpretedStepAllowed("route:fallback:again") {
		t.Fatal("unbounded step")
	}
}

func TestVibeV15ReadinessDoesNotInventReturnPolicy(t *testing.T) {
	p, _ := memoryPlan(t, Document{}, "Build me a returns agent for Shopify")
	p.AuthoringVersion = interpretedAuthoringVersion
	v := interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: p.Submission.Content})
	route, err := decodeInterpretation(raw(v), p)
	if err != nil || route.Intent != "clarify" || route.Count != 0 || route.Memory.Question == nil {
		t.Fatal("job-only request permitted invented tests", err)
	}
}

func TestVibeV15RepairKeepsOriginalRequestLast(t *testing.T) {
	p, _ := memoryPlan(t, Document{}, "My agent handles returns.")
	p.AuthoringVersion = interpretedAuthoringVersion
	messages := interpretationMessages(p, nil)
	next := stageFeedback(messages, provider.Message{Role: "user", Content: "Internal feedback"})
	if next[len(next)-1].Content != p.Submission.Content || messages[len(messages)-1].Content != p.Submission.Content {
		t.Fatal("repair replaced original request")
	}
}

func TestVibeV15AdmissionFreezesOnlyApprovedAlternative(t *testing.T) {
	for _, primaryID := range []string{"deepseek/deepseek-v4.1-flash", "deepseek/deepseek-v4-pro", "openai/gpt-5.4-mini"} {
		t.Run(primaryID, func(t *testing.T) {
			cfg := testConfig()
			cfg.InterpretedAuthoring, cfg.AssistantFallback = true, true
			base := cfg.Profiles[DefaultModels().Assistant]
			for id, route := range map[string]string{"deepseek/deepseek-v4.1-flash": "coreweave/fp8", "deepseek/deepseek-v4-pro": "baidu/fp8", "openai/gpt-5.4-mini": "openai"} {
				p := base
				p.ID = id
				p.Route = route
				p.StructuredOutputs, p.OmitTemperature, p.DisableReasoning = true, true, true
				cfg.Profiles[id] = p
			}
			p, _ := memoryPlan(t, Document{}, "hello")
			p.AuthoringVersion = guidedAuthoringVersion
			primary := cfg.Profiles[primaryID]
			p.Conversation.Profile = &primary
			if err := prepareInterpretedPlan(&p, cfg, primary); err != nil {
				t.Fatal(err)
			}
			if !p.interpreted() || p.AssistantRecovery == nil || p.AssistantRecovery.Profile.ID == primaryID || p.AssistantRecovery.Profile.ID == "deepseek/deepseek-v4-pro" || p.Calls != 9 {
				t.Fatal("wrong alternative graph")
			}
			cost, _ := primary.BoundCost(primary.inputLimit(p.limits()), p.limits().OutputTokens)
			p.MaxCost = 8*cost + p.AssistantRecovery.MaxCost
			if err := validateInterpretedAllowance(p, cfg); err != nil {
				t.Fatal(err)
			}
			p.MaxCost--
			if validateInterpretedAllowance(p, cfg) == nil {
				t.Fatal("underfunded graph accepted")
			}
			p.MaxCost++
			p.Calls++
			if validateInterpretedAllowance(p, cfg) == nil {
				t.Fatal("extra model call accepted")
			}
		})
	}
}
