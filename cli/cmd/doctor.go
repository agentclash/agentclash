package cmd

import (
	"context"
	"fmt"

	"github.com/agentclash/agentclash/cli/internal/auth"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type doctorCheck struct {
	Name     string         `json:"name"`
	Status   string         `json:"status"`
	Detail   string         `json:"detail"`
	NextStep string         `json:"next_step,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check auth, workspace, and eval readiness",
	Long: `Inspect the current CLI setup and report whether the happy path is ready:

auth login -> link -> challenge-pack init/publish -> eval start -> baseline set -> eval scorecard`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		checks := make([]doctorCheck, 0, 6)
		appendCheck := func(check doctorCheck) {
			checks = append(checks, check)
		}

		appendCheck(doctorCheck{
			Name:   "install",
			Status: "ok",
			Detail: fmt.Sprintf("agentclash %s on %s", cliVersion, rc.Client.BaseURL()),
			Metadata: map[string]any{
				"version": cliVersion,
				"api_url": rc.Client.BaseURL(),
			},
		})

		authOK := false
		authResult, authErr := validateDoctorAuth(cmd.Context(), rc)
		if authErr != nil {
			appendCheck(doctorCheck{
				Name:     "auth",
				Status:   "warn",
				Detail:   authErr.Error(),
				NextStep: "Run `agentclash auth login`.",
			})
		} else {
			authOK = true
			appendCheck(doctorCheck{
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
			appendCheck(doctorCheck{
				Name:     "workspace",
				Status:   "warn",
				Detail:   "Workspace check skipped until authentication succeeds.",
				NextStep: "Run `agentclash auth login`, then `agentclash link`.",
			})
			return printDoctorResult(rc, checks)
		}

		if workspaceID == "" {
			workspaceCount := 0
			if choices, err := listAccessibleWorkspaces(cmd, rc); err == nil {
				workspaceCount = len(choices)
			}
			appendCheck(doctorCheck{
				Name:     "workspace",
				Status:   "warn",
				Detail:   fmt.Sprintf("No default workspace is linked (%d accessible workspace(s) found).", workspaceCount),
				NextStep: "Run `agentclash link`.",
			})
			appendCheck(doctorCheck{
				Name:     "challenge_packs",
				Status:   "warn",
				Detail:   "Challenge-pack visibility check skipped until a workspace is linked.",
				NextStep: "Run `agentclash link`.",
			})
			appendCheck(doctorCheck{
				Name:     "deployments",
				Status:   "warn",
				Detail:   "Deployment visibility check skipped until a workspace is linked.",
				NextStep: "Run `agentclash link`.",
			})
			appendCheck(doctorCheck{
				Name:     "baseline",
				Status:   "warn",
				Detail:   "Baseline check skipped until a workspace is linked.",
				NextStep: "Run `agentclash link`.",
			})
			return printDoctorResult(rc, checks)
		}

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/details", nil)
		if err != nil {
			appendCheck(doctorCheck{
				Name:     "workspace",
				Status:   "warn",
				Detail:   fmt.Sprintf("Could not load workspace %s: %v", workspaceID, err),
				NextStep: "Run `agentclash link` to relink the workspace.",
			})
			return printDoctorResult(rc, checks)
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			appendCheck(doctorCheck{
				Name:     "workspace",
				Status:   "warn",
				Detail:   fmt.Sprintf("Workspace %s is not accessible: %s", workspaceID, apiErr.Message),
				NextStep: "Run `agentclash link` to pick an accessible workspace.",
			})
			return printDoctorResult(rc, checks)
		}

		var workspace map[string]any
		if err := resp.DecodeJSON(&workspace); err != nil {
			return err
		}
		appendCheck(doctorCheck{
			Name:   "workspace",
			Status: "ok",
			Detail: fmt.Sprintf("%s (%s)", str(workspace["name"]), workspaceID),
			Metadata: map[string]any{
				"workspace_id":   workspaceID,
				"workspace_name": str(workspace["name"]),
				"workspace_slug": str(workspace["slug"]),
			},
		})

		packs, packErr := listChallengePacksForWorkflow(cmd, rc, workspaceID)
		if packErr != nil {
			appendCheck(doctorCheck{
				Name:     "challenge_packs",
				Status:   "warn",
				Detail:   fmt.Sprintf("Could not list challenge packs: %v", packErr),
				NextStep: "Check workspace access or API connectivity.",
			})
		} else if len(packs) == 0 {
			appendCheck(doctorCheck{
				Name:     "challenge_packs",
				Status:   "warn",
				Detail:   "No challenge packs are published in this workspace.",
				NextStep: "Run `agentclash challenge-pack init`, `validate`, and `publish`.",
			})
		} else {
			appendCheck(doctorCheck{
				Name:   "challenge_packs",
				Status: "ok",
				Detail: fmt.Sprintf("%d challenge pack(s) visible.", len(packs)),
				Metadata: map[string]any{
					"count": len(packs),
				},
			})
		}

		deployments, deploymentErr := listDeploymentsForWorkflow(cmd, rc, workspaceID)
		if deploymentErr != nil {
			appendCheck(doctorCheck{
				Name:     "deployments",
				Status:   "warn",
				Detail:   fmt.Sprintf("Could not list deployments: %v", deploymentErr),
				NextStep: "Check workspace access or API connectivity.",
			})
		} else if len(deployments) == 0 {
			appendCheck(doctorCheck{
				Name:     "deployments",
				Status:   "warn",
				Detail:   "No deployments are available in this workspace.",
				NextStep: "Create or deploy an agent before starting an eval.",
			})
		} else {
			appendCheck(doctorCheck{
				Name:   "deployments",
				Status: "ok",
				Detail: fmt.Sprintf("%d deployment(s) visible.", len(deployments)),
				Metadata: map[string]any{
					"count": len(deployments),
				},
			})
		}

		if bookmark, ok := rc.Config.BaselineBookmark(workspaceID); ok {
			appendCheck(doctorCheck{
				Name:   "baseline",
				Status: "ok",
				Detail: fmt.Sprintf("Baseline set to %s (%s).", fallbackDisplay(bookmark.RunName, bookmark.RunID), bookmark.RunID),
				Metadata: map[string]any{
					"run_id":       bookmark.RunID,
					"run_agent_id": bookmark.RunAgentID,
				},
			})
		} else {
			appendCheck(doctorCheck{
				Name:     "baseline",
				Status:   "warn",
				Detail:   "No baseline is bookmarked for this workspace.",
				NextStep: "After your first run, use `agentclash baseline set`.",
			})
		}

		return printDoctorResult(rc, checks)
	},
}

func validateDoctorAuth(ctx context.Context, rc *RunContext) (*auth.LoginResult, error) {
	if rc == nil || rc.Client == nil || rc.Client.Token() == "" {
		return nil, fmt.Errorf("Not logged in. No API token is configured.")
	}
	return auth.ValidateToken(ctx, rc.Client)
}

func printDoctorResult(rc *RunContext, checks []doctorCheck) error {
	ready := true
	nextSteps := make([]string, 0, len(checks))
	seenSteps := make(map[string]struct{}, len(checks))
	for _, check := range checks {
		if check.Status != "ok" {
			ready = false
		}
		if check.NextStep == "" {
			continue
		}
		if _, ok := seenSteps[check.NextStep]; ok {
			continue
		}
		seenSteps[check.NextStep] = struct{}{}
		nextSteps = append(nextSteps, check.NextStep)
	}

	result := map[string]any{
		"ready":      ready,
		"api_url":    rc.Client.BaseURL(),
		"workspace":  rc.Workspace,
		"checks":     checks,
		"next_steps": nextSteps,
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(result)
	}

	fmt.Fprintln(rc.Output.Writer(), "AgentClash Doctor")
	fmt.Fprintln(rc.Output.Writer())
	for _, check := range checks {
		fmt.Fprintf(rc.Output.Writer(), "%-18s %s\n", doctorStatusLabel(check.Name), check.Detail)
		if check.NextStep != "" {
			fmt.Fprintf(rc.Output.Writer(), "%-18s %s\n", "", "next: "+check.NextStep)
		}
	}
	if len(nextSteps) > 0 {
		fmt.Fprintln(rc.Output.Writer())
		fmt.Fprintln(rc.Output.Writer(), "Suggested next steps")
		for _, step := range nextSteps {
			fmt.Fprintf(rc.Output.Writer(), "  - %s\n", step)
		}
	}
	return nil
}

func doctorStatusLabel(name string) string {
	return name + ":"
}
