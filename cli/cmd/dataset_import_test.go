package cmd

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestDatasetImportMappingFromMapFlags(t *testing.T) {
	cmd := testDatasetImportMappingCommand()
	cmd.Flags().Set("map", "input_keys=prompt,locale")
	cmd.Flags().Set("map", "output_keys=answer")
	cmd.Flags().Set("map", "metadata_keys=source")
	cmd.Flags().Set("map", "tags_key=labels")
	cmd.Flags().Set("map", "id_key=stable_id")

	raw, err := datasetImportMappingFromFlags(cmd)
	if err != nil {
		t.Fatalf("datasetImportMappingFromFlags() error = %v", err)
	}
	var mapping map[string]any
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		t.Fatalf("mapping JSON invalid: %v", err)
	}
	if got := mapping["tags_key"]; got != "labels" {
		t.Fatalf("tags_key = %v, want labels", got)
	}
	inputKeys, ok := mapping["input_keys"].([]any)
	if !ok || len(inputKeys) != 2 || inputKeys[0] != "prompt" || inputKeys[1] != "locale" {
		t.Fatalf("input_keys = %#v, want [prompt locale]", mapping["input_keys"])
	}
}

func TestDatasetImportMappingPrefersRawMapping(t *testing.T) {
	cmd := testDatasetImportMappingCommand()
	cmd.Flags().Set("mapping", `{"id_key":"id"}`)
	cmd.Flags().Set("map", "id_key=other")

	raw, err := datasetImportMappingFromFlags(cmd)
	if err != nil {
		t.Fatalf("datasetImportMappingFromFlags() error = %v", err)
	}
	if raw != `{"id_key":"id"}` {
		t.Fatalf("mapping = %q, want raw mapping", raw)
	}
}

func testDatasetImportMappingCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("mapping", "", "")
	cmd.Flags().StringArray("map", nil, "")
	return cmd
}
