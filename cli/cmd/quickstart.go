package cmd

import (
	"fmt"

	"github.com/agentclash/agentclash/cli/internal/auth"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(quickstartCmd)
}

type quickstartCheck struct {
	Name     string         `json:"name"`
	Status   string         `json:"status"`
	Detail   string         `json:"detail"`
	NextStep string         `json:"next_step,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Check eval readiness and show the next best command",
	Long: `Check the current AgentClash CLI setup without creating resources or
starting runs. Quickstart is a friendly workflow guide; use 'agentclash doctor'
when you need a CI-style readiness gate.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		result := buildQuickstartResult(cmd, rc)
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		renderQuickstartResult(rc, result)
		return nil
	},
}

func buildQuickstartResult(cmd *cobra.Command, rc *RunContext) map[string]any {
	checks := make([]quickstartCheck, 0, 6)
	appendCheck := func(check quickstartCheck) {
		checks = append(checks, check)
	}

	appendCheck(quickstartCheck{
		Name:   "install",
		Status: "ok",
		Detail: fmt.Sprintf("agentclash %s using %s", cliVersion, rc.Client.BaseURL()),
		Metadata: map[string]any{
			"version": cliVersion,
			"api_url": rc.Client.BaseURL(),
		},
	})

	authOK := false
	if rc.Client.Token() == "" {
		appendCheck(quickstartCheck{
			Name:     "auth",
			Status:   "todo",
			Detail:   "No API token is configured.",
			NextStep: "agentclash auth login",
		})
	} else if authResult, err := auth.ValidateToken(cmd.Context(), rc.Client); err != nil {
		appendCheck(quickstartCheck{
			Name:     "auth",
			Status:   "todo",
			Detail:   err.Error(),
			NextStep: "agentclash auth login",
		})
	} else {
		authOK = true
		appendCheck(quickstartCheck{
			Name:   "auth",
			Status: "ok",
			Detail: fmt.Sprintf("Logged in as %s", loginIdentityLabel(authResult)),
			Metadata: map[string]any{
				"user_id": authResult.UserID,
				"email":   authResult.Email,
			},
		})
	}

	workspaceID := rc.Workspace
	if !authOK {
		appendCheck(quickstartCheck{
			Name:     "workspace",
			Status:   "blocked",
			Detail:   "Workspace checks need a logged-in account.",
			NextStep: "agentclash auth login",
		})
		return quickstartResult(rc, checks)
	}

	if workspaceID == "" {
		detail := "No default workspace is linked."
		if choices, err := listAccessibleWorkspaces(cmd, rc); err == nil {
			detail = fmt.Sprintf("No default workspace is linked; %d accessible workspace(s) found.", len(choices))
		}
		appendCheck(quickstartCheck{
			Name:     "workspace",
			Status:   "todo",
			Detail:   detail,
			NextStep: "agentclash link",
		})
		return quickstartResult(rc, checks)
	}

	if workspace, err := fetchQuickstartWorkspace(cmd, rc, workspaceID); err != nil {
		appendCheck(quickstartCheck{
			Name:     "workspace",
			Status:   "todo",
			Detail:   fmt.Sprintf("Could not load workspace %s: %v", workspaceID, err),
			NextStep: "agentclash link",
		})
		return quickstartResult(rc, checks)
	} else {
		appendCheck(quickstartCheck{
			Name:   "workspace",
			Status: "ok",
			Detail: fmt.Sprintf("%s (%s)", fallbackDisplay(str(workspace["name"]), workspaceID), workspaceID),
			Metadata: map[string]any{
				"workspace_id":   workspaceID,
				"workspace_name": str(workspace["name"]),
				"workspace_slug": str(workspace["slug"]),
			},
		})
	}

	if packs, err := listEvalPacksForWorkflow(cmd, rc, workspaceID); err != nil {
		appendCheck(quickstartCheck{
			Name:     "eval_packs",
			Status:   "todo",
			Detail:   fmt.Sprintf("Could not list eval packs: %v", err),
			NextStep: "agentclash eval-pack init agentclash-pack.yaml",
		})
	} else if len(packs) == 0 {
		appendCheck(quickstartCheck{
			Name:     "eval_packs",
			Status:   "todo",
			Detail:   "No eval packs are published in this workspace.",
			NextStep: "agentclash eval-pack init agentclash-pack.yaml",
		})
	} else {
		appendCheck(quickstartCheck{
			Name:   "eval_packs",
			Status: "ok",
			Detail: fmt.Sprintf("%d eval pack(s) visible.", len(packs)),
			Metadata: map[string]any{
				"count": len(packs),
			},
		})
	}

	if deployments, err := listDeploymentsForWorkflow(cmd, rc, workspaceID); err != nil {
		appendCheck(quickstartCheck{
			Name:     "deployments",
			Status:   "todo",
			Detail:   fmt.Sprintf("Could not list deployments: %v", err),
			NextStep: "agentclash deployment list",
		})
	} else if len(deployments) == 0 {
		appendCheck(quickstartCheck{
			Name:     "deployments",
			Status:   "todo",
			Detail:   "No deployments are available in this workspace.",
			NextStep: "agentclash deployment create --from-file deployment.json",
		})
	} else {
		appendCheck(quickstartCheck{
			Name:   "deployments",
			Status: "ok",
			Detail: fmt.Sprintf("%d deployment(s) visible.", len(deployments)),
			Metadata: map[string]any{
				"count": len(deployments),
			},
		})
	}

	if bookmark, ok := rc.Config.BaselineBookmark(workspaceID); ok {
		appendCheck(quickstartCheck{
			Name:   "baseline",
			Status: "ok",
			Detail: fmt.Sprintf("Baseline set to %s (%s).", fallbackDisplay(bookmark.RunName, bookmark.RunID), bookmark.RunID),
			Metadata: map[string]any{
				"run_id":       bookmark.RunID,
				"run_agent_id": bookmark.RunAgentID,
			},
		})
	} else {
		appendCheck(quickstartCheck{
			Name:     "baseline",
			Status:   "info",
			Detail:   "No baseline bookmark yet.",
			NextStep: "agentclash baseline set",
		})
	}

	return quickstartResult(rc, checks)
}

func fetchQuickstartWorkspace(cmd *cobra.Command, rc *RunContext, workspaceID string) (map[string]any, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/details", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var workspace map[string]any
	if err := resp.DecodeJSON(&workspace); err != nil {
		return nil, err
	}
	return workspace, nil
}

func quickstartResult(rc *RunContext, checks []quickstartCheck) map[string]any {
	ready := true
	blockingSteps := make([]string, 0, len(checks))
	advisorySteps := make([]string, 0, len(checks))
	seen := map[string]struct{}{}
	for _, check := range checks {
		switch check.Status {
		case "ok", "info":
		default:
			ready = false
		}
		if check.NextStep == "" {
			continue
		}
		if _, ok := seen[check.NextStep]; ok {
			continue
		}
		seen[check.NextStep] = struct{}{}
		if check.Status == "info" {
			advisorySteps = append(advisorySteps, check.NextStep)
		} else {
			blockingSteps = append(blockingSteps, check.NextStep)
		}
	}

	nextCommand := ""
	nextSteps := make([]string, 0, len(blockingSteps)+len(advisorySteps)+2)
	if len(blockingSteps) > 0 {
		nextCommand = blockingSteps[0]
		nextSteps = append(nextSteps, blockingSteps...)
	} else if _, ok := rc.Config.BaselineBookmark(rc.Workspace); ok {
		nextCommand = "agentclash eval start --follow"
		nextSteps = append(nextSteps, nextCommand, "agentclash compare latest --gate")
	} else if ready {
		nextCommand = "agentclash eval start --follow"
		nextSteps = append(nextSteps, nextCommand)
		nextSteps = append(nextSteps, advisorySteps...)
	}

	return map[string]any{
		"ready":        ready,
		"api_url":      rc.Client.BaseURL(),
		"workspace":    rc.Workspace,
		"checks":       checks,
		"next_command": nextCommand,
		"next_steps":   nextSteps,
	}
}

func renderQuickstartResult(rc *RunContext, result map[string]any) {
	fmt.Fprintln(rc.Output.Writer(), output.Bold("AgentClash Quickstart"))
	fmt.Fprintln(rc.Output.Writer())
	rc.Output.PrintDetail("API URL", mapString(result, "api_url"))
	if workspace := mapString(result, "workspace"); workspace != "" {
		rc.Output.PrintDetail("Workspace", workspace)
	}
	fmt.Fprintln(rc.Output.Writer())

	checks, _ := result["checks"].([]quickstartCheck)
	for _, check := range checks {
		fmt.Fprintf(rc.Output.Writer(), "%-18s %s\n", quickstartStatusLabel(check), check.Detail)
		if check.NextStep != "" {
			fmt.Fprintf(rc.Output.Writer(), "%-18s %s\n", "", "next: "+check.NextStep)
		}
	}
	if next := mapString(result, "next_command"); next != "" {
		fmt.Fprintln(rc.Output.Writer())
		fmt.Fprintln(rc.Output.Writer(), output.Bold("Next Command"))
		fmt.Fprintf(rc.Output.Writer(), "  %s\n", next)
	}
}

func quickstartStatusLabel(check quickstartCheck) string {
	return fmt.Sprintf("%s:", check.Name)
}
