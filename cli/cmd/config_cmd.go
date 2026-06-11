package cmd

import (
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long:  "Get, set, and list configuration values.\n\nValid keys: " + strings.Join(config.Keys(), ", "),
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		key := args[0]
		// Reject an unknown key up front, the same way `config set` does, so a
		// typo can never read as "valid but unset". (A corrupt config file is
		// already surfaced earlier by PersistentPreRunE's own load — this
		// guards the key, not the file.)
		if !isValidConfigKey(key) {
			return &cliError{
				Code:    "invalid_argument",
				Message: fmt.Sprintf("unknown config key %q. Valid keys: %s", key, strings.Join(config.Keys(), ", ")),
			}
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		val := cfg.Get(key)

		if rc != nil && rc.Output.IsStructured() {
			// An unset key is data, not an error: {"key","value":null}, exit 0.
			payload := map[string]any{"key": key, "value": nil}
			if val != "" {
				payload["value"] = val
			}
			return rc.Output.PrintRaw(payload)
		}

		if val == "" {
			return fmt.Errorf("key %q is not set", key)
		}
		fmt.Println(val)
		return nil
	},
}

func isValidConfigKey(key string) bool {
	for _, k := range config.Keys() {
		if k == key {
			return true
		}
	}
	return false
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := cfg.Set(args[0], args[1]); err != nil {
			return fmt.Errorf("%w\nValid keys: %s", err, strings.Join(config.Keys(), ", "))
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		rc := GetRunContext(cmd)
		if rc != nil {
			rc.Output.PrintSuccess(fmt.Sprintf("Set %s = %s", args[0], args[1]))
		}
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all config values",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if rc != nil && rc.Output.IsStructured() {
			return rc.Output.PrintRaw(map[string]string{
				"default_workspace": cfg.DefaultWorkspace,
				"default_org":      cfg.DefaultOrg,
				"api_url":          cfg.APIURL,
				"output":           cfg.Output,
			})
		}

		for _, key := range config.Keys() {
			val := cfg.Get(key)
			if val == "" {
				val = "(not set)"
			}
			fmt.Printf("%-20s %s\n", key, val)
		}
		return nil
	},
}
