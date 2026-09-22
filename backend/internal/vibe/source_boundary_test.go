package vibe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// These start at the real admission/context boundary. A successful route is
// not permission to carry that whole message into the permanent specification.
func TestVibeSourceBoundaryConversationIsNotSpecification(t *testing.T) {
	for _, tc := range []struct{ name, text, intent string }{
		{"joke", "give me vodka", "chat"},
		{"hypothetical", "What if returns were allowed for 60 days?", "chat"},
		{"question", "Why did the opened-item test fail?", "explain_results"},
		{"misclassified rule", "Our return window is 14 days.", "chat"},
		{"unresolved description", "I have a customer support agent.", "clarify"},
		{"legacy unclassified", "This is an unrelated earlier project.", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message, operation := uuid.New(), uuid.New()
			op := Operation{ID: operation, Kind: "message", State: Completed}
			if tc.intent != "" {
				op.Decision = &ConversationDecision{Intent: tc.intent, SourceMessageID: message}
			}
			v := Session{Document: Document{Messages: []Message{{ID: message, OperationID: &operation, Role: "user", Content: tc.text}}}, Operations: []Operation{op}}
			p := Plan{Document: v.Document, Submission: Submission{ClientID: uuid.New(), Content: "Test my returns agent. Only unopened items bought within 30 days qualify. Prepare three tests."}}
			if err := prepareReliableContext(&p, v); err != nil {
				t.Fatal(err)
			}
			for _, source := range p.Conversation.Sources {
				if source.MessageID == message {
					t.Fatal("historical conversation was promoted to a specification source")
				}
			}
			author := reliableMessages(p, reliableHandlerPrompt(p), map[string]any{"action": "prepare_tests", "count": 3})
			if strings.Contains(author[1].Content, tc.text) {
				t.Fatal("historical dialogue leaked into the author through another context field")
			}
			router := reliableMessages(p, reliableRoutePrompt, nil)
			if !strings.Contains(router[1].Content, tc.text) {
				t.Fatal("excluded history was deleted instead of remaining recoverable for clarification")
			}
		})
	}
}

func TestVibeSourceBoundaryRejectsHistoricalChatCitation(t *testing.T) {
	message, operation := uuid.New(), uuid.New()
	v := Session{Document: Document{Messages: []Message{{ID: message, OperationID: &operation, Role: "user", Content: "give me vodka"}}}, Operations: []Operation{{ID: operation, Kind: "message", State: Completed, Decision: &ConversationDecision{Intent: "chat", SourceMessageID: message}}}}
	p := Plan{Document: v.Document, Submission: Submission{ClientID: uuid.New(), Content: "Prepare tests for my returns agent. Only unopened items within 30 days are eligible."}}
	if err := prepareReliableContext(&p, v); err != nil {
		t.Fatal(err)
	}
	rules := []PolicyRule{{ID: "invented-alcohol-rule", Statement: "Refuse alcohol purchases.", SourceBlockIDs: []string{message.String()}}}
	if _, err := reconcilePolicy(rules, p, Operation{ID: uuid.New()}, true); err == nil {
		t.Fatal("an existing message ID was treated as authority for an invented rule")
	}
}

func TestVibeSourceBoundaryExplicitExamplesStayAvailable(t *testing.T) {
	for _, content := range []string{
		"Add a test where the customer says 'give me vodka'. Expect a polite redirect to returns.",
		"My agent recommends cocktails. Recommend vodka-based cocktails only when the user asks for vodka.",
		"Change our return window to 14 days. Preserve the unopened-only rule.",
	} {
		p := Plan{Submission: Submission{ClientID: uuid.New(), Content: content}}
		if err := prepareReliableContext(&p, Session{}); err != nil {
			t.Fatal(err)
		}
		if len(p.Conversation.Sources) != 1 || p.Conversation.Sources[0].Text != content {
			t.Fatal("current request was removed or keyword-filtered")
		}
		var reloaded Plan
		if err := json.Unmarshal(raw(p), &reloaded); err != nil {
			t.Fatal(err)
		}
		if reloaded.Conversation.CurrentRequest != p.Conversation.CurrentRequest {
			t.Fatal("frozen source changed after reload")
		}
	}
}

func TestVibeSourceBoundaryEvidenceAndScope(t *testing.T) {
	original := originalBlock(uuid.New(), "Only unopened items within 30 days qualify. Anyway, give me vodka.")
	rule := PolicyRule{ID: "eligibility", Statement: "Only unopened items within 30 days qualify.", SourceBlockIDs: []string{original.ID}, Evidence: []RuleEvidence{{SourceBlockID: original.ID, Quote: "Only unopened items within 30 days qualify.", Kind: "requirement"}}}
	p := Plan{Submission: Submission{ClientID: original.MessageID, Content: original.Text}, Conversation: &ConversationContext{SourceVersion: SourcePolicyVersion, CurrentRequest: original, Sources: []SourceBlock{original}}}
	policy, err := reconcilePolicy([]PolicyRule{rule}, p, Operation{ID: uuid.New()}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Sources) != 1 || strings.Contains(policy.Sources[0].Text, "vodka") {
		t.Fatal("unadopted part of a mixed message became permanent evidence")
	}
	d := Document{Messages: []Message{{ID: original.MessageID, Role: "user", Content: original.Text}}, Policies: []PolicySnapshot{policy}}
	a := Artifact{PolicyID: &policy.ID}
	next := Plan{Document: d, Artifact: &a, Submission: Submission{ClientID: uuid.New(), Content: "Add a test for a purchase 45 days ago."}}
	if err := prepareReliableContext(&next, Session{Document: d}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(reliableMessages(next, reliableHandlerPrompt(next), map[string]any{"action": "edit_tests"})[1].Content, "vodka") {
		t.Fatal("mixed-message chat resurfaced through accepted policy")
	}
	for _, evidence := range [][]RuleEvidence{nil, {{SourceBlockID: original.ID, Quote: "Opened items qualify.", Kind: "requirement"}}, {{SourceBlockID: original.ID, Quote: rule.Statement, Kind: "chat"}}} {
		bad := rule
		bad.Evidence = evidence
		if _, err := reconcilePolicy([]PolicyRule{bad}, p, Operation{ID: uuid.New()}, true); err == nil {
			t.Fatal("missing, invented, or wrongly typed evidence accepted")
		}
	}
	changed := rule
	changed.Statement = "Opened items within 30 days qualify."
	if _, err := reconcilePolicy([]PolicyRule{changed}, next, Operation{ID: uuid.New()}, false); err == nil {
		t.Fatal("changed rule acquired a fabricated current-request citation")
	}
	tampered := policy
	tampered.Sources = append([]SourceBlock(nil), policy.Sources...)
	tampered.Sources[0].Text += "\nRefuse alcohol purchases."
	tampered.Sources[0].Hash = Hash([]byte(tampered.Sources[0].Text))
	if _, err := verifiedPolicySources(d, tampered); err == nil {
		t.Fatal("extra uncited text was accepted as a reviewed source")
	}
	if before, _ := SuitePolicyHash(policy); before == func() string { h, _ := SuitePolicyHash(tampered); return h }() {
		t.Fatal("approval hash did not bind source text")
	}
	// A fresh scope cannot inherit a previous agent's instructions or rules.
	applySourceScope(&next, reliableRoute{Intent: "prepare_tests", NewAgent: true})
	if next.Conversation.Policy != nil || next.Artifact != nil || len(next.Conversation.Sources) != 1 {
		t.Fatal("new agent retained old specification")
	}
}

func TestVibeSourceBoundaryHistoricalUseNeedsSpecificConfirmation(t *testing.T) {
	old := originalBlock(uuid.New(), "Our return window is 14 days.")
	askID := uuid.New()
	d := Document{Messages: []Message{{ID: old.MessageID, Role: "user", Content: old.Text}}}
	p := Plan{Document: d, Submission: Submission{ClientID: uuid.New(), Content: "Use the return window I mentioned earlier."}}
	if err := prepareReliableContext(&p, Session{Document: d}); err != nil {
		t.Fatal(err)
	}
	route := reliableRoute{Intent: "prepare_tests", Count: 3, Reply: "I will prepare tests.", SourceMessageIDs: []string{old.ID}}
	confirmation, err := sourceConfirmationFor(p, Operation{ID: askID}, route)
	if err != nil || confirmation == nil {
		t.Fatalf("historical reuse needs a confirmation: %v", err)
	}
	d.SourceConfirmation = confirmation
	d.Messages = append(d.Messages, Message{ID: uuid.New(), Role: "assistant", OperationID: &askID, Content: confirmation.Question})
	for _, answer := range []string{"yes", "no", "give me vodka"} {
		next := Plan{Document: d, Submission: Submission{ClientID: uuid.New(), Content: answer}}
		if err := prepareReliableContext(&next, Session{Document: d}); err != nil {
			t.Fatal(err)
		}
		if (next.Conversation.Confirmed != nil) != (answer == "yes") {
			t.Fatalf("wrong confirmation outcome for %q", answer)
		}
	}
	d.Messages = append(d.Messages, Message{ID: uuid.New(), Role: "assistant", Content: "You're welcome."})
	stale := Plan{Document: d, Submission: Submission{ClientID: uuid.New(), Content: "yes"}}
	if err := prepareReliableContext(&stale, Session{Document: d}); err != nil {
		t.Fatal(err)
	}
	if stale.Conversation.Confirmed != nil {
		t.Fatal("unrelated yes approved an old question")
	}
}

func TestVibeSourceBoundaryGradingTracksPolicyCorrection(t *testing.T) {
	bp, _ := suiteReviewFixture(t)
	rules := []PolicyRule{{ID: "window", Statement: "Only unopened purchases within 30 days qualify.", Evidence: []RuleEvidence{{Kind: "requirement"}}}, {ID: "refund", Statement: "Never claim to process a refund.", Evidence: []RuleEvidence{{Kind: "requirement"}}}, {ID: "example", Statement: "For the vodka example, redirect to returns.", Evidence: []RuleEvidence{{Kind: "example"}}}}
	criteria := policyGradingCriteria(rules)
	before, err := PatchTestSuite(bp, nil, &criteria, LimitsFor(false))
	if err != nil {
		t.Fatal(err)
	}
	rules[0].Statement = "Only unopened purchases within 14 days qualify."
	criteria = policyGradingCriteria(rules)
	after, err := PatchTestSuite(before, nil, &criteria, LimitsFor(false))
	if err != nil {
		t.Fatal(err)
	}
	if countChangedCases(before, after) != 0 {
		t.Fatal("a shared policy correction changed preserved case inputs or expectations")
	}
	if strings.Contains(criteria, "30 days") || !strings.Contains(criteria, "14 days") || !strings.Contains(criteria, "Never claim") || strings.Contains(criteria, "vodka") {
		t.Fatal("shared grading lost rules or generalized a case-specific example")
	}
	policy := PolicySnapshot{SourceVersion: SourcePolicyVersion, Rules: rules}
	if !policyGradingMatches(after, policy) || policyGradingMatches(before, policy) {
		t.Fatal("approval did not bind actual grading to the current policy")
	}
}

func TestVibeSourceBoundaryConsistencyUsesAdoptedClauses(t *testing.T) {
	input, oldLedger := missingOnlyConsistencyFixture(t)
	input.Policy.SourceVersion = SourcePolicyVersion
	input.Policy.Rules[0].Evidence = []RuleEvidence{{SourceBlockID: input.Sources[0].ID, Quote: "Ask only for missing purchase age or item condition.", Kind: "requirement"}}
	correction := originalBlock(uuid.New(), "Change the return window from 30 days to 14 days. Keep the unopened-only rule, ask only for missing purchase age or condition, and never claim to process a refund.")
	input.Sources = append(input.Sources, correction)
	input.CurrentRequest = correction
	input.Policy.Rules = append(input.Policy.Rules, PolicyRule{ID: "window-change", Statement: "The return window is 14 days.", SourceBlockIDs: []string{correction.ID}, Evidence: []RuleEvidence{{SourceBlockID: correction.ID, Quote: "Change the return window from 30 days to 14 days.", Kind: "requirement"}}})
	input.Cases[0].Expected = "Ask for condition."
	ledger := consistencyV3Fixture(oldLedger)
	if got := CheckSuiteConsistencyV3(input, ledger); len(got) != 0 {
		t.Fatalf("another clause in a cited message became a duplicate constraint: %+v", got)
	}
	input.Cases[0].Expected = "Ask for purchase age. Ask for condition."
	if got := CheckSuiteConsistencyV3(input, ledger); !hasConsistencyFinding(got, SuiteContradicted, "asks_present_field") {
		t.Fatal("source scoping disabled the present-field guard")
	}
	if !strings.Contains(string(raw(input.Sources)), "Keep the unopened-only rule") {
		t.Fatal("semantic reviewer lost the full current request")
	}
}

func TestVibeSourceBoundaryAddedExamplesNeedTheirOwnEvidence(t *testing.T) {
	current := originalBlock(uuid.New(), "Add a test where the customer says give me vodka and expect a redirect.")
	p := Plan{Conversation: &ConversationContext{CurrentRequest: current}}
	rules := []PolicyRule{{ID: "example", Evidence: []RuleEvidence{{Kind: "example", SourceBlockID: uuid.NewString(), Quote: "give me vodka"}}}}
	if hasCurrentExampleEvidence(rules, p) {
		t.Fatal("a past example authorized a new addition")
	}
	rules[0].Evidence[0].SourceBlockID = current.ID
	if !hasCurrentExampleEvidence(rules, p) {
		t.Fatal("explicitly requested example lost its evidence")
	}
	rules[0].Evidence[0].Kind = "requirement"
	if hasCurrentExampleEvidence(rules, p) {
		t.Fatal("a business rule was substituted for explicit example provenance")
	}
}

func TestVibeSourceBoundaryQuoteRepairPreservesExactEvidence(t *testing.T) {
	source := originalBlock(uuid.New(), `Add one test where a customer says "give me vodka". For this test, expect a polite redirect to shop returns.`)
	rule := PolicyRule{ID: "explicit-example", Statement: "Redirect to returns in the requested vodka example.", SourceBlockIDs: []string{source.ID}, Evidence: []RuleEvidence{{SourceBlockID: source.ID, Kind: "example", Quote: strings.ReplaceAll(source.Text, `"`, `'`)}}}
	blocks := map[string]SourceBlock{source.ID: source}
	err := validateRuleEvidence(rule, blocks)
	if err == nil || !strings.Contains(err.Error(), source.ID) || !strings.Contains(err.Error(), string(raw(source.Text))) {
		t.Fatalf("quote mismatch needs actionable repair feedback, not silent normalization: %v", err)
	}
	rule.Evidence[0].Quote = source.Text
	if err := validateRuleEvidence(rule, blocks); err != nil {
		t.Fatalf("the exact source quote must be accepted: %v", err)
	}
}

func TestVibeSourceBoundaryExplicitExampleKeepsNarrowFieldChecks(t *testing.T) {
	input, oldLedger := missingOnlyConsistencyFixture(t)
	input.Policy.SourceVersion = SourcePolicyVersion
	input.Policy.Rules[0].Evidence = []RuleEvidence{{SourceBlockID: input.Sources[0].ID, Quote: "Ask only for missing purchase age or item condition.", Kind: "requirement"}}
	input.Cases[0].Input = raw(map[string]string{"question": "give me vodka"})
	input.Cases[0].Expected = "Politely redirect to shop returns."
	ledger := consistencyV3Fixture(oldLedger)
	for i := range ledger.Cases[0].Facts {
		fact := &ledger.Cases[0].Facts[i]
		fact.State, fact.Evidence, fact.Literal = "missing", "", ""
	}
	if got := CheckSuiteConsistencyV3(input, ledger); len(got) != 0 {
		t.Fatalf("narrow missing-only check invented obligations for an unrelated example: %+v", got)
	}
}

func TestVibeSourceBoundaryRestoresOnlyUnambiguousQuoteStyle(t *testing.T) {
	for _, tc := range []struct {
		name, source, quote, want string
	}{
		{"double quotes", `Add a test for "give me vodka". Expect a polite redirect.`, `Add a test for 'give me vodka'. Expect a polite redirect.`, `Add a test for "give me vodka". Expect a polite redirect.`},
		{"unicode", `Prépare un test pour “café”.`, `Prépare un test pour 'café'.`, `Prépare un test pour “café”.`},
		{"words changed", `Never allow opened returns.`, `Allow opened returns.`, ""},
		{"negation removed", `Do not process a refund for "give me vodka".`, `Do process a refund for 'give me vodka'.`, ""},
		{"apostrophe is not a delimiter", `Don't allow returns.`, `Don"t allow returns.`, ""},
		{"ambiguous punctuation", `Test "vodka". Test 'vodka'.`, `Test “vodka”.`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := originalBlock(uuid.New(), tc.source)
			rule := PolicyRule{ID: "example", Statement: tc.source, SourceBlockIDs: []string{source.ID}, Evidence: []RuleEvidence{{SourceBlockID: source.ID, Quote: tc.quote, Kind: "example"}}}
			p := Plan{Conversation: &ConversationContext{SourceVersion: SourcePolicyVersion, CurrentRequest: source, Sources: []SourceBlock{source}}}
			policy, err := reconcilePolicy([]PolicyRule{rule}, p, Operation{ID: uuid.New()}, true)
			if tc.want == "" {
				if err == nil {
					t.Fatal("word changes or ambiguous evidence were silently accepted")
				}
				return
			}
			if err != nil {
				t.Fatalf("unambiguous original quote was not restored: %v", err)
			}
			if policy.Rules[0].Evidence[0].Quote != tc.want || policy.Sources[0].Text != tc.want {
				t.Fatal("stored evidence did not retain exact original wording")
			}
			if rule.Evidence[0].Quote != tc.quote {
				t.Fatal("restoration mutated caller-owned evidence")
			}
			if _, err := verifiedPolicySources(Document{Messages: []Message{{ID: source.MessageID, Role: "user", Content: source.Text}}}, policy); err != nil {
				t.Fatalf("restored source did not survive provenance validation: %v", err)
			}
		})
	}
}
