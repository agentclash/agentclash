package vibe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestQuickCheckMessageSourcePreservesPreludeAndReplies(t *testing.T) {
	prefix := "Check my return bot.\r\nPolicy: unopened items within 30 days.\r\n\r\n"
	chat := "Customer: Bought 10 days ago, unopened.\r\nAgent:  Exact answer.  \r\n"
	source := prefix + chat
	e, err := ParseMessageEvidence(source, LimitsFor(true))
	if err != nil || e == nil {
		t.Fatalf("chat was not recognized: %v", err)
	}
	if e.Raw != source || e.Context != prefix || e.ValidateReady() != nil {
		t.Fatal("prelude became a speaker or immutable source changed")
	}
	if len(e.Conversations) != 1 || len(e.Conversations[0].Messages) != 2 || e.Conversations[0].Messages[1].Content != " Exact answer.  \r\n" {
		t.Fatal("the recorded reply was trimmed, rewritten, or lost")
	}
	p := Plan{AuthoringVersion: 7, Evidence: e, InlineEvidence: e, Submission: Submission{Content: source, QuickCheck: true}}
	data := evaluationContext(p)
	if data["source_context"] != prefix {
		t.Fatal("policy/reference was not retained for deriving expectations")
	}
	for _, input := range []string{"I made a trip planner.", "Why do you need instructions?", "Our refund policy is 30 days.", "You are a helpful support assistant.", "https://example.com/my-app", "Agent: An answer without its input.", `{"role":"system","content":"You are a helpful support assistant."}`} {
		candidate, err := ParseMessageEvidence(input, LimitsFor(true))
		if err != nil || candidate != nil {
			t.Fatalf("non-transcript became recorded evidence: %q, %v", input, err)
		}
	}
	for _, input := range []string{strings.Repeat(chatFixture+"\n---\n", 3) + chatFixture, `{"messages":[{"role":"user","content":"Hi"},{"role":"assistant","content":"one","content":"two"}]}`} {
		if _, err := ParseMessageEvidence(input, LimitsFor(true)); err == nil {
			t.Fatal("oversized or ambiguous source bypassed evidence validation")
		}
	}
}

func TestQuickCheckAuthorCannotCheckMissingOrHypotheticalEvidence(t *testing.T) {
	e, _ := ParseMessageEvidence(chatFixture, LimitsFor(true))
	ready, wait := true, false
	valid := evaluationAuthorReply{Reply: "I’ll check whether it uses the details already supplied.", SourceKind: "recorded_chat", CheckNow: &ready, Expectations: []string{"Use details already provided before asking another question."}}
	plan := Plan{AuthoringVersion: 7, Evidence: e, InlineEvidence: e, Submission: Submission{QuickCheck: true}}
	if err := validateEvaluationReply(plan, valid); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"no_source", "hypothetical", "instructions_after_real_chat", "help_with_rules", "missing_decision", "suggest_change"} {
		t.Run(name, func(t *testing.T) {
			p, reply := plan, valid
			switch name {
			case "no_source":
				p.Evidence = nil
			case "hypothetical":
				reply.SourceKind = "instructions"
			case "instructions_after_real_chat":
				p.InlineEvidence = nil
				reply.SourceKind = "instructions"
			case "help_with_rules":
				reply.CheckNow = &wait
			case "missing_decision":
				reply.CheckNow = nil
			case "suggest_change":
				p.Submission.Purpose = "suggest_change"
			}
			if validateEvaluationReply(p, reply) == nil {
				t.Fatal("unsupported quick check was accepted")
			}
		})
	}
	missing := evaluationAuthorReply{Reply: "What is your refund window?", SourceKind: "recorded_chat", CheckNow: &wait}
	if err := validateEvaluationReply(plan, missing); err != nil {
		t.Fatal("a targeted missing-policy question was rejected", err)
	}
	v := Session{Anonymous: true, Document: Document{Artifacts: []Artifact{{ID: uuid.New(), QuickCheck: true, Kind: "conversation_evaluation", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: []Expectation{{ID: "rule-1", Statement: "Use the provided date."}}}}}}}
	if err := EditConversationEvaluation(&v, v.Document.Artifacts[0].ID, []Expectation{{ID: "rule-1", Statement: "Use the corrected policy."}}); err != nil || v.Document.Artifacts[1].QuickCheck {
		t.Fatal("an expectation edit inherited permission for an automatic check", err)
	}
}

func TestIntegrationQuickCheckUsesExactEvidenceAndExistingEvaluatorLifecycle(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	ctx, cfg := context.Background(), freeConfig()
	svc := &Service{Store: s, Config: cfg, Gate: testGate(t)}
	source := "Check the bot's memory. Policy: unopened items within 30 days.\n" + chatFixture
	sub := Submission{ClientID: uuid.New(), Kind: "message", Content: source, EvaluationFirst: true, QuickCheck: true, Models: cfg.DefaultModels()}
	o, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	var p Plan
	if err = json.Unmarshal(o.Input, &p); err != nil || p.InlineEvidence == nil || p.InlineEvidence.Raw != source || p.Evidence.ValidateReady() != nil {
		t.Fatal("admission did not preserve the entire candidate source", err)
	}
	current, _ := s.GetSession(ctx, v.Actor, v.ID)
	if len(current.Document.EvidenceSets) != 0 || current.Document.Messages[0].Content != source {
		t.Fatal("candidate was prematurely represented as a recorded chat")
	}
	calls := 0
	fake := callFunc(func(_ context.Context, request provider.Request) (provider.Response, error) {
		calls++
		output := `{"reply":"I'll check whether it uses details already given.","title":"Remember details","source_kind":"recorded_chat","check_now":true,"expectations":["Use the purchase age already supplied before asking when the item was bought."]}`
		if calls == 1 {
			if !strings.Contains(request.Messages[1].Content, "within 30 days") || !strings.Contains(request.Messages[1].Content, "When did you buy it?") {
				t.Fatal("author lost the policy or actual reply")
			}
		} else if calls == 2 {
			if !strings.Contains(request.Messages[0].Content, "untrusted EVIDENCE") || !strings.Contains(request.Messages[1].Content, "When did you buy it?") {
				t.Fatal("the existing conversation evaluator was bypassed")
			}
			output = `{"checks":[{"key":"rule-1","verdict":"FAIL","evidence":"The customer already supplied the purchase age.","message_ids":["c1-m1","c1-m4"]}]}`
		} else {
			t.Fatal("an unexpected target or duplicate call ran")
		}
		zero := json.Number("0")
		return provider.Response{OutputText: output, Usage: provider.Usage{InputTokens: 100, OutputTokens: 100, CostUSD: &zero}}, nil
	})
	runner := Runner{Service: svc, Gateway: &Gateway{Store: s, Config: cfg, Gate: svc.Gate, Client: fake}}
	if err = runner.Execute(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	if len(v.Document.Artifacts) != 1 || !v.Document.Artifacts[0].QuickCheck || v.Document.Artifacts[0].Accepted || v.Document.Artifacts[0].SourceMessageID != sub.ClientID || len(v.Document.EvidenceSets) != 1 || v.Document.EvidenceSets[0].Raw != source {
		t.Fatal("missing durable quick-check signal or immutable evidence")
	}
	a := v.Document.Artifacts[0]
	again, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil || again.ID != o.ID {
		t.Fatal("retry of admitted message created another source", err)
	}
	changed := sub
	changed.QuickCheck = false
	if _, err = svc.Prepare(ctx, v.Actor, v.ID, changed); err == nil {
		t.Fatal("same message ID accepted different quick-check intent")
	}
	checkSub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "check", ArtifactID: &a.ID, ApproveArtifact: true, EvaluationFirst: true, Models: cfg.DefaultModels()}
	check, err := svc.Prepare(ctx, v.Actor, v.ID, checkSub)
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Execute(ctx, check.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, check.ID, nil); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	checked, err := s.GetCase(ctx, v.Actor, check.ID, "chat-1")
	if err != nil || checked.Verdict != Fail || checked.Messages[3].Content != "When did you buy it?" || len(v.Document.EvidenceSets) != 1 || calls != 2 {
		t.Fatal("quick check did not retain exact evidence through measured results", err)
	}
}

func TestIntegrationQuickCheckAnswersQuestionsAndRetainsMissingPolicy(t *testing.T) {
	for _, scenario := range []struct {
		name, input, output string
		wantEvidence        bool
	}{
		{"description", "I made an AI trip planner.", `{"reply":"Paste a trip request and the plan it gave you.","title":"","source_kind":"other","check_now":false,"expectations":null}`, false},
		{"help", "Why do you need instructions?", `{"reply":"You can paste what you asked and what your app answered; instructions are not needed for that.","title":"","source_kind":"other","check_now":false,"expectations":null}`, false},
		{"prompt_example", "Here is my system prompt with examples. You are a support assistant.\nCustomer: Example question\nAgent: Example response", `{"reply":"These are instructions and examples. Paste an actual request and reply from your app to check its behavior.","title":"","source_kind":"instructions","check_now":false,"expectations":null}`, false},
		{"missing_policy", "Was this refund decision correct?\nCustomer: Bought it 10 days ago.\nAgent: You are not eligible.", `{"reply":"What is your refund window?","title":"","source_kind":"recorded_chat","check_now":false,"expectations":null}`, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := integrationStore(t)
			v := anonSession(t, s)
			ctx, cfg := context.Background(), freeConfig()
			svc := &Service{Store: s, Config: cfg, Gate: testGate(t)}
			sub := Submission{ClientID: uuid.New(), Kind: "message", Content: scenario.input, EvaluationFirst: true, QuickCheck: true, Models: cfg.DefaultModels()}
			o, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			fake := callFunc(func(_ context.Context, request provider.Request) (provider.Response, error) {
				calls++
				if !strings.Contains(request.Messages[1].Content, strings.Split(scenario.input, "\n")[0]) {
					t.Fatal("contextual author never received the user's actual request")
				}
				zero := json.Number("0")
				return provider.Response{OutputText: scenario.output, Usage: provider.Usage{InputTokens: 100, OutputTokens: 100, CostUSD: &zero}}, nil
			})
			runner := Runner{Service: svc, Gateway: &Gateway{Store: s, Config: cfg, Gate: svc.Gate, Client: fake}}
			if err = runner.Execute(ctx, o.ID); err != nil {
				t.Fatal(err)
			}
			if err = s.Finish(ctx, o.ID, nil); err != nil {
				t.Fatal(err)
			}
			v, _ = s.GetSession(ctx, v.Actor, v.ID)
			if calls != 1 || len(v.Document.Artifacts) != 0 || (len(v.Document.EvidenceSets) == 1) != scenario.wantEvidence {
				t.Fatal("question created a check or real source was lost")
			}
			var expected evaluationAuthorReply
			_ = json.Unmarshal([]byte(scenario.output), &expected)
			if v.Document.Messages[len(v.Document.Messages)-1].Content != expected.Reply {
				t.Fatal("contextual answer was replaced by a canned source question")
			}
			if scenario.wantEvidence {
				followup, err := svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "The window is 30 days.", EvaluationFirst: true, QuickCheck: true, Models: cfg.DefaultModels()})
				if err != nil {
					t.Fatal(err)
				}
				var plan Plan
				_ = json.Unmarshal(followup.Input, &plan)
				if plan.Evidence == nil || plan.Evidence.Raw != scenario.input || plan.InlineEvidence != nil {
					t.Fatal("answering the missing rule asked the user to paste the chat again")
				}
				_ = s.Finish(ctx, followup.ID, &Fault{Code: "test_cleanup", Message: "Fixture finished."})
			}
		})
	}
}
