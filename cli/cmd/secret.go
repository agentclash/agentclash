package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(secretCmd)
	secretCmd.AddCommand(secretListCmd)
	secretCmd.AddCommand(secretSetCmd)
	secretCmd.AddCommand(secretDeleteCmd)

	secretSetCmd.Flags().String("value", "", "Secret value (reads from stdin if omitted)")
}

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage workspace secrets",
}

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspace secret keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/secrets", nil)
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

		cols := []output.Column{{Header: "Key"}, {Header: "Created"}, {Header: "Updated"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{str(item["key"]), str(item["created_at"]), str(item["updated_at"])}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var secretSetCmd = &cobra.Command{
	Use:   "set <key>",
	Short: "Create or update a secret",
	Long:  "Set a workspace secret. Value can be provided via --value flag\nor piped via stdin (e.g., echo $SECRET | agentclash secret set KEY).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		value, _ := cmd.Flags().GetString("value")
		if value == "" {
			// Read from stdin.
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					value = scanner.Text()
				}
			} else {
				fmt.Fprint(os.Stderr, "Enter secret value: ")
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					value = scanner.Text()
				}
			}
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("secret value cannot be empty")
		}

		body := map[string]any{"value": value}
		resp, err := rc.Client.Put(cmd.Context(), "/v1/workspaces/"+wsID+"/secrets/"+args[0], body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Secret %s set", args[0]))
		return nil
	},
}

var secretDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Delete(cmd.Context(), "/v1/workspaces/"+wsID+"/secrets/"+args[0])
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Secret %s deleted", args[0]))
		return nil
	},
}
