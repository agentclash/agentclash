package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The proposal is a frozen group of displayed facts, never a model-issued
// permission. Viewing it leaves all facts proposed until an explicit adoption.
type RuleProposal struct {
	ID       string               `json:"id"`
	Revision int64                `json:"revision"`
	ScopeID  string               `json:"scope_id"`
	Status   string               `json:"status"`
	Question interaction.Question `json:"question"`
	Facts    []interaction.Fact   `json:"facts"`
}
type InteractionReceipt struct {
	ID          string `json:"id"`
	RequestHash string `json:"request_hash"`
	Revision    int64  `json:"revision"`
	Kind        string `json:"kind"`
	Summary     string `json:"summary"`
}

// Only the most recent reversible change retains an undo snapshot. Historical
// messages, policy versions, results and lightweight idempotency receipts stay.
type ConversationChange struct {
	ID               string             `json:"id"`
	Revision         int64              `json:"revision"`
	ScopeID          string             `json:"scope_id"`
	MessageID        string             `json:"message_id"`
	Summary          string             `json:"summary"`
	BeforeArtifactID *uuid.UUID         `json:"before_artifact_id,omitempty"`
	AfterArtifactID  *uuid.UUID         `json:"after_artifact_id,omitempty"`
	BeforeState      *ConversationState `json:"before_state,omitempty"`
	AfterStateHash   string             `json:"after_state_hash"`
	RuleIDs          []string           `json:"rule_ids,omitempty"`
}

func DecodeInteraction(b []byte) (interaction.Action, error) {
	var a interaction.Action
	e, err := interaction.Decode(b)
	if err != nil || e.Kind != "action" {
		return a, fault("invalid_request", "Choose one available action.")
	}
	err = json.Unmarshal(e.Payload, &a)
	return a, err
}
func staleInteraction() error {
	return fault("revision_conflict", "This choice has changed. Reload and use the latest version.")
}
func activeQuestion(s *ConversationState, a interaction.Action) (*interaction.Question, error) {
	if s == nil || a.ScopeID != s.Brief.ScopeID {
		return nil, staleInteraction()
	}
	q := s.PendingQuestion
	if q == nil || q.Status != "active" || q.ScopeID != a.ScopeID || q.ID != a.TargetID || q.Revision != a.TargetRevision {
		return nil, staleInteraction()
	}
	return q, nil
}

// Both typed options and model-interpreted natural answers use this resolver.
// It validates the displayed question, options and source; it grants no other action.
func answerBoundQuestion(s *ConversationState, a interaction.Action, current Message, quote string, unknown bool) error {
	q, err := activeQuestion(s, a)
	if err != nil {
		return err
	}
	if q.Purpose != "clarify_job" && q.Purpose != "clarify_rule" {
		return fmt.Errorf("this question needs its explicit confirmation action")
	}
	if a.Kind != "answer_question" || strings.TrimSpace(quote) == "" || !strings.Contains(current.Content, quote) || len(a.OptionIDs) > q.MaxSelections || unknown && len(a.OptionIDs) > 0 {
		return fmt.Errorf("invalid answer binding")
	}
	seen := map[string]bool{}
	for _, id := range a.OptionIDs {
		found := false
		for _, option := range q.Options {
			if option.ID == id {
				found = true
			}
		}
		if !found || seen[id] {
			return fmt.Errorf("unknown or duplicate option")
		}
		seen[id] = true
	}
	for _, option := range q.Options {
		if strings.EqualFold(strings.TrimSpace(quote), option.Label) && len(a.OptionIDs) > 0 && (len(a.OptionIDs) != 1 || a.OptionIDs[0] != option.ID) {
			return fmt.Errorf("selected option contradicts the literal answer")
		}
	}
	q.Status = "answered"
	s.Answers = append(s.Answers, QuestionAnswer{Question: *q, Source: stateSource(current, quote), OptionIDs: a.OptionIDs, Unknown: unknown, Action: &a})
	return nil
}

func applyStateInteraction(s *ConversationState, a interaction.Action, current Message) (string, error) {
	if s == nil || a.ScopeID != s.Brief.ScopeID {
		return "", staleInteraction()
	}
	switch a.Kind {
	case "answer_question":
		if err := answerBoundQuestion(s, a, current, current.Content, false); err != nil {
			return "", err
		}
		return "Answer noted. You can add more detail or ask me to prepare tests.", nil
	case "dismiss_help":
		q, err := activeQuestion(s, a)
		if err != nil {
			return "", err
		}
		if q.Purpose != "offer_help" {
			return "", fault("invalid_request", "Only optional help can be skipped here.")
		}
		q.Status = "dismissed"
		s.Guidance.Events = append(s.Guidance.Events, GuidanceEvent{MessageID: current.ID.String(), Kind: "dismissed", Topic: q.Purpose})
		if len(s.Guidance.Events) > 32 {
			s.Guidance.Events = s.Guidance.Events[len(s.Guidance.Events)-32:]
		}
		return "Help skipped.", nil
	case "adopt_proposal", "reject_proposal":
		p := s.Proposal
		if p == nil || p.Status != "proposed" || p.ID != a.TargetID || p.Revision != a.TargetRevision || p.ScopeID != a.ScopeID {
			return "", staleInteraction()
		}
		if a.Kind == "reject_proposal" {
			p.Status = "rejected"
			for i, f := range s.Brief.Facts {
				for _, proposed := range p.Facts {
					if f.ID == proposed.ID && f.Status == "proposed" {
						s.Brief.Facts[i].Status = "superseded"
					}
				}
			}
			s.Brief.Revision++
			return "Those suggestions were left out.", nil
		}
		p.Status = "adopted"
		q := p.Question
		q.Status = "answered"
		s.Answers = append(s.Answers, QuestionAnswer{Question: q, Source: stateSource(current, current.Content), Action: &a, AdoptedFacts: append([]interaction.Fact(nil), p.Facts...)})
		for _, proposed := range p.Facts {
			found := false
			for i, f := range s.Brief.Facts {
				if f.ID == proposed.ID && f.Status == "proposed" && Hash(raw(f)) == Hash(raw(proposed)) {
					found = true
					s.Brief.Facts[i].Status = "accepted"
					// The short consent is interpreted with the immutable displayed
					// question above, which travels to the independent reviewer.
					id := current.ID.String()
					s.Brief.Facts[i].AdoptionMessageID = &id
					s.Brief.Facts[i].Sources = []interaction.Source{stateSource(current, current.Content)}
				}
			}
			if !found {
				return "", staleInteraction()
			}
		}
		s.Brief.Revision++
		return "I’ll use these checks for your tests.", nil
	default:
		return "", fault("invalid_request", "That action is unavailable here.")
	}
}

func interactionMessage(s *ConversationState, a interaction.Action) (Message, error) {
	content := ""
	switch a.Kind {
	case "answer_question":
		q, err := activeQuestion(s, a)
		if err != nil {
			return Message{}, err
		}
		if a.Text != nil {
			content = *a.Text
		} else {
			labels := []string{}
			for _, id := range a.OptionIDs {
				for _, o := range q.Options {
					if o.ID == id {
						labels = append(labels, o.Label)
					}
				}
			}
			content = strings.Join(labels, ", ")
		}
	case "adopt_proposal":
		content = "Use these checks"
	case "reject_proposal":
		content = "Leave these suggestions out"
	case "dismiss_help":
		content = "Skip this help"
	case "undo":
		content = "Undo this change"
	}
	if strings.TrimSpace(content) == "" {
		return Message{}, fault("invalid_request", "Choose an answer first.")
	}
	return Message{ID: uuid.MustParse(a.IdempotencyKey), Role: "user", Content: content, Origin: "interaction", CreatedAt: timestamp()}, nil
}

// Exact immediate answers can bypass classification too. No substring matching:
// quotations, compound instructions and floating yeses take the normal route.
func boundDialogueAction(p Plan) *interaction.Action {
	if !p.precise() || p.Conversation == nil || p.Conversation.State == nil {
		return nil
	}
	s := p.Conversation.State
	last := ""
	for i := len(p.Document.Messages) - 1; i >= 0; i-- {
		m := p.Document.Messages[i]
		if m.ID != p.sourceMessageID() && m.Origin != "playground" {
			if m.Role == "assistant" {
				last = m.ID.String()
			}
			break
		}
	}
	a := interaction.Action{IdempotencyKey: p.sourceMessageID().String(), ScopeID: s.Brief.ScopeID, SessionRevision: p.Submission.Revision, OptionIDs: []string{}}
	text := strings.TrimSpace(p.Submission.Content)
	if proposal := s.Proposal; proposal != nil && proposal.Status == "proposed" && proposal.Question.OriginMessageID == last {
		switch strings.ToLower(strings.Trim(text, ".!")) {
		case "yes", "yes please", "use these checks", "use those":
			a.Kind = "adopt_proposal"
		case "no", "no thanks", "leave these out":
			a.Kind = "reject_proposal"
		}
		if a.Kind != "" {
			a.TargetID = proposal.ID
			a.TargetRevision = proposal.Revision
			return &a
		}
	}
	if q := s.PendingQuestion; q != nil && q.Status == "active" && q.OriginMessageID == last && (q.Purpose == "clarify_job" || q.Purpose == "clarify_rule") {
		for _, option := range q.Options {
			if strings.EqualFold(text, option.Label) {
				a.Kind = "answer_question"
				a.TargetID = q.ID
				a.TargetRevision = q.Revision
				a.OptionIDs = []string{option.ID}
				return &a
			}
		}
	}
	return nil
}

func (s *Store) ApplyInteraction(ctx context.Context, actor string, id uuid.UUID, a interaction.Action) error {
	if checkWire("action", a) != nil {
		return fault("invalid_request", "Choose one available action.")
	}
	return s.transaction(ctx, func(tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", id))
		if err != nil {
			return err
		}
		if actor != v.Actor {
			return fault("not_found", "Conversation is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, true); err != nil {
			return err
		}
		for _, r := range v.Document.Interactions {
			if r.ID == a.IdempotencyKey {
				if r.RequestHash != Hash(raw(a)) {
					return fault("idempotency_conflict", "This action ID was already used for another choice.")
				}
				return nil
			}
		}
		var used bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND client_id=$2)", id, a.IdempotencyKey).Scan(&used); err != nil {
			return err
		}
		if used {
			return fault("idempotency_conflict", "This message ID is already in use.")
		}
		// Action and message identities share a namespace, including failed turns.
		for _, m := range v.Document.Messages {
			if m.ID.String() == a.IdempotencyKey {
				return fault("idempotency_conflict", "This message ID is already in use.")
			}
		}
		if v.Revision != a.SessionRevision {
			return staleInteraction()
		}
		var busy bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND state IN ('AWAITING_APPROVAL','QUEUED','RUNNING','CANCELLING','FINALIZING'))", id).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return fault("operation_running", "Wait for the current response or stop it before making this choice.")
		}
		if err = validateConversationState(v.Document.ConversationState, v.Document); err != nil {
			return err
		}
		before := cloneState(v.Document.ConversationState)
		message, err := interactionMessage(before, a)
		if err != nil {
			return err
		}
		summary := ""
		if a.Kind == "undo" {
			summary, err = undoConversationChange(&v, a)
		} else {
			summary, err = applyStateInteraction(v.Document.ConversationState, a, message)
		}
		if err != nil {
			return err
		}
		replyID := deterministicID(message.ID, "interaction-reply")
		v.Document.ConversationState.ThroughMessageID = replyID.String()
		v.Document.Messages = append(v.Document.Messages, message, Message{ID: replyID, Role: "assistant", Content: summary, Origin: "interaction", CreatedAt: timestamp()})
		if err = validateConversationState(v.Document.ConversationState, v.Document); err != nil {
			return err
		}
		r := InteractionReceipt{ID: a.IdempotencyKey, RequestHash: Hash(raw(a)), Revision: v.Revision + 1, Kind: a.Kind, Summary: summary}
		v.Document.Interactions = append(v.Document.Interactions, r)
		if a.Kind != "undo" {
			v.Document.LastChange = &ConversationChange{ID: a.IdempotencyKey, Revision: v.Revision + 1, ScopeID: a.ScopeID, MessageID: replyID.String(), Summary: summary, BeforeState: before, AfterStateHash: Hash(raw(v.Document.ConversationState)), BeforeArtifactID: latestArtifactID(v.Document), AfterArtifactID: latestArtifactID(v.Document)}
		}
		if err = s.updateDocument(ctx, tx, v); err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "document.updated")
	})
}

func latestArtifactID(d Document) *uuid.UUID {
	if len(d.Artifacts) == 0 {
		return nil
	}
	id := d.Artifacts[len(d.Artifacts)-1].ID
	return &id
}
func undoConversationChange(v *Session, a interaction.Action) (string, error) {
	c := v.Document.LastChange
	s := v.Document.ConversationState
	if c == nil || s == nil || c.BeforeState == nil || c.ID != a.TargetID || c.Revision != a.TargetRevision || c.ScopeID != a.ScopeID || s.Brief.ScopeID != a.ScopeID || c.AfterStateHash != Hash(raw(s)) || Hash(raw(c.AfterArtifactID)) != Hash(raw(latestArtifactID(v.Document))) {
		return "", staleInteraction()
	}
	if Hash(raw(c.BeforeArtifactID)) != Hash(raw(c.AfterArtifactID)) {
		var previous *Artifact
		for _, artifact := range v.Document.Artifacts {
			if c.BeforeArtifactID != nil && artifact.ID == *c.BeforeArtifactID {
				copy := artifact
				previous = &copy
				break
			}
		}
		if previous == nil {
			return "", staleInteraction()
		}
		previous.ID = deterministicID(uuid.MustParse(a.IdempotencyKey), "undo-artifact")
		previous.ParentID = c.AfterArtifactID
		previous.Accepted = false
		previous.Dismissed = false
		previous.CreatedAt = timestamp()
		previous.SourceMessageID = uuid.MustParse(a.IdempotencyKey)
		reply := deterministicID(previous.SourceMessageID, "interaction-reply")
		previous.ProposalMessageID = &reply
		v.Document.Artifacts = append(v.Document.Artifacts, *previous)
	}
	restored := cloneState(c.BeforeState)
	restored.ActionsVersion = 1
	restored.Brief.Revision = s.Brief.Revision + 1
	if restored.PendingQuestion != nil && restored.PendingQuestion.Status == "active" {
		restored.PendingQuestion.Revision++
	}
	if restored.Proposal != nil && restored.Proposal.Status == "proposed" {
		restored.Proposal.Revision++
		restored.Proposal.Question.Revision++
		rev := restored.Proposal.Revision
		restored.Proposal.Question.ProposalRevision = &rev
	}
	v.Document.ConversationState = restored
	v.Document.SourceConfirmation = nil // An earlier affirmative is never resurrected.
	v.Document.LastChange = nil
	return "Change undone. Earlier tests and results are still kept.", nil
}

func validateProposal(p *RuleProposal, d Document) error {
	if p == nil {
		return nil
	}
	if p.Status != "proposed" && p.Status != "adopted" && p.Status != "rejected" {
		return fmt.Errorf("invalid proposal status")
	}
	q := p.Question
	if p.ID == "" || p.Revision < 1 || q.ProposalID == nil || *q.ProposalID != p.ID || q.ProposalRevision == nil || *q.ProposalRevision != p.Revision || q.ScopeID != p.ScopeID || q.Purpose != "adopt_proposal" || checkWire("question", q) != nil || len(p.Facts) == 0 || len(p.Facts) > 4 {
		return fmt.Errorf("invalid displayed proposal")
	}
	seen := map[string]bool{}
	for _, f := range p.Facts {
		if seen[f.ID] || f.Status != "proposed" || f.Text == nil || !strings.Contains(q.Text, *f.Text) || len(f.Sources) != 1 || f.Sources[0].MessageID != q.OriginMessageID || !sourceExists(d, f.Sources[0], "assistant") {
			return fmt.Errorf("proposal does not match displayed suggestions")
		}
		seen[f.ID] = true
	}
	for _, m := range d.Messages {
		if m.ID.String() == q.OriginMessageID && m.Role == "assistant" && strings.Contains(m.Content, q.Text) {
			return nil
		}
	}
	return fmt.Errorf("proposal question was not displayed")
}

// Recover only an identical immutable action, independently of current models,
// question state and source-confirmation cleanup after a completed operation.
func (s *Store) submissionReceiptForAction(ctx context.Context, id uuid.UUID, a interaction.Action) (*Operation, error) {
	var o Operation
	err := s.DB.QueryRow(ctx, "SELECT id,input FROM vibe_operations WHERE session_id=$1 AND client_id=$2", id, a.IdempotencyKey).Scan(&o.ID, &o.Input)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Plan
	if json.Unmarshal(o.Input, &p) != nil || p.Submission.Interaction == nil || Hash(raw(p.Submission.Interaction)) != Hash(raw(a)) {
		return nil, fault("idempotency_conflict", "This action ID was already used for another choice.")
	}
	return &o, nil
}
