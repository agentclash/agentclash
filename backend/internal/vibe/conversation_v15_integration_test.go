package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func interpretedService(t *testing.T) (*Service, Session) {
	s, v, _ := memoryService(t)
	s.Config.PreciseActions, s.Config.ContextGuidance, s.Config.InterpretedAuthoring = true, true, true
	return s, v
}

func TestIntegrationVibeV15JokeThenShopifyAndBoundAnswer(t *testing.T) {
	s, v := interpretedService(t)
	v, _ = memoryExecute(t, s, v, "drin vodka hewhe", func(provider.Request) any {
		return interpretationFixture(replyAction{Kind: "reply", Text: "Tell me what your agent should help with."})
	})
	if v.Document.ConversationState.PendingPreparation != nil {
		t.Fatal("joke became a pending test request")
	}
	job := "Build me a returns agent for Shopify"
	v, _ = memoryExecute(t, s, v, job, func(req provider.Request) any {
		if strings.Contains(req.Messages[1].Content, "supersedes_id") {
			t.Fatal("legacy mutation contract sent")
		}
		return interpretationFixture(askAction{Kind: "ask", Text: "Which returns qualify?", Purpose: "clarify_rule", Options: []string{"Items bought within 30 days"}}, factObservation{Kind: "job", Quote: job})
	})
	if len(v.Document.Artifacts) != 0 || len(v.Document.Policies) != 0 {
		t.Fatal("invented tests from a job alone")
	}
	if pending := v.Document.ConversationState.PendingPreparation; pending == nil || pending.Text != job {
		t.Fatal("clarification lost the original preparation request")
	}
	q := v.Document.ConversationState.PendingQuestion
	a := interaction.Action{IdempotencyKey: uuid.NewString(), SessionRevision: v.Revision, ScopeID: q.ScopeID, Kind: "answer_question", TargetID: q.ID, TargetRevision: q.Revision, OptionIDs: []string{"option-1"}}
	ctx := context.Background()
	if err := s.Interact(ctx, v.Actor, v.ID, a); err != nil {
		t.Fatal(err)
	}
	if err := s.Interact(ctx, v.Actor, v.ID, a); err != nil {
		t.Fatal("duplicate answer", err)
	}
	updated, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, e := s.Store.submissionReceiptForAction(ctx, v.ID, a)
	if e != nil || receipt == nil {
		t.Fatal("missing receipt", e)
	}
	op := *receipt
	if op.ID == uuid.Nil {
		t.Fatal("button did not continue through authoring")
	}
	calls := 0
	r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		calls++
		var output any
		switch calls {
		case 1:
			v := interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 1}, factObservation{Kind: "rule", Quote: "Items bought within 30 days"})
			v.Answer = &answerObservation{Quote: "Items bought within 30 days"}
			output = v
		case 2:
			if strings.Contains(req.Messages[1].Content, "vodka") {
				t.Fatal("joke entered author context")
			}
			var input taskInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			if len(input.QuestionAnswers) != 1 {
				t.Fatal("lost bound answer")
			}
			id := input.CurrentRequest.ID
			output = createSuiteCommand{Rules: []PolicyRule{{ID: "returns", Statement: "Items bought within 30 days qualify for return.", SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: "Items bought within 30 days", Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Returns", Summary: "Checks the return window.", SuccessCriteria: "Follow the return window.", Scenarios: []TestScenario{{Input: "Bought 10 days ago.", Expected: "Eligible for return."}}}}
		case 3:
			var input SuiteReviewInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
			if strings.Contains(req.Messages[1].Content, "vodka") || len(input.QuestionAnswers) != 1 {
				t.Fatal("review lost clean sources")
			}
			output = supportedSuiteReview(input)
		default:
			t.Fatal("unexpected additional dispatch")
		}
		cost := json.Number("0.000001")
		return provider.Response{OutputText: string(raw(output)), Usage: provider.Usage{CostUSD: &cost}}, nil
	})}}
	if err = r.Execute(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	if err = r.Finalize(ctx, op.ID, nil); err != nil {
		t.Fatal(err)
	}
	updated, err = s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Document.Artifacts) != 1 || len(updated.Document.ConversationState.Answers) != 1 || len(updated.Document.Interactions) != 1 {
		t.Fatal("answer/test transaction incomplete")
	}
	if updated.Document.ConversationState.PendingPreparation != nil {
		t.Fatal("completed preparation was left pending")
	}
	if err = s.Interact(ctx, v.Actor, v.ID, a); err != nil {
		t.Fatal("completed answer delivery was repeated", err)
	}
	if calls != 3 {
		t.Fatal("duplicate model calls")
	}
}

func TestIntegrationVibeV15OneAlternativeAndReplay(t *testing.T) {
	s, v := interpretedService(t)
	ctx := context.Background()
	o, p := memoryOperation(t, s, v, "drin vodka hewhe")
	alternative := s.Config.Profiles["openai/gpt-4.1-mini"]
	p.AssistantRecovery = &AssistantRecovery{Profile: alternative, MaxCost: p.MaxCost / 8}
	p.Calls = 9
	p.MaxCost += p.AssistantRecovery.MaxCost
	if _, err := s.Store.DB.Exec(ctx, "UPDATE vibe_operations SET input=$2,max_cost=$3 WHERE id=$1", o.ID, raw(p), p.MaxCost); err != nil {
		t.Fatal(err)
	}
	calls := 0
	r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		calls++
		output := `{"bad":"shape"}`
		if calls == 3 {
			if req.Model != alternative.ID {
				t.Fatal("alternative not selected")
			}
			output = string(raw(interpretationFixture(replyAction{Kind: "reply", Text: "Tell me what your agent should help with."})))
		}
		if calls > 3 {
			t.Fatal("more than one fallback")
		}
		cost := json.Number("0.000001")
		return provider.Response{OutputText: output, Usage: provider.Usage{CostUSD: &cost}}, nil
	})}}
	if err := r.Execute(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.Store.Operation(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Models != o.Models || got.ModelCalls != 3 || got.Completion == nil {
		t.Fatal("fallback changed roles or lost completion")
	}
	var steps string
	if err = s.Store.DB.QueryRow(ctx, "SELECT string_agg(step_key,',' ORDER BY created_at) FROM vibe_attempts WHERE operation_id=$1", o.ID).Scan(&steps); err != nil {
		t.Fatal(err)
	}
	if steps != "route,route:repair,route:fallback" {
		t.Fatal(steps)
	}
	// Replay reads the same three settled outputs even when provider config is gone.
	r.Gateway.Config.Profiles = nil
	if err = r.converseReliable(context.WithValue(ctx, reliableReplayKey{}, true), got, p); err != nil {
		t.Fatal("replay", err)
	}
	if calls != 3 {
		t.Fatal("replay spent again")
	}
	// Exercise the transactional fallback guard itself, independently of the
	// completed-operation/configuration checks that also prevent dispatch.
	err = s.Store.transaction(ctx, func(tx pgx.Tx) error {
		return checkAssistantRecovery(ctx, tx, got, p, Attempt{Step: "handler:fallback", MaxCost: p.AssistantRecovery.MaxCost})
	})
	requireFault(t, err, "operation_limit")
}

func TestIntegrationVibeV15UncertainBillingStopsRecovery(t *testing.T) {
	s, v := interpretedService(t)
	ctx := context.Background()
	o, p := memoryOperation(t, s, v, "hi")
	p.AssistantRecovery = &AssistantRecovery{Profile: s.Config.Profiles["openai/gpt-4.1-mini"], MaxCost: p.MaxCost / 8}
	p.Calls = 9
	p.MaxCost += p.AssistantRecovery.MaxCost
	if _, err := s.Store.DB.Exec(ctx, "UPDATE vibe_operations SET input=$2,max_cost=$3 WHERE id=$1", o.ID, raw(p), p.MaxCost); err != nil {
		t.Fatal(err)
	}
	calls := 0
	r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(context.Context, provider.Request) (provider.Response, error) {
		calls++
		return provider.Response{OutputText: `{}`}, nil
	})}}
	err := r.Execute(ctx, o.ID)
	if err == nil || issueFrom(err).Code != "usage_unknown" || calls != 1 {
		t.Fatal(fmt.Sprint(err), calls)
	}
	err = s.Store.transaction(ctx, func(tx pgx.Tx) error {
		return checkAssistantRecovery(ctx, tx, o, p, Attempt{Step: "route:fallback", MaxCost: p.AssistantRecovery.MaxCost})
	})
	requireFault(t, err, "operation_limit")
}

func TestIntegrationVibeV15RetriesOnlyFailedStageAndRecordsReviewer(t *testing.T) {
	for _, failedStage := range []string{"handler", "review"} {
		t.Run(failedStage, func(t *testing.T) {
			s, v := interpretedService(t)
			ctx := context.Background()
			text := "My agent handles returns. Items bought within 30 days qualify. Prepare one test."
			o, p := memoryOperation(t, s, v, text)
			alternative := s.Config.Profiles["openai/gpt-4.1-mini"]
			p.AssistantRecovery = &AssistantRecovery{Profile: alternative, MaxCost: p.MaxCost / 8}
			p.Calls = 9
			p.MaxCost += p.AssistantRecovery.MaxCost
			if _, err := s.Store.DB.Exec(ctx, "UPDATE vibe_operations SET input=$2,max_cost=$3 WHERE id=$1", o.ID, raw(p), p.MaxCost); err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
				var format struct {
					JSONSchema struct {
						Name string `json:"name"`
					} `json:"json_schema"`
				}
				_ = json.Unmarshal(req.ResponseFormat, &format)
				stage := "review"
				if strings.HasPrefix(req.Messages[0].Content, interpretationPrompt) {
					stage = "route"
				}
				if format.JSONSchema.Name == "vibe_prepare_tests_v11" {
					stage = "handler"
				}
				counts[stage]++
				var output any
				if stage == failedStage && counts[stage] <= 2 {
					output = map[string]any{"invalid": "fixture"}
				} else {
					switch stage {
					case "route":
						output = interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 1}, factObservation{Kind: "job", Quote: "My agent handles returns."}, factObservation{Kind: "rule", Quote: "Items bought within 30 days qualify."})
					case "handler":
						var input taskInput
						_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
						id := input.CurrentRequest.ID
						output = createSuiteCommand{Rules: []PolicyRule{{ID: "window", Statement: "Items bought within 30 days qualify.", SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: "Items bought within 30 days qualify.", Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Returns", Summary: "Checks eligibility.", SuccessCriteria: "Follow the return window.", Scenarios: []TestScenario{{Input: "Bought 10 days ago.", Expected: "Eligible for return."}}}}
					case "review":
						var input SuiteReviewInput
						_ = json.Unmarshal([]byte(req.Messages[1].Content), &input)
						output = supportedSuiteReview(input)
					}
				}
				if stage == failedStage && counts[stage] == 3 && req.Model != alternative.ID {
					t.Fatal("did not use frozen alternative")
				}
				cost := json.Number("0.000001")
				return provider.Response{OutputText: string(raw(output)), Usage: provider.Usage{CostUSD: &cost}}, nil
			})}}
			if err := r.Execute(ctx, o.ID); err != nil {
				t.Fatal(err)
			}
			if err := r.Finalize(ctx, o.ID, nil); err != nil {
				t.Fatal(err)
			}
			got, err := s.Store.GetSession(ctx, v.Actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			if counts["route"] != 1 || counts[failedStage] != 3 || len(got.Document.Artifacts) != 1 {
				t.Fatal("repeated completed stage", counts)
			}
			reviewer := o.Models.Assistant
			if failedStage == "review" {
				reviewer = alternative.ID
			}
			if got.Document.Artifacts[0].Validation.Model != reviewer {
				t.Fatal("review provenance names the wrong model")
			}
			if got.Operations[0].Models != o.Models || got.Operations[0].ModelCalls != 5 || got.Operations[0].Diagnostics.AlternativeAssistant != alternative.ID {
				t.Fatal("role/call/progress invariants failed")
			}
		})
	}
}
