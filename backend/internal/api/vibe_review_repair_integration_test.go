package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

// These scripts reproduce an ungrounded reviewer, not model quality: the
// reviewer reports a contradiction while sourcing a case fact from policy.
// The real service, runner, compiler, journal and database own every effect.
type reviewRepairClient struct {
	t              *testing.T
	base           *reliabilityClient
	firstReview    string // valid, ungrounded, or malformed
	laterReview    string // valid, ungrounded, malformed, or contradicted
	spendRepairOn  string // route or handler
	repairSpent    bool
	calls          []string
	reviewRequests []provider.Request
	reviewInputs   []vibe.SuiteReviewInput
}

func (f *reviewRepairClient) InvokeModel(ctx context.Context, req provider.Request) (provider.Response, error) {
	var format struct {
		JSONSchema struct {
			Name string `json:"name"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(req.ResponseFormat, &format); err != nil {
		return provider.Response{}, err
	}
	name := format.JSONSchema.Name
	f.calls = append(f.calls, name)
	if !f.repairSpent && (f.spendRepairOn == "route" && name == "vibe_route_v11" || f.spendRepairOn == "handler" && name == "vibe_prepare_tests_v11") {
		f.repairSpent = true
		return provider.Response{OutputText: `{}`, Usage: reliabilityUsage()}, nil
	}
	if name != "vibe_suite_review_v3" {
		if name == "vibe_edit_tests_v11" {
			return provider.Response{}, fmt.Errorf("invalid reviewer evidence authorized a candidate edit")
		}
		return f.base.InvokeModel(ctx, req)
	}
	if len(req.Messages) < 2 {
		return provider.Response{}, fmt.Errorf("review lost original input")
	}
	var input vibe.SuiteReviewInput
	if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
		return provider.Response{}, err
	}
	f.reviewRequests = append(f.reviewRequests, req)
	f.reviewInputs = append(f.reviewInputs, input)
	mode := f.firstReview
	if len(f.reviewInputs) > 1 {
		mode = f.laterReview
	}
	if mode == "malformed" {
		return provider.Response{OutputText: `{}`, Usage: reliabilityUsage()}, nil
	}
	reply := reviewRepairResponse(input)
	if mode == "ungrounded" || mode == "contradicted" {
		cases := reply["cases"].([]vibe.SuiteCaseReview)
		last := &cases[len(cases)-1]
		last.Status = vibe.SuiteContradicted
		last.Reason = "The reviewer claims this request repeats an already supplied fact."
		if mode == "ungrounded" {
			ledger := reply["consistency"].(vibe.ConsistencyLedgerV3)
			facts := ledger.Cases[len(ledger.Cases)-1].Facts
			facts[0].State = "present"
			facts[0].Evidence = "Ask only for missing purchase age or item condition."
			facts[0].Literal = "purchase age"
			reply["consistency"] = ledger
		}
	}
	output, err := json.Marshal(reply)
	if err != nil {
		return provider.Response{}, err
	}
	if mode == "ungrounded" {
		parsed, err := vibe.ParseSuiteReview(output, input, (vibe.Config{LocalTesting: true}).Limits(true))
		if err != nil || parsed.Status != vibe.SuiteContradicted || parsed.Consistency == nil || len(parsed.Consistency.Findings) == 0 {
			f.t.Fatalf("fixture did not reproduce aggregated contradiction with invalid evidence: %+v %v", parsed, err)
		}
	}
	return provider.Response{OutputText: string(output), Usage: reliabilityUsage()}, nil
}

func reviewRepairResponse(input vibe.SuiteReviewInput) map[string]any {
	sourceID, missingRule := "", ""
	ids := []string{}
	for _, rule := range input.Policy.Rules {
		ids = append(ids, rule.ID)
		if strings.Contains(rule.Statement, "Ask only for missing") {
			sourceID, missingRule = rule.SourceBlockIDs[0], rule.ID
		}
	}
	span := func(text string) vibe.ConsistencySourceSpan {
		return vibe.ConsistencySourceSpan{SourceBlockID: sourceID, Text: text}
	}
	finding := func(ruleIDs []string) vibe.SuiteReviewFinding {
		return vibe.SuiteReviewFinding{Status: vibe.SuiteSupported, RuleIDs: ruleIDs, SourceBlockIDs: []string{sourceID}, Reason: "The original source and actual scenario support this expected behavior."}
	}
	ledger := vibe.ConsistencyLedgerV3{
		Entities: []vibe.ConsistencyEntity{{ID: "item", Source: span("items")}},
		Fields: []vibe.ConsistencyField{
			{ID: "age", EntityID: "item", Kind: "number", Aliases: []vibe.ConsistencySourceSpan{span("purchase age")}},
			{ID: "condition", EntityID: "item", Kind: "text", Aliases: []vibe.ConsistencySourceSpan{span("item condition")}},
		},
		MissingOnly: []vibe.MissingOnlyConstraint{{RuleID: missingRule, Source: span("Ask only for missing purchase age or item condition."), FieldIDs: []string{"age", "condition"}}},
	}
	cases := []vibe.SuiteCaseReview{}
	for _, c := range input.Cases {
		cases = append(cases, vibe.SuiteCaseReview{CaseKey: c.CaseKey, SuiteReviewFinding: finding(ids)})
		// Only case-3 asks for information in the fixed generated fixture.
		// Other facts may be unresolved without authorizing unrelated edits.
		state, evidence := "unclear", ""
		if c.CaseKey == "case-3" {
			state, evidence = "missing", "Can I return an item?"
		}
		ledger.Cases = append(ledger.Cases, vibe.CaseFactLedgerV3{CaseKey: c.CaseKey, Facts: []vibe.ConsistencyFact{
			{EntityID: "item", FieldID: "age", State: state, InputPointer: "/question", Evidence: evidence},
			{EntityID: "item", FieldID: "condition", State: state, InputPointer: "/question", Evidence: evidence},
		}})
	}
	rules := []vibe.SuiteRuleReview{}
	for _, rule := range input.Policy.Rules {
		rules = append(rules, vibe.SuiteRuleReview{RuleID: rule.ID, SuiteReviewFinding: finding([]string{rule.ID})})
	}
	return map[string]any{"cases": cases, "rules": rules, "shared_criteria": finding(ids), "policy_reconciliation": finding(ids), "consistency": ledger}
}

func newReviewRepairHarness(t *testing.T) (*reliabilityHarness, *reviewRepairClient) {
	t.Helper()
	h := newReliabilityHarness(t, 3)
	h.svc.Config.SuiteReviewVersion = vibe.LatestSuiteValidatorVersion
	h.runner.Gateway.Config = h.svc.Config
	fake := &reviewRepairClient{t: t, base: h.fake, firstReview: "valid", laterReview: "valid"}
	h.runner.Gateway.Client = fake
	return h, fake
}

func (f *reviewRepairClient) reset(first, later, spendOn string) {
	f.firstReview, f.laterReview, f.spendRepairOn = first, later, spendOn
	f.repairSpent, f.calls, f.reviewInputs, f.reviewRequests = false, nil, nil, nil
}

func assertReviewRepairNoCandidateEdit(t *testing.T, fake *reviewRepairClient) {
	t.Helper()
	for _, name := range fake.calls {
		if name == "vibe_edit_tests_v11" {
			t.Fatal("an ungrounded review caused a candidate-edit call")
		}
	}
	if len(fake.reviewInputs) == 2 {
		first, second := fake.reviewInputs[0], fake.reviewInputs[1]
		if first.BlueprintHash != second.BlueprintHash || first.PolicyHash != second.PolicyHash || !reflect.DeepEqual(first, second) || fake.reviewRequests[0].Messages[1].Content != fake.reviewRequests[1].Messages[1].Content || !bytes.Equal(fake.reviewRequests[0].ResponseFormat, fake.reviewRequests[1].ResponseFormat) {
			t.Fatal("review-only repair changed the candidate, policy, original request or review schema")
		}
		if len(fake.reviewRequests[1].Messages) <= len(fake.reviewRequests[0].Messages) {
			t.Fatal("review repair did not carry separate diagnostic feedback")
		}
	}
}

func TestVibeReviewRepairIntegrationRepairsOnlyUngroundedReview(t *testing.T) {
	for _, first := range []string{"ungrounded", "malformed"} {
		t.Run(first, func(t *testing.T) {
			h, fake := newReviewRepairHarness(t)
			fake.reset(first, "valid", "")
			op, err := h.send(reliabilityOriginalRequest(t), nil)
			if err != nil || op.State != vibe.Completed || op.ModelCalls != 4 || len(h.session.Document.Artifacts) != 1 || op.Completion == nil {
				t.Fatalf("review-only repair failed to commit the unchanged candidate: %+v %v", op, err)
			}
			want := []string{"vibe_route_v11", "vibe_prepare_tests_v11", "vibe_suite_review_v3", "vibe_suite_review_v3"}
			if !reflect.DeepEqual(fake.calls, want) {
				t.Fatalf("unexpected call graph: %v", fake.calls)
			}
			assertReviewRepairNoCandidateEdit(t, fake)
			a := h.session.Document.Artifacts[0]
			hash, err := vibe.CanonicalJSONHash(a.Blueprint)
			if err != nil || hash != fake.reviewInputs[0].BlueprintHash || a.PolicyID == nil || *a.PolicyID != fake.reviewInputs[0].Policy.ID || !vibe.SuiteValidationMatches(a.Validation, a.Blueprint, fake.reviewInputs[0].Policy) {
				t.Fatal("committed candidate differs from the candidate presented to the first review")
			}
			var firstStage, firstStatus, repairStage, repairStatus, firstHash, repairHash string
			err = h.db.QueryRow(h.ctx, `SELECT first.domain_outcome->>'stage',first.domain_outcome->>'status',repair.domain_outcome->>'stage',repair.domain_outcome->>'status',first.domain_outcome->>'candidate_hash',repair.domain_outcome->>'candidate_hash'
 FROM vibe_attempts first JOIN vibe_attempts repair ON repair.operation_id=first.operation_id AND repair.step_key='repair' WHERE first.operation_id=$1 AND first.step_key='review'`, op.ID).Scan(&firstStage, &firstStatus, &repairStage, &repairStatus, &firstHash, &repairHash)
			if err != nil || firstStage != "review" || repairStage != "review" || firstStatus != "rejected" || repairStatus != "accepted" || firstHash != repairHash || firstHash != hash {
				t.Fatalf("review repair lost its rejected/accepted journal or exact candidate: %s/%s %s/%s %v", firstStage, firstStatus, repairStage, repairStatus, err)
			}
		})
	}
}

func TestVibeReviewRepairIntegrationFailureAndExhaustionPreservePreviousSuite(t *testing.T) {
	for _, tc := range []struct{ name, first, later, spendOn, issue string }{
		{"still_ungrounded", "ungrounded", "ungrounded", "", "validation_unavailable"},
		{"still_malformed", "malformed", "malformed", "", "validation_unavailable"},
		{"route_spent_repair", "ungrounded", "valid", "route", "validation_unavailable"},
		{"handler_spent_repair", "ungrounded", "valid", "handler", "validation_unavailable"},
		{"grounded_conflict_after_review_repair", "ungrounded", "contradicted", "", "test_policy_conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, fake := newReviewRepairHarness(t)
			if _, err := h.send(reliabilityOriginalRequest(t), nil); err != nil {
				t.Fatal(err)
			}
			before := mustJSON(t, h.session.Document.Artifacts)
			policies := mustJSON(t, h.session.Document.Policies)
			selected := h.session.Document.Artifacts[0].ID
			fake.reset(tc.first, tc.later, tc.spendOn)
			request := reliabilityOriginalRequest(t) + "Prepare a new suite for these same rules."
			op, err := h.send(request, &selected)
			var issue *vibe.Fault
			if !errors.As(err, &issue) || issue.Code != tc.issue || op.State != vibe.Failed || op.ModelCalls != 4 || op.Completion != nil {
				t.Fatalf("invalid or exhausted review performed a mutation: %+v %v", op, err)
			}
			assertReviewRepairNoCandidateEdit(t, fake)
			if !bytes.Equal(before, mustJSON(t, h.session.Document.Artifacts)) || !bytes.Equal(policies, mustJSON(t, h.session.Document.Policies)) {
				t.Fatal("failed review changed the previous suite or effective policy")
			}
			last := h.session.Document.Messages[len(h.session.Document.Messages)-1]
			if last.Role != "user" || last.Content != request {
				t.Fatal("failure discarded the original request or fabricated a completion")
			}
			expectedReviews := 2
			if tc.spendOn != "" {
				expectedReviews = 1
			}
			if len(fake.reviewInputs) != expectedReviews {
				t.Fatalf("shared repair allowance was exceeded: %v", fake.calls)
			}
		})
	}
}

func TestVibeReviewRepairIntegrationManualEditCannotRepairOrApplyUngroundedReview(t *testing.T) {
	h, fake := newReviewRepairHarness(t)
	if _, err := h.send(reliabilityOriginalRequest(t), nil); err != nil {
		t.Fatal(err)
	}
	before := h.session.Document.Artifacts[0]
	policies := mustJSON(t, h.session.Document.Policies)
	expected := "Ask for purchase age. Ask for item condition."
	blueprint, err := vibe.PatchTestSuite(before.Blueprint, []vibe.CaseChange{{Action: "update", CaseKey: "case-3", Expected: &expected}}, nil, h.svc.Config.Limits(h.session.Anonymous))
	if err != nil {
		t.Fatal(err)
	}
	fake.reset("ungrounded", "valid", "")
	op, err := h.svc.PrepareSuiteEdit(h.ctx, h.actor, h.session.ID, h.session.Revision, before.ID, blueprint)
	if err != nil {
		t.Fatal(err)
	}
	var plan vibe.Plan
	if err := json.Unmarshal(op.Input, &plan); err != nil || plan.Calls != 1 || plan.Conversation == nil || plan.Conversation.ValidatorVersion != vibe.LatestSuiteValidatorVersion {
		t.Fatalf("manual edit did not retain its single-call plan: %+v %v", plan, err)
	}
	op, err = h.executeOperation(op.ID)
	var issue *vibe.Fault
	if !errors.As(err, &issue) || issue.Code != "validation_unavailable" || op.ModelCalls != 1 || op.State != vibe.Failed || op.Completion != nil || len(h.session.Document.Artifacts) != 1 {
		t.Fatalf("manual edit exceeded its allowance or accepted invalid evidence: %+v %v", op, err)
	}
	if !bytes.Equal(before.Blueprint, h.session.Document.Artifacts[0].Blueprint) || !bytes.Equal(policies, mustJSON(t, h.session.Document.Policies)) || !reflect.DeepEqual(fake.calls, []string{"vibe_suite_review_v3"}) {
		t.Fatal("manual review changed the prior suite or dispatched an extra call")
	}
	if len(h.session.Document.PendingPolicyChanges) != 1 || h.session.Document.PendingPolicyChanges[0].Status == "applied" || h.session.Document.PendingPolicyChanges[0].OperationID != op.ID {
		t.Fatal("manual edit failure did not retain its pending request")
	}
	if op.ID == uuid.Nil {
		t.Fatal("manual operation identity disappeared")
	}
}
