package vibe

import (
	"encoding/json"
	"strings"
	"testing"
)

func missingOnlyConsistencyFixture(t *testing.T) (SuiteReviewInput, ConsistencyLedger) {
	t.Helper()
	_, input := suiteReviewFixture(t)
	source := &input.Sources[0]
	source.Text = "Only unopened items bought within 14 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund."
	source.Hash = Hash([]byte(source.Text))
	input.CurrentRequest = *source
	input.Policy.Rules = input.Policy.Rules[:1]
	input.Policy.Rules[0].Statement = source.Text
	input.Cases[0].Input = raw(map[string]string{"question": "My item was bought 10 days ago; I have not said its condition."})
	input.Cases[0].Expected = "Ask for both purchase age and condition."
	span := func(text string) ConsistencySourceSpan {
		return ConsistencySourceSpan{SourceBlockID: source.ID, Text: text}
	}
	ledger := ConsistencyLedger{
		Entities: []ConsistencyEntity{{ID: "item", Source: span("items")}},
		Fields: []ConsistencyField{
			{ID: "age", EntityID: "item", Kind: "number", Aliases: []ConsistencySourceSpan{span("purchase age")}},
			{ID: "condition", EntityID: "item", Kind: "enum", Aliases: []ConsistencySourceSpan{span("item condition"), span("condition")}},
		},
		MissingOnly: []MissingOnlyConstraint{{RuleID: input.Policy.Rules[0].ID, Source: span("Ask only for missing purchase age or item condition."), FieldIDs: []string{"age", "condition"}}},
		Cases: []CaseFactLedger{{CaseKey: input.Cases[0].CaseKey,
			Facts: []ConsistencyFact{
				{EntityID: "item", FieldID: "age", State: "present", InputPointer: "/question", Evidence: "My item was bought 10 days ago;", Literal: "10"},
				{EntityID: "item", FieldID: "condition", State: "missing", InputPointer: "/question", Evidence: "I have not said its condition.", Literal: ""},
			},
			Obligations: []ConsistencyObligation{
				{EntityID: "item", FieldID: "age", Kind: "ask", Evidence: input.Cases[0].Expected},
				{EntityID: "item", FieldID: "condition", Kind: "ask", Evidence: input.Cases[0].Expected},
			},
		}},
	}
	return input, ledger
}

func hasConsistencyFinding(findings []ConsistencyFinding, status, code string) bool {
	for _, finding := range findings {
		if finding.Status == status && finding.Code == code {
			return true
		}
	}
	return false
}

func TestConsistencyRejectsCapturedFalseAcceptanceFromExactExpectation(t *testing.T) {
	input, ledger := missingOnlyConsistencyFixture(t)
	if !RequiresConsistency(input) {
		t.Fatal("original missing-only instruction was missed")
	}
	got := CheckSuiteConsistency(input, ledger)
	if len(got) != 1 || got[0].Status != SuiteContradicted || got[0].Code != "asks_present_field" || got[0].FieldID != "age" || got[0].CaseKey != input.Cases[0].CaseKey {
		t.Fatalf("captured invented obligation was accepted: %+v", got)
	}
	// The v1 critic falsely claimed the expectation did not ask for age. Its
	// equivalent omission from this ledger cannot suppress the exact-text check.
	ledger.Cases[0].Obligations = ledger.Cases[0].Obligations[1:]
	got = CheckSuiteConsistency(input, ledger)
	if !hasConsistencyFinding(got, SuiteContradicted, "asks_present_field") || !hasConsistencyFinding(got, SuiteUnclear, "obligation_coverage") {
		t.Fatalf("omitted expectation escaped coverage: %+v", got)
	}
}

func TestConsistencyAllowsGroundedParaphrasesAndExplicitNegation(t *testing.T) {
	for _, expected := range []string{"Ask only for the item condition.", "Request the missing condition.", "Please ask for condition.", "Do not ask for purchase age. Ask for condition."} {
		t.Run(expected, func(t *testing.T) {
			input, ledger := missingOnlyConsistencyFixture(t)
			input.Cases[0].Expected = expected
			ledger.Cases[0].Obligations = []ConsistencyObligation{{EntityID: "item", FieldID: "condition", Kind: "ask", Evidence: expected}}
			if strings.HasPrefix(expected, "Do not") {
				ledger.Cases[0].Obligations = []ConsistencyObligation{{EntityID: "item", FieldID: "age", Kind: "do_not_ask", Evidence: "Do not ask for purchase age."}, {EntityID: "item", FieldID: "condition", Kind: "ask", Evidence: "Ask for condition."}}
			}
			if got := CheckSuiteConsistency(input, ledger); len(got) != 0 {
				t.Fatalf("grounded expected wording was rejected: %+v", got)
			}
		})
	}
}

func TestConsistencyLeavesNonRequestsAndUnrelatedUncertaintyToSemanticReview(t *testing.T) {
	for _, mode := range []string{"assertion_with_field", "unrelated_uncertain_fact", "negative_request"} {
		t.Run(mode, func(t *testing.T) {
			input, ledger := missingOnlyConsistencyFixture(t)
			ledger.Cases[0].Facts[0].State = "unclear"
			switch mode {
			case "assertion_with_field":
				input.Cases[0].Expected = "Explain that the item condition is ineligible."
				ledger.Cases[0].Obligations = nil
			case "unrelated_uncertain_fact":
				input.Cases[0].Expected = "Ask for condition."
				ledger.Cases[0].Obligations = []ConsistencyObligation{{EntityID: "item", FieldID: "condition", Kind: "ask", Evidence: input.Cases[0].Expected}}
			case "negative_request":
				input.Cases[0].Expected = "Do not ask for purchase age."
				ledger.Cases[0].Obligations = []ConsistencyObligation{{EntityID: "item", FieldID: "age", Kind: "do_not_ask", Evidence: input.Cases[0].Expected}}
			}
			if got := CheckSuiteConsistency(input, ledger); len(got) != 0 {
				t.Fatalf("narrow guard evaluated an unrelated assertion or fact: %+v", got)
			}
		})
	}
}

func TestConsistencyAbstainsOnConditionalsAlternativesAndUnclearFacts(t *testing.T) {
	for _, mode := range []string{"conditional_expected", "alternative_expected", "conditional_fact", "negated_fact", "clipped_negation", "unknown_fact", "partial_number", "converted_number", "source_negation", "unknown_alias", "ambiguous_alias", "changed_source"} {
		t.Run(mode, func(t *testing.T) {
			input, ledger := missingOnlyConsistencyFixture(t)
			switch mode {
			case "conditional_expected":
				input.Cases[0].Expected = "If purchase age is missing, ask for purchase age."
				ledger.Cases[0].Obligations = []ConsistencyObligation{{EntityID: "item", FieldID: "age", Kind: "ask", Evidence: input.Cases[0].Expected}}
			case "alternative_expected":
				input.Cases[0].Expected = "Ask for purchase age or condition."
				for i := range ledger.Cases[0].Obligations {
					ledger.Cases[0].Obligations[i].Evidence = input.Cases[0].Expected
				}
			case "conditional_fact", "negated_fact", "clipped_negation":
				text := "If my item was bought 10 days ago;"
				if mode != "conditional_fact" {
					text = "My item was not bought 10 days ago;"
				}
				input.Cases[0].Input = raw(map[string]string{"question": text + " I have not said its condition."})
				ledger.Cases[0].Facts[0].Evidence = text
				if mode == "clipped_negation" {
					ledger.Cases[0].Facts[0].Evidence = "bought 10 days ago;"
				}
			case "unknown_fact":
				ledger.Cases[0].Facts[0].State = "unclear"
			case "partial_number", "converted_number":
				value := "10.5"
				if mode == "converted_number" {
					value = "240 hours"
				}
				text := "My item was bought " + value + " days ago;"
				input.Cases[0].Input = raw(map[string]string{"question": text + " I have not said its condition."})
				ledger.Cases[0].Facts[0].Evidence = text
			case "source_negation":
				input.Sources[0].Text = strings.Replace(input.Sources[0].Text, "Ask only", "Never ask only", 1)
				input.Sources[0].Hash = Hash([]byte(input.Sources[0].Text))
				ledger.MissingOnly[0].Source.Text = "Never ask only for missing purchase age or item condition."
			case "unknown_alias":
				ledger.Fields[0].Aliases[0].Text = "delivery age"
			case "ambiguous_alias":
				ledger.Fields[1].Aliases = append(ledger.Fields[1].Aliases, ledger.Fields[0].Aliases[0])
			case "changed_source":
				input.Sources[0].Text += " Changed without its original hash."
			}
			got := CheckSuiteConsistency(input, ledger)
			if len(got) == 0 {
				t.Fatal("unresolved extraction was accepted")
			}
			for _, finding := range got {
				if finding.Status != SuiteUnclear {
					t.Fatalf("unsupported interpretation was called a contradiction: %+v", got)
				}
			}
		})
	}
}

func TestConsistencyRequiresCompleteGroundedCoverage(t *testing.T) {
	for _, mode := range []string{"empty", "missing_field", "missing_case", "missing_fact", "unknown_entity", "unknown_rule", "omitted_source_rule", "truncated_source_rule", "invented_obligation", "invented_fact", "duplicate_fact", "bad_pointer"} {
		t.Run(mode, func(t *testing.T) {
			input, ledger := missingOnlyConsistencyFixture(t)
			switch mode {
			case "empty":
				ledger = ConsistencyLedger{}
			case "missing_field":
				ledger.Fields = ledger.Fields[1:]
				ledger.MissingOnly[0].FieldIDs = []string{"condition"}
				ledger.Cases[0].Facts = ledger.Cases[0].Facts[1:]
				ledger.Cases[0].Obligations = ledger.Cases[0].Obligations[1:]
			case "missing_case":
				ledger.Cases = nil
			case "missing_fact":
				ledger.Cases[0].Facts = ledger.Cases[0].Facts[1:]
			case "unknown_entity":
				ledger.Cases[0].Facts[0].EntityID = "another-item"
			case "unknown_rule":
				ledger.MissingOnly[0].RuleID = "invented-rule"
			case "omitted_source_rule":
				ledger.MissingOnly = nil
			case "truncated_source_rule":
				ledger.MissingOnly[0].Source.Text = "missing purchase age or item condition."
			case "invented_obligation":
				ledger.Cases[0].Obligations[0].Evidence = "Ask for condition."
			case "invented_fact":
				ledger.Cases[0].Facts[0].Evidence = "My item was bought 9 days ago;"
			case "duplicate_fact":
				ledger.Cases[0].Facts[1] = ledger.Cases[0].Facts[0]
			case "bad_pointer":
				ledger.Cases[0].Facts[0].InputPointer = "/does-not-exist"
			}
			got := CheckSuiteConsistency(input, ledger)
			if len(got) == 0 {
				t.Fatal("incomplete ledger escaped the guard")
			}
		})
	}
}

func TestConsistencyHasNoDomainKeywordDependency(t *testing.T) {
	input, ledger := missingOnlyConsistencyFixture(t)
	replacements := strings.NewReplacer("items", "reservations", "item", "reservation", "purchase age", "arrival date", "condition", "room type", "10 days ago", "2026-09-30")
	var changedInput SuiteReviewInput
	var changedLedger ConsistencyLedger
	if err := json.Unmarshal([]byte(replacements.Replace(string(raw(input)))), &changedInput); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(replacements.Replace(string(raw(ledger)))), &changedLedger); err != nil {
		t.Fatal(err)
	}
	changedInput.Sources[0].Hash = Hash([]byte(changedInput.Sources[0].Text))
	changedInput.Cases[0].Expected = "Request only room type."
	changedLedger.Fields[0].Kind = "text"
	changedLedger.Cases[0].Facts[0].Literal = "2026-09-30"
	changedLedger.Cases[0].Obligations = []ConsistencyObligation{{EntityID: "reservation", FieldID: "room type", Kind: "ask", Evidence: changedInput.Cases[0].Expected}}
	if got := CheckSuiteConsistency(changedInput, changedLedger); len(got) != 0 {
		t.Fatalf("generic reservation fields were rejected: %+v", got)
	}
	_, ordinary := suiteReviewFixture(t)
	if RequiresConsistency(ordinary) || len(CheckSuiteConsistency(ordinary, ConsistencyLedger{})) != 0 {
		t.Fatal("an unrelated policy required this narrow missing-only check")
	}
}

func TestConsistencySchemaRequiresBoundedClosedObjects(t *testing.T) {
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if v["type"] == "object" {
				properties := v["properties"].(map[string]any)
				required := v["required"].([]string)
				if v["additionalProperties"] != false || len(properties) != len(required) {
					t.Fatal("ledger schema contains an open or optional object field")
				}
			}
			if v["type"] == "string" && v["enum"] == nil && v["maxLength"] == nil || v["type"] == "array" && v["maxItems"] == nil {
				t.Fatal("ledger schema has an unbounded field")
			}
			for _, child := range v {
				visit(child)
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		}
	}
	visit(ConsistencyLedgerSchema())
	visit(ConsistencyLedgerSchemaV3())
}

func consistencyV3Fixture(legacy ConsistencyLedger) ConsistencyLedgerV3 {
	ledger := ConsistencyLedgerV3{Entities: legacy.Entities, Fields: legacy.Fields, MissingOnly: legacy.MissingOnly}
	for _, c := range legacy.Cases {
		ledger.Cases = append(ledger.Cases, CaseFactLedgerV3{CaseKey: c.CaseKey, Facts: c.Facts})
	}
	return ledger
}

func TestConsistencyV3DerivesAsksWithoutRedundantModelEvidence(t *testing.T) {
	input, legacy := missingOnlyConsistencyFixture(t)
	input.Cases[0].Expected = "Ask only for condition."
	// The live positive control copied its SOURCE rule into obligation.evidence.
	// V2 must retain that historical rejection; V3 never requests this duplicate.
	legacy.Cases[0].Obligations = []ConsistencyObligation{{EntityID: "item", FieldID: "condition", Kind: "ask", Evidence: legacy.MissingOnly[0].Source.Text}}
	if got := CheckSuiteConsistency(input, legacy); !hasConsistencyFinding(got, SuiteUnclear, "obligation_evidence") {
		t.Fatalf("v2 historical behavior changed: %+v", got)
	}
	ledger := consistencyV3Fixture(legacy)
	if got := CheckSuiteConsistencyV3(input, ledger); len(got) != 0 {
		t.Fatalf("a redundant model obligation blocked the correct expected request: %+v", got)
	}
	input.Cases[0].Expected = "Ask for both purchase age and condition."
	if got := CheckSuiteConsistencyV3(input, ledger); !hasConsistencyFinding(got, SuiteContradicted, "asks_present_field") {
		t.Fatalf("server-derived asks missed the captured contradiction: %+v", got)
	}
}

func TestConsistencyV3AllowsOnlyOmittedTrailingClausePunctuation(t *testing.T) {
	input, legacy := missingOnlyConsistencyFixture(t)
	ledger := consistencyV3Fixture(legacy)
	ledger.MissingOnly[0].Source.Text = strings.TrimSuffix(ledger.MissingOnly[0].Source.Text, ".")
	ledger.Cases[0].Facts[0].Evidence = strings.TrimSuffix(ledger.Cases[0].Facts[0].Evidence, ";")
	ledger.Cases[0].Facts[1].Evidence = strings.TrimSuffix(ledger.Cases[0].Facts[1].Evidence, ".")
	if got := CheckSuiteConsistencyV3(input, ledger); len(got) != 1 || !hasConsistencyFinding(got, SuiteContradicted, "asks_present_field") {
		t.Fatalf("omitted trailing punctuation hid a concrete contradiction: %+v", got)
	}
	for _, mode := range []string{"clipped_subject", "clipped_negation", "clipped_condition", "replacement_punctuation", "internal_whitespace", "ambiguous_clause"} {
		t.Run(mode, func(t *testing.T) {
			input, old := missingOnlyConsistencyFixture(t)
			ledger := consistencyV3Fixture(old)
			switch mode {
			case "clipped_subject":
				ledger.Cases[0].Facts[0].Evidence = "bought 10 days ago"
			case "clipped_negation":
				input.Cases[0].Input = raw(map[string]string{"question": "My item was not bought 10 days ago; I have not said its condition."})
				ledger.Cases[0].Facts[0].Evidence = "bought 10 days ago"
			case "clipped_condition":
				input.Cases[0].Input = raw(map[string]string{"question": "If my item was bought 10 days ago; I have not said its condition."})
				ledger.Cases[0].Facts[0].Evidence = "my item was bought 10 days ago"
			case "replacement_punctuation":
				ledger.Cases[0].Facts[0].Evidence = "My item was bought 10 days ago."
			case "internal_whitespace":
				ledger.Cases[0].Facts[0].Evidence = "My item was bought  10 days ago"
			case "ambiguous_clause":
				input.Cases[0].Input = raw(map[string]string{"question": "My item was bought 10 days ago; My item was bought 10 days ago. I have not said its condition."})
				ledger.Cases[0].Facts[0].Evidence = "My item was bought 10 days ago"
			}
			got := CheckSuiteConsistencyV3(input, ledger)
			if !hasConsistencyFinding(got, SuiteUnclear, "present_fact_evidence") || hasConsistencyFinding(got, SuiteContradicted, "asks_present_field") {
				t.Fatalf("punctuation tolerance changed the meaning of evidence: %+v", got)
			}
		})
	}
}

func TestConsistencyV3ResolvesOnlyUniqueGroundedSuffixAliases(t *testing.T) {
	input, legacy := missingOnlyConsistencyFixture(t)
	legacy.Fields[1].Aliases = legacy.Fields[1].Aliases[:1] // source name: item condition
	input.Cases[0].Expected = "Ask only for condition."
	legacy.Cases[0].Obligations = []ConsistencyObligation{{EntityID: "item", FieldID: "condition", Kind: "ask", Evidence: input.Cases[0].Expected}}
	if got := CheckSuiteConsistency(input, legacy); !hasConsistencyFinding(got, SuiteUnclear, "expected_fields") {
		t.Fatalf("v2 alias resolution changed: %+v", got)
	}
	ledger := consistencyV3Fixture(legacy)
	if got := CheckSuiteConsistencyV3(input, ledger); len(got) != 0 {
		t.Fatalf("unique condition suffix was not resolved: %+v", got)
	}
	input.Cases[0].Expected = "Ask for age and condition."
	if got := CheckSuiteConsistencyV3(input, ledger); !hasConsistencyFinding(got, SuiteContradicted, "asks_present_field") {
		t.Fatalf("unique age suffix lost its fact identity: %+v", got)
	}

	// An explicit short alias must not win over another field's matching suffix.
	source := &input.Sources[0]
	source.Text = "Handle deliveries. Ask only for missing address or billing address."
	source.Hash = Hash([]byte(source.Text))
	span := func(text string) ConsistencySourceSpan {
		return ConsistencySourceSpan{SourceBlockID: source.ID, Text: text}
	}
	input.Policy.Rules[0].Statement = source.Text
	input.Cases[0].Expected = "Ask for address."
	ledger.Entities = []ConsistencyEntity{{ID: "delivery", Source: span("deliveries")}}
	ledger.Fields = []ConsistencyField{{ID: "shipping", EntityID: "delivery", Kind: "text", Aliases: []ConsistencySourceSpan{span("address")}}, {ID: "billing", EntityID: "delivery", Kind: "text", Aliases: []ConsistencySourceSpan{span("billing address")}}}
	// The bare name occurs twice; using the full source phrase below also covers
	// cases where each longer alias has a unique source span.
	source.Text = "Handle deliveries. Ask only for missing shipping address or billing address."
	source.Hash = Hash([]byte(source.Text))
	input.Policy.Rules[0].Statement = source.Text
	ledger.Fields[0].Aliases = []ConsistencySourceSpan{span("shipping address")}
	ledger.MissingOnly = []MissingOnlyConstraint{{RuleID: input.Policy.Rules[0].ID, Source: span("Ask only for missing shipping address or billing address."), FieldIDs: []string{"shipping", "billing"}}}
	ledger.Cases[0].Facts = []ConsistencyFact{{EntityID: "delivery", FieldID: "shipping", State: "missing", InputPointer: "/question"}, {EntityID: "delivery", FieldID: "billing", State: "missing", InputPointer: "/question"}}
	got := CheckSuiteConsistencyV3(input, ledger)
	if !hasConsistencyFinding(got, SuiteUnclear, "expected_fields") {
		t.Fatalf("ambiguous address suffix was assigned to one field: %+v", got)
	}
}

func TestConsistencyV3SchemaAndDecoderExcludeModelObligations(t *testing.T) {
	_, legacy := missingOnlyConsistencyFixture(t)
	var current ConsistencyLedgerV3
	if err := Decode(raw(legacy), LimitsFor(false), &current); err == nil {
		t.Fatal("v3 decoder silently accepted legacy model obligations")
	}
	before := string(raw(ConsistencyLedgerSchema()))
	schema := ConsistencyLedgerSchemaV3()
	cases := schema["properties"].(map[string]any)["cases"].(map[string]any)
	properties := cases["items"].(map[string]any)["properties"].(map[string]any)
	if _, ok := properties["obligations"]; ok || len(properties) != 2 {
		t.Fatal("v3 schema still asks the model to restate expected obligations")
	}
	if before != string(raw(ConsistencyLedgerSchema())) {
		t.Fatal("building v3 mutated the frozen v2 schema")
	}
	ledger := consistencyV3Fixture(legacy)
	if err := Decode(raw(ledger), LimitsFor(false), &current); err != nil {
		t.Fatalf("v3 ledger did not match its declared struct: %v", err)
	}
}

func TestConsistencyV3NegativeAlternativesPreserveCapturedValidExpectation(t *testing.T) {
	input, legacy := missingOnlyConsistencyFixture(t)
	input.Sources[0].Text = strings.Replace(input.Sources[0].Text, "14 days", "30 days", 1)
	input.Sources[0].Hash = Hash([]byte(input.Sources[0].Text))
	input.Policy.Rules[0].Statement = input.Sources[0].Text
	input.Cases[0].Input = raw(map[string]string{"question": "I bought this unopened item 10 days ago. Can I return it?"})
	legacy.Cases[0].Facts[0].Evidence = "I bought this unopened item 10 days ago."
	legacy.Cases[0].Facts[1].State = "present"
	legacy.Cases[0].Facts[1].Evidence = legacy.Cases[0].Facts[0].Evidence
	legacy.Cases[0].Facts[1].Literal = "unopened"
	for _, prohibition := range []string{"Do not ask for purchase age or item condition.", "Never ask for purchase age or item condition.", "Don't ask for purchase age or item condition."} {
		t.Run(prohibition, func(t *testing.T) {
			// The first variant is the exact expectation from the original v3
			// live preparation. Neither prohibited question requests a known fact.
			input.Cases[0].Expected = "State that the item is eligible for return because it is unopened and bought within 30 days. " + prohibition + " Do not claim to process a refund."
			legacy.Cases[0].Obligations = []ConsistencyObligation{{EntityID: "item", FieldID: "age", Kind: "do_not_ask", Evidence: prohibition}, {EntityID: "item", FieldID: "condition", Kind: "do_not_ask", Evidence: prohibition}}
			if got := CheckSuiteConsistencyV3(input, consistencyV3Fixture(legacy)); len(got) != 0 {
				t.Fatalf("explicit prohibition of both questions was rejected: %+v", got)
			}
			if got := CheckSuiteConsistency(input, legacy); !hasConsistencyFinding(got, SuiteUnclear, "expected_fields") {
				t.Fatalf("v2 negative-alternative behavior changed: %+v", got)
			}
		})
	}
	input.Cases[0].Expected = "Ask for purchase age or item condition."
	for _, got := range [][]ConsistencyFinding{CheckSuiteConsistency(input, legacy), CheckSuiteConsistencyV3(input, consistencyV3Fixture(legacy))} {
		if !hasConsistencyFinding(got, SuiteUnclear, "expected_fields") || hasConsistencyFinding(got, SuiteContradicted, "asks_present_field") {
			t.Fatalf("a positive choice was treated as two unconditional requests: %+v", got)
		}
	}
}
