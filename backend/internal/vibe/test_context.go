package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func (s *Service) prepareTestConversation(ctx context.Context, actor string, v Session, sub Submission, p Plan) (Operation, error) {
	if strings.TrimSpace(sub.Content) == "" {
		return Operation{}, fault("invalid_message", "Write a message first.")
	}
	if sub.QuickCheck || sub.EvidenceSetID != nil || sub.Instructions != "" {
		return Operation{}, fault("invalid_message", "Add your agent's instructions using Add your agent; use this conversation to discuss the tests.")
	}
	p.AuthoringVersion = 10
	p.Document = Document{TestJourney: true, Requirements: append([]Requirement(nil), v.Document.Requirements...)}
	for _, m := range v.Document.Messages {
		if m.Origin != "playground" {
			p.Document.Messages = append(p.Document.Messages, m)
		}
	}
	selected := sub.ArtifactID
	if selected == nil && len(v.Document.Artifacts) > 0 {
		id := v.Document.Artifacts[len(v.Document.Artifacts)-1].ID
		selected = &id
	}
	for _, a := range v.Document.Artifacts {
		if selected != nil && a.ID == *selected && a.IsTestSuite() {
			copy := a
			p.Artifact = &copy
			break
		}
	}
	if selected != nil && p.Artifact == nil {
		return Operation{}, fault("artifact_required", "Choose the tests you want to discuss.")
	}
	viewed := sub.ViewedRunID
	if sub.BaselineID != nil {
		viewed = sub.BaselineID
	}
	if sub.ViewedRunID != nil && sub.BaselineID != nil && *sub.ViewedRunID != *sub.BaselineID {
		return Operation{}, fault("baseline_required", "Choose one result to discuss.")
	}
	if sub.ViewedCaseKey != "" && viewed == nil {
		return Operation{}, fault("baseline_required", "Choose the result containing that test.")
	}
	if viewed != nil {
		o, err := s.Store.Operation(ctx, *viewed)
		if err != nil || o.SessionID != v.ID || (o.Kind != "check" && o.Kind != "retest") {
			return Operation{}, fault("baseline_required", "That result is unavailable in this conversation.")
		}
		if !o.State.Terminal() {
			return Operation{}, fault("baseline_required", "Those tests are still running. Wait for their results before discussing them.")
		}
		var original Plan
		if err = json.Unmarshal(o.Input, &original); err != nil {
			return Operation{}, err
		}
		if original.Artifact == nil || !original.Artifact.IsTestSuite() {
			return Operation{}, fault("baseline_required", "The tests and instructions for that result are unavailable.")
		}
		p.ObservedArtifact = original.Artifact
		// A historical suite can gain review metadata immediately before this
		// run. Its admitted input remains immutable; recover only matching
		// integrity metadata, never newer instructions or different test bytes.
		for _, saved := range v.Document.Artifacts {
			if saved.ID == original.Artifact.ID && validArtifactPolicy(v.Document, saved) {
				before, e1 := CanonicalJSONHash(original.Artifact.Blueprint)
				after, e2 := CanonicalJSONHash(saved.Blueprint)
				if e1 == nil && e2 == nil && before == after {
					copy := *original.Artifact
					copy.PolicyID, copy.Validation, copy.Provenance = saved.PolicyID, saved.Validation, saved.Provenance
					p.ObservedArtifact = &copy
				}
				break
			}
		}
		keys := append([]string(nil), original.Cases...)
		// Match the scorecard snapshot ordering for references such as “second”.
		sort.Strings(keys)
		if sub.ViewedCaseKey != "" {
			found := false
			for _, key := range keys {
				if key == sub.ViewedCaseKey {
					found = true
					break
				}
			}
			if !found {
				return Operation{}, fault("case_not_found", "That test is not part of the selected result.")
			}
			keys = []string{sub.ViewedCaseKey}
		}
		for _, key := range keys {
			c, err := s.Store.GetCase(ctx, actor, o.ID, key)
			if err != nil {
				return Operation{}, err
			}
			if c.Version != original.Artifact.ID.String() {
				return Operation{}, fault("baseline_required", "The result does not match its recorded instructions.")
			}
			p.Observations = append(p.Observations, c)
		}
	}
	p.Calls = 3 // Router, optional narrow handler, and one repair across the whole turn.
	if s.Config.ReliableAuthoring {
		if err := prepareReliableContext(&p, v, s.Config.SourcePolicyVersion); err != nil {
			return Operation{}, err
		}
		if err := s.freezeReviewVersion(&p); err != nil {
			return Operation{}, err
		}
		if s.Config.ConversationState {
			if err := prepareConversationState(&p, v); err != nil {
				return Operation{}, err
			}
			if s.Config.PreciseActions {
				p.AuthoringVersion = preciseAuthoringVersion
				p.Conversation.ContractVersion = "vibe-v13"
			}
		}
	}
	profile, err := s.Config.Profile(sub.Models.Assistant)
	if err != nil {
		return Operation{}, err
	}
	if p.Conversation != nil {
		p.Conversation.Profile = &profile
	}
	l := p.limits()
	cost, err := profile.BoundCost(profile.inputLimit(l), l.OutputTokens)
	if err != nil {
		return Operation{}, err
	}
	p.MaxCost = cost * int64(p.Calls)
	if p.AuthoringVersion >= 11 {
		err = fitReliableContext(&p, profile)
	} else {
		err = fitTestConversation(&p, profile)
	}
	if err != nil {
		return Operation{}, err
	}
	return s.Store.Submit(ctx, actor, v.ID, sub, p, s.Config)
}

// Exact source excerpts and versioned artifacts are durable memory. This is
// extractive compaction, not a lossy paraphrase that can change a policy.
func testConversationMessages(p Plan) []provider.Message {
	history := make([]map[string]any, 0, len(p.Document.Messages))
	for _, m := range p.Document.Messages {
		history = append(history, map[string]any{"id": m.ID, "role": m.Role, "content": m.Content})
	}
	facts := []map[string]any{}
	for _, q := range p.Document.Requirements {
		if q.Status == "superseded" || q.Status == "rejected" {
			continue
		}
		facts = append(facts, map[string]any{"id": q.ID, "source_message_id": q.SourceMessageID, "excerpt": q.Statement, "status": q.Status})
	}
	compact := func(a *Artifact) any {
		if a == nil {
			return nil
		}
		return map[string]any{"id": a.ID, "title": a.Title, "instructions": a.AgentPrompt, "tests": a.Blueprint, "dismissed": a.Dismissed}
	}
	return []provider.Message{{Role: "system", Content: testConversationPrompt}, {Role: "user", Content: string(raw(map[string]any{
		"recent_conversation": history, "earlier_conversation_compacted_through": p.ContextThrough,
		"source_excerpts": facts, "selected_tests": compact(p.Artifact), "viewed_agent": compact(p.ObservedArtifact),
		"viewed_results": p.Observations, "requested_purpose": p.Submission.Purpose,
	}))}, {Role: "user", Content: p.Submission.Content}}
}

func fitTestConversation(p *Plan, profile ModelProfile) error {
	// Keep a useful recent exchange by byte budget even on large-context models;
	// saved source excerpts carry older requirements forward without full replies.
	historyBytes := func() int {
		n := 0
		for _, m := range p.Document.Messages {
			n += len(m.Content)
		}
		return n
	}
	trim := func() bool {
		if len(p.Document.Messages) == 0 || len(p.Document.Requirements) == 0 && len(p.Document.Messages) <= 1 {
			return false
		}
		index := 0
		// Old v9 sessions may not yet have source excerpts. Keep their initial
		// description until durable memory exists, together with recent turns.
		if len(p.Document.Requirements) == 0 {
			index = 1
		}
		id := p.Document.Messages[index].ID
		p.ContextThrough = &id
		p.Document.Messages = append(p.Document.Messages[:index:index], p.Document.Messages[index+1:]...)
		return true
	}
	for historyBytes() > 12000 {
		if !trim() {
			break
		}
	}
	for {
		_, err := CountContext(provider.Request{Messages: testConversationMessages(*p), ResponseFormat: testConversationFormat(profile, *p), MaxOutputTokens: p.limits().OutputTokens}, profile, p.limits())
		if err == nil {
			return nil
		}
		if !trim() {
			return err
		}
	}
}

func applyContextQuotes(d *Document, changes []ContextQuote, sub Submission, replyID uuid.UUID) error {
	if len(changes) > 5 {
		return fmt.Errorf("remember at most five relevant source excerpts")
	}
	for _, change := range changes {
		quote := strings.TrimSpace(change.Quote)
		if quote == "" || len(quote) > 4000 || !strings.Contains(sub.Content, quote) {
			return fmt.Errorf("memory quote must be an exact, bounded excerpt of the latest user message")
		}
		var supersedes *uuid.UUID
		if change.SupersedesID != "" {
			id, err := uuid.Parse(change.SupersedesID)
			if err != nil {
				return fmt.Errorf("memory supersedes_id must reference an active source excerpt")
			}
			found := false
			for i := range d.Requirements {
				q := &d.Requirements[i]
				if q.ID == id && q.Status == "provided" {
					q.Status = "superseded"
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("only an active provided source excerpt may be superseded")
			}
			supersedes = &id
		}
		duplicate := false
		for _, q := range d.Requirements {
			if q.Status == "provided" && q.Statement == quote {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		d.Requirements = append(d.Requirements, Requirement{ID: uuid.New(), Statement: quote, Status: "provided", SourceMessageID: sub.ClientID, ProposalMessageID: &replyID, ProposedBy: "user_excerpt", SupersedesID: supersedes})
	}
	return nil
}
