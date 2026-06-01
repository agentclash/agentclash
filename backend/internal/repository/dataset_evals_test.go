package repository

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestBuildDatasetMaterializedExamplesStableOrderAndPayload(t *testing.T) {
	firstID := "case-b"
	secondID := "case-a"
	examples := []DatasetExample{
		{ID: uuid.New(), ExternalID: &firstID, Input: json.RawMessage(`{"question":"b"}`), Expected: json.RawMessage(`{"answer":"B"}`), Metadata: json.RawMessage(`{"source":"test"}`)},
		{ID: uuid.New(), ExternalID: &secondID, Input: json.RawMessage(`{"question":"a"}`), Expected: json.RawMessage(`{"answer":"A"}`), Metadata: json.RawMessage(`{}`)},
	}

	items, err := buildDatasetMaterializedExamples(examples)
	if err != nil {
		t.Fatalf("buildDatasetMaterializedExamples() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].ItemKey != "case-a" || items[1].ItemKey != "case-b" {
		t.Fatalf("item order = [%s %s], want [case-a case-b]", items[0].ItemKey, items[1].ItemKey)
	}

	var stored struct {
		SchemaVersion int32  `json:"schema_version"`
		CaseKey       string `json:"case_key"`
		Payload       map[string]any
		Inputs        []struct {
			Key   string `json:"key"`
			Kind  string `json:"kind"`
			Value any    `json:"value"`
		} `json:"inputs"`
		Expectations []struct {
			Key    string `json:"key"`
			Kind   string `json:"kind"`
			Source string `json:"source"`
			Value  any    `json:"value"`
		} `json:"expectations"`
	}
	if err := json.Unmarshal(items[0].Payload, &stored); err != nil {
		t.Fatalf("payload is invalid JSON: %v", err)
	}
	if stored.SchemaVersion != 1 || stored.CaseKey != "case-a" {
		t.Fatalf("stored envelope = version %d case %q, want version 1 case-a", stored.SchemaVersion, stored.CaseKey)
	}
	if len(stored.Inputs) != 1 || stored.Inputs[0].Key != "input" || stored.Inputs[0].Kind != "json" {
		t.Fatalf("inputs = %#v, want canonical dataset input", stored.Inputs)
	}
	if len(stored.Expectations) != 1 || stored.Expectations[0].Key != "expected" || stored.Expectations[0].Source != "dataset" {
		t.Fatalf("expectations = %#v, want dataset expected contract", stored.Expectations)
	}
}

func TestDatasetEvalInputChecksumChangesWithVersion(t *testing.T) {
	exampleID := "case-1"
	items, err := buildDatasetMaterializedExamples([]DatasetExample{{
		ID: uuid.New(), ExternalID: &exampleID, Input: json.RawMessage(`{"question":"x"}`), Metadata: json.RawMessage(`{}`),
	}})
	if err != nil {
		t.Fatalf("buildDatasetMaterializedExamples() error = %v", err)
	}
	params := MaterializeDatasetVersionInputSetParams{DatasetVersionID: uuid.New(), ChallengePackVersionID: uuid.New(), ChallengeKey: "support"}

	v1 := DatasetVersion{ManifestChecksum: "one"}
	v2 := DatasetVersion{ManifestChecksum: "two"}
	if datasetEvalInputChecksum(v1, params, items) == datasetEvalInputChecksum(v2, params, items) {
		t.Fatal("checksum did not change when dataset version manifest checksum changed")
	}
}
