package vibe

import (
	"context"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
	"github.com/google/uuid"
)

func validateSourceAction(v Session, sub Submission) error {
	a := sub.Interaction
	if a == nil || checkWire("action", a) != nil || a.SessionRevision != v.Revision || sub.Revision != v.Revision || sub.ClientID.String() != a.IdempotencyKey || sub.Kind != "message" || !sub.TestJourney || sub.Content != "Use these messages" {
		return staleInteraction()
	}
	// A source choice cannot carry a second command through /messages.
	if sub.Purpose != "" || sub.Instructions != "" || sub.RetryOf != nil || sub.BaselineID != nil || sub.ViewedRunID != nil || sub.ViewedCaseKey != "" || sub.ApproveArtifact || sub.QuickCheck || sub.EvaluationFirst || sub.EvidenceSetID != nil || sub.PreviewThreadID != nil || sub.JourneyMode != "" {
		return staleInteraction()
	}
	q, err := activeQuestion(v.Document.ConversationState, *a)
	if err != nil {
		return err
	}
	if a.Kind != "answer_question" || q.Purpose != "choose_source" || len(a.OptionIDs) != 1 || a.OptionIDs[0] != "use-sources" || a.Text != nil || v.Document.SourceConfirmation == nil {
		return staleInteraction()
	}
	c := v.Document.SourceConfirmation
	if q.OriginMessageID != deterministicID(c.OperationID, "completion-message").String() || q.Text != c.Question || Hash(raw(sub.ArtifactID)) != Hash(raw(c.ArtifactID)) || !originalConfirmationSources(v.Document, c) {
		return staleInteraction()
	}
	return nil
}

// Most choices are pure state changes. Source adoption continues the exact
// already-requested authoring action, using frozen source IDs and case count.
func (s *Service) Interact(ctx context.Context, actor string, id uuid.UUID, a interaction.Action) error {
	if checkWire("action", a) != nil {
		return fault("invalid_request", "Choose one available action.")
	}
	if !s.Config.PreciseActions {
		return fault("invalid_request", "Reload to use the current conversation controls.")
	}
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return err
	}
	// Recover source-click delivery even after it resolves the question.
	if a.Kind == "answer_question" && len(a.OptionIDs) == 1 && a.OptionIDs[0] == "use-sources" {
		sub := Submission{ClientID: uuid.MustParse(a.IdempotencyKey), Revision: a.SessionRevision, Kind: "message", Content: "Use these messages", Models: v.Document.Models, TestJourney: true, Interaction: &a}
		if v.Document.SourceConfirmation != nil {
			sub.ArtifactID = v.Document.SourceConfirmation.ArtifactID
		}
		if op, e := s.Store.submissionReceiptForAction(ctx, id, a); op != nil || e != nil {
			return e
		}
		if err = validateSourceAction(v, sub); err != nil {
			return err
		}
		_, err = s.Prepare(ctx, actor, id, sub)
		return err
	}
	return s.Store.ApplyInteraction(ctx, actor, id, a)
}

func changedRuleIDs(before, after PolicySnapshot) []string {
	old := map[string]string{}
	changed := []string{}
	for _, r := range before.Rules {
		old[r.ID] = Hash(raw(r))
	}
	for _, r := range after.Rules {
		if old[r.ID] != Hash(raw(r)) {
			changed = append(changed, r.ID)
		}
		delete(old, r.ID)
	}
	for _, r := range before.Rules {
		if _, ok := old[r.ID]; ok {
			changed = append(changed, r.ID)
		}
	}
	return changed
}
