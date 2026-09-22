package vibe

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
	"github.com/google/uuid"
)

const conversationStateVersion = 1
const statefulAuthoringVersion = 12

// Dialogue memory is not executable policy. Only a reviewed PolicySnapshot
// authorizes tests. No user sophistication or inferred persona is stored here.
type ConversationState struct {
	ActionsVersion   int                   `json:"actions_version,omitempty"`
	Proposal         *RuleProposal         `json:"proposal,omitempty"`
	Version          int                   `json:"version"`
	ThroughMessageID string                `json:"through_message_id,omitempty"`
	Brief            interaction.Brief     `json:"brief"`
	PendingQuestion  *interaction.Question `json:"pending_question,omitempty"`
	Answers          []QuestionAnswer      `json:"answers,omitempty"`
	Guidance         GuidanceHistory       `json:"guidance"`
}
type QuestionAnswer struct {
	Action       *interaction.Action  `json:"action,omitempty"`
	AdoptedFacts []interaction.Fact   `json:"adopted_facts,omitempty"`
	Obsolete     bool                 `json:"obsolete,omitempty"`
	Question     interaction.Question `json:"question"`
	Source       interaction.Source   `json:"source"`
	OptionIDs    []string             `json:"option_ids,omitempty"`
	Unknown      bool                 `json:"unknown"`
}
type GuidanceHistory struct {
	Events  []GuidanceEvent     `json:"events,omitempty"`
	Brevity *interaction.Source `json:"brevity_preference,omitempty"`
}
type GuidanceEvent struct {
	MessageID string `json:"message_id"`
	Kind      string `json:"kind"`
	Topic     string `json:"topic"`
}

// Provider output nominates exact excerpts, never IDs, hashes, accepted facts
// or a replacement brief. The server merges it against the frozen base.
type memoryFact struct {
	Kind         string `json:"kind"`
	Quote        string `json:"quote"`
	SupersedesID string `json:"supersedes_id"`
}
type memoryAnswer struct {
	QuestionID       string   `json:"question_id"`
	QuestionRevision int64    `json:"question_revision"`
	Quote            string   `json:"quote"`
	OptionIDs        []string `json:"option_ids"`
	Unknown          bool     `json:"unknown"`
}
type memoryQuestion struct {
	Purpose       string   `json:"purpose"`
	Text          string   `json:"text"`
	Options       []string `json:"options"`
	MaxSelections int      `json:"max_selections"`
}
type memoryUpdate struct {
	Facts               []memoryFact    `json:"facts"`
	Suggestions         []memoryFact    `json:"suggestions"`
	Answer              *memoryAnswer   `json:"answer"`
	Question            *memoryQuestion `json:"question"`
	NewScopeQuote       string          `json:"new_scope_quote"`
	CancelQuestionQuote string          `json:"cancel_question_quote"`
	BrevityQuote        string          `json:"brevity_quote"`
	GuidanceKind        string          `json:"guidance_kind"`
	GuidanceTopic       string          `json:"guidance_topic"`
}

func (p Plan) stateful() bool { return p.AuthoringVersion == statefulAuthoringVersion || p.precise() }
func cloneState(s *ConversationState) *ConversationState {
	if s == nil {
		return nil
	}
	var out ConversationState
	_ = json.Unmarshal(raw(s), &out)
	return &out
}
func newConversationState(scope uuid.UUID) *ConversationState {
	s := &ConversationState{Version: conversationStateVersion, Brief: interaction.Brief{ScopeID: scope.String(), Revision: 1, Facts: []interaction.Fact{}}}
	for _, kind := range []string{"job", "has_agent", "has_pack"} {
		s.Brief.Facts = append(s.Brief.Facts, interaction.Fact{ID: deterministicID(scope, "unknown/"+kind).String(), Kind: kind, Status: "unknown", Sources: []interaction.Source{}})
	}
	return s
}
func stateSource(m Message, quote string) interaction.Source {
	return interaction.Source{MessageID: m.ID.String(), Quote: quote, SHA256: Hash([]byte(m.Content))}
}
func sourceExists(d Document, ref interaction.Source, role string) bool {
	for _, m := range d.Messages {
		if m.ID.String() == ref.MessageID && m.Role == role && m.Origin != "playground" && Hash([]byte(m.Content)) == ref.SHA256 && strings.TrimSpace(ref.Quote) != "" && strings.Contains(m.Content, ref.Quote) {
			return true
		}
	}
	return false
}
func checkWire(kind string, value any) error {
	_, err := interaction.Decode(raw(map[string]any{"version": 1, "kind": kind, "payload": value}))
	return err
}
func validateConversationState(s *ConversationState, d Document) error {
	if s == nil {
		return nil
	} // Legacy sessions have no reconstructed consent.
	if s.Version != conversationStateVersion || checkWire("brief", s.Brief) != nil {
		return fmt.Errorf("invalid conversation brief version or shape")
	}
	if err := validateProposal(s.Proposal, d); err != nil {
		return err
	}
	for _, fact := range s.Brief.Facts {
		for _, source := range fact.Sources {
			role := "user"
			if fact.Status == "proposed" {
				role = "assistant"
			}
			if !sourceExists(d, source, role) && !(fact.Status == "superseded" && sourceExists(d, source, "assistant")) {
				return fmt.Errorf("brief source does not match the original message")
			}
		}
		if fact.Status == "accepted" {
			matched := false
			for _, answer := range s.Answers {
				if answer.Action != nil && answer.Action.Kind == "adopt_proposal" && fact.AdoptionMessageID != nil && *fact.AdoptionMessageID == answer.Source.MessageID && Hash(raw(fact.Sources)) == Hash(raw([]interaction.Source{answer.Source})) {
					for _, adopted := range answer.AdoptedFacts {
						if adopted.ID == fact.ID && Hash(raw(adopted.Text)) == Hash(raw(fact.Text)) && adopted.Kind == fact.Kind {
							matched = true
						}
					}
				}
			}
			if !matched {
				return fmt.Errorf("accepted fact lacks its displayed proposal adoption")
			}
		}
		if fact.AdoptionMessageID != nil {
			found := false
			for _, m := range d.Messages {
				if m.ID.String() == *fact.AdoptionMessageID && m.Role == "user" && m.Origin != "playground" {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("brief adoption source is unavailable")
			}
		}
	}
	validateQuestion := func(q interaction.Question) error {
		if checkWire("question", q) != nil {
			return fmt.Errorf("invalid saved question")
		}
		for _, m := range d.Messages {
			if m.ID.String() == q.OriginMessageID && m.Role == "assistant" && m.Origin != "playground" && strings.Contains(m.Content, q.Text) {
				return nil
			}
		}
		return fmt.Errorf("question does not match its displayed assistant message")
	}
	if q := s.PendingQuestion; q != nil {
		if err := validateQuestion(*q); err != nil {
			return err
		}
		if q.Status == "active" && q.ScopeID != s.Brief.ScopeID {
			return fmt.Errorf("active question belongs to another agent")
		}
	}
	for _, answer := range s.Answers {
		if err := validateQuestion(answer.Question); err != nil {
			return err
		}
		if answer.Action != nil {
			a := answer.Action
			if checkWire("action", a) != nil || a.ScopeID != answer.Question.ScopeID || a.IdempotencyKey != answer.Source.MessageID {
				return fmt.Errorf("invalid stored answer action")
			}
			if a.Kind == "adopt_proposal" {
				proposal := &RuleProposal{ID: a.TargetID, Revision: a.TargetRevision, ScopeID: a.ScopeID, Status: "adopted", Question: answer.Question, Facts: answer.AdoptedFacts}
				if err := validateProposal(proposal, d); err != nil {
					return err
				}
			} else if a.Kind != "answer_question" || a.TargetID != answer.Question.ID || a.TargetRevision != answer.Question.Revision {
				return fmt.Errorf("answer target changed")
			}
		}
		if answer.Question.Status != "answered" || !sourceExists(d, answer.Source, "user") {
			return fmt.Errorf("invalid saved question answer")
		}
	}
	if s.Guidance.Brevity != nil && !sourceExists(d, *s.Guidance.Brevity, "user") {
		return fmt.Errorf("preference has no original source")
	}
	for _, e := range s.Guidance.Events {
		if e.Kind != "explanation" && e.Kind != "example" && e.Kind != "dismissed" || strings.TrimSpace(e.Topic) == "" || len(e.Topic) > 80 {
			return fmt.Errorf("invalid guidance event")
		}
		found := false
		for _, m := range d.Messages {
			if m.ID.String() == e.MessageID && m.Origin != "playground" && (e.Kind == "dismissed" && m.Role == "user" || e.Kind != "dismissed" && m.Role == "assistant") {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("guidance event has no displayed source")
		}
	}
	return nil
}

// Called before history is compacted. Required references are reloaded from the
// saved document and verified; retrieval itself never adopts old dialogue.
func prepareConversationState(p *Plan, v Session) error {
	if !p.sourceBoundary() {
		return fmt.Errorf("conversation state requires source separation")
	}
	if err := validateConversationState(v.Document.ConversationState, v.Document); err != nil {
		return err
	}
	p.AuthoringVersion = statefulAuthoringVersion
	p.Conversation.ContractVersion = "vibe-v12"
	s := cloneState(v.Document.ConversationState)
	if s == nil {
		scope := deterministicID(v.ID, "conversation-scope")
		if p.Conversation.Policy != nil {
			scope = p.Conversation.Policy.ScopeID
		}
		s = newConversationState(scope)
	}
	if v.Document.ConversationState != nil && s.ThroughMessageID != "" {
		// A rollback may complete a v11 turn without updating v12 memory. Keep
		// that saved history, but never bind a new answer to an obsolete question.
		seen, advanced := false, false
		for _, m := range v.Document.Messages {
			if m.ID.String() == s.ThroughMessageID {
				seen = true
				continue
			}
			if seen && m.Role == "assistant" && m.Origin != "playground" {
				advanced = true
			}
		}
		if !seen {
			return fmt.Errorf("conversation checkpoint is unavailable")
		}
		if advanced {
			if s.PendingQuestion != nil {
				s.PendingQuestion.Status = "superseded"
			}
			for i := range s.Brief.Facts {
				f := &s.Brief.Facts[i]
				if f.Status != "unknown" {
					f.Status = "superseded"
				}
			}
			s.Brief.Revision++
			for i := range s.Answers {
				s.Answers[i].Obsolete = true
			}
		}
	}
	p.Document.ConversationState = cloneState(v.Document.ConversationState)
	p.Conversation.State = s
	p.Conversation.StateBaseHash = Hash(raw(v.Document.ConversationState))
	if v.Document.ConversationState != nil && p.Artifact != nil && (p.Conversation.Policy == nil || p.Conversation.Policy.ScopeID.String() != s.Brief.ScopeID) {
		p.Artifact, p.ObservedArtifact, p.Observations = nil, nil, nil
		p.Conversation.Policy, p.Conversation.ObservedPolicy, p.Conversation.Pending = nil, nil, nil
		p.Conversation.Confirmed = nil
		p.Conversation.Sources = []SourceBlock{p.Conversation.CurrentRequest}
	}
	// Existing policy is carried separately, never guessed from old messages.
	refs := map[string]bool{}
	for _, f := range s.Brief.Facts {
		if f.Status == "stated" || f.Status == "accepted" {
			for _, ref := range f.Sources {
				refs[ref.MessageID] = true
			}
		}
	}
	for _, a := range s.Answers {
		if !a.Obsolete && a.Question.ScopeID == s.Brief.ScopeID {
			refs[a.Source.MessageID] = true
		}
	}
	for _, m := range v.Document.Messages {
		if refs[m.ID.String()] && m.Role == "user" && m.Origin != "playground" {
			p.Conversation.MemorySources = append(p.Conversation.MemorySources, originalBlock(m.ID, m.Content))
		}
	}
	return nil
}

func proposeConversationState(p Plan, route reliableRoute, o Operation) (*ConversationState, error) {
	if !p.stateful() {
		return nil, nil
	}
	if p.Conversation == nil || p.Conversation.State == nil || route.Memory == nil {
		return nil, fmt.Errorf("return the conversation memory contract")
	}
	u := route.Memory
	if len(u.Facts) > 8 || len(u.Suggestions) > 4 {
		return nil, fmt.Errorf("too many memory excerpts")
	}
	if route.Intent != "chat" && route.Intent != "clarify" && route.Intent != "explain_results" && (len(u.Suggestions) > 0 || u.GuidanceKind != "") {
		return nil, fmt.Errorf("mutation acknowledgements do not display guidance or examples")
	}
	s := cloneState(p.Conversation.State)
	current := Message{ID: p.sourceMessageID(), Role: "user", Content: p.Submission.Content}
	replyID := deterministicID(o.ID, "completion-message").String()
	s.ThroughMessageID = replyID
	exact := func(q string) bool {
		return strings.TrimSpace(q) != "" && len(q) <= 2000 && strings.Contains(current.Content, q)
	}
	if route.NewAgent {
		if !exact(u.NewScopeQuote) {
			return nil, fmt.Errorf("a new agent scope needs an explicit current quote")
		}
		s = newConversationState(deterministicID(o.ID, "scope"))
		s.ThroughMessageID = replyID
		// Preferences are conversation-wide; questions and rules are scoped.
		s.Guidance = p.Conversation.State.Guidance
		s.Answers = append([]QuestionAnswer(nil), p.Conversation.State.Answers...)
	} else if u.NewScopeQuote != "" {
		return nil, fmt.Errorf("new scope quote requires a new agent route")
	}
	if u.Answer != nil {
		if p.precise() {
			a := u.Answer
			quote := a.Quote
			if !exact(quote) {
				return nil, fmt.Errorf("answer requires an exact current excerpt")
			}
			action := interaction.Action{IdempotencyKey: current.ID.String(), ScopeID: s.Brief.ScopeID, SessionRevision: p.Submission.Revision, Kind: "answer_question", TargetID: a.QuestionID, TargetRevision: a.QuestionRevision, OptionIDs: a.OptionIDs}
			if len(action.OptionIDs) == 0 {
				action.OptionIDs = []string{}
				action.Text = &quote
			}
			if err := answerBoundQuestion(s, action, current, quote, a.Unknown); err != nil {
				return nil, err
			}
		} else {
			a, q := u.Answer, s.PendingQuestion
			if q == nil || q.Status != "active" || q.ScopeID != s.Brief.ScopeID || q.ID != a.QuestionID || q.Revision != a.QuestionRevision || !exact(a.Quote) {
				return nil, fmt.Errorf("answer must reference the active question and exact current message")
			}
			// Proposal/source consent has its own versioned action boundary. A setup
			// interpretation cannot bypass that boundary or execute Run/Keep.
			if q.Purpose != "clarify_job" && q.Purpose != "clarify_rule" {
				return nil, fmt.Errorf("this question needs its explicit confirmation action")
			}
			if len(a.OptionIDs) > q.MaxSelections || a.Unknown && len(a.OptionIDs) > 0 {
				return nil, fmt.Errorf("invalid question selections")
			}
			seen := map[string]bool{}
			for _, id := range a.OptionIDs {
				found := false
				for _, option := range q.Options {
					if id == option.ID {
						found = true
					}
				}
				if !found || seen[id] {
					return nil, fmt.Errorf("unknown or duplicate option")
				}
				seen[id] = true
			}
			for _, option := range q.Options {
				if strings.EqualFold(strings.TrimSpace(a.Quote), option.Label) && len(a.OptionIDs) > 0 && (len(a.OptionIDs) != 1 || a.OptionIDs[0] != option.ID) {
					return nil, fmt.Errorf("selected option contradicts the literal answer")
				}
			}
			q.Status = "answered"
			s.Answers = append(s.Answers, QuestionAnswer{Question: *q, Source: stateSource(current, a.Quote), OptionIDs: a.OptionIDs, Unknown: a.Unknown})
		}
	}
	if u.CancelQuestionQuote != "" {
		if !exact(u.CancelQuestionQuote) || u.Answer != nil {
			return nil, fmt.Errorf("question dismissal needs an explicit current request")
		}
		if s.PendingQuestion != nil && s.PendingQuestion.Status == "active" {
			s.PendingQuestion.Status = "dismissed"
		}
	}
	for _, f := range u.Facts {
		if !exact(f.Quote) || (f.Kind != "job" && f.Kind != "rule" && f.Kind != "has_agent" && f.Kind != "has_pack") {
			return nil, fmt.Errorf("fact needs a typed exact current-user excerpt")
		}
		if route.Intent == "chat" && (f.Kind == "job" || f.Kind == "rule") {
			return nil, fmt.Errorf("casual chat cannot introduce a job or policy")
		}
		if u.Answer != nil && u.Answer.Unknown && f.Quote == u.Answer.Quote {
			return nil, fmt.Errorf("an unknown answer is not a stated fact")
		}
		if err := mergeMemoryFact(s, f, stateSource(current, f.Quote), "stated", o.ID); err != nil {
			return nil, err
		}
	}
	for _, f := range u.Suggestions {
		if f.SupersedesID != "" || (f.Kind != "job" && f.Kind != "rule") || strings.TrimSpace(f.Quote) == "" || len(f.Quote) > 2000 || !strings.Contains(route.Reply, f.Quote) {
			return nil, fmt.Errorf("suggestions must quote displayed examples and cannot replace facts")
		}
		m := Message{ID: deterministicID(o.ID, "completion-message"), Content: route.Reply}
		if err := mergeMemoryFact(s, f, stateSource(m, f.Quote), "proposed", o.ID); err != nil {
			return nil, err
		}
	}
	if u.Question != nil {
		q := u.Question
		if route.Intent != "clarify" || (q.Purpose != "clarify_job" && q.Purpose != "clarify_rule" && q.Purpose != "offer_help") || !strings.Contains(route.Reply, q.Text) {
			return nil, fmt.Errorf("a question must appear in the actual clarification reply")
		}
		next := &interaction.Question{ID: deterministicID(o.ID, "question").String(), ScopeID: s.Brief.ScopeID, Revision: 1, OriginMessageID: replyID, Purpose: q.Purpose, Status: "active", Text: q.Text, MaxSelections: q.MaxSelections, Options: []interaction.Option{}}
		for i, label := range q.Options {
			if !strings.Contains(route.Reply, label) {
				return nil, fmt.Errorf("question options must be displayed")
			}
			next.Options = append(next.Options, interaction.Option{ID: fmt.Sprintf("option-%d", i+1), Label: label})
		}
		if checkWire("question", next) != nil {
			return nil, fmt.Errorf("invalid question shape")
		}
		if old := s.PendingQuestion; old != nil && old.Status == "answered" && old.Text == next.Text {
			return nil, fmt.Errorf("do not ask the same answered question again")
		}
		if old := s.PendingQuestion; old != nil && old.Status == "active" && old.Text == next.Text && Hash(raw(old.Options)) == Hash(raw(next.Options)) {
			next = old
		}
		s.PendingQuestion = next
	} else if route.Intent == "clarify" {
		return nil, fmt.Errorf("a clarification needs one structured displayed question")
	}
	if u.BrevityQuote != "" {
		if !exact(u.BrevityQuote) {
			return nil, fmt.Errorf("brevity needs an explicit preference quote")
		}
		ref := stateSource(current, u.BrevityQuote)
		s.Guidance.Brevity = &ref
	}
	if u.GuidanceKind != "" {
		if u.GuidanceKind != "explanation" && u.GuidanceKind != "example" && u.GuidanceKind != "dismissed" || u.GuidanceTopic == "" || len(u.GuidanceTopic) > 80 {
			return nil, fmt.Errorf("invalid guidance history event")
		}
		if u.GuidanceKind == "dismissed" && u.CancelQuestionQuote == "" {
			return nil, fmt.Errorf("help dismissal needs a user request")
		}
		id := replyID
		if u.GuidanceKind == "dismissed" {
			id = current.ID.String()
		}
		s.Guidance.Events = append(s.Guidance.Events, GuidanceEvent{MessageID: id, Kind: u.GuidanceKind, Topic: u.GuidanceTopic})
		if len(s.Guidance.Events) > 32 {
			s.Guidance.Events = s.Guidance.Events[len(s.Guidance.Events)-32:]
		}
	}
	if !route.NewAgent && Hash(raw(s.Brief.Facts)) != Hash(raw(p.Conversation.State.Brief.Facts)) {
		s.Brief.Revision++
	}
	if p.precise() {
		s.ActionsVersion = 1
		if len(u.Suggestions) > 0 {
			id := deterministicID(o.ID, "proposal").String()
			rev := int64(1)
			proposal := &RuleProposal{ID: id, Revision: rev, ScopeID: s.Brief.ScopeID, Status: "proposed", Question: interaction.Question{ID: deterministicID(o.ID, "proposal-question").String(), ScopeID: s.Brief.ScopeID, Revision: 1, OriginMessageID: replyID, Purpose: "adopt_proposal", Status: "active", Text: route.Reply, Options: []interaction.Option{}, MaxSelections: 1, ProposalID: &id, ProposalRevision: &rev}}
			for _, fact := range s.Brief.Facts {
				if fact.Status == "proposed" && len(fact.Sources) == 1 && fact.Sources[0].MessageID == replyID {
					proposal.Facts = append(proposal.Facts, fact)
				}
			}
			if checkWire("question", proposal.Question) != nil {
				return nil, fmt.Errorf("keep a displayed proposal below 1600 characters")
			}
			s.Proposal = proposal
		}
	}
	if checkWire("brief", s.Brief) != nil {
		return nil, fmt.Errorf("working brief exceeds its schema bounds")
	}
	return s, nil
}

func mergeMemoryFact(s *ConversationState, f memoryFact, source interaction.Source, status string, operation uuid.UUID) error {
	var supersedes *string
	for i := range s.Brief.Facts {
		old := &s.Brief.Facts[i]
		if old.Status == status && old.Kind == f.Kind && old.Text != nil && *old.Text == f.Quote {
			return nil
		}
		if f.SupersedesID != "" && old.ID == f.SupersedesID {
			if old.Kind != f.Kind || old.Status != "stated" {
				return fmt.Errorf("only a matching active stated fact can be corrected here")
			}
			id := old.ID
			supersedes = &id
			old.Status = "superseded"
		}
		if status == "stated" && f.Kind != "rule" && old.Kind == f.Kind && (old.Status == "stated" || old.Status == "accepted") && f.SupersedesID != old.ID {
			return fmt.Errorf("correct an existing job or ownership fact by its stable ID")
		}
	}
	if f.SupersedesID != "" && supersedes == nil {
		return fmt.Errorf("missing fact to supersede")
	}
	if status == "stated" {
		facts := s.Brief.Facts[:0]
		for _, old := range s.Brief.Facts {
			if old.Kind != f.Kind || old.Status != "unknown" {
				facts = append(facts, old)
			}
		}
		s.Brief.Facts = facts
	}
	text := f.Quote
	s.Brief.Facts = append(s.Brief.Facts, interaction.Fact{ID: deterministicID(operation, "fact/"+status+"/"+f.Kind+"/"+f.Quote).String(), Kind: f.Kind, Status: status, Text: &text, Sources: []interaction.Source{source}, SupersedesID: supersedes})
	return nil
}
