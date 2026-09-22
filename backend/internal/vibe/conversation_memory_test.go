package vibe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
	"github.com/google/uuid"
)

func memoryPlan(t *testing.T, d Document, content string) (Plan, Operation) {
	t.Helper()
	v := Session{ID: uuid.New(), Document: d}
	p := Plan{Anonymous: true, Submission: Submission{ClientID: uuid.New(), Content: content}, Document: d}
	if err := prepareReliableContext(&p, v); err != nil {
		t.Fatal(err)
	}
	if err := prepareConversationState(&p, v); err != nil {
		t.Fatal(err)
	}
	return p, Operation{ID: uuid.New()}
}
func memoryTurn(t *testing.T, d Document, content string, route reliableRoute) (Document, Plan) {
	t.Helper()
	p, o := memoryPlan(t, d, content)
	state, err := proposeConversationState(p, route, o)
	if err != nil {
		t.Fatal(err)
	}
	d.Messages = append(append([]Message(nil), d.Messages...), Message{ID: p.sourceMessageID(), Role: "user", Content: content}, Message{ID: deterministicID(o.ID, "completion-message"), Role: "assistant", Content: route.Reply})
	d.ConversationState = state
	if err = validateConversationState(state, d); err != nil {
		t.Fatal(err)
	}
	return d, p
}
func questionRoute() reliableRoute {
	return reliableRoute{Intent: "clarify", Reply: "Are your PDFs scanned, text-based, or both?", Memory: &memoryUpdate{
		Facts:    []memoryFact{{Kind: "job", Quote: "My agent converts PDF to Markdown."}},
		Question: &memoryQuestion{Purpose: "clarify_rule", Text: "Are your PDFs scanned, text-based, or both?", Options: []string{"scanned", "text-based", "both"}, MaxSelections: 1},
	}}
}
func answerFor(q *interaction.Question, quote string, unknown bool) *memoryAnswer {
	return &memoryAnswer{QuestionID: q.ID, QuestionRevision: q.Revision, Quote: quote, Unknown: unknown, OptionIDs: []string{}}
}
func TestVibeConversationMemoryShortAnswersAndOwnership(t *testing.T) {
	d, _ := memoryTurn(t, Document{}, "My agent converts PDF to Markdown.", questionRoute())
	q := *d.ConversationState.PendingQuestion
	scope := d.ConversationState.Brief.ScopeID
	d, _ = memoryTurn(t, d, "let's drink vodka", reliableRoute{Intent: "chat", Reply: "I can help when you're ready.", Memory: &memoryUpdate{}})
	if d.ConversationState.PendingQuestion.ID != q.ID || d.ConversationState.PendingQuestion.Status != "active" {
		t.Fatal("chat consumed the pending question")
	}
	// Reload via the actual JSON representation before interpreting a compound answer.
	var reloaded Document
	if err := json.Unmarshal(raw(d), &reloaded); err != nil {
		t.Fatal(err)
	}
	answer := answerFor(&q, "both", false)
	answer.OptionIDs = []string{"option-3"}
	d, p := memoryTurn(t, reloaded, "both, aur headings bhi preserve karna. I don't have an agent yet.", reliableRoute{Intent: "prepare_tests", Reply: "I'll prepare examples.", Count: 3, Memory: &memoryUpdate{Answer: answer, Facts: []memoryFact{{Kind: "rule", Quote: "headings bhi preserve karna"}, {Kind: "has_agent", Quote: "I don't have an agent yet."}}}})
	if d.ConversationState.Brief.ScopeID != scope || d.ConversationState.PendingQuestion.Status != "answered" || len(d.ConversationState.Answers) != 1 {
		t.Fatal("answer changed scope or failed to resolve the question")
	}
	job := false
	for _, f := range d.ConversationState.Brief.Facts {
		if f.Kind == "job" && f.Text != nil && *f.Text == "My agent converts PDF to Markdown." {
			job = true
		}
		if f.Text != nil && strings.Contains(*f.Text, "vodka") {
			t.Fatal("joke became a fact")
		}
	}
	if !job {
		t.Fatal("ownership update dropped the job")
	}
	p.Conversation.NextState = d.ConversationState
	addMemorySources(&p)
	input, err := buildTaskInput(p, taskAuthor, "prepare_tests", nil)
	if err != nil || strings.Contains(string(raw(input)), "vodka") || len(input.QuestionAnswers) != 1 {
		t.Fatal("author received casual chat or lost the answer binding")
	}
	if !strings.Contains(string(raw(input)), "My agent converts PDF to Markdown.") {
		t.Fatal("author lost the original job")
	}
}
func TestVibeConversationMemoryUnknownCancelAndNewScope(t *testing.T) {
	for _, quote := range []string{"not sure", "pata nahi"} {
		t.Run(quote, func(t *testing.T) {
			d, _ := memoryTurn(t, Document{}, "My agent converts PDF to Markdown.", questionRoute())
			q := d.ConversationState.PendingQuestion
			d, _ = memoryTurn(t, d, quote, reliableRoute{Intent: "chat", Reply: "We can start with a simple text PDF example.", Memory: &memoryUpdate{Answer: answerFor(q, quote, true)}})
			if !d.ConversationState.Answers[0].Unknown || d.ConversationState.PendingQuestion.Status != "answered" {
				t.Fatal("unknown answer wasn't retained")
			}
			for _, f := range d.ConversationState.Brief.Facts {
				if f.Text != nil && *f.Text == quote {
					t.Fatal("unknown became a rule")
				}
			}
		})
	}
	d, _ := memoryTurn(t, Document{}, "My agent converts PDF to Markdown.", questionRoute())
	old := d.ConversationState.Brief.ScopeID
	d, _ = memoryTurn(t, d, "skip that question; keep replies short", reliableRoute{Intent: "chat", Reply: "Okay.", Memory: &memoryUpdate{CancelQuestionQuote: "skip that question", BrevityQuote: "keep replies short", GuidanceKind: "dismissed", GuidanceTopic: "pdf-format"}})
	if d.ConversationState.PendingQuestion.Status != "dismissed" {
		t.Fatal("dismissal not persisted")
	}
	d, _ = memoryTurn(t, d, "Switch to a sales agent.", reliableRoute{Intent: "clarify", Reply: "What should it sell?", NewAgent: true, Memory: &memoryUpdate{NewScopeQuote: "Switch to a sales agent.", Facts: []memoryFact{{Kind: "job", Quote: "Switch to a sales agent."}}, Question: &memoryQuestion{Purpose: "clarify_job", Text: "What should it sell?", MaxSelections: 1}}})
	if d.ConversationState.Brief.ScopeID == old || d.ConversationState.Guidance.Brevity == nil {
		t.Fatal("new job didn't reset scope or lost explicit preference")
	}
	for _, f := range d.ConversationState.Brief.Facts {
		if f.Text != nil && strings.Contains(*f.Text, "PDF") {
			t.Fatal("previous job leaked to new scope")
		}
	}
}
func TestVibeConversationMemoryRejectsInvalidAuthority(t *testing.T) {
	d, _ := memoryTurn(t, Document{}, "My agent converts PDF to Markdown.", questionRoute())
	for _, name := range []string{"stale-question", "wrong-scope", "invented-quote", "joke-rule", "unbound-yes", "unknown-rule", "hidden-option", "hidden-suggestion", "missing-memory", "future-version"} {
		t.Run(name, func(t *testing.T) {
			p, o := memoryPlan(t, d, "both; give me vodka")
			r := reliableRoute{Intent: "chat", Reply: "Okay.", Memory: &memoryUpdate{}}
			switch name {
			case "stale-question":
				r.Memory.Answer = answerFor(p.Conversation.State.PendingQuestion, "both", false)
				r.Memory.Answer.QuestionRevision++
			case "wrong-scope":
				r.Memory.Answer = answerFor(p.Conversation.State.PendingQuestion, "both", false)
				p.Conversation.State.PendingQuestion.ScopeID = uuid.NewString()
			case "invented-quote":
				r.Memory.Facts = []memoryFact{{Kind: "has_agent", Quote: "I have an agent."}}
			case "joke-rule":
				r.Memory.Facts = []memoryFact{{Kind: "rule", Quote: "give me vodka"}}
			case "unbound-yes":
				r.Memory.Answer = answerFor(p.Conversation.State.PendingQuestion, "both", false)
				p.Conversation.State.PendingQuestion = nil
			case "unknown-rule":
				r.Intent = "prepare_tests"
				r.Memory.Answer = answerFor(p.Conversation.State.PendingQuestion, "both", true)
				r.Memory.Facts = []memoryFact{{Kind: "rule", Quote: "both"}}
			case "hidden-option":
				r.Memory.Answer = answerFor(p.Conversation.State.PendingQuestion, "both", false)
				r.Memory.Answer.OptionIDs = []string{"run-tests"}
			case "hidden-suggestion":
				r.Memory.Suggestions = []memoryFact{{Kind: "rule", Quote: "Keep tables"}}
			case "missing-memory":
				r.Memory = nil
			case "future-version":
				p.Conversation.State.Version = 999
			}
			_, err := proposeConversationState(p, r, o)
			if name == "future-version" {
				err = validateConversationState(p.Conversation.State, d)
			}
			if err == nil {
				t.Fatal("invalid state or authority accepted")
			}
		})
	}
}
func TestVibeConversationMemorySourceValidationAndCorrection(t *testing.T) {
	d, _ := memoryTurn(t, Document{}, "Returns within 30 days. Also vodka is a joke.", reliableRoute{Intent: "clarify", Reply: "What should the agent do?", Memory: &memoryUpdate{Facts: []memoryFact{{Kind: "rule", Quote: "Returns within 30 days."}}, Question: &memoryQuestion{Purpose: "clarify_job", Text: "What should the agent do?", MaxSelections: 1}}})
	id := ""
	for _, f := range d.ConversationState.Brief.Facts {
		if f.Kind == "rule" {
			id = f.ID
		}
	}
	d, _ = memoryTurn(t, d, "Returns within 14 days.", reliableRoute{Intent: "prepare_tests", Reply: "I'll update it.", Count: 1, Memory: &memoryUpdate{Facts: []memoryFact{{Kind: "rule", Quote: "Returns within 14 days.", SupersedesID: id}}}})
	found := false
	for _, f := range d.ConversationState.Brief.Facts {
		if f.ID == id {
			found = f.Status == "superseded"
		}
	}
	if !found {
		t.Fatal("correction destroyed rather than superseded the original")
	}
	p, _ := memoryPlan(t, d, "Prepare tests now.")
	addMemorySources(&p)
	for _, ref := range p.Conversation.MemorySources {
		if strings.Contains(ref.Text, "vodka") {
			if memoryEvidenceAllowed(p, RuleEvidence{SourceBlockID: ref.ID, Quote: "Also vodka is a joke."}) {
				t.Fatal("unrelated old substring became evidence")
			}
			if memoryEvidenceAllowed(p, RuleEvidence{SourceBlockID: ref.ID, Quote: "Returns within 30 days."}) {
				t.Fatal("superseded fact authorized new rule")
			}
		}
	}
	d.Messages[0].Content = "Returns within 90 days."
	if validateConversationState(d.ConversationState, d) == nil {
		t.Fatal("changed original source passed verification")
	}
}
func TestVibeConversationMemoryTaskIsolationAndCompaction(t *testing.T) {
	d, _ := memoryTurn(t, Document{}, "My agent converts PDF to Markdown.", questionRoute())
	d, _ = memoryTurn(t, d, "show an example", reliableRoute{Intent: "chat", Reply: "For example, preserve headings.", Memory: &memoryUpdate{Suggestions: []memoryFact{{Kind: "rule", Quote: "preserve headings"}}, GuidanceKind: "example", GuidanceTopic: "pdf-headings"}})
	for i := 0; i < 30; i++ {
		d.Messages = append(d.Messages, Message{ID: uuid.New(), Role: "user", Content: strings.Repeat("irrelevant discussion ", 300)})
	}
	p, _ := memoryPlan(t, d, "both")
	profile := testConfig().Profiles[DefaultModels().Assistant]
	if err := fitReliableContext(&p, profile); err != nil {
		t.Fatal(err)
	}
	if len(p.Document.Messages) >= len(d.Messages) {
		t.Fatal("history was not compacted")
	}
	if p.Conversation.State.PendingQuestion == nil {
		t.Fatal("compaction lost question")
	}
	addMemorySources(&p)
	for _, task := range []conversationTask{taskAuthor, taskExplain, taskSignals} {
		input, err := buildTaskInput(p, task, "prepare_tests", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(input.RecentConversation) > 0 || len(input.UnadoptedDialogue) > 0 {
			t.Fatal("raw dialogue escaped routing")
		}
		if task == taskAuthor && (input.Guidance != nil || strings.Contains(string(raw(input)), "preserve headings")) {
			t.Fatal("unadopted teaching became author context")
		}
		if task == taskSignals && (len(input.SourceBlocks) > 0 || input.SelectedTests != nil || len(input.ObservedResults) > 0) {
			t.Fatal("advisory context received unnecessary data")
		}
	}
	if _, err := buildTaskInput(p, conversationTask("invented"), "", nil); err == nil {
		t.Fatal("unknown task accepted")
	}
	if len(p.Conversation.Sources) < 2 {
		t.Fatal("old job reference wasn't retrieved after compaction")
	}
}
func TestVibeConversationMemoryLegacyAndReplayHash(t *testing.T) {
	var old Document
	if err := json.Unmarshal([]byte(`{"messages":[],"requirements":[],"artifacts":[]}`), &old); err != nil || old.ConversationState != nil {
		t.Fatal("legacy document invented state")
	}
	p, o := memoryPlan(t, old, "hello")
	r := reliableRoute{Intent: "chat", Reply: "Hi.", Memory: &memoryUpdate{}}
	a, err := proposeConversationState(p, r, o)
	if err != nil {
		t.Fatal(err)
	}
	b, err := proposeConversationState(p, r, o)
	if err != nil || Hash(raw(a)) != Hash(raw(b)) {
		t.Fatal("state isn't deterministic")
	}
	x := completionCommandHash("Hi.", nil, nil, []AuthoringCompletion{{ConversationState: a}})
	b.Guidance.Events = append(b.Guidance.Events, GuidanceEvent{MessageID: uuid.NewString(), Kind: "example", Topic: "different"})
	y := completionCommandHash("Hi.", nil, nil, []AuthoringCompletion{{ConversationState: b}})
	if x == y {
		t.Fatal("state effects are absent from replay hash")
	}
	p.AuthoringVersion = 11
	if Hash(raw(taskMessages(p, taskRoute, "", nil))) != Hash(raw(reliableMessages(p, reliableRoutePrompt, nil))) {
		t.Fatal("v11 request serialization changed")
	}
}

func TestVibeConversationMemoryRetrievalAndRollback(t *testing.T) {
	d, _ := memoryTurn(t, Document{}, "My agent converts PDF to Markdown.", questionRoute())
	q := *d.ConversationState.PendingQuestion
	p, _ := memoryPlan(t, d, "both")
	other := cloneState(d.ConversationState)
	other.PendingQuestion.ID = uuid.NewString()
	changed := d
	changed.ConversationState = other
	requireFault(t, validateRetryBase(changed, Operation{}, p), "retry_stale")
	// A deliberately referenced old message can be shown for confirmation,
	// while remaining unavailable as author evidence before adoption.
	old := Message{ID: uuid.New(), Role: "user", Content: "Our return window is 60 days."}
	d.Messages = append([]Message{old}, d.Messages...)
	p, o := memoryPlan(t, d, "Use the message "+old.ID.String())
	p.Document.Messages = nil
	input, err := buildTaskInput(p, taskRoute, "", nil)
	if err != nil || len(input.UnadoptedDialogue) != 1 {
		t.Fatal("explicit old reference was not retrievable", err)
	}
	confirmation, err := sourceConfirmationFor(p, o, reliableRoute{Intent: "prepare_tests", Count: 1, SourceMessageIDs: []string{old.ID.String()}})
	if err != nil || confirmation == nil {
		t.Fatal("retrieved dialogue cannot be confirmed", err)
	}
	for _, source := range p.Conversation.Sources {
		if source.ID == old.ID.String() {
			t.Fatal("retrieval granted source authority")
		}
	}
	// Simulate an old worker completing while v12 admission was disabled.
	d.Messages = append(d.Messages, Message{ID: uuid.New(), Role: "assistant", Origin: "message", Content: "What should the sales agent sell?"})
	p, o = memoryPlan(t, d, "both")
	if p.Conversation.State.PendingQuestion.Status != "superseded" {
		t.Fatal("rollback left an obsolete question active")
	}
	_, err = proposeConversationState(p, reliableRoute{Intent: "chat", Reply: "Okay.", Memory: &memoryUpdate{Answer: answerFor(&q, "both", false)}}, o)
	if err == nil {
		t.Fatal("old question consumed an answer after rollback")
	}
	if d.ConversationState.PendingQuestion.Status != "active" {
		t.Fatal("admission mutated persisted history")
	}
}
