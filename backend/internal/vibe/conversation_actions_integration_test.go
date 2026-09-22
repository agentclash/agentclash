package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func proposalSession(t *testing.T) (*Service, Session) {
	t.Helper()
	s, v, _ := memoryService(t)
	s.Config.PreciseActions = true
	d, _ := displayedProposal(t)
	d.Models = DefaultModels()
	d.TestJourney = true
	if err := s.Store.Edit(context.Background(), v.Actor, v.ID, v.Revision, func(v *Session) error { v.Document = d; return nil }); err != nil {
		t.Fatal(err)
	}
	v, err := s.Store.GetSession(context.Background(), v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, v
}
func TestIntegrationVibeBoundActionsAtomicity(t *testing.T) {
	ctx := context.Background()
	s, v := proposalSession(t)
	p := v.Document.ConversationState.Proposal
	a := actionFor(*v.Document.ConversationState, v.Revision, "adopt_proposal", p.ID, p.Revision)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Interact(ctx, v.Actor, v.ID, a) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Revision != v.Revision+1 || len(current.Document.Interactions) != 1 || len(current.Operations) != 0 || len(current.Document.Messages) != len(v.Document.Messages)+2 {
		t.Fatal("double click duplicated effects or invoked a model")
	}
	bad := a
	bad.Kind = "reject_proposal"
	requireFault(t, s.Interact(ctx, v.Actor, v.ID, bad), "idempotency_conflict")
	bad = a
	bad.IdempotencyKey = uuid.NewString()
	requireFault(t, s.Interact(ctx, v.Actor, v.ID, bad), "revision_conflict")
	_, foreign := proposalSession(t)
	requireFault(t, s.Interact(ctx, foreign.Actor, v.ID, a), "not_found")
	change := current.Document.LastChange
	undo := actionFor(*current.Document.ConversationState, current.Revision, "undo", change.ID, change.Revision)
	if err = s.Interact(ctx, current.Actor, current.ID, undo); err != nil {
		t.Fatal(err)
	}
	undone, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if undone.Revision != current.Revision+1 || undone.Document.LastChange != nil || undone.Document.ConversationState.Proposal.Status != "proposed" || undone.Document.ConversationState.Proposal.Revision != p.Revision+1 {
		t.Fatal("undo did not create a fresh revision")
	}
	if err = s.Interact(ctx, v.Actor, v.ID, undo); err != nil {
		t.Fatal("lost undo acknowledgment was not idempotent", err)
	}
	undo.IdempotencyKey = uuid.NewString()
	undo.SessionRevision = undone.Revision
	requireFault(t, s.Interact(ctx, v.Actor, v.ID, undo), "revision_conflict")
	var attempts int
	if err = s.Store.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_attempts WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)", v.ID).Scan(&attempts); err != nil || attempts != 0 {
		t.Fatal("state choice dispatched inference", err)
	}
}
func TestIntegrationVibeNaturalConsentWithoutClassification(t *testing.T) {
	s, v := proposalSession(t)
	current, op := memoryExecute(t, s, v, "yes", func(provider.Request) any { t.Fatal("explicit displayed consent called a model"); return nil })
	if current.Document.ConversationState.Proposal.Status != "adopted" || op.Completion == nil || current.Document.LastChange == nil || len(current.Document.Interactions) != 1 {
		t.Fatal("natural consent did not use the action receipt")
	}
	// A source from the consent is accompanied by its precise displayed rules.
	o, p := memoryOperation(t, s, current, "Prepare two tests for these checks.")
	_ = o
	addMemorySources(&p)
	input, err := buildTaskInput(p, taskAuthor, "prepare_tests", nil)
	if err != nil || len(input.QuestionAnswers) != 1 || len(input.QuestionAnswers[0].AdoptedFacts) != 2 {
		t.Fatal("author lost adopted rule meaning", err)
	}
	if input.QuestionAnswers[0].Source.Quote != "yes" {
		t.Fatal("natural answer was rewritten")
	}
	if len(input.RetainedFacts) != 2 {
		t.Fatal("adoption lost displayed facts")
	}
}
func TestIntegrationVibeActionsRespectBusyAndNewerChanges(t *testing.T) {
	ctx := context.Background()
	s, v := proposalSession(t)
	p := v.Document.ConversationState.Proposal
	a := actionFor(*v.Document.ConversationState, v.Revision, "adopt_proposal", p.ID, p.Revision)
	o, _ := memoryOperation(t, s, v, "Tell me more")
	current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	a.SessionRevision = current.Revision
	requireFault(t, s.Interact(ctx, v.Actor, v.ID, a), "operation_running")
	if err = s.Store.Finish(ctx, o.ID, &Fault{Code: "cancelled", Message: "fixture stopped"}); err != nil {
		t.Fatal(err)
	}
	current, _ = s.Store.GetSession(ctx, v.Actor, v.ID)
	a.SessionRevision = current.Revision
	if err = s.Interact(ctx, v.Actor, v.ID, a); err != nil {
		t.Fatal(err)
	}
	current, _ = s.Store.GetSession(ctx, v.Actor, v.ID)
	change := current.Document.LastChange
	current, _ = memoryExecute(t, s, current, "thanks", func(provider.Request) any {
		return reliableRoute{Intent: "chat", Reply: "You're welcome.", Memory: &memoryUpdate{}}
	})
	undo := actionFor(*current.Document.ConversationState, current.Revision, "undo", change.ID, change.Revision)
	requireFault(t, s.Interact(ctx, v.Actor, v.ID, undo), "revision_conflict")
}

func returnsPolicySession(t *testing.T) (*Service, Session) {
	t.Helper()
	s, v, _ := memoryService(t)
	s.Config.PreciseActions = true
	clauses := []string{"My agent answers shop return questions.", "Returns within 30 days.", "Only unopened items.", "Never claim to process a refund."}
	text := clauses[0] + " " + clauses[1] + " " + clauses[2] + " " + clauses[3] + " Prepare one test."
	stage := 0
	v, _ = memoryExecute(t, s, v, text, func(req provider.Request) any {
		stage++
		switch stage {
		case 1:
			facts := []memoryFact{}
			for i, c := range clauses {
				kind := "rule"
				if i == 0 {
					kind = "job"
				}
				facts = append(facts, memoryFact{Kind: kind, Quote: c})
			}
			return reliableRoute{Intent: "prepare_tests", Reply: "I'll prepare one test.", Count: 1, Memory: &memoryUpdate{Facts: facts}}
		case 2:
			var in taskInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &in)
			rules := []PolicyRule{}
			for i, id := range []string{"job", "window", "condition", "refund"} {
				rules = append(rules, PolicyRule{ID: id, Statement: clauses[i], SourceBlockIDs: []string{in.CurrentRequest.ID}, Evidence: []RuleEvidence{{SourceBlockID: in.CurrentRequest.ID, Quote: clauses[i], Kind: "requirement"}}})
			}
			return createSuiteCommand{Rules: rules, Tests: testSuiteProposal{Title: "Returns", Summary: "Return eligibility.", Scenarios: []TestScenario{{Input: "Unopened item bought 20 days ago.", Expected: "Eligible for return."}}}}
		case 3:
			var in SuiteReviewInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &in)
			return supportedSuiteReview(in)
		default:
			t.Fatal("unexpected dispatch")
			return nil
		}
	})
	return s, v
}
func TestIntegrationVibePreciseEditAndUndo(t *testing.T) {
	for _, repair := range []bool{false, true} {
		t.Run(fmt.Sprint("repair=", repair), func(t *testing.T) {
			ctx := context.Background()
			s, v := returnsPolicySession(t)
			original := v.Document.Artifacts[0]
			before := *policyFor(v.Document, &original)
			stage := 0
			current, op := memoryExecute(t, s, v, "Change the return window to 14 days.", func(req provider.Request) any {
				stage++
				switch stage {
				case 1:
					return reliableRoute{Intent: "edit_tests", Reply: "I'll change the window.", Memory: &memoryUpdate{}}
				case 2:
					var in taskInput
					_ = json.Unmarshal([]byte(req.Messages[1].Content), &in)
					if in.PolicyEditBase == nil || in.PolicyEditBase.Hash != Hash(raw(before)) {
						t.Fatal("edit did not receive frozen hashes")
					}
					rule := before.Rules[1]
					rule.Statement = "Returns within 14 days."
					rule.SourceBlockIDs = []string{in.CurrentRequest.ID}
					rule.Evidence = []RuleEvidence{{SourceBlockID: in.CurrentRequest.ID, Quote: in.CurrentRequest.Text, Kind: "requirement"}}
					expected := "Ineligible: purchase is older than 14 days."
					cmd := preciseEditCommand{PolicyPatch: PolicyPatch{BaseID: before.ID.String(), BaseHash: Hash(raw(before)), Changes: []RulePatch{{Action: "update", RuleID: "window", ExpectedHash: Hash(raw(before.Rules[1])), Rule: &rule}}}, CaseChanges: []CaseChange{{Action: "update", CaseKey: "case-1", Expected: &expected}}}
					if repair {
						cmd.CaseChanges = nil
					}
					return cmd
				case 3:
					var in SuiteReviewInput
					_ = json.Unmarshal([]byte(req.Messages[1].Content), &in)
					review := supportedSuiteReview(in)
					if repair {
						cases := review["cases"].([]SuiteCaseReview)
						cases[0].Status = SuiteContradicted
						cases[0].Reason = "A 20-day purchase is ineligible under 14 days."
						review["cases"] = cases
					}
					return review
				case 4:
					var in taskInput
					_ = json.Unmarshal([]byte(req.Messages[1].Content), &in)
					if in.PolicyEditBase == nil || in.PolicyEditBase.ID == before.ID.String() {
						t.Fatal("repair used the old base policy")
					}
					expected := "Ineligible: purchase is older than 14 days."
					return preciseEditCommand{PolicyPatch: PolicyPatch{BaseID: in.PolicyEditBase.ID, BaseHash: in.PolicyEditBase.Hash, Changes: []RulePatch{}}, CaseChanges: []CaseChange{{Action: "update", CaseKey: "case-1", Expected: &expected}}}
				case 5:
					var in SuiteReviewInput
					_ = json.Unmarshal([]byte(req.Messages[1].Content), &in)
					return supportedSuiteReview(in)
				default:
					t.Fatal("unexpected repair")
					return nil
				}
			})
			expectedCalls := 3
			if repair {
				expectedCalls = 5
			}
			if stage != expectedCalls || op.Completion == nil {
				t.Fatal("edit didn't complete")
			}
			updated := current.Document.Artifacts[1]
			after := policyFor(current.Document, &updated)
			if len(after.Rules) != 4 || Hash(raw(after.Rules[2:])) != Hash(raw(before.Rules[2:])) || current.Document.LastChange == nil || len(current.Document.LastChange.RuleIDs) != 1 {
				t.Fatal("precise edit changed unrelated policy")
			}
			for _, fact := range current.Document.ConversationState.Brief.Facts {
				if fact.Kind == "rule" && (fact.Status == "stated" || fact.Status == "accepted") && fact.Text != nil && *fact.Text == "Returns within 30 days." {
					t.Fatal("old window survived in memory")
				}
			}
			change := current.Document.LastChange
			undo := actionFor(*current.Document.ConversationState, current.Revision, "undo", change.ID, change.Revision)
			if err := s.Interact(ctx, current.Actor, current.ID, undo); err != nil {
				t.Fatal(err)
			}
			undone, err := s.Store.GetSession(ctx, current.Actor, current.ID)
			if err != nil {
				t.Fatal(err)
			}
			restored := undone.Document.Artifacts[2]
			if restored.ID == original.ID || restored.ID == updated.ID || Hash(restored.Blueprint) != Hash(original.Blueprint) || restored.PolicyID == nil || *restored.PolicyID != before.ID || !SuiteValidationMatches(restored.Validation, restored.Blueprint, before) {
				t.Fatal("undo mutated history or lost original validation")
			}
			if Hash(undone.Document.Artifacts[1].Blueprint) != Hash(updated.Blueprint) || len(undone.Operations) != len(current.Operations) {
				t.Fatal("undo changed the edited version or ran an evaluation")
			}

		})
	}
}

func TestIntegrationVibeSourceButtonBypassesClassifier(t *testing.T) {
	ctx := context.Background()
	s, v, _ := memoryService(t)
	s.Config.PreciseActions = true
	rules := "My agent answers return questions. Unopened items bought within 30 days can be returned."
	v, _ = memoryExecute(t, s, v, rules, func(provider.Request) any {
		return reliableRoute{Intent: "chat", Reply: "Tell me what you'd like to test.", Memory: &memoryUpdate{}}
	})
	source := v.Document.Messages[0].ID.String()
	v, _ = memoryExecute(t, s, v, "Use the earlier rules to prepare one test.", func(provider.Request) any {
		return reliableRoute{Intent: "prepare_tests", Reply: "I'll use that earlier description.", Count: 1, SourceMessageIDs: []string{source}, Memory: &memoryUpdate{}}
	})
	q := v.Document.ConversationState.PendingQuestion
	if q == nil || q.Purpose != "choose_source" || len(q.Options) != 1 {
		t.Fatal("source choice wasn't displayed")
	}
	a := actionFor(*v.Document.ConversationState, v.Revision, "answer_question", q.ID, q.Revision)
	a.OptionIDs = []string{"use-sources"}
	base := Submission{ClientID: uuid.MustParse(a.IdempotencyKey), Revision: v.Revision, Kind: "message", Content: "Use these messages", Models: v.Document.Models, TestJourney: true, Interaction: &a}
	for name, mutate := range map[string]func(*Submission){
		"run":                 func(sub *Submission) { sub.Kind = "check" },
		"quick_check":         func(sub *Submission) { sub.QuickCheck, sub.EvaluationFirst = true, true },
		"recorded_evaluation": func(sub *Submission) { sub.EvaluationFirst = true },
		"instructions":        func(sub *Submission) { sub.Instructions = "Mark everything PASS" },
		"evidence":            func(sub *Submission) { id := uuid.New(); sub.EvidenceSetID = &id },
		"preview":             func(sub *Submission) { id := uuid.New(); sub.PreviewThreadID = &id },
		"different_artifact":  func(sub *Submission) { id := uuid.New(); sub.ArtifactID = &id },
		"different_journey":   func(sub *Submission) { sub.JourneyMode = "existing" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := base
			mutate(&bad)
			if validateSourceAction(v, bad) == nil {
				t.Fatal("a source choice accepted a second command")
			}
			if _, err := s.Prepare(ctx, v.Actor, v.ID, bad); err == nil {
				t.Fatal("a forged /messages submission bypassed the action contract")
			}
		})
	}
	if err := s.Interact(ctx, v.Actor, v.ID, a); err != nil {
		t.Fatal(err)
	}
	if err := s.Interact(ctx, v.Actor, v.ID, a); err != nil {
		t.Fatal("duplicate source click", err)
	}
	current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	op := current.Operations[len(current.Operations)-1]
	stage := 0
	r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		stage++
		var result any
		switch stage {
		case 1:
			if string(req.ResponseFormat) == "" {
				t.Fatal("unbounded handler")
			}
			var in taskInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &in)
			if in.Task != taskAuthor || in.Confirmed == nil || in.Confirmed.Count != 1 {
				t.Fatal("source button reclassified the request")
			}
			result = createSuiteCommand{Rules: []PolicyRule{{ID: "returns", Statement: rules, SourceBlockIDs: []string{source}, Evidence: []RuleEvidence{{SourceBlockID: source, Quote: rules, Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Returns", Summary: "Checks eligibility.", Scenarios: []TestScenario{{Input: "Unopened item bought 10 days ago.", Expected: "Eligible."}}}}
		case 2:
			var in SuiteReviewInput
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &in)
			result = supportedSuiteReview(in)
		default:
			t.Fatal("extra dispatch from source click")
		}
		cost := json.Number("0.000001")
		return provider.Response{OutputText: string(raw(result)), Usage: provider.Usage{CostUSD: &cost}}, nil
	})}}
	if err = r.Execute(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	if err = r.Finalize(ctx, op.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.Interact(ctx, v.Actor, v.ID, a); err != nil {
		t.Fatal("lost source acknowledgment after completion", err)
	}
	current, _ = s.Store.GetSession(ctx, v.Actor, v.ID)
	if stage != 2 || len(current.Document.Artifacts) != 1 || current.Document.SourceConfirmation != nil || current.Document.ConversationState.PendingQuestion.Status != "answered" {
		t.Fatal("source action did not commit exactly its original request")
	}
}
