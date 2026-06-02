package cmd

import (
	"fmt"

	"github.com/agentclash/agentclash/cli/internal/skills"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Export bundled AgentClash Agent Skills",
	Long: `Commands for the embedded AgentClash Agent Skills snapshot synced from
web/content/agent-skills.

Use export to write the bundle to a directory or .tar.gz archive for offline
install or air-gapped environments. For direct install into a coding agent's
skills directory, prefer:

  agentclash integration <host> install`,
}

var skillsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export bundled skills to a directory or tarball",
	Long: `Write every bundled AgentClash skill to disk for offline install.

Directory export (flat layout):

  agentclash skills export --dir ./agentclash-skills
  # -> ./agentclash-skills/<skill>/SKILL.md

Host layout (ready to merge into a home or project directory):

  agentclash skills export --dir ./out --host claude
  # -> ./out/.claude/skills/<skill>/SKILL.md

Tarball export:

  agentclash skills export --dir ./bundle.tar.gz --format tar.gz
  agentclash skills export --dir ./claude-skills.tar.gz --host claude --format tar.gz

Extract a host tarball into $HOME (or a project root) to match integration install paths.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		dir, _ := cmd.Flags().GetString("dir")
		host, _ := cmd.Flags().GetString("host")
		formatRaw, _ := cmd.Flags().GetString("format")

		if dir == "" {
			return fmt.Errorf("--dir is required")
		}
		format, err := skills.ParseExportFormat(formatRaw)
		if err != nil {
			return err
		}
		if host != "" {
			if _, err := skills.AgentSubdir(host); err != nil {
				return err
			}
		}

		report, err := skills.Export(dir, host, format)
		if err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(report)
		}
		w := rc.Output.Writer()
		fmt.Fprintf(w, "Exported %d skills (snapshot v%s)\n", report.Count, report.SnapshotVersion)
		fmt.Fprintf(w, "  format: %s\n", report.Format)
		if report.Host != "" {
			fmt.Fprintf(w, "  host layout: %s\n", report.Host)
		}
		fmt.Fprintf(w, "  path: %s\n", report.Path)
		rc.Output.PrintSuccess("Extract or copy the exported tree into your agent skills directory.")
		return nil
	},
}

func init() {
	skillsExportCmd.Flags().String("dir", "", "Output directory or .tar.gz archive path (required)")
	skillsExportCmd.Flags().String("host", "", "Layout for a coding agent host (claude, codex, cursor, openclaw, hermes, opencode)")
	skillsExportCmd.Flags().String("format", "dir", "Output format: dir or tar.gz")
	skillsCmd.AddCommand(skillsExportCmd)
	rootCmd.AddCommand(skillsCmd)
}
