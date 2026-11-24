package cmd

import (
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	datasetCmd.AddCommand(datasetBaselineCmd)
	datasetBaselineCmd.AddCommand(datasetBaselineCreateCmd)
	datasetBaselineCmd.AddCommand(datasetBaselineListCmd)

	datasetBaselineCreateCmd.Flags().String("run", "", "Completed dataset eval run ID to freeze as the baseline")
	datasetBaselineCreateCmd.Flags().String("deployment", "", "Agent deployment ID; keep only the outcomes of this deployment")
	datasetBaselineCreateCmd.Flags().String("label", "", "Optional label, for example the release name")
	datasetBaselineCreateCmd.MarkFlagRequired("run")

	datasetBaselineListCmd.Flags().Int("limit", 0, "Maximum baselines to return (1-100; 0 = server default of 50)")
	datasetBaselineListCmd.Flags().Int("offset", 0, "Number of baselines to skip before the first result")
}

var datasetBaselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Manage server-side dataset baselines for the dataset gate",
	Long: `Manage server-side dataset baselines for the dataset gate.

A dataset baseline freezes the per-example outcomes of one completed dataset
eval run. 'agentclash dataset test' compares a candidate run with it.

This is not the same as 'agentclash baseline set'. That command saves a
local bookmark of a run in your CLI config. A dataset baseline is a server
record that every workspace member and every CI job can use.`,
}

var datasetBaselineCreateCmd = &cobra.Command{
	Use:   "create <datasetId>",
	Short: "Freeze a completed dataset eval run as a dataset baseline",
	Long: `Freeze a completed dataset eval run as a dataset baseline.

The run must come from 'agentclash dataset eval' on the same dataset, and it
must be completed. Each call adds a new baseline record; the server does not
replace an older one.`,
	Example: `  # Freeze the run of the current release
  agentclash dataset baseline create <datasetId> --run <runId> --label v1.4.0

  # Keep only the outcomes of one deployment of a comparison run
  agentclash dataset baseline create <datasetId> --run <runId> --deployment <deploymentId>

  # Print only the new baseline ID
  agentclash dataset baseline create <datasetId> --run <runId> --json --query '.id'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		datasetID := args[0]
		runID, _ := cmd.Flags().GetString("run")
		deploymentID, _ := cmd.Flags().GetString("deployment")
		label, _ := cmd.Flags().GetString("label")

		if err := requireUUID("dataset ID", datasetID); err != nil {
			return err
		}
		if err := requireUUID("--run", runID); err != nil {
			return err
		}
		if cmd.Flags().Changed("deployment") {
			if err := requireUUID("--deployment", deploymentID); err != nil {
				return err
			}
		}
		wsID := RequireWorkspace(cmd)

		body := map[string]any{"run_id": runID}
		if deploymentID != "" {
			body["agent_deployment_id"] = deploymentID
		}
		if strings.TrimSpace(label) != "" {
			body["label"] = label
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+datasetID+"/baselines", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		baselineID := str(result["id"])
		rc.Output.PrintSuccess(fmt.Sprintf("Created dataset baseline %s", baselineID))
		renderDatasetBaselineDetail(rc, result)
		rc.Output.PrintDetail("Next", fmt.Sprintf("agentclash dataset test %s --baseline %s --run <candidate-run-id>", datasetID, baselineID))
		return nil
	},
}

var datasetBaselineListCmd = &cobra.Command{
	Use:   "list <datasetId>",
	Short: "List the dataset baselines of a dataset",
	Example: `  agentclash dataset baseline list <datasetId>

  # Page through the baselines
  agentclash dataset baseline list <datasetId> --limit 20 --offset 20

  # IDs only, newest first
  agentclash dataset baseline list <datasetId> --json --query '.items[].id'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		datasetID := args[0]
		if err := requireUUID("dataset ID", datasetID); err != nil {
			return err
		}
		q, err := listPaginationQuery(cmd)
		if err != nil {
			return err
		}
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+datasetID+"/baselines", q)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var result struct {
			Items  []map[string]any `json:"items"`
			Total  int64            `json:"total"`
			Limit  int              `json:"limit"`
			Offset int              `json:"offset"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}
		list := newPaginatedList(result.Items, result.Total, result.Limit, result.Offset)
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(list)
		}
		renderDatasetBaselinesTable(rc, result.Items)
		printListPagingHint(rc, list)
		return nil
	},
}

func renderDatasetBaselineDetail(rc *RunContext, baseline map[string]any) {
	rc.Output.PrintDetail("ID", str(baseline["id"]))
	rc.Output.PrintDetail("Dataset Version", str(baseline["dataset_version_id"]))
	rc.Output.PrintDetail("Run", str(baseline["run_id"]))
	rc.Output.PrintDetail("Deployment", fallbackDisplay(str(baseline["agent_deployment_id"]), "-"))
	rc.Output.PrintDetail("Pass Rate", fmtPercent(baseline["pass_rate"]))
	rc.Output.PrintDetail("Examples", datasetBaselineExampleCount(baseline))
	rc.Output.PrintDetail("Label", fallbackDisplay(str(baseline["label"]), "-"))
	rc.Output.PrintDetail("Created", str(baseline["created_at"]))
}

func renderDatasetBaselinesTable(rc *RunContext, items []map[string]any) {
	cols := []output.Column{{Header: "ID"}, {Header: "Label"}, {Header: "Run"}, {Header: "Deployment"}, {Header: "Pass Rate"}, {Header: "Examples"}, {Header: "Created"}}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = []string{
			str(item["id"]),
			fallbackDisplay(str(item["label"]), "-"),
			str(item["run_id"]),
			fallbackDisplay(str(item["agent_deployment_id"]), "-"),
			fmtPercent(item["pass_rate"]),
			datasetBaselineExampleCount(item),
			str(item["created_at"]),
		}
	}
	rc.Output.PrintTable(cols, rows)
}

// datasetBaselineExampleCount reads metrics.example_count, which the server
// writes when it freezes the baseline.
func datasetBaselineExampleCount(baseline map[string]any) string {
	if metrics := mapObject(baseline, "metrics"); metrics != nil {
		if count := mapString(metrics, "example_count"); count != "" {
			return count
		}
	}
	return "-"
}
