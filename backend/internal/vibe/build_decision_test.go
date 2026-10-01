package vibe

import (
	"context"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestIntegrationBuildBanterPreservesQuestionAndContinuation(t *testing.T) {
	s, v := buildService(t)
	ctx := context.Background()
	job := "Build a returns assistant."
	o, quote := startBuild(t, s, v, job)
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(buildAskAction{askAction: askAction{Kind: "ask", Text: "Which returns qualify?", Purpose: "clarify_rule", Options: []string{}}, MissingFactType: "correctness_rule", WhyNeeded: "Eligibility requires a policy"}, factObservation{Kind: "job", Quote: job})
	})
	question := v.Document.ConversationState.PendingQuestion.ID
	for _, content := range []string{"let's drink vodka yay", "hello again"} {
		sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: content, Models: DefaultModels(), TestJourney: true, CycleID: &quote.ID}
		next, err := s.Prepare(ctx, v.Actor, v.ID, sub)
		if err != nil {
			t.Fatal(err)
		}
		duplicate, err := s.Prepare(ctx, v.Actor, v.ID, sub)
		if err != nil || duplicate.ID != next.ID {
			t.Fatal("duplicate follow-up was not idempotent", err)
		}
		calls := 0
		v = executeBuildFixture(t, s, next, func(provider.Request) any {
			calls++
			return interpretationFixture(replyAction{Kind: "reply", Text: "Your returns task is still here when you are ready."})
		})
		if calls != 1 || len(v.Document.Artifacts) != 0 || len(v.Document.Policies) != 0 || v.Document.Build.Phase != "clarifying" || v.Document.Build.ClarificationsUsed != 1 {
			t.Fatal("banter changed the specification, spent the question, or generated work", calls, string(raw(v.Document.Build)))
		}
		if q := v.Document.ConversationState.PendingQuestion; q.ID != question || q.Status != "active" {
			t.Fatal("question lost")
		}
		for _, fact := range v.Document.ConversationState.Brief.Facts {
			if fact.Text != nil && strings.Contains(*fact.Text, "vodka") {
				t.Fatal("banter became a requirement")
			}
		}
	}
	next, err := s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: "I don't know", Models: DefaultModels(), TestJourney: true, CycleID: &quote.ID})
	if err != nil {
		t.Fatal(err)
	}
	v = executeBuildFixture(t, s, next, func(provider.Request) any {
		value := interpretationFixture(replyAction{Kind: "reply", Text: "Let's try a sample policy."})
		value.Answer = &answerObservation{Quote: "I don't know", Unknown: true}
		return value
	})
	if len(v.Document.Artifacts) != 1 || v.Document.Build.Phase != "checking" || v.Document.Build.ClarificationsUsed != 1 {
		t.Fatal("the real answer did not continue the original cycle", string(raw(v.Document.Build)))
	}
	checks := 0
	for _, op := range v.Operations {
		if op.Kind == "check" {
			checks++
			if len(op.Results) != 3 {
				t.Fatal("expected three admitted checks")
			}
		}
	}
	if checks != 1 {
		t.Fatal("expected exactly one check operation", checks)
	}
}

func TestIntegrationBuildFirstBanterDoesNotAskOrBuild(t *testing.T) {
	s, v := buildService(t)
	o, _ := startBuild(t, s, v, "let's drink vodka yay")
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(replyAction{Kind: "reply", Text: "I'm here when you have an AI task to try."})
	})
	if v.Document.Build.Phase != "waiting" || v.Document.Build.ClarificationsUsed != 0 || len(v.Document.Artifacts) != 0 || v.Document.ConversationState.PendingQuestion != nil {
		t.Fatal("casual first message started onboarding or execution")
	}
}

func TestIntegrationBuildEmailStarterNeedsNoQuestion(t *testing.T) {
	s, v := buildService(t)
	job := "Sort spam from customer emails."
	o, _ := startBuild(t, s, v, job)
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(buildAskAction{askAction: askAction{Kind: "ask", Text: "Which messages are spam?", Purpose: "clarify_rule", Options: []string{}}, MissingFactType: "correctness_rule", WhyNeeded: "Classification criteria"}, factObservation{Kind: "job", Quote: job})
	})
	if v.Document.Build.ClarificationsUsed != 0 || v.Document.Build.Phase != "checking" || len(v.Document.Artifacts) != 1 || v.Document.Artifacts[0].Sample != "email_sorting" {
		t.Fatal("common task did not create a labelled starter without interviewing", string(raw(v.Document.Build)))
	}
}
