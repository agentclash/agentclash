package cmd

import (
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	evalPackCmd.AddCommand(cpCatalogCmd)
	cpCatalogCmd.AddCommand(cpCatalogListCmd)
	cpCatalogCmd.AddCommand(cpCatalogUseCmd)
}

var cpCatalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Browse and use the curated eval-pack library",
}

var cpCatalogListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the curated, ready-to-run eval packs",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/eval-pack-catalog", nil)
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

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		cols := []output.Column{{Header: "Slug"}, {Header: "Name"}, {Header: "Category"}, {Header: "Family"}, {Header: "Difficulty"}, {Header: "Mode"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["slug"]),
				str(item["name"]),
				str(item["category"]),
				str(item["family"]),
				str(item["difficulty"]),
				str(item["execution_mode"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var cpCatalogUseCmd = &cobra.Command{
	Use:   "use <slug>",
	Short: "Add a library pack to your workspace and make it runnable",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		slug := args[0]

		sp := output.NewSpinner("Adding template to your workspace...", flagQuiet)
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/eval-pack-catalog/"+slug+"/instantiate", nil)
		if err != nil {
			sp.StopWithError("Failed")
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			sp.StopWithError("Failed")
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			sp.StopWithError("Failed")
			return err
		}

		sp.StopWithSuccess("Added to workspace")

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		if already, ok := result["already_existed"].(bool); ok && already {
			rc.Output.PrintWarning("This template was already in your workspace; returned the existing copy.")
		}
		rc.Output.PrintDetail("Pack ID", str(result["eval_pack_id"]))
		rc.Output.PrintDetail("Version ID", str(result["eval_pack_version_id"]))
		rc.Output.PrintDetail("Run it", "agentclash run create --eval-pack-version-id "+str(result["eval_pack_version_id"]))
		return nil
	},
}
