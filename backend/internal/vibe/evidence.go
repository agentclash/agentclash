package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Evidence is versioned separately from instructions. Content is always copied
// from the submitted source, never reconstructed by the author or evaluator.
type EvidenceMessage struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}
type EvidenceConversation struct {
	Key      string            `json:"key"`
	Title    string            `json:"title"`
	Messages []EvidenceMessage `json:"messages"`
}
type EvidenceSet struct {
	ID            uuid.UUID              `json:"id"`
	ParentID      *uuid.UUID             `json:"parent_id,omitempty"`
	Label         string                 `json:"label"`
	Raw           string                 `json:"raw"`
	Context       string                 `json:"context,omitempty"`
	Conversations []EvidenceConversation `json:"conversations"`
	CreatedAt     time.Time              `json:"created_at"`
}
type Expectation struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
}
type ConversationEvaluation struct {
	EvidenceSetID uuid.UUID     `json:"evidence_set_id"`
	Expectations  []Expectation `json:"expectations"`
}
type EvaluationSource struct {
	Kind          string     `json:"kind"` // provided_conversations or prompt
	Label         string     `json:"label"`
	EvidenceSetID *uuid.UUID `json:"evidence_set_id,omitempty"`
	ArtifactID    uuid.UUID  `json:"artifact_id"`
	Comparison    string     `json:"comparison,omitempty"` // updated_replies or rechecked
}

func (a Artifact) IsConversationEvaluation() bool {
	return a.Kind == "conversation_evaluation" && a.ConversationEvaluation != nil
}

var speakerLine = regexp.MustCompile(`(?im)^[\t ]*(?:\*\*|__)?(customer|user|human|agent|assistant|bot|system|tool)(?:\*\*|__)?[\t ]*:(?:\*\*|__)?[\t ]?`)
var conversationBreak = regexp.MustCompile(`(?m)^---[\t ]*\r?$`)

func evidenceRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "customer", "user", "human":
		return "user"
	case "agent", "assistant", "bot":
		return "assistant"
	case "system", "tool":
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return "unknown"
	}
}

// JSON accepts an ordered messages array, {messages:[...]}, or
// {conversations:[{title, messages:[...]}]}. Text uses speaker labels and an
// explicit --- line between chats. Unrecognised text stays visible as unknown.
func ParseEvidence(source, label string, l Limits) (EvidenceSet, error) {
	e := EvidenceSet{ID: uuid.New(), Label: strings.TrimSpace(label), Raw: source, CreatedAt: timestamp()}
	if e.Label == "" {
		e.Label = "Pasted conversations"
	}
	if len(e.Label) > 160 || strings.TrimSpace(source) == "" || len(source) > l.FileBytes || !utf8.ValidString(source) {
		return e, fault("invalid_evidence", "Add a UTF-8 chat within the attachment size limit.")
	}
	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if err := ValidateJSON([]byte(source), l); err != nil {
			return e, fault("invalid_evidence", "The chat JSON has invalid, ambiguous or oversized fields. The source has not been changed.")
		}
		type turn struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		type chat struct {
			Title    string `json:"title"`
			Messages []turn `json:"messages"`
		}
		var envelope struct {
			Messages      []turn `json:"messages"`
			Conversations []chat `json:"conversations"`
		}
		var err error
		if strings.HasPrefix(trimmed, "[") {
			err = json.Unmarshal([]byte(source), &envelope.Messages)
		} else {
			err = json.Unmarshal([]byte(source), &envelope)
		}
		if err != nil || len(envelope.Messages) > 0 && len(envelope.Conversations) > 0 {
			return e, fault("invalid_evidence", "Use one ordered messages array, or a conversations array containing messages.")
		}
		chats := envelope.Conversations
		if len(envelope.Messages) > 0 {
			chats = []chat{{Messages: envelope.Messages}}
		}
		if len(chats) == 0 {
			return e, fault("invalid_evidence", "No messages found. Each message needs a role and text content.")
		}
		for _, c := range chats {
			next := EvidenceConversation{Title: c.Title}
			for _, m := range c.Messages {
				next.Messages = append(next.Messages, EvidenceMessage{Role: evidenceRole(m.Role), Content: m.Content})
			}
			e.Conversations = append(e.Conversations, next)
		}
	} else {
		for _, part := range conversationBreak.Split(source, -1) {
			if strings.TrimSpace(part) == "" {
				return e, fault("invalid_evidence", "Remove empty chats between separator lines.")
			}
			c := EvidenceConversation{}
			matches := speakerLine.FindAllStringSubmatchIndex(part, -1)
			if len(matches) == 0 {
				c.Messages = append(c.Messages, EvidenceMessage{Role: "unknown", Content: part})
			} else {
				if strings.TrimSpace(part[:matches[0][0]]) != "" {
					c.Messages = append(c.Messages, EvidenceMessage{Role: "unknown", Content: part[:matches[0][0]]})
				}
				for i, m := range matches {
					end := len(part)
					if i+1 < len(matches) {
						end = matches[i+1][0]
					}
					c.Messages = append(c.Messages, EvidenceMessage{Role: evidenceRole(part[m[2]:m[3]]), Content: part[m[1]:end]})
				}
			}
			e.Conversations = append(e.Conversations, c)
		}
	}
	if len(e.Conversations) > 3 {
		return e, fault("case_limit", "Choose up to 3 whole conversations for this check. Nothing has been dropped or checked.")
	}
	for i := range e.Conversations {
		c := &e.Conversations[i]
		if c.Title == "" {
			c.Title = fmt.Sprintf("Conversation %d", i+1)
		}
		if len(c.Title) > 160 || len(c.Messages) == 0 || len(c.Messages) > 200 {
			return e, fault("invalid_evidence", "Each chat needs 1–200 messages and a short title.")
		}
		for j := range c.Messages {
			m := &c.Messages[j]
			m.ID = fmt.Sprintf("c%d-m%d", i+1, j+1)
			if strings.TrimSpace(m.Content) == "" {
				return e, fault("invalid_evidence", "A chat contains an empty message. Add its text or remove the empty turn.")
			}
		}
		c.Key = fmt.Sprintf("chat-%d", i+1)
	}
	return e, nil
}

// ParseMessageEvidence finds a candidate source without asking the author to
// rewrite any reply. A prelude belongs to the user's brief/reference, never to
// an invented speaker. The author must still distinguish a recorded chat from
// examples in instructions before this candidate becomes active evidence.
func ParseMessageEvidence(source string, l Limits) (*EvidenceSet, error) {
	trimmed := strings.TrimSpace(source)
	prefix := ""
	transcript := source
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if !chatJSONShape.MatchString(trimmed) && !(strings.HasPrefix(trimmed, "[") && chatArrayRole.MatchString(trimmed)) {
			return nil, nil
		}
	} else {
		matches := speakerLine.FindAllStringSubmatchIndex(source, -1)
		user, assistant := false, false
		for _, m := range matches {
			switch evidenceRole(source[m[2]:m[3]]) {
			case "user":
				user = true
			case "assistant":
				assistant = true
			}
		}
		if !user || !assistant {
			return nil, nil
		}
		prefix, transcript = source[:matches[0][0]], source[matches[0][0]:]
	}
	e, err := ParseEvidence(transcript, "Pasted conversation", l)
	if err != nil {
		return nil, err
	}
	e.Raw, e.Context = source, prefix
	return &e, nil
}

var chatJSONShape = regexp.MustCompile(`"(?:messages|conversations)"\s*:`)
var chatArrayRole = regexp.MustCompile(`"role"\s*:`)

func (e EvidenceSet) ValidateReady() error {
	if len(e.Conversations) == 0 || len(e.Conversations) > 3 {
		return fault("invalid_evidence", "Choose 1–3 complete chats.")
	}
	for _, c := range e.Conversations {
		user, agent := false, false
		for _, m := range c.Messages {
			switch m.Role {
			case "user":
				user = true
			case "assistant":
				agent = true
			case "system", "tool":
			default:
				return fault("evidence_roles_required", "Identify every speaker before checking. The original chat is preserved.")
			}
		}
		if !user || !agent {
			return fault("invalid_evidence", "Each chat must contain a customer message and an actual agent reply.")
		}
	}
	return nil
}

func findEvidence(d Document, id *uuid.UUID) *EvidenceSet {
	if id == nil {
		id = d.ActiveEvidenceID
	}
	if id != nil {
		for _, e := range d.EvidenceSets {
			if e.ID == *id {
				copy := e
				return &copy
			}
		}
	}
	return nil
}

type EvidenceInput struct {
	Revision int64             `json:"revision"`
	Content  string            `json:"content"`
	Label    string            `json:"label"`
	ParentID *uuid.UUID        `json:"parent_id,omitempty"`
	Roles    map[string]string `json:"roles,omitempty"`
}

func (s *Service) AddEvidence(ctx context.Context, actor string, id uuid.UUID, in EvidenceInput) error {
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return err
	}
	l := s.Config.Limits(v.Anonymous)
	if err = s.Gate.Check(ctx, actor, l); err != nil {
		return err
	}
	var evidence EvidenceSet
	if in.ParentID != nil {
		old := findEvidence(v.Document, in.ParentID)
		if old == nil {
			return fault("not_found", "The original chat is unavailable.")
		}
		evidence, err = ParseEvidence(old.Raw, old.Label, l)
		if err == nil {
			err = json.Unmarshal(raw(old.Conversations), &evidence.Conversations)
		}
		evidence.ParentID = in.ParentID
		evidence.Context = old.Context
	} else {
		evidence, err = ParseEvidence(in.Content, in.Label, l)
	}
	if err != nil {
		return err
	}
	for key, role := range in.Roles {
		if role != "user" && role != "assistant" && role != "system" && role != "tool" {
			return fault("invalid_evidence", "Choose customer, agent, system or tool for each speaker.")
		}
		found := false
		for i := range evidence.Conversations {
			for j := range evidence.Conversations[i].Messages {
				m := &evidence.Conversations[i].Messages[j]
				if m.ID == key {
					m.Role = role
					found = true
				}
			}
		}
		if !found {
			return fault("invalid_evidence", "A speaker correction refers to a missing message.")
		}
	}
	return s.Store.Edit(ctx, actor, id, in.Revision, func(v *Session) error {
		return appendEvidence(v, evidence, l, s.Config.TestingLocally())
	})
}

func appendEvidence(v *Session, evidence EvidenceSet, l Limits, localTesting bool) error {
	if evidence.ParentID == nil && !localTesting && v.Document.AttachmentCount >= l.Files {
		return fault("attachment_limit", "This conversation has reached its attachment limit.")
	}
	total := len(raw(evidence))
	for _, old := range v.Document.EvidenceSets {
		total += len(raw(old))
	}
	for _, a := range v.Document.Artifacts {
		total += len(a.Blueprint)
	}
	if !localTesting && total > l.StoredBytes {
		return fault("attachment_limit", "This conversation's attachment allowance is full.")
	}
	if evidence.ParentID == nil {
		v.Document.AttachmentCount++
	}
	v.Document.EvaluationFirst = true
	v.Document.EvidenceSets = append(v.Document.EvidenceSets, evidence)
	v.Document.ActiveEvidenceID = &evidence.ID
	return nil
}

func validateExpectations(items []Expectation, l Limits) error {
	if len(items) < 1 || len(items) > l.Checks {
		return fault("invalid_evaluation", "Use between 1 and 8 expectations.")
	}
	seen := map[string]bool{}
	for _, q := range items {
		if q.ID == "" || len(q.ID) > 64 || seen[q.ID] || strings.TrimSpace(q.Statement) == "" || len(q.Statement) > 2000 {
			return fault("invalid_evaluation", "Each expectation needs a unique ID and a short, specific rule.")
		}
		seen[q.ID] = true
	}
	return nil
}

func EditConversationEvaluation(v *Session, id uuid.UUID, expectations []Expectation) error {
	if err := validateExpectations(expectations, LimitsFor(v.Anonymous)); err != nil {
		return err
	}
	for _, a := range v.Document.Artifacts {
		if a.ID == id && a.IsConversationEvaluation() {
			copy := a
			copy.ID = uuid.New()
			copy.ParentID = &a.ID
			copy.Accepted = false
			copy.QuickCheck = false
			copy.CreatedAt = timestamp()
			copy.ConversationEvaluation = &ConversationEvaluation{EvidenceSetID: a.ConversationEvaluation.EvidenceSetID, Expectations: expectations}
			v.Document.Artifacts = append(v.Document.Artifacts, copy)
			return nil
		}
	}
	return fault("artifact_required", "Choose the conversation check to edit.")
}
