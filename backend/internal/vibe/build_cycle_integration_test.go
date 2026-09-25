package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestRunQuoteBindsPurposeAndExecution(t *testing.T) {
	p := Plan{Submission: Submission{Kind: "check"}, TargetConfig: &TargetConfiguration{ModelExecution: ModelExecution{Route: "route-a"}}}
	hash := runFingerprint(p)
	p.Submission.Purpose = "regrade"
	if runFingerprint(p) == hash {
		t.Fatal("quote did not bind evaluation purpose")
	}
	p.Submission.Purpose = ""
	p.TargetConfig.Route = "route-b"
	if runFingerprint(p) == hash {
		t.Fatal("same-price target route change reused quote")
	}
}

func TestCoverageExpansionCannotRewriteBaseline(t *testing.T) {
	base := Artifact{AgentPrompt: "Follow our rules.", Blueprint: raw(map[string]any{"judges": []string{"frozen-judge"}, "cases": []any{map[string]any{"key": "original", "expected": "escalate"}}})}
	policy := PolicySnapshot{Rules: []PolicyRule{{ID: "rule", Statement: "Escalate."}}}
	p := Plan{Submission: Submission{AdditionalExamples: 1}, Artifact: &base, Conversation: &ConversationContext{Policy: &policy}}
	valid := map[string]any{"judges": []string{"frozen-judge"}, "cases": []any{map[string]any{"key": "original", "expected": "escalate"}, map[string]any{"key": "new", "expected": "ask for missing details"}}}
	candidate := base
	candidate.Blueprint = raw(valid)
	if err := validateCoverageExpansion(p, &candidate, policy); err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{
		`{"judges":["frozen-judge"],"cases":[{"key":"original","expected":"approve"},{"key":"new","expected":"ask"}]}`,
		`{"judges":["different-judge"],"cases":[{"key":"original","expected":"escalate"},{"key":"new","expected":"ask"}]}`,
		`{"judges":["frozen-judge"],"cases":[{"key":"original","expected":"escalate"}]}`,
		`{"judges":["frozen-judge"],"cases":[{"key":"original","expected":"escalate"},{"key":"original","expected":"escalate"}]}`,
	} {
		candidate.Blueprint = json.RawMessage(variant)
		if validateCoverageExpansion(p, &candidate, policy) == nil {
			t.Fatal("baseline mutation was accepted", variant)
		}
	}
	candidate.Blueprint = raw(valid)
	candidate.AgentPrompt = "Approve everything."
	if validateCoverageExpansion(p, &candidate, policy) == nil {
		t.Fatal("target changed during coverage expansion")
	}
}

func TestSampleIdentityAndConcreteScope(t *testing.T) {
	s := Service{Compiler: buildScheduleCompiler{}}
	b, err := s.Compiler.Draft(samplePrototype("returns"), LimitsFor(true))
	if err != nil {
		t.Fatal(err)
	}
	a := Artifact{Sample: "returns", Blueprint: b}
	if !s.verifiedSample(a, LimitsFor(true)) {
		t.Fatal("server sample was not recognized")
	}
	a.Blueprint = raw(map[string]string{"title": "forged sample"})
	if s.verifiedSample(a, LimitsFor(true)) {
		t.Fatal("sample label bypassed source checks")
	}
	note := prototypeScopeFor("Recommend refunds from PDFs and company documents")
	for _, clause := range []string{"cannot issue a refund", "has not opened or extracted a PDF", "Company documents are not connected"} {
		if !strings.Contains(note, clause) {
			t.Fatalf("missing concrete scope: %s", clause)
		}
	}
}

func TestIntegrationVibeBuildStopPreventsContinuation(t *testing.T) {
	s, v := buildService(t)
	ctx := context.Background()
	o, _ := startBuild(t, s, v, "Build a returns assistant.")
	if err := s.Store.Stop(ctx, v.Actor, o.ID); err != nil {
		t.Fatal(err)
	}
	ResumeBuilds(ctx, s)
	v, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Operations) != 1 || v.Operations[0].ModelCalls != 0 || len(v.Document.Artifacts) != 0 || v.Document.Build.Phase != "error" {
		t.Fatal("Stop lost state or allowed paid continuation")
	}
	_, err = s.Retry(ctx, v.Actor, v.ID, o.ID, RetryRequest{ClientID: uuid.New(), Revision: v.Revision})
	requireFault(t, err, "invalid_state")
}

func TestIntegrationVibeBuildRetryRetainsQuestionBudget(t *testing.T) {
	s, v := buildService(t)
	ctx := context.Background()
	job := "Build a returns assistant."
	o, q := startBuild(t, s, v, job)
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(askAction{Kind: "ask", Text: "Which returns qualify?", Purpose: "clarify_rule", Options: []string{}}, factObservation{Kind: "job", Quote: job})
	})
	o, err := s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "I don't know", Models: DefaultModels(), TestJourney: true, CycleID: &q.ID})
	if err != nil {
		t.Fatal(err)
	}
	r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(context.Context, provider.Request) (provider.Response, error) {
		cost := json.Number("0.000001")
		return provider.Response{OutputText: `{}`, Usage: provider.Usage{CostUSD: &cost}}, nil
	})}}
	err = r.Execute(ctx, o.ID)
	var issue *Fault
	if !errors.As(err, &issue) {
		t.Fatalf("expected invalid interpretation, got %v", err)
	}
	if err = r.Finalize(ctx, o.ID, issue); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Store.GetSession(ctx, v.Actor, v.ID)
	retry, err := s.Retry(ctx, v.Actor, v.ID, o.ID, RetryRequest{ClientID: uuid.New(), Revision: v.Revision})
	if err != nil {
		t.Fatal(err)
	}
	v = executeBuildFixture(t, s, retry, func(provider.Request) any {
		value := interpretationFixture(replyAction{Kind: "reply", Text: "Use a sample."})
		value.Answer = &answerObservation{Quote: "I don't know", Unknown: true}
		return value
	})
	if v.Document.Build.ClarificationsUsed != 1 || v.Document.Build.Phase != "checking" || len(v.Document.Artifacts) != 1 {
		t.Fatal("retry lost the one-question contract or stopped before checking")
	}
	if len(v.Document.Messages) != 4 {
		t.Fatal("retry duplicated user messages", len(v.Document.Messages))
	}
}

func buildService(t *testing.T) (*Service, Session) {
	s, root := interpretedService(t)
	s.Config.TwoDoor = true
	s.Compiler = buildScheduleCompiler{}
	v, err := s.Store.CreateEvaluation(context.Background(), root.Actor, root.ID, uuid.New(), "build")
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, s.Store, v.ID)
	return s, v
}

// This fixture tests scheduling, not grading. The API journey uses the real compiler.
type buildScheduleCompiler struct{ memoryCompiler }

func (buildScheduleCompiler) Compile(b json.RawMessage, _ string, _ uuid.UUID, _ Limits) (Compiled, error) {
	var doc struct {
		Cases []struct {
			Key     string         `json:"key"`
			Payload map[string]any `json:"payload"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return Compiled{}, err
	}
	out := Compiled{}
	for _, c := range doc.Cases {
		out.Cases = append(out.Cases, challengepack.CaseDefinition{CaseKey: c.Key, Payload: c.Payload})
	}
	return out, nil
}
func startBuild(t *testing.T, s *Service, v Session, content string) (Operation, BuildQuote) {
	t.Helper()
	ctx := context.Background()
	q, err := s.QuoteBuild(ctx, v.Actor, v.ID, BuildQuoteRequest{Content: content, Models: DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	o, err := s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: content, Models: DefaultModels(), TestJourney: true, CycleID: &q.ID})
	if err != nil {
		t.Fatal(err)
	}
	return o, q
}
func executeBuildFixture(t *testing.T, s *Service, o Operation, answer func(provider.Request) any) Session {
	t.Helper()
	ctx := context.Background()
	r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		cost := json.Number("0.000001")
		return provider.Response{OutputText: string(raw(answer(req))), Usage: provider.Usage{CostUSD: &cost}}, nil
	})}}
	if err := r.Execute(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Finalize(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	v, err := s.Store.GetSession(ctx, o.Actor, o.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestIntegrationVibeBuildContextsDoNotInheritRules(t *testing.T) {
	s, root, _ := memoryService(t)
	ctx := context.Background()
	key := uuid.New()
	a, err := s.Store.CreateEvaluation(ctx, root.Actor, root.ID, key, "build")
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, s.Store, a.ID)
	again, err := s.Store.CreateEvaluation(ctx, root.Actor, root.ID, key, "build")
	if err != nil || again.ID != a.ID {
		t.Fatal("duplicate context", err)
	}
	_, err = s.Store.CreateEvaluation(ctx, root.Actor, root.ID, key, "test")
	requireFault(t, err, "idempotency_conflict")
	_, err = s.Store.CreateEvaluation(ctx, "anon:other", root.ID, uuid.New(), "test")
	requireFault(t, err, "not_found")
	b, err := s.Store.CreateEvaluation(ctx, root.Actor, root.ID, uuid.New(), "test")
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, s.Store, b.ID)
	_, _ = submitPlan(t, s.Store, a, s.Config, 1000)
	b, err = s.Store.GetSession(ctx, root.Actor, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Document.Messages) != 0 || len(b.Operations) != 0 || b.Document.ConversationState != nil {
		t.Fatal("cross evaluation pollution")
	}
	if err = s.Store.Edit(ctx, b.Actor, b.ID, b.Revision, func(v *Session) error { v.Title = "Support bot"; return nil }); err != nil {
		t.Fatal("another evaluation blocked editing", err)
	}
}

func TestIntegrationVibeBuildOneQuestionThenUnknownRunsSample(t *testing.T) {
	s, v := buildService(t)
	ctx := context.Background()
	job := "Build a returns assistant."
	o, q := startBuild(t, s, v, job)
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(askAction{Kind: "ask", Text: "Which returns qualify?", Purpose: "clarify_rule", Options: []string{}}, factObservation{Kind: "job", Quote: job})
	})
	if v.Document.Build.ClarificationsUsed != 1 || v.Document.Build.Phase != "clarifying" {
		t.Fatal("question budget was not persisted")
	}
	next, err := s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "I don't know", Models: DefaultModels(), TestJourney: true, CycleID: &q.ID})
	if err != nil {
		t.Fatal(err)
	}
	v = executeBuildFixture(t, s, next, func(provider.Request) any {
		value := interpretationFixture(replyAction{Kind: "reply", Text: "We can use a sample."})
		value.Answer = &answerObservation{Quote: "I don't know", Unknown: true}
		return value
	})
	if len(v.Document.Artifacts) != 1 || v.Document.Artifacts[0].Sample != "returns" || v.Document.Build.ClarificationsUsed != 1 {
		t.Fatal("unknown did not become labelled sample", string(raw(v.Document.Build)))
	}
	if len(v.Document.Policies) != 0 || len(v.Document.Requirements) != 0 {
		t.Fatal("sample policy became business requirements")
	}
	checks := 0
	for _, op := range v.Operations {
		if op.Kind == "check" {
			checks++
			if len(op.Results) != 3 {
				t.Fatal("missing planned cases")
			}
		}
	}
	if checks != 1 {
		t.Fatal("sample stopped at preparation", checks)
	}
	ResumeBuilds(ctx, s)
	again, _ := s.Store.GetSession(ctx, v.Actor, v.ID)
	if len(again.Operations) != len(v.Operations) {
		t.Fatal("continuation duplicated")
	}
}

func TestIntegrationVibeBuildClearBriefCreatesRunnablePrototype(t *testing.T) {
	s, v := buildService(t)
	job := "My agent helps with returns."
	rule := "Only unopened items bought within 30 days qualify."
	o, _ := startBuild(t, s, v, job+" "+rule)
	calls := 0
	v = executeBuildFixture(t, s, o, func(req provider.Request) any {
		calls++
		switch calls {
		case 1:
			return interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: job}, factObservation{Kind: "rule", Quote: rule})
		case 2:
			var input taskInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			id := input.CurrentRequest.ID
			return createSuiteCommand{Rules: []PolicyRule{{ID: "returns", Statement: rule, SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: rule, Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Returns", Summary: "Unopened, within 30 days.", SuccessCriteria: rule, Scenarios: []TestScenario{{Input: "Unopened, 10 days.", Expected: "Eligible."}, {Input: "Opened, 10 days.", Expected: "Not eligible."}, {Input: "Unopened, 45 days.", Expected: "Not eligible."}}}}
		case 3:
			var input SuiteReviewInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			return supportedSuiteReview(input)
		case 4:
			if strings.Contains(req.Messages[1].Content, "Eligible.") || strings.Contains(req.Messages[1].Content, "45 days") {
				t.Fatal("expected answers reached prototype writer")
			}
			return map[string]string{"instructions": rule}
		default:
			t.Fatal("unexpected dispatch")
			return nil
		}
	})
	if v.Document.Build.ClarificationsUsed != 0 || v.Document.Build.Phase != "checking" {
		t.Fatal("clear brief stopped early", string(raw(v.Document.Build)))
	}
	if len(v.Document.Artifacts) != 1 || v.Document.Artifacts[0].AgentPrompt == "" {
		t.Fatal("prototype not runnable")
	}
}

func TestIntegrationVibeBuildQuoteIsBoundAndCannotResetQuestionBudget(t *testing.T) {
	s, v := buildService(t)
	ctx := context.Background()
	request := BuildQuoteRequest{Content: "Build a returns assistant.", Models: DefaultModels()}
	q, err := s.QuoteBuild(ctx, v.Actor, v.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	spare, err := s.QuoteBuild(ctx, v.Actor, v.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "changed", Models: DefaultModels(), TestJourney: true, CycleID: &q.ID})
	requireFault(t, err, "quote_expired")
	o, err := s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: request.Content, Models: DefaultModels(), TestJourney: true, CycleID: &q.ID})
	if err != nil {
		t.Fatal(err)
	}
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(askAction{Kind: "ask", Text: "Which returns qualify?", Purpose: "clarify_rule", Options: []string{}}, factObservation{Kind: "job", Quote: request.Content})
	})
	_, err = s.QuoteBuild(ctx, v.Actor, v.ID, request)
	requireFault(t, err, "invalid_request")
	_, err = s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: request.Content, Models: DefaultModels(), TestJourney: true, CycleID: &spare.ID})
	requireFault(t, err, "invalid_state")
}

func TestIntegrationVibeRunQuoteDoesNotExecuteAndBindsVersion(t *testing.T) {
	s, v := buildService(t)
	ctx := context.Background()
	proposal := samplePrototype("returns")
	b, err := s.Compiler.Draft(proposal, s.Config.Limits(v.Anonymous))
	if err != nil {
		t.Fatal(err)
	}
	a := Artifact{ID: uuid.New(), Kind: "test_suite", Sample: "returns", AgentPrompt: proposal.AgentPrompt, Blueprint: b, CreatedAt: timestamp()}
	if err = s.Store.Edit(ctx, v.Actor, v.ID, v.Revision, func(x *Session) error { x.Document.Artifacts = append(x.Document.Artifacts, a); return nil }); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Store.GetSession(ctx, v.Actor, v.ID)
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "check", Models: DefaultModels(), ArtifactID: &a.ID, ApproveArtifact: true}
	q, err := s.QuoteRun(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.Store.GetSession(ctx, v.Actor, v.ID)
	if len(before.Operations) > 0 || before.Revision != v.Revision {
		t.Fatal("quote changed state or ran work")
	}
	sub.RunQuoteID = &q.ID
	o, err := s.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil || again.ID != o.ID {
		t.Fatal("lost acknowledgement duplicated run", err)
	}
	changed := sub
	changed.ClientID = uuid.New()
	changed.Revision++
	_, err = s.Prepare(ctx, v.Actor, v.ID, changed)
	if err == nil {
		t.Fatal("quote reused for another operation")
	}
}

func TestIntegrationVibeBuildVagueThenUnknownDoesNotInterview(t *testing.T) {
	s, v := buildService(t)
	ctx := context.Background()
	o, q := startBuild(t, s, v, "I want to automate stuff")
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(replyAction{Kind: "reply", Text: "Tell me more."})
	})
	if v.Document.Build.ClarificationsUsed != 1 || v.Document.ConversationState.PendingQuestion == nil {
		t.Fatal("canonical question not persisted")
	}
	o, err := s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "I don't know", Models: DefaultModels(), TestJourney: true, CycleID: &q.ID})
	if err != nil {
		t.Fatal(err)
	}
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		value := interpretationFixture(replyAction{Kind: "reply", Text: "Here is a sample."})
		value.Answer = &answerObservation{Quote: "I don't know", Unknown: true}
		return value
	})
	if v.Document.Build.ClarificationsUsed != 1 || v.Document.Build.Phase != "checking" || v.Document.Artifacts[0].Sample != "email" {
		t.Fatal("vague request became another interview")
	}
}
