package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// schemaDocVersion is the version of the schema DOCUMENT shape (not the CLI).
// Bump it when the JSON structure below changes incompatibly so consumers can
// detect breaking changes independent of the CLI version.
//
// v2: positional args, per-flag allowed_values, command examples, and the
// top-level error_codes and status_enums registries.
const schemaDocVersion = 2

type flagSchema struct {
	Name      string `json:"name" yaml:"name"`
	Shorthand string `json:"shorthand,omitempty" yaml:"shorthand,omitempty"`
	Usage     string `json:"usage" yaml:"usage"`
	Type      string `json:"type" yaml:"type"`
	Default   string `json:"default,omitempty" yaml:"default,omitempty"`
	Required  bool   `json:"required,omitempty" yaml:"required,omitempty"`
	// AllowedValues is the closed value set for enum-like flags, populated
	// from flagAllowedValues — only flags with an authoritative, validated
	// set are listed; absence means "free-form", not "unknown".
	AllowedValues []string `json:"allowed_values,omitempty" yaml:"allowed_values,omitempty"`
}

// argSchema describes one positional argument, parsed from the command's Use
// line: `<x>` is required, `[x]` is optional, and a trailing `...` marks the
// argument as variadic.
type argSchema struct {
	Name     string `json:"name" yaml:"name"`
	Required bool   `json:"required" yaml:"required"`
	Variadic bool   `json:"variadic,omitempty" yaml:"variadic,omitempty"`
}

type commandSchema struct {
	Name        string          `json:"name" yaml:"name"`
	Path        string          `json:"path" yaml:"path"`
	Short       string          `json:"short" yaml:"short"`
	Aliases     []string        `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	Runnable    bool            `json:"runnable" yaml:"runnable"`
	Args        []argSchema     `json:"args,omitempty" yaml:"args,omitempty"`
	Example     string          `json:"example,omitempty" yaml:"example,omitempty"`
	Flags       []flagSchema    `json:"flags,omitempty" yaml:"flags,omitempty"`
	Subcommands []commandSchema `json:"subcommands,omitempty" yaml:"subcommands,omitempty"`
}

type cliSchema struct {
	SchemaVersion int             `json:"schema_version" yaml:"schema_version"`
	CLIVersion    string          `json:"cli_version" yaml:"cli_version"`
	GlobalFlags   []flagSchema    `json:"global_flags" yaml:"global_flags"`
	Commands      []commandSchema `json:"commands" yaml:"commands"`
	ExitCodes     []ExitCode      `json:"exit_codes" yaml:"exit_codes"`
	ErrorCodes    []ErrorCode     `json:"error_codes" yaml:"error_codes"`
	StatusEnums   []StatusEnum    `json:"status_enums" yaml:"status_enums"`
}

// flagAllowedValues registers the closed value sets for enum-like flags.
// Key "" scopes a global persistent flag; a command path (e.g.
// "agentclash run create") scopes a local flag. Only add entries whose set is
// authoritative (enforced by validation or the backend) — a speculative or
// stale enum is worse for an agent than none.
var flagAllowedValues = map[string]map[string][]string{
	"": {
		"output": {"table", "json", "yaml"},
	},
	"agentclash run create": {
		"scope": {"full", "suite_only"},
	},
}

// parseArgsFromUse extracts positional arguments from a cobra Use line.
// Tokens after the command name follow the conventional grammar:
// `<name>` required, `[name]` optional, trailing `...` (inside or outside the
// brackets) variadic. Plain tokens (subcommand words) are ignored.
func parseArgsFromUse(use string) []argSchema {
	// Normalize "<x> ..." / "[x ...]" so the ellipsis stays attached to its
	// argument token through Fields.
	use = strings.ReplaceAll(use, " ...", "...")
	fields := strings.Fields(use)
	if len(fields) < 2 {
		return nil
	}
	var out []argSchema
	for _, tok := range fields[1:] {
		variadic := strings.Contains(tok, "...")
		tok = strings.ReplaceAll(tok, "...", "")
		var name string
		var required bool
		switch {
		case strings.HasPrefix(tok, "<") && strings.HasSuffix(tok, ">"):
			name, required = tok[1:len(tok)-1], true
		case strings.HasPrefix(tok, "[") && strings.HasSuffix(tok, "]"):
			name, required = tok[1:len(tok)-1], false
		default:
			continue
		}
		if name == "" {
			continue
		}
		out = append(out, argSchema{Name: name, Required: required, Variadic: variadic})
	}
	return out
}

// flagsOf serialises a flag set deterministically, skipping hidden flags and any
// flag whose name is in skip (used to keep the global persistent flags from being
// repeated on every command). scope selects the flagAllowedValues entry: "" for
// globals, the command path for local flags.
func flagsOf(fs *pflag.FlagSet, skip map[string]bool, scope string) []flagSchema {
	enums := flagAllowedValues[scope]
	var out []flagSchema
	fs.VisitAll(func(f *pflag.Flag) {
		// Skip the `--help` flag cobra injects onto every command during
		// Execute() — it's implicit and would make the schema differ depending
		// on whether Execute has run (it has, in the full test suite).
		if f.Hidden || f.Name == "help" || (skip != nil && skip[f.Name]) {
			return
		}
		out = append(out, flagSchema{
			Name:          f.Name,
			Shorthand:     f.Shorthand,
			Usage:         f.Usage,
			Type:          f.Value.Type(),
			Default:       f.DefValue,
			Required:      f.Annotations[cobra.BashCompOneRequiredFlag] != nil,
			AllowedValues: enums[f.Name],
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
			Args:        parseArgsFromUse(c.Use),
			Example:     c.Example,
			Flags:       flagsOf(c.LocalFlags(), globals, c.CommandPath()),
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
	globalFlags := flagsOf(root.PersistentFlags(), nil, "")
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
		ErrorCodes:    documentedErrorCodes,
		StatusEnums:   documentedStatusEnums,
	}
}

// findSchemaSubtree resolves a command path (e.g. ["run", "get"]) against the
// schema tree, matching names and aliases.
func findSchemaSubtree(cmds []commandSchema, path []string) *commandSchema {
	if len(path) == 0 {
		return nil
	}
	for i := range cmds {
		if !schemaNameMatches(&cmds[i], path[0]) {
			continue
		}
		if len(path) == 1 {
			return &cmds[i]
		}
		return findSchemaSubtree(cmds[i].Subcommands, path[1:])
	}
	return nil
}

func schemaNameMatches(c *commandSchema, name string) bool {
	if c.Name == name {
		return true
	}
	for _, alias := range c.Aliases {
		if alias == name {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}

var schemaCmd = &cobra.Command{
	Use:   "schema [command ...]",
	Short: "Print the CLI command tree as machine-readable JSON",
	Long: `Print the full command tree — every command, its positional args and flags
(with types, defaults, and enum values), the documented process exit codes,
the CLI-local error codes, and the resource status enums — as a single
machine-readable JSON document, so agents and scripts can introspect the CLI
without parsing --help text.

Pass a command path to print just that subtree:
  agentclash schema run get

Output is always structured (JSON by default, or YAML with --output yaml); the
format is NOT auto-switched based on whether stdout is a pipe — pass --json or
--output explicitly, consistent with every other AgentClash command.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		doc := buildCLISchema(rootCmd, cliVersion)

		var payload any = doc
		if len(args) > 0 {
			sub := findSchemaSubtree(doc.Commands, args)
			if sub == nil {
				return &cliError{
					Code:    "invalid_argument",
					Message: fmt.Sprintf("unknown command path %q; run `agentclash schema` for the full tree", strings.Join(args, " ")),
				}
			}
			payload = sub
		}

		if rc := GetRunContext(cmd); rc != nil {
			return rc.Output.PrintRaw(payload)
		}
		// Defensive fallback (RunContext is always set via PersistentPreRunE).
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	},
}
