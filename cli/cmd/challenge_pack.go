package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(challengePackCmd)
	challengePackCmd.AddCommand(cpListCmd)
	challengePackCmd.AddCommand(cpPublishCmd)
	challengePackCmd.AddCommand(cpValidateCmd)
}

var challengePackCmd = &cobra.Command{
	Use:     "challenge-pack",
	Aliases: []string{"cp"},
	Short:   "Manage challenge packs",
}

var cpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List challenge packs",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/challenge-packs", nil)
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

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Slug"}, {Header: "Status"}, {Header: "Versions"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			versionCount := "0"
			if versions, ok := item["versions"].([]any); ok {
				versionCount = fmt.Sprintf("%d", len(versions))
			}
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				str(item["slug"]),
				output.StatusColor(str(item["lifecycle_status"])),
				versionCount,
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var cpPublishCmd = &cobra.Command{
	Use:   "publish <file>",
	Short: "Publish a challenge pack YAML bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		sp := output.NewSpinner("Publishing challenge pack...", flagQuiet)
		resp, err := rc.Client.PostRaw(cmd.Context(), "/v1/workspaces/"+wsID+"/challenge-packs", "application/octet-stream", strings.NewReader(string(data)))
		if err != nil {
			sp.StopWithError("Publish failed")
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			sp.StopWithError("Publish failed")
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		sp.StopWithSuccess("Published")

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		rc.Output.PrintDetail("Pack ID", str(result["challenge_pack_id"]))
		rc.Output.PrintDetail("Version ID", str(result["challenge_pack_version_id"]))
		return nil
	},
}

var cpValidateCmd = &cobra.Command{
	Use:   "validate <file>",
	Short: "Validate a challenge pack YAML bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		resp, err := rc.Client.PostRaw(cmd.Context(), "/v1/workspaces/"+wsID+"/challenge-packs/validate", "application/octet-stream", strings.NewReader(string(data)))
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		if valid, ok := result["valid"].(bool); ok && valid {
			rc.Output.PrintSuccess("Challenge pack is valid")
		} else {
			rc.Output.PrintError("Challenge pack has errors")
			if errors, ok := result["errors"].([]any); ok {
				for _, e := range errors {
					fmt.Fprintf(os.Stderr, "  - %v\n", e)
				}
			}
			return fmt.Errorf("validation failed")
		}
		return nil
	},
}
