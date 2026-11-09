package templateutil

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateToolParameterSchemaItemsForm(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		wantErr bool
	}{
		{name: "items schema", schema: `{"type":"object","properties":{"tags":{"type":"array","items":{"type":"string"}}}}`},
		{name: "prefixItems", schema: `{"type":"array","prefixItems":[{"type":"string"}]}`},
		// jsonschema-go v0.3.0 did not parse "dependencies", so its content stays unchecked.
		{name: "dependencies content", schema: `{"type":"object","dependencies":{"a":{"items":[]}}}`},
		{name: "root array items", schema: `{"type":"array","items":[{"type":"string"}]}`, wantErr: true},
		{name: "nested array items", schema: `{"type":"object","properties":{"pair":{"anyOf":[{"type":"array","items":[]}]}}}`, wantErr: true},
		{name: "array items in $defs", schema: `{"$defs":{"pair":{"items":[{"type":"string"}]}},"$ref":"#/$defs/pair"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolParameterSchema(json.RawMessage(tt.schema))
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), `"items" must be a schema, not an array`) {
				t.Fatalf("error = %v, want the array items error", err)
			}
		})
	}
}
