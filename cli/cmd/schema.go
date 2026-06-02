package cmd

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// schemaDocVersion is the version of the schema DOCUMENT shape (not the CLI).
// Bump it when the JSON structure below changes incompatibly so consumers can
// detect breaking changes independent of the CLI version.
const schemaDocVersion = 1

type flagSchema struct {
	Name      string `json:"name" yaml:"name"`
	Shorthand string `json:"shorthand,omitempty" yaml:"shorthand,omitempty"`
	Usage     string `json:"usage" yaml:"usage"`
	Type      string `json:"type" yaml:"type"`
	Default   string `json:"default,omitempty" yaml:"default,omitempty"`
	Required  bool   `json:"required,omitempty" yaml:"required,omitempty"`
}

type commandSchema struct {
	Name        string          `json:"name" yaml:"name"`
	Path        string          `json:"path" yaml:"path"`
	Short       string          `json:"short" yaml:"short"`
	Aliases     []string        `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	Runnable    bool            `json:"runnable" yaml:"runnable"`
	Flags       []flagSchema    `json:"flags,omitempty" yaml:"flags,omitempty"`
	Subcommands []commandSchema `json:"subcommands,omitempty" yaml:"subcommands,omitempty"`
}

type cliSchema struct {
	SchemaVersion int             `json:"schema_version" yaml:"schema_version"`
	CLIVersion    string          `json:"cli_version" yaml:"cli_version"`
	GlobalFlags   []flagSchema    `json:"global_flags" yaml:"global_flags"`
	Commands      []commandSchema `json:"commands" yaml:"commands"`
	ExitCodes     []ExitCode      `json:"exit_codes" yaml:"exit_codes"`
}

// flagsOf serialises a flag set deterministically, skipping hidden flags and any
// flag whose name is in skip (used to keep the global persistent flags from being
// repeated on every command).
func flagsOf(fs *pflag.FlagSet, skip map[string]bool) []flagSchema {
	var out []flagSchema
	fs.VisitAll(func(f *pflag.Flag) {
		// Skip the `--help` flag cobra injects onto every command during
		// Execute() — it's implicit and would make the schema differ depending
		// on whether Execute has run (it has, in the full test suite).
		if f.Hidden || f.Name == "help" || (skip != nil && skip[f.Name]) {
			return
		}
		out = append(out, flagSchema{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Usage:     f.Usage,
			Type:      f.Value.Type(),
			Default:   f.DefValue,
			Required:  f.Annotations[cobra.BashCompOneRequiredFlag] != nil,
		})
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// skipCommand filters out commands that should not appear in the machine schema:
// hidden commands, additional help topics, and the auto-generated help/completion
// commands (which cobra adds lazily during Execute, so excluding them keeps the
// schema stable whether or not Execute has run).
func skipCommand(c *cobra.Command) bool {
	if c.Hidden || c.IsAdditionalHelpTopicCommand() {
		return true
	}
	switch c.Name() {
	case "help", "completion":
		return true
	}
	return false
}

func commandsOf(parent *cobra.Command, globals map[string]bool) []commandSchema {
	var out []commandSchema
	for _, c := range parent.Commands() {
		if skipCommand(c) {
			continue
		}
		out = append(out, commandSchema{
			Name:        c.Name(),
			Path:        c.CommandPath(),
			Short:       c.Short,
			Aliases:     c.Aliases,
			Runnable:    c.Runnable(),
			Flags:       flagsOf(c.LocalFlags(), globals),
			Subcommands: commandsOf(c, globals),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// buildCLISchema walks the command tree and returns a deterministic, machine-
// readable description of every command, its flags, and the documented exit
// codes. version is injected (rather than read from cliVersion) so tests can pin
// a stable value for the golden snapshot.
func buildCLISchema(root *cobra.Command, version string) cliSchema {
	globalFlags := flagsOf(root.PersistentFlags(), nil)
	globalNames := make(map[string]bool, len(globalFlags))
	for _, f := range globalFlags {
		globalNames[f.Name] = true
	}
	return cliSchema{
		SchemaVersion: schemaDocVersion,
		CLIVersion:    version,
		GlobalFlags:   globalFlags,
		Commands:      commandsOf(root, globalNames),
		ExitCodes:     documentedExitCodes,
	}
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the CLI command tree as machine-readable JSON",
	Long: `Print the full command tree — every command, its flags (with types and
defaults), and the documented process exit codes — as a single machine-readable
JSON document, so agents and scripts can introspect the CLI without parsing
--help text.

Output is always structured (JSON by default, or YAML with --output yaml); the
format is NOT auto-switched based on whether stdout is a pipe — pass --json or
--output explicitly, consistent with every other AgentClash command.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		doc := buildCLISchema(rootCmd, cliVersion)
		if rc := GetRunContext(cmd); rc != nil {
			return rc.Output.PrintRaw(doc)
		}
		// Defensive fallback (RunContext is always set via PersistentPreRunE).
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	},
}
