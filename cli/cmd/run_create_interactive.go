package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type challengePackSummary struct {
	ID       string                      `json:"id"`
	Name     string                      `json:"name"`
	Versions []challengePackVersionBrief `json:"versions"`
}

type challengePackVersionBrief struct {
	ID              string `json:"id"`
	VersionNumber   int    `json:"version_number"`
	LifecycleStatus string `json:"lifecycle_status"`
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
	challengePackVersionID string
	challengeInputSetID    string
	deploymentIDs          []string
}

func resolveRunCreateSelections(cmd *cobra.Command, rc *RunContext, workspaceID string) (runCreateSelections, error) {
	cpvID, _ := cmd.Flags().GetString("challenge-pack-version")
	deployments, _ := cmd.Flags().GetStringSlice("deployments")
	inputSetID, _ := cmd.Flags().GetString("input-set")

	selections := runCreateSelections{
		challengePackVersionID: cpvID,
		challengeInputSetID:    inputSetID,
		deploymentIDs:          deployments,
	}

	if !isInteractiveTerminal(rc) {
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
	if selections.challengePackVersionID == "" {
		selectedVersion, err := promptForChallengePackVersion(cmd, rc, workspaceID, picker)
		if err != nil {
			return runCreateSelections{}, err
		}
		selections.challengePackVersionID = selectedVersion
	}

	// Always offer the input-set picker when --input-set was omitted in a
	// TTY, even if --challenge-pack-version was passed explicitly. Skipping
	// the picker on explicit cpv silently changes the meaning of an existing
	// flag combination — workflow-first auto-resolution belongs in
	// `agentclash eval start`, not as a hidden side effect of `run create`.
	if selections.challengeInputSetID == "" {
		selectedInputSet, err := maybePromptForChallengeInputSet(cmd, rc, workspaceID, selections.challengePackVersionID, picker)
		if err != nil {
			return runCreateSelections{}, err
		}
		selections.challengeInputSetID = selectedInputSet
	}

	if len(selections.deploymentIDs) == 0 {
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
	if selections.challengePackVersionID == "" {
		missing = append(missing, "challenge pack version")
	}
	if len(selections.deploymentIDs) == 0 {
		missing = append(missing, "deployment selection")
	}
	return missing
}

func missingRunCreateFlags(selections runCreateSelections) []string {
	var missing []string
	if selections.challengePackVersionID == "" {
		missing = append(missing, "--challenge-pack-version")
	}
	if len(selections.deploymentIDs) == 0 {
		missing = append(missing, "--deployments")
	}
	return missing
}

func promptForChallengePackVersion(cmd *cobra.Command, rc *RunContext, workspaceID string, picker interactivePicker) (string, error) {
	packs, err := listChallengePacks(cmd, rc, workspaceID)
	if err != nil {
		return "", err
	}

	options := make([]pickerOption, 0, len(packs))
	for _, pack := range packs {
		if len(pack.Versions) == 0 {
			continue
		}
		options = append(options, pickerOption{
			Label:       pack.Name,
			Description: fmt.Sprintf("%d runnable version(s) • %s", len(pack.Versions), pack.ID),
			Value:       pack.ID,
		})
	}

	selectedPack, err := selectOneOrAuto(picker, "Choose a challenge pack", options)
	if err != nil {
		return "", err
	}

	var versions []challengePackVersionBrief
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
			Label:       fmt.Sprintf("v%d", version.VersionNumber),
			Description: fmt.Sprintf("status: %s • %s", version.LifecycleStatus, version.ID),
			Value:       version.ID,
		})
	}

	selectedVersion, err := selectOneOrAuto(picker, "Choose a challenge pack version", versionOptions)
	if err != nil {
		return "", err
	}
	return selectedVersion.Value, nil
}

func maybePromptForChallengeInputSet(cmd *cobra.Command, rc *RunContext, workspaceID, challengePackVersionID string, picker interactivePicker) (string, error) {
	inputSets, err := listChallengeInputSets(cmd, rc, workspaceID, challengePackVersionID)
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

func listChallengePacks(cmd *cobra.Command, rc *RunContext, workspaceID string) ([]challengePackSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/challenge-packs", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []challengePackSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func listChallengeInputSets(cmd *cobra.Command, rc *RunContext, workspaceID, challengePackVersionID string) ([]challengeInputSetSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/challenge-pack-versions/"+challengePackVersionID+"/input-sets", nil)
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
