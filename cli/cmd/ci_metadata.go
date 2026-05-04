package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type ciMetadata struct {
	Provider           string `json:"provider,omitempty"`
	Repository         string `json:"repository,omitempty"`
	PullRequestNumber  *int   `json:"pull_request_number,omitempty"`
	Branch             string `json:"branch,omitempty"`
	Ref                string `json:"ref,omitempty"`
	CommitSHA          string `json:"commit_sha,omitempty"`
	Workflow           string `json:"workflow,omitempty"`
	WorkflowRunID      string `json:"workflow_run_id,omitempty"`
	WorkflowRunAttempt string `json:"workflow_run_attempt,omitempty"`
	WorkflowRunURL     string `json:"workflow_run_url,omitempty"`
	EventName          string `json:"event_name,omitempty"`
	DefaultBranch      string `json:"default_branch,omitempty"`
}

func ciMetadataFromFlags(cmd *cobra.Command) (map[string]any, error) {
	metadata := detectGitHubActionsCIMetadata(os.Getenv, os.ReadFile)

	flags := cmd.Flags()
	if flags.Changed("ci-provider") {
		metadata.Provider, _ = flags.GetString("ci-provider")
	}
	if flags.Changed("ci-repository") {
		metadata.Repository, _ = flags.GetString("ci-repository")
	}
	if flags.Changed("ci-pull-request") {
		value, _ := flags.GetInt("ci-pull-request")
		if value <= 0 {
			return nil, fmt.Errorf("--ci-pull-request must be greater than 0")
		}
		metadata.PullRequestNumber = &value
	}
	if flags.Changed("ci-branch") {
		metadata.Branch, _ = flags.GetString("ci-branch")
	}
	if flags.Changed("ci-ref") {
		metadata.Ref, _ = flags.GetString("ci-ref")
	}
	if flags.Changed("ci-commit") {
		metadata.CommitSHA, _ = flags.GetString("ci-commit")
	}
	if flags.Changed("ci-workflow") {
		metadata.Workflow, _ = flags.GetString("ci-workflow")
	}
	if flags.Changed("ci-workflow-run-id") {
		metadata.WorkflowRunID, _ = flags.GetString("ci-workflow-run-id")
	}
	if flags.Changed("ci-workflow-run-attempt") {
		metadata.WorkflowRunAttempt, _ = flags.GetString("ci-workflow-run-attempt")
	}
	if flags.Changed("ci-workflow-run-url") {
		metadata.WorkflowRunURL, _ = flags.GetString("ci-workflow-run-url")
	}
	if flags.Changed("ci-event") {
		metadata.EventName, _ = flags.GetString("ci-event")
	}
	if flags.Changed("ci-default-branch") {
		metadata.DefaultBranch, _ = flags.GetString("ci-default-branch")
	}

	return ciMetadataMap(metadata), nil
}

func detectGitHubActionsCIMetadata(getenv func(string) string, readFile func(string) ([]byte, error)) ciMetadata {
	if strings.TrimSpace(getenv("GITHUB_ACTIONS")) != "true" {
		return ciMetadata{}
	}

	metadata := ciMetadata{
		Provider:           "github_actions",
		Repository:         getenv("GITHUB_REPOSITORY"),
		Branch:             firstNonEmptyString(getenv("GITHUB_HEAD_REF"), getenv("GITHUB_REF_NAME")),
		Ref:                getenv("GITHUB_REF"),
		CommitSHA:          getenv("GITHUB_SHA"),
		Workflow:           getenv("GITHUB_WORKFLOW"),
		WorkflowRunID:      getenv("GITHUB_RUN_ID"),
		WorkflowRunAttempt: getenv("GITHUB_RUN_ATTEMPT"),
		EventName:          getenv("GITHUB_EVENT_NAME"),
	}
	if prNumber := githubPullRequestNumber(getenv, readFile); prNumber > 0 {
		metadata.PullRequestNumber = &prNumber
	}
	metadata.DefaultBranch = githubDefaultBranch(getenv, readFile)
	if metadata.Repository != "" && metadata.WorkflowRunID != "" {
		serverURL := strings.TrimRight(firstNonEmptyString(getenv("GITHUB_SERVER_URL"), "https://github.com"), "/")
		metadata.WorkflowRunURL = serverURL + "/" + metadata.Repository + "/actions/runs/" + metadata.WorkflowRunID
	}
	return metadata
}

func githubPullRequestNumber(getenv func(string) string, readFile func(string) ([]byte, error)) int {
	if fromRef := githubPullRequestNumberFromRef(getenv("GITHUB_REF")); fromRef > 0 {
		return fromRef
	}
	eventPath := strings.TrimSpace(getenv("GITHUB_EVENT_PATH"))
	if eventPath == "" {
		return 0
	}
	payload, err := readFile(eventPath)
	if err != nil {
		return 0
	}
	var event struct {
		Number      int `json:"number"`
		PullRequest struct {
			Number int `json:"number"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return 0
	}
	if event.PullRequest.Number > 0 {
		return event.PullRequest.Number
	}
	return event.Number
}

func githubDefaultBranch(getenv func(string) string, readFile func(string) ([]byte, error)) string {
	eventPath := strings.TrimSpace(getenv("GITHUB_EVENT_PATH"))
	if eventPath == "" {
		return ""
	}
	payload, err := readFile(eventPath)
	if err != nil {
		return ""
	}
	var event struct {
		Repository struct {
			DefaultBranch string `json:"default_branch"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return ""
	}
	return event.Repository.DefaultBranch
}

func githubPullRequestNumberFromRef(ref string) int {
	parts := strings.Split(strings.TrimSpace(ref), "/")
	if len(parts) >= 3 && parts[0] == "refs" && parts[1] == "pull" {
		number, err := strconv.Atoi(parts[2])
		if err == nil && number > 0 {
			return number
		}
	}
	return 0
}

func ciMetadataMap(metadata ciMetadata) map[string]any {
	out := make(map[string]any)
	addString := func(key, value string) {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out[key] = trimmed
		}
	}
	addString("provider", metadata.Provider)
	addString("repository", metadata.Repository)
	if metadata.PullRequestNumber != nil && *metadata.PullRequestNumber > 0 {
		out["pull_request_number"] = *metadata.PullRequestNumber
	}
	addString("branch", metadata.Branch)
	addString("ref", metadata.Ref)
	addString("commit_sha", metadata.CommitSHA)
	addString("workflow", metadata.Workflow)
	addString("workflow_run_id", metadata.WorkflowRunID)
	addString("workflow_run_attempt", metadata.WorkflowRunAttempt)
	addString("workflow_run_url", metadata.WorkflowRunURL)
	addString("event_name", metadata.EventName)
	addString("default_branch", metadata.DefaultBranch)
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
