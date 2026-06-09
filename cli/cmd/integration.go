package cmd

import (
	"fmt"

	"github.com/agentclash/agentclash/cli/internal/skills"
	"github.com/spf13/cobra"
)

func init() {
	for _, agent := range []string{"claude", "codex", "cursor", "openclaw", "hermes", "opencode"} {
		integrationCmd.AddCommand(newIntegrationAgentCmd(agent))
	}
	rootCmd.AddCommand(integrationCmd)
}

var integrationCmd = &cobra.Command{
	Use:   "integration",
	Short: "Install and verify AgentClash Agent Skills for coding agents",
	Long: `Install the bundled AgentClash Agent Skills into a coding agent's skills
directory, and verify the integration.

  agentclash integration claude install   # -> ~/.claude/skills/<skill>/SKILL.md
  agentclash integration codex install    # -> ~/.agents/skills/<skill>/SKILL.md
  agentclash integration cursor install   # -> ~/.cursor/skills/<skill>/SKILL.md
  agentclash integration openclaw install # -> ~/.openclaw/skills/<skill>/SKILL.md
  agentclash integration hermes install   # -> ~/.hermes/skills/<skill>/SKILL.md
  agentclash integration opencode install # -> ~/.config/opencode/skills/<skill>/SKILL.md
  agentclash integration claude doctor    # report installed / missing / drifted skills

The skills teach an agent to drive the AgentClash CLI. install is idempotent and
writes only SKILL.md files — it never modifies CLAUDE.md, AGENTS.md, .mcp.json,
or any other project config.

To export the same bundle for offline install, use:

  agentclash skills export --dir ./bundle.tar.gz --host claude --format tar.gz`,
}

func agentLabel(agent string) string {
	switch agent {
	case "claude":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "cursor":
		return "Cursor"
	case "openclaw":
		return "OpenClaw"
	case "hermes":
		return "Hermes"
	case "opencode":
		return "OpenCode"
	default:
		return agent
	}
}

func newIntegrationAgentCmd(agent string) *cobra.Command {
	c := &cobra.Command{
		Use:   agent,
		Short: fmt.Sprintf("Manage the AgentClash skills integration for %s", agentLabel(agent)),
	}
	c.AddCommand(newIntegrationInstallCmd(agent), newIntegrationDoctorCmd(agent))
	return c
}

func addIntegrationFlags(c *cobra.Command) {
	c.Flags().String("dir", "", "Install root (defaults to your home directory)")
	// Reserved per the thin-wrapper MCP verdict: defining the flag now keeps the
	// command contract stable if `agentclash mcp serve` ever ships. MCP stays
	// deferred because CLI+Skills is the correct runtime for shell-having coding
	// agents; a thin server is justified only on a specific trigger (chat-only
	// clients -> remote HTTP, MCP-registry discoverability, or multi-tenant
	// OAuth) — see docs/cli-workflow-handoff.md. It does NOT write any
	// MCP/.mcp.json config today (that would violate the anti-feature rule and is
	// an active security posture); install reports it's unavailable and doctor
	// adds an info check.
	c.Flags().Bool("with-mcp", false, "Reserved: also configure an MCP server (not available yet)")
}

func newIntegrationInstallCmd(agent string) *cobra.Command {
	c := &cobra.Command{
		Use:   "install",
		Short: fmt.Sprintf("Install AgentClash skills for %s", agentLabel(agent)),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rc := GetRunContext(cmd)
			dir, _ := cmd.Flags().GetString("dir")
			withMCP, _ := cmd.Flags().GetBool("with-mcp")

			destRoot, err := skills.DestRoot(dir)
			if err != nil {
				return err
			}
			report, err := skills.Install(agent, destRoot)
			if err != nil {
				return err
			}
			if withMCP {
				rc.Output.PrintWarning("MCP integration is not available yet (`agentclash mcp serve` is unimplemented); installed skills only.")
			}
			if rc.Output.IsStructured() {
				return rc.Output.PrintRaw(report)
			}
			w := rc.Output.Writer()
			fmt.Fprintf(w, "Installed AgentClash skills for %s → %s\n", agentLabel(agent), report.Dir)
			fmt.Fprintf(w, "  %d created, %d updated, %d unchanged (snapshot v%s)\n",
				report.Created, report.Updated, report.Unchanged, report.SnapshotVersion)
			rc.Output.PrintSuccess(fmt.Sprintf(
				"Reload skills in %s, then run `agentclash integration %s doctor` to verify.",
				agentLabel(agent), agent))
			return nil
		},
	}
	addIntegrationFlags(c)
	return c
}

func newIntegrationDoctorCmd(agent string) *cobra.Command {
	c := &cobra.Command{
		Use:   "doctor",
		Short: fmt.Sprintf("Verify the AgentClash skills integration for %s", agentLabel(agent)),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rc := GetRunContext(cmd)
			dir, _ := cmd.Flags().GetString("dir")
			withMCP, _ := cmd.Flags().GetBool("with-mcp")

			destRoot, err := skills.DestRoot(dir)
			if err != nil {
				return err
			}
			return printDoctorResult(rc, integrationDoctorChecks(rc, agent, destRoot, withMCP))
		},
	}
	addIntegrationFlags(c)
	return c
}

// integrationDoctorChecks builds the doctor checks for the skills integration.
// Auth is advisory (info) because installing/verifying skills works offline; the
// `skills` check drives the exit code (warn => non-zero via printDoctorResult).
func integrationDoctorChecks(rc *RunContext, agent, destRoot string, withMCP bool) []doctorCheck {
	checks := []doctorCheck{
		{
			Name:     "install",
			Status:   "ok",
			Detail:   fmt.Sprintf("agentclash %s", cliVersion),
			Metadata: map[string]any{"version": cliVersion},
		},
	}

	if rc != nil && rc.Client != nil && rc.Client.Token() != "" {
		checks = append(checks, doctorCheck{Name: "auth", Status: "ok", Detail: "API token configured."})
	} else {
		checks = append(checks, doctorCheck{
			Name:   "auth",
			Status: "info",
			Detail: "No API token (skills work offline; run `agentclash auth login` for eval/run commands).",
		})
	}

	audit, err := skills.Audit(agent, destRoot)
	if err != nil {
		checks = append(checks, doctorCheck{Name: "skills", Status: "fail", Detail: err.Error()})
		return checks
	}
	if len(audit.Missing) == 0 && len(audit.Drifted) == 0 {
		checks = append(checks, doctorCheck{
			Name:   "skills",
			Status: "ok",
			Detail: fmt.Sprintf("%d/%d skills installed for %s (v%s) in %s",
				len(audit.Installed), audit.Total, agent, audit.SnapshotVersion, audit.Dir),
			Metadata: map[string]any{
				"installed":        len(audit.Installed),
				"total":            audit.Total,
				"snapshot_version": audit.SnapshotVersion,
			},
		})
	} else {
		detail := fmt.Sprintf("%d/%d installed in %s", len(audit.Installed), audit.Total, audit.Dir)
		if len(audit.Drifted) > 0 {
			detail += fmt.Sprintf("; %d differ from bundled v%s", len(audit.Drifted), audit.SnapshotVersion)
		}
		checks = append(checks, doctorCheck{
			Name:     "skills",
			Status:   "warn",
			Detail:   detail,
			NextStep: fmt.Sprintf("agentclash integration %s install", agent),
			Metadata: map[string]any{
				"missing":          audit.Missing,
				"drifted":          audit.Drifted,
				"snapshot_version": audit.SnapshotVersion,
			},
		})
	}

	if withMCP {
		checks = append(checks, doctorCheck{
			Name:   "mcp",
			Status: "info",
			Detail: "MCP server not configured (skills-only integration; `agentclash mcp serve` is not implemented yet).",
		})
	}
	return checks
}
