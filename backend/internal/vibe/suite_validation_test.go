package vibe

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func suiteConsistencyReviewFixture(t *testing.T, expected string) (json.RawMessage, SuiteReviewInput) {
	t.Helper()
	text := "Only unopened items bought within 14 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund."
	source := SourceBlock{ID: "source-policy", MessageID: uuid.New(), Text: text, Hash: Hash([]byte(text))}
	policy := PolicySnapshot{ID: uuid.New(), ScopeID: uuid.New(), SourceMessageID: source.MessageID, Rules: []PolicyRule{{ID: "supplied-contract", Statement: text, SourceBlockIDs: []string{source.ID}}}}
	blueprint := raw(map[string]any{
		"instructions": "{{question}}",
		"cases":        []map[string]any{{"key": "case-1", "payload": map[string]any{"question": "My item was bought 10 days ago; I have not said its condition."}, "expectations": []map[string]any{{"key": ExpectedBehaviorKey, "kind": "text", "value": expected}}}},
		"judges":       []map[string]any{{"key": "behavior", "assertion": ScenarioCriteriaPrefix + text, "context_from": []string{ExpectedBehaviorReference}}},
		"validators":   []map[string]any{{"key": "has_answer", "type": "regex_match", "target": "final_output", "expected_from": "literal:.+"}},
		"dimensions":   []map[string]any{{"key": "behavior", "source": "llm_judge", "judge_key": "behavior"}},
	})
	input, err := BuildSuiteReviewInput(blueprint, policy, []SourceBlock{source}, source, 1, LimitsFor(false))
	if err != nil {
		t.Fatal(err)
	}
	input.ValidatorVersion = ConsistencySuiteValidatorVersion
	return blueprint, input
}

func suiteConsistencyLedgerReply(expected string, askAge bool) map[string]any {
	quote := func(text string) map[string]any {
		return map[string]any{"source_block_id": "source-policy", "text": text}
	}
	obligations := []map[string]any{{"entity_id": "item", "field_id": "condition", "kind": "ask", "evidence": expected}}
	if askAge {
		obligations = append(obligations, map[string]any{"entity_id": "item", "field_id": "purchase-age", "kind": "ask", "evidence": expected})
	}
	return map[string]any{
		"entities": []map[string]any{{"id": "item", "source": quote("items")}},
		"fields": []map[string]any{
			{"id": "purchase-age", "entity_id": "item", "kind": "number", "aliases": []map[string]any{quote("purchase age")}},
			{"id": "condition", "entity_id": "item", "kind": "enum", "aliases": []map[string]any{quote("item condition"), quote("condition")}},
		},
		"missing_only": []map[string]any{{"rule_id": "supplied-contract", "source": quote("Ask only for missing purchase age or item condition."), "field_ids": []string{"purchase-age", "condition"}}},
		"cases": []map[string]any{{"case_key": "case-1", "facts": []map[string]any{
			{"entity_id": "item", "field_id": "purchase-age", "state": "present", "input_pointer": "/question", "evidence": "My item was bought 10 days ago;", "literal": "10"},
			{"entity_id": "item", "field_id": "condition", "state": "missing", "input_pointer": "/question", "evidence": "I have not said its condition.", "literal": ""},
		}, "obligations": obligations}},
	}
}

func suiteReviewFixture(t *testing.T) (json.RawMessage, SuiteReviewInput) {
	t.Helper()
	text := "Only unopened items bought within 14 days are eligible. Never claim to process a refund."
	source := SourceBlock{ID: "source-policy", MessageID: uuid.New(), Text: text, Hash: Hash([]byte(text))}
	policy := PolicySnapshot{ID: uuid.New(), ScopeID: uuid.New(), SourceMessageID: source.MessageID, Rules: []PolicyRule{{ID: "eligibility", Statement: "Only unopened items within 14 days qualify.", SourceBlockIDs: []string{source.ID}}, {ID: "no-refund", Statement: "Never claim to process a refund.", SourceBlockIDs: []string{source.ID}}}}
	blueprint := raw(map[string]any{
		"instructions": "DO NOT INCLUDE TARGET INSTRUCTIONS IN REVIEW",
		"cases":        []map[string]any{{"key": "case-1", "payload": map[string]any{"question": "An unopened item was bought 10 days ago."}, "expectations": []map[string]any{{"key": ExpectedBehaviorKey, "kind": "text", "value": "Confirm eligibility without claiming to process a refund."}}}},
		"judges":       []map[string]any{{"key": "behavior", "assertion": ScenarioCriteriaPrefix + text, "context_from": []string{ExpectedBehaviorReference}}},
		"validators":   []map[string]any{{"key": "has_answer", "type": "regex_match", "target": "final_output", "expected_from": "literal:.+"}},
		"dimensions":   []map[string]any{{"key": "behavior", "source": "llm_judge", "judge_key": "behavior"}},
	})
	input, err := BuildSuiteReviewInput(blueprint, policy, []SourceBlock{source}, source, 1, LimitsFor(false))
	if err != nil {
		t.Fatal(err)
	}
	return blueprint, input
}

func supportedSuiteReview(input SuiteReviewInput) map[string]any {
	finding := func(ids []string) SuiteReviewFinding {
		return SuiteReviewFinding{Status: SuiteSupported, RuleIDs: ids, SourceBlockIDs: []string{input.Sources[0].ID}, Reason: "The expected behavior follows the stated facts and policy."}
	}
	ruleIDs := []string{}
	rules := []SuiteRuleReview{}
	for _, r := range input.Policy.Rules {
		ruleIDs = append(ruleIDs, r.ID)
		rules = append(rules, SuiteRuleReview{RuleID: r.ID, SuiteReviewFinding: finding([]string{r.ID})})
	}
	cases := []SuiteCaseReview{}
	for _, c := range input.Cases {
		cases = append(cases, SuiteCaseReview{CaseKey: c.CaseKey, SuiteReviewFinding: finding(ruleIDs)})
	}
	return map[string]any{"cases": cases, "rules": rules, "shared_criteria": finding(ruleIDs), "policy_reconciliation": finding(ruleIDs)}
}

func TestSuiteReviewRequiresCompleteGroundedEvidence(t *testing.T) {
	_, input := suiteReviewFixture(t)
	for _, name := range []string{"missing_case", "duplicate_case", "unknown_case", "missing_rule", "unknown_rule", "missing_shared", "missing_reconciliation", "unknown_source", "duplicate_source", "missing_source", "unknown_rule_reference", "empty_case_rules", "empty_reason", "whitespace_reason", "unknown_status", "invented_overall_status"} {
		t.Run(name, func(t *testing.T) {
			reply := supportedSuiteReview(input)
			cases := reply["cases"].([]SuiteCaseReview)
			switch name {
			case "missing_case":
				reply["cases"] = []SuiteCaseReview{}
			case "duplicate_case":
				reply["cases"] = append(cases, cases[0])
			case "unknown_case":
				cases[0].CaseKey = "missing"
			case "missing_rule":
				reply["rules"] = reply["rules"].([]SuiteRuleReview)[:1]
			case "unknown_rule":
				reply["rules"].([]SuiteRuleReview)[0].RuleID = "missing"
			case "missing_shared":
				delete(reply, "shared_criteria")
			case "missing_reconciliation":
				delete(reply, "policy_reconciliation")
			case "unknown_source":
				cases[0].SourceBlockIDs = []string{"missing"}
			case "duplicate_source":
				cases[0].SourceBlockIDs = []string{input.Sources[0].ID, input.Sources[0].ID}
			case "missing_source":
				cases[0].SourceBlockIDs = nil
			case "unknown_rule_reference":
				cases[0].RuleIDs = []string{"missing"}
			case "empty_case_rules":
				cases[0].RuleIDs = nil
			case "empty_reason":
				cases[0].Reason = ""
			case "whitespace_reason":
				cases[0].Reason = " \n\t"
			case "unknown_status":
				cases[0].Status = "pass"
			case "invented_overall_status":
				reply["status"] = "supported"
			}
			if result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false)); err == nil || result != nil {
				t.Fatalf("incomplete or ungrounded report became an acceptance: %+v %v", result, err)
			}
		})
	}
}

func TestSuiteReviewDerivesStatusAndPinsExactContract(t *testing.T) {
	blueprint, input := suiteReviewFixture(t)
	accepted, err := ParseSuiteReview(raw(supportedSuiteReview(input)), input, LimitsFor(false))
	if err != nil || !SuiteValidationMatches(accepted, blueprint, input.Policy) {
		t.Fatalf("valid report could not be reused: %+v %v", accepted, err)
	}
	for _, status := range []string{SuiteUnclear, SuiteContradicted} {
		reply := supportedSuiteReview(input)
		cases := reply["cases"].([]SuiteCaseReview)
		cases[0].Status, cases[0].Reason = status, "The case depends on an unresolved policy boundary."
		result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
		if err != nil || result.Status != status || len(result.Problems) != 1 || SuiteValidationMatches(result, blueprint, input.Policy) {
			t.Fatalf("unresolved review became runnable: %+v %v", result, err)
		}
	}
	for _, subject := range []string{"shared_criteria", "policy_reconciliation"} {
		reply := supportedSuiteReview(input)
		finding := reply[subject].(SuiteReviewFinding)
		finding.Status, finding.Reason = SuiteContradicted, "The no-refund prohibition was removed without authorization."
		reply[subject] = finding
		result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
		if err != nil || result.Status != SuiteContradicted || len(result.Problems) != 1 {
			t.Fatalf("shared rule drift was accepted: %+v %v", result, err)
		}
	}
	var changed map[string]any
	_ = json.Unmarshal(blueprint, &changed)
	changed["validators"] = []any{}
	if SuiteValidationMatches(accepted, raw(changed), input.Policy) {
		t.Fatal("changed grading retained a cached review")
	}
	policy := input.Policy
	policy.ID = uuid.New()
	if SuiteValidationMatches(accepted, blueprint, policy) {
		t.Fatal("another policy version retained a cached review")
	}
	accepted.Cases = nil
	if SuiteValidationMatches(accepted, blueprint, input.Policy) {
		t.Fatal("missing review coverage was accepted")
	}
	if SuiteValidationMatches(nil, blueprint, input.Policy) {
		t.Fatal("legacy absent validation was accepted")
	}
}

func TestSuiteReviewMalformedOutputCannotBecomeAcceptance(t *testing.T) {
	_, input := suiteReviewFixture(t)
	for _, output := range []string{"", "null", "{}", "not json", `{"cases":[],"cases":[],"rules":[]}`} {
		if result, err := ParseSuiteReview([]byte(output), input, LimitsFor(false)); err == nil || result != nil {
			t.Fatalf("malformed review became an acceptance: %q %+v %v", output, result, err)
		}
	}
	reply := supportedSuiteReview(input)
	rules := reply["rules"].([]SuiteRuleReview)
	rules[0].Status = SuiteUnclear
	cases := reply["cases"].([]SuiteCaseReview)
	cases[0].Status = SuiteContradicted
	result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
	if err != nil || result.Status != SuiteContradicted || len(result.Problems) != 2 {
		t.Fatalf("uncertainty hid a concrete conflict: %+v %v", result, err)
	}
}

func TestSuiteReviewBuilderRejectsMissingSourcesAndCounts(t *testing.T) {
	blueprint, input := suiteReviewFixture(t)
	if _, err := BuildSuiteReviewInput(blueprint, input.Policy, input.Sources, input.CurrentRequest, 5, LimitsFor(false)); err == nil {
		t.Fatal("requested count mismatch was accepted")
	}
	bad := input.CurrentRequest
	bad.Text = "A fabricated source replacement."
	if _, err := BuildSuiteReviewInput(blueprint, input.Policy, input.Sources, bad, 1, LimitsFor(false)); err == nil {
		t.Fatal("source text changed without its hash")
	}
	bad.Hash = Hash([]byte(bad.Text))
	if _, err := BuildSuiteReviewInput(blueprint, input.Policy, input.Sources, bad, 1, LimitsFor(false)); err == nil {
		t.Fatal("duplicate source identity was ambiguous")
	}
	policy := input.Policy
	policy.Rules = append([]PolicyRule(nil), policy.Rules...)
	policy.Rules[0].SourceBlockIDs = []string{"absent-source"}
	if _, err := BuildSuiteReviewInput(blueprint, policy, input.Sources, input.CurrentRequest, 1, LimitsFor(false)); err == nil {
		t.Fatal("ungrounded rule was accepted")
	}
}

func TestSuiteReviewContextSeparatesTargetAndOriginalEvidence(t *testing.T) {
	_, input := suiteReviewFixture(t)
	previous := input.Policy
	previous.ID = uuid.New()
	input.PreviousPolicy = &previous
	messages := SuiteReviewMessages(input)
	if len(messages) != 2 || messages[0].Role != "system" || messages[1].Role != "user" {
		t.Fatal("review evidence is not separated from instructions")
	}
	data := messages[1].Content
	if strings.Contains(data, "DO NOT INCLUDE TARGET INSTRUCTIONS") || strings.Contains(data, "target_response") || strings.Contains(data, "prior_score") {
		t.Fatal("review context contains target or score evidence")
	}
	for _, value := range []string{input.CurrentRequest.Text, "previous_policy", "requested_count", "shared_criteria", "source-policy"} {
		if !strings.Contains(data, value) {
			t.Fatalf("review context lost %q", value)
		}
	}
}

func TestSuiteCanonicalHashPreservesSemanticBytesAndLargeNumbers(t *testing.T) {
	a, err := CanonicalJSONHash(json.RawMessage(`{"b":9007199254740993,"a":"Never approve.\nKeep this line."}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := CanonicalJSONHash(json.RawMessage("{\n \"a\":\"Never approve.\\nKeep this line.\", \"b\":9007199254740993}"))
	if err != nil || a != b {
		t.Fatal("object representation changed identity")
	}
	for _, other := range []string{`{"b":9007199254740992,"a":"Never approve.\nKeep this line."}`, `{"b":9007199254740993,"a":"Never approve. Keep this line."}`} {
		hash, err := CanonicalJSONHash(json.RawMessage(other))
		if err != nil || hash == a {
			t.Fatal("hash erased significant source or number bytes")
		}
	}
	if _, err := CanonicalJSONHash(json.RawMessage(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("duplicate keys created an ambiguous hash")
	}
}

func TestSuiteReliabilityFixturesRetainExactFailureEvidence(t *testing.T) {
	original, err := os.ReadFile("testdata/reliability/returns-original-request.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := "My agent answers questions about shop returns.\n\nOnly unopened items bought within 30 days are eligible.\nAsk only for missing purchase age or item condition.\nNever claim to process a refund.\n\nPrepare three tests:\n- Unopened item bought 10 days ago.\n- Opened item bought 10 days ago.\n- Customer asks for a return without any details.\n"
	if string(original) != want {
		t.Fatal("the original multiline request was flattened or changed")
	}
	data, err := os.ReadFile("testdata/reliability/captured-failures.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Fixtures []struct {
			ID                string          `json:"id"`
			Request           string          `json:"request"`
			ProviderOutputs   []string        `json:"provider_outputs"`
			CapturedBlueprint json.RawMessage `json:"captured_blueprint"`
			PreviousBlueprint json.RawMessage `json:"previous_blueprint"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Fixtures) != 4 || file.Fixtures[0].Request != want {
		t.Fatal("captured failure coverage changed")
	}
	last := file.Fixtures[3]
	if len(last.PreviousBlueprint) == 0 || !strings.Contains(string(last.CapturedBlueprint), "exactly 30 days") || !strings.Contains(string(last.CapturedBlueprint), "exactly 14 days") {
		t.Fatal("captured policy contradiction lost its before/after evidence")
	}
}

func TestSuiteCapturedPolicyConflictReportCannotApproveCandidate(t *testing.T) {
	data, err := os.ReadFile("testdata/reliability/captured-failures.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Fixtures []struct {
			ID        string          `json:"id"`
			Request   string          `json:"request"`
			Blueprint json.RawMessage `json:"captured_blueprint"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	_, base := suiteReviewFixture(t)
	for _, fixture := range file.Fixtures {
		if fixture.ID != "fourteen-day-contradiction" {
			continue
		}
		request := SourceBlock{ID: "correction", MessageID: uuid.New(), Text: fixture.Request, Hash: Hash([]byte(fixture.Request))}
		input, err := BuildSuiteReviewInput(fixture.Blueprint, base.Policy, base.Sources, request, 6, LimitsFor(false))
		if err != nil {
			t.Fatal(err)
		}
		reply := supportedSuiteReview(input)
		found := false
		for i, c := range input.Cases {
			if strings.Contains(string(c.Input), "exactly 30 days") && strings.Contains(c.Expected, "exactly 14 days") {
				// This is a scripted negative-control critic response, not a
				// claim that a model has detected the captured semantic error.
				finding := &reply["cases"].([]SuiteCaseReview)[i]
				finding.Status = SuiteContradicted
				finding.Reason = "The input says 30 days; the expectation instead asserts eligibility for an imagined 14-day purchase."
				found = true
			}
		}
		result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
		if !found || err != nil || result.Status != SuiteContradicted || SuiteValidationMatches(result, fixture.Blueprint, base.Policy) {
			t.Fatalf("captured conflict was not retained as a blocking review: %+v %v", result, err)
		}
		return
	}
	t.Fatal("captured policy edit missing")
}

func TestSuiteCalibrationManifestSeparatesCalibrationAndHeldOut(t *testing.T) {
	data, err := os.ReadFile("testdata/calibration/suite-validity.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Fixtures []struct {
			ID, Domain, Split, Source, Input, Expected string
			Status                                     string `json:"reference_status"`
			Reason                                     string `json:"reference_reason"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	counts, domains, ids := map[string]int{}, map[string]bool{}, map[string]bool{}
	for _, f := range file.Fixtures {
		if f.ID == "" || ids[f.ID] || f.Source == "" || f.Input == "" || f.Expected == "" || f.Reason == "" {
			t.Fatal("incomplete or duplicate calibration fixture")
		}
		if f.Status != SuiteSupported && f.Status != SuiteContradicted && f.Status != SuiteUnclear {
			t.Fatal("unknown reference label")
		}
		if f.Split != "calibration" && f.Split != "held_out" {
			t.Fatal("unknown data split")
		}
		ids[f.ID], domains[f.Domain] = true, true
		counts[f.Split+":"+f.Status]++
	}
	if len(file.Fixtures) != 72 || len(domains) != 6 {
		t.Fatal("calibration domain or case coverage shrank")
	}
	for _, status := range []string{SuiteSupported, SuiteContradicted, SuiteUnclear} {
		if counts["calibration:"+status] != 4 || counts["held_out:"+status] != 20 {
			t.Fatalf("imbalanced split for %s: %v", status, counts)
		}
	}
}

func TestSuiteCalibrationArithmeticAndEnumReferenceSubset(t *testing.T) {
	data, err := os.ReadFile("testdata/calibration/suite-validity.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Fixtures []struct {
			ID        string `json:"id"`
			Status    string `json:"reference_status"`
			Reference *struct {
				Kind                                          string `json:"kind"`
				Actual, Limit, Start, Duration, End, Expected float64
				ExpectedDecision                              bool              `json:"expected_decision"`
				Key                                           string            `json:"key"`
				Mapping                                       map[string]string `json:"mapping"`
				ExpectedText                                  string            `json:"expected_text"`
			} `json:"deterministic_reference"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, fixture := range file.Fixtures {
		r := fixture.Reference
		if r == nil {
			continue
		}
		checked++
		var consistent bool
		switch r.Kind {
		case "less_than":
			consistent = (r.Actual < r.Limit) == r.ExpectedDecision
		case "interval_ends_by":
			consistent = (r.Start+r.Duration <= r.End) == r.ExpectedDecision
		case "numeric_equal":
			consistent = r.Actual == r.Expected
		case "enum_lookup":
			expected, ok := r.Mapping[r.Key]
			consistent = ok && expected == r.ExpectedText
		default:
			t.Fatalf("unknown deterministic reference %q", r.Kind)
		}
		want := SuiteContradicted
		if consistent {
			want = SuiteSupported
		}
		if fixture.Status != want {
			t.Fatalf("%s has an inconsistent arithmetic/enum label: got %s want %s", fixture.ID, fixture.Status, want)
		}
	}
	if checked != 8 {
		t.Fatal("deterministic reference coverage changed")
	}
}

func TestSuiteReviewVersionSelectionPreservesV1Contract(t *testing.T) {
	_, input := suiteReviewFixture(t)
	profile := ModelProfile{StructuredOutputs: true}
	legacyFormat := SuiteReviewFormat(profile)
	legacyMessages := SuiteReviewMessages(input)
	if !bytes.Equal(legacyFormat, SuiteReviewFormatFor(profile, input)) {
		t.Fatal("default version changed the v1 schema")
	}
	if legacyMessages[0].Content != suiteReviewPrompt+"\nResponse schema: "+string(raw(suiteReviewSchema())) || strings.Contains(legacyMessages[1].Content, "validator_version") {
		t.Fatal("default version changed the v1 prompt or input")
	}
	input.ValidatorVersion = SuiteValidatorVersion
	if !bytes.Equal(legacyFormat, SuiteReviewFormatFor(profile, input)) || SuiteReviewMessages(input)[0].Content != legacyMessages[0].Content {
		t.Fatal("explicit v1 changed the v1 prompt or schema")
	}
	result, err := ParseSuiteReview(raw(supportedSuiteReview(input)), input, LimitsFor(false))
	if err != nil || result.ValidatorVersion != SuiteValidatorVersion || result.Consistency != nil {
		t.Fatalf("legacy acceptance no longer has its original version: %+v %v", result, err)
	}
	reply := supportedSuiteReview(input)
	reply["consistency"] = nil
	if result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false)); err == nil || result != nil {
		t.Fatal("v1 silently accepted a field outside its frozen response schema")
	}
	input.ValidatorVersion = "suite-review-unknown"
	if result, err := ParseSuiteReview(raw(supportedSuiteReview(input)), input, LimitsFor(false)); err == nil || result != nil {
		t.Fatal("unknown validator version became an acceptance")
	}
}

func TestSuiteReviewV2RequiresCompleteLedger(t *testing.T) {
	_, input := suiteConsistencyReviewFixture(t, "Ask only for item condition.")
	if !RequiresConsistency(input) {
		t.Fatal("missing-only rule lost the deterministic consistency requirement")
	}
	format := string(SuiteReviewFormatFor(ModelProfile{StructuredOutputs: true}, input))
	if !strings.Contains(format, "vibe_suite_review_v2") || !strings.Contains(format, "\"consistency\"") || !strings.Contains(SuiteReviewMessages(input)[0].Content, ConsistencyReviewInstructions) {
		t.Fatal("v2 required ledger was omitted from the response contract")
	}
	for _, name := range []string{"absent", "null", "empty", "unknown_property", "missing_case", "unknown_case", "missing_fact"} {
		t.Run(name, func(t *testing.T) {
			reply := supportedSuiteReview(input)
			ledger := suiteConsistencyLedgerReply(input.Cases[0].Expected, false)
			switch name {
			case "absent":
			case "null":
				reply["consistency"] = nil
			case "empty":
				reply["consistency"] = map[string]any{}
			case "unknown_property":
				ledger["trust_me"] = true
				reply["consistency"] = ledger
			case "missing_case":
				ledger["cases"] = []any{}
				reply["consistency"] = ledger
			case "unknown_case":
				ledger["cases"].([]map[string]any)[0]["case_key"] = "other-case"
				reply["consistency"] = ledger
			case "missing_fact":
				ledger["cases"].([]map[string]any)[0]["facts"] = []any{}
				reply["consistency"] = ledger
			}
			result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
			if err == nil && result.Status == SuiteSupported {
				t.Fatalf("incomplete v2 ledger became an acceptance: %+v", result)
			}
		})
	}
}

func TestSuiteReviewV2BlocksCapturedReturns07FalseAcceptance(t *testing.T) {
	// Exact input, expectation and successful reviewer reason from the fixed
	// v1 held-out measurement, operation d8faaf99-22c0-4d54-9a19-f5ab31695207.
	// The ledger variations below are scripted regression inputs, not a claim
	// that a new provider has produced or correctly extracted these ledgers.
	blueprint, input := suiteConsistencyReviewFixture(t, "Ask for both purchase age and condition.")
	reply := supportedSuiteReview(input)
	reply["cases"].([]SuiteCaseReview)[0].Reason = "Input states purchase age (10 days) and omits condition; policy requires asking only for missing purchase age or item condition, so asking for condition is supported. The expected answer does not ask for purchase age, which is already provided, and does not claim a refund."
	legacyInput := input
	legacyInput.ValidatorVersion = SuiteValidatorVersion
	legacy, err := ParseSuiteReview(raw(reply), legacyInput, LimitsFor(false))
	if err != nil || legacy.Status != SuiteSupported || !SuiteValidationMatches(legacy, blueprint, input.Policy) {
		t.Fatalf("the regression no longer reproduces the measured v1 false acceptance: %+v %v", legacy, err)
	}
	for _, includeAge := range []bool{true, false} {
		reply["consistency"] = suiteConsistencyLedgerReply(input.Cases[0].Expected, includeAge)
		result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
		if err != nil {
			t.Fatalf("scripted complete ledger failed to parse (include_age=%t): %v", includeAge, err)
		}
		if (result.Status != SuiteContradicted && result.Status != SuiteUnclear) || result.Cases[0].Status == SuiteSupported || len(result.Problems) == 0 || result.Consistency == nil || len(result.Consistency.Findings) == 0 || SuiteValidationMatches(result, blueprint, input.Policy) {
			t.Fatalf("misread or omitted age obligation became a v2 acceptance (include_age=%t): %+v", includeAge, result)
		}
	}
}

func TestSuiteReviewV2SupportedLedgerCannotBeDroppedOrDowngraded(t *testing.T) {
	blueprint, input := suiteConsistencyReviewFixture(t, "Ask only for item condition.")
	reply := supportedSuiteReview(input)
	reply["consistency"] = suiteConsistencyLedgerReply(input.Cases[0].Expected, false)
	result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
	if err != nil || result.Status != SuiteSupported || result.ValidatorVersion != ConsistencySuiteValidatorVersion || result.Consistency == nil || !result.Consistency.Required || !SuiteValidationMatches(result, blueprint, input.Policy) {
		t.Fatalf("supported v2 ledger did not retain its acceptance identity: %+v %v", result, err)
	}
	for _, mutate := range []func(*SuiteValidation){
		func(v *SuiteValidation) { v.Consistency = nil },
		func(v *SuiteValidation) { v.Consistency.Ledger = nil },
		func(v *SuiteValidation) { v.Consistency.Required = false },
		func(v *SuiteValidation) {
			v.Consistency.Findings = []ConsistencyFinding{{Status: SuiteUnclear, Message: "An unresolved mapping remains."}}
		},
		func(v *SuiteValidation) { v.ValidatorVersion = SuiteValidatorVersion },
		func(v *SuiteValidation) { v.ValidatorVersion = "unknown" },
	} {
		var changed SuiteValidation
		if err := json.Unmarshal(raw(result), &changed); err != nil {
			t.Fatal(err)
		}
		mutate(&changed)
		if SuiteValidationMatches(&changed, blueprint, input.Policy) {
			t.Fatal("missing, unresolved or differently versioned ledger retained acceptance")
		}
	}
}

func TestSuiteReviewV2KeepsNonApplicableAndGlobalFindingsDistinct(t *testing.T) {
	blueprint, input := suiteReviewFixture(t) // No missing-only rule in this source.
	input.ValidatorVersion = ConsistencySuiteValidatorVersion
	if RequiresConsistency(input) {
		t.Fatal("unrelated numeric eligibility rule triggered the missing-only check")
	}
	result, err := ParseSuiteReview(raw(supportedSuiteReview(input)), input, LimitsFor(false))
	if err != nil || result.Status != SuiteSupported || result.Consistency == nil || result.Consistency.Required || result.Consistency.Ledger != nil || !SuiteValidationMatches(result, blueprint, input.Policy) {
		t.Fatalf("non-applicable v2 check lost its explicit server record: %+v %v", result, err)
	}
	if strings.Contains(string(SuiteReviewFormatFor(ModelProfile{StructuredOutputs: true}, input)), "\"consistency\"") {
		t.Fatal("non-applicable v2 check demanded an unsupported ledger")
	}
	blueprint, input = suiteConsistencyReviewFixture(t, "Ask only for item condition.")
	reply := supportedSuiteReview(input)
	ledger := suiteConsistencyLedgerReply(input.Cases[0].Expected, false)
	ledger["missing_only"].([]map[string]any)[0]["source"] = map[string]any{"source_block_id": "source-policy", "text": "Ask only for missing purchase age."}
	reply["consistency"] = ledger
	result, err = ParseSuiteReview(raw(reply), input, LimitsFor(false))
	if err != nil || result.Status != SuiteUnclear || len(result.Problems) == 0 || SuiteValidationMatches(result, blueprint, input.Policy) {
		t.Fatalf("a global source-coverage finding was discarded: %+v %v", result, err)
	}
}

func TestSuiteReviewV3DerivesObligationsWithoutChangingV2(t *testing.T) {
	for _, expected := range []string{"Ask only for item condition.", "Ask for both purchase age and item condition.", "Do not ask for purchase age. Ask for item condition."} {
		t.Run(expected, func(t *testing.T) {
			_, input := suiteConsistencyReviewFixture(t, expected)
			ledger := suiteConsistencyLedgerReply(expected, true)
			c := ledger["cases"].([]map[string]any)[0]
			// The stopped v2 controls copied this original source sentence
			// into obligation.evidence, despite different expected text.
			for _, obligation := range c["obligations"].([]map[string]any) {
				obligation["evidence"] = "Ask only for missing purchase age or item condition."
			}
			if expected == "Ask for both purchase age and item condition." {
				input.Cases[0].Input = raw(map[string]string{"question": "I want to return an item."})
				for _, fact := range c["facts"].([]map[string]any) {
					fact["state"], fact["evidence"], fact["literal"] = "missing", "I want to return an item.", ""
				}
			}
			reply := supportedSuiteReview(input)
			reply["consistency"] = ledger
			v2, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
			if err != nil || v2.Status != SuiteUnclear || v2.ValidatorVersion != ConsistencySuiteValidatorVersion {
				t.Fatalf("the captured v2 bookkeeping failure changed behavior: %+v %v", v2, err)
			}
			v2Format, v2Prompt := SuiteReviewFormatFor(ModelProfile{StructuredOutputs: true}, input), SuiteReviewMessages(input)[0].Content
			if !strings.Contains(string(v2Format), "vibe_suite_review_v2") || !strings.Contains(string(v2Format), "\"obligations\"") || !strings.Contains(v2Prompt, ConsistencyReviewInstructions) {
				t.Fatal("the v2 prompt or obligations schema was not preserved")
			}
			delete(c, "obligations")
			input.ValidatorVersion = LatestSuiteValidatorVersion
			v3, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
			if err != nil || v3.Status != SuiteSupported || v3.ValidatorVersion != LatestSuiteValidatorVersion || v3.Consistency == nil || v3.Consistency.LedgerV3 == nil || v3.Consistency.Ledger != nil {
				t.Fatalf("v3 could not derive the supported expected request: %+v %v", v3, err)
			}
			v3Format := string(SuiteReviewFormatFor(ModelProfile{StructuredOutputs: true}, input))
			if !strings.Contains(v3Format, "vibe_suite_review_v3") || strings.Contains(v3Format, "\"obligations\"") || !strings.Contains(SuiteReviewMessages(input)[0].Content, ConsistencyReviewInstructionsV3) {
				t.Fatal("v3 asked the model to duplicate deterministic obligations")
			}
		})
	}
}

func TestSuiteReviewV3BlocksPresentFieldAndRejectsOldResponseFields(t *testing.T) {
	blueprint, input := suiteConsistencyReviewFixture(t, "Ask for both purchase age and condition.")
	input.ValidatorVersion = LatestSuiteValidatorVersion
	reply := supportedSuiteReview(input)
	ledger := suiteConsistencyLedgerReply(input.Cases[0].Expected, true)
	c := ledger["cases"].([]map[string]any)[0]
	delete(c, "obligations")
	reply["consistency"] = ledger
	result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
	if err != nil || result.Status != SuiteContradicted || result.Cases[0].Status != SuiteContradicted || len(result.Consistency.Findings) == 0 || SuiteValidationMatches(result, blueprint, input.Policy) {
		t.Fatalf("v3 lost the known-age contradiction: %+v %v", result, err)
	}
	for _, value := range []any{nil, []any{}, []map[string]any{{"field_id": "purchase-age", "kind": "ask"}}} {
		c["obligations"] = value
		if result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false)); err == nil || result != nil {
			t.Fatal("v3 silently accepted model-supplied obligations")
		}
	}
}

func TestSuiteReviewV3PinsStoredLedgerVersionAndCoverage(t *testing.T) {
	blueprint, input := suiteConsistencyReviewFixture(t, "Ask only for item condition.")
	input.ValidatorVersion = LatestSuiteValidatorVersion
	reply := supportedSuiteReview(input)
	ledger := suiteConsistencyLedgerReply(input.Cases[0].Expected, false)
	delete(ledger["cases"].([]map[string]any)[0], "obligations")
	reply["consistency"] = ledger
	result, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
	if err != nil || !SuiteValidationMatches(result, blueprint, input.Policy) {
		t.Fatalf("v3 supported ledger could not be retained: %+v %v", result, err)
	}
	for _, mutate := range []func(*SuiteValidation){
		func(v *SuiteValidation) { v.ValidatorVersion = ConsistencySuiteValidatorVersion },
		func(v *SuiteValidation) { v.Consistency = nil },
		func(v *SuiteValidation) { v.Consistency.LedgerV3 = nil },
		func(v *SuiteValidation) { v.Consistency.LedgerV3.Cases = nil },
		func(v *SuiteValidation) { v.Consistency.Ledger = &ConsistencyLedger{} },
		func(v *SuiteValidation) { v.Consistency.Required = false },
	} {
		var changed SuiteValidation
		if err := json.Unmarshal(raw(result), &changed); err != nil {
			t.Fatal(err)
		}
		mutate(&changed)
		if SuiteValidationMatches(&changed, blueprint, input.Policy) {
			t.Fatal("mixed, incomplete or differently versioned consistency evidence retained acceptance")
		}
	}
}
