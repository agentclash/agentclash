package vibe

import (
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/scoring"
)

func observed(id, text string) *Finding {
	return &Finding{Kind: "observed", Quotes: []ReplyQuote{{MessageID: id, Text: text}}, CoveredMessageIDs: []string{}}
}
func TestVibeGroundedJudgeEvidence(t *testing.T) {
	j := scoring.LLMJudgeDeclaration{Key: "refund", Mode: scoring.JudgeMethodAssertion}
	output := strings.Repeat("General policy information. ", 40) + "Actually, I have processed your refund."
	for _, tc := range []struct {
		name    string
		finding *Finding
		pass    any
		valid   bool
	}{
		{"contradiction at end", observed("output", "Actually, I have processed your refund."), false, true},
		{"invented quote", observed("output", "Your refund is denied."), false, false},
		{"normalized whitespace", observed("output", "Actually,  I have processed your refund."), false, false},
		{"wrong message", observed("user", "Actually, I have processed your refund."), false, false},
		{"missing finding", nil, false, false},
		{"missing behavior", &Finding{Kind: "missing_behavior", Missing: "Ask for the purchase age.", CoveredMessageIDs: []string{"output"}}, false, true},
		{"absence cannot pass", &Finding{Kind: "missing_behavior", Missing: "Ask for the purchase age.", CoveredMessageIDs: []string{"output"}}, true, false},
		{"unassessed", &Finding{Kind: "unassessed"}, nil, true},
		{"unknown with claim", observed("output", "General policy information."), nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseGroundedJudge(j, output, raw(map[string]any{"key": j.Key, "pass": tc.pass, "reasoning": "Evidence from the full reply.", "finding": tc.finding}), LimitsFor(true))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if err != nil && result.Verdict != Unknown {
				t.Fatal("invalid evidence got a verdict")
			}
			if err == nil && (result.EvidenceVersion != 1 || result.Finding == nil) {
				t.Fatal("lost grounded finding")
			}
		})
	}
	absent := &Finding{Kind: "missing_behavior", Missing: "Ask for missing information.", CoveredMessageIDs: []string{"output"}}
	if validateFinding(absent, Fail, []EvidenceMessage{{ID: "output", Role: "assistant", Content: ""}}) == nil {
		t.Fatal("empty reply supports absence")
	}
	falseValue := false
	j.Expect = &falseValue
	r, err := parseGroundedJudge(j, output, raw(map[string]any{"key": j.Key, "pass": true, "reasoning": "Forbidden action occurred.", "finding": observed("output", "I have processed your refund.")}), LimitsFor(true))
	if err != nil || r.Verdict != Fail {
		t.Fatal("assertion polarity lost", r, err)
	}
	j.Mode = scoring.JudgeMethodRubric
	j.Expect = nil
	j.ScoreScale = &scoring.ScoreScale{Min: 0, Max: 10}
	r, err = parseGroundedJudge(j, output, raw(map[string]any{"key": j.Key, "score": 10, "reasoning": "Supplied rubric.", "finding": observed("output", "General policy information.")}), LimitsFor(true))
	if err != nil || r.Verdict != Pass {
		t.Fatal("rubric scale lost", r, err)
	}
}

func TestVibeGroundedConversationFindings(t *testing.T) {
	c := EvidenceConversation{Messages: []EvidenceMessage{{ID: "u", Role: "user", Content: "I refunded it."}, {ID: "a1", Role: "assistant", Content: "Please wait."}, {ID: "a2", Role: "assistant", Content: "I refunded it."}}}
	e := []Expectation{{ID: "actions", Statement: "Never claim a refund."}, {ID: "date", Statement: "Ask for missing purchase age."}}
	makeOutput := func(first, second *Finding) []byte {
		return raw(map[string]any{"checks": []any{map[string]any{"key": "actions", "verdict": "FAIL", "evidence": "The reply claims a refund.", "finding": first}, map[string]any{"key": "date", "verdict": "FAIL", "evidence": "No purchase age was requested.", "finding": second}}})
	}
	missing := &Finding{Kind: "missing_behavior", Missing: "Ask for purchase age.", CoveredMessageIDs: []string{"a1", "a2"}}
	checks, err := parseGroundedConversation(makeOutput(observed("a2", "I refunded it."), missing), e, c, LimitsFor(true))
	if err != nil || len(checks) != 2 || checks[0].Verdict != Fail || checks[1].Verdict != Fail {
		t.Fatal(checks, err)
	}
	checks, err = parseGroundedConversation(makeOutput(observed("u", "I refunded it."), missing), e, c, LimitsFor(true))
	if err != nil || checks[0].Verdict != Unknown || checks[0].Evidence != "" || checks[0].Finding != nil || checks[1].Verdict != Fail {
		t.Fatal("customer quote trusted or unrelated valid finding lost", checks, err)
	}
	missing.CoveredMessageIDs = []string{"a2"}
	checks, err = parseGroundedConversation(makeOutput(observed("a2", "I refunded it."), missing), e, c, LimitsFor(true))
	if err != nil || checks[1].Verdict != Unknown {
		t.Fatal("partial coverage proved absence", checks, err)
	}
	b := makeOutput(observed("a2", "I refunded it."), missing)
	if _, err = parseGroundedConversation([]byte(strings.Replace(string(b), `"key":"date"`, `"key":"actions"`, 1)), e, c, LimitsFor(true)); err == nil {
		t.Fatal("duplicate rule accepted")
	}
}
