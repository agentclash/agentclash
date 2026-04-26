package cmd

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type userProfileResponse struct {
	UserID        string                    `json:"user_id"`
	Email         string                    `json:"email"`
	DisplayName   string                    `json:"display_name"`
	Organizations []userProfileOrganization `json:"organizations"`
}

type userProfileOrganization struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Slug       string                 `json:"slug"`
	Role       string                 `json:"role"`
	Workspaces []userProfileWorkspace `json:"workspaces"`
}

type userProfileWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Role string `json:"role"`
}

type linkedWorkspaceChoice struct {
	ID      string
	Name    string
	Slug    string
	Role    string
	OrgID   string
	OrgName string
	OrgSlug string
	OrgRole string
}

type challengePackWorkflowSummary struct {
	ID       string                             `json:"id"`
	Name     string                             `json:"name"`
	Slug     string                             `json:"slug"`
	Versions []challengePackWorkflowVersionInfo `json:"versions"`
}

type challengePackWorkflowVersionInfo struct {
	ID              string `json:"id"`
	VersionNumber   int    `json:"version_number"`
	LifecycleStatus string `json:"lifecycle_status"`
}

type deploymentWorkflowSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type regressionSuiteSummary struct {
	ID                    string `json:"id"`
	WorkspaceID           string `json:"workspace_id"`
	SourceChallengePackID string `json:"source_challenge_pack_id"`
	Name                  string `json:"name"`
	Status                string `json:"status"`
	CaseCount             int    `json:"case_count"`
}

type runWorkflowSummary struct {
	ID                     string `json:"id"`
	WorkspaceID            string `json:"workspace_id"`
	Name                   string `json:"name"`
	Status                 string `json:"status"`
	ChallengePackVersionID string `json:"challenge_pack_version_id"`
	ChallengeInputSetID    string `json:"challenge_input_set_id"`
	OfficialPackMode       string `json:"official_pack_mode"`
	CreatedAt              string `json:"created_at"`
}

type runAgentWorkflowSummary struct {
	ID     string `json:"id"`
	RunID  string `json:"run_id"`
	Label  string `json:"label"`
	Status string `json:"status"`
}

type resolvedChallengePack struct {
	PackID              string
	PackName            string
	PackSlug            string
	VersionID           string
	VersionNumber       int
	ChallengeInputSetID string
}

type resolvedRunTarget struct {
	Run      runWorkflowSummary
	RunAgent runAgentWorkflowSummary
}

func getUserProfile(cmd *cobra.Command, rc *RunContext) (userProfileResponse, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/users/me", nil)
	if err != nil {
		return userProfileResponse{}, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return userProfileResponse{}, apiErr
	}

	var profile userProfileResponse
	if err := resp.DecodeJSON(&profile); err != nil {
		return userProfileResponse{}, err
	}
	return profile, nil
}

func listAccessibleWorkspaces(cmd *cobra.Command, rc *RunContext) ([]linkedWorkspaceChoice, error) {
	profile, err := getUserProfile(cmd, rc)
	if err != nil {
		return nil, err
	}

	choices := make([]linkedWorkspaceChoice, 0)
	for _, org := range profile.Organizations {
		for _, workspace := range org.Workspaces {
			choices = append(choices, linkedWorkspaceChoice{
				ID:      workspace.ID,
				Name:    workspace.Name,
				Slug:    workspace.Slug,
				Role:    workspace.Role,
				OrgID:   org.ID,
				OrgName: org.Name,
				OrgSlug: org.Slug,
				OrgRole: org.Role,
			})
		}
	}

	sort.SliceStable(choices, func(i, j int) bool {
		if strings.EqualFold(choices[i].OrgName, choices[j].OrgName) {
			return strings.ToLower(choices[i].Name) < strings.ToLower(choices[j].Name)
		}
		return strings.ToLower(choices[i].OrgName) < strings.ToLower(choices[j].OrgName)
	})
	return choices, nil
}

func resolveWorkspaceChoice(cmd *cobra.Command, rc *RunContext, selector string) (linkedWorkspaceChoice, error) {
	choices, err := listAccessibleWorkspaces(cmd, rc)
	if err != nil {
		return linkedWorkspaceChoice{}, err
	}
	if len(choices) == 0 {
		return linkedWorkspaceChoice{}, fmt.Errorf("no accessible workspaces found for the current account")
	}

	if selector != "" {
		return matchWorkspaceChoice(selector, choices)
	}

	if len(choices) == 1 {
		return choices[0], nil
	}
	if !isInteractiveTerminal(rc) {
		return linkedWorkspaceChoice{}, fmt.Errorf("multiple workspaces available; pass a workspace id/slug or rerun `agentclash link` in a TTY")
	}

	options := make([]pickerOption, 0, len(choices))
	for _, choice := range choices {
		options = append(options, pickerOption{
			Label:       choice.Name,
			Description: fmt.Sprintf("%s (%s) • %s", choice.OrgName, choice.OrgSlug, choice.ID),
			Value:       choice.ID,
		})
	}

	selected, err := selectOneOrAuto(newInteractivePicker(), "Choose a workspace to link", options)
	if err != nil {
		return linkedWorkspaceChoice{}, err
	}
	return matchWorkspaceChoice(selected.Value, choices)
}

func matchWorkspaceChoice(selector string, choices []linkedWorkspaceChoice) (linkedWorkspaceChoice, error) {
	var matches []linkedWorkspaceChoice
	for _, choice := range choices {
		if selectorMatches(selector, choice.ID, choice.Slug, choice.Name) {
			matches = append(matches, choice)
		}
	}
	switch len(matches) {
	case 0:
		return linkedWorkspaceChoice{}, fmt.Errorf("no accessible workspace matched %q", selector)
	case 1:
		return matches[0], nil
	default:
		return linkedWorkspaceChoice{}, fmt.Errorf("workspace selector %q matched multiple workspaces; use the workspace id", selector)
	}
}

func listChallengePacksForWorkflow(cmd *cobra.Command, rc *RunContext, workspaceID string) ([]challengePackWorkflowSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/challenge-packs", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []challengePackWorkflowSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func resolveChallengePackForEval(cmd *cobra.Command, rc *RunContext, workspaceID, packSelector, versionSelector, inputSetSelector string) (resolvedChallengePack, error) {
	packs, err := listChallengePacksForWorkflow(cmd, rc, workspaceID)
	if err != nil {
		return resolvedChallengePack{}, err
	}
	if len(packs) == 0 {
		return resolvedChallengePack{}, fmt.Errorf("no challenge packs found in workspace %s", workspaceID)
	}

	pack, err := selectChallengePack(cmd, rc, packs, packSelector, versionSelector)
	if err != nil {
		return resolvedChallengePack{}, err
	}
	version, err := selectChallengePackVersion(pack, versionSelector)
	if err != nil {
		return resolvedChallengePack{}, err
	}
	inputSetID, err := resolveChallengeInputSetID(cmd, rc, workspaceID, version.ID, inputSetSelector)
	if err != nil {
		return resolvedChallengePack{}, err
	}

	return resolvedChallengePack{
		PackID:              pack.ID,
		PackName:            pack.Name,
		PackSlug:            pack.Slug,
		VersionID:           version.ID,
		VersionNumber:       version.VersionNumber,
		ChallengeInputSetID: inputSetID,
	}, nil
}

func selectChallengePack(cmd *cobra.Command, rc *RunContext, packs []challengePackWorkflowSummary, packSelector, versionSelector string) (challengePackWorkflowSummary, error) {
	if packSelector != "" {
		return matchChallengePack(packSelector, packs)
	}

	if versionSelector != "" {
		var matches []challengePackWorkflowSummary
		for _, pack := range packs {
			for _, version := range pack.Versions {
				if selectorMatches(versionSelector, version.ID) {
					matches = append(matches, pack)
					break
				}
			}
		}
		switch len(matches) {
		case 0:
			return challengePackWorkflowSummary{}, fmt.Errorf("no challenge pack version matched %q", versionSelector)
		case 1:
			return matches[0], nil
		default:
			return challengePackWorkflowSummary{}, fmt.Errorf("challenge pack version selector %q matched multiple packs; pass --pack as well", versionSelector)
		}
	}

	if len(packs) == 1 {
		return packs[0], nil
	}
	if !isInteractiveTerminal(rc) {
		return challengePackWorkflowSummary{}, fmt.Errorf("multiple challenge packs available; pass --pack, --pack-version, or rerun `agentclash eval start` in a TTY")
	}

	options := make([]pickerOption, 0, len(packs))
	for _, pack := range packs {
		options = append(options, pickerOption{
			Label:       pack.Name,
			Description: fmt.Sprintf("slug: %s • %d version(s)", pack.Slug, len(pack.Versions)),
			Value:       pack.ID,
		})
	}
	selected, err := selectOneOrAuto(newInteractivePicker(), "Choose a challenge pack", options)
	if err != nil {
		return challengePackWorkflowSummary{}, err
	}
	return matchChallengePack(selected.Value, packs)
}

func matchChallengePack(selector string, packs []challengePackWorkflowSummary) (challengePackWorkflowSummary, error) {
	var matches []challengePackWorkflowSummary
	for _, pack := range packs {
		if selectorMatches(selector, pack.ID, pack.Slug, pack.Name) {
			matches = append(matches, pack)
		}
	}
	switch len(matches) {
	case 0:
		return challengePackWorkflowSummary{}, fmt.Errorf("no challenge pack matched %q", selector)
	case 1:
		return matches[0], nil
	default:
		return challengePackWorkflowSummary{}, fmt.Errorf("challenge pack selector %q matched multiple packs; use the pack id or slug", selector)
	}
}

func selectChallengePackVersion(pack challengePackWorkflowSummary, versionSelector string) (challengePackWorkflowVersionInfo, error) {
	if len(pack.Versions) == 0 {
		return challengePackWorkflowVersionInfo{}, fmt.Errorf("challenge pack %s has no versions", pack.Name)
	}

	versions := append([]challengePackWorkflowVersionInfo(nil), pack.Versions...)
	sort.SliceStable(versions, func(i, j int) bool {
		return versions[i].VersionNumber > versions[j].VersionNumber
	})

	if versionSelector == "" {
		return versions[0], nil
	}

	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(versionSelector), "v"))
	versionNumber, versionNumberErr := strconv.Atoi(trimmed)

	var matches []challengePackWorkflowVersionInfo
	for _, version := range versions {
		if selectorMatches(versionSelector, version.ID) {
			matches = append(matches, version)
			continue
		}
		if versionNumberErr == nil && version.VersionNumber == versionNumber {
			matches = append(matches, version)
		}
	}
	switch len(matches) {
	case 0:
		return challengePackWorkflowVersionInfo{}, fmt.Errorf("no challenge pack version matched %q for pack %s", versionSelector, pack.Name)
	case 1:
		return matches[0], nil
	default:
		return challengePackWorkflowVersionInfo{}, fmt.Errorf("challenge pack version selector %q matched multiple versions", versionSelector)
	}
}

func resolveChallengeInputSetID(cmd *cobra.Command, rc *RunContext, workspaceID, challengePackVersionID, selector string) (string, error) {
	inputSets, err := listChallengeInputSets(cmd, rc, workspaceID, challengePackVersionID)
	if err != nil {
		return "", err
	}
	if len(inputSets) == 0 {
		return "", nil
	}

	if selector != "" {
		var matches []challengeInputSetSummary
		for _, inputSet := range inputSets {
			if selectorMatches(selector, inputSet.ID, inputSet.InputKey, inputSet.Name) {
				matches = append(matches, inputSet)
			}
		}
		switch len(matches) {
		case 0:
			return "", fmt.Errorf("no challenge input set matched %q", selector)
		case 1:
			return matches[0].ID, nil
		default:
			return "", fmt.Errorf("input-set selector %q matched multiple input sets; use the input set id", selector)
		}
	}

	if len(inputSets) == 1 {
		return inputSets[0].ID, nil
	}
	if !isInteractiveTerminal(rc) {
		return "", fmt.Errorf("challenge pack has multiple input sets; pass --input-set or rerun `agentclash eval start` in a TTY")
	}

	options := make([]pickerOption, 0, len(inputSets))
	for _, inputSet := range inputSets {
		label := inputSet.Name
		if label == "" {
			label = inputSet.InputKey
		}
		options = append(options, pickerOption{
			Label:       label,
			Description: fmt.Sprintf("key: %s • %s", inputSet.InputKey, inputSet.ID),
			Value:       inputSet.ID,
		})
	}
	selected, err := selectOneOrAuto(newInteractivePicker(), "Choose a challenge input set", options)
	if err != nil {
		return "", err
	}
	return selected.Value, nil
}

func listDeploymentsForWorkflow(cmd *cobra.Command, rc *RunContext, workspaceID string) ([]deploymentWorkflowSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/agent-deployments", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []deploymentWorkflowSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func resolveDeploymentIDs(cmd *cobra.Command, rc *RunContext, workspaceID string, selectors []string) ([]string, error) {
	deployments, err := listDeploymentsForWorkflow(cmd, rc, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(deployments) == 0 {
		return nil, fmt.Errorf("no deployments found in workspace %s", workspaceID)
	}

	if len(selectors) == 0 {
		if len(deployments) == 1 {
			return []string{deployments[0].ID}, nil
		}
		if !isInteractiveTerminal(rc) {
			return nil, fmt.Errorf("multiple deployments available; pass --deployment or rerun `agentclash eval start` in a TTY")
		}
		options := make([]pickerOption, 0, len(deployments))
		for _, deployment := range deployments {
			options = append(options, pickerOption{
				Label:       deployment.Name,
				Description: fmt.Sprintf("status: %s • %s", deployment.Status, deployment.ID),
				Value:       deployment.ID,
			})
		}
		selected, err := selectManyOrAuto(newInteractivePicker(), "Choose one or more deployments (space to toggle, enter to confirm)", options, 1)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(selected))
		for _, item := range selected {
			ids = append(ids, item.Value)
		}
		return ids, nil
	}

	ids := make([]string, 0, len(selectors))
	seen := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		matched, err := matchDeployment(selector, deployments)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[matched.ID]; ok {
			continue
		}
		seen[matched.ID] = struct{}{}
		ids = append(ids, matched.ID)
	}
	return ids, nil
}

func matchDeployment(selector string, deployments []deploymentWorkflowSummary) (deploymentWorkflowSummary, error) {
	var matches []deploymentWorkflowSummary
	for _, deployment := range deployments {
		if selectorMatches(selector, deployment.ID, deployment.Name) {
			matches = append(matches, deployment)
		}
	}
	switch len(matches) {
	case 0:
		return deploymentWorkflowSummary{}, fmt.Errorf("no deployment matched %q", selector)
	case 1:
		return matches[0], nil
	default:
		return deploymentWorkflowSummary{}, fmt.Errorf("deployment selector %q matched multiple deployments; use the deployment id", selector)
	}
}

func listRegressionSuites(cmd *cobra.Command, rc *RunContext, workspaceID string) ([]regressionSuiteSummary, error) {
	query := url.Values{}
	query.Set("limit", "100")
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/regression-suites", query)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []regressionSuiteSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func resolveRegressionSuiteIDs(cmd *cobra.Command, rc *RunContext, workspaceID, packID string, selectors []string) ([]string, error) {
	if len(selectors) == 0 {
		return nil, nil
	}

	suites, err := listRegressionSuites(cmd, rc, workspaceID)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(selectors))
	seen := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		matched, err := matchRegressionSuite(selector, packID, suites)
		if err != nil {
			return nil, err
		}
		if matched.Status != "active" {
			return nil, fmt.Errorf("regression suite %s is not active", matched.Name)
		}
		if _, ok := seen[matched.ID]; ok {
			continue
		}
		seen[matched.ID] = struct{}{}
		ids = append(ids, matched.ID)
	}
	return ids, nil
}

func matchRegressionSuite(selector, packID string, suites []regressionSuiteSummary) (regressionSuiteSummary, error) {
	var matches []regressionSuiteSummary
	for _, suite := range suites {
		if packID != "" && suite.SourceChallengePackID != "" && suite.SourceChallengePackID != packID {
			continue
		}
		if selectorMatches(selector, suite.ID, suite.Name) {
			matches = append(matches, suite)
		}
	}
	switch len(matches) {
	case 0:
		if packID != "" {
			return regressionSuiteSummary{}, fmt.Errorf("no regression suite matched %q for the selected challenge pack", selector)
		}
		return regressionSuiteSummary{}, fmt.Errorf("no regression suite matched %q", selector)
	case 1:
		return matches[0], nil
	default:
		return regressionSuiteSummary{}, fmt.Errorf("regression suite selector %q matched multiple suites; use the suite id", selector)
	}
}

func listRunsForWorkflow(cmd *cobra.Command, rc *RunContext, workspaceID string) ([]runWorkflowSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/runs", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []runWorkflowSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func getRunSummaryByID(cmd *cobra.Command, rc *RunContext, runID string) (runWorkflowSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+runID, nil)
	if err != nil {
		return runWorkflowSummary{}, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return runWorkflowSummary{}, apiErr
	}

	var run runWorkflowSummary
	if err := resp.DecodeJSON(&run); err != nil {
		return runWorkflowSummary{}, err
	}
	return run, nil
}

func resolveRunSummary(cmd *cobra.Command, rc *RunContext, workspaceID, selector string) (runWorkflowSummary, error) {
	if selector == "" {
		runs, err := listRunsForWorkflow(cmd, rc, workspaceID)
		if err != nil {
			return runWorkflowSummary{}, err
		}
		if len(runs) == 0 {
			return runWorkflowSummary{}, fmt.Errorf("no runs found in workspace %s", workspaceID)
		}
		// Defensive: don't rely on backend ordering. The current backend
		// returns ORDER BY created_at DESC, but a future change there must
		// not silently flip "latest run" semantics for `eval scorecard` or
		// the post-`eval start --follow` summary.
		sort.SliceStable(runs, func(i, j int) bool {
			return runs[i].CreatedAt > runs[j].CreatedAt
		})
		return runs[0], nil
	}

	runs, err := listRunsForWorkflow(cmd, rc, workspaceID)
	if err != nil {
		return runWorkflowSummary{}, err
	}

	var matches []runWorkflowSummary
	for _, run := range runs {
		if selectorMatches(selector, run.ID, run.Name) {
			matches = append(matches, run)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		run, err := getRunSummaryByID(cmd, rc, selector)
		if err != nil {
			return runWorkflowSummary{}, err
		}
		if run.WorkspaceID != "" && workspaceID != "" && run.WorkspaceID != workspaceID {
			return runWorkflowSummary{}, fmt.Errorf("run %s does not belong to workspace %s", selector, workspaceID)
		}
		return run, nil
	default:
		return runWorkflowSummary{}, fmt.Errorf("run selector %q matched multiple runs; use the run id", selector)
	}
}

func selectRunSummaryInteractively(cmd *cobra.Command, rc *RunContext, workspaceID string) (runWorkflowSummary, error) {
	runs, err := listRunsForWorkflow(cmd, rc, workspaceID)
	if err != nil {
		return runWorkflowSummary{}, err
	}
	if len(runs) == 0 {
		return runWorkflowSummary{}, fmt.Errorf("no runs found in workspace %s", workspaceID)
	}
	if len(runs) == 1 {
		return runs[0], nil
	}
	if !isInteractiveTerminal(rc) {
		return runWorkflowSummary{}, fmt.Errorf("multiple runs available; pass a run id or rerun the command in a TTY")
	}

	options := make([]pickerOption, 0, len(runs))
	for _, run := range runs {
		label := run.Name
		if label == "" {
			label = run.ID
		}
		options = append(options, pickerOption{
			Label:       label,
			Description: fmt.Sprintf("status: %s • %s", run.Status, run.ID),
			Value:       run.ID,
		})
	}
	selected, err := selectOneOrAuto(newInteractivePicker(), "Choose a run", options)
	if err != nil {
		return runWorkflowSummary{}, err
	}
	return resolveRunSummary(cmd, rc, workspaceID, selected.Value)
}

func listRunAgentsForWorkflow(cmd *cobra.Command, rc *RunContext, runID string) ([]runAgentWorkflowSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+runID+"/agents", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []runAgentWorkflowSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func resolveRunAgentSummary(cmd *cobra.Command, rc *RunContext, runID, selector string) (runAgentWorkflowSummary, error) {
	agents, err := listRunAgentsForWorkflow(cmd, rc, runID)
	if err != nil {
		return runAgentWorkflowSummary{}, err
	}
	if len(agents) == 0 {
		return runAgentWorkflowSummary{}, fmt.Errorf("run %s has no agent results", runID)
	}

	if selector != "" {
		var matches []runAgentWorkflowSummary
		for _, agent := range agents {
			if selectorMatches(selector, agent.ID, agent.Label) {
				matches = append(matches, agent)
			}
		}
		switch len(matches) {
		case 0:
			return runAgentWorkflowSummary{}, fmt.Errorf("no run agent matched %q for run %s", selector, runID)
		case 1:
			return matches[0], nil
		default:
			return runAgentWorkflowSummary{}, fmt.Errorf("run-agent selector %q matched multiple agents; use the run agent id", selector)
		}
	}

	if len(agents) == 1 {
		return agents[0], nil
	}
	if !isInteractiveTerminal(rc) {
		return runAgentWorkflowSummary{}, fmt.Errorf("run %s has multiple agents; pass --agent or rerun the command in a TTY", runID)
	}

	options := make([]pickerOption, 0, len(agents))
	for _, agent := range agents {
		options = append(options, pickerOption{
			Label:       agent.Label,
			Description: fmt.Sprintf("status: %s • %s", agent.Status, agent.ID),
			Value:       agent.ID,
		})
	}
	selected, err := selectOneOrAuto(newInteractivePicker(), "Choose a run agent", options)
	if err != nil {
		return runAgentWorkflowSummary{}, err
	}
	return resolveRunAgentSummary(cmd, rc, runID, selected.Value)
}

func selectorMatches(selector string, values ...string) bool {
	needle := strings.TrimSpace(selector)
	if needle == "" {
		return false
	}
	for _, value := range values {
		candidate := strings.TrimSpace(value)
		if candidate == "" {
			continue
		}
		if needle == candidate || strings.EqualFold(needle, candidate) {
			return true
		}
	}
	return false
}
