package cmd

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestDatasetTraceImportBodyRequiresSource(t *testing.T) {
	cmd := testDatasetTraceImportCommand()
	cmd.Flags().Set("source", "otel")
	cmd.SetArgs([]string{"dataset-1", "trace.json"})
	_, err := datasetTraceImportBody(cmd, []string{"dataset-1"})
	if err == nil {
		t.Fatal("expected payload error when no file provided")
	}
}

func TestDatasetTraceImportBodyEncodesFlags(t *testing.T) {
	cmd := testDatasetTraceImportCommand()
	cmd.Flags().Set("source", "agentclash")
	cmd.Flags().Set("run", "run-1")
	cmd.Flags().Set("run-agent", "agent-1")
	cmd.Flags().Set("redaction", `{"drop_metadata_keys":["email"]}`)

	body, err := datasetTraceImportBody(cmd, []string{"dataset-1"})
	if err != nil {
		t.Fatalf("datasetTraceImportBody() error = %v", err)
	}
	if body["source_platform"] != "agentclash" || body["run_id"] != "run-1" || body["run_agent_id"] != "agent-1" {
		t.Fatalf("body = %#v", body)
	}
	redactionBytes, _ := json.Marshal(body["redaction"])
	if string(redactionBytes) != `{"drop_metadata_keys":["email"]}` {
		t.Fatalf("redaction = %s", redactionBytes)
	}
}

func TestDatasetTracePromoteBodyEncodesExpected(t *testing.T) {
	cmd := testDatasetTracePromoteCommand()
	cmd.Flags().Set("expected", `{"answer":"yes"}`)
	cmd.Flags().Set("tag", "billing")
	cmd.Flags().Set("tag", "refund")

	body, err := datasetTracePromoteBody(cmd)
	if err != nil {
		t.Fatalf("datasetTracePromoteBody() error = %v", err)
	}
	expectedBytes, _ := json.Marshal(body["expected"])
	if string(expectedBytes) != `{"answer":"yes"}` {
		t.Fatalf("expected = %s", expectedBytes)
	}
	tags, ok := body["tags"].([]string)
	if !ok || len(tags) != 2 || tags[0] != "billing" || tags[1] != "refund" {
		t.Fatalf("tags = %#v", body["tags"])
	}
}

func testDatasetTraceImportCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("source", "", "")
	cmd.Flags().String("run", "", "")
	cmd.Flags().String("run-agent", "", "")
	cmd.Flags().String("artifact", "", "")
	cmd.Flags().String("redaction", "", "")
	cmd.Flags().String("from-file", "", "")
	return cmd
}

func testDatasetTracePromoteCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("expected", "", "")
	cmd.Flags().String("from-file", "", "")
	cmd.Flags().StringSlice("tag", nil, "")
	return cmd
}
