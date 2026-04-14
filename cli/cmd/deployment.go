package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(deploymentCmd)
	deploymentCmd.AddCommand(deploymentListCmd)
	deploymentCmd.AddCommand(deploymentCreateCmd)

	deploymentCreateCmd.Flags().String("from-file", "", "JSON file with deployment spec")
	deploymentCreateCmd.Flags().String("name", "", "Deployment name")
	deploymentCreateCmd.Flags().String("build-version-id", "", "Agent build version ID")
	deploymentCreateCmd.Flags().String("runtime-profile-id", "", "Runtime profile ID")
	deploymentCreateCmd.Flags().String("provider-account-id", "", "Provider account ID")
	deploymentCreateCmd.Flags().String("model-alias-id", "", "Model alias ID")
}

var deploymentCmd = &cobra.Command{
	Use:     "deployment",
	Aliases: []string{"deploy"},
	Short:   "Manage agent deployments",
}

var deploymentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent deployments",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-deployments", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result struct {
			Items []map[string]any `json:"items"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Status"}, {Header: "Build Version"}, {Header: "Created"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				output.StatusColor(str(item["status"])),
				str(item["agent_build_version_id"]),
				str(item["created_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var deploymentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an agent deployment",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		var body map[string]any

		if fromFile, _ := cmd.Flags().GetString("from-file"); fromFile != "" {
			data, err := os.ReadFile(fromFile)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			if err := json.Unmarshal(data, &body); err != nil {
				return fmt.Errorf("parsing file: %w", err)
			}
		} else {
			body = make(map[string]any)
			setFlagIfChanged(cmd, body, "name", "name")
			setFlagIfChanged(cmd, body, "build-version-id", "agent_build_version_id")
			setFlagIfChanged(cmd, body, "runtime-profile-id", "runtime_profile_id")
			setFlagIfChanged(cmd, body, "provider-account-id", "provider_account_id")
			setFlagIfChanged(cmd, body, "model-alias-id", "model_alias_id")
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-deployments", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var deployment map[string]any
		if err := resp.DecodeJSON(&deployment); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(deployment)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Created deployment %s (%s)", str(deployment["name"]), str(deployment["id"])))
		return nil
	},
}

func setFlagIfChanged(cmd *cobra.Command, body map[string]any, flagName, jsonKey string) {
	if cmd.Flags().Changed(flagName) {
		v, _ := cmd.Flags().GetString(flagName)
		body[jsonKey] = v
	}
}
