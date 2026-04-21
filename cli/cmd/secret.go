package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	rootCmd.AddCommand(secretCmd)
	secretCmd.AddCommand(secretListCmd)
	secretCmd.AddCommand(secretSetCmd)
	secretCmd.AddCommand(secretDeleteCmd)

	secretSetCmd.Flags().String("value", "", "Secret value (reads from stdin if omitted)")
}

var readSecretPassword = term.ReadPassword

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

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
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
		value, err := readSecretSetValue(value, cmd.Flags().Changed("value"), os.Stdin, os.Stderr)
		if err != nil {
			return err
		}
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

func readSecretSetValue(flagValue string, flagProvided bool, stdin *os.File, stderr io.Writer) (string, error) {
	if flagProvided {
		return flagValue, nil
	}

	stat, err := stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	return readSecretSetValueFromInput(stdin, int(stdin.Fd()), (stat.Mode()&os.ModeCharDevice) != 0, stderr)
}

func readSecretSetValueFromInput(stdin io.Reader, stdinFD int, isTerminal bool, stderr io.Writer) (string, error) {
	if !isTerminal {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("reading secret value from stdin: %w", err)
		}
		return string(data), nil
	}

	fmt.Fprint(stderr, "Enter secret value: ")
	data, err := readSecretPassword(stdinFD)
	fmt.Fprintln(stderr)
	if err != nil {
		return "", fmt.Errorf("reading secret value: %w", err)
	}
	return string(data), nil
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
