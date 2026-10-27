package vibe

import (
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestObservedReportSnapshotTypedProjection(t *testing.T) {
	source := ObservedEvidence{Binding: ProjectResultBinding{ResultRef: ResultRef{Kind: ResultSDKReport, ID: uuid.New()}}, EvidenceMetadata: EvidenceMetadata{EvidenceProvenance: EvidenceProvenance{Origin: OriginImported}}, Cases: []ObservedCase{{EntryID: "case", SourceIndex: 0, CaseEvidenceBinding: CaseEvidenceBinding{CaseKey: "PRIVATE_CASE_LABEL", SourceCaseID: "PRIVATE_UPLOADER_ID"}, Status: "failed", CaptureScope: UnknownCaptureScope(), CaptureAssurance: "imported", Entries: []ObservedEntry{{EntryID: "grade", Kind: "grade", Value: json.RawMessage(`{"passed":null,"score":"1.25","normalized_score":1.2300000000000000001e-10,"reason":"PRIVATE_REASON","name":"PRIVATE_NAME","finding":{"quotes":[{"text":"PRIVATE_QUOTE","extra":"PRIVATE_UNSELECTED"}]},"headers":{"Authorization":"PRIVATE_CREDENTIAL"}}`)}}}}}
	sel := ReportSelection{Cases: []ReportCaseSelection{{EntryID: "case", Entries: []ReportEntrySelection{{EntryID: "grade"}}}}}
	copy, err := projectReport(source, sel)
	if err != nil {
		t.Fatal(err)
	}
	facts := copy.Cases[0].Entries[0].Facts
	if len(facts) != 1 || facts[0].Value != "1.2300000000000000001e-10" {
		t.Fatal("default projection coerced null/string into typed scalar or rounded numeric lexeme", facts)
	}
	if strings.Contains(string(raw(copy)), "PRIVATE_") {
		t.Fatal("default private text copied")
	}
	sel.Cases[0].Entries[0].IncludeText = true
	copy, err = projectReport(source, sel)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw(copy))
	for _, text := range []string{"PRIVATE_REASON", "PRIVATE_NAME", "PRIVATE_QUOTE"} {
		if !strings.Contains(body, text) {
			t.Fatal("explicit text leaf missing", text)
		}
	}
	for _, text := range []string{"PRIVATE_UNSELECTED", "PRIVATE_CREDENTIAL", "PRIVATE_UPLOADER_ID", "PRIVATE_CASE_LABEL"} {
		if strings.Contains(body, text) {
			t.Fatal("whole source object copied", text)
		}
	}
}
