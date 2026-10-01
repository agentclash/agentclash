package vibe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
)

func TestBuildReviewRequiresEveryAssertion(t *testing.T) {
	_, input := suiteReviewFixture(t)
	input.ValidatorVersion = AssertionSuiteValidatorVersion
	legacy := supportedSuiteReview(input)
	delete(legacy, "assertions")
	if _, err := ParseSuiteReview(raw(legacy), input, LimitsFor(false)); err == nil {
		t.Fatal("case-level approval must not bypass assertion review")
	}
	claims := suiteAssertions(input)
	reviews := []SuiteAssertionReview{}
	for _, claim := range claims {
		reviews = append(reviews, SuiteAssertionReview{ID: claim.ID, SuiteReviewFinding: SuiteReviewFinding{
			Status: SuiteSupported, RuleIDs: []string{"eligibility", "no-refund"},
			SourceBlockIDs: []string{"source-policy"}, Reason: "These original rules support every part of this assertion.",
		}, Support: []AssertionSupport{{RuleID: "eligibility", SourceBlockID: "source-policy", Quote: input.Sources[0].Text}}})
	}
	legacy["assertions"] = reviews
	valid, err := ParseSuiteReview(raw(legacy), input, LimitsFor(false))
	if err != nil || valid.Status != SuiteSupported {
		t.Fatalf("legitimate refund policy rejected: %+v %v", valid, err)
	}
	reviews[len(reviews)-1].Status = SuiteContradicted
	reviews[len(reviews)-1].Reason = "The source does not authorize this prohibition; harmless is not supported."
	reviews[len(reviews)-1].Support = nil
	rejected, err := ParseSuiteReview(raw(legacy), input, LimitsFor(false))
	if err != nil || rejected.Status != SuiteContradicted {
		t.Fatalf("unsupported assertion accepted: %+v %v", rejected, err)
	}
	reviews[len(reviews)-1].Status = SuiteSupported
	reviews[len(reviews)-1].Support = []AssertionSupport{{RuleID: "no-refund", SourceBlockID: "source-policy", Quote: "invented source text"}}
	if _, err = ParseSuiteReview(raw(legacy), input, LimitsFor(false)); err == nil {
		t.Fatal("invented support accepted")
	}
}

func TestBuildAssertionsIncludeUnrelatedProhibitions(t *testing.T) {
	input := SuiteReviewInput{Cases: []SuiteReviewCase{{CaseKey: "spam", Expected: "Classify the message as spam. Do not claim any refund was processed."}}, SharedCriteria: "Prizes or requests for bank details are spam."}
	claims := suiteAssertions(input)
	if len(claims) != 3 || claims[1].Text != "Do not claim any refund was processed." {
		t.Fatalf("prohibition escaped review: %+v", claims)
	}
	input.Cases[0].Expected = "Allow amounts below $10.50; otherwise escalate."
	claims = suiteAssertions(input)
	if len(claims) != 3 || !strings.Contains(claims[0].Text, "10.50") {
		t.Fatalf("decimal changed: %+v", claims)
	}
}

func TestBuildSamplesMatchTask(t *testing.T) {
	for _, tc := range []struct{ job, want string }{
		{"Separate spam from customer emails", "email_sorting"},
		{"Draft customer email replies", "email"},
		{"Handle shop refunds", "returns"},
		{"Convert PDF to markdown", ""},
		{"I need help", ""},
	} {
		if got := sampleKindForJob(tc.job); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.job, got, tc.want)
		}
	}
	sample := samplePrototype("email_sorting")
	if len(sample.Scenarios) != 3 || strings.Contains(sample.SuccessCriteria, "refund") {
		t.Fatalf("wrong sample: %+v", sample)
	}
}

func TestBuildQuestionNeedsCorrectnessReason(t *testing.T) {
	job := "My agent sorts spam emails."
	rule := "Prize promises are spam. Order questions are customer messages."
	p, _ := memoryPlan(t, Document{}, job+" "+rule)
	p.AuthoringVersion = buildAuthoringVersion
	a := buildAskAction{askAction: askAction{Kind: "ask", Text: "Move email where?", Purpose: "clarify_rule", Options: []string{}}, MissingFactType: "integration", WhyNeeded: "Moving email needs a destination", CanNarrow: true}
	v := interpretationFixture(a, factObservation{Kind: "job", Quote: job}, factObservation{Kind: "rule", Quote: rule})
	route, err := decodeInterpretation(raw(v), p)
	if err != nil || route.Intent != "prepare_tests" || route.Memory.Question != nil {
		t.Fatal("optional integration prevented a useful supported version", err)
	}
	a.WhyNeeded = ""
	v.Action = raw(a)
	if _, err = decodeInterpretation(raw(v), p); err == nil {
		t.Fatal("unjustified question accepted")
	}
}

func TestBuildReadinessRequiresSeparateRuleEvidence(t *testing.T) {
	job := "I waste time sorting emails."
	rules := "Label messages promising prizes or asking for bank details as spam. Label order questions as customer messages. For anything else, say unsure. Only suggest labels; do not move or send any email."
	p, _ := memoryPlan(t, Document{}, job+" "+rules)
	p.AuthoringVersion = buildAuthoringVersion
	compound := interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: p.Submission.Content})
	if _, err := decodeInterpretation(raw(compound), p); err == nil {
		t.Fatal("inconsistent readiness must request internal repair, not ask the user or promote the job to a rule")
	}
	separated := interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: job}, factObservation{Kind: "rule", Quote: rules})
	if r, err := decodeInterpretation(raw(separated), p); err != nil || r.Intent != "prepare_tests" || r.Memory.Question != nil {
		t.Fatalf("complete brief needs no clarification: %+v %v", r, err)
	}
	p.Submission.Content = job
	missing := interpretationFixture(buildAskAction{askAction: askAction{Kind: "ask", Text: "Which messages count as spam?", Purpose: "clarify_rule", Options: []string{}}, MissingFactType: "correctness_rule", WhyNeeded: "The classification needs a spam rule"}, factObservation{Kind: "job", Quote: job})
	if r, err := decodeInterpretation(raw(missing), p); err != nil || r.Intent != "clarify" {
		t.Fatal("a real missing rule still needs one question", err)
	}
}

func TestIntegrationVibeBuildRejectsInventedExpectation(t *testing.T) {
	s, session := buildService(t)
	job, rule := "My agent sorts spam emails.", "Prize promises or bank detail requests are spam. Order questions are customer messages."
	op, _ := startBuild(t, s, session, job+" "+rule)
	calls := 0
	r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		calls++
		var out any
		switch calls {
		case 1:
			out = interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: job}, factObservation{Kind: "rule", Quote: rule})
		case 2:
			var input taskInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			id := input.CurrentRequest.ID
			out = createSuiteCommand{Rules: []PolicyRule{{ID: "spam", Statement: rule, SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: rule, Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Spam sorter", Summary: rule, SuccessCriteria: rule, Scenarios: []TestScenario{{Input: "You won a prize!", Expected: "Classify as spam. Do not claim any refund was processed."}, {Input: "Send bank details.", Expected: "Classify as spam."}, {Input: "Where is my order?", Expected: "Classify as a customer message."}}}}
		case 3:
			var input SuiteReviewInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			out = supportedSuiteReview(input)
			reviews := out.(map[string]any)["assertions"].([]SuiteAssertionReview)
			rejected := false
			for i, a := range suiteAssertions(input) {
				if strings.Contains(a.Text, "refund") {
					reviews[i].Status, reviews[i].Reason, reviews[i].Support = SuiteContradicted, "No original rule supports this refund prohibition.", nil
					rejected = true
				}
			}
			if !rejected {
				t.Fatal("bad assertion escaped review")
			}
		case 4:
			// A malformed bounded repair must stop, not dispatch the target.
			out = map[string]any{"unsupported": "repair"}
		default:
			t.Fatal("invalid suite caused extra dispatch", calls)
		}
		cost := json.Number("0.000001")
		return provider.Response{OutputText: string(raw(out)), Usage: provider.Usage{CostUSD: &cost}}, nil
	})}}
	err := r.Execute(context.Background(), op.ID)
	if f, ok := err.(*Fault); ok && f.Context != nil {
		t.Logf("calls=%d context=%+v", calls, f.Context)
	}
	requireFault(t, err, "invalid_repair")
	if e := r.Finalize(context.Background(), op.ID, err.(*Fault)); e != nil {
		t.Fatal(e)
	}
	got, err := s.Store.GetSession(context.Background(), session.Actor, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 || len(got.Document.Artifacts) != 0 || len(got.Operations) != 1 {
		t.Fatal("unsupported suite was saved, run, or retried without bounds")
	}
}
