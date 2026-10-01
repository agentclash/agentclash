package interaction

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const other = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

func wire(kind string, p map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"version": Version, "kind": kind, "payload": p})
	return b
}
func question() map[string]any {
	return map[string]any{"id": id, "scope_id": other, "revision": 1, "origin_message_id": id, "purpose": "clarify_rule", "status": "active", "text": "What is the return window?", "options": []any{map[string]any{"id": "30-days", "label": "30 days"}}, "max_selections": 1, "proposal_id": nil, "proposal_revision": nil}
}
func action() map[string]any {
	return map[string]any{"idempotency_key": id, "scope_id": other, "session_revision": 0, "kind": "answer_question", "target_id": id, "target_revision": 1, "option_ids": []string{"30-days"}, "text": nil}
}
func fact() map[string]any {
	return map[string]any{"id": id, "kind": "rule", "status": "stated", "text": "Unopened only", "sources": []any{map[string]any{"message_id": other, "quote": "Unopened only", "sha256": string(bytes.Repeat([]byte("a"), 64))}}, "adoption_message_id": nil, "supersedes_id": nil}
}
func card(kind string, fields map[string]any) map[string]any {
	p := map[string]any{"id": id, "scope_id": other, "origin_message_id": id, "kind": kind}
	for k, v := range fields {
		p[k] = v
	}
	return p
}

func TestContractsAcceptSupportedWireData(t *testing.T) {
	for _, tt := range []struct {
		kind string
		p    map[string]any
	}{
		{"brief", map[string]any{"scope_id": id, "revision": 1, "facts": []any{fact()}}},
		{"question", question()}, {"action", action()},
		{"card", card("text", map[string]any{"text": "One question at a time."})},
		{"card", card("example", map[string]any{"input": "Basic $20, Pro $50", "expected": "Keep prices with their products.", "illustrative": true})},
		{"card", card("choice", map[string]any{"question_id": id, "question_revision": 1})},
		{"card", card("expectation", map[string]any{"proposal_id": id, "proposal_revision": 1, "text": "Keep headings."})},
		{"card", card("evidence", map[string]any{"operation_id": id, "case_key": "late", "check_key": "policy", "finding": "quote", "quote": "Refund processed."})},
		{"card", card("progress", map[string]any{"operation_id": id, "phase": "grading", "completed": 1, "total": 3})},
		{"card", card("recovery", map[string]any{"operation_id": id, "code": "busy", "retry_available_at": nil})},
	} {
		name := tt.kind
		if kind, ok := tt.p["kind"].(string); ok {
			name += "/" + kind
		}
		t.Run(name, func(t *testing.T) {
			got, err := Decode(wire(tt.kind, tt.p))
			if err != nil || got.Kind != tt.kind || got.Version != Version {
				t.Fatalf("decode: %v", err)
			}
		})
	}
}

func TestContractsRejectAmbiguousOrAuthorityBearingData(t *testing.T) {
	for _, tt := range []struct {
		name, kind string
		build      func() map[string]any
	}{
		{"run-is-not-a-setup-answer", "action", func() map[string]any { p := action(); p["kind"] = "run"; return p }},
		{"arbitrary-url", "action", func() map[string]any { p := action(); p["url"] = "https://example.test/run"; return p }},
		{"two-answer-representations", "action", func() map[string]any { p := action(); p["text"] = "yes"; return p }},
		{"missing-answer", "action", func() map[string]any { p := action(); p["option_ids"] = []string{}; return p }},
		{"duplicate-selection", "action", func() map[string]any { p := action(); p["option_ids"] = []string{"a", "a"}; return p }},
		{"adoption-with-answer-payload", "action", func() map[string]any { p := action(); p["kind"] = "adopt_proposal"; return p }},
		{"unbound-adoption", "question", func() map[string]any { p := question(); p["purpose"] = "adopt_proposal"; return p }},
		{"duplicate-options", "question", func() map[string]any {
			p := question()
			p["options"] = []any{map[string]any{"id": "a", "label": "Yes"}, map[string]any{"id": "a", "label": "No"}}
			return p
		}},
		{"impossible-selection", "question", func() map[string]any { p := question(); p["max_selections"] = 2; return p }},
		{"example-is-not-measurement", "card", func() map[string]any {
			return card("example", map[string]any{"input": "PDF", "expected": "Markdown", "illustrative": false})
		}},
		{"inconsistent-progress", "card", func() map[string]any {
			return card("progress", map[string]any{"operation_id": id, "phase": "running", "completed": 4, "total": 3})
		}},
		{"unquoted-evidence", "card", func() map[string]any {
			return card("evidence", map[string]any{"operation_id": id, "case_key": "a", "check_key": "b", "finding": "quote", "quote": nil})
		}},
		{"no-source-for-statement", "brief", func() map[string]any {
			f := fact()
			f["sources"] = []any{}
			return map[string]any{"scope_id": id, "revision": 1, "facts": []any{f}}
		}},
		{"accepted-without-adoption", "brief", func() map[string]any {
			f := fact()
			f["status"] = "accepted"
			return map[string]any{"scope_id": id, "revision": 1, "facts": []any{f}}
		}},
		{"unknown-is-not-a-rule", "brief", func() map[string]any {
			f := fact()
			f["status"] = "unknown"
			return map[string]any{"scope_id": id, "revision": 1, "facts": []any{f}}
		}},
		{"duplicate-facts", "brief", func() map[string]any {
			return map[string]any{"scope_id": id, "revision": 1, "facts": []any{fact(), fact()}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decode(wire(tt.kind, tt.build())); !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected rejection: %v", err)
			}
		})
	}
}

func TestContractsRejectMalformedAndFutureVersions(t *testing.T) {
	valid := wire("question", question())
	for _, b := range [][]byte{
		bytes.Replace(valid, []byte(`"version":1`), []byte(`"version":2`), 1),
		bytes.Replace(valid, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1),
		bytes.Replace(valid, []byte(`"revision":1`), []byte(`"revision":1,"revision":2`), 1),
		append(bytes.Clone(valid), []byte(` {}`)...), []byte(`null`), []byte(`[]`),
		bytes.Repeat([]byte(" "), MaxBytes+1), []byte(`{"version":1e999}`),
	} {
		if _, err := Decode(b); !errors.Is(err, ErrInvalid) {
			t.Fatalf("malformed value accepted: %v", err)
		}
	}
	a := Schema()
	a[0] = 'x'
	if Schema()[0] == 'x' {
		t.Fatal("schema exposed mutable storage")
	}
}
