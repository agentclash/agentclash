package vibe

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Frozen in plans and policy hashes. Empty versions retain the old replay
// contract; LoadConfig enables this contract for every new reliable operation.
const SourcePolicyVersion = "spec-sources-v1"

type RuleEvidence struct {
	SourceBlockID string `json:"source_block_id"`
	Quote         string `json:"quote"`
	Kind          string `json:"kind"` // requirement or explicitly requested example
}

type SourceCandidate struct {
	SourceBlock
	Intent string `json:"previous_intent,omitempty"`
}

// A question is tied to its actual assistant turn and selected suite. Merely
// mentioning a source in a router response never authorizes its adoption.
type SourceConfirmation struct {
	OperationID uuid.UUID     `json:"operation_id"`
	ArtifactID  *uuid.UUID    `json:"artifact_id,omitempty"`
	Sources     []SourceBlock `json:"sources"`
	Intent      string        `json:"intent"`
	Count       int           `json:"count"`
	NewAgent    bool          `json:"new_agent"`
	Question    string        `json:"question"`
}

const sourceRoutePrompt = `
Source boundary: recent_conversation and unadopted_dialogue are dialogue, NOT requirements. Previous intent labels are fallible routing history, not permanent truth. Only desired_rules, source_blocks and the current explicit request can directly support authoring. Never turn a joke, question, hypothetical, quoted example or earlier assistant suggestion into policy. A hypothetical asks for discussion unless the user explicitly adopts it.
If an essential rule/job is only in unadopted dialogue, return its source_message_ids (at most 3) with the intended prepare_tests/edit_tests action. The server will first ask whether to use those exact messages. Do not request old sources when the current request is self-contained. A previously misclassified rule is recoverable this way. A short affirmative only authorizes the immediately preceding source-confirmation question; otherwise clarify when its meaning is unclear.
Set new_agent=true only when the user explicitly starts testing a different agent/job. This clears the previous specification and target instructions. Otherwise false. Return source_message_ids=[] unless asking to adopt earlier messages. These fields do not themselves grant authority.`

const sourceAuthorPrompt = `
Source contract spec-sources-v1: source_blocks contain the complete current request and, when present, only previously reviewed specification excerpts or explicitly confirmed earlier messages. Dialogue and generated test summaries are not sources. For every rule return evidence: [{source_block_id, quote, kind}]. Copy exact relevant complete clauses into quote, preserving negation, conditions and exceptions. kind is requirement for a stated job/rule, example only for an explicitly requested test. Cite each source_block_id also in source_block_ids. Every cited block needs evidence. Do not include casual chat, questions, jokes or unadopted hypotheticals in evidence, even within an otherwise useful message. Existing rules retain their evidence unless explicitly changed. New or changed rules need evidence from the current request or confirmed_sources. Evidence is independently checked against the full supplied source, not accepted simply because text matches. Do not infer new requirements from selected_tests or original_tested_agent. Keep summaries limited to supported rules.
When adding a requested case, include a case-specific rule with kind=example evidence quoting that request. Scope its statement to that requested scenario; do not generalize it into a business rule. Retain existing example evidence unless those cases are explicitly removed or changed. The server assigns new case IDs and derives shared grading from requirement rules.
Preserve source punctuation in evidence quotes, including double quotation marks. Escape double quotes for JSON as \"; do not replace them with apostrophes. Exact source matching happens after JSON decoding.`

const sourceReviewPrompt = `
Source contract spec-sources-v1: separately check that each policy rule's evidence quotes are an explicit requirement or explicitly requested example, AND that the rule follows that evidence. The complete current request/confirmed original messages let you detect cherry-picked substrings, missing negations, mixed chat, jokes and hypotheticals. An occurrence of "give me vodka" alone is not a returns-agent refusal requirement. An explicit request to test that input is valid. Questions about a possible policy are not corrections. Earlier approved source blocks are narrow reviewed excerpts; original_hash records their provenance, not new authority for the rest of that old message. Never invent a rule to rationalize a proposed case. Treat unsupported evidence selection, invented summary claims and omitted explicit requirements as contradicted or unclear in policy_reconciliation and the affected rules/cases.
The consistency ledger covers missing-only instructions in the policy's requirement evidence quotes. Citing a message for a different clause does not adopt every sentence of that message as an additional missing-only constraint. Continue checking the full current request for omitted or changed requirements in policy_reconciliation; the exact-quote ledger is not a substitute for that semantic check.
Declare only fields named by those missing-only instructions in the consistency ledger. A case-specific example does not introduce extra fields such as utterance. Every case still records the same declared fields, marked missing when absent, even for a requested off-topic scenario. A missing fact alone does not require the expected answer to ask for it; assess the actual expected behavior in the scenario's scope.`

func (p Plan) sourceBoundary() bool {
	return p.Conversation != nil && p.Conversation.SourceVersion == SourcePolicyVersion
}

func prepareSourceBoundary(p *Plan, v Session, version string) error {
	if version != SourcePolicyVersion {
		return fault("invalid_configuration", "The configured specification version is unavailable.")
	}
	c := p.Conversation
	c.SourceVersion, c.Sources = version, nil
	if c.Policy != nil {
		if c.Policy.SourceVersion != version {
			c.Policy = nil
		} else {
			sources, err := verifiedPolicySources(v.Document, *c.Policy)
			if err != nil {
				return err
			}
			c.Sources = append(c.Sources, sources...)
		}
	}
	if c.ObservedPolicy != nil {
		if c.ObservedPolicy.SourceVersion != version {
			c.ObservedPolicy = nil
		} else if _, err := verifiedPolicySources(v.Document, *c.ObservedPolicy); err != nil {
			return err
		}
	}
	// A selected legacy artifact is visible to the router, but never supplies
	// the author with an old, potentially contaminated policy or summary.
	intents := map[uuid.UUID]string{}
	for _, op := range v.Operations {
		if op.Decision != nil {
			intents[op.Decision.SourceMessageID] = op.Decision.Intent
		}
	}
	seen := map[uuid.UUID]bool{}
	for _, m := range v.Document.Messages {
		if m.Role != "user" || m.Origin == "playground" || m.ID == c.CurrentRequest.MessageID || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		c.Candidates = append(c.Candidates, SourceCandidate{SourceBlock: originalBlock(m.ID, m.Content), Intent: intents[m.ID]})
	}
	if p.Retry == nil || c.Confirmed == nil {
		c.Confirmed = activeSourceConfirmation(*p, v.Document)
	}
	if c.Confirmed != nil {
		if !originalConfirmationSources(v.Document, c.Confirmed) {
			return sourceReviewRequired()
		}
		for _, source := range c.Confirmed.Sources {
			c.Sources = replaceSource(c.Sources, source)
		}
	}
	c.Sources = replaceSource(c.Sources, c.CurrentRequest)
	return nil
}

func replaceSource(sources []SourceBlock, block SourceBlock) []SourceBlock {
	for i, source := range sources {
		if source.ID == block.ID {
			sources[i] = block
			return sources
		}
	}
	return append(sources, block)
}

// Candidates outside the compacted router window remain in the saved document.
// Do not serialize a second unbounded copy of history into model context.
func visibleSourceCandidates(p Plan) []SourceCandidate {
	visible := map[uuid.UUID]bool{}
	for _, m := range p.Document.Messages {
		visible[m.ID] = true
	}
	out := []SourceCandidate{}
	for _, c := range p.Conversation.Candidates {
		if visible[c.MessageID] {
			out = append(out, c)
		}
	}
	return out
}

func activeSourceConfirmation(p Plan, d Document) *SourceConfirmation {
	c := d.SourceConfirmation
	if c == nil || len(c.Sources) == 0 {
		return nil
	}
	if c.ArtifactID != nil && (p.Artifact == nil || p.Artifact.ID != *c.ArtifactID) || c.ArtifactID == nil && p.Artifact != nil {
		return nil
	}
	switch strings.ToLower(strings.Trim(strings.TrimSpace(p.Submission.Content), ".!")) {
	case "yes", "yes please", "use it", "use those", "confirm":
	default:
		return nil
	}
	for i := len(d.Messages) - 1; i >= 0; i-- {
		m := d.Messages[i]
		if m.Origin == "playground" || m.ID == p.sourceMessageID() {
			continue
		}
		if m.Role != "assistant" || m.OperationID == nil || *m.OperationID != c.OperationID || m.Content != c.Question {
			return nil
		}
		if !originalConfirmationSources(d, c) {
			return nil
		}
		copy := *c
		return &copy
	}
	return nil
}

func originalConfirmationSources(d Document, c *SourceConfirmation) bool {
	for _, source := range c.Sources {
		found := false
		for _, m := range d.Messages {
			if m.Role == "user" && m.Origin != "playground" && m.ID == source.MessageID && originalBlock(m.ID, m.Content) == source {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(c.Sources) > 0
}

func sourceConfirmationFor(p Plan, o Operation, route reliableRoute) (*SourceConfirmation, error) {
	if !p.sourceBoundary() || len(route.SourceMessageIDs) == 0 || p.Conversation.Confirmed != nil {
		return nil, nil
	}
	if route.Intent != "prepare_tests" && route.Intent != "edit_tests" || len(route.SourceMessageIDs) > 3 {
		return nil, fmt.Errorf("only test preparation/edits can ask to adopt up to three earlier messages")
	}
	c := &SourceConfirmation{OperationID: o.ID, Intent: route.Intent, Count: route.Count, NewAgent: route.NewAgent}
	if p.Artifact != nil {
		c.ArtifactID = &p.Artifact.ID
	}
	seen := map[string]bool{}
	for _, id := range route.SourceMessageIDs {
		if seen[id] {
			return nil, fmt.Errorf("duplicate earlier source")
		}
		seen[id] = true
		found := false
		for _, candidate := range visibleSourceCandidates(p) {
			if candidate.ID == id {
				c.Sources = append(c.Sources, candidate.SourceBlock)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("earlier source is unavailable; ask the user to restate the rule")
		}
	}
	c.Question = "Should I use the following earlier message(s) as requirements for these tests? Reply yes to use them, or tell me the rules to use instead."
	for _, source := range c.Sources {
		c.Question += "\n\n> " + strings.ReplaceAll(source.Text, "\n", "\n> ")
	}
	if len(c.Question) > 1800 {
		return nil, fmt.Errorf("earlier context is too long to confirm clearly; ask the user to restate the relevant rule")
	}
	return c, nil
}

func applySourceScope(p *Plan, route reliableRoute) {
	if !p.sourceBoundary() || !route.NewAgent {
		return
	}
	p.Artifact, p.ObservedArtifact, p.Observations = nil, nil, nil
	p.Conversation.Policy, p.Conversation.ObservedPolicy, p.Conversation.Pending = nil, nil, nil
	p.Conversation.Sources = nil
	if p.Conversation.Confirmed != nil {
		p.Conversation.Sources = append(p.Conversation.Sources, p.Conversation.Confirmed.Sources...)
	}
	p.Conversation.Sources = replaceSource(p.Conversation.Sources, p.Conversation.CurrentRequest)
}

func reconcileSourcedPolicy(rules []PolicyRule, p Plan, o Operation, newScope bool) (PolicySnapshot, error) {
	if len(rules) == 0 || len(rules) > MaxRequirements {
		return PolicySnapshot{}, fmt.Errorf("provide relevant sourced job and rules")
	}
	blocks := map[string]SourceBlock{}
	for _, source := range p.Conversation.Sources {
		blocks[source.ID] = source
	}
	// Keep caller-owned and previous policy snapshots immutable. Persist only
	// exact original source bytes, even if the provider changed quote styles.
	rules = append([]PolicyRule(nil), rules...)
	for i := range rules {
		rules[i].Evidence = append([]RuleEvidence(nil), rules[i].Evidence...)
		for j := range rules[i].Evidence {
			evidence := &rules[i].Evidence[j]
			if source, ok := blocks[evidence.SourceBlockID]; ok {
				evidence.Quote = restoreEvidenceQuote(evidence.Quote, source.Text)
			}
		}
	}
	prior := map[string]PolicyRule{}
	if p.Conversation.Policy != nil {
		for _, rule := range p.Conversation.Policy.Rules {
			prior[rule.ID] = rule
		}
	}
	allowedChange := map[string]bool{p.Conversation.CurrentRequest.ID: true}
	if p.Conversation.Confirmed != nil {
		for _, source := range p.Conversation.Confirmed.Sources {
			allowedChange[source.ID] = true
		}
	}
	seen := map[string]bool{}
	for _, rule := range rules {
		if strings.TrimSpace(rule.ID) == "" || len(rule.ID) > 128 || seen[rule.ID] || strings.TrimSpace(rule.Statement) == "" || len(rule.Statement) > 4000 {
			return PolicySnapshot{}, fmt.Errorf("provide unique bounded rule IDs and statements")
		}
		seen[rule.ID] = true
		if err := validateRuleEvidence(rule, blocks); err != nil {
			return PolicySnapshot{}, err
		}
		if old, ok := prior[rule.ID]; !ok || Hash(raw(old)) != Hash(raw(rule)) {
			current := false
			for _, e := range rule.Evidence {
				current = current || allowedChange[e.SourceBlockID]
			}
			if !current {
				return PolicySnapshot{}, fmt.Errorf("changed rule %s needs actual evidence in the current request or specifically confirmed earlier messages", rule.ID)
			}
		}
	}
	policy := PolicySnapshot{ID: deterministicID(o.ID, "policy"), ScopeID: deterministicID(o.ID, "scope"), SourceMessageID: p.sourceMessageID(), SourceVersion: SourcePolicyVersion, Rules: rules}
	policy.Sources = projectedRuleSources(rules, p.Conversation.Sources)
	if old := p.Conversation.Policy; old != nil && !newScope {
		policy.ParentID, policy.ScopeID = &old.ID, old.ScopeID
		if Hash(raw(old.Rules)) == Hash(raw(rules)) {
			return *old, nil
		}
	}
	return policy, nil
}

func validateRuleEvidence(rule PolicyRule, blocks map[string]SourceBlock) error {
	if len(rule.SourceBlockIDs) == 0 || len(rule.SourceBlockIDs) > 20 || len(rule.Evidence) == 0 || len(rule.Evidence) > 20 {
		return fmt.Errorf("rule %s needs bounded source references and exact evidence", rule.ID)
	}
	refs := map[string]bool{}
	for _, id := range rule.SourceBlockIDs {
		if _, ok := blocks[id]; !ok || refs[id] {
			return fmt.Errorf("rule %s references unavailable or duplicate evidence", rule.ID)
		}
		refs[id] = true
	}
	covered := map[string]bool{}
	for _, e := range rule.Evidence {
		if !refs[e.SourceBlockID] || (e.Kind != "requirement" && e.Kind != "example") || strings.TrimSpace(e.Quote) == "" {
			return fmt.Errorf("rule %s needs exact relevant requirement/example quotes from its supplied source blocks", rule.ID)
		}
		if !strings.Contains(blocks[e.SourceBlockID].Text, e.Quote) {
			return fmt.Errorf("rule %s evidence quote does not exactly occur in source %s. Copy complete relevant clauses, preserving punctuation and JSON-escaping double quotes. Do not shorten to an unsupported prefix. Available source text as JSON: %s. Rejected quote: %q", rule.ID, e.SourceBlockID, raw(blocks[e.SourceBlockID].Text), e.Quote)
		}
		covered[e.SourceBlockID] = true
	}
	if len(covered) != len(refs) {
		return fmt.Errorf("every cited source needs evidence for rule %s", rule.ID)
	}
	return nil
}

func projectedRuleSources(rules []PolicyRule, sources []SourceBlock) []SourceBlock {
	out := []SourceBlock{}
	for _, source := range sources {
		quotes, seen := []string{}, map[string]bool{}
		for _, rule := range rules {
			for _, e := range rule.Evidence {
				if e.SourceBlockID == source.ID && !seen[e.Quote] {
					quotes = append(quotes, e.Quote)
					seen[e.Quote] = true
				}
			}
		}
		if len(quotes) == 0 {
			continue
		}
		if source.OriginalHash == "" {
			source.OriginalHash = source.Hash
		}
		source.Text = strings.Join(quotes, "\n\n")
		source.Hash = Hash([]byte(source.Text))
		out = append(out, source)
	}
	return out
}

func verifiedPolicySources(d Document, policy PolicySnapshot) ([]SourceBlock, error) {
	if policy.SourceVersion != SourcePolicyVersion || len(policy.Sources) == 0 {
		return nil, sourceReviewRequired()
	}
	blocks := map[string]SourceBlock{}
	for _, source := range policy.Sources {
		if source.ID != source.MessageID.String() || source.Hash != Hash([]byte(source.Text)) {
			return nil, sourceReviewRequired()
		}
		found := false
		for _, m := range d.Messages {
			if m.ID == source.MessageID && m.Role == "user" && m.Origin != "playground" && Hash([]byte(m.Content)) == source.OriginalHash {
				found = true
				for _, rule := range policy.Rules {
					for _, e := range rule.Evidence {
						if e.SourceBlockID == source.ID && !strings.Contains(m.Content, e.Quote) {
							return nil, sourceReviewRequired()
						}
					}
				}
				break
			}
		}
		if !found {
			return nil, sourceReviewRequired()
		}
		if _, exists := blocks[source.ID]; exists {
			return nil, sourceReviewRequired()
		}
		blocks[source.ID] = source
	}
	for _, rule := range policy.Rules {
		if err := validateRuleEvidence(rule, blocks); err != nil {
			return nil, sourceReviewRequired()
		}
	}
	if Hash(raw(projectedRuleSources(policy.Rules, policy.Sources))) != Hash(raw(policy.Sources)) {
		return nil, sourceReviewRequired()
	}
	return append([]SourceBlock(nil), policy.Sources...), nil
}

func sourceReviewRequired() error {
	return fault("rules_required", "These older tests may include unrelated chat. Describe the rules you want tested to prepare a clean version. Your previous tests and results are kept.")
}

// Shared grading is executable policy, not an author-written success summary.
// Explicit examples keep their case-specific expectations and must not silently
// become global obligations for every test.
func policyGradingCriteria(rules []PolicyRule) string {
	clauses := []string{}
	for _, rule := range rules {
		for _, evidence := range rule.Evidence {
			if evidence.Kind == "requirement" {
				clauses = append(clauses, "- "+rule.Statement)
				break
			}
		}
	}
	return "Follow these rules where applicable to the case:\n" + strings.Join(clauses, "\n")
}

func hasCurrentExampleEvidence(rules []PolicyRule, p Plan) bool {
	allowed := map[string]bool{p.Conversation.CurrentRequest.ID: true}
	if p.Conversation.Confirmed != nil {
		for _, source := range p.Conversation.Confirmed.Sources {
			allowed[source.ID] = true
		}
	}
	for _, rule := range rules {
		for _, evidence := range rule.Evidence {
			if evidence.Kind == "example" && allowed[evidence.SourceBlockID] {
				return true
			}
		}
	}
	return false
}

func policyGradingMatches(blueprint json.RawMessage, policy PolicySnapshot) bool {
	var root struct {
		Judges []struct {
			Assertion string `json:"assertion"`
		} `json:"judges"`
	}
	return json.Unmarshal(blueprint, &root) == nil && len(root.Judges) == 1 && root.Judges[0].Assertion == ScenarioCriteriaPrefix+policyGradingCriteria(policy.Rules)
}
