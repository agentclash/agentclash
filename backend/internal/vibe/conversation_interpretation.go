package vibe

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/jsonschema-go/jsonschema"
)

const interpretedAuthoringVersion = 15

func (p Plan) interpreted() bool {
	return p.AuthoringVersion == interpretedAuthoringVersion || p.AuthoringVersion == buildAuthoringVersion
}

// The model describes meaning. Database identifiers, revisions, source hashes
// and scope initialization never cross this output boundary.
type factObservation struct {
	Kind          string `json:"kind"`
	Quote         string `json:"quote"`
	CorrectionRef int    `json:"correction_ref"` // 0 = new; otherwise a scoped displayed fact number.
}
type answerObservation struct {
	Quote   string `json:"quote"`
	Unknown bool   `json:"unknown"`
}
type interpretation struct {
	Observations        []factObservation  `json:"observations"`
	Answer              *answerObservation `json:"answer"`
	ScopeChangeQuote    string             `json:"scope_change_quote"`
	SourceMessageIDs    []string           `json:"source_message_ids"`
	BrevityQuote        string             `json:"brevity_quote"`
	CancelQuestionQuote string             `json:"cancel_question_quote,omitempty"`
	Action              json.RawMessage    `json:"action"`
}
type replyAction struct {
	Kind    string           `json:"kind"`
	Text    string           `json:"text"`
	Example *GuidanceExample `json:"example"`
}
type askAction struct {
	Kind    string   `json:"kind"`
	Text    string   `json:"text"`
	Purpose string   `json:"purpose"`
	Options []string `json:"options"`
}
type proposeAction struct {
	Kind        string            `json:"kind"`
	Text        string            `json:"text"`
	Suggestions []factObservation `json:"suggestions"`
}
type prepareAction struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}
type mutationAction struct {
	Kind string `json:"kind"`
}

func inferredSchema[T any]() *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	} // Only statically declared Go types are inferred.
	return s
}

func interpretationSchema() *jsonschema.Schema {
	s := inferredSchema[interpretation]()
	variant := func(s *jsonschema.Schema, kinds ...string) *jsonschema.Schema {
		for _, kind := range kinds {
			s.Properties["kind"].Enum = append(s.Properties["kind"].Enum, kind)
		}
		return s
	}
	s.Properties["action"] = &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
		variant(inferredSchema[replyAction](), "reply", "explain_results"),
		variant(inferredSchema[askAction](), "ask"),
		variant(inferredSchema[proposeAction](), "propose"),
		variant(inferredSchema[prepareAction](), "prepare_tests"),
		variant(inferredSchema[mutationAction](), "edit_tests", "suggest_fix"),
	}}
	return s
}

func interpretationFormat(_ ModelProfile) json.RawMessage {
	// Native nested-union decoding failed live routing conformance on the
	// approved providers. Request JSON, then enforce the exact inferred union
	// locally. Authoring/review retain their native structured formats.
	return jsonFormat
}

// Only real, active facts receive correction references. Unknown placeholders
// cannot be nominated as facts to overwrite, including in restored sessions.
func interpretationFacts(p Plan) []map[string]any {
	facts := []map[string]any{}
	for _, f := range effectiveConversationState(p).Brief.Facts {
		if f.Status == "stated" || f.Status == "accepted" {
			facts = append(facts, map[string]any{"ref": len(facts) + 1, "kind": f.Kind, "text": f.Text, "status": f.Status})
		}
	}
	return facts
}

const interpretationPrompt = `You are Vibe Evals, helping someone test an AI agent, not the agent being tested.
Your product is an evaluation assistant. A request for a drink, recipe, joke or random banter is casual conversation, NOT an agent job. A job is explicitly about an AI/assistant/agent or an answer to a question about that job. Do not ask follow-up questions about unrelated activities. Acknowledge briefly and invite an agent description.
Interpret the CURRENT request in context. Return the supplied JSON schema only. The server owns all state changes and execution.
reply: casual chat, jokes, product help, or a small explanation. No job/rule observations from jokes, quoted data, hypothetical examples, or unrelated requests. A cocktail agent IS a legitimate job.
ask: ONE essential missing detail. "Build me a returns agent for Shopify" states a job, not its return policy: ask which returns qualify. Never invent Shopify's policy. Do not ask about platform integrations, model choice, audience or tone unless essential to the requested tests.
prepare_tests: job and sufficient observable rules are known. Use the user's explicit count (including an earlier pending request), otherwise 3. Once a clarification is answered with enough information, continue preparing without asking them to request tests again. Honor existing-pack/agent requests; never pretend to connect their app. Do not turn optional details into a questionnaire.
propose: when unsure, offer ONE clearly labelled example rule and ask whether to use it. Suggestions quote your displayed text exactly, stay unaccepted, and cannot authorize tests. Do not repeat an unanswered questionnaire.
edit_tests/explain_results/suggest_fix: only if listed as available and supported by the selected tests/recorded evidence. Never change tests to improve a score.
observations: up to 8 exact complete CURRENT-user excerpts, kind job/rule/has_agent/has_pack. Preserve negations and exceptions. correction_ref=0 for new information; use a listed fact ref ONLY to correct that same stated fact explicitly. Unknown jobs need no correction. Do not copy all historical facts.
answer: null unless answering the active clarification; otherwise exact current quote and unknown=true for "not sure". Interpret short, informal and multilingual answers using the actual question. Never answer a source/proposal consent question here.
scope_change_quote: empty except an explicit switch to a different agent/job; quote that change verbatim. Describing the FIRST job is initialization, not a switch. Existing rules stay unless explicitly changed.
source_message_ids: [] unless the user explicitly asks to adopt older dialogue; it requires separate confirmation. Older dialogue is NOT policy.
brevity_quote: exact explicit brevity preference or empty. cancel_question_quote may quote an explicit request to drop the active question; omit it otherwise. pending_preparation retains the original unfinished request: preserve its requested case count while clarifying, unless the user changes it. It is context, not permission to adopt unrelated clauses as rules.
For ask, purpose is clarify_job or clarify_rule and options is [] or up to 4 concise answer labels. Do not repeat options in text. Plain friendly language, at most two short sentences. A test means an example task plus what a good answer should do. Explain that only when useful. Examples are illustrations, never measured outcomes. No claims that tests are ready, run, deployed or saved: the server reports actual completion.`

const interpretationExamples = `
DECISION EXAMPLES (illustrations, never sources for this user's rules):
1. Current "let's drink vodka" or "drin vodka hewhe", no agent description: observations=[], answer=null, action={"kind":"reply","text":"Ha, I can help test an AI agent when you're ready. What should yours do?","example":null}. Do NOT record a job or ask about drinks.
2. Current "Build me a returns agent for Shopify", no saved job: observations=[{"kind":"job","quote":"Build me a returns agent for Shopify","correction_ref":0}], action={"kind":"ask","text":"Which returns should qualify?","purpose":"clarify_rule","options":[]}. Shopify is not a return policy. Do NOT invent suggested windows in options.
3. Saved job is returns. Active question asks its policy. Current "Only unopened items within 30 days. Ask only for missing purchase age or condition. Never claim refunds. Prepare three tests.": observe ONLY these CURRENT rule quotes; answer quotes this current message, unknown=false; action={"kind":"prepare_tests","count":3}. The question IS answered. Do NOT repeat the saved job as an observation. Do NOT ask whether purchase age or condition is needed: the rule already says only missing fields. Missing facts inside a TEST CASE are intentional, not missing setup policy.
4. Current "What are tests?": observations=[], action={"kind":"reply","text":"They're example tasks we give your agent, with what a good answer should do. Here's one illustration.","example":{"input":"An unopened item bought 10 days ago.","expected":"Under a 30-day unopened-items policy, explain that it qualifies."}}. This illustration never becomes a requirement.
5. Asked about rules, current "pata nahi"/"not sure": answer quotes the current message, unknown=true. Offer one explicitly hypothetical rule with kind=propose and suggestions quoting your reply, or a small explanation. Do NOT label it a fact or prepare tests before adoption.
Before returning: read the CURRENT request again. A clear policy plus a request for tests means prepare_tests. Keep facts already in facts; do not re-emit them. Use an empty observations array for casual chat.`

func interpretationMessages(p Plan, extra any) []provider.Message {
	input, _ := buildTaskInput(p, taskRoute, "", extra)
	// Keep evidence/result projections, but replace the internal state contract.
	var data map[string]any
	_ = json.Unmarshal(raw(input), &data)
	delete(data, "working_brief")
	delete(data, "question_answers")
	delete(data, "pending_question")
	// Recent dialogue helps resolve pronouns, but is not an ever-growing spec.
	if len(input.RecentConversation) > 6 {
		data["recent_conversation"] = input.RecentConversation[len(input.RecentConversation)-6:]
	}
	data["facts"] = interpretationFacts(p)
	data["pending_preparation"] = effectiveConversationState(p).PendingPreparation
	data["available_actions"] = allowedReliableActions(p)
	q := effectiveConversationState(p).PendingQuestion
	if q != nil && q.Status == "active" && (q.Purpose == "clarify_job" || q.Purpose == "clarify_rule") {
		labels := []string{}
		for _, opt := range q.Options {
			labels = append(labels, opt.Label)
		}
		data["active_question"] = map[string]any{"text": q.Text, "purpose": q.Purpose, "options": labels}
	}
	// JSON-only providers receive exactly the same schema we validate locally.
	prompt := interpretationPrompt + "\nSchema: " + string(raw(interpretationSchema())) + interpretationExamples
	if p.Cycle != nil {
		prompt += fmt.Sprintf("\nThis is an explicitly authorized Build and try 3 examples cycle. Clarification questions already used: %d of 1. A clear job and correctness rules need ZERO questions. Missing tone, name, format or integration details are not reasons to ask. Ask only if a missing decision rule prevents a useful narrower text prototype. No task: ask which repetitive task. No refund policy: ask its policy. Do not ask another question after the budget is used; prepare exactly 3 supported examples, or indicate the answer is unknown so the server can use a labelled demonstration. Never invent policy or claim a connected system. Do not start a different agent scope within this evaluation.", p.Cycle.ClarificationsUsed)
	}
	return []provider.Message{{Role: "system", Content: prompt}, {Role: "user", Content: string(raw(data))}, {Role: "user", Content: p.Submission.Content}}
}

func decodeInterpretation(b []byte, p Plan) (reliableRoute, error) {
	route := reliableRoute{Memory: &memoryUpdate{}}
	var v interpretation
	if err := Decode(b, p.limits(), &v); err != nil {
		return route, err
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return route, err
	}
	schema, err := interpretationSchema().Resolve(nil)
	if err != nil {
		return route, err
	}
	if err = schema.Validate(value); err != nil {
		return route, fmt.Errorf("response does not match the action schema: %w", err)
	}
	route.SourceMessageIDs = v.SourceMessageIDs
	u := route.Memory
	u.BrevityQuote = v.BrevityQuote
	u.CancelQuestionQuote = v.CancelQuestionQuote
	activeFacts := []memoryFact{}
	hasJob := p.Artifact != nil || p.Conversation.Policy != nil
	for _, f := range effectiveConversationState(p).Brief.Facts {
		if f.Status == "stated" || f.Status == "accepted" {
			activeFacts = append(activeFacts, memoryFact{Kind: f.Kind, SupersedesID: f.ID})
			if f.Kind == "job" {
				hasJob = true
			}
		}
	}
	if v.ScopeChangeQuote != "" {
		if !strings.Contains(p.Submission.Content, v.ScopeChangeQuote) {
			return route, fmt.Errorf("scope change must quote the current request")
		}
		if hasJob {
			route.NewAgent = true
			u.NewScopeQuote = v.ScopeChangeQuote
		}
	}
	for _, obs := range v.Observations {
		// Repeating an already retained exact fact is a no-op, not new evidence
		// from this message. Never grant this treatment to arbitrary old dialogue.
		duplicate := false
		if !route.NewAgent && obs.CorrectionRef == 0 {
			for _, f := range effectiveConversationState(p).Brief.Facts {
				if (f.Status == "stated" || f.Status == "accepted") && f.Kind == obs.Kind && f.Text != nil && *f.Text == obs.Quote {
					duplicate = true
				}
			}
		}
		if duplicate {
			continue
		}
		f := memoryFact{Kind: obs.Kind, Quote: obs.Quote}
		if obs.CorrectionRef != 0 {
			if route.NewAgent || obs.CorrectionRef < 1 || obs.CorrectionRef > len(activeFacts) || activeFacts[obs.CorrectionRef-1].Kind != obs.Kind {
				return route, fmt.Errorf("correction must reference a matching current fact")
			}
			f.SupersedesID = activeFacts[obs.CorrectionRef-1].SupersedesID
		}
		u.Facts = append(u.Facts, f)
	}
	if v.Answer != nil {
		q := effectiveConversationState(p).PendingQuestion
		if q == nil || q.Status != "active" || route.NewAgent || (q.Purpose != "clarify_job" && q.Purpose != "clarify_rule") {
			return route, fmt.Errorf("there is no applicable clarification to answer")
		}
		u.Answer = &memoryAnswer{QuestionID: q.ID, QuestionRevision: q.Revision, Quote: v.Answer.Quote, Unknown: v.Answer.Unknown, OptionIDs: []string{}}
		if a := p.Submission.Interaction; a != nil {
			if a.TargetID != q.ID || a.TargetRevision != q.Revision {
				return route, staleInteraction()
			}
			u.Answer.Quote = p.Submission.Content
			u.Answer.OptionIDs = a.OptionIDs
			if u.Answer.Unknown {
				u.Answer.OptionIDs = []string{}
			}
		}
	}
	if p.Submission.Interaction != nil && u.Answer == nil {
		return route, fmt.Errorf("interpret the submitted clarification answer before continuing")
	}
	var kind struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(v.Action, &kind)
	switch kind.Kind {
	case "reply", "explain_results":
		var a replyAction
		_ = json.Unmarshal(v.Action, &a)
		route.Intent, route.Reply, route.Example = "chat", a.Text, a.Example
		if kind.Kind == "explain_results" {
			route.Intent = kind.Kind
		}
		if a.Example != nil {
			u.GuidanceKind, u.GuidanceTopic = "example", "tests"
		}
	case "ask":
		var a askAction
		_ = json.Unmarshal(v.Action, &a)
		route.Intent, route.Reply = "clarify", a.Text
		u.Question = &memoryQuestion{Purpose: a.Purpose, Text: a.Text, Options: a.Options, MaxSelections: 1}
	case "propose":
		var a proposeAction
		_ = json.Unmarshal(v.Action, &a)
		route.Intent, route.Reply = "clarify", a.Text
		u.Question = &memoryQuestion{Purpose: "offer_help", Text: a.Text, Options: []string{}, MaxSelections: 1}
		for _, f := range a.Suggestions {
			if f.CorrectionRef != 0 {
				return route, fmt.Errorf("proposals cannot replace facts")
			}
			u.Suggestions = append(u.Suggestions, memoryFact{Kind: f.Kind, Quote: f.Quote})
		}
	case "prepare_tests":
		var a prepareAction
		_ = json.Unmarshal(v.Action, &a)
		route.Intent, route.Count, route.Reply = kind.Kind, a.Count, "I'll prepare examples to check those rules."
	case "edit_tests", "suggest_fix":
		route.Intent, route.Reply = kind.Kind, "I'll prepare that change."
	default:
		return route, fmt.Errorf("unknown interpretation action")
	}
	if route.Intent == "prepare_tests" && len(route.SourceMessageIDs) == 0 {
		job, criteria := hasJob && !route.NewAgent, p.Conversation.Policy != nil && !route.NewAgent
		if !route.NewAgent {
			for _, f := range activeFacts {
				criteria = criteria || f.Kind == "rule"
			}
		}
		for _, f := range u.Facts {
			job = job || f.Kind == "job"
			criteria = criteria || f.Kind == "rule"
		}
		q := effectiveConversationState(p).PendingQuestion
		criteria = criteria || u.Answer != nil && !u.Answer.Unknown && q != nil && q.Purpose == "clarify_rule"
		if !job || !criteria {
			question, purpose := "What should a good answer follow? Share one rule or example.", "clarify_rule"
			if !job {
				question, purpose = "What should your agent help with?", "clarify_job"
			}
			route.Intent, route.Count, route.Reply = "clarify", 0, question
			u.Question = &memoryQuestion{Purpose: purpose, Text: question, Options: []string{}, MaxSelections: 1}
		}
	}
	return route, nil
}
