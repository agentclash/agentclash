package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

type recoveryCompiler struct {
	repairCompiler
	blueprint json.RawMessage
}

func (c recoveryCompiler) Draft(DraftProposal, Limits) (json.RawMessage, error) {
	return c.blueprint, nil
}

// Seed exactly the evidence a worker has when it loses its acknowledgement after
// the paid review, before CompleteDocument. No test invokes a provider.
func recordedAuthoring(t *testing.T, mode string) (*Store, Session, Operation, *Runner, *int) {
	t.Helper()
	ctx := context.Background()
	s := integrationStore(t)
	v := anonSession(t, s)
	cfg := testConfig()
	blueprint, fixture := suiteReviewFixture(t)
	profile := cfg.Profiles[DefaultModels().Assistant]
	profile.StructuredOutputs = true
	cfg.Profiles[profile.ID] = profile
	sub := Submission{ClientID: fixture.CurrentRequest.MessageID, Revision: v.Revision, Kind: "message", Content: fixture.CurrentRequest.Text, Models: DefaultModels(), TestJourney: true}
	l := LimitsFor(true)
	l.OperationSeconds = 315
	p := Plan{AuthoringVersion: 11, Anonymous: true, Submission: sub, Document: v.Document, Calls: 3, MaxCost: NanoUSD / 100, ExecutionLimits: &l,
		Conversation: &ConversationContext{Sources: fixture.Sources, CurrentRequest: fixture.CurrentRequest, ContractVersion: "v11", Profile: &profile}}
	o, err := s.Submit(ctx, v.Actor, v.ID, sub, p, cfg)
	if err != nil {
		t.Fatal(err)
	}
	o, _, err = s.Start(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(o.Input, &p); err != nil {
		t.Fatal(err)
	}
	calls := new(int)
	r := &Runner{Service: &Service{Store: s, Compiler: recoveryCompiler{blueprint: blueprint}}, Gateway: &Gateway{Store: s, Config: cfg, Gate: testGate(t), Client: callFunc(func(context.Context, provider.Request) (provider.Response, error) {
		*calls++
		return provider.Response{}, fmt.Errorf("recovery attempted a provider call")
	})}}
	record := func(step string, messages []provider.Message, format json.RawMessage, output []byte, uncertain bool) {
		t.Helper()
		a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: step, Role: Assistant, Model: o.Models.Assistant, Policy: raw(map[string]any{"response_format": format}), RequestHash: Hash(raw(messages)), InputBound: 1000, MaxOutput: 1000, MaxCost: 1000}
		if mode == "wrong_request" && step == "review" {
			a.RequestHash = Hash([]byte("another request"))
		}
		if err := s.BeginAttempt(ctx, a); err != nil {
			t.Fatal(err)
		}
		cost := int64(100)
		var known *int64 = &cost
		if uncertain {
			known = nil
		}
		response := provider.Response{OutputText: string(output)}
		if err := s.EndAttempt(ctx, a, response.OutputText, raw(response), known, nil); err != nil {
			t.Fatal(err)
		}
	}
	record("route", reliableMessages(p, reliableRoutePrompt, nil), reliableRouteFormat(profile, p), raw(reliableRoute{Intent: "prepare_tests", Reply: "I'll prepare one test.", Count: 1}), false)
	p.Conversation.RequiredCount = 1
	command := raw(createSuiteCommand{Tests: testSuiteProposal{Title: "Returns", Summary: "Checks eligibility and refund claims.", SuccessCriteria: fixture.SharedCriteria, Scenarios: []TestScenario{{Input: "An unopened item was bought 10 days ago.", Expected: "Confirm eligibility without claiming to process a refund."}}}, Rules: fixture.Policy.Rules})
	record("handler", reliableMessages(p, reliableAuthoringPrompt, map[string]any{"action": "prepare_tests", "count": 1}), reliableCommandFormat(profile, p, "prepare_tests"), command, false)
	candidate, policy, _, err := r.buildReliableCandidate(command, "prepare_tests", o, p)
	if err != nil {
		t.Fatal(err)
	}
	input, err := BuildSuiteReviewInput(candidate.Blueprint, policy, p.Conversation.Sources, p.Conversation.CurrentRequest, 1, p.limits())
	if err != nil {
		t.Fatal(err)
	}
	if mode != "missing_review" {
		output := raw(supportedSuiteReview(input))
		if mode == "invalid_review" {
			output = []byte(`{"cases":[]}`)
		}
		record("review", SuiteReviewMessages(input), SuiteReviewFormat(profile), output, mode == "uncertain_review")
	}
	if mode == "profile_changed" {
		r.Gateway.Config.Profiles = nil
	}
	return s, v, o, r, calls
}

func TestIntegrationVibeReliableRecoveryCommitsRecordedReviewWithoutDispatch(t *testing.T) {
	for _, mode := range []string{"complete", "profile_changed"} {
		t.Run(mode, func(t *testing.T) {
			s, v, o, r, calls := recordedAuthoring(t, mode)
			ctx := context.Background()
			// Recovery is allowed after the paid deadline, within the finalizer's
			// own deadline, because it cannot spend any more money.
			if _, err := s.DB.Exec(ctx, "UPDATE vibe_operations SET deadline=$2 WHERE id=$1", o.ID, timestamp().Add(-time.Minute)); err != nil {
				t.Fatal(err)
			}
			issue := &Fault{Code: "worker_interrupted", Message: "Worker stopped before committing."}
			if err := r.Finalize(ctx, o.ID, issue); err != nil {
				t.Fatal(err)
			}
			if err := r.Finalize(ctx, o.ID, issue); err != nil {
				t.Fatal(err)
			}
			current, err := s.GetSession(ctx, v.Actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			got := current.Operations[0]
			if got.State != Completed || got.Error != nil || got.Completion == nil || got.Completion.ValidationStatus != SuiteSupported || got.Completion.SourceMessageID != current.Document.Messages[0].ID || got.Billing != Settled {
				t.Fatalf("recorded review was not recovered: %+v", got)
			}
			if *calls != 0 || got.ModelCalls != 3 || len(current.Document.Artifacts) != 1 || len(current.Document.Messages) != 2 || len(current.Document.Policies) != 1 || current.Document.Messages[1].Content != "1 test is ready." {
				t.Fatalf("recovery repeated work: calls=%d operation=%+v document=%+v", *calls, got, current.Document)
			}
			artifact := current.Document.Artifacts[0]
			if artifact.ID != deterministicID(o.ID, "artifact") || !SuiteValidationMatches(artifact.Validation, artifact.Blueprint, current.Document.Policies[0]) {
				t.Fatal("recovery did not preserve deterministic artifact identity and validation")
			}
			var attempts, journaled int
			if err := s.DB.QueryRow(ctx, "SELECT count(*),count(domain_outcome) FROM vibe_attempts WHERE operation_id=$1", o.ID).Scan(&attempts, &journaled); err != nil || attempts != 3 || journaled != 3 {
				t.Fatalf("recovery lost domain outcomes: %d %d %v", attempts, journaled, err)
			}
		})
	}
}

func TestIntegrationVibeReliableRecoveryRejectsIncompleteEvidenceAndStoppedWork(t *testing.T) {
	for _, mode := range []string{"missing_review", "uncertain_review", "wrong_request", "invalid_review", "cancelled", "domain_failure"} {
		t.Run(mode, func(t *testing.T) {
			s, v, o, r, calls := recordedAuthoring(t, mode)
			ctx := context.Background()
			issue := &Fault{Code: "worker_interrupted", Message: "Worker stopped before committing."}
			if mode == "domain_failure" {
				issue = &Fault{Code: "test_policy_conflict", Message: "Review rejected these tests."}
			}
			if mode == "cancelled" {
				if err := s.Stop(ctx, v.Actor, o.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.Finalize(ctx, o.ID, issue); err != nil {
				t.Fatal(err)
			}
			current, err := s.GetSession(ctx, v.Actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			got := current.Operations[0]
			wantState, wantCalls := Failed, 3
			if mode == "cancelled" {
				wantState = Cancelled
			}
			if mode == "missing_review" {
				wantCalls = 2
			}
			if got.State != wantState || got.Completion != nil || *calls != 0 || got.ModelCalls != wantCalls || len(current.Document.Artifacts) != 0 || len(current.Document.Messages) != 1 {
				t.Fatalf("incomplete evidence or stopped work was replayed: calls=%d operation=%+v document=%+v", *calls, got, current.Document)
			}
			if mode != "cancelled" && (got.Error == nil || got.Error.Code != issue.Code) {
				t.Fatalf("recovery replaced original fault: %+v", got.Error)
			}
			if mode == "uncertain_review" && got.Billing != Reconciling {
				t.Fatalf("uncertain cost was released: %+v", got)
			}
		})
	}
}
