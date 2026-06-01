package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/auth"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().String("pack", "", "Challenge pack YAML file to check for run readiness")
}

type doctorCheck struct {
	Name     string         `json:"name"`
	Status   string         `json:"status"`
	Detail   string         `json:"detail"`
	NextStep string         `json:"next_step,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type doctorPackBundle struct {
	Pack struct {
		Slug   string `yaml:"slug"`
		Name   string `yaml:"name"`
		Family string `yaml:"family"`
	} `yaml:"pack"`
	Version struct {
		Number             int                           `yaml:"number"`
		DeploymentDefaults *doctorPackDeploymentDefaults `yaml:"deployment_defaults"`
		Sandbox            *doctorPackSandbox            `yaml:"sandbox"`
		EvaluationSpec     map[string]any                `yaml:"evaluation_spec"`
	} `yaml:"version"`
	Tools      map[string]any        `yaml:"tools"`
	Challenges []doctorPackChallenge `yaml:"challenges"`
	InputSets  []doctorPackInputSet  `yaml:"input_sets"`
}

type doctorPackDeploymentDefaults struct {
	Aliases map[string]string   `yaml:"aliases"`
	Lineups map[string][]string `yaml:"lineups"`
}

type doctorPackSandbox struct {
	EnvVars map[string]string `yaml:"env_vars"`
}

type doctorPackChallenge struct {
	Key        string `yaml:"key"`
	Title      string `yaml:"title"`
	Category   string `yaml:"category"`
	Difficulty string `yaml:"difficulty"`
}

type doctorPackInputSet struct {
	Key   string                `yaml:"key"`
	Name  string                `yaml:"name"`
	Cases []doctorPackInputCase `yaml:"cases"`
}

type doctorPackInputCase struct {
	ChallengeKey string `yaml:"challenge_key"`
	CaseKey      string `yaml:"case_key"`
	ItemKey      string `yaml:"item_key"`
}

type doctorWorkspaceSecretSummary struct {
	Key string `json:"key"`
}

var (
	doctorTemplateSecretPattern       = regexp.MustCompile(`\$\{secrets\.([A-Za-z_][A-Za-z0-9_]*)\}`)
	doctorWorkspaceSecretRefPattern   = regexp.MustCompile(`workspace-secret://([A-Za-z_][A-Za-z0-9_]*)`)
	doctorDeploymentReadyStatusValues = map[string]struct{}{
		"active": {},
		"ready":  {},
	}
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check auth, workspace, and eval readiness",
	Long: `Inspect the current CLI setup and report whether the happy path is ready:

auth login -> link -> challenge-pack init/publish -> eval start -> baseline set -> eval scorecard

Exits non-zero (code 1) when any check reports 'warn' or 'fail', so this
command can be used as a CI gate: 'agentclash doctor && agentclash eval start --json'.

Checks with status 'info' are advisory only and do not flip ready=false.
The 'baseline' check is 'info' on a fresh workspace because a baseline can only
be set after the first eval run completes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		packPath, _ := cmd.Flags().GetString("pack")

		checks := make([]doctorCheck, 0, 10)
		var cachedDeployments []deploymentWorkflowSummary
		cachedDeploymentsOK := false
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
			appendDoctorPackChecks(cmd, rc, packPath, "", false, nil, false, appendCheck)
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
			appendDoctorPackChecks(cmd, rc, packPath, "", false, nil, false, appendCheck)
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
			appendCheck(doctorCheck{
				Name:     "challenge_packs",
				Status:   "warn",
				Detail:   "Challenge-pack visibility check skipped until workspace is relinked.",
				NextStep: "Run `agentclash link` to relink the workspace.",
			})
			appendCheck(doctorCheck{
				Name:     "deployments",
				Status:   "warn",
				Detail:   "Deployment visibility check skipped until workspace is relinked.",
				NextStep: "Run `agentclash link` to relink the workspace.",
			})
			appendCheck(doctorCheck{
				Name:   "baseline",
				Status: "warn",
				Detail: "Baseline check skipped until workspace is relinked.",
			})
			appendDoctorPackChecks(cmd, rc, packPath, workspaceID, false, nil, false, appendCheck)
			return printDoctorResult(rc, checks)
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			appendCheck(doctorCheck{
				Name:     "workspace",
				Status:   "warn",
				Detail:   fmt.Sprintf("Workspace %s is not accessible: %s", workspaceID, apiErr.Message),
				NextStep: "Run `agentclash link` to pick an accessible workspace.",
			})
			appendCheck(doctorCheck{
				Name:     "challenge_packs",
				Status:   "warn",
				Detail:   "Challenge-pack visibility check skipped until workspace is relinked.",
				NextStep: "Run `agentclash link` to pick an accessible workspace.",
			})
			appendCheck(doctorCheck{
				Name:     "deployments",
				Status:   "warn",
				Detail:   "Deployment visibility check skipped until workspace is relinked.",
				NextStep: "Run `agentclash link` to pick an accessible workspace.",
			})
			appendCheck(doctorCheck{
				Name:   "baseline",
				Status: "warn",
				Detail: "Baseline check skipped until workspace is relinked.",
			})
			appendDoctorPackChecks(cmd, rc, packPath, workspaceID, false, nil, false, appendCheck)
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
		} else {
			cachedDeployments = deployments
			cachedDeploymentsOK = true
		}
		if deploymentErr == nil && len(deployments) == 0 {
			appendCheck(doctorCheck{
				Name:     "deployments",
				Status:   "warn",
				Detail:   "No deployments are available in this workspace.",
				NextStep: "Create or deploy an agent before starting an eval.",
			})
		} else if deploymentErr == nil {
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
				Status:   "info",
				Detail:   "No baseline is bookmarked for this workspace.",
				NextStep: "After your first run, use `agentclash baseline set`.",
			})
		}

		appendDoctorPackChecks(cmd, rc, packPath, workspaceID, true, cachedDeployments, cachedDeploymentsOK, appendCheck)
		return printDoctorResult(rc, checks)
	},
}

func appendDoctorPackChecks(cmd *cobra.Command, rc *RunContext, packPath, workspaceID string, remoteReady bool, cachedDeployments []deploymentWorkflowSummary, cachedDeploymentsOK bool, appendCheck func(doctorCheck)) {
	packPath = strings.TrimSpace(packPath)
	if packPath == "" {
		return
	}

	data, err := os.ReadFile(packPath)
	if err != nil {
		appendCheck(doctorCheck{
			Name:     "pack_manifest",
			Status:   "fail",
			Detail:   fmt.Sprintf("Could not read %s: %v", packPath, err),
			NextStep: "Pass a readable challenge-pack YAML file to `agentclash doctor --pack`.",
			Metadata: map[string]any{"path": packPath},
		})
		return
	}

	var bundle doctorPackBundle
	if err := yaml.Unmarshal(data, &bundle); err != nil {
		appendCheck(doctorCheck{
			Name:     "pack_manifest",
			Status:   "fail",
			Detail:   fmt.Sprintf("Challenge-pack YAML is invalid: %v", err),
			NextStep: "Fix the challenge-pack YAML, then rerun `agentclash doctor --pack`.",
			Metadata: map[string]any{"path": packPath},
		})
		return
	}

	if issues := doctorPackManifestIssues(bundle); len(issues) > 0 {
		appendCheck(doctorCheck{
			Name:     "pack_manifest",
			Status:   "fail",
			Detail:   "Challenge-pack manifest is missing required fields: " + strings.Join(issues, ", ") + ".",
			NextStep: "Run `agentclash challenge-pack validate " + packPath + "` for full schema validation.",
			Metadata: map[string]any{"path": packPath, "missing": issues},
		})
		return
	}

	appendCheck(doctorCheck{
		Name:   "pack_manifest",
		Status: "ok",
		Detail: fmt.Sprintf("%s v%d parsed from %s.", fallbackDisplay(bundle.Pack.Name, bundle.Pack.Slug), bundle.Version.Number, packPath),
		Metadata: map[string]any{
			"path":       packPath,
			"pack_slug":  bundle.Pack.Slug,
			"pack_name":  bundle.Pack.Name,
			"version":    bundle.Version.Number,
			"challenges": len(bundle.Challenges),
		},
	})

	appendDoctorPackInputSetCheck(bundle, appendCheck)
	appendDoctorPackSecretCheck(cmd, rc, data, workspaceID, remoteReady, appendCheck)
	appendDoctorPackDeploymentCheck(cmd, rc, bundle, workspaceID, remoteReady, cachedDeployments, cachedDeploymentsOK, appendCheck)
}

func doctorPackManifestIssues(bundle doctorPackBundle) []string {
	var issues []string
	if strings.TrimSpace(bundle.Pack.Slug) == "" {
		issues = append(issues, "pack.slug")
	}
	if strings.TrimSpace(bundle.Pack.Name) == "" {
		issues = append(issues, "pack.name")
	}
	if strings.TrimSpace(bundle.Pack.Family) == "" {
		issues = append(issues, "pack.family")
	}
	if bundle.Version.Number <= 0 {
		issues = append(issues, "version.number")
	}
	if len(bundle.Challenges) == 0 {
		issues = append(issues, "challenges")
	}
	return issues
}

func appendDoctorPackInputSetCheck(bundle doctorPackBundle, appendCheck func(doctorCheck)) {
	if len(bundle.InputSets) == 0 {
		appendCheck(doctorCheck{
			Name:     "pack_input_sets",
			Status:   "warn",
			Detail:   "Pack declares no input sets; run creation will not have cases to execute.",
			NextStep: "Add at least one `input_sets` entry with cases.",
		})
		return
	}

	totalCases := 0
	emptySets := make([]string, 0)
	for _, inputSet := range bundle.InputSets {
		totalCases += len(inputSet.Cases)
		if len(inputSet.Cases) == 0 {
			emptySets = append(emptySets, fallbackDisplay(inputSet.Key, inputSet.Name))
		}
	}
	if len(emptySets) > 0 {
		appendCheck(doctorCheck{
			Name:     "pack_input_sets",
			Status:   "warn",
			Detail:   fmt.Sprintf("%d input set(s) have no cases: %s.", len(emptySets), strings.Join(emptySets, ", ")),
			NextStep: "Add at least one case to every input set before publishing.",
			Metadata: map[string]any{
				"empty_input_sets": emptySets,
				"input_set_count":  len(bundle.InputSets),
			},
		})
		return
	}

	appendCheck(doctorCheck{
		Name:   "pack_input_sets",
		Status: "ok",
		Detail: fmt.Sprintf("%d input set(s), %d case(s).", len(bundle.InputSets), totalCases),
		Metadata: map[string]any{
			"input_set_count": len(bundle.InputSets),
			"case_count":      totalCases,
		},
	})
}

func appendDoctorPackSecretCheck(cmd *cobra.Command, rc *RunContext, data []byte, workspaceID string, remoteReady bool, appendCheck func(doctorCheck)) {
	refs := collectDoctorPackSecretRefs(data)
	if len(refs) == 0 {
		appendCheck(doctorCheck{
			Name:   "pack_secrets",
			Status: "ok",
			Detail: "No workspace secret references found.",
		})
		return
	}

	if !remoteReady || strings.TrimSpace(workspaceID) == "" {
		appendCheck(doctorCheck{
			Name:     "pack_secrets",
			Status:   "warn",
			Detail:   fmt.Sprintf("%d workspace secret reference(s) found; remote secret check skipped until a workspace is linked.", len(refs)),
			NextStep: "Run `agentclash auth login` and `agentclash link`, then rerun `agentclash doctor --pack`.",
			Metadata: map[string]any{"referenced": refs},
		})
		return
	}

	available, err := listDoctorWorkspaceSecretKeys(cmd, rc, workspaceID)
	if err != nil {
		appendCheck(doctorCheck{
			Name:     "pack_secrets",
			Status:   "warn",
			Detail:   fmt.Sprintf("Could not list workspace secrets: %v", err),
			NextStep: "Check workspace permissions or run `agentclash secret list`.",
			Metadata: map[string]any{"referenced": refs},
		})
		return
	}

	missing := missingStrings(refs, available)
	if len(missing) > 0 {
		appendCheck(doctorCheck{
			Name:     "pack_secrets",
			Status:   "warn",
			Detail:   fmt.Sprintf("%d referenced workspace secret(s) are missing: %s.", len(missing), strings.Join(missing, ", ")),
			NextStep: "Create the missing secret(s) with `agentclash secret set <KEY>`.",
			Metadata: map[string]any{
				"referenced": refs,
				"missing":    missing,
			},
		})
		return
	}

	appendCheck(doctorCheck{
		Name:   "pack_secrets",
		Status: "ok",
		Detail: fmt.Sprintf("%d workspace secret reference(s) exist in the linked workspace.", len(refs)),
		Metadata: map[string]any{
			"referenced": refs,
		},
	})
}

func collectDoctorPackSecretRefs(data []byte) []string {
	seen := map[string]struct{}{}
	for _, matches := range doctorTemplateSecretPattern.FindAllSubmatch(data, -1) {
		if len(matches) == 2 {
			seen[string(matches[1])] = struct{}{}
		}
	}
	for _, matches := range doctorWorkspaceSecretRefPattern.FindAllSubmatch(data, -1) {
		if len(matches) == 2 {
			seen[string(matches[1])] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

func listDoctorWorkspaceSecretKeys(cmd *cobra.Command, rc *RunContext, workspaceID string) ([]string, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/secrets", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []doctorWorkspaceSecretSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		if strings.TrimSpace(item.Key) != "" {
			keys = append(keys, item.Key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func appendDoctorPackDeploymentCheck(cmd *cobra.Command, rc *RunContext, bundle doctorPackBundle, workspaceID string, remoteReady bool, cachedDeployments []deploymentWorkflowSummary, cachedDeploymentsOK bool, appendCheck func(doctorCheck)) {
	defaults := bundle.Version.DeploymentDefaults
	if defaults == nil || len(defaults.Lineups) == 0 {
		appendCheck(doctorCheck{
			Name:     "pack_deployments",
			Status:   "warn",
			Detail:   "Pack has no deployment defaults; non-interactive run creation needs deployment flags.",
			NextStep: "Add `version.deployment_defaults.lineups.default` or pass `--deployments` when creating runs.",
		})
		return
	}

	lineupSelectors := doctorPackResolvedLineups(defaults)
	if len(lineupSelectors) == 0 {
		appendCheck(doctorCheck{
			Name:     "pack_deployments",
			Status:   "warn",
			Detail:   "Deployment defaults do not resolve to any selectors.",
			NextStep: "Add at least one selector under `version.deployment_defaults.lineups.default`.",
		})
		return
	}

	if _, ok := lineupSelectors["default"]; !ok {
		appendCheck(doctorCheck{
			Name:     "pack_deployments",
			Status:   "warn",
			Detail:   "Deployment defaults do not define a `default` lineup.",
			NextStep: "Add `version.deployment_defaults.lineups.default` for guided run creation.",
			Metadata: map[string]any{"lineups": lineupSelectors},
		})
		return
	}

	totalSelectors := countLineupSelectors(lineupSelectors)
	if !remoteReady || strings.TrimSpace(workspaceID) == "" {
		appendCheck(doctorCheck{
			Name:     "pack_deployments",
			Status:   "warn",
			Detail:   fmt.Sprintf("%d deployment selector(s) found; remote deployment check skipped until a workspace is linked.", totalSelectors),
			NextStep: "Run `agentclash auth login` and `agentclash link`, then rerun `agentclash doctor --pack`.",
			Metadata: map[string]any{"lineups": lineupSelectors},
		})
		return
	}

	deployments := cachedDeployments
	if !cachedDeploymentsOK {
		var err error
		deployments, err = listDeploymentsForWorkflow(cmd, rc, workspaceID)
		if err != nil {
			appendCheck(doctorCheck{
				Name:     "pack_deployments",
				Status:   "warn",
				Detail:   fmt.Sprintf("Could not list deployments for pack readiness: %v", err),
				NextStep: "Check workspace access or API connectivity.",
				Metadata: map[string]any{"lineups": lineupSelectors},
			})
			return
		}
	}

	missing, unhealthy := doctorPackDeploymentGaps(lineupSelectors, deployments)
	if len(missing) > 0 || len(unhealthy) > 0 {
		details := make([]string, 0, 2)
		if len(missing) > 0 {
			details = append(details, fmt.Sprintf("missing selectors: %s", strings.Join(missing, ", ")))
		}
		if len(unhealthy) > 0 {
			details = append(details, fmt.Sprintf("not ready: %s", strings.Join(unhealthy, ", ")))
		}
		appendCheck(doctorCheck{
			Name:     "pack_deployments",
			Status:   "warn",
			Detail:   "Deployment defaults have readiness gaps (" + strings.Join(details, "; ") + ").",
			NextStep: "Create or fix the referenced deployment(s), or update `version.deployment_defaults`.",
			Metadata: map[string]any{
				"lineups":   lineupSelectors,
				"missing":   missing,
				"unhealthy": unhealthy,
			},
		})
		return
	}

	appendCheck(doctorCheck{
		Name:     "pack_deployments",
		Status:   "ok",
		Detail:   fmt.Sprintf("%d deployment selector(s) across %d lineup(s) resolve in the linked workspace.", totalSelectors, len(lineupSelectors)),
		Metadata: map[string]any{"lineups": lineupSelectors},
	})
}

func doctorPackResolvedLineups(defaults *doctorPackDeploymentDefaults) map[string][]string {
	lineups := make(map[string][]string, len(defaults.Lineups))
	for name, selectors := range defaults.Lineups {
		lineupName := strings.TrimSpace(name)
		if lineupName == "" {
			continue
		}
		resolved := make([]string, 0, len(selectors))
		for _, selector := range selectors {
			selector = strings.TrimSpace(selector)
			if selector == "" {
				continue
			}
			if replacement := strings.TrimSpace(defaults.Aliases[selector]); replacement != "" {
				selector = replacement
			}
			resolved = append(resolved, selector)
		}
		if len(resolved) > 0 {
			lineups[lineupName] = resolved
		}
	}
	return lineups
}

func doctorPackDeploymentGaps(lineups map[string][]string, deployments []deploymentWorkflowSummary) ([]string, []string) {
	missingSet := map[string]struct{}{}
	unhealthySet := map[string]struct{}{}
	for _, selectors := range lineups {
		for _, selector := range selectors {
			matched, err := matchDeployment(selector, deployments)
			if err != nil {
				missingSet[selector] = struct{}{}
				continue
			}
			if !doctorDeploymentIsReady(matched.Status) {
				unhealthySet[fallbackDisplay(matched.Name, matched.ID)+" ("+matched.Status+")"] = struct{}{}
			}
		}
	}
	return sortedKeys(missingSet), sortedKeys(unhealthySet)
}

func doctorDeploymentIsReady(status string) bool {
	_, ok := doctorDeploymentReadyStatusValues[strings.ToLower(strings.TrimSpace(status))]
	return ok
}

func missingStrings(needles, haystack []string) []string {
	available := make(map[string]struct{}, len(haystack))
	for _, item := range haystack {
		available[item] = struct{}{}
	}
	var missing []string
	for _, item := range needles {
		if _, ok := available[item]; !ok {
			missing = append(missing, item)
		}
	}
	sort.Strings(missing)
	return missing
}

func countLineupSelectors(lineups map[string][]string) int {
	count := 0
	for _, selectors := range lineups {
		count += len(selectors)
	}
	return count
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
		if check.Status != "ok" && check.Status != "info" {
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
		if err := rc.Output.PrintRaw(result); err != nil {
			return err
		}
	} else {
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
	}

	if !ready {
		// Silent ExitCodeError: the rendered output above (table or JSON
		// envelope with `ready: false`) already conveys the failure state;
		// main should not print an additional error line.
		return &ExitCodeError{Code: 1}
	}
	return nil
}

func doctorStatusLabel(name string) string {
	return name + ":"
}
