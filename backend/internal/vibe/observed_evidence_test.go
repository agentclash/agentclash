package vibe

import (
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestObservedSDKPreservesOriginalGradesOrderAndCapture(t *testing.T) {
	report := SDKReport{ID: uuid.New(), SourceKind: "eval_report", ContentHash: "original-bytes", Binding: ProjectResultBinding{SessionID: uuid.New(), ArtifactID: uuid.New(), ResultRef: ResultRef{Kind: ResultSDKReport, ID: uuid.New()}}, EvidenceMetadata: EvidenceMetadata{EvidenceProvenance: EvidenceProvenance{Origin: OriginImported, CaptureScope: UnknownCaptureScope()}}, Cases: []SDKCaseBinding{{SourceCaseID: "a", CaseKey: "same"}, {SourceCaseID: "z", CaseKey: "same", CaptureScope: &CaptureScope{Messages: DimensionCapture{Scope: "complete", Instrumentation: "instrumented"}, Tools: DimensionCapture{Scope: "complete", Instrumentation: "instrumented"}, Retrieval: DimensionCapture{Scope: "complete", Instrumentation: "instrumented"}}}}, SourceJSON: `{"cases":[{"case":{"case_id":"z"},"status":"failed","metrics":[{"key":"private-key","score":0.1234567890123456789,"passed":false}],"agent_result":{"input":"hi","messages":[],"tool_calls":[],"retrieval_context":[]}},{"case":{"case_id":"a"},"status":"passed","metrics":[{"score":0}],"agent_result":{"input":"hi","tool_calls":[{"name":"lookup","arguments":{"id":1},"result":"found"}],"retrieval_context":[{"content":"observed document","score":0.5}]}}]}`}
	got, err := observedSDK(report)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Cases) != 2 || got.Cases[0].SourceCaseID != "z" || got.Cases[0].SourceIndex != 0 {
		t.Fatalf("original order lost: %+v", got.Cases)
	}
	if got.Cases[0].CaptureScope.Tools.Scope != "complete" || len(got.Cases[0].Entries) != 1 {
		t.Fatalf("complete empty tools confused: %+v", got.Cases[0])
	}
	if !strings.Contains(string(got.Cases[0].Entries[0].Value), "0.1234567890123456789") {
		t.Fatal("numeric grade narrowed or rounded")
	}
	if got.Cases[0].Entries[0].ScoreText != "0.1234567890123456789" || got.Cases[0].Entries[0].Path != "/cases/0/metrics/0" {
		t.Fatal("original numeric lexeme or selected metric coordinate lost", got.Cases[0].Entries[0])
	}
	if got.Cases[1].CaptureScope.Tools.Scope != "unknown" {
		t.Fatal("uploaded observations upgraded capture")
	}
	if got.Cases[0].EntryID == got.Cases[1].EntryID {
		t.Fatal("repeated local keys merged")
	}
	for _, c := range got.Cases {
		if strings.Contains(c.EntryID, "private") || strings.Contains(c.EntryID, "same") {
			t.Fatal("identity exposes uploader label")
		}
	}
	if !json.Valid(raw(got)) {
		t.Fatal("invalid normalized output")
	}
}
func TestObservedCanonicalSeparatesProposalsExecutionsAndMappedOutput(t *testing.T) {
	ref := ResultRef{Kind: ResultRun, ID: uuid.New()}
	agent := uuid.New()
	proposed := normalizeCanonicalEvent(ref, agent, CanonicalEvent{ID: "9223372036854775806", Sequence: 8, Type: "model.call.completed", PayloadHash: "h"}, []byte(`{"output_text":"planning","tool_calls":[{"id":"call1","name":"lookup"}],"reference_corpus":["not retrieved"]}`))
	if len(proposed) != 2 || proposed[1].Kind != "tool_proposal" || proposed[1].EventID != "9223372036854775806" || proposed[1].Index != 0 {
		t.Fatalf("proposal confused: %+v", proposed)
	}
	executed := normalizeCanonicalEvent(ref, agent, CanonicalEvent{ID: "9", Sequence: 9, Type: "tool.call.completed"}, []byte(`{"tool_call_id":"call1","tool_name":"lookup","result":{"content":"actual"}}`))
	if len(executed) != 1 || executed[0].Kind != "tool_execution" {
		t.Fatalf("execution lost: %+v", executed)
	}
	output := normalizeCanonicalEvent(ref, agent, CanonicalEvent{ID: "10", Sequence: 10, Type: "system.output.finalized"}, []byte(`{"adapter":"http_json","output":{"tools":["arbitrary"]},"final_output":{"tools":["arbitrary"]}}`))
	if len(output) != 1 || output[0].Kind != "output" {
		t.Fatalf("mapped output invented tool or duplicate: %+v", output)
	}
}
