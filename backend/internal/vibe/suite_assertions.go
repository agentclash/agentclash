package vibe

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

func applyAssertionFindings(result *SuiteValidation, input SuiteReviewInput) {
	subjects := map[string]string{}
	for _, a := range suiteAssertions(input) {
		subjects[a.ID] = a.Subject
	}
	for _, a := range result.Assertions {
		if a.Status == SuiteSupported {
			continue
		}
		update := func(f *SuiteReviewFinding) {
			if a.Status == SuiteContradicted || f.Status != SuiteContradicted {
				f.Status, f.Reason = a.Status, a.Reason
			}
		}
		if subjects[a.ID] == "shared_criteria" {
			update(&result.SharedCriteria)
		}
		for i := range result.Cases {
			if subjects[a.ID] == "case:"+result.Cases[i].CaseKey {
				update(&result.Cases[i].SuiteReviewFinding)
			}
		}
	}
}

func assertionReviewMatches(result *SuiteValidation, blueprint json.RawMessage, policy PolicySnapshot) bool {
	var root struct {
		Cases []struct {
			Key          string `json:"key"`
			Expectations []struct {
				Value string `json:"value"`
			} `json:"expectations"`
		} `json:"cases"`
		Judges []struct {
			Assertion string `json:"assertion"`
		} `json:"judges"`
	}
	if json.Unmarshal(blueprint, &root) != nil || len(root.Judges) != 1 {
		return false
	}
	input := SuiteReviewInput{ValidatorVersion: result.ValidatorVersion, Policy: policy, SharedCriteria: strings.TrimPrefix(root.Judges[0].Assertion, ScenarioCriteriaPrefix)}
	for _, c := range root.Cases {
		if len(c.Expectations) != 1 {
			return false
		}
		input.Cases = append(input.Cases, SuiteReviewCase{CaseKey: c.Key, Expected: c.Expectations[0].Value})
	}
	claims := suiteAssertions(input)
	if len(claims) != len(result.Assertions) {
		return false
	}
	ids := map[string]bool{}
	for _, c := range claims {
		ids[c.ID] = true
	}
	rules := map[string]PolicyRule{}
	for _, r := range policy.Rules {
		rules[r.ID] = r
	}
	for _, r := range result.Assertions {
		if result.ValidatorVersion == EntailmentSuiteValidatorVersion && (!validApplicabilityFor(r.Applicability, r.ID) || r.Applicability.OppositeAllowed) {
			return false
		}
		if !ids[r.ID] || r.Status != SuiteSupported || len(r.Support) == 0 || strings.TrimSpace(r.Reason) == "" {
			return false
		}
		delete(ids, r.ID)
		for _, e := range r.Support {
			if !containsString(rules[e.RuleID].SourceBlockIDs, e.SourceBlockID) || !containsString(r.RuleIDs, e.RuleID) || !containsString(r.SourceBlockIDs, e.SourceBlockID) || strings.TrimSpace(e.Quote) == "" {
				return false
			}
		}
	}
	return len(ids) == 0
}

const AssertionSuiteValidatorVersion = "suite-review-v4"
const EntailmentSuiteValidatorVersion = "suite-review-v5"

func assertionSuiteVersion(v string) bool {
	return v == AssertionSuiteValidatorVersion || v == EntailmentSuiteValidatorVersion
}

type AssertionApplicability struct {
	InputFacts      string `json:"input_facts"`
	RuleDirection   string `json:"rule_direction"`
	OppositeAllowed bool   `json:"opposite_allowed"`
	Explanation     string `json:"explanation"`
}

func validApplicability(a *AssertionApplicability) bool {
	return a != nil && strings.TrimSpace(a.InputFacts) != "" && strings.TrimSpace(a.RuleDirection) != "" && strings.TrimSpace(a.Explanation) != "" && len(a.InputFacts) <= 1200 && len(a.RuleDirection) <= 1200 && len(a.Explanation) <= 1200
}

func validApplicabilityFor(a *AssertionApplicability, id string) bool {
	return validApplicability(a) && (!strings.HasPrefix(id, "shared_criteria:") || a.InputFacts == "shared rule")
}

const entailmentReviewPrompt = `
For each assertion also return applicability: {input_facts, rule_direction, opposite_allowed, explanation}. First state the actual input facts (or "shared rule"), then the direction and conditions the original rule establishes. Independently ask: could a DIFFERENT decision from this expectation still comply with every supplied rule? If yes, opposite_allowed=true and status=unclear (or contradicted for a conflict). Never approve a decision merely because it is plausible. A=>B says nothing about not-A: "unknown senders are spam" DOES NOT establish "known senders are safe". Likewise "approve only if X" makes X necessary, not sufficient. Preserve negation, every condition, threshold inclusivity and explicitly stated precedence. Missing boundaries are gaps, not permission to guess. Mark unsupported cases so repair can replace them with situations that the actual rules decide. Do not invent policy to achieve the requested count. For shared_criteria assertions, input_facts MUST be exactly "shared rule". Review the global conditional against the source policy, WITHOUT using any particular case input. A source rule can be fully supported even when it is not triggered by a particular example. Set opposite_allowed=true only when the source permits violating that proposed global rule. Example: sources say "Unknown senders are spam. Known senders are safe." The shared clause "Unknown senders are spam" is supported with opposite_allowed=false, even if all current cases have known senders. By contrast "All senders are spam" adds an unsupported obligation. Do not demand that every shared rule decide every case.`

type SuiteAssertion struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
}
type AssertionSupport struct {
	RuleID        string `json:"rule_id"`
	SourceBlockID string `json:"source_block_id"`
	Quote         string `json:"quote"`
}
type SuiteAssertionReview struct {
	ID string `json:"id"`
	SuiteReviewFinding
	Support       []AssertionSupport      `json:"support"`
	Applicability *AssertionApplicability `json:"applicability,omitempty"`
}

// Server-selected spans cover all expected behavior, including prohibitions.
// Splitting is for coverage, not a claim to parse or prove natural language.
func suiteAssertions(input SuiteReviewInput) []SuiteAssertion {
	out := []SuiteAssertion{}
	add := func(subject, text string) {
		start, index := 0, 0
		emit := func(end int) {
			if value := strings.TrimSpace(text[start:end]); value != "" {
				index++
				out = append(out, SuiteAssertion{ID: fmt.Sprintf("%s:%d", subject, index), Subject: subject, Text: value})
			}
			start = end
		}
		for i, ch := range text {
			if ch == '\n' || ch == ';' || (ch == '.' || ch == '?' || ch == '!') && (i+1 == len(text) || unicode.IsSpace(rune(text[i+1]))) {
				emit(i + 1)
			}
		}
		emit(len(text))
	}
	for _, c := range input.Cases {
		add("case:"+c.CaseKey, c.Expected)
	}
	criteria := input.SharedCriteria
	if input.ValidatorVersion == EntailmentSuiteValidatorVersion && input.Policy.SourceVersion == SourcePolicyVersion && criteria == policyGradingCriteria(input.Policy.Rules) {
		// This exact server-owned heading merely applies the independently
		// reviewed rules. It is not a user obligation requiring its own quote.
		// Never strip arbitrary text or a mismatched/imported grading contract.
		criteria = strings.TrimPrefix(criteria, policyGradingPreamble)
	}
	add("shared_criteria", criteria)
	return out
}

const assertionReviewPrompt = `
Review assertions independently. The server's assertions cover ALL expected behavior and shared criteria. Return assertions with exactly one finding per supplied id, plus support: an array of {rule_id,source_block_id,quote} containing exact original source excerpts supporting that assertion.
A supported finding means EVERY clause is positively justified by the supplied rules AND applies to this input. Noncontradiction, harmlessness, generic safety advice, and a matching topic are NOT support. An extra prohibition is an invented obligation too: a spam policy does not justify an unrelated refund prohibition. Mark such an assertion contradicted even if another part is supported. Use unclear for genuinely missing business information. Unsupported findings may have empty support; supported findings must have exact supporting source quotes linked to current rules. A citation by itself is not entailment.
Also check whether initial cases exercise distinct decisions from the actual rules. If a rule uses OR, independently exercise its alternatives when possible: prize-only, bank-details-only, customer-order email. Do not accept superficial rewrites as new coverage. Mark a redundant case contradicted with a specific reason so the bounded repair can replace it. Never invent a rule to fill coverage.
Do not treat runtime limitations or instructions addressed to the test author as business requirements. Cases are fictional inputs; their facts may be made up, but their expected decisions must follow sourced rules.`

func assertionSchema() map[string]any {
	fields := suiteReviewSchema()["properties"].(map[string]any)["shared_criteria"].(map[string]any)["properties"].(map[string]any)
	fields["id"] = map[string]any{"type": "string", "minLength": 1}
	str := map[string]any{"type": "string", "minLength": 1}
	fields["support"] = map[string]any{"type": "array", "maxItems": MaxRequirements, "items": objectSchema(map[string]any{"rule_id": str, "source_block_id": str, "quote": str})}
	return map[string]any{"type": "array", "minItems": 1, "items": objectSchema(fields)}
}

func validateAssertionReviews(result *SuiteValidation, input SuiteReviewInput) error {
	claims := suiteAssertions(input)
	if len(claims) != len(result.Assertions) || len(claims) == 0 {
		return fmt.Errorf("review every supplied assertion exactly once, including prohibitions")
	}
	byID := map[string]SuiteAssertion{}
	for _, c := range claims {
		byID[c.ID] = c
	}
	sources := map[string]string{}
	for _, s := range input.Sources {
		sources[s.ID] = s.Text
	}
	rules := map[string]PolicyRule{}
	for _, r := range input.Policy.Rules {
		rules[r.ID] = r
	}
	for i := range result.Assertions {
		r := &result.Assertions[i]
		if input.ValidatorVersion == EntailmentSuiteValidatorVersion {
			if !validApplicabilityFor(r.Applicability, r.ID) {
				return fmt.Errorf("each assertion needs a bounded applicability audit; shared_criteria input_facts must be exactly shared rule, judged globally against sources rather than any case input")
			}
			if r.Applicability.OppositeAllowed && r.Status == SuiteSupported {
				r.Status, r.Reason = SuiteUnclear, r.Applicability.Explanation
			}
		}
		if _, ok := byID[r.ID]; !ok {
			return fmt.Errorf("unknown or repeated assertion")
		}
		delete(byID, r.ID)
		if r.Status != SuiteSupported && r.Status != SuiteContradicted && r.Status != SuiteUnclear || strings.TrimSpace(r.Reason) == "" || len(r.Reason) > 1200 {
			return fmt.Errorf("assertion requires a bounded semantic finding")
		}
		if r.Status == SuiteSupported && (len(r.Support) == 0 || len(r.RuleIDs) == 0 || len(r.SourceBlockIDs) == 0) {
			return fmt.Errorf("supported assertion requires positive original evidence")
		}
		for _, id := range r.RuleIDs {
			if _, ok := rules[id]; !ok {
				return fmt.Errorf("assertion references an unknown rule")
			}
		}
		for _, id := range r.SourceBlockIDs {
			if _, ok := sources[id]; !ok {
				return fmt.Errorf("assertion references an unknown source")
			}
		}
		for _, e := range r.Support {
			rule, ok := rules[e.RuleID]
			if !ok || !containsString(rule.SourceBlockIDs, e.SourceBlockID) || !containsString(r.RuleIDs, e.RuleID) || !containsString(r.SourceBlockIDs, e.SourceBlockID) || strings.TrimSpace(e.Quote) == "" || !strings.Contains(sources[e.SourceBlockID], e.Quote) {
				return fmt.Errorf("assertion support must quote its referenced rule's original source")
			}
			if input.Policy.SourceVersion == SourcePolicyVersion {
				inReviewedClause := false
				for _, clause := range rule.Evidence {
					if clause.SourceBlockID == e.SourceBlockID && strings.Contains(clause.Quote, e.Quote) {
						inReviewedClause = true
					}
				}
				if !inReviewedClause {
					return fmt.Errorf("assertion support must stay inside the cited rule's reviewed clause")
				}
			}
		}
	}
	return nil
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func entailmentAssertionSchema() map[string]any {
	schema := assertionSchema()
	fields := schema["items"].(map[string]any)["properties"].(map[string]any)
	text := map[string]any{"type": "string", "minLength": 1, "maxLength": 1200}
	fields["applicability"] = objectSchema(map[string]any{"input_facts": text, "rule_direction": text, "opposite_allowed": map[string]any{"type": "boolean"}, "explanation": text})
	schema["items"] = objectSchema(fields)
	return schema
}

// Structural constraints reduce omitted assertions and mistyped evidence IDs;
// they do not establish semantic support. The parser still checks exact coverage,
// uniqueness, source quotes, and the review's applicability findings.
func bindReviewEvidenceSchema(schema map[string]any, input SuiteReviewInput) {
	properties := schema["properties"].(map[string]any)
	rules, sources, cases := []string{}, []string{}, []string{}
	for _, r := range input.Policy.Rules {
		rules = append(rules, r.ID)
	}
	for _, s := range input.Sources {
		sources = append(sources, s.ID)
	}
	for _, c := range input.Cases {
		cases = append(cases, c.CaseKey)
	}
	enum := func(ids []string) map[string]any { return map[string]any{"type": "string", "enum": ids} }
	bindFinding := func(fields map[string]any) {
		fields["rule_ids"] = map[string]any{"type": "array", "items": enum(rules), "maxItems": len(rules)}
		fields["source_block_ids"] = map[string]any{"type": "array", "items": enum(sources), "maxItems": len(sources)}
	}
	supportVariants := []any{}
	for _, rule := range input.Policy.Rules {
		if len(rule.Evidence) > 0 {
			for _, evidence := range rule.Evidence {
				supportVariants = append(supportVariants, objectSchema(map[string]any{
					"rule_id": enum([]string{rule.ID}), "source_block_id": enum([]string{evidence.SourceBlockID}),
					"quote": enum([]string{evidence.Quote}),
				}))
			}
			continue
		}
		supportVariants = append(supportVariants, objectSchema(map[string]any{
			"rule_id": enum([]string{rule.ID}), "source_block_id": enum(rule.SourceBlockIDs),
			"quote": map[string]any{"type": "string", "minLength": 1},
		}))
	}
	for _, name := range []string{"cases", "rules", "shared_criteria", "policy_reconciliation"} {
		item := properties[name].(map[string]any)
		if name == "cases" || name == "rules" {
			ids, key := cases, "case_key"
			if name == "rules" {
				ids, key = rules, "rule_id"
			}
			item["minItems"], item["maxItems"] = len(ids), len(ids)
			item = item["items"].(map[string]any)
			item["properties"].(map[string]any)[key] = enum(ids)
		}
		bindFinding(item["properties"].(map[string]any))
	}
	claims := suiteAssertions(input)
	caseIDs, sharedIDs := []string{}, []string{}
	for _, a := range claims {
		if a.Subject == "shared_criteria" {
			sharedIDs = append(sharedIDs, a.ID)
		} else {
			caseIDs = append(caseIDs, a.ID)
		}
	}
	variants := []any{}
	for _, group := range []struct {
		ids    []string
		shared bool
	}{{caseIDs, false}, {sharedIDs, true}} {
		if len(group.ids) == 0 {
			continue
		}
		item := entailmentAssertionSchema()["items"].(map[string]any)
		fields := item["properties"].(map[string]any)
		fields["id"] = enum(group.ids)
		bindFinding(fields)
		fields["support"].(map[string]any)["items"] = map[string]any{"anyOf": supportVariants}
		if group.shared {
			audit := fields["applicability"].(map[string]any)["properties"].(map[string]any)
			audit["input_facts"] = enum([]string{"shared rule"})
		}
		variants = append(variants, item)
	}
	properties["assertions"] = map[string]any{"type": "array", "minItems": len(claims), "maxItems": len(claims), "items": map[string]any{"anyOf": variants}}
}
