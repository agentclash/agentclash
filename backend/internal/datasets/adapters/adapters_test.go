package adapters

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestImportAdaptersNormalizeEquivalentExamples(t *testing.T) {
	tests := []struct {
		name   string
		format string
		data   string
	}{
		{
			name:   "openai",
			format: "openai",
			data:   `{"input":{"question":"refund?"},"ideal":{"answer":"yes"},"metadata":{"difficulty":"easy"},"tags":["billing"],"external_id":"case-1"}`,
		},
		{
			name:   "braintrust",
			format: "braintrust",
			data:   `{"input":{"question":"refund?"},"expected":{"answer":"yes"},"metadata":{"difficulty":"easy"},"tags":["billing"],"external_id":"case-1"}`,
		},
		{
			name:   "langsmith",
			format: "langsmith",
			data:   `{"inputs":{"question":"refund?"},"outputs":{"answer":"yes"},"metadata":{"difficulty":"easy"},"tags":["billing"],"id":"case-1"}`,
		},
		{
			name:   "phoenix",
			format: "phoenix",
			data:   `{"input":{"question":"refund?"},"output":{"answer":"yes"},"metadata":{"difficulty":"easy"},"tags":["billing"],"example_id":"case-1"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Import(tt.format, []byte(tt.data+"\n"), Mapping{})
			if err != nil {
				t.Fatalf("Import() error = %v", err)
			}
			if len(result.Errors) > 0 {
				t.Fatalf("Import() errors = %+v", result.Errors)
			}
			if len(result.Examples) != 1 {
				t.Fatalf("len(examples) = %d, want 1", len(result.Examples))
			}
			example := result.Examples[0]
			assertJSONEqual(t, example.Input, `{"question":"refund?"}`)
			assertJSONEqual(t, example.Expected, `{"answer":"yes"}`)
			assertJSONEqual(t, example.Metadata, `{"difficulty":"easy"}`)
			if example.ExternalID == nil || *example.ExternalID != "case-1" {
				t.Fatalf("external_id = %v, want case-1", example.ExternalID)
			}
			if len(example.Tags) != 1 || example.Tags[0] != "billing" {
				t.Fatalf("tags = %#v, want [billing]", example.Tags)
			}
		})
	}
}

func TestImportOpenAIItemRows(t *testing.T) {
	result, err := Import("openai", []byte(`{"item":{"prompt":"Classify","label":"yes"},"ideal":"yes","id":"item-1"}`+"\n"), Mapping{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("Import() errors = %+v", result.Errors)
	}
	example := result.Examples[0]
	assertJSONEqual(t, example.Input, `{"prompt":"Classify","label":"yes"}`)
	assertJSONEqual(t, example.Expected, `"yes"`)
	if example.ExternalID == nil || *example.ExternalID != "item-1" {
		t.Fatalf("external_id = %v, want item-1", example.ExternalID)
	}
}

func TestImportGenericJSONLMapping(t *testing.T) {
	mapping := Mapping{
		InputKeys:    []string{"prompt", "locale"},
		OutputKeys:   []string{"answer"},
		MetadataKeys: []string{"source"},
		TagsKey:      "labels",
		IDKey:        "stable_id",
	}
	result, err := Import("jsonl", []byte(`{"stable_id":"row-1","prompt":"Hi","locale":"en","answer":"Hello","source":"golden","labels":"greeting, smoke"}`+"\n"), mapping)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("Import() errors = %+v", result.Errors)
	}
	example := result.Examples[0]
	assertJSONEqual(t, example.Input, `{"prompt":"Hi","locale":"en"}`)
	assertJSONEqual(t, example.Expected, `"Hello"`)
	assertJSONEqual(t, example.Metadata, `"golden"`)
	if example.ExternalID == nil || *example.ExternalID != "row-1" {
		t.Fatalf("external_id = %v, want row-1", example.ExternalID)
	}
	if got := strings.Join(example.Tags, ","); got != "greeting,smoke" {
		t.Fatalf("tags = %q, want greeting,smoke", got)
	}
}

func TestImportCSVMappedRows(t *testing.T) {
	data := "id,prompt,expected,metadata,tags\ncase-1,hello,hi,\"{ \"\"source\"\": \"\"csv\"\" }\",\"smoke,csv\"\n"
	result, err := Import("csv", []byte(data), Mapping{InputKeys: []string{"prompt"}, OutputKeys: []string{"expected"}, MetadataKeys: []string{"metadata"}, TagsKey: "tags", IDKey: "id"})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("Import() errors = %+v", result.Errors)
	}
	example := result.Examples[0]
	assertJSONEqual(t, example.Input, `"hello"`)
	assertJSONEqual(t, example.Expected, `"hi"`)
	assertJSONEqual(t, example.Metadata, `{"source":"csv"}`)
	if example.ExternalID == nil || *example.ExternalID != "case-1" {
		t.Fatalf("external_id = %v, want case-1", example.ExternalID)
	}
}

func TestImportReportsRowErrors(t *testing.T) {
	result, err := Import("braintrust", []byte(`{"expected":"missing input"}`+"\nnot-json\n"), Mapping{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(result.Examples) != 0 {
		t.Fatalf("len(examples) = %d, want 0", len(result.Examples))
	}
	if len(result.Errors) != 2 {
		t.Fatalf("len(errors) = %d, want 2: %+v", len(result.Errors), result.Errors)
	}
	if result.Errors[0].Row != 1 || result.Errors[1].Row != 2 {
		t.Fatalf("rows = %+v, want row-indexed errors", result.Errors)
	}
}

func TestExportRoundTripPreservesCanonicalFields(t *testing.T) {
	original, err := Import("braintrust", []byte(`{"input":{"q":"x"},"expected":{"a":"y"},"metadata":{"m":1},"tags":["t"],"external_id":"e1"}`+"\n"), Mapping{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	exported, _, err := Export("phoenix", original.Examples)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	roundTrip, err := Import("phoenix", exported, Mapping{})
	if err != nil {
		t.Fatalf("Import(roundTrip) error = %v", err)
	}
	if len(roundTrip.Errors) > 0 {
		t.Fatalf("roundTrip errors = %+v", roundTrip.Errors)
	}
	got := roundTrip.Examples[0]
	assertJSONEqual(t, got.Input, `{"q":"x"}`)
	assertJSONEqual(t, got.Expected, `{"a":"y"}`)
	assertJSONEqual(t, got.Metadata, `{"m":1}`)
	if got.ExternalID == nil || *got.ExternalID != "e1" {
		t.Fatalf("external_id = %v, want e1", got.ExternalID)
	}
}

func assertJSONEqual(t *testing.T, got json.RawMessage, want string) {
	t.Helper()
	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("got invalid JSON %q: %v", string(got), err)
	}
	var wantValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("want invalid JSON %q: %v", want, err)
	}
	gotBytes, _ := json.Marshal(gotValue)
	wantBytes, _ := json.Marshal(wantValue)
	if string(gotBytes) != string(wantBytes) {
		t.Fatalf("JSON = %s, want %s", gotBytes, wantBytes)
	}
}
