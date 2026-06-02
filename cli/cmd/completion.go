package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	// Replace cobra's default `completion` command with our own. The default
	// exits 0 on an unknown shell (e.g. `completion fish-shell` prints help and
	// succeeds); ours validates the shell argument and errors cleanly. Disabling
	// only affects the user-facing `completion` command — the hidden `__complete`
	// runtime that powers interactive tab-completion is unaffected.
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(completionCmd)
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate a shell completion script",
	Long: `Generate a shell completion script for agentclash.

Load completions for your shell:

  Bash:
    agentclash completion bash | sudo tee /etc/bash_completion.d/agentclash > /dev/null
    # or, per-user: echo 'source <(agentclash completion bash)' >> ~/.bashrc

  Zsh:
    agentclash completion zsh > "${fpath[1]}/_agentclash"
    # ensure 'autoload -U compinit && compinit' is in ~/.zshrc, then restart the shell

  fish:
    agentclash completion fish > ~/.config/fish/completions/agentclash.fish

  PowerShell:
    agentclash completion powershell | Out-String | Invoke-Expression`,
	// ExactValidArgs(1) + ValidArgs rejects both a missing shell and an unknown
	// shell with a non-zero exit, which is the behavior the default command lacks.
	Args:                  cobra.ExactValidArgs(1),
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	DisableFlagsInUseLine: true,
	// Completion scripts are shell code, not data — always written raw to stdout,
	// never wrapped by --json/--output.
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			// Unreachable given ExactValidArgs + ValidArgs, but explicit.
			return fmt.Errorf("unsupported shell %q", args[0])
		}
	},
}
