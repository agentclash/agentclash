package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
)

func TestVibeEvidenceUploadPreservesUnambiguousJSON(t *testing.T) {
	for _, body := range []string{
		`{"content":"Customer: Hi\nAgent: \uD800"}`,
		`{"content":"Customer: Hi\nAgent: First","content":"Customer: Hi\nAgent: Different"}`,
		`{"content":"Customer: Hi\nAgent: First"} {}`,
	} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if err := decodeEvidenceInput(httptest.NewRecorder(), r, true, new(vibe.EvidenceInput)); err == nil {
			t.Fatal("ambiguous evidence accepted", body)
		}
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"revision":3,"content":"Customer: Hi\r\nAgent:  exact reply  \r\n"}`))
	r.Header.Set("Content-Type", "application/json")
	var input vibe.EvidenceInput
	if err := decodeEvidenceInput(httptest.NewRecorder(), r, true, &input); err != nil {
		t.Fatal(err)
	}
	if input.Revision != 3 || input.Content != "Customer: Hi\r\nAgent:  exact reply  \r\n" {
		t.Fatal("evidence was normalized during decoding")
	}
}
