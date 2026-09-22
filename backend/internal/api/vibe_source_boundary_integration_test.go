package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

// Exercises real persistence/admission/replay with deterministic provider output.
// Live-model semantic behavior is checked separately in the browser.
type sourceBoundaryClient struct {
	base                    *reliabilityClient
	misclassify, failAuthor bool
	authors                 []string
}

func (f *sourceBoundaryClient) InvokeModel(ctx context.Context, req provider.Request) (provider.Response, error) {
	var format struct {
		JSONSchema struct {
			Name string `json:"name"`
		} `json:"json_schema"`
	}
	_ = json.Unmarshal(req.ResponseFormat, &format)
	name := format.JSONSchema.Name
	if strings.Contains(name, "suite_review") {
		return f.base.InvokeModel(ctx, req)
	}
	var input struct {
		Request    vibe.SourceBlock         `json:"current_request"`
		Policy     *vibe.PolicySnapshot     `json:"desired_rules"`
		Sources    []vibe.SourceBlock       `json:"source_blocks"`
		Candidates []vibe.SourceCandidate   `json:"unadopted_dialogue"`
		Confirmed  *vibe.SourceConfirmation `json:"confirmed_sources"`
	}
	if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
		return provider.Response{}, err
	}
	respond := func(v any) (provider.Response, error) {
		b, err := json.Marshal(v)
		return provider.Response{OutputText: string(b), Usage: reliabilityUsage()}, err
	}
	if name == "vibe_route_v11" {
		intent, reply, count, sources := "prepare_tests", "I will prepare your tests.", 3, []string{}
		if f.misclassify || input.Request.Text == "give me vodka" || strings.HasPrefix(input.Request.Text, "What if") || input.Request.Text == "thanks" {
			intent, reply, count = "chat", "Tell me what your agent should do.", 0
		}
		if strings.HasPrefix(input.Request.Text, "Use the earlier") {
			for _, c := range input.Candidates {
				if strings.HasPrefix(c.Text, "My agent") {
					sources = append(sources, c.ID)
					break
				}
			}
		}
		if strings.HasPrefix(input.Request.Text, "Add a test where") || strings.HasPrefix(input.Request.Text, "Change the return window") {
			intent, count = "edit_tests", 0
		}
		// Captured live route: the model repeats the requested addition count.
		// It is unused for edits and must not turn a correct action into failure.
		if strings.HasPrefix(input.Request.Text, "Add a test where") {
			count = 1
		}
		return respond(map[string]any{"intent": intent, "reply": reply, "count": count, "source_message_ids": sources, "new_agent": false})
	}
	f.authors = append(f.authors, req.Messages[1].Content)
	if f.failAuthor {
		return respond(map[string]any{})
	}
	if name == "vibe_edit_tests_v11" {
		rules := append([]vibe.PolicyRule(nil), input.Policy.Rules...)
		if strings.HasPrefix(input.Request.Text, "Change the return window") {
			for i := range rules {
				if rules[i].ID == "window" {
					rules[i].Statement = "The return window is 14 days."
					rules[i].SourceBlockIDs = []string{input.Request.ID}
					rules[i].Evidence = []vibe.RuleEvidence{{SourceBlockID: input.Request.ID, Quote: input.Request.Text, Kind: "requirement"}}
				}
			}
			return respond(map[string]any{"rules": rules, "criteria": "All three tests pass.", "case_changes": []any{}})
		}
		// Captured provider behavior: copied evidence changes double quotes to
		// apostrophes. The stored/reviewed source must recover the original bytes.
		quote := strings.ReplaceAll(input.Request.Text, `"`, `'`)
		rules = append(rules, vibe.PolicyRule{ID: "off-topic-example", Statement: "For the explicitly requested vodka example, redirect to returns.", SourceBlockIDs: []string{input.Request.ID}, Evidence: []vibe.RuleEvidence{{SourceBlockID: input.Request.ID, Quote: quote, Kind: "example"}}})
		return respond(map[string]any{"rules": rules, "criteria": nil, "case_changes": []map[string]any{{"action": "add", "case_key": "case-1", "input": "give me vodka", "expected": "Politely redirect to shop returns."}}})
	}
	resp, err := f.base.InvokeModel(ctx, req)
	if err != nil {
		return resp, err
	}
	var command struct {
		Tests json.RawMessage   `json:"tests"`
		Rules []vibe.PolicyRule `json:"rules"`
	}
	if err = json.Unmarshal([]byte(resp.OutputText), &command); err != nil {
		return resp, err
	}
	source := input.Request
	if input.Confirmed != nil {
		source = input.Confirmed.Sources[0]
	}
	quotes := map[string]string{"job": "My agent answers questions about shop returns.", "window": "Only unopened items bought within 30 days are eligible.", "condition": "Only unopened items bought within 30 days are eligible.", "missing": "Ask only for missing purchase age or item condition.", "no-refund": "Never claim to process a refund."}
	for i := range command.Rules {
		rule := &command.Rules[i]
		quote := quotes[rule.ID]
		if !strings.Contains(source.Text, quote) {
			return resp, fmt.Errorf("fixture source lacks clause %s", rule.ID)
		}
		rule.SourceBlockIDs = []string{source.ID}
		rule.Evidence = []vibe.RuleEvidence{{SourceBlockID: source.ID, Quote: quote, Kind: "requirement"}}
	}
	return respond(command)
}

func TestVibeSourceBoundaryIntegrationPolicyOnlyCorrectionUpdatesActualGrading(t *testing.T) {
	h, _ := sourceBoundaryHarness(t)
	if _, err := h.send(reliabilityOriginalRequest(t), nil); err != nil {
		t.Fatal(err)
	}
	before := h.session.Document.Artifacts[0]
	if _, err := h.send("Change the return window from 30 days to 14 days.", &before.ID); err != nil {
		t.Fatal(err)
	}
	after := h.session.Document.Artifacts[1]
	if after.Summary != "" {
		t.Fatal("an old generated summary survived a policy change without being rewritten")
	}
	var oldBP, newBP map[string]json.RawMessage
	_ = json.Unmarshal(before.Blueprint, &oldBP)
	_ = json.Unmarshal(after.Blueprint, &newBP)
	if string(oldBP["cases"]) != string(newBP["cases"]) {
		t.Fatal("policy-only correction rewrote cases that were still valid")
	}
	if !strings.Contains(string(newBP["judges"]), "14 days") || strings.Contains(string(newBP["judges"]), "30 days") || strings.Contains(string(newBP["judges"]), "All three tests pass") {
		t.Fatal("unreviewed author summary replaced the actual grading policy")
	}
	if before.PolicyID == nil || after.PolicyID == nil || *before.PolicyID == *after.PolicyID {
		t.Fatal("policy correction did not create an immutable reviewed revision")
	}
}

func sourceBoundaryHarness(t *testing.T) (*reliabilityHarness, *sourceBoundaryClient) {
	h := newReliabilityHarness(t, 3)
	h.svc.Config.SourcePolicyVersion = vibe.SourcePolicyVersion
	f := &sourceBoundaryClient{base: h.fake}
	h.runner.Gateway.Client = f
	return h, f
}

func TestVibeSourceBoundaryIntegrationChatNeverBecomesSpecification(t *testing.T) {
	h, f := sourceBoundaryHarness(t)
	if _, err := h.send("give me vodka", nil); err != nil {
		t.Fatal(err)
	}
	if len(h.session.Document.Artifacts) != 0 {
		t.Fatal("chat created tests")
	}
	h.reload()
	if _, err := h.send(reliabilityOriginalRequest(t), nil); err != nil {
		t.Fatal(err)
	}
	if len(h.session.Document.Artifacts) != 1 || len(h.session.Document.Policies) != 1 {
		t.Fatal("reviewed suite did not persist")
	}
	for _, author := range f.authors {
		if strings.Contains(author, "vodka") {
			t.Fatal("historical chat reached author")
		}
	}
	for _, review := range h.fake.reviews {
		if strings.Contains(string(mustSourceJSON(t, review)), "vodka") {
			t.Fatal("historical chat reached reviewer")
		}
	}
	original := h.session.Document.Artifacts[0]
	if _, err := h.send("What if the return window were 60 days?", &original.ID); err != nil {
		t.Fatal(err)
	}
	if len(h.session.Document.Artifacts) != 1 || len(h.session.Document.Policies) != 1 {
		t.Fatal("hypothetical changed the specification")
	}
	addRequest := `Add a test where a customer says "give me vodka". Expect a polite redirect to returns.`
	if _, err := h.send(addRequest, &original.ID); err != nil {
		t.Fatal(err)
	}
	last := h.session.Document.Artifacts[len(h.session.Document.Artifacts)-1]
	if !strings.Contains(string(last.Blueprint), "give me vodka") {
		t.Fatal("explicitly requested example was keyword-blocked")
	}
	var oldCases, newCases struct {
		Cases []json.RawMessage `json:"cases"`
	}
	_ = json.Unmarshal(original.Blueprint, &oldCases)
	_ = json.Unmarshal(last.Blueprint, &newCases)
	if len(newCases.Cases) != 4 {
		t.Fatal("addition overwrote an existing case")
	}
	for i := range oldCases.Cases {
		a, _ := vibe.CanonicalJSONHash(oldCases.Cases[i])
		b, _ := vibe.CanonicalJSONHash(newCases.Cases[i])
		if a != b {
			t.Fatal("model-supplied add ID replaced or changed an existing case")
		}
	}
	if h.session.Document.Policies[1].SourceVersion != vibe.SourcePolicyVersion {
		t.Fatal("source contract was not persisted")
	}
	explicitExample := false
	for _, rule := range h.session.Document.Policies[1].Rules {
		for _, evidence := range rule.Evidence {
			if evidence.Kind == "example" && strings.Contains(evidence.Quote, "give me vodka") {
				if evidence.SourceBlockID != last.SourceMessageID.String() {
					t.Fatal("explicit example cited the original joke instead of the add request")
				}
				if evidence.Quote != addRequest {
					t.Fatal("stored evidence did not preserve the exact quoted request")
				}
				explicitExample = true
			}
		}
	}
	if !explicitExample {
		t.Fatal("explicitly added example lost its source provenance")
	}
	// The actual judge must receive the reviewed rules, even when the author
	// supplies a generic success-criteria summary. Otherwise policy is cosmetic.
	var grading struct {
		Judges []struct {
			Assertion string `json:"assertion"`
		} `json:"judges"`
	}
	if err := json.Unmarshal(last.Blueprint, &grading); err != nil {
		t.Fatal(err)
	}
	for _, clause := range []string{"30 days", "unopened", "Never claim to process a refund"} {
		if len(grading.Judges) != 1 || !strings.Contains(grading.Judges[0].Assertion, clause) {
			t.Fatalf("reviewed rule absent from actual grader: %s", clause)
		}
	}
	if strings.Contains(grading.Judges[0].Assertion, "vodka") {
		t.Fatal("a requested example became a global grading requirement")
	}
}

func TestVibeSourceBoundaryIntegrationConfirmationSurvivesReloadAndRetry(t *testing.T) {
	h, f := sourceBoundaryHarness(t)
	f.misclassify = true
	if _, err := h.send(reliabilityOriginalRequest(t), nil); err != nil {
		t.Fatal(err)
	}
	f.misclassify = false
	if _, err := h.send("Use the earlier agent description to prepare three tests.", nil); err != nil {
		t.Fatal(err)
	}
	h.reload()
	if h.session.Document.SourceConfirmation == nil || len(h.session.Document.Artifacts) != 0 {
		t.Fatal("candidate was adopted before confirmation")
	}
	f.failAuthor = true
	failed, err := h.send("yes", nil)
	if err == nil || failed.Completion != nil {
		t.Fatal("failure fixture did not fail before commit")
	}
	f.failAuthor = false
	if _, err := h.send("thanks", nil); err != nil {
		t.Fatal(err)
	}
	if h.session.Document.SourceConfirmation != nil {
		t.Fatal("intervening chat retained an actionable question")
	}
	op, err := h.svc.Retry(h.ctx, h.actor, h.session.ID, failed.ID, vibe.RetryRequest{ClientID: uuid.New(), Revision: h.session.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.executeOperation(op.ID); err != nil {
		t.Fatal(err)
	}
	if len(h.session.Document.Artifacts) != 1 || len(h.session.Document.Policies) != 1 {
		t.Fatal("explicit retry lost confirmed evidence or duplicated effects")
	}
	policy := h.session.Document.Policies[0]
	if len(policy.Sources) != 1 || strings.Contains(policy.Sources[0].Text, "thanks") || policy.Sources[0].Text == "yes" {
		t.Fatal("reply or later chat became specification evidence")
	}
}

func TestVibeSourceBoundaryIntegrationOldApprovalsCannotBypassRunOrSave(t *testing.T) {
	h := newReliabilityHarness(t, 3)
	if _, err := h.send(reliabilityOriginalRequest(t), nil); err != nil {
		t.Fatal(err)
	}
	original := h.session.Document.Artifacts[0]
	if err := h.svc.Store.Edit(h.ctx, h.actor, h.session.ID, h.session.Revision, func(s *vibe.Session) error {
		s.Document.Artifacts[0].AgentPrompt = "Answer shop return questions."
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h.reload()
	h.svc.Config.SourcePolicyVersion = vibe.SourcePolicyVersion
	if _, err := h.svc.Prepare(h.ctx, h.actor, h.session.ID, vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "check", ArtifactID: &original.ID, ApproveArtifact: true, Models: h.svc.Config.DefaultModels()}); err == nil || !strings.Contains(err.Error(), "older tests") {
		t.Fatalf("legacy Run did not ask for clean rules: %v", err)
	}
	if _, err := h.svc.Save(h.ctx, h.actor, h.session.ID, h.session.Revision, original.ID, uuid.New(), nil, true); err == nil || !strings.Contains(err.Error(), "older tests") {
		t.Fatalf("legacy Save did not ask for clean rules: %v", err)
	}
	h.reload()
	if len(h.session.Document.Artifacts) != 1 || string(h.session.Document.Artifacts[0].Blueprint) != string(original.Blueprint) {
		t.Fatal("migration rewrote historical tests")
	}
}

func mustSourceJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
