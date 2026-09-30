package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Scripted semantic verdicts exercise the real admission, compiler, journal and
// transaction boundary. They do not establish a provider's semantic accuracy.
type reliabilityClient struct {
	t        *testing.T
	count    int
	mode     string
	boundary bool
	calls    []string
	reviews  []vibe.SuiteReviewInput
	requests []vibe.SourceBlock
	contexts []json.RawMessage
}

func (f *reliabilityClient) InvokeModel(_ context.Context, req provider.Request) (provider.Response, error) {
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
	var output any
	if name == "vibe_suite_review_v1" {
		var input vibe.SuiteReviewInput
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
			return provider.Response{}, err
		}
		f.reviews = append(f.reviews, input)
		if f.mode == "unavailable" {
			return provider.Response{OutputText: `{}`, Usage: reliabilityUsage()}, nil
		}
		finding := func(ids []string) vibe.SuiteReviewFinding {
			return vibe.SuiteReviewFinding{Status: vibe.SuiteSupported, RuleIDs: ids, SourceBlockIDs: []string{input.Sources[0].ID}, Reason: "The scenario and expected behavior follow the supplied policy."}
		}
		ids := []string{}
		rules := []vibe.SuiteRuleReview{}
		for _, rule := range input.Policy.Rules {
			ids = append(ids, rule.ID)
			rules = append(rules, vibe.SuiteRuleReview{RuleID: rule.ID, SuiteReviewFinding: finding([]string{rule.ID})})
		}
		cases := []vibe.SuiteCaseReview{}
		for i, c := range input.Cases {
			entry := vibe.SuiteCaseReview{CaseKey: c.CaseKey, SuiteReviewFinding: finding(ids)}
			if (f.mode == "policy_reject" || f.mode == "manual_reject") && i == 0 {
				entry.Status = vibe.SuiteContradicted
				entry.Reason = "The actual input cannot receive this eligibility outcome under the supplied rule."
			}
			cases = append(cases, entry)
		}
		output = map[string]any{"cases": cases, "rules": rules, "shared_criteria": finding(ids), "policy_reconciliation": finding(ids)}
	} else {
		f.contexts = append(f.contexts, json.RawMessage(req.Messages[1].Content))
		var input struct {
			Request  vibe.SourceBlock     `json:"current_request"`
			Sources  []vibe.SourceBlock   `json:"source_blocks"`
			Policy   *vibe.PolicySnapshot `json:"desired_rules"`
			Artifact *vibe.Artifact       `json:"selected_tests"`
			Server   struct {
				Repair          bool                 `json:"repair_candidate"`
				CandidatePolicy *vibe.PolicySnapshot `json:"candidate_policy"`
			} `json:"server_context"`
		}
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
			return provider.Response{}, err
		}
		f.requests = append(f.requests, input.Request)
		if req.Messages[len(req.Messages)-1].Content != input.Request.Text {
			f.t.Fatal("repair or routing replaced the current user request")
		}
		switch name {
		case "vibe_route_v11":
			intent, count := "prepare_tests", f.count
			if f.mode == "policy_reject" {
				intent, count = "edit_tests", 0
			} else if f.mode == "suggest_fix" {
				intent, count = "suggest_fix", 0
			}
			output = map[string]any{"intent": intent, "count": count, "reply": "I will prepare the requested tests."}
		case "vibe_prepare_tests_v11":
			rules := []vibe.PolicyRule{
				{ID: "job", Statement: "Answer shop return questions.", SourceBlockIDs: []string{input.Request.ID}},
				{ID: "window", Statement: "The return window is 30 days.", SourceBlockIDs: []string{input.Request.ID}},
				{ID: "condition", Statement: "Only unopened items are eligible.", SourceBlockIDs: []string{input.Request.ID}},
				{ID: "missing", Statement: "Ask only for missing purchase age or item condition.", SourceBlockIDs: []string{input.Request.ID}},
				{ID: "no-refund", Statement: "Never claim to process a refund.", SourceBlockIDs: []string{input.Request.ID}},
			}
			age := 10
			if f.boundary {
				age = 30
			}
			cases := []vibe.TestScenario{{Input: fmt.Sprintf("I bought an unopened item exactly %d days ago. Can I return it?", age), Expected: "Confirm eligibility without claiming to process a refund."}, {Input: "I opened the item bought 10 days ago. Can I return it?", Expected: "Explain that opened items are ineligible."}, {Input: "Can I return an item?", Expected: "Ask only for purchase age and condition."}}
			if f.count == 5 {
				cases = append(cases, vibe.TestScenario{Input: "An unopened item was bought 45 days ago.", Expected: "Explain that it is outside the return window."}, vibe.TestScenario{Input: "I bought the item 10 days ago but have not said its condition.", Expected: "Ask only for the condition."})
			}
			output = map[string]any{"rules": rules, "tests": map[string]any{"title": "Return checks", "summary": "Eligibility and missing details.", "success_criteria": "Only unopened purchases within 30 days are eligible. Ask only for missing age or condition. Never claim to process a refund.", "scenarios": cases}}
		case "vibe_edit_tests_v11":
			if input.Policy == nil || input.Artifact == nil {
				return provider.Response{}, fmt.Errorf("edit fixture lost policy or artifact")
			}
			rules := append([]vibe.PolicyRule(nil), input.Policy.Rules...)
			for i := range rules {
				if rules[i].ID == "window" {
					rules[i].Statement = "The return window is now 14 days."
					rules[i].SourceBlockIDs = append(append([]string(nil), rules[i].SourceBlockIDs...), input.Request.ID)
				}
			}
			expected := "Confirm eligibility for an unopened item bought exactly 14 days ago."
			var criteria *string
			if input.Server.Repair {
				rules = input.Server.CandidatePolicy.Rules
				expected = "Confirm that the unopened purchase at 14 days is eligible."
			} else {
				value := "Only unopened purchases within 14 days are eligible. Ask only for missing age or condition. Never claim to process a refund."
				criteria = &value
			}
			output = map[string]any{"rules": rules, "criteria": criteria, "case_changes": []vibe.CaseChange{{Action: "update", CaseKey: "case-1", Expected: &expected}}}
		case "vibe_suggest_fix_v11":
			output = map[string]any{"instruction_edits": []map[string]string{{"before": "Opened items are eligible.", "after": "Only unopened items are eligible."}}}
		default:
			return provider.Response{}, fmt.Errorf("unexpected paid-role call in deterministic fixture: %s", name)
		}
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return provider.Response{}, err
	}
	return provider.Response{OutputText: string(encoded), Usage: reliabilityUsage()}, nil
}

func reliabilityUsage() provider.Usage {
	zero := json.Number("0")
	return provider.Usage{InputTokens: 100, OutputTokens: 200, CostUSD: &zero}
}

type reliabilityHarness struct {
	t       *testing.T
	ctx     context.Context
	db      *pgxpool.Pool
	svc     *vibe.Service
	runner  *vibe.Runner
	fake    *reliabilityClient
	session vibe.Session
	actor   string
	user    uuid.UUID
}

func newReliabilityHarness(t *testing.T, count int) *reliabilityHarness {
	t.Helper()
	dsn := os.Getenv("VIBE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated migrated VIBE_TEST_DATABASE_URL")
	}
	if !strings.Contains(dsn, "vibe_test") {
		t.Fatal("refusing a non-test database")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	model := "liquid/lfm-2.5-2.6b:free"
	cfg := vibe.Config{Enabled: true, FreeOnly: true, LocalTesting: true, Credential: "fake-no-network", DefaultModel: model, Campaign: uuid.NewString(), AnonymousDaily: vibe.NanoUSD, AnonymousCampaign: 5 * vibe.NanoUSD, Profiles: map[string]vibe.ModelProfile{model: {ID: model, Route: "liquid/fp8", Free: true, Conformed: true, StructuredOutputs: true, Context: 65536, FramingAllowance: 4096, ExpiresAt: time.Now().Add(time.Hour)}}}
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	store := vibe.NewStore(db, cfg)
	svc := &vibe.Service{Store: store, Config: cfg, Gate: vibe.Gate{Redis: rc}, Compiler: VibePackCompiler{}}
	user := uuid.New()
	if _, err = db.Exec(ctx, "INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", user, user.String(), user.String()+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	actor := "user:" + user.String()
	session, err := store.CreateSession(ctx, actor, nil, uuid.New(), cfg.DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	fake := &reliabilityClient{t: t, count: count}
	h := &reliabilityHarness{t: t, ctx: ctx, db: db, svc: svc, session: session, fake: fake, actor: actor, user: user}
	h.runner = &vibe.Runner{Service: svc, Gateway: &vibe.Gateway{Store: store, Config: cfg, Gate: svc.Gate, Client: fake}}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM vibe_attempts WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)", session.ID)
	})
	return h
}

func (h *reliabilityHarness) reload() {
	h.t.Helper()
	var err error
	h.session, err = h.svc.Store.GetSession(h.ctx, h.actor, h.session.ID)
	if err != nil {
		h.t.Fatal(err)
	}
}

func (h *reliabilityHarness) executeOperation(id uuid.UUID) (vibe.Operation, error) {
	h.t.Helper()
	runErr := h.runner.Execute(h.ctx, id)
	var issue *vibe.Fault
	if runErr != nil && !errors.As(runErr, &issue) {
		issue = &vibe.Fault{Code: "fixture_failure", Message: runErr.Error()}
	}
	if err := h.svc.Store.Finish(h.ctx, id, issue); err != nil {
		h.t.Fatal(err)
	}
	h.reload()
	for _, op := range h.session.Operations {
		if op.ID == id {
			return op, runErr
		}
	}
	h.t.Fatal("operation disappeared")
	return vibe.Operation{}, runErr
}

func (h *reliabilityHarness) send(message string, selected *uuid.UUID) (vibe.Operation, error) {
	h.t.Helper()
	op, err := h.svc.Prepare(h.ctx, h.actor, h.session.ID, vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "message", Content: message, TestJourney: true, Models: h.svc.Config.DefaultModels(), ArtifactID: selected})
	if err != nil {
		return op, err
	}
	var plan vibe.Plan
	if err := json.Unmarshal(op.Input, &plan); err != nil {
		h.t.Fatal(err)
	}
	if plan.AuthoringVersion != 11 || plan.Calls != 5 {
		h.t.Fatalf("v11 was not admitted with its complete call allowance: %+v", plan)
	}
	return h.executeOperation(op.ID)
}

func reliabilityOriginalRequest(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../vibe/testdata/reliability/returns-original-request.txt")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func testReliabilityFixViewedPolicy(t *testing.T, legacy bool) {
	h := newReliabilityHarness(t, 3)
	if _, err := h.send(reliabilityOriginalRequest(t), nil); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Store.Edit(h.ctx, h.actor, h.session.ID, h.session.Revision, func(s *vibe.Session) error {
		s.Document.Artifacts[0].AgentPrompt = "Answer return questions. The window is 30 days. Opened items are eligible. Never claim to process a refund."
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h.reload()
	original := h.session.Document.Artifacts[0]
	oldPolicy := h.session.Document.Policies[0]
	if legacy {
		// Freeze an old pre-upgrade check without semantic metadata. The
		// session artifact is hydrated below as if preflight reviewed it;
		// the historical operation input must remain unchanged.
		if err := h.svc.Store.Edit(h.ctx, h.actor, h.session.ID, h.session.Revision, func(s *vibe.Session) error {
			a := &s.Document.Artifacts[0]
			a.Provenance, a.PolicyID, a.Validation = "", nil, nil
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		h.reload()
	}
	baseline, err := h.svc.Prepare(h.ctx, h.actor, h.session.ID, vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "check", ArtifactID: &original.ID, ApproveArtifact: true, Models: h.svc.Config.DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	// Seed a recorded behavioral failure through the real result store. This
	// fixture deliberately does not make target or evaluator provider calls.
	if _, _, err := h.svc.Store.Start(h.ctx, baseline.ID); err != nil {
		t.Fatal(err)
	}
	var checked vibe.Plan
	if err := json.Unmarshal(baseline.Input, &checked); err != nil {
		t.Fatal(err)
	}
	for _, key := range checked.Cases {
		result := vibe.CaseResult{CaseKey: key, Version: original.ID.String(), Input: json.RawMessage(`{"question":"I opened an item bought 10 days ago."}`), Output: "You can return the opened item.", Verdict: vibe.Fail, ExpectedChecks: 1, Checks: []vibe.CheckResult{{Key: "behavior", Verdict: vibe.Fail, Evidence: "Opened items are ineligible under the original rule."}}}
		if err := h.svc.Store.PutResult(h.ctx, baseline.ID, result); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.svc.Store.Finish(h.ctx, baseline.ID, nil); err != nil {
		t.Fatal(err)
	}
	h.reload()
	newID, newSource := uuid.New(), uuid.New()
	if err := h.svc.Store.Edit(h.ctx, h.actor, h.session.ID, h.session.Revision, func(s *vibe.Session) error {
		if legacy {
			s.Document.Artifacts[0].PolicyID, s.Document.Artifacts[0].Validation = original.PolicyID, original.Validation
		}
		policy := oldPolicy
		policy.ID, policy.ParentID, policy.SourceMessageID = uuid.New(), &oldPolicy.ID, newSource
		policy.Rules = append([]vibe.PolicyRule(nil), oldPolicy.Rules...)
		for i := range policy.Rules {
			if policy.Rules[i].ID == "window" {
				policy.Rules[i].Statement = "The return window is now 14 days."
				policy.Rules[i].SourceBlockIDs = []string{newSource.String()}
			}
		}
		s.Document.Policies = append(s.Document.Policies, policy)
		s.Document.Messages = append(s.Document.Messages, vibe.Message{ID: newSource, Role: "user", Content: "The policy for the new suite is 14 days.", CreatedAt: time.Now()})
		newer := original
		newer.ID, newer.PolicyID, newer.Validation, newer.SourceMessageID = newID, &policy.ID, nil, newSource
		s.Document.Artifacts = append(s.Document.Artifacts, newer)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h.reload()
	h.fake.mode = "suggest_fix"
	fixed, err := h.svc.Prepare(h.ctx, h.actor, h.session.ID, vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "message", Content: "Fix the opened-item failure in the original result.", Purpose: "suggest_change", TestJourney: true, ArtifactID: &newID, BaselineID: &baseline.ID, Models: h.svc.Config.DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	op, err := h.executeOperation(fixed.ID)
	if err != nil || op.ModelCalls != 2 {
		t.Fatalf("instruction fix did not complete in its narrow branch: %+v %v", op, err)
	}
	var handler struct {
		Policy   *vibe.PolicySnapshot `json:"desired_rules"`
		Sources  []vibe.SourceBlock   `json:"source_blocks"`
		Selected *vibe.Artifact       `json:"selected_tests"`
	}
	if err := json.Unmarshal(h.fake.contexts[len(h.fake.contexts)-1], &handler); err != nil {
		t.Fatal(err)
	}
	if handler.Policy == nil || handler.Policy.ID != oldPolicy.ID || handler.Selected == nil || handler.Selected.ID != original.ID {
		t.Fatalf("fix was instructed by a different version's policy: %+v", handler)
	}
	for _, source := range handler.Sources {
		if source.MessageID == newSource {
			t.Fatal("newer policy leaked into the original-run instruction repair")
		}
	}
	last := h.session.Document.Artifacts[len(h.session.Document.Artifacts)-1]
	if !bytes.Equal(last.Blueprint, original.Blueprint) || last.PolicyID == nil || *last.PolicyID != oldPolicy.ID || !vibe.SuiteValidationMatches(last.Validation, last.Blueprint, oldPolicy) || !strings.Contains(last.AgentPrompt, "Only unopened items are eligible.") {
		t.Fatal("same-tests fix did not preserve the original suite and its validation")
	}
}
