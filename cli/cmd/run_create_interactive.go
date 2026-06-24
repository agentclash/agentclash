package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type evalPackSummary struct {
	ID       string                      `json:"id"`
	Name     string                      `json:"name"`
	Versions []evalPackVersionBrief `json:"versions"`
}

type evalPackVersionBrief struct {
	ID                  string              `json:"id"`
	VersionNumber       int                 `json:"version_number"`
	LifecycleStatus     string              `json:"lifecycle_status"`
	Modality            string              `json:"modality,omitempty"`
	InterfaceTransports []string            `json:"interface_transports,omitempty"`
	DeploymentDefaults  *deploymentDefaults `json:"deployment_defaults,omitempty"`
}

type deploymentDefaults struct {
	Aliases map[string]string   `json:"aliases,omitempty"`
	Lineups map[string][]string `json:"lineups,omitempty"`
}

type challengeInputSetSummary struct {
	ID       string `json:"id"`
	InputKey string `json:"input_key"`
	Name     string `json:"name"`
}

type deploymentSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type runCreateSelections struct {
	evalPackVersionID string
	challengeInputSetID    string
	deploymentIDs          []string
	mode                   string
}

func resolveRunCreateSelections(cmd *cobra.Command, rc *RunContext, workspaceID string) (runCreateSelections, error) {
	cpvID, _ := cmd.Flags().GetString("eval-pack-version")
	deployments, _ := cmd.Flags().GetStringSlice("deployments")
	lineup, _ := cmd.Flags().GetString("deployment-lineup")
	lineups, _ := cmd.Flags().GetStringSlice("deployment-lineups")
	inputSetID, _ := cmd.Flags().GetString("input-set")
	if len(deployments) > 0 && strings.TrimSpace(lineup) != "" {
		return runCreateSelections{}, fmt.Errorf("--deployment-lineup cannot be combined with --deployments")
	}
	if len(compactNonEmptyStrings(lineups)) > 0 {
		return runCreateSelections{}, fmt.Errorf("--deployment-lineups must be used with --seeds and cannot use guided deployment selection")
	}

	selections := runCreateSelections{
		evalPackVersionID: cpvID,
		challengeInputSetID:    inputSetID,
		deploymentIDs:          deployments,
	}

	interactive := isInteractiveTerminal(rc)
	if selections.evalPackVersionID != "" && len(selections.deploymentIDs) == 0 && !interactive {
		resolved, ok, err := resolveDefaultDeploymentLineup(cmd, rc, workspaceID, selections.evalPackVersionID, lineup)
		if err != nil {
			return runCreateSelections{}, err
		}
		if ok {
			selections.deploymentIDs = resolved
		}
	}

	if !interactive {
		missing := missingRunCreateInputs(selections)
		if len(missing) > 0 {
			return runCreateSelections{}, fmt.Errorf(
				"%s required in non-interactive mode; pass %s or rerun `agentclash run create` in a TTY for guided selection",
				joinHumanList(missing),
				joinHumanList(missingRunCreateFlags(selections)),
			)
		}
		return selections, nil
	}

	picker := newInteractivePicker()
	if selections.evalPackVersionID == "" {
		selectedVersion, err := promptForEvalPackVersion(cmd, rc, workspaceID, picker)
		if err != nil {
			return runCreateSelections{}, err
		}
		selections.evalPackVersionID = selectedVersion.ID
		selections.mode = suggestedRunModeForVersion(selectedVersion)
	}

	// Always offer the input-set picker when --input-set was omitted in a
	// TTY, even if --eval-pack-version was passed explicitly. Skipping
	// the picker on explicit cpv silently changes the meaning of an existing
	// flag combination — workflow-first auto-resolution belongs in
	// `agentclash eval start`, not as a hidden side effect of `run create`.
	if selections.challengeInputSetID == "" {
		selectedInputSet, err := maybePromptForChallengeInputSet(cmd, rc, workspaceID, selections.evalPackVersionID, picker)
		if err != nil {
			return runCreateSelections{}, err
		}
		selections.challengeInputSetID = selectedInputSet
	}

	if len(selections.deploymentIDs) == 0 {
		resolved, ok, err := resolveDefaultDeploymentLineup(cmd, rc, workspaceID, selections.evalPackVersionID, lineup)
		if err != nil {
			return runCreateSelections{}, err
		}
		if ok {
			selections.deploymentIDs = resolved
			return selections, nil
		}
		selectedDeployments, err := promptForDeployments(cmd, rc, workspaceID, picker)
		if err != nil {
			return runCreateSelections{}, err
		}
		selections.deploymentIDs = selectedDeployments
	}

	return selections, nil
}

func missingRunCreateInputs(selections runCreateSelections) []string {
	var missing []string
	if selections.evalPackVersionID == "" {
		missing = append(missing, "eval pack version")
	}
	if len(selections.deploymentIDs) == 0 {
		missing = append(missing, "deployment selection")
	}
	return missing
}

func missingRunCreateFlags(selections runCreateSelections) []string {
	var missing []string
	if selections.evalPackVersionID == "" {
		missing = append(missing, "--eval-pack-version")
	}
	if len(selections.deploymentIDs) == 0 {
		missing = append(missing, "--deployments or --deployment-lineup")
	}
	return missing
}

func promptForEvalPackVersion(cmd *cobra.Command, rc *RunContext, workspaceID string, picker interactivePicker) (evalPackVersionBrief, error) {
	packs, err := listEvalPacks(cmd, rc, workspaceID)
	if err != nil {
		return evalPackVersionBrief{}, err
	}

	options := make([]pickerOption, 0, len(packs))
	for _, pack := range packs {
		if len(pack.Versions) == 0 {
			continue
		}
		options = append(options, pickerOption{
			Label:       evalPackPickerLabel(pack),
			Description: evalPackPickerDescription(pack),
			Value:       pack.ID,
		})
	}

	selectedPack, err := selectOneOrAuto(picker, "Choose a eval pack", options)
	if err != nil {
		return evalPackVersionBrief{}, err
	}

	var versions []evalPackVersionBrief
	for _, pack := range packs {
		if pack.ID == selectedPack.Value {
			versions = append(versions, pack.Versions...)
			break
		}
	}
	sort.SliceStable(versions, func(i, j int) bool {
		return versions[i].VersionNumber > versions[j].VersionNumber
	})

	versionOptions := make([]pickerOption, 0, len(versions))
	for _, version := range versions {
		versionOptions = append(versionOptions, pickerOption{
			Label:       evalPackVersionPickerLabel(version),
			Description: evalPackVersionPickerDescription(version),
			Value:       version.ID,
		})
	}

	selectedVersion, err := selectOneOrAuto(picker, "Choose a eval pack version", versionOptions)
	if err != nil {
		return evalPackVersionBrief{}, err
	}
	for _, version := range versions {
		if version.ID == selectedVersion.Value {
			return version, nil
		}
	}
	return evalPackVersionBrief{}, fmt.Errorf("selected eval pack version %s was not found", selectedVersion.Value)
}

func maybePromptForChallengeInputSet(cmd *cobra.Command, rc *RunContext, workspaceID, evalPackVersionID string, picker interactivePicker) (string, error) {
	inputSets, err := listChallengeInputSets(cmd, rc, workspaceID, evalPackVersionID)
	if err != nil {
		return "", err
	}
	if len(inputSets) == 0 {
		return "", nil
	}

	options := make([]pickerOption, 0, len(inputSets))
	for _, inputSet := range inputSets {
		description := inputSet.InputKey
		if inputSet.Name != "" {
			description = fmt.Sprintf("key: %s", inputSet.InputKey)
		}
		label := inputSet.Name
		if label == "" {
			label = inputSet.InputKey
		}
		options = append(options, pickerOption{
			Label:       label,
			Description: fmt.Sprintf("%s • %s", description, inputSet.ID),
			Value:       inputSet.ID,
		})
	}

	selectedInputSet, err := selectOneOrAuto(picker, "Choose a challenge input set", options)
	if err != nil {
		return "", err
	}
	return selectedInputSet.Value, nil
}

func promptForDeployments(cmd *cobra.Command, rc *RunContext, workspaceID string, picker interactivePicker) ([]string, error) {
	deployments, err := listDeployments(cmd, rc, workspaceID)
	if err != nil {
		return nil, err
	}

	options := make([]pickerOption, 0, len(deployments))
	for _, deployment := range deployments {
		options = append(options, pickerOption{
			Label:       deployment.Name,
			Description: fmt.Sprintf("status: %s • %s", deployment.Status, deployment.ID),
			Value:       deployment.ID,
		})
	}

	selectedDeployments, err := selectManyOrAuto(picker, "Choose one or more deployments (space to toggle, enter to confirm)", options, 1)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(selectedDeployments))
	for _, deployment := range selectedDeployments {
		ids = append(ids, deployment.Value)
	}
	return ids, nil
}

func resolveDefaultDeploymentLineup(cmd *cobra.Command, rc *RunContext, workspaceID, evalPackVersionID, lineup string) ([]string, bool, error) {
	version, ok, err := findEvalPackVersion(cmd, rc, workspaceID, evalPackVersionID)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		if strings.TrimSpace(lineup) != "" {
			return nil, false, fmt.Errorf("eval pack version %s was not found while resolving deployment lineup", evalPackVersionID)
		}
		return nil, false, nil
	}
	if version.DeploymentDefaults == nil {
		if strings.TrimSpace(lineup) != "" {
			return nil, false, fmt.Errorf("eval pack version %s does not declare deployment defaults", evalPackVersionID)
		}
		return nil, false, nil
	}

	selectors, err := deploymentSelectorsForLineup(version.DeploymentDefaults, lineup)
	if err != nil {
		return nil, false, err
	}
	ids, err := resolveRunCreateDeploymentSelectors(cmd, rc, workspaceID, selectors)
	if err != nil {
		return nil, false, err
	}
	return ids, true, nil
}

func resolveRunCreateDeploymentLineups(cmd *cobra.Command, rc *RunContext, workspaceID, evalPackVersionID string, lineups []string) ([]runCreateDeploymentLineup, error) {
	if strings.TrimSpace(evalPackVersionID) == "" {
		return nil, fmt.Errorf("--eval-pack-version is required with --deployment-lineups")
	}
	names := compactNonEmptyStrings(lineups)
	if len(names) == 0 {
		return nil, fmt.Errorf("at least one deployment lineup is required")
	}
	singleLineup, _ := cmd.Flags().GetString("deployment-lineup")
	deployments, _ := cmd.Flags().GetStringSlice("deployments")
	if strings.TrimSpace(singleLineup) != "" {
		return nil, fmt.Errorf("--deployment-lineups cannot be combined with --deployment-lineup")
	}
	if len(compactNonEmptyStrings(deployments)) > 0 {
		return nil, fmt.Errorf("--deployment-lineups cannot be combined with --deployments")
	}

	resolved := make([]runCreateDeploymentLineup, 0, len(names))
	for _, name := range names {
		ids, _, err := resolveDefaultDeploymentLineup(cmd, rc, workspaceID, evalPackVersionID, name)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, runCreateDeploymentLineup{
			Name:          name,
			DeploymentIDs: ids,
		})
	}
	return resolved, nil
}

func findEvalPackVersion(cmd *cobra.Command, rc *RunContext, workspaceID, evalPackVersionID string) (evalPackVersionBrief, bool, error) {
	packs, err := listEvalPacks(cmd, rc, workspaceID)
	if err != nil {
		return evalPackVersionBrief{}, false, err
	}
	for _, pack := range packs {
		for _, version := range pack.Versions {
			if version.ID == evalPackVersionID {
				return version, true, nil
			}
		}
	}
	return evalPackVersionBrief{}, false, nil
}

func deploymentSelectorsForLineup(defaults *deploymentDefaults, lineup string) ([]string, error) {
	lineupName := strings.TrimSpace(lineup)
	if lineupName == "" {
		lineupName = "default"
	}
	selectors, ok := defaults.Lineups[lineupName]
	if !ok {
		return nil, fmt.Errorf("deployment lineup %q is not declared by the eval pack version", lineupName)
	}
	if len(selectors) == 0 {
		return nil, fmt.Errorf("deployment lineup %q does not include any deployment selectors", lineupName)
	}

	resolved := make([]string, 0, len(selectors))
	for i, selector := range selectors {
		trimmed := strings.TrimSpace(selector)
		if trimmed == "" {
			return nil, fmt.Errorf("deployment lineup %q entry %d is empty", lineupName, i)
		}
		if alias, ok := defaults.Aliases[trimmed]; ok {
			trimmed = strings.TrimSpace(alias)
		}
		if trimmed == "" {
			return nil, fmt.Errorf("deployment lineup %q entry %d resolves to an empty selector", lineupName, i)
		}
		resolved = append(resolved, trimmed)
	}
	return resolved, nil
}

func resolveRunCreateDeploymentSelectors(cmd *cobra.Command, rc *RunContext, workspaceID string, selectors []string) ([]string, error) {
	deployments, err := listDeployments(cmd, rc, workspaceID)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(selectors))
	seen := map[string]struct{}{}
	for _, selector := range selectors {
		matched, err := matchRunCreateDeployment(selector, deployments)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[matched.ID]; exists {
			continue
		}
		seen[matched.ID] = struct{}{}
		ids = append(ids, matched.ID)
	}
	return ids, nil
}

func matchRunCreateDeployment(selector string, deployments []deploymentSummary) (deploymentSummary, error) {
	var matches []deploymentSummary
	for _, deployment := range deployments {
		if selectorMatches(selector, deployment.ID, deployment.Name) {
			matches = append(matches, deployment)
		}
	}
	switch len(matches) {
	case 0:
		return deploymentSummary{}, fmt.Errorf("no deployment matched %q", selector)
	case 1:
		return matches[0], nil
	default:
		return deploymentSummary{}, fmt.Errorf("deployment selector %q matched multiple deployments; use the deployment id", selector)
	}
}

func selectOneOrAuto(picker interactivePicker, prompt string, options []pickerOption) (pickerOption, error) {
	switch len(options) {
	case 0:
		return pickerOption{}, fmt.Errorf("no options available for %s", prompt)
	case 1:
		return options[0], nil
	default:
		return picker.Select(prompt, options)
	}
}

func selectManyOrAuto(picker interactivePicker, prompt string, options []pickerOption, min int) ([]pickerOption, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options available for %s", prompt)
	}
	if len(options) == min {
		return options, nil
	}
	return picker.MultiSelect(prompt, options, min)
}

func listEvalPacks(cmd *cobra.Command, rc *RunContext, workspaceID string) ([]evalPackSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/eval-packs", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []evalPackSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func listChallengeInputSets(cmd *cobra.Command, rc *RunContext, workspaceID, evalPackVersionID string) ([]challengeInputSetSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/eval-pack-versions/"+evalPackVersionID+"/input-sets", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []challengeInputSetSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func listDeployments(cmd *cobra.Command, rc *RunContext, workspaceID string) ([]deploymentSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/agent-deployments", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []deploymentSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func joinHumanList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		prefix := ""
		for i := 0; i < len(items)-1; i++ {
			if i > 0 {
				prefix += ", "
			}
			prefix += items[i]
		}
		return prefix + ", and " + items[len(items)-1]
	}
}
