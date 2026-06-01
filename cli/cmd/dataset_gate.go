package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	datasetCmd.AddCommand(datasetTestCmd)
	datasetTestCmd.Flags().String("baseline", "", "Baseline ID to compare against")
	datasetTestCmd.Flags().String("run", "", "Candidate run ID (required unless --eval is set)")
	datasetTestCmd.Flags().Bool("eval", false, "Start a dataset eval before gating")
	datasetTestCmd.Flags().String("version", "", "Dataset version ID for eval")
	datasetTestCmd.Flags().String("pack", "", "Challenge pack version ID for eval")
	datasetTestCmd.Flags().String("challenge", "", "Challenge key for eval")
	datasetTestCmd.Flags().StringSlice("deployment", nil, "Agent deployment ID (repeatable)")
	datasetTestCmd.Flags().Float64("min-pass-rate", 0, "Minimum pass rate required to pass the gate")
	datasetTestCmd.Flags().Int("max-regressions", -1, "Maximum allowed regressions versus baseline")
	datasetTestCmd.Flags().String("format", "text", "Output format: text or json")
	datasetTestCmd.Flags().Duration("timeout", 30*time.Minute, "Maximum time to wait for an eval run started with --eval")
	datasetTestCmd.Flags().Duration("poll-interval", 5*time.Second, "Polling interval while waiting for eval completion")
}

var datasetTestCmd = &cobra.Command{
	Use:   "test <datasetId>",
	Short: "Run a dataset eval gate against a baseline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		datasetID := args[0]

		runID, err := cmd.Flags().GetString("run")
		if err != nil {
			return err
		}
		runEval, err := cmd.Flags().GetBool("eval")
		if err != nil {
			return err
		}
		if runID == "" && !runEval {
			return fmt.Errorf("either --run or --eval is required")
		}
		if runEval {
			evalBody, err := datasetEvalBody(cmd)
			if err != nil {
				return err
			}
			resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+datasetID+"/evals", evalBody)
			if err != nil {
				return err
			}
			if apiErr := resp.ParseError(); apiErr != nil {
				return apiErr
			}
			var evalResult map[string]any
			if err := resp.DecodeJSON(&evalResult); err != nil {
				return err
			}
			run := mapObject(evalResult, "run")
			if run == nil {
				return fmt.Errorf("dataset eval response missing run")
			}
			runID = str(run["id"])
			if runID == "" {
				return fmt.Errorf("dataset eval response missing run id")
			}
			timeout, err := cmd.Flags().GetDuration("timeout")
			if err != nil {
				return err
			}
			pollInterval, err := cmd.Flags().GetDuration("poll-interval")
			if err != nil {
				return err
			}
			if _, err := waitForCIRunCompletion(cmd, rc, runID, timeout, pollInterval); err != nil {
				return err
			}
		}

		baselineID, err := cmd.Flags().GetString("baseline")
		if err != nil {
			return err
		}
		if baselineID == "" {
			return fmt.Errorf("--baseline is required")
		}

		body := map[string]any{
			"baseline_id": baselineID,
			"run_id":      runID,
		}
		if cmd.Flags().Changed("min-pass-rate") {
			value, _ := cmd.Flags().GetFloat64("min-pass-rate")
			body["min_pass_rate"] = value
		}
		if cmd.Flags().Changed("max-regressions") {
			value, _ := cmd.Flags().GetInt("max-regressions")
			body["max_regressions"] = value
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+datasetID+"/gate", body)
		if err != nil {
			return err
		}
		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			if apiErr := resp.ParseError(); apiErr != nil {
				return apiErr
			}
			var result map[string]any
			if err := resp.DecodeJSON(&result); err != nil {
				return err
			}
			return rc.Output.PrintRaw(result)
		}

		if resp.StatusCode == 422 {
			var result map[string]any
			_ = resp.DecodeJSON(&result)
			printDatasetGateFailure(result)
			os.Exit(1)
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}
		gate := mapObject(result, "gate")
		if gate == nil {
			return fmt.Errorf("gate response missing gate payload")
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Dataset gate passed (pass_rate=%s regressions=%s)",
			mapString(gate, "pass_rate"), mapString(gate, "regression_count")))
		return nil
	},
}

func printDatasetGateFailure(result map[string]any) {
	gate := mapObject(result, "gate")
	if gate == nil {
		fmt.Fprintln(os.Stderr, "dataset gate failed")
		return
	}
	fmt.Fprintf(os.Stderr, "dataset gate failed: pass_rate=%s baseline_pass_rate=%s regressions=%s\n",
		mapString(gate, "pass_rate"), mapString(gate, "baseline_pass_rate"), mapString(gate, "regression_count"))
	regressions := mapSlice(gate, "regressions")
	for _, item := range regressions {
		row, _ := item.(map[string]any)
		if row == nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "  - example %s (%s)\n", mapString(row, "dataset_example_id"), mapString(row, "reason"))
	}
}
