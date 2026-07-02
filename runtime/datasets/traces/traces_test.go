package traces_test

import (
	"encoding/json"
	"testing"

	datasettraces "github.com/agentclash/agentclash/runtime/datasets/traces"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
)

func TestImportOTLPGenAISpans(t *testing.T) {
	payload := []byte(`{
	  "resourceSpans": [{
	    "scopeSpans": [{
	      "spans": [{
	        "traceId": "abc123",
	        "spanId": "span-1",
	        "name": "chat",
	        "attributes": [
	          {"key":"gen_ai.request.model","value":{"stringValue":"gpt-4.1-mini"}},
	          {"key":"gen_ai.input.messages","value":{"stringValue":"[{\"role\":\"user\",\"content\":\"refund?\"}]"}},
	          {"key":"gen_ai.output.messages","value":{"stringValue":"[{\"role\":\"assistant\",\"content\":\"approved\"}]"}},
	          {"key":"gen_ai.usage.input_tokens","value":{"intValue":"12"}},
	          {"key":"gen_ai.response.finish_reasons","value":{"stringValue":"[\"stop\"]"}}
	        ]
	      }]
	    }]
	  }]
	}`)
	result, err := datasettraces.ImportFromPayload(datasettraces.SourceOTel, payload)
	if err != nil {
		t.Fatalf("ImportFromPayload() error = %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("errors = %+v", result.Errors)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(result.Candidates))
	}
	candidate := result.Candidates[0]
	if candidate.SourceTraceID != "abc123" {
		t.Fatalf("source_trace_id = %q", candidate.SourceTraceID)
	}
	assertJSONContains(t, candidate.Input, "refund?")
	assertJSONContains(t, candidate.Output, "approved")
	assertJSONContains(t, candidate.Metadata, "gen_ai.request.model")
}

func TestImportVendorSpanExportBraintrust(t *testing.T) {
	data := []byte(`{"input":{"question":"refund?"},"expected":{"answer":"yes"},"metadata":{"model":"gpt-4"},"tags":["billing"],"external_id":"trace-1"}` + "\n")
	result, err := datasettraces.ImportFromPayload(datasettraces.SourceBraintrust, data)
	if err != nil {
		t.Fatalf("ImportFromPayload() error = %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("len(candidates) = %d", len(result.Candidates))
	}
	candidate := result.Candidates[0]
	assertJSONContains(t, candidate.Input, "refund?")
	assertJSONContains(t, candidate.Expected, "yes")
}

func TestRedactCandidateMetadata(t *testing.T) {
	candidate := datasettraces.Candidate{
		Metadata: json.RawMessage(`{"email":"secret@example.com","model":"gpt-4"}`),
	}
	redacted, err := datasettraces.RedactCandidate(candidate, datasettraces.RedactionConfig{
		DropMetadataKeys: []string{"email"},
	})
	if err != nil {
		t.Fatalf("RedactCandidate() error = %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(redacted.Metadata, &metadata); err != nil {
		t.Fatalf("Unmarshal metadata: %v", err)
	}
	if _, ok := metadata["email"]; ok {
		t.Fatalf("email should be dropped")
	}
	if metadata["model"] != "gpt-4" {
		t.Fatalf("model = %#v", metadata["model"])
	}
}

func TestRedactCandidateMetadataNestedPath(t *testing.T) {
	candidate := datasettraces.Candidate{
		Metadata: json.RawMessage(`{"user":{"email":"secret@example.com"},"model":"gpt-4"}`),
	}
	redacted, err := datasettraces.RedactCandidate(candidate, datasettraces.RedactionConfig{
		DropMetadataPaths: []string{"user.email"},
	})
	if err != nil {
		t.Fatalf("RedactCandidate() error = %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(redacted.Metadata, &metadata); err != nil {
		t.Fatalf("Unmarshal metadata: %v", err)
	}
	user, ok := metadata["user"].(map[string]any)
	if !ok {
		t.Fatalf("user metadata missing")
	}
	if _, ok := user["email"]; ok {
		t.Fatalf("nested email should be dropped")
	}
}

func TestCandidatesFromRunEventsTranscript(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	turnIndex := 0
	events := []runevents.Envelope{
		{
			RunID: runID, RunAgentID: runAgentID, SequenceNumber: 1,
			EventType: runevents.EventTypeTurnUserMessage,
			Summary:   runevents.SummaryMetadata{TurnIndex: &turnIndex},
			Payload:   json.RawMessage(`{"content":"Need a refund"}`),
		},
		{
			RunID: runID, RunAgentID: runAgentID, SequenceNumber: 2,
			EventType: runevents.EventTypeTurnAssistantMessage,
			Summary:   runevents.SummaryMetadata{TurnIndex: &turnIndex},
			Payload:   json.RawMessage(`{"content":"Approved"}`),
		},
	}
	candidates, err := datasettraces.CandidatesFromRunEvents(runID, runAgentID, events)
	if err != nil {
		t.Fatalf("CandidatesFromRunEvents() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if candidates[0].SourceRunID == nil || *candidates[0].SourceRunID != runID {
		t.Fatalf("source_run_id = %v", candidates[0].SourceRunID)
	}
	assertJSONContains(t, candidates[0].Input, "Need a refund")
	assertJSONContains(t, candidates[0].Output, "Approved")
}

func assertJSONContains(t *testing.T, raw json.RawMessage, needle string) {
	t.Helper()
	if !jsonContains(raw, needle) {
		t.Fatalf("json %s does not contain %q", string(raw), needle)
	}
}

func jsonContains(raw json.RawMessage, needle string) bool {
	return len(raw) > 0 && string(raw) != "" && (string(raw) == needle || containsSubstring(string(raw), needle))
}

func containsSubstring(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexSubstring(haystack, needle) >= 0)
}

func indexSubstring(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
