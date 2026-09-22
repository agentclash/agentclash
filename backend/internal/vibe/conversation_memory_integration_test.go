package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

type memoryCompiler struct{ repairCompiler }

func (memoryCompiler) Draft(p DraftProposal, _ Limits) (json.RawMessage, error) {
	cases := []any{}
	for i, c := range p.Scenarios {
		cases = append(cases, map[string]any{"key": fmt.Sprintf("case-%d", i+1), "payload": map[string]any{"question": c.Input}, "expectations": []any{map[string]any{"key": ExpectedBehaviorKey, "kind": "text", "value": c.Expected}}})
	}
	return raw(map[string]any{"instructions": "{{question}}", "cases": cases, "judges": []any{map[string]any{"key": "behavior", "assertion": ScenarioCriteriaPrefix + p.SuccessCriteria, "context_from": []string{ExpectedBehaviorReference}}}, "validators": []any{map[string]any{"key": "has_answer", "type": "regex_match", "target": "final_output", "expected_from": "literal:.+"}}, "dimensions": []any{map[string]any{"key": "behavior", "source": "llm_judge", "judge_key": "behavior"}}}), nil
}
func memoryService(t *testing.T) (*Service, Session, Config) {
	t.Helper()
	s := integrationStore(t)
	v := anonSession(t, s)
	cfg := testConfig()
	cfg.ReliableAuthoring, cfg.ConversationState = true, true
	cfg.SourcePolicyVersion = SourcePolicyVersion
	profile := cfg.Profiles[DefaultModels().Assistant]
	profile.StructuredOutputs = true
	cfg.Profiles[profile.ID] = profile
	return &Service{Store: s, Config: cfg, Gate: testGate(t), Compiler: memoryCompiler{}}, v, cfg
}
func memoryOperation(t *testing.T, s *Service, v Session, content string) (Operation, Plan) {
	t.Helper()
	o, err := s.Prepare(context.Background(), v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: content, Models: DefaultModels(), TestJourney: true})
	if err != nil {
		t.Fatal(string(raw(issueFrom(err))))
	}
	var p Plan
	if err = json.Unmarshal(o.Input, &p); err != nil || !p.stateful() {
		t.Fatal("v12 plan was not frozen", err)
	}
	return o, p
}
func memoryExecute(t *testing.T, s *Service, v Session, content string, answer func(provider.Request) any) (Session, Operation) {
	t.Helper()
	ctx := context.Background()
	o, _ := memoryOperation(t, s, v, content)
	calls := 0
	r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(_ context.Context, request provider.Request) (provider.Response, error) {
		calls++
		cost := json.Number("0.000001")
		return provider.Response{OutputText: string(raw(answer(request))), Usage: provider.Usage{CostUSD: &cost}}, nil
	})}}
	if err := r.Execute(ctx, o.ID); err != nil {
		t.Fatalf("after %d fixture calls: %s", calls, raw(issueFrom(err)))
	}
	if err := r.Finalize(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := s.Store.Operation(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	return current, operation
}
func TestIntegrationVibeConversationStateJourney(t *testing.T) {
	s, v, _ := memoryService(t)
	v, _ = memoryExecute(t, s, v, "give me vodka", func(provider.Request) any {
		return reliableRoute{Intent: "chat", Reply: "I'm here when you're ready.", Memory: &memoryUpdate{}}
	})
	job := "My agent answers shop return questions."
	v, _ = memoryExecute(t, s, v, job, func(provider.Request) any {
		return reliableRoute{Intent: "clarify", Reply: "How long after purchase can items be returned?", Memory: &memoryUpdate{Facts: []memoryFact{{Kind: "job", Quote: job}}, Question: &memoryQuestion{Purpose: "clarify_rule", Text: "How long after purchase can items be returned?", MaxSelections: 1}}}
	})
	question := *v.Document.ConversationState.PendingQuestion
	jobSource := v.Document.Messages[len(v.Document.Messages)-2].ID.String()
	v, _ = memoryExecute(t, s, v, "lol", func(provider.Request) any {
		return reliableRoute{Intent: "chat", Reply: "Ha.", Memory: &memoryUpdate{}}
	})
	if v.Document.ConversationState.PendingQuestion.ID != question.ID {
		t.Fatal("chat lost question binding")
	}
	stage := 0
	v, op := memoryExecute(t, s, v, "30 days", func(request provider.Request) any {
		stage++
		switch stage {
		case 1:
			var input taskInput
			if err := json.Unmarshal([]byte(request.Messages[1].Content), &input); err != nil || input.PendingQuestion == nil || input.PendingQuestion.ID != question.ID {
				t.Fatal("saved question was not supplied after reload", err)
			}
			return reliableRoute{Intent: "prepare_tests", Reply: "I'll prepare one example.", Count: 1, Memory: &memoryUpdate{Answer: answerFor(&question, "30 days", false), Facts: []memoryFact{{Kind: "rule", Quote: "30 days"}}}}
		case 2:
			if strings.Contains(request.Messages[1].Content, "vodka") || strings.Contains(request.Messages[1].Content, "lol") {
				t.Fatal("chat leaked into the author")
			}
			var input taskInput
			_ = json.Unmarshal([]byte(request.Messages[1].Content), &input)
			if len(input.QuestionAnswers) != 1 {
				t.Fatal("short answer lost its meaning")
			}
			current := input.CurrentRequest.ID
			return createSuiteCommand{Rules: []PolicyRule{
				{ID: "job", Statement: job, SourceBlockIDs: []string{jobSource}, Evidence: []RuleEvidence{{SourceBlockID: jobSource, Quote: job, Kind: "requirement"}}},
				{ID: "window", Statement: "Items bought within 30 days can be returned.", SourceBlockIDs: []string{current}, Evidence: []RuleEvidence{{SourceBlockID: current, Quote: "30 days", Kind: "requirement"}}},
			}, Tests: testSuiteProposal{Title: "Returns", Summary: "Checks the return window.", SuccessCriteria: "Follow the return window.", Scenarios: []TestScenario{{Input: "Bought 10 days ago.", Expected: "Eligible for return."}}}}
		case 3:
			var input SuiteReviewInput
			if err := json.Unmarshal([]byte(request.Messages[1].Content), &input); err != nil {
				t.Fatal(err)
			}
			if len(input.QuestionAnswers) != 1 || input.QuestionAnswers[0].Question.ID != question.ID {
				t.Fatal("reviewer didn't receive the original question and answer")
			}
			if strings.Contains(request.Messages[1].Content, "vodka") {
				t.Fatal("reviewer inherited casual chat")
			}
			return supportedSuiteReview(input)
		default:
			t.Fatal("unexpected model dispatch")
			return nil
		}
	})
	if stage != 3 || op.Completion == nil || op.State != Completed || len(v.Document.Artifacts) != 1 || v.Document.ConversationState.PendingQuestion.Status != "answered" {
		t.Fatal("journey did not commit tests and state together")
	}
	a := v.Document.Artifacts[0]
	policy := policyFor(v.Document, &a)
	if policy == nil || len(policy.QuestionAnswers) != 1 || !SuiteValidationMatches(a.Validation, a.Blueprint, *policy) {
		t.Fatal("answer meaning was not frozen in the validated policy")
	}
	if _, err := verifiedPolicySources(v.Document, *policy); err != nil {
		t.Fatal(err)
	}
	input, err := BuildSuiteReviewInput(a.Blueprint, *policy, policy.Sources, policy.Sources[0], 1, LimitsFor(true))
	if err != nil || len(input.QuestionAnswers) != 1 {
		t.Fatal("later revalidation lost the short answer context", err)
	}
}
func TestIntegrationVibeConversationStateAtomicity(t *testing.T) {
	ctx := context.Background()
	s, v, _ := memoryService(t)
	o, p := memoryOperation(t, s, v, "My agent converts PDF to Markdown.")
	if _, _, err := s.Store.Start(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	route := questionRoute()
	state, err := proposeConversationState(p, route, o)
	if err != nil {
		t.Fatal(err)
	}
	completion := AuthoringCompletion{ConversationState: state, Outcome: &CompletionReceipt{Action: "clarify"}}
	bad := cloneState(state)
	bad.Brief.Facts[len(bad.Brief.Facts)-1].Sources[0].SHA256 = strings.Repeat("0", 64)
	invalid := completion
	invalid.ConversationState = bad
	if err = s.Store.CompleteDocument(ctx, o.ID, route.Reply, nil, nil, invalid); err == nil {
		t.Fatal("invalid sources committed")
	}
	current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Document.ConversationState != nil || len(current.Document.Messages) != 1 {
		t.Fatal("state or reply partially committed")
	}
	if err = s.Store.CompleteDocument(ctx, o.ID, route.Reply, nil, nil, completion); err != nil {
		t.Fatal(err)
	}
	if err = s.Store.CompleteDocument(ctx, o.ID, route.Reply, nil, nil, completion); err != nil {
		t.Fatal("exact replay failed", err)
	}
	changed := cloneState(state)
	changed.PendingQuestion.Status = "dismissed"
	different := completion
	different.ConversationState = changed
	requireFault(t, s.Store.CompleteDocument(ctx, o.ID, route.Reply, nil, nil, different), "completion_conflict")
	current, err = s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Document.Messages) != 2 || Hash(raw(current.Document.ConversationState)) != Hash(raw(state)) {
		t.Fatal("replay changed saved state or duplicated reply")
	}
	operation, _ := s.Store.Operation(ctx, o.ID)
	if operation.Completion == nil || operation.Completion.EffectHash == "" {
		t.Fatal("state completion has no receipt")
	}
}
func TestIntegrationVibeConversationStateRecoveryWithoutDispatch(t *testing.T) {
	for _, version := range []int{12, 13, 14} {
		t.Run(fmt.Sprint("version=", version), func(t *testing.T) {
			ctx := context.Background()
			s, v, cfg := memoryService(t)
			s.Config.PreciseActions = version >= 13
			s.Config.ContextGuidance = version >= 14
			o, p := memoryOperation(t, s, v, "My agent converts PDF to Markdown.")
			var err error
			o, _, err = s.Store.Start(ctx, o.ID)
			if err != nil {
				t.Fatal(err)
			}
			profile := cfg.Profiles[o.Models.Assistant]
			route := questionRoute()
			if version == 14 {
				route.Example = &GuidanceExample{Input: "PDF heading: Refunds", Expected: "# Refunds"}
			}
			output := raw(route)
			attempt := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "route", Role: Assistant, Model: o.Models.Assistant, RequestHash: Hash(raw(taskMessages(p, taskRoute, "", nil))), Policy: raw(map[string]any{"response_format": reliableRouteFormat(profile, p)}), InputBound: 15000, MaxOutput: 2048, MaxCost: 100000}
			if err = s.Store.BeginAttempt(ctx, attempt); err != nil {
				t.Fatal(err)
			}
			cost := int64(100)
			if err = s.Store.EndAttempt(ctx, attempt, string(output), raw(provider.Response{OutputText: string(output)}), &cost, nil); err != nil {
				t.Fatal(err)
			}
			calls := 0
			r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: cfg, Gate: s.Gate, Client: callFunc(func(context.Context, provider.Request) (provider.Response, error) {
				calls++
				return provider.Response{}, fmt.Errorf("unexpected paid dispatch")
			})}}
			issue := &Fault{Code: "worker_interrupted", Message: "Worker interrupted."}
			if err = r.Finalize(ctx, o.ID, issue); err != nil {
				t.Fatal(err)
			}
			if err = r.Finalize(ctx, o.ID, issue); err != nil {
				t.Fatal(err)
			}
			current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 0 || version == 14 && len(current.Document.Messages[len(current.Document.Messages)-1].Cards) != 1 || len(current.Document.Messages) != 2 || current.Document.ConversationState == nil || current.Document.ConversationState.PendingQuestion == nil || current.Operations[0].State != Completed {
				t.Fatal("recorded response did not recover state exactly once")
			}

		})
	}
}

func TestIntegrationVibeConversationStateConflictAndStop(t *testing.T) {
	for _, mode := range []string{"conflict", "stop"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			s, v, _ := memoryService(t)
			o, p := memoryOperation(t, s, v, "My agent converts PDF to Markdown.")
			if _, _, err := s.Store.Start(ctx, o.ID); err != nil {
				t.Fatal(err)
			}
			route := questionRoute()
			state, err := proposeConversationState(p, route, o)
			if err != nil {
				t.Fatal(err)
			}
			completion := AuthoringCompletion{ConversationState: state, Outcome: &CompletionReceipt{Action: "clarify"}}
			if mode == "stop" {
				if err = s.Store.Stop(ctx, v.Actor, o.ID); err != nil {
					t.Fatal(err)
				}
				requireFault(t, s.Store.CompleteDocument(ctx, o.ID, route.Reply, nil, nil, completion), "operation_stopped")
			} else {
				other := newConversationState(uuid.New())
				if _, err = s.Store.DB.Exec(ctx, "UPDATE vibe_sessions SET document=jsonb_set(document,'{conversation_state}',$2::jsonb) WHERE id=$1", v.ID, raw(other)); err != nil {
					t.Fatal(err)
				}
				requireFault(t, s.Store.CompleteDocument(ctx, o.ID, route.Reply, nil, nil, completion), "conversation_state_conflict")
			}
			current, _ := s.Store.GetSession(ctx, v.Actor, v.ID)
			if len(current.Document.Messages) != 1 {
				t.Fatal("stopped or stale response was displayed")
			}
		})
	}
}
