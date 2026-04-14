package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/api"
	"github.com/Atharva-Kanherkar/agentclash/cli/internal/auth"
	"github.com/Atharva-Kanherkar/agentclash/cli/internal/config"
	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

// Global flags.
var (
	flagJSON      bool
	flagOutput    string
	flagQuiet     bool
	flagVerbose   bool
	flagNoColor   bool
	flagWorkspace string
	flagAPIURL    string
	flagYes       bool
)

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

// RequireWorkspace returns the workspace ID or exits with an error.
func RequireWorkspace(cmd *cobra.Command) string {
	rc := GetRunContext(cmd)
	if rc.Workspace == "" {
		fmt.Fprintln(os.Stderr, "Error: no workspace specified. Use --workspace, set AGENTCLASH_WORKSPACE, run 'agentclash workspace use', or create .agentclash.yaml with 'agentclash init'.")
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
  agentclash auth login         Log in to your account
  agentclash workspace use      Set your default workspace
  agentclash run create         Create an evaluation run
  agentclash run events <id>    Stream live run events`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		output.InitColors(flagNoColor)

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

		if devUser := mgr.DevUserID(); devUser != "" {
			opts = append(opts, api.WithDevMode(devUser, mgr.DevOrgMemberships(), mgr.DevWorkspaceMemberships()))
		}

		client := api.NewClient(mgr.APIURL(), token, opts...)
		formatter := output.NewFormatter(mgr.OutputFormat(), flagJSON, flagQuiet)

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
	rootCmd.PersistentFlags().BoolVar(&flagYes, "yes", false, "Skip confirmation prompts")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
