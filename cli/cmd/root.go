package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/auth"
	"github.com/agentclash/agentclash/cli/internal/config"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Global flags.
var (
	flagJSON           bool
	flagOutput         string
	flagQuiet          bool
	flagVerbose        bool
	flagNoColor        bool
	flagWorkspace      string
	flagAPIURL         string
	flagNonInteractive bool
	flagQuery          string
)

// nonInteractiveMode reports whether the CLI must never prompt for input and
// should fail fast when interactive input would otherwise be required. It is
// driven by the explicit --non-interactive flag, the AGENTCLASH_NONINTERACTIVE
// or CI environment variables, or a non-TTY stdin/stdout. Note: --json/--output
// alone does NOT imply non-interactive — a human at a real terminal can still
// run an interactive flow and ask for a machine-readable result.
func nonInteractiveMode() bool {
	if flagNonInteractive || envTruthy(os.Getenv("AGENTCLASH_NONINTERACTIVE")) || envTruthy(os.Getenv("CI")) {
		return true
	}
	return !stdioIsInteractive()
}

// envTruthy treats empty, "0", "false", "no", and "off" (case-insensitive) as
// false; any other non-empty value is true. This matches the de-facto CI
// convention (e.g. GitHub Actions sets CI=true).
func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// RunContext is passed to all commands via cobra context.
type RunContext struct {
	Client    *api.Client
	Config    *config.Manager
	Output    *output.Formatter
	Workspace string
}

type contextKey struct{}

// GetRunContext retrieves the RunContext from a cobra command.
func GetRunContext(cmd *cobra.Command) *RunContext {
	if v := cmd.Context().Value(contextKey{}); v != nil {
		return v.(*RunContext)
	}
	return nil
}

// progressWriter returns the writer for human progress/diagnostics and whether
// structured output is active. In structured (--json/--output) mode, progress
// goes to stderr so stdout stays a clean machine-readable stream; otherwise it
// goes to stdout. Safe when rc is nil (early-init paths): defaults to stdout.
func progressWriter(rc *RunContext) (io.Writer, bool) {
	if rc == nil {
		return os.Stdout, false
	}
	if rc.Output.IsStructured() {
		return rc.Output.ErrWriter(), true
	}
	return rc.Output.Writer(), false
}

// RequireWorkspace returns the workspace ID or exits with an error.
func RequireWorkspace(cmd *cobra.Command) string {
	rc := GetRunContext(cmd)
	if rc.Workspace == "" {
		err := &cliError{
			Code:    "missing_workspace",
			Message: "no workspace specified. Run 'agentclash link' to choose a default workspace, or pass --workspace, set AGENTCLASH_WORKSPACE, or create .agentclash.yaml with 'agentclash init'.",
		}
		if _, rendered := RenderError(err, os.Stderr); !rendered {
			fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		}
		os.Exit(2)
	}
	return rc.Workspace
}

var rootCmd = &cobra.Command{
	Use:   "agentclash",
	Short: "AgentClash CLI — evaluate, compare, and deploy AI agents",
	Long: `AgentClash CLI is the command-line interface for the AgentClash platform.

Manage organizations, workspaces, agent builds, deployments, evaluation runs,
challenge packs, playgrounds, and infrastructure — all from your terminal.

Get started:
  agentclash auth login                   Log in to your account
  agentclash link                         Choose and save your default workspace
  agentclash quickstart                   Check readiness and show the next command
  agentclash challenge-pack init <file>   Scaffold a challenge pack
  agentclash eval start --follow          Create and follow an evaluation run
  agentclash baseline set                 Bookmark a baseline run
  agentclash eval scorecard               Show scorecards and regression verdicts`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		output.InitColors(flagNoColor)

		// --json always wins over --output, so don't fail a command like
		// `agentclash version --json --output invalid` — the invalid value
		// is dead letter. Only enforce when --output is actually in effect.
		if !flagJSON {
			if err := config.ValidateOutputFormat(flagOutput); err != nil {
				return err
			}
		}

		flags := config.FlagOverrides{
			APIURL:    flagAPIURL,
			Workspace: flagWorkspace,
			Output:    flagOutput,
			JSON:      flagJSON,
		}

		mgr, err := config.NewManager(flags)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Resolve token: env var > stored credentials.
		token := mgr.Token()
		if token == "" {
			creds, _ := auth.LoadCredentials()
			token = creds.Token
		}

		// Build client options.
		var opts []api.Option
		opts = append(opts, api.WithVerbose(flagVerbose))
		// User-Agent carries the dotted command path only when pointed at
		// the hosted backend (api.agentclash.dev). Self-hosted backends and
		// localhost see the neutral form — no command leaks to a third party.
		opts = append(opts, api.WithUserAgent(api.BuildUserAgent(mgr.APIURL(), cliVersion, cmd.CommandPath())))

		if devUser := mgr.DevUserID(); devUser != "" {
			opts = append(opts, api.WithDevMode(devUser, mgr.DevOrgMemberships(), mgr.DevWorkspaceMemberships()))
		}

		client := api.NewClient(mgr.APIURL(), token, opts...)
		formatter := output.NewFormatter(mgr.OutputFormat(), flagJSON, flagQuiet)
		setRuntimeOutputFormat(formatter.Format())

		// Validate --query before any work (and before any network call):
		// a bad expression must be a fast, clean validation error.
		if flagQuery != "" {
			if !formatter.IsStructured() {
				return &cliError{
					Code:    "invalid_argument",
					Message: "--query requires structured output; add --json or --output json|yaml",
				}
			}
			parsed, err := gojq.Parse(flagQuery)
			if err != nil {
				return &cliError{
					Code:    "invalid_argument",
					Message: fmt.Sprintf("invalid --query expression: %v", err),
				}
			}
			compiled, err := gojq.Compile(parsed)
			if err != nil {
				return &cliError{
					Code:    "invalid_argument",
					Message: fmt.Sprintf("invalid --query expression: %v", err),
				}
			}
			formatter.SetQuery(compiled)
		}

		rc := &RunContext{
			Client:    client,
			Config:    mgr,
			Output:    formatter,
			Workspace: mgr.WorkspaceID(),
		}

		ctx := context.WithValue(cmd.Context(), contextKey{}, rc)
		cmd.SetContext(ctx)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "", "Output format: table, json, yaml")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable debug output on stderr")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().StringVarP(&flagWorkspace, "workspace", "w", "", "Workspace ID (overrides config)")
	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "API base URL (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&flagNonInteractive, "non-interactive", false, "Never prompt; fail fast when interactive input is required (also set by AGENTCLASH_NONINTERACTIVE or CI)")
	rootCmd.PersistentFlags().StringVar(&flagQuery, "query", "", "jq expression applied to structured output (alias: --jq); strings print raw, other results one compact JSON document per line. Requires --json or --output json|yaml.")
	// --jq is the de-facto name agents know from gh; normalize it onto --query
	// so both spellings parse while the schema documents a single flag. The
	// GLOBAL normalization func is required — a flag-set-local one would not
	// propagate to subcommand parsing.
	rootCmd.SetGlobalNormalizationFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "jq" {
			name = "query"
		}
		return pflag.NormalizedName(name)
	})
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
