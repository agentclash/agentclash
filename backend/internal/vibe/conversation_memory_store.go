package vibe

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
)

// State, response, policy and completion receipt are written by the existing
// session transaction. There is no separate best-effort memory write.
func (r *Runner) completeReliableDocument(ctx context.Context, o Operation, p Plan, reply string, artifact *Artifact, requirements []Requirement, completion ...AuthoringCompletion) error {
	if !p.stateful() {
		return r.Service.Store.CompleteDocument(ctx, o.ID, reply, artifact, requirements, completion...)
	}
	if len(completion) != 1 || p.Conversation.NextState == nil {
		return fmt.Errorf("missing v12 conversation completion")
	}
	c := completion[0]
	s := cloneState(p.Conversation.NextState)
	if c.SourceConfirmation != nil {
		q := c.SourceConfirmation
		if p.precise() {
			copy := *q
			q = &copy
			c.SourceConfirmation = q
			q.Question += "\nUse these messages"
			reply = q.Question
		}
		s.PendingQuestion = &interaction.Question{ID: deterministicID(o.ID, "question").String(), ScopeID: s.Brief.ScopeID, Revision: 1, OriginMessageID: deterministicID(o.ID, "completion-message").String(), Purpose: "choose_source", Status: "active", Text: q.Question, Options: []interaction.Option{}, MaxSelections: 1}
		if p.precise() {
			s.PendingQuestion.Options = []interaction.Option{{ID: "use-sources", Label: "Use these messages"}}
		}
	} else if c.Outcome != nil && c.Outcome.Action == "clarify" && (s.PendingQuestion == nil || !strings.Contains(reply, s.PendingQuestion.Text)) {
		// A review can ask a necessary question after routing prepared a suite.
		s.PendingQuestion = &interaction.Question{ID: deterministicID(o.ID, "question").String(), ScopeID: s.Brief.ScopeID, Revision: 1, OriginMessageID: deterministicID(o.ID, "completion-message").String(), Purpose: "clarify_rule", Status: "active", Text: reply, Options: []interaction.Option{}, MaxSelections: 1}
	}
	if p.Conversation.Confirmed != nil && s.PendingQuestion != nil && s.PendingQuestion.Purpose == "choose_source" && c.SourceConfirmation == nil {
		s.PendingQuestion.Status = "answered"
	}
	if p.precise() {
		s.ActionsVersion = 1
	}
	if p.precise() && c.Policy != nil && p.Conversation.Policy != nil {
		alignBriefToPolicy(s, *p.Conversation.Policy, *c.Policy)
	}
	c.ConversationState = s
	return r.Service.Store.CompleteDocument(ctx, o.ID, reply, artifact, requirements, c)
}
