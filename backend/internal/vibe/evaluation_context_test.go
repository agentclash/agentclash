package vibe

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestVibeEvaluationContextDeduplicatesWithoutChangingEvidence(t *testing.T) {
	chat := EvidenceConversation{Key: "chat-one", Title: "Return", Messages: []EvidenceMessage{
		{ID: "m1", Role: "user", Content: "Unopened, bought 10 days ago. " + strings.Repeat("Order details preserved exactly. ", 100)},
		{ID: "m2", Role: "assistant", Content: "When did you buy it?"},
	}}
	e := EvidenceSet{ID: uuid.New(), Label: "Support chats", Raw: string(raw(chat)), Conversations: []EvidenceConversation{chat}, CreatedAt: timestamp()}
	q := []Expectation{{ID: "rule-1", Statement: "Remember details already provided. Do not ask for them again."}}
	a := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", Title: "Return check", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: q}}
	c := conversationResult(Plan{Artifact: &a}, chat)
	c.Verdict, c.Checks = Fail, []CheckResult{{Key: q[0].ID, Verdict: Fail, Evidence: "The reply asks for the purchase age again.", MessageIDs: []string{"m1", "m2"}}}
	p := Plan{AuthoringVersion: 5, Evidence: &e, Artifact: &a, Observations: []CaseResult{c}, Submission: Submission{Content: "How can I fix this?"}, Document: Document{Messages: []Message{{ID: uuid.New(), Role: "user", Content: "Apply the 30-day policy.", CreatedAt: timestamp()}}}}
	before := raw(p)
	legacy := evaluationAuthoringMessages(p)
	p.AuthoringVersion = 6
	compact := evaluationAuthoringMessages(p)
	if len(raw(compact))*100 >= len(raw(legacy))*65 {
		t.Fatalf("duplicated source remains: compact=%d legacy=%d", len(raw(compact)), len(raw(legacy)))
	}
	t.Logf("assembled context bytes: %d -> %d", len(raw(legacy)), len(raw(compact)))
	var got struct {
		Provided struct {
			Conversations []EvidenceConversation `json:"conversations"`
			Raw           string                 `json:"raw"`
		} `json:"provided_chats"`
		Current struct {
			Expectations []Expectation `json:"expectations"`
		} `json:"current_check"`
		Observations []struct {
			Checks          []CheckResult     `json:"checks"`
			Messages        []EvidenceMessage `json:"messages"`
			MessagesRef     string            `json:"messages_ref"`
			ExpectationsRef string            `json:"expectations_ref"`
		} `json:"observations"`
	}
	if err := json.Unmarshal([]byte(compact[1].Content), &got); err != nil {
		t.Fatal(err)
	}
	if got.Provided.Raw != "" || !bytes.Equal(raw(got.Provided.Conversations), raw(e.Conversations)) || !bytes.Equal(raw(got.Current.Expectations), raw(q)) || !bytes.Equal(raw(got.Observations[0].Checks), raw(c.Checks)) {
		t.Fatal("compaction changed evidence, rules, citations or verdicts")
	}
	if len(got.Observations[0].Messages) != 0 || got.Observations[0].MessagesRef == "" || got.Observations[0].ExpectationsRef == "" {
		t.Fatal("duplicate evidence was not replaced by explicit references")
	}
	p.AuthoringVersion = 5
	if !bytes.Equal(before, raw(p)) {
		t.Fatal("projection mutated the stored plan")
	}
	// Identical keys are insufficient: a result from another reply/rule must
	// carry its original evidence, not refer to the newly selected upload.
	p.AuthoringVersion = 6
	p.Observations[0].Messages = []EvidenceMessage{{ID: "m2", Role: "assistant", Content: "It is eligible."}}
	p.Observations[0].Expectations = []Expectation{{ID: "rule-1", Statement: "An older, different rule."}}
	var mismatch map[string]json.RawMessage
	data := evaluationContext(p)["observations"].([]map[string]any)[0]
	if err := json.Unmarshal(raw(data), &mismatch); err != nil {
		t.Fatal(err)
	}
	if mismatch["messages_ref"] != nil || mismatch["expectations_ref"] != nil || !bytes.Equal(mismatch["messages"], raw(p.Observations[0].Messages)) || !bytes.Equal(mismatch["expectations"], raw(p.Observations[0].Expectations)) {
		t.Fatal("compaction conflated different test evidence")
	}
}
