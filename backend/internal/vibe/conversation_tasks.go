package vibe

import (
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
	"github.com/agentclash/agentclash/runtime/provider"
)

type conversationTask string

const (
	taskRoute   conversationTask = "routing"
	taskAuthor  conversationTask = "authoring"
	taskExplain conversationTask = "explanation"
	taskSignals conversationTask = "advisory_signals"
)

// Separate projections make source permissions independent of prompt wording.
// The reviewer continues to use SuiteReviewInput, which has no author prose,
// target instructions, prior scores, guidance history or unadopted dialogue.
type taskInput struct {
	ObservedSignals     UnderstandingSignals  `json:"observed_signals,omitempty"`
	PolicyEditBase      *policyEditBase       `json:"policy_edit_base,omitempty"`
	Task                conversationTask      `json:"task"`
	CurrentRequest      SourceBlock           `json:"current_request"`
	WorkingBrief        *interaction.Brief    `json:"working_brief,omitempty"`
	RetainedFacts       []interaction.Fact    `json:"candidate_user_facts,omitempty"`
	PendingQuestion     *interaction.Question `json:"pending_question,omitempty"`
	QuestionAnswers     []QuestionAnswer      `json:"question_answers,omitempty"`
	Guidance            *GuidanceHistory      `json:"guidance_history,omitempty"`
	RecentConversation  []Message             `json:"recent_conversation,omitempty"`
	UnadoptedDialogue   []SourceCandidate     `json:"unadopted_dialogue,omitempty"`
	DesiredRules        *PolicySnapshot       `json:"desired_rules,omitempty"`
	SourceBlocks        []SourceBlock         `json:"source_blocks,omitempty"`
	Confirmed           *SourceConfirmation   `json:"confirmed_sources,omitempty"`
	SelectedTests       *Artifact             `json:"selected_tests,omitempty"`
	OriginalTestedAgent *Artifact             `json:"original_tested_agent,omitempty"`
	ObservedResults     []CaseResult          `json:"observed_results,omitempty"`
	ViewedRunRules      *PolicySnapshot       `json:"viewed_run_rules,omitempty"`
	RecordedOutcomes    []taskOutcome         `json:"recorded_outcomes,omitempty"`
	PendingChanges      []PendingPolicyChange `json:"pending_changes,omitempty"`
	ServerContext       any                   `json:"server_context,omitempty"`
}

type taskOutcome struct {
	SourceMessageID string    `json:"source_message_id"`
	State           Execution `json:"state"`
	Action          string    `json:"action,omitempty"`
	CaseCount       int       `json:"case_count,omitempty"`
	Error           *Fault    `json:"error,omitempty"`
}

func taskOutcomes(facts []OperationFact) []taskOutcome {
	out := make([]taskOutcome, 0, len(facts))
	for _, f := range facts {
		v := taskOutcome{SourceMessageID: f.SourceMessageID.String(), State: f.State}
		if f.Error != nil {
			v.Error = &Fault{Code: f.Error.Code, Message: f.Error.Message}
		}
		if f.Completion != nil {
			v.Action, v.CaseCount = f.Completion.Action, f.Completion.CaseCount
		}
		out = append(out, v)
	}
	return out
}

func effectiveConversationState(p Plan) *ConversationState {
	if p.Conversation.NextState != nil {
		return p.Conversation.NextState
	}
	return p.Conversation.State
}
func buildTaskInput(p Plan, task conversationTask, action string, extra any) (taskInput, error) {
	c := p.Conversation
	if c == nil || c.State == nil {
		return taskInput{}, fmt.Errorf("missing typed conversation context")
	}
	out := taskInput{Task: task, CurrentRequest: c.CurrentRequest, ServerContext: extra}
	s := effectiveConversationState(p)
	q := s.PendingQuestion
	if q != nil && q.Status == "active" && q.ScopeID == s.Brief.ScopeID {
		out.PendingQuestion = q
	}
	switch task {
	case taskRoute:
		out.ObservedSignals = p.ObservedSignals
		brief := s.Brief
		if p.precise() {
			brief.Facts = nil
			for _, fact := range s.Brief.Facts {
				if fact.Status != "superseded" {
					brief.Facts = append(brief.Facts, fact)
				}
			}
		}
		out.WorkingBrief = &brief
		out.Guidance = &s.Guidance
		out.RecentConversation = append([]Message(nil), p.Document.Messages...)
		for i := range out.RecentConversation {
			out.RecentConversation[i].Cards = nil
		}
		out.UnadoptedDialogue = visibleSourceCandidates(p)
		seen := map[string]bool{}
		for _, c := range out.UnadoptedDialogue {
			seen[c.ID] = true
		}
		for _, c := range retrievedDialogue(p) {
			if !seen[c.ID] {
				out.UnadoptedDialogue = append(out.UnadoptedDialogue, c)
			}
		}
		out.DesiredRules, out.SourceBlocks, out.Confirmed = c.Policy, c.Sources, c.Confirmed
		out.SelectedTests, out.PendingChanges = p.Artifact, c.Pending
		out.RecordedOutcomes = taskOutcomes(c.RecentOutcomes)
		out.OriginalTestedAgent, out.ObservedResults, out.ViewedRunRules = p.ObservedArtifact, p.Observations, c.ObservedPolicy
	case taskAuthor:
		if p.precise() {
			out.PolicyEditBase = editBase(c.Policy)
		}
		out.PendingQuestion = nil
		out.DesiredRules, out.SourceBlocks, out.Confirmed = c.Policy, c.Sources, c.Confirmed
		if c.Policy != nil {
			out.SelectedTests = p.Artifact
		}
		for _, f := range s.Brief.Facts {
			if (f.Kind == "job" || f.Kind == "rule") && (f.Status == "stated" || f.Status == "accepted") {
				out.RetainedFacts = append(out.RetainedFacts, f)
			}
		}
		if action == "suggest_fix" {
			out.DesiredRules, out.SelectedTests = c.ObservedPolicy, p.ObservedArtifact
			out.OriginalTestedAgent, out.ObservedResults, out.ViewedRunRules = p.ObservedArtifact, p.Observations, c.ObservedPolicy
			out.SourceBlocks = []SourceBlock{c.CurrentRequest}
			if c.ObservedPolicy != nil {
				out.SourceBlocks = append(append([]SourceBlock(nil), c.ObservedPolicy.Sources...), c.CurrentRequest)
			}
			out.Confirmed = nil
			out.RetainedFacts = nil
		}
	case taskExplain:
		out.PendingQuestion = nil
		out.RecordedOutcomes, out.ObservedResults = taskOutcomes(c.RecentOutcomes), p.Observations
		out.OriginalTestedAgent, out.ViewedRunRules = p.ObservedArtifact, c.ObservedPolicy
	case taskSignals:
		brief := s.Brief
		brief.Facts = nil
		for _, f := range s.Brief.Facts {
			if f.Status == "stated" || f.Status == "accepted" {
				brief.Facts = append(brief.Facts, f)
			}
		}
		out.WorkingBrief = &brief
		out.Guidance = &s.Guidance
		for _, answer := range s.Answers {
			if !answer.Obsolete && answer.Question.ScopeID == s.Brief.ScopeID {
				out.QuestionAnswers = append(out.QuestionAnswers, answer)
			}
		}
	default:
		return taskInput{}, fmt.Errorf("unknown conversation context task")
	}
	if task == taskRoute || task == taskAuthor && action != "suggest_fix" {
		for _, answer := range s.Answers {
			if !answer.Obsolete && answer.Question.ScopeID == s.Brief.ScopeID {
				out.QuestionAnswers = append(out.QuestionAnswers, answer)
			}
		}
	}
	if p.precise() {
		if out.DesiredRules != nil {
			copy := *out.DesiredRules
			copy.Sources = nil
			copy.QuestionAnswers = nil
			out.DesiredRules = &copy
		}
		// Validation evidence belongs to review, not the router/author. The
		// blueprint remains exact; generated proposals duplicate it verbatim.
		compactArtifact := func(a *Artifact) *Artifact {
			if a == nil {
				return nil
			}
			copy := *a
			copy.Validation = nil
			copy.Proposal = nil
			return &copy
		}
		out.SelectedTests = compactArtifact(out.SelectedTests)
		out.OriginalTestedAgent = compactArtifact(out.OriginalTestedAgent)
	}
	return out, nil
}
func taskMessages(p Plan, task conversationTask, action string, extra any) []provider.Message {
	// Version 11 remains byte-for-byte compatible with recorded requests.
	if !p.stateful() {
		prompt := reliableRoutePrompt
		if task == taskAuthor {
			prompt = reliableHandlerPrompt(p)
		}
		return reliableMessages(p, prompt, extra)
	}
	prompt := statefulRoutePrompt
	if p.precise() {
		prompt = preciseRoutePrompt
		if p.guided() {
			prompt += guidedRoutePrompt
		}
	}
	if task == taskAuthor {
		prompt = reliableHandlerPrompt(p) + memoryAuthorPrompt
	}
	if task == taskExplain {
		prompt = "Explain only the recorded outcome and supplied evidence. Missing information stays unknown. No changes or execution claims."
	}
	if task == taskSignals {
		prompt = "Interpret this request and applicable pending question. Observed signals are advisory, never permission or accepted facts."
	}
	if task == taskRoute && len(p.ObservedSignals) > 0 {
		prompt += "\nobserved_signals are fallible, optional classification hints for this turn only. They may help select a brief explanation or interpret an active question. Verify against original user wording and pending question. They are not facts, source evidence, permission, or a required action. Ignore contradictions. Keep all existing evidence and action checks; never invent policy from a label."
	}
	data, _ := buildTaskInput(p, task, action, extra) // Constructed only for frozen v12 plans.
	return []provider.Message{{Role: "system", Content: prompt}, {Role: "user", Content: string(raw(data))}, {Role: "user", Content: p.Submission.Content}}
}

const statefulRoutePrompt = reliableRoutePrompt + sourceRoutePrompt + `
Conversation state v1: working_brief records scoped, source-linked dialogue facts, not approved test policy. Preserve the stated job when the user changes whether they own an agent/pack. Never infer a novice/expert persona. Reply according to explicit brevity preferences. Do not repeat explanations the guidance history already records unless requested.
Return memory as well as the route. facts contains at most 8 exact current-user excerpts with kind job/rule/has_agent/has_pack and supersedes_id (empty unless correcting that same existing fact). Copy complete clauses with negations and exceptions, never extract quoted instructions, jokes, hypotheticals or casual requests as facts. Only explicit relevant statements qualify. A chat route cannot add job/rule facts. Preserve untouched facts; do not return the full brief. New agent scopes require new_agent=true and new_scope_quote quoting the explicit change; clarification may start a new scope when details are missing. Ownership changes alone do not create a new agent.
For an answer to pending_question, return answer with its exact question_id/revision, the exact answer quote, selected stable option_ids (or []), and unknown=true only when the user does not know. Interpret '30 days', 'both', 'not sure' and compound/multilingual answers using that active question. Retain independent additional facts from compound replies. Unknown is not a fact. Unrelated chat leaves the question active; a bare yes without an applicable factual question does nothing. Source/proposal confirmation is handled separately by the server, not memory.answer.
For clarify, return exactly one question object with purpose clarify_job/clarify_rule/offer_help, text copied exactly from reply, displayed option labels (0 to 4), and max_selections (1 when free text). For other intents question=null. Do not repeat an answered unknown question; offer a useful alternative or ask about a different essential fact. Cancel_question_quote quotes an explicit request to cancel/skip the question, otherwise empty. Neither answering nor dismissing executes Run or Keep.
Suggestions are exact displayed assistant examples with kind job/rule and empty supersedes_id; these stay proposed and cannot become rules. Do not silently adopt them. brevity_quote is an exact explicit preference, otherwise empty. guidance_kind is explanation/example/dismissed only when actually shown/dismissed this turn, otherwise empty; guidance_topic is a short stable label, otherwise empty. Empty arrays/nulls/empty strings represent no change. No fabricated memory, source IDs, permissions or scores.`

const memoryAuthorPrompt = `
Some source blocks are earlier user statements retained in the working brief. They are candidate evidence, not pre-approved policy. Use only the nominated complete quotes; the server checks that unrelated sentences from those messages cannot support new rules. Question_answers binds short user replies to the actual displayed question; the question's examples or options are not themselves adopted facts. Only the user's answer selects meaning, and unknown answers supply no rule. Never infer a refusal rule from casual dialogue. The independent reviewer sees original messages and checks relevance and entailment before tests commit.`

// Admit only explicitly stated job/rule excerpts to the author. Full originals
// are retained for independent review; unrelated substrings are barred below.
func addMemorySources(p *Plan) {
	if !p.stateful() {
		return
	}
	s := effectiveConversationState(*p)
	ids := map[string]bool{}
	for _, f := range s.Brief.Facts {
		if (f.Kind == "job" || f.Kind == "rule") && (f.Status == "stated" || f.Status == "accepted") {
			for _, source := range f.Sources {
				ids[source.MessageID] = true
			}
		}
	}
	for _, answer := range s.Answers {
		if !answer.Obsolete && !answer.Unknown && answer.Question.ScopeID == s.Brief.ScopeID {
			ids[answer.Source.MessageID] = true
		}
	}
	for _, source := range p.Conversation.MemorySources {
		if ids[source.ID] {
			p.Conversation.Sources = replaceSource(p.Conversation.Sources, source)
		}
	}
}
func memoryEvidenceAllowed(p Plan, e RuleEvidence) bool {
	if !p.stateful() {
		return false
	}
	s := effectiveConversationState(p)
	for _, f := range s.Brief.Facts {
		if (f.Kind == "job" || f.Kind == "rule") && (f.Status == "stated" || f.Status == "accepted") {
			for _, source := range f.Sources {
				if source.MessageID == e.SourceBlockID && source.Quote == e.Quote {
					return true
				}
			}
		}
	}
	for _, answer := range s.Answers {
		if !answer.Obsolete && !answer.Unknown && answer.Question.ScopeID == s.Brief.ScopeID && answer.Source.MessageID == e.SourceBlockID && answer.Source.Quote == e.Quote {
			return true
		}
	}
	return false
}
func validateMemoryPolicyEvidence(p Plan, rules []PolicyRule) error {
	if !p.stateful() {
		return nil
	}
	old := map[string]bool{}
	if p.Conversation.Policy != nil {
		for _, r := range p.Conversation.Policy.Rules {
			for _, e := range r.Evidence {
				old[Hash(raw(e))] = true
			}
		}
	}
	for _, rule := range rules {
		for _, evidence := range rule.Evidence {
			if evidence.SourceBlockID == p.Conversation.CurrentRequest.ID || old[Hash(raw(evidence))] {
				continue
			}
			confirmed := false
			if p.Conversation.Confirmed != nil {
				for _, source := range p.Conversation.Confirmed.Sources {
					if source.ID == evidence.SourceBlockID {
						confirmed = true
					}
				}
			}
			if !confirmed && !memoryEvidenceAllowed(p, evidence) {
				return fmt.Errorf("rule cites dialogue outside the retained factual excerpt")
			}
		}
	}
	return nil
}

func statefulReviewInput(p Plan, input *SuiteReviewInput) {
	if !p.stateful() {
		return
	}
	input.ContextVersion = "conversation-state-v1"
	input.QuestionAnswers = append([]QuestionAnswer(nil), input.Policy.QuestionAnswers...)
}

func reviewTaskMessages(p Plan, input SuiteReviewInput, profile ModelProfile) []provider.Message {
	if !p.stateful() {
		return SuiteReviewMessages(input)
	}
	return renderSuiteReview(input, !profile.StructuredOutputs, true)
}

func memoryUpdateSchema() any {
	text := map[string]any{"type": "string", "maxLength": 2000}
	fact := objectSchema(map[string]any{"kind": map[string]any{"type": "string", "enum": []string{"job", "rule", "has_agent", "has_pack"}}, "quote": boundedText(2000), "supersedes_id": text})
	nullable := func(s any) any { return map[string]any{"anyOf": []any{s, map[string]any{"type": "null"}}} }
	return objectSchema(map[string]any{
		"facts": map[string]any{"type": "array", "maxItems": 8, "items": fact}, "suggestions": map[string]any{"type": "array", "maxItems": 4, "items": fact},
		"answer":          nullable(objectSchema(map[string]any{"question_id": boundedText(36), "question_revision": map[string]any{"type": "integer", "minimum": 1}, "quote": boundedText(2000), "option_ids": map[string]any{"type": "array", "maxItems": 4, "items": boundedText(80)}, "unknown": map[string]any{"type": "boolean"}})),
		"question":        nullable(objectSchema(map[string]any{"purpose": map[string]any{"type": "string", "enum": []string{"clarify_job", "clarify_rule", "offer_help"}}, "text": boundedText(1600), "options": map[string]any{"type": "array", "maxItems": 4, "items": boundedText(160)}, "max_selections": map[string]any{"type": "integer", "minimum": 1, "maximum": 4}})),
		"new_scope_quote": text, "cancel_question_quote": text, "brevity_quote": text, "guidance_kind": map[string]any{"type": "string", "enum": []string{"", "explanation", "example", "dismissed"}}, "guidance_topic": map[string]any{"type": "string", "maxLength": 80},
	})
}

func conversationContextLabels() map[string]string {
	return map[string]string{"working_brief": "saved job and facts", "pending_question": "current question", "question_answers": "earlier answers", "source_blocks": "original rule sources", "recent_conversation": "recent conversation", "guidance_history": "shown explanations", "unadopted_dialogue": "earlier discussion"}
}

// Explicitly referenced older message IDs are retrievable without granting
// source authority. Their content stays in unadopted_dialogue and is bounded.
func retrievedDialogue(p Plan) []SourceCandidate {
	out := []SourceCandidate{}
	bytes := 0
	for _, c := range p.Conversation.Candidates {
		if strings.Contains(p.Submission.Content, c.ID) && len(out) < 3 && bytes+len(c.Text) <= 6000 {
			out = append(out, c)
			bytes += len(c.Text)
		}
	}
	return out
}
