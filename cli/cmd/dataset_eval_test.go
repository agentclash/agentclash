package cmd

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestDatasetEvalBodyEncodesRequiredFlags(t *testing.T) {
	cmd := testDatasetEvalCommand()
	cmd.Flags().Set("version", "version-1")
	cmd.Flags().Set("pack", "pack-version-1")
	cmd.Flags().Set("challenge", "support")
	cmd.Flags().Set("deployment", "dep-1")
	cmd.Flags().Set("deployment", "dep-2")
	cmd.Flags().Set("name", "Golden eval")
	cmd.Flags().Set("mapping", `{"input_key":"input"}`)

	body, err := datasetEvalBody(cmd)
	if err != nil {
		t.Fatalf("datasetEvalBody() error = %v", err)
	}
	if body["version_id"] != "version-1" || body["eval_pack_version_id"] != "pack-version-1" || body["challenge_id"] != "support" {
		t.Fatalf("body has wrong identifiers: %#v", body)
	}
	deployments, ok := body["agent_deployment_ids"].([]string)
	if !ok || len(deployments) != 2 || deployments[0] != "dep-1" || deployments[1] != "dep-2" {
		t.Fatalf("deployments = %#v, want [dep-1 dep-2]", body["agent_deployment_ids"])
	}
	mappingBytes, _ := json.Marshal(body["mapping"])
	if string(mappingBytes) != `{"input_key":"input"}` {
		t.Fatalf("mapping = %s", mappingBytes)
	}
}

func TestDatasetEvalBodyRequiresFlags(t *testing.T) {
	_, err := datasetEvalBody(testDatasetEvalCommand())
	if err == nil {
		t.Fatal("expected missing flags error")
	}
}

func testDatasetEvalCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("version", "", "")
	cmd.Flags().String("pack", "", "")
	cmd.Flags().String("challenge", "", "")
	cmd.Flags().StringSlice("deployment", nil, "")
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("mapping", "", "")
	return cmd
}
