package cmd

import (
	"fmt"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/config"
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
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		val := cfg.Get(args[0])
		if val == "" {
			return fmt.Errorf("key %q is not set", args[0])
		}
		fmt.Println(val)
		return nil
	},
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
		if !cfg.Set(args[0], args[1]) {
			return fmt.Errorf("unknown config key %q\nValid keys: %s", args[0], strings.Join(config.Keys(), ", "))
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

		if rc != nil && rc.Output.IsJSON() {
			return rc.Output.PrintJSON(map[string]string{
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
