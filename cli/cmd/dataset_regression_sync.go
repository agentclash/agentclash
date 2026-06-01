package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	datasetCmd.AddCommand(datasetSyncRegressionSuiteCmd)
	datasetSyncRegressionSuiteCmd.Flags().String("version", "", "Dataset version ID to sync")
	datasetSyncRegressionSuiteCmd.Flags().String("pack", "", "Challenge pack version ID")
	datasetSyncRegressionSuiteCmd.Flags().String("challenge", "", "Challenge key")
	datasetSyncRegressionSuiteCmd.Flags().String("suite", "", "Existing regression suite ID (optional)")
	datasetSyncRegressionSuiteCmd.Flags().String("suite-name", "", "Name for a newly created regression suite")
	datasetSyncRegressionSuiteCmd.Flags().String("format", "text", "Output format: text or json")
}

var datasetSyncRegressionSuiteCmd = &cobra.Command{
	Use:   "sync-regression-suite <datasetId>",
	Short: "Promote dataset examples into a linked regression suite",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		datasetID := args[0]

		versionID, err := cmd.Flags().GetString("version")
		if err != nil {
			return err
		}
		packID, err := cmd.Flags().GetString("pack")
		if err != nil {
			return err
		}
		challengeKey, err := cmd.Flags().GetString("challenge")
		if err != nil {
			return err
		}
		if versionID == "" || packID == "" || challengeKey == "" {
			return fmt.Errorf("--version, --pack, and --challenge are required")
		}

		body := map[string]any{
			"version_id":                versionID,
			"challenge_pack_version_id": packID,
			"challenge_key":             challengeKey,
		}
		if cmd.Flags().Changed("suite") {
			value, _ := cmd.Flags().GetString("suite")
			body["regression_suite_id"] = value
		}
		if cmd.Flags().Changed("suite-name") {
			value, _ := cmd.Flags().GetString("suite-name")
			body["suite_name"] = value
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+datasetID+"/regression-suite/sync", body)
		if err != nil {
			return err
		}
		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			var result map[string]any
			if err := resp.DecodeJSON(&result); err != nil {
				return err
			}
			return rc.Output.PrintRaw(result)
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}
		suite := mapObject(result, "suite")
		rc.Output.PrintSuccess(fmt.Sprintf(
			"Synced dataset regression suite %s (created=%s skipped=%s total=%s)",
			mapString(suite, "name"),
			mapString(result, "created_cases"),
			mapString(result, "skipped_cases"),
			mapString(result, "total_examples"),
		))
		return nil
	},
}
