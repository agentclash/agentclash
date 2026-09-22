package vibe

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxConsistencyFields = 16

// The ledger records a model's interpretation of named fields and scenario
// facts. Exact quotations prove location, not entailment. The checks below are
// deliberately limited to missing-only requests; they are not a policy DSL.
type ConsistencyLedger struct {
	Entities    []ConsistencyEntity     `json:"entities"`
	Fields      []ConsistencyField      `json:"fields"`
	MissingOnly []MissingOnlyConstraint `json:"missing_only"`
	Cases       []CaseFactLedger        `json:"cases"`
}

// V3 removes redundant model-written obligations. The server derives requests
// directly from the exact expected text; the paid attempt still retains the raw
// model response for diagnostics.
type ConsistencyLedgerV3 struct {
	Entities    []ConsistencyEntity     `json:"entities"`
	Fields      []ConsistencyField      `json:"fields"`
	MissingOnly []MissingOnlyConstraint `json:"missing_only"`
	Cases       []CaseFactLedgerV3      `json:"cases"`
}

type CaseFactLedgerV3 struct {
	CaseKey string            `json:"case_key"`
	Facts   []ConsistencyFact `json:"facts"`
}

type ConsistencySourceSpan struct {
	SourceBlockID string `json:"source_block_id"`
	Text          string `json:"text"`
}

type ConsistencyEntity struct {
	ID     string                `json:"id"`
	Source ConsistencySourceSpan `json:"source"`
}

type ConsistencyField struct {
	ID       string                  `json:"id"`
	EntityID string                  `json:"entity_id"`
	Kind     string                  `json:"kind"` // number, enum, or text; no unit conversion
	Aliases  []ConsistencySourceSpan `json:"aliases"`
}

type MissingOnlyConstraint struct {
	RuleID   string                `json:"rule_id"`
	Source   ConsistencySourceSpan `json:"source"`
	FieldIDs []string              `json:"field_ids"`
}

type CaseFactLedger struct {
	CaseKey     string                  `json:"case_key"`
	Facts       []ConsistencyFact       `json:"facts"`
	Obligations []ConsistencyObligation `json:"obligations"`
}

type ConsistencyFact struct {
	EntityID     string `json:"entity_id"`
	FieldID      string `json:"field_id"`
	State        string `json:"state"` // present, missing, or unclear
	InputPointer string `json:"input_pointer"`
	Evidence     string `json:"evidence"`
	Literal      string `json:"literal"`
}

type ConsistencyObligation struct {
	EntityID string `json:"entity_id"`
	FieldID  string `json:"field_id"`
	Kind     string `json:"kind"` // ask or do_not_ask
	Evidence string `json:"evidence"`
}

type ConsistencyFinding struct {
	CaseKey string `json:"case_key"`
	FieldID string `json:"field_id"`
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

const ConsistencyReviewInstructions = `When consistency is required, also return a consistency ledger. This is a narrow missing-information check, not a claim that arbitrary prose has been proved.
Use stable entity and field IDs. Ground entity names and every field alias in an exact, unique original-source substring. Alias-to-field and scenario-fact mappings are your interpretations: report unclear if ambiguous. Do not invent an alias or change the meaning of a name. Cite the complete original missing-only instruction, including negation or conditions, and every field it names. Never omit a named field to make the check pass.
For every case, give exactly one present, missing, or unclear fact per declared field. input_pointer is a JSON Pointer to a string in the supplied input object. Present facts require an exact unique COMPLETE input clause and the literal value within it; never cut off negation, a condition, a disjunction or uncertainty. Number literals must be exact decimal text, with no conversion or arithmetic. A missing fact may have empty evidence and literal; it is an interpretation of absence, not proof. Use unclear if a fact cannot be resolved.
Extract obligations from the ACTUAL expected text, not from the behavior you think it should request. Include every ask and do_not_ask obligation for a recognized field, quoting its entire clause. "Ask for both A and B" has TWO ask obligations, even if A is already present. Do not silently correct it to ask only for B. Conditional, disjunctive, quoted, or otherwise unsupported instructions must be treated as unclear, not rewritten as unconditional requests. Do not provide confidence scores.`

const ConsistencyReviewInstructionsV3 = `When consistency is required, also return a consistency ledger containing entities, fields, missing_only, and cases. This is a narrow missing-information check, not a claim that arbitrary prose has been proved.
Use stable entity and field IDs. Ground entity names and every field alias in an exact, unique original-source substring. Alias-to-field and scenario-fact mappings are your interpretations: report unclear if ambiguous. Do not invent an alias or change the meaning of a name. Cite the complete original missing-only instruction, including negation or conditions, and every field it names. Never omit a named field to make the check pass.
For every case, give exactly one present, missing, or unclear fact per declared field. input_pointer is a JSON Pointer to a string in the supplied input object. Present facts require an exact unique COMPLETE input clause and the literal value within it. Its final sentence delimiter may be omitted, but never cut off leading words, negation, a condition, a disjunction or uncertainty. Number literals must be exact decimal text, with no conversion or arithmetic. A missing fact may have empty evidence and literal; it is an interpretation of absence, not proof. Use unclear if a fact cannot be resolved.
Cases contain only case_key and facts. Do not output obligations or rewrite the expected behavior. The server separately derives information requests from the original expected text. Review that exact text semantically as well: "Ask for both A and B" requests both, even if A is already present. Do not provide confidence scores.`

func ConsistencyLedgerSchema() map[string]any {
	text := func(max int, empty bool) map[string]any {
		min := 1
		if empty {
			min = 0
		}
		return map[string]any{"type": "string", "minLength": min, "maxLength": max}
	}
	list := func(item any, min, max int) map[string]any {
		return map[string]any{"type": "array", "items": item, "minItems": min, "maxItems": max}
	}
	enum := func(values ...string) map[string]any { return map[string]any{"type": "string", "enum": values} }
	id := text(MaxKeyBytes, false)
	span := objectSchema(map[string]any{"source_block_id": id, "text": text(4096, false)})
	entity := objectSchema(map[string]any{"id": id, "source": span})
	field := objectSchema(map[string]any{"id": id, "entity_id": id, "kind": enum("number", "enum", "text"), "aliases": list(span, 1, 8)})
	rule := objectSchema(map[string]any{"rule_id": id, "source": span, "field_ids": list(id, 1, maxConsistencyFields)})
	fact := objectSchema(map[string]any{"entity_id": id, "field_id": id, "state": enum("present", "missing", "unclear"), "input_pointer": text(512, false), "evidence": text(4096, true), "literal": text(256, true)})
	obligation := objectSchema(map[string]any{"entity_id": id, "field_id": id, "kind": enum("ask", "do_not_ask"), "evidence": text(4096, false)})
	c := objectSchema(map[string]any{"case_key": id, "facts": list(fact, 1, maxConsistencyFields), "obligations": list(obligation, 0, 2*maxConsistencyFields)})
	return objectSchema(map[string]any{"entities": list(entity, 1, maxConsistencyFields), "fields": list(field, 1, maxConsistencyFields), "missing_only": list(rule, 1, maxConsistencyFields), "cases": list(c, 1, 200)})
}

func ConsistencyLedgerSchemaV3() map[string]any {
	schema := ConsistencyLedgerSchema()
	properties := schema["properties"].(map[string]any)
	cases := properties["cases"].(map[string]any)
	fields := cases["items"].(map[string]any)["properties"].(map[string]any)
	delete(fields, "obligations")
	cases["items"] = objectSchema(fields)
	return schema
}

func consistencyLedgerV3Base(ledger ConsistencyLedgerV3) ConsistencyLedger {
	base := ConsistencyLedger{Entities: ledger.Entities, Fields: ledger.Fields, MissingOnly: ledger.MissingOnly}
	for _, c := range ledger.Cases {
		base.Cases = append(base.Cases, CaseFactLedger{CaseKey: c.CaseKey, Facts: c.Facts})
	}
	return base
}

func CheckConsistencyLedgerShapeV3(ledger ConsistencyLedgerV3, caseKeys []string) error {
	return consistencyLedgerShapeV3(consistencyLedgerV3Base(ledger), caseKeys)
}

func consistencyLedgerShapeV3(ledger ConsistencyLedger, caseKeys []string) error {
	if err := CheckConsistencyLedgerShape(ledger, caseKeys); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, rule := range ledger.MissingOnly {
		key := rule.Source.SourceBlockID + "\x00" + consistencyClauseBody(rule.Source.Text)
		if seen[key] {
			return fmt.Errorf("consistency ledger repeats a source instruction")
		}
		seen[key] = true
	}
	return nil
}

var missingInstruction = regexp.MustCompile(`(?i)\b(?:ask|request)\b.*\bonly\b.*\bmissing\b|\bonly\b.*\b(?:ask|request)\b.*\bmissing\b`)
var simpleMissingInstruction = regexp.MustCompile(`^(?:ask only for (?:the )?missing|only ask for (?:the )?missing|request only (?:the )?missing) (.+)$`)
var positiveAsk = regexp.MustCompile(`^(?:please )?(?:ask(?: only)? for|request(?: only)?(?: for)?) (.+)$`)
var negativeAsk = regexp.MustCompile(`^(?:do not|don't|never) (?:ask(?: only)? for|request(?: only)?(?: for)?) (.+)$`)
var askWord = regexp.MustCompile(`\b(?:ask|request)\b`)
var uncertainFact = regexp.MustCompile(`(?i)\b(?:not|no|never|if|unless|maybe|might|would|could|unknown|missing|absent|either|or|except|about|around|approximately)\b|n['’]t\b|["“”]`)
var decimalLiteral = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
var numberToken = regexp.MustCompile(`[+-]?[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?`)

type missingSourceClause struct {
	id, text string
}

// Legacy reviews scan whole source blocks. The source-boundary contract checks
// exact requirement evidence; citing one clause must not adopt every other
// sentence in that message. The semantic review still sees the full request and
// rejects omitted rules or misleading evidence selection independently.
func RequiresConsistency(input SuiteReviewInput) bool { return len(missingSourceClauses(input)) != 0 }

func missingSourceClauses(input SuiteReviewInput) []missingSourceClause {
	used := map[string]bool{}
	for _, rule := range input.Policy.Rules {
		for _, id := range rule.SourceBlockIDs {
			used[id] = true
		}
	}
	var out []missingSourceClause
	seen := map[string]bool{}
	sources := input.Sources
	if input.Policy.SourceVersion == SourcePolicyVersion {
		sources = nil
		for _, rule := range input.Policy.Rules {
			for _, evidence := range rule.Evidence {
				if evidence.Kind == "requirement" {
					sources = append(sources, SourceBlock{ID: evidence.SourceBlockID, Text: evidence.Quote})
				}
			}
		}
	}
	for _, source := range sources {
		if !used[source.ID] {
			continue
		}
		for _, clause := range consistencyClauses(source.Text) {
			key := source.ID + "\x00" + clause
			if missingInstruction.MatchString(clause) && !seen[key] {
				seen[key] = true
				out = append(out, missingSourceClause{source.ID, clause})
			}
		}
	}
	return out
}

// CheckConsistencyLedgerShape validates the server-stored trace without claiming
// to rerun source grounding. The caller must separately bind its saved review to
// the exact suite, policy, and validator version that were originally checked.
func CheckConsistencyLedgerShape(ledger ConsistencyLedger, caseKeys []string) error {
	bad := func() error {
		return fmt.Errorf("consistency ledger has incomplete, ambiguous, or unbounded references")
	}
	idOK := func(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= MaxKeyBytes }
	spanOK := func(s ConsistencySourceSpan) bool {
		return idOK(s.SourceBlockID) && strings.TrimSpace(s.Text) != "" && len(s.Text) <= 4096
	}
	if len(ledger.Entities) < 1 || len(ledger.Entities) > maxConsistencyFields || len(ledger.Fields) < 1 || len(ledger.Fields) > maxConsistencyFields || len(ledger.MissingOnly) < 1 || len(ledger.MissingOnly) > maxConsistencyFields || len(caseKeys) < 1 || len(caseKeys) > 200 || len(ledger.Cases) != len(caseKeys) {
		return bad()
	}
	entities, fields := map[string]bool{}, map[string]ConsistencyField{}
	for _, entity := range ledger.Entities {
		if !idOK(entity.ID) || entities[entity.ID] || !spanOK(entity.Source) {
			return bad()
		}
		entities[entity.ID] = true
	}
	for _, field := range ledger.Fields {
		if !idOK(field.ID) || fields[field.ID].ID != "" || !entities[field.EntityID] || (field.Kind != "number" && field.Kind != "enum" && field.Kind != "text") || len(field.Aliases) < 1 || len(field.Aliases) > 8 {
			return bad()
		}
		aliases := map[string]bool{}
		for _, alias := range field.Aliases {
			key := alias.SourceBlockID + "\x00" + alias.Text
			if !spanOK(alias) || aliases[key] {
				return bad()
			}
			aliases[key] = true
		}
		fields[field.ID] = field
	}
	usedFields, constraints := map[string]bool{}, map[string]bool{}
	for _, rule := range ledger.MissingOnly {
		key := rule.Source.SourceBlockID + "\x00" + rule.Source.Text
		if !idOK(rule.RuleID) || !spanOK(rule.Source) || constraints[key] || len(rule.FieldIDs) < 1 || len(rule.FieldIDs) > len(fields) {
			return bad()
		}
		constraints[key] = true
		seen := map[string]bool{}
		for _, id := range rule.FieldIDs {
			if fields[id].ID == "" || seen[id] {
				return bad()
			}
			seen[id], usedFields[id] = true, true
		}
	}
	if len(usedFields) != len(fields) {
		return bad()
	}
	keys := map[string]bool{}
	for _, key := range caseKeys {
		if !idOK(key) || keys[key] {
			return bad()
		}
		keys[key] = true
	}
	seenCases := map[string]bool{}
	for _, c := range ledger.Cases {
		if !keys[c.CaseKey] || seenCases[c.CaseKey] || len(c.Facts) != len(fields) || len(c.Obligations) > 2*maxConsistencyFields {
			return bad()
		}
		seenCases[c.CaseKey] = true
		facts := map[string]bool{}
		for _, fact := range c.Facts {
			field := fields[fact.FieldID]
			if field.ID == "" || field.EntityID != fact.EntityID || facts[fact.FieldID] || (fact.State != "present" && fact.State != "missing" && fact.State != "unclear") || len(fact.InputPointer) < 1 || len(fact.InputPointer) > 512 || fact.InputPointer[0] != '/' || len(fact.Evidence) > 4096 || len(fact.Literal) > 256 {
				return bad()
			}
			facts[fact.FieldID] = true
		}
		obligations := map[string]bool{}
		for _, obligation := range c.Obligations {
			field := fields[obligation.FieldID]
			key := obligation.FieldID + "\x00" + obligation.Kind + "\x00" + obligation.Evidence
			if field.ID == "" || field.EntityID != obligation.EntityID || (obligation.Kind != "ask" && obligation.Kind != "do_not_ask") || strings.TrimSpace(obligation.Evidence) == "" || len(obligation.Evidence) > 4096 || obligations[key] {
				return bad()
			}
			obligations[key] = true
		}
	}
	return nil
}

// CheckSuiteConsistency produces only contradictions or abstentions. An empty
// result means this narrow guard found no conflict, not that the suite is true.
func CheckSuiteConsistency(input SuiteReviewInput, ledger ConsistencyLedger) []ConsistencyFinding {
	return checkSuiteConsistency(input, ledger, false)
}

func CheckSuiteConsistencyV3(input SuiteReviewInput, ledger ConsistencyLedgerV3) []ConsistencyFinding {
	return checkSuiteConsistency(input, consistencyLedgerV3Base(ledger), true)
}

func checkSuiteConsistency(input SuiteReviewInput, ledger ConsistencyLedger, v3 bool) []ConsistencyFinding {
	var findings []ConsistencyFinding
	add := func(caseKey, fieldID, status, code, message string) {
		findings = append(findings, ConsistencyFinding{CaseKey: caseKey, FieldID: fieldID, Status: status, Code: code, Message: message})
	}
	clauses := missingSourceClauses(input)
	if len(clauses) == 0 {
		if len(ledger.MissingOnly) != 0 {
			add("", "", SuiteUnclear, "unsupported_missing_rule", "The supplied missing-only rule is not resolved by this consistency check.")
		}
		return findings
	}
	caseKeys := make([]string, 0, len(input.Cases))
	for _, c := range input.Cases {
		caseKeys = append(caseKeys, c.CaseKey)
	}
	shape := CheckConsistencyLedgerShape
	wholeClause := wholeConsistencyClause
	sourceKey := func(id, text string) string { return id + "\x00" + text }
	if v3 {
		shape = consistencyLedgerShapeV3
		wholeClause = wholeConsistencyClauseV3
		sourceKey = func(id, text string) string { return id + "\x00" + consistencyClauseBody(text) }
	}
	if err := shape(ledger, caseKeys); err != nil {
		add("", "", SuiteUnclear, "consistency_coverage", err.Error())
		return findings
	}
	sources := map[string]SourceBlock{}
	for _, source := range input.Sources {
		if source.Hash != Hash([]byte(source.Text)) || sources[source.ID].ID != "" && sources[source.ID] != source {
			add("", "", SuiteUnclear, "source_mismatch", "Original source identity or text changed.")
			return findings
		}
		sources[source.ID] = source
	}
	grounded := func(span ConsistencySourceSpan) bool {
		source, ok := sources[span.SourceBlockID]
		return ok && uniqueExactSpan(source.Text, span.Text) && hasBoundedPhrase(source.Text, span.Text)
	}
	for _, entity := range ledger.Entities {
		if !grounded(entity.Source) {
			add("", "", SuiteUnclear, "entity_source", "An entity name does not identify a unique original source span.")
		}
	}
	fields := map[string]ConsistencyField{}
	for _, field := range ledger.Fields {
		fields[field.ID] = field
		for _, alias := range field.Aliases {
			if !grounded(alias) {
				add("", field.ID, SuiteUnclear, "field_source", "A field alias does not identify a unique original source span.")
			}
		}
	}
	if len(findings) != 0 {
		return findings
	}
	if v3 {
		fields = consistencySuffixFields(fields)
	}
	bySource := map[string]MissingOnlyConstraint{}
	for _, constraint := range ledger.MissingOnly {
		found := false
		for _, rule := range input.Policy.Rules {
			for _, id := range rule.SourceBlockIDs {
				found = found || rule.ID == constraint.RuleID && id == constraint.Source.SourceBlockID
			}
		}
		if !found || !grounded(constraint.Source) || !wholeClause(sources[constraint.Source.SourceBlockID].Text, constraint.Source.Text) {
			add("", "", SuiteUnclear, "missing_rule_source", "A missing-only rule lacks its complete, source-backed instruction.")
			continue
		}
		bySource[sourceKey(constraint.Source.SourceBlockID, constraint.Source.Text)] = constraint
	}
	for _, clause := range clauses {
		constraint, ok := bySource[sourceKey(clause.id, clause.text)]
		match := simpleMissingInstruction.FindStringSubmatch(normalizeConsistency(clause.text))
		if !ok || len(match) != 2 {
			add("", "", SuiteUnclear, "missing_rule_coverage", "An original missing-only instruction is omitted or uses unresolved grammar.")
			continue
		}
		ids, valid := parseFieldList(match[1], fields, true)
		if !valid || !sameStringSet(ids, constraint.FieldIDs) {
			add("", "", SuiteUnclear, "missing_rule_fields", "The field ledger does not cover every field in the original missing-only instruction.")
		}
		delete(bySource, sourceKey(clause.id, clause.text))
	}
	if len(bySource) != 0 {
		add("", "", SuiteUnclear, "extra_missing_rule", "The ledger includes an unsupported missing-only instruction.")
	}
	if len(findings) != 0 {
		return findings
	}
	ledgerCases := map[string]CaseFactLedger{}
	for _, c := range ledger.Cases {
		ledgerCases[c.CaseKey] = c
	}
	for _, c := range input.Cases {
		trace := ledgerCases[c.CaseKey]
		derived, asked := map[string]bool{}, map[string]bool{}
		for _, clause := range consistencyClauses(c.Expected) {
			normal := normalizeConsistency(clause)
			kind := "ask"
			match := positiveAsk.FindStringSubmatch(normal)
			if negative := negativeAsk.FindStringSubmatch(normal); len(negative) == 2 {
				kind, match = "do_not_ask", negative
			}
			if len(match) != 2 {
				if askWord.MatchString(normal) {
					add(c.CaseKey, "", SuiteUnclear, "expected_grammar", "An expected request uses unsupported grammar; it cannot be treated as an unconditional obligation.")
				}
				// Non-request assertions remain the semantic reviewer's job,
				// even when they mention one of the declared fields.
				continue
			}
			// "Do not ask for A or B" forbids each request. Positive "A or B"
			// remains an unresolved choice, and the frozen v2 parser is unchanged.
			ids, valid := parseFieldList(match[1], fields, v3 && kind == "do_not_ask")
			if !valid {
				add(c.CaseKey, "", SuiteUnclear, "expected_fields", "The exact expected request contains an unresolved field, alternative, condition, or additional clause.")
				continue
			}
			for _, id := range ids {
				derived[obligationKey(id, kind, clause)] = true
				if kind == "ask" {
					asked[id] = true
				}
			}
		}
		present := map[string]bool{}
		for _, fact := range trace.Facts {
			if !asked[fact.FieldID] {
				// An unrelated uncertain fact cannot invalidate an otherwise
				// resolved request for a different missing field.
				continue
			}
			value, ok := consistencyInputString(c.Input, fact.InputPointer)
			if !ok || fact.State == "unclear" {
				add(c.CaseKey, fact.FieldID, SuiteUnclear, "fact_unresolved", "An input field cannot be resolved from its original input location.")
				continue
			}
			if fact.State == "missing" {
				if fact.Literal != "" || fact.Evidence != "" && (!uniqueExactSpan(value, fact.Evidence) || !wholeClause(value, fact.Evidence)) {
					add(c.CaseKey, fact.FieldID, SuiteUnclear, "missing_fact_evidence", "A missing-field interpretation contains inconsistent evidence.")
				}
				for _, clause := range consistencyClauses(value) {
					if mentionsField(clause, fields[fact.FieldID]) && !uncertainFact.MatchString(clause) {
						add(c.CaseKey, fact.FieldID, SuiteUnclear, "missing_fact_mentioned", "A field marked missing is explicitly mentioned in a positive input clause.")
						break
					}
				}
				continue
			}
			if fact.Literal == "" || !uniqueExactSpan(value, fact.Evidence) || !wholeClause(value, fact.Evidence) || uncertainFact.MatchString(fact.Evidence) || !hasBoundedPhrase(fact.Evidence, fact.Literal) || fields[fact.FieldID].Kind == "number" && !hasExactNumber(fact.Evidence, fact.Literal) {
				add(c.CaseKey, fact.FieldID, SuiteUnclear, "present_fact_evidence", "A present fact lacks an exact affirmative, unconditional input clause and literal.")
				continue
			}
			present[fact.FieldID] = true
		}
		for _, field := range ledger.Fields {
			if asked[field.ID] && present[field.ID] {
				add(c.CaseKey, field.ID, SuiteContradicted, "asks_present_field", "The expectation asks for a field already supplied in the input, while the original instruction permits asking only for missing fields.")
			}
		}
		if v3 {
			// All safety-relevant requests came from the actual expectation;
			// no duplicated model obligation prose participates in this check.
			continue
		}
		for _, obligation := range trace.Obligations {
			key := obligationKey(obligation.FieldID, obligation.Kind, obligation.Evidence)
			if !uniqueExactSpan(c.Expected, obligation.Evidence) || !wholeConsistencyClause(c.Expected, obligation.Evidence) || !derived[key] {
				add(c.CaseKey, obligation.FieldID, SuiteUnclear, "obligation_evidence", "An obligation does not match the complete original expected clause and recognized field.")
				continue
			}
			delete(derived, key)
		}
		if len(derived) != 0 {
			add(c.CaseKey, "", SuiteUnclear, "obligation_coverage", "The ledger omits one or more obligations named in the exact expected text.")
		}
	}
	return findings
}

func obligationKey(field, kind, evidence string) string {
	return field + "\x00" + kind + "\x00" + evidence
}

func hasExactNumber(text, literal string) bool {
	if !decimalLiteral.MatchString(literal) {
		return false
	}
	for _, token := range numberToken.FindAllString(text, -1) {
		if token == literal {
			return true
		}
	}
	return false
}

func normalizeConsistency(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimRight(strings.TrimSpace(text), ".!?;")), " "))
}

func uniqueExactSpan(text, span string) bool {
	return strings.TrimSpace(span) != "" && strings.Count(text, span) == 1
}

// Delimiters stay in the evidence. A suffix cannot silently drop the beginning
// of a clause, including "not", "if", or a quotation mark.
func consistencyClauses(text string) []string {
	var clauses []string
	start := 0
	for i, r := range text {
		end := i + utf8.RuneLen(r)
		separator := r == ';' || r == '\n' || r == '!' || r == '?'
		if r == '.' && (end == len(text) || unicode.IsSpace(rune(text[end]))) {
			separator = true
		}
		if separator {
			if clause := strings.TrimSpace(text[start:end]); clause != "" {
				clauses = append(clauses, clause)
			}
			start = end
		}
	}
	if clause := strings.TrimSpace(text[start:]); clause != "" {
		clauses = append(clauses, clause)
	}
	return clauses
}

func wholeConsistencyClause(text, span string) bool {
	if span == strings.TrimSpace(text) {
		return true
	}
	for _, clause := range consistencyClauses(text) {
		if clause == span {
			return true
		}
	}
	return false
}

func consistencyClauseBody(text string) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(text), ".!?;"))
}

func wholeConsistencyClauseV3(text, span string) bool {
	// uniqueExactSpan is checked separately: callers may omit a delimiter, but
	// cannot replace punctuation or normalize internal words or whitespace.
	want := consistencyClauseBody(span)
	if want == "" {
		return false
	}
	if consistencyClauseBody(text) == want {
		return true
	}
	matches := 0
	for _, clause := range consistencyClauses(text) {
		if consistencyClauseBody(clause) == want {
			matches++
		}
	}
	return matches == 1
}

func consistencySuffixFields(fields map[string]ConsistencyField) map[string]ConsistencyField {
	out := make(map[string]ConsistencyField, len(fields))
	for id, field := range fields {
		copy := field
		copy.Aliases = append([]ConsistencySourceSpan(nil), field.Aliases...)
		seen := map[string]bool{}
		for _, alias := range field.Aliases {
			seen[normalizeConsistency(alias.Text)] = true
		}
		for _, alias := range field.Aliases {
			// At most seven suffixes of a grounded name. Keep suffixes for
			// different fields even when identical: parseFieldList must reject
			// that ambiguity rather than prefer an existing short alias.
			starts := []int{}
			space := false
			for i, r := range alias.Text {
				if space && !unicode.IsSpace(r) {
					starts = append(starts, i)
				}
				space = unicode.IsSpace(r)
			}
			if len(starts) > 7 {
				starts = starts[len(starts)-7:]
			}
			for _, start := range starts {
				suffix := strings.TrimSpace(alias.Text[start:])
				key := normalizeConsistency(suffix)
				if key != "" && !seen[key] {
					copy.Aliases = append(copy.Aliases, ConsistencySourceSpan{SourceBlockID: alias.SourceBlockID, Text: suffix})
					seen[key] = true
				}
			}
		}
		out[id] = copy
	}
	return out
}

func hasBoundedPhrase(text, phrase string) bool {
	text, phrase = strings.ToLower(text), strings.ToLower(phrase)
	for from := 0; from <= len(text); {
		index := strings.Index(text[from:], phrase)
		if index < 0 || phrase == "" {
			return false
		}
		index += from
		end := index + len(phrase)
		left, right := true, true
		if index > 0 {
			r, _ := utf8.DecodeLastRuneInString(text[:index])
			left = !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '-' && r != '+' && r != '.'
		}
		if end < len(text) {
			r, _ := utf8.DecodeRuneInString(text[end:])
			right = !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_'
		}
		if left && right {
			return true
		}
		from = index + 1
	}
	return false
}

func mentionsField(text string, field ConsistencyField) bool {
	for _, alias := range field.Aliases {
		if hasBoundedPhrase(text, alias.Text) {
			return true
		}
	}
	return false
}

func parseFieldList(text string, fields map[string]ConsistencyField, allowOr bool) ([]string, bool) {
	type alias struct{ text, id string }
	aliases := []alias{}
	for id, field := range fields {
		for _, name := range field.Aliases {
			aliases = append(aliases, alias{normalizeConsistency(name.Text), id})
		}
	}
	sort.Slice(aliases, func(i, j int) bool {
		if len(aliases[i].text) != len(aliases[j].text) {
			return len(aliases[i].text) > len(aliases[j].text)
		}
		return aliases[i].id < aliases[j].id
	})
	ids, seen := []string{}, map[string]bool{}
	remaining := normalizeConsistency(text)
	for remaining != "" {
		remaining = strings.TrimLeft(remaining, " ,")
		if remaining == "" {
			break
		}
		chosen := alias{}
		for _, candidate := range aliases {
			if strings.HasPrefix(remaining, candidate.text) && hasBoundedPhrase(remaining[:len(candidate.text)], candidate.text) && (len(remaining) == len(candidate.text) || remaining[len(candidate.text)] == ' ' || remaining[len(candidate.text)] == ',') {
				if chosen.text != "" && chosen.text == candidate.text && chosen.id != candidate.id {
					return nil, false
				}
				if chosen.text == "" {
					chosen = candidate
				}
			}
		}
		if chosen.text != "" {
			if !seen[chosen.id] {
				seen[chosen.id] = true
				ids = append(ids, chosen.id)
			}
			remaining = strings.TrimSpace(remaining[len(chosen.text):])
			continue
		}
		word, tail, _ := strings.Cut(remaining, " ")
		switch word {
		case "the", "both", "and", "only", "missing", "just":
		case "or":
			if !allowOr {
				return nil, false
			}
		default:
			return nil, false
		}
		remaining = tail
	}
	return ids, len(ids) > 0
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]bool{}
	for _, s := range a {
		m[s] = true
	}
	for _, s := range b {
		if !m[s] {
			return false
		}
	}
	return true
}

func consistencyInputString(input json.RawMessage, pointer string) (string, bool) {
	var value any
	if json.Unmarshal(input, &value) != nil || !strings.HasPrefix(pointer, "/") {
		return "", false
	}
	for _, part := range strings.Split(pointer[1:], "/") {
		for i := 0; i < len(part); i++ {
			if part[i] == '~' && (i+1 == len(part) || part[i+1] != '0' && part[i+1] != '1') {
				return "", false
			}
			if part[i] == '~' {
				i++
			}
		}
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch v := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = v[part]
			if !ok {
				return "", false
			}
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(v) || strconv.Itoa(i) != part {
				return "", false
			}
			value = v[i]
		default:
			return "", false
		}
	}
	text, ok := value.(string)
	return text, ok
}
