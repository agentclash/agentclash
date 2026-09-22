package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Current requests retain complete original blocks. Historical specification
// blocks contain only reviewed evidence, with the original hash for provenance.
type SourceBlock struct {
	OriginalHash string    `json:"original_hash,omitempty"`
	ID           string    `json:"id"`
	MessageID    uuid.UUID `json:"message_id"`
	Text         string    `json:"text"`
	Hash         string    `json:"hash"`
}
type PolicyRule struct {
	Evidence       []RuleEvidence `json:"evidence,omitempty"`
	ID             string         `json:"id"`
	Statement      string         `json:"statement"`
	SourceBlockIDs []string       `json:"source_block_ids"`
}
type PolicySnapshot struct {
	QuestionAnswers []QuestionAnswer `json:"question_answers,omitempty"`
	SourceVersion   string           `json:"source_version,omitempty"`
	Sources         []SourceBlock    `json:"sources,omitempty"`
	ID              uuid.UUID        `json:"id"`
	ParentID        *uuid.UUID       `json:"parent_id,omitempty"`
	ScopeID         uuid.UUID        `json:"scope_id"`
	SourceMessageID uuid.UUID        `json:"source_message_id"`
	Rules           []PolicyRule     `json:"rules"`
}
type SourceCoverage struct {
	MessageID uuid.UUID `json:"message_id"`
	State     string    `json:"state"`
}
type PendingPolicyChange struct {
	OperationID     uuid.UUID  `json:"operation_id"`
	SourceMessageID uuid.UUID  `json:"source_message_id"`
	ArtifactID      *uuid.UUID `json:"artifact_id,omitempty"`
	Status          string     `json:"status"`
	Message         string     `json:"message"`
}
type OperationFact struct {
	ID              uuid.UUID          `json:"id"`
	SourceMessageID uuid.UUID          `json:"source_message_id"`
	State           Execution          `json:"state"`
	Error           *Fault             `json:"error,omitempty"`
	Completion      *CompletionReceipt `json:"completion,omitempty"`
}
type ManualSuiteEdit struct {
	Blueprint json.RawMessage `json:"blueprint"`
}
type ConversationContext struct {
	Example          *GuidanceExample      `json:"example,omitempty"`
	State            *ConversationState    `json:"state,omitempty"`
	StateBaseHash    string                `json:"state_base_hash,omitempty"`
	MemorySources    []SourceBlock         `json:"memory_sources,omitempty"`
	NextState        *ConversationState    `json:"-"`
	SourceVersion    string                `json:"source_version,omitempty"`
	Candidates       []SourceCandidate     `json:"source_candidates,omitempty"`
	Confirmed        *SourceConfirmation   `json:"confirmed_sources,omitempty"`
	Policy           *PolicySnapshot       `json:"policy,omitempty"`
	ObservedPolicy   *PolicySnapshot       `json:"observed_policy,omitempty"`
	Sources          []SourceBlock         `json:"sources"`
	CurrentRequest   SourceBlock           `json:"current_request"`
	RecentOutcomes   []OperationFact       `json:"recent_outcomes"`
	Pending          []PendingPolicyChange `json:"pending_changes,omitempty"`
	Manual           *ManualSuiteEdit      `json:"manual,omitempty"`
	RequiredCount    int                   `json:"required_count,omitempty"`
	ContractVersion  string                `json:"contract_version"`
	ValidatorVersion string                `json:"validator_version,omitempty"`
	Profile          *ModelProfile         `json:"profile,omitempty"`
}

func originalBlock(id uuid.UUID, content string) SourceBlock {
	return SourceBlock{ID: id.String(), MessageID: id, Text: content, Hash: Hash([]byte(content))}
}
func policyFor(d Document, a *Artifact) *PolicySnapshot {
	if a == nil || a.PolicyID == nil {
		return nil
	}
	for _, policy := range d.Policies {
		if policy.ID == *a.PolicyID {
			copy := policy
			return &copy
		}
	}
	return nil
}
func deterministicID(operation uuid.UUID, purpose string) uuid.UUID {
	return uuid.NewSHA1(operation, []byte(purpose))
}

func prepareReliableContext(p *Plan, v Session, version ...string) error {
	p.AuthoringVersion = 11
	p.Calls = 5
	l := p.limits()
	// This is an immutable admission allowance, not a new cumulative quota.
	l.OperationSeconds = max(l.OperationSeconds, l.QueueSeconds+5*l.ProviderSeconds+30)
	p.ExecutionLimits = &l
	p.Document.Policies = append([]PolicySnapshot(nil), v.Document.Policies...)
	p.Document.SourceCoverage = append([]SourceCoverage(nil), v.Document.SourceCoverage...)
	p.Document.PendingPolicyChanges = append([]PendingPolicyChange(nil), v.Document.PendingPolicyChanges...)
	c := &ConversationContext{ContractVersion: "vibe-v11", Policy: policyFor(v.Document, p.Artifact), CurrentRequest: originalBlock(p.sourceMessageID(), p.Submission.Content), Pending: v.Document.PendingPolicyChanges}
	c.ObservedPolicy = policyFor(v.Document, p.ObservedArtifact)
	if p.ObservedArtifact != nil && c.ObservedPolicy == nil {
		if old, _, err := legacySuitePolicy(v.Document, *p.ObservedArtifact); err == nil {
			c.ObservedPolicy = &old
		}
	}
	seen := map[uuid.UUID]bool{}
	for _, m := range v.Document.Messages {
		if m.Role == "user" && m.Origin != "playground" && !seen[m.ID] {
			c.Sources = append(c.Sources, originalBlock(m.ID, m.Content))
			seen[m.ID] = true
		}
	}
	if !seen[c.CurrentRequest.MessageID] {
		c.Sources = append(c.Sources, c.CurrentRequest)
	}
	for _, op := range v.Operations {
		if op.Kind != "message" && op.Kind != "build" {
			continue
		}
		if op.Error == nil && op.Completion == nil {
			continue
		}
		id := operationSource(v, op, map[uuid.UUID]bool{})
		if op.Completion != nil {
			id = op.Completion.SourceMessageID
		}
		c.RecentOutcomes = append(c.RecentOutcomes, OperationFact{ID: op.ID, SourceMessageID: id, State: op.State, Error: op.Error, Completion: op.Completion})
	}
	if len(c.RecentOutcomes) > 8 {
		c.RecentOutcomes = c.RecentOutcomes[len(c.RecentOutcomes)-8:]
	}
	if p.Retry != nil && p.Conversation != nil {
		c.Confirmed = p.Conversation.Confirmed
	}
	p.Conversation = c
	sourceVersion := SourcePolicyVersion
	if len(version) > 0 {
		sourceVersion = version[0]
	}
	if sourceVersion != "" {
		return prepareSourceBoundary(p, v, sourceVersion)
	}
	return nil
}

func operationSource(v Session, op Operation, seen map[uuid.UUID]bool) uuid.UUID {
	if seen[op.ID] {
		return uuid.Nil
	}
	seen[op.ID] = true
	for _, m := range v.Document.Messages {
		if m.OperationID != nil && *m.OperationID == op.ID && m.Role == "user" {
			return m.ID
		}
	}
	if op.RetryOfOperationID != nil {
		for _, parent := range v.Operations {
			if parent.ID == *op.RetryOfOperationID {
				return operationSource(v, parent, seen)
			}
		}
	}
	return uuid.Nil
}

func reliableMessages(p Plan, instruction string, extra any) []provider.Message {
	desired := p.Conversation.Policy
	sources := p.Conversation.Sources
	// A feature rollback can admit a legacy request in a session that already
	// contains illustrations. Keep those display-only cards out of its context.
	history := append([]Message(nil), p.Document.Messages...)
	for i := range history {
		history[i].Cards = nil
	}
	pending := p.Conversation.Pending
	selected := p.Artifact
	if action, ok := extra.(map[string]any); ok && action["action"] == "suggest_fix" {
		// Improvements target the viewed run's contract, even if the user has
		// since selected a newer suite with different business rules.
		desired, selected = p.Conversation.ObservedPolicy, p.ObservedArtifact
		history, pending = nil, nil
		if desired != nil {
			refs := map[string]bool{p.Conversation.CurrentRequest.ID: true, desired.SourceMessageID.String(): true}
			for _, rule := range desired.Rules {
				for _, id := range rule.SourceBlockIDs {
					refs[id] = true
				}
			}
			if p.sourceBoundary() {
				sources = append(append([]SourceBlock(nil), desired.Sources...), p.Conversation.CurrentRequest)
			} else {
				sources = nil
				for _, source := range p.Conversation.Sources {
					if refs[source.ID] {
						sources = append(sources, source)
					}
				}
			}
		}
	}
	var candidates []SourceCandidate
	if p.sourceBoundary() {
		if instruction == reliableRoutePrompt {
			candidates = visibleSourceCandidates(p)
			instruction += sourceRoutePrompt
		} else {
			history, pending = nil, nil
			if desired == nil {
				selected = nil
			}
		}
	}
	contextFields := map[string]any{
		"current_request": p.Conversation.CurrentRequest,
		"desired_rules":   desired, "source_blocks": sources, "viewed_run_rules": p.Conversation.ObservedPolicy,
		"pending_changes": pending, "recorded_outcomes": p.Conversation.RecentOutcomes,
		"selected_tests": selected, "original_tested_agent": p.ObservedArtifact,
		"observed_results": p.Observations, "recent_conversation": history, "server_context": extra,
	}
	if p.sourceBoundary() {
		if instruction != reliableRoutePrompt+sourceRoutePrompt {
			contextFields["recorded_outcomes"] = nil
			if action, ok := extra.(map[string]any); !ok || action["action"] != "suggest_fix" {
				contextFields["original_tested_agent"], contextFields["observed_results"], contextFields["viewed_run_rules"] = nil, nil, nil
			}
		}
		contextFields["unadopted_dialogue"] = candidates
		contextFields["confirmed_sources"] = p.Conversation.Confirmed
	}
	return []provider.Message{
		{Role: "system", Content: instruction},
		{Role: "user", Content: string(raw(contextFields))},
		{Role: "user", Content: p.Submission.Content},
	}
}
func fitReliableContext(p *Plan, profile ModelProfile) error {
	// Accepted rules survive compaction. Dialogue remains in the document and
	// is only a bounded routing aid, never implicit specification authority.
	for {
		messages := taskMessages(*p, taskRoute, "", nil)
		_, err := CountContext(provider.Request{Messages: messages, ResponseFormat: reliableRouteFormat(profile, *p), MaxOutputTokens: p.limits().OutputTokens}, profile, p.limits())
		bytes := 0
		for _, m := range p.Document.Messages {
			bytes += len(m.Content)
		}
		if err == nil && bytes <= 12000 {
			return nil
		}
		if len(p.Document.Messages) == 0 {
			return err
		}
		id := p.Document.Messages[0].ID
		p.ContextThrough = &id
		p.Document.Messages = p.Document.Messages[1:]
	}
}

func reconcilePolicy(rules []PolicyRule, p Plan, o Operation, newScope bool) (PolicySnapshot, error) {
	if p.sourceBoundary() {
		return reconcileSourcedPolicy(rules, p, o, newScope)
	}
	if len(rules) == 0 || len(rules) > MaxRequirements {
		return PolicySnapshot{}, fmt.Errorf("provide the relevant sourced job and rules")
	}
	sources := map[string]SourceBlock{}
	for _, s := range p.Conversation.Sources {
		sources[s.ID] = s
	}
	seen := map[string]bool{}
	prior := map[string]PolicyRule{}
	if p.Conversation.Policy != nil && !newScope {
		for _, rule := range p.Conversation.Policy.Rules {
			prior[rule.ID] = rule
		}
	}
	// Bind the operation's own source independently of model-generated refs.
	// A corrected clause cannot cite only the older, contradictory wording.
	rules = append([]PolicyRule(nil), rules...)
	for index, rule := range rules {
		if strings.TrimSpace(rule.ID) == "" || len(rule.ID) > 128 || seen[rule.ID] || strings.TrimSpace(rule.Statement) == "" || len(rule.Statement) > 4000 || len(rule.SourceBlockIDs) == 0 || len(rule.SourceBlockIDs) > 20 {
			return PolicySnapshot{}, fmt.Errorf("rules need unique IDs, bounded statements and original source references")
		}
		seen[rule.ID] = true
		seenSources := map[string]bool{}
		for _, id := range rule.SourceBlockIDs {
			if _, ok := sources[id]; !ok || seenSources[id] {
				return PolicySnapshot{}, fmt.Errorf("rule %s references an unavailable source block", rule.ID)
			}
			seenSources[id] = true
		}
		if old, ok := prior[rule.ID]; !ok || old.Statement != rule.Statement {
			current := p.Conversation.CurrentRequest.ID
			if current == "" {
				current = p.sourceMessageID().String()
			}
			if _, ok := sources[current]; !ok {
				return PolicySnapshot{}, fmt.Errorf("changed rule has no original request source")
			}
			found := false
			for _, id := range rule.SourceBlockIDs {
				if id == current {
					found = true
				}
			}
			if !found {
				rules[index].SourceBlockIDs = append(append([]string(nil), rule.SourceBlockIDs...), current)
				if len(rules[index].SourceBlockIDs) > 20 {
					return PolicySnapshot{}, fmt.Errorf("rule source references exceed the bounded allowance")
				}
			}
		}
	}
	policy := PolicySnapshot{ID: deterministicID(o.ID, "policy"), ScopeID: deterministicID(o.ID, "scope"), SourceMessageID: p.sourceMessageID(), Rules: rules}
	if old := p.Conversation.Policy; old != nil && !newScope {
		policy.ParentID = &old.ID
		policy.ScopeID = old.ScopeID
		if Hash(raw(old.Rules)) == Hash(raw(rules)) {
			return *old, nil
		}
	}
	return policy, nil
}

func applyAuthoringPolicyCompletion(d *Document, c AuthoringCompletion, p Plan, artifact *Artifact) error {
	if p.sourceBoundary() {
		d.SourceConfirmation = c.SourceConfirmation
	}
	if c.Policy != nil {
		if artifact == nil || artifact.PolicyID == nil || *artifact.PolicyID != c.Policy.ID || !SuiteValidationMatches(artifact.Validation, artifact.Blueprint, *c.Policy) {
			return fmt.Errorf("policy completion lacks matching suite validation")
		}
		found := false
		for _, old := range d.Policies {
			if old.ID == c.Policy.ID {
				found = true
				if Hash(raw(old)) != Hash(raw(c.Policy)) {
					return fmt.Errorf("policy revision changed")
				}
			}
		}
		if !found {
			if len(d.Policies) >= MaxRevisions {
				return fault("document_limit", "This conversation has too many saved rule versions.")
			}
			d.Policies = append(d.Policies, *c.Policy)
		}
	}
	for _, coverage := range c.Coverage {
		found := false
		for i, old := range d.SourceCoverage {
			if old.MessageID == coverage.MessageID {
				d.SourceCoverage[i] = coverage
				found = true
				break
			}
		}
		if !found {
			d.SourceCoverage = append(d.SourceCoverage, coverage)
		}
	}
	if c.Pending != nil {
		found := false
		for i, old := range d.PendingPolicyChanges {
			if old.OperationID == c.Pending.OperationID {
				d.PendingPolicyChanges[i] = *c.Pending
				found = true
				break
			}
		}
		if !found {
			d.PendingPolicyChanges = append(d.PendingPolicyChanges, *c.Pending)
		}
	}
	if artifact != nil && c.Policy != nil && SuiteValidationMatches(artifact.Validation, artifact.Blueprint, *c.Policy) {
		represented := map[string]bool{}
		for _, rule := range c.Policy.Rules {
			for _, source := range rule.SourceBlockIDs {
				represented[source] = true
			}
		}
		for i, old := range d.PendingPolicyChanges {
			if old.SourceMessageID == p.sourceMessageID() || represented[old.SourceMessageID.String()] && samePolicyScope(*d, old.ArtifactID, c.Policy.ScopeID) {
				d.PendingPolicyChanges[i].Status = "applied"
			}
		}
	}
	return nil
}

func samePolicyScope(d Document, artifactID *uuid.UUID, scope uuid.UUID) bool {
	if artifactID == nil {
		return false
	}
	for _, a := range d.Artifacts {
		if a.ID == *artifactID {
			p := policyFor(d, &a)
			return p != nil && p.ScopeID == scope
		}
	}
	return false
}

func (s *Store) recordPendingPolicy(ctx context.Context, o Operation, p Plan) error {
	if p.Artifact == nil {
		return nil
	}
	return s.transaction(ctx, func(tx pgx.Tx) error {
		var state Execution
		if err := tx.QueryRow(ctx, "SELECT state FROM vibe_operations WHERE id=$1 FOR UPDATE", o.ID).Scan(&state); err != nil {
			return err
		}
		if state != Running {
			return fault("operation_stopped", "The operation was stopped.")
		}
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", o.SessionID))
		if err != nil {
			return err
		}
		for _, old := range v.Document.PendingPolicyChanges {
			if old.OperationID == o.ID {
				return nil
			}
		}
		if len(v.Document.PendingPolicyChanges) >= MaxConversationOperations {
			return fault("document_limit", "This conversation has too many pending changes.")
		}
		v.Document.PendingPolicyChanges = append(v.Document.PendingPolicyChanges, PendingPolicyChange{OperationID: o.ID, SourceMessageID: p.sourceMessageID(), ArtifactID: &p.Artifact.ID, Status: "pending", Message: "This test update has not been applied. Your previous tests are unchanged."})
		if err = s.updateDocument(ctx, tx, v); err != nil {
			return err
		}
		return event(ctx, tx, v.ID, &o.ID, "tests.preparing")
	})
}

// Stable timestamps belong to the operation, not the wall clock at replay.
func operationTime(o Operation) time.Time {
	if !o.CreatedAt.IsZero() {
		return o.CreatedAt
	}
	return time.Unix(0, 0).UTC()
}
