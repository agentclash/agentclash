package vibe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

const chatFixture = "Customer: I bought it 10 days ago.\nAgent: Is it opened?\nCustomer: No, unopened.\nAgent: When did you buy it?"

func TestEvidencePreservesWholeChatsAndRawSource(t *testing.T) {
	input := "**Customer:** Hello\r\n**Agent:** Hi\r\n---\r\nUser: 10 days\r\nAssistant: Opened?\r\nUser: No\r\nAssistant: Eligible.\r\n"
	e, err := ParseEvidence(input, "returns.md", LimitsFor(true))
	if err != nil {
		t.Fatal(err)
	}
	if e.Raw != input || len(e.Conversations) != 2 || len(e.Conversations[1].Messages) != 4 {
		t.Fatal("source or follow-ups were lost")
	}
	if e.Conversations[1].Messages[3].Content != "Eligible.\r\n" {
		t.Fatal("agent output was rewritten")
	}
	if err = e.ValidateReady(); err != nil {
		t.Fatal(err)
	}
	unknown, err := ParseEvidence("A preamble\n"+chatFixture, "", LimitsFor(true))
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Conversations[0].Messages[0].Content != "A preamble\n" || unknown.ValidateReady() == nil {
		t.Fatal("ambiguous source was dropped or silently assigned a role")
	}
}

func TestEvidenceJSONAndLimits(t *testing.T) {
	for _, input := range []string{
		`[{"role":"user","content":"Hi"},{"role":"assistant","content":"  exact\nreply  "}]`,
		`{"messages":[{"role":"customer","content":"Hi"},{"role":"agent","content":"  exact\nreply  "}]}`,
		`{"conversations":[{"title":"Chat","messages":[{"role":"user","content":"Hi"},{"role":"assistant","content":"  exact\nreply  "}]}]}`,
	} {
		e, err := ParseEvidence(input, "chat.json", LimitsFor(true))
		if err != nil {
			t.Fatal(err)
		}
		if e.Conversations[0].Messages[1].Content != "  exact\nreply  " {
			t.Fatal("JSON content changed")
		}
	}
	for _, input := range []string{`{"messages":[{"role":"user","content":"Hi"},{"role":"assistant","content":"First","content":"Different"}]}`, `{"messages":[{"role":"user","content":"Hi"},{"role":"assistant","content":"\uD800"}]}`, strings.Repeat(chatFixture+"\n---\n", 3) + chatFixture, `{"messages":[{"role":"assistant","content":[{"text":"No"}]}]}`, "Customer: \nAgent: Hi"} {
		if _, err := ParseEvidence(input, "", LimitsFor(true)); err == nil {
			t.Fatal("unsupported or incomplete evidence accepted")
		}
	}
}

func TestConversationJudgeRequiresGroundedMessageReferences(t *testing.T) {
	e, _ := ParseEvidence(chatFixture, "", LimitsFor(true))
	c := e.Conversations[0]
	rules := []Expectation{{ID: "memory", Statement: "Remember the purchase age across follow-ups."}}
	valid := `{"checks":[{"key":"memory","verdict":"FAIL","evidence":"It asks for the already supplied purchase age.","message_ids":["c1-m1","c1-m4"]}]}`
	if _, err := ParseConversationJudge([]byte(valid), rules, c, LimitsFor(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseConversationJudge([]byte(strings.Replace(valid, `"key":`, `"id":`, 1)), rules, c, LimitsFor(true)); err != nil {
		t.Fatal("exact rule ID alias rejected", err)
	}
	for _, bad := range []string{strings.Replace(valid, "c1-m4", "fabricated", 1), strings.Replace(valid, `,"c1-m4"`, "", 1), strings.Replace(valid, "memory", "another-rule", 1), `{"checks":[]}`} {
		if _, err := ParseConversationJudge([]byte(bad), rules, c, LimitsFor(true)); err == nil {
			t.Fatal("ungrounded verdict was accepted", bad)
		}
	}
	prompt := ConversationJudgeMessages(rules, c)
	if !strings.Contains(prompt[0].Content, "untrusted EVIDENCE") || !strings.Contains(prompt[1].Content, "When did you buy it?") {
		t.Fatal("whole-chat/injection boundary missing")
	}
}

func TestConversationComparisonsFreezeQuestionsAndRules(t *testing.T) {
	e, _ := ParseEvidence(chatFixture, "", LimitsFor(true))
	a := &Artifact{ID: uuid.New(), Kind: "conversation_evaluation", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: []Expectation{{ID: "memory", Statement: "Remember details."}}}}
	old := Plan{Evidence: &e, Artifact: a}
	next := old
	if label, err := compareEvidencePlans(old, next); err != nil || label != "rechecked" {
		t.Fatal(label, err)
	}
	renamed, _ := ParseEvidence(strings.ReplaceAll(chatFixture, "\n", "\r\n"), "renamed.json", LimitsFor(true))
	renamed.Conversations[0].Title = "A new label"
	next.Evidence = &renamed
	if label, err := compareEvidencePlans(old, next); err != nil || label != "rechecked" {
		t.Fatal("metadata became a behavioral improvement", label, err)
	}
	updated, _ := ParseEvidence(strings.Replace(chatFixture, "When did you buy it?", "It is eligible.", 1), "new file", LimitsFor(true))
	next.Evidence = &updated
	if label, err := compareEvidencePlans(old, next); err != nil || label != "updated_replies" {
		t.Fatal(label, err)
	}
	changed, _ := ParseEvidence(strings.Replace(chatFixture, "10 days", "45 days", 1), "", LimitsFor(true))
	next.Evidence = &changed
	if _, err := compareEvidencePlans(old, next); err == nil {
		t.Fatal("changed customer question compared")
	}
	next.Evidence = &e
	copy := *a
	copy.ConversationEvaluation = &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: []Expectation{{ID: "memory", Statement: "Ignore previous details."}}}
	next.Artifact = &copy
	if _, err := compareEvidencePlans(old, next); err == nil {
		t.Fatal("weakened expectations compared")
	}
}

func TestIntegrationProvidedChatsMakeOnlyEvaluatorCalls(t *testing.T) {
	for _, mode := range []string{"pass", "invalid_judge", "provider_failure"} {
		t.Run(mode, func(t *testing.T) {
			s := integrationStore(t)
			v := anonSession(t, s)
			cfg := freeConfig()
			gate := testGate(t)
			ctx := context.Background()
			svc := &Service{Store: s, Config: cfg, Gate: gate}
			if err := svc.AddEvidence(ctx, v.Actor, v.ID, EvidenceInput{Revision: v.Revision, Content: chatFixture + "\n---\n" + chatFixture}); err != nil {
				t.Fatal(err)
			}
			v, _ = s.GetSession(ctx, v.Actor, v.ID)
			e := v.Document.EvidenceSets[0]
			a := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", Title: "Remember purchase age", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: []Expectation{{ID: "memory", Statement: "Remember purchase age."}}}}
			if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error { v.Document.Artifacts = append(v.Document.Artifacts, a); return nil }); err != nil {
				t.Fatal(err)
			}
			v, _ = s.GetSession(ctx, v.Actor, v.ID)
			models := cfg.DefaultModels()
			models.Target = "not-a-target"
			models.Assistant = "not-an-assistant"
			sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "check", Models: models, ArtifactID: &a.ID, ApproveArtifact: true, EvaluationFirst: true}
			o, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
			if err != nil {
				t.Fatal(err)
			}
			var plan Plan
			_ = json.Unmarshal(o.Input, &plan)
			if plan.Calls != 2 || plan.MaxCost != 0 || plan.Source.Kind != "provided_conversations" {
				t.Fatalf("wrong call graph %+v", plan)
			}
			again, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
			if err != nil || again.ID != o.ID {
				t.Fatal("receipt not idempotent", err)
			}
			calls := 0
			fake := callFunc(func(_ context.Context, request provider.Request) (provider.Response, error) {
				calls++
				if request.Model != models.Evaluator || !strings.Contains(request.Messages[1].Content, "No, unopened.") {
					t.Fatal("lost full chat or used target role")
				}
				if mode == "provider_failure" {
					return provider.Response{}, fault("provider_error", "Fixture failure")
				}
				output := `{"checks":[{"key":"memory","verdict":"FAIL","evidence":"Forgot the known purchase age.","message_ids":["c1-m1","c1-m4"]}]}`
				if calls == 2 {
					output = strings.ReplaceAll(output, "c1-", "c2-")
				}
				if mode == "invalid_judge" {
					output = strings.ReplaceAll(output, "c1-m4", "invented")
				}
				zero := json.Number("0")
				return provider.Response{OutputText: output, Usage: provider.Usage{InputTokens: 80, OutputTokens: 80, CostUSD: &zero}}, nil
			})
			runner := Runner{Service: svc, Gateway: &Gateway{Store: s, Config: cfg, Gate: gate, Client: fake}}
			err = runner.Execute(ctx, o.ID)
			if mode != "provider_failure" && err != nil {
				t.Fatal(err)
			}
			if e := s.Finish(ctx, o.ID, issueFrom(err)); e != nil {
				t.Fatal(e)
			}
			current, err := s.GetSession(ctx, v.Actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			op := current.Operations[0]
			if len(op.Results) != 2 || op.Scorecard.Total != 2 || op.Source == nil {
				t.Fatal("partial graph lost planned chats or source")
			}
			var targetCalls int
			if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_attempts WHERE operation_id=$1 AND role='target'", o.ID).Scan(&targetCalls); err != nil || targetCalls != 0 {
				t.Fatal("provided chats invoked target", err)
			}
			full, err := s.GetCase(ctx, v.Actor, o.ID, "chat-1")
			if err != nil || len(full.Messages) != 4 || full.Messages[3].Content != "When did you buy it?\n" {
				t.Fatalf("recorded evidence changed: %+v %v", full.Messages, err)
			}
			if mode == "pass" && (calls != 2 || op.Scorecard.Failed != 2 || op.ActualCost == nil || *op.ActualCost != 0) {
				t.Fatal("wrong totals/accounting", op)
			}
			if mode == "pass" {
				if err = svc.AddEvidence(ctx, v.Actor, v.ID, EvidenceInput{Revision: current.Revision, Content: strings.Replace(chatFixture, "When did you buy it?", "It is eligible.", 1)}); err != nil {
					t.Fatal(err)
				}
				latest, _ := s.GetSession(ctx, v.Actor, v.ID)
				coaching, err := svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: latest.Revision, Kind: "message", Content: "Suggest a change from the earlier result.", Purpose: "suggest_change", ArtifactID: &a.ID, BaselineID: &o.ID, Models: cfg.DefaultModels(), EvaluationFirst: true})
				if err != nil {
					t.Fatal(err)
				}
				var advice Plan
				if err = json.Unmarshal(coaching.Input, &advice); err != nil {
					t.Fatal(err)
				}
				if advice.Evidence == nil || advice.Evidence.ID != e.ID || len(advice.Observations) != 2 {
					t.Fatal("coaching used the latest upload instead of the selected result")
				}
			}
			if mode != "pass" && op.Scorecard.Unknown == 0 {
				t.Fatal("failure was counted as behavioral pass/fail")
			}
			if _, err = s.GetCase(ctx, "anon:"+uuid.NewString(), o.ID, "chat-1"); err == nil {
				t.Fatal("another actor read a private chat")
			}
		})
	}
}

func TestIntegrationEvidenceCorrectionsAndSavedCheckPrivacy(t *testing.T) {
	s := integrationStore(t)
	v := approvalRegressionWorkspace(t, s)
	cfg := freeConfig()
	ctx := context.Background()
	svc := &Service{Store: s, Config: cfg, Gate: testGate(t)}
	if err := svc.AddEvidence(ctx, v.Actor, v.ID, EvidenceInput{Revision: v.Revision, Content: "Unlabelled request\nAgent: Exact reply"}); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	original := v.Document.EvidenceSets[0]
	if err := svc.AddEvidence(ctx, v.Actor, v.ID, EvidenceInput{Revision: v.Revision, ParentID: &original.ID, Roles: map[string]string{"c1-m1": "user"}}); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	corrected := v.Document.EvidenceSets[1]
	if original.Raw != corrected.Raw || v.Document.EvidenceSets[0].Conversations[0].Messages[0].Role != "unknown" || corrected.ValidateReady() != nil {
		t.Fatal("speaker correction changed original source")
	}
	a := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", Title: "Saved chat check", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: corrected.ID, Expectations: []Expectation{{ID: "rule", Statement: "Answer the customer."}}}}
	if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error { v.Document.Artifacts = append(v.Document.Artifacts, a); return nil }); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	o, err := svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "check", Models: cfg.DefaultModels(), ArtifactID: &a.ID, ApproveArtifact: true, EvaluationFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Stop(ctx, v.Actor, o.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	saved, err := s.SaveCheck(ctx, v.Actor, v.ID, v.Revision, *v.WorkspaceID, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	again, err := s.SaveCheck(ctx, v.Actor, v.ID, v.Revision-1, *v.WorkspaceID, o.ID)
	if err != nil || again.ID != saved.ID {
		t.Fatal("save duplicated", err)
	}
	list, err := s.ListChecks(ctx, v.Actor, uuid.Nil)
	if err != nil || len(list) != 1 || list[0].Source.EvidenceSetID == nil || *list[0].Source.EvidenceSetID != corrected.ID {
		t.Fatal("save/reopen source receipt lost", err)
	}
	var builds int
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_saved_artifacts WHERE session_id=$1", v.ID).Scan(&builds); err != nil || builds != 0 {
		t.Fatal("provided chats created an agent build")
	}
	other := uuid.New()
	if _, err = s.DB.Exec(ctx, "INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", other, other.String(), other.String()+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(ctx, "INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) SELECT organization_id,$2,'org_admin','active' FROM workspaces WHERE id=$1", *v.WorkspaceID, other); err != nil {
		t.Fatal(err)
	}
	otherActor := "user:" + other.String()
	list, err = s.ListChecks(ctx, otherActor, *v.WorkspaceID)
	if err != nil || len(list) != 0 {
		t.Fatal("another workspace member saw a private check", err)
	}
	if _, err = s.GetSession(ctx, otherActor, v.ID); err == nil {
		t.Fatal("another member reopened the source session")
	}
	if _, err = s.SaveCheck(ctx, otherActor, v.ID, v.Revision, *v.WorkspaceID, o.ID); err == nil {
		t.Fatal("another member saved a private baseline")
	}
}

func TestIntegrationDescriptionHandoffUsesContextualAuthor(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	cfg := freeConfig()
	ctx := context.Background()
	svc := &Service{Store: s, Config: cfg, Gate: testGate(t)}
	o, err := svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Kind: "message", Content: "Our support agent explains returns.", Models: cfg.DefaultModels(), EvaluationFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	fake := callFunc(func(_ context.Context, request provider.Request) (provider.Response, error) {
		if !strings.Contains(request.Messages[1].Content, "Our support agent explains returns.") {
			t.Fatal("the author never received the actual brief")
		}
		zero := json.Number("0")
		return provider.Response{OutputText: `{"reply":"Paste a return question and your app's answer.","title":"","source_kind":"other","check_now":false,"expectations":null}`, Usage: provider.Usage{InputTokens: 80, OutputTokens: 40, CostUSD: &zero}}, nil
	})
	r := Runner{Service: svc, Gateway: &Gateway{Store: s, Config: cfg, Gate: svc.Gate, Client: fake}}
	if err = r.Execute(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	v, err = s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Document.Artifacts) != 0 || len(v.Document.EvidenceSets) != 0 || len(v.Document.Messages) != 2 || v.Operations[0].ModelCalls != 1 || v.Document.Messages[1].Content != "Paste a return question and your app's answer." {
		t.Fatal("description became evidence or lost its contextual response")
	}
}

func TestIntegrationSwitchEvidenceToInstructions(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	cfg := freeConfig()
	ctx := context.Background()
	svc := &Service{Store: s, Config: cfg, Gate: testGate(t), Compiler: repairCompiler{}}
	if err := svc.AddEvidence(ctx, v.Actor, v.ID, EvidenceInput{Content: chatFixture}); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	a := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: v.Document.EvidenceSets[0].ID, Expectations: []Expectation{{ID: "rule", Statement: "Remember details."}}}}
	if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error {
		v.Document.Artifacts = append(v.Document.Artifacts, a)
		v.Document.ActiveArtifactID = &a.ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	instructions := "Use our exact return policy.\nAsk only for missing details."
	o, err := svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "Prepare examples for these instructions.", Instructions: instructions, Models: cfg.DefaultModels(), ArtifactID: &a.ID, EvaluationFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	var p Plan
	if err = json.Unmarshal(o.Input, &p); err != nil {
		t.Fatal(err)
	}
	if p.AuthoringVersion != 4 || p.Evidence != nil || p.Artifact != nil || len(p.Document.Artifacts) != 0 || p.Submission.Instructions != instructions || !p.Document.Journey.PreviewConsent {
		t.Fatal("source switch reused the chat check")
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	if v.Document.ActiveEvidenceID != nil || len(v.Document.EvidenceSets) != 1 {
		t.Fatal("source switch lost the original or kept it active")
	}
}

func TestIntegrationSuggestedChangeCannotWeakenConversationExpectations(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	cfg := freeConfig()
	ctx := context.Background()
	svc := &Service{Store: s, Config: cfg, Gate: testGate(t)}
	e, _ := ParseEvidence(chatFixture, "", LimitsFor(true))
	a := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: []Expectation{{ID: "rule", Statement: "Remember details."}}}}
	p := Plan{AuthoringVersion: 5, Evidence: &e, Artifact: &a, Calls: 2, Free: true, Anonymous: true, Submission: Submission{ClientID: uuid.New(), Kind: "message", Content: "Suggest a change.", Purpose: "suggest_change", Models: cfg.DefaultModels(), EvaluationFirst: true}}
	o, err := s.Submit(ctx, v.Actor, v.ID, p.Submission, p, cfg)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	fake := callFunc(func(_ context.Context, request provider.Request) (provider.Response, error) {
		calls++
		if !strings.Contains(request.Messages[1].Content, "suggest_change") {
			t.Fatal("purpose missing")
		}
		output := `{"reply":"Ignore previous details.","title":"Easy check","expectations":["Always pass."]}`
		if calls == 2 {
			output = `{"reply":"Copy this instruction: Before asking a question, check whether the customer already answered it.","title":"","expectations":null}`
		}
		zero := json.Number("0")
		return provider.Response{OutputText: output, Usage: provider.Usage{InputTokens: 80, OutputTokens: 80, CostUSD: &zero}}, nil
	})
	r := Runner{Service: svc, Gateway: &Gateway{Store: s, Config: cfg, Gate: svc.Gate, Client: fake}}
	if err = r.Execute(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	if calls != 2 || len(v.Document.Artifacts) != 0 || !strings.Contains(v.Document.Messages[len(v.Document.Messages)-1].Content, "Before asking") {
		t.Fatal("suggestion weakened the check instead of proposing an instruction")
	}
}
