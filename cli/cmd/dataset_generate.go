package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	datasetCmd.AddCommand(datasetGenerateCmd)
	datasetGenerateCmd.Flags().String("strategy", "self-instruct", "Generation strategy (v1: self-instruct)")
	datasetGenerateCmd.Flags().Int("count", 0, "Target number of accepted synthetic examples")
	datasetGenerateCmd.Flags().String("provider-account", "", "Provider account ID")
	datasetGenerateCmd.Flags().String("model-alias", "", "Model alias ID")
	datasetGenerateCmd.Flags().String("seeds-tag", "", "Only use seed examples with this tag")
	datasetGenerateCmd.Flags().Bool("create-version", false, "Snapshot a dataset version when generation completes")
	datasetGenerateCmd.Flags().String("version-label", "", "Optional label for the generated dataset version")
	datasetGenerateCmd.Flags().Bool("follow", false, "Poll generation job status until it finishes")
	datasetGenerateCmd.Flags().String("format", "text", "Output format: text or json")
}

var datasetGenerateCmd = &cobra.Command{
	Use:   "generate <datasetId>",
	Short: "Start in-house synthetic dataset generation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		datasetID := args[0]

		count, err := cmd.Flags().GetInt("count")
		if err != nil {
			return err
		}
		providerAccount, err := cmd.Flags().GetString("provider-account")
		if err != nil {
			return err
		}
		modelAlias, err := cmd.Flags().GetString("model-alias")
		if err != nil {
			return err
		}
		if count <= 0 || providerAccount == "" || modelAlias == "" {
			return fmt.Errorf("--count, --provider-account, and --model-alias are required")
		}

		strategy, _ := cmd.Flags().GetString("strategy")
		seedsTag, _ := cmd.Flags().GetString("seeds-tag")
		createVersion, _ := cmd.Flags().GetBool("create-version")
		versionLabel, _ := cmd.Flags().GetString("version-label")

		body := map[string]any{
			"strategy":            strategy,
			"target_count":        count,
			"provider_account_id": providerAccount,
			"model_alias_id":      modelAlias,
			"create_version":      createVersion,
		}
		if seedsTag != "" {
			body["seeds_tag"] = seedsTag
		}
		if versionLabel != "" {
			body["version_label"] = versionLabel
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+datasetID+"/generate", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var job map[string]any
		if err := resp.DecodeJSON(&job); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(job)
		}

		jobID, _ := job["id"].(string)
		fmt.Fprintf(rc.Output.Writer(), "generation job %s queued\n", jobID)

		follow, _ := cmd.Flags().GetBool("follow")
		if !follow || jobID == "" {
			return nil
		}

		for {
			statusResp, pollErr := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+datasetID+"/generations/"+jobID, nil)
			if pollErr != nil {
				return pollErr
			}
			if apiErr := statusResp.ParseError(); apiErr != nil {
				return apiErr
			}
			var current map[string]any
			if err := statusResp.DecodeJSON(&current); err != nil {
				return err
			}
			status, _ := current["status"].(string)
			fmt.Fprintf(rc.Output.Writer(), "status: %s accepted=%v rejected=%v\n", status, current["accepted_count"], current["rejected_count"])
			switch status {
			case "completed", "failed":
				if rc.Output.IsStructured() {
					return rc.Output.PrintRaw(current)
				}
				return nil
			}
			time.Sleep(2 * time.Second)
		}
	},
}
