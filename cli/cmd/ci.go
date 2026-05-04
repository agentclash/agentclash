package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(ciCmd)
	ciCmd.AddCommand(ciInitCmd)
	ciCmd.AddCommand(ciValidateCmd)
	ciCmd.AddCommand(ciShouldRunCmd)
	ciCmd.AddCommand(ciBaselineCmd)

	ciInitCmd.Flags().Bool("force", false, "Overwrite an existing manifest")
	ciValidateCmd.Flags().Bool("remote", false, "Validate manifest resource IDs against the selected workspace")
	ciBaselineCmd.Flags().String("manifest", ".agentclash/ci.yaml", "Path to the AgentClash CI manifest")
	ciShouldRunCmd.Flags().String("manifest", ".agentclash/ci.yaml", "Path to the AgentClash CI manifest")
	ciShouldRunCmd.Flags().StringArray("changed-file", nil, "Changed file path; may be repeated")
	ciShouldRunCmd.Flags().StringSlice("labels", nil, "Pull request labels; may be comma-separated or repeated")
	ciShouldRunCmd.Flags().String("base", "", "Base git ref for deriving changed files")
	ciShouldRunCmd.Flags().String("head", "", "Head git ref for deriving changed files")
	ciShouldRunCmd.Flags().String("repo", ".", "Git repository path for --base/--head diff")
}

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Manage AgentClash CI manifests",
	Long: `Manage AgentClash CI manifests for main-product agent evaluation.

The CI manifest is a repo-tracked contract that maps source changes to a
candidate agent build version, deployment settings, evaluation workload,
baseline, release gate, and regression promotion policy.`,
}

var ciInitCmd = &cobra.Command{
	Use:   "init <file>",
	Short: "Write a sample AgentClash CI manifest",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		target := args[0]
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf("%s already exists; pass --force to overwrite", target)
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("checking %s: %w", target, err)
			}
		}
		if parent := filepath.Dir(target); parent != "." {
			if err := os.MkdirAll(parent, 0o755); err != nil {
				return fmt.Errorf("creating parent directory for %s: %w", target, err)
			}
		}
		if err := os.WriteFile(target, []byte(sampleCIManifestYAML), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", target, err)
		}

		result := map[string]any{
			"path":  target,
			"valid": true,
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Created %s", target))
		rc.Output.PrintDetail("Next", fmt.Sprintf("agentclash ci validate %s", target))
		return nil
	},
}

var ciValidateCmd = &cobra.Command{
	Use:   "validate <file>",
	Short: "Validate an AgentClash CI manifest",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		remote, _ := cmd.Flags().GetBool("remote")
		result, err := validateCIManifestFile(args[0])
		if remote && err == nil {
			workspaceID := RequireWorkspace(cmd)
			remoteResult, remoteErr := validateCIManifestRemote(cmd, rc, workspaceID, args[0], *result.Manifest)
			result.Remote = &remoteResult
			if len(remoteResult.Errors) > 0 {
				result.Errors = append(result.Errors, remoteResult.Errors...)
			}
			if !remoteResult.Valid {
				result.Valid = false
			}
			if remoteErr != nil {
				err = fmt.Errorf("remote ci manifest validation failed: %w", remoteErr)
			} else if !remoteResult.Valid {
				err = fmt.Errorf("ci manifest remote validation failed")
			}
		}
		if rc.Output.IsStructured() {
			if printErr := rc.Output.PrintRaw(result); printErr != nil {
				return printErr
			}
			if err != nil {
				return err
			}
			return nil
		}

		if err != nil {
			rc.Output.PrintError("AgentClash CI manifest is invalid")
			for _, msg := range result.Errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", output.SanitizeLine(msg))
			}
			return err
		}
		rc.Output.PrintSuccess("AgentClash CI manifest is valid")
		rc.Output.PrintDetail("Watched Paths", fmt.Sprintf("%d", len(result.Manifest.Trigger.Paths)))
		rc.Output.PrintDetail("Evaluation", result.Manifest.Evaluation.ChallengePackVersionID)
		if result.Remote != nil {
			rc.Output.PrintDetail("Workspace", result.Remote.WorkspaceID)
			rc.Output.PrintDetail("Remote Checks", fmt.Sprintf("%d", len(result.Remote.Checks)))
		}
		return nil
	},
}

var ciShouldRunCmd = &cobra.Command{
	Use:   "should-run",
	Short: "Decide whether AgentClash CI should run",
	Long: `Decide whether AgentClash CI should run for a change set.

The decision uses trigger.paths and trigger.labels from the CI manifest. Pass
changed files explicitly with --changed-file, or pass --base and --head to derive
changed files from git diff.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		manifestPath, _ := cmd.Flags().GetString("manifest")
		changedFiles, _ := cmd.Flags().GetStringArray("changed-file")
		labels, _ := cmd.Flags().GetStringSlice("labels")
		base, _ := cmd.Flags().GetString("base")
		head, _ := cmd.Flags().GetString("head")
		repo, _ := cmd.Flags().GetString("repo")

		validation, err := validateCIManifestFile(manifestPath)
		if err != nil {
			return err
		}
		for _, pattern := range validation.Manifest.Trigger.Paths {
			if err := ciValidateGlob(pattern); err != nil {
				return err
			}
		}

		base, head = defaultCIShouldRunRefs(base, head)
		if len(changedFiles) == 0 && (base != "" || head != "") {
			derived, err := gitChangedFiles(repo, base, head)
			if err != nil {
				return err
			}
			changedFiles = derived
		}

		result, err := evaluateCIShouldRun(manifestPath, *validation.Manifest, changedFiles, labels)
		if err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		if result.ShouldRun {
			rc.Output.PrintSuccess("AgentClash CI should run")
		} else {
			rc.Output.PrintDetail("Decision", "skip")
		}
		rc.Output.PrintDetail("Reason", result.Reason)
		if len(result.MatchedPaths) > 0 {
			rc.Output.PrintDetail("Matched Paths", fmt.Sprintf("%d", len(result.MatchedPaths)))
		}
		if len(result.MatchedLabels) > 0 {
			rc.Output.PrintDetail("Matched Labels", strings.Join(result.MatchedLabels, ", "))
		}
		return nil
	},
}

var ciBaselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Resolve the baseline selected by an AgentClash CI manifest",
	Long: `Resolve the baseline selected by an AgentClash CI manifest.

For pull request gates, prefer a locked baseline.run_id. A deployment baseline
is resolved to the newest completed, workload-compatible run whose participant
used baseline.deployment_id. The result explains the source, refresh policy, and
max_age_days staleness policy, and the exact run id that downstream gate
commands should compare against.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		workspaceID := RequireWorkspace(cmd)
		manifestPath, _ := cmd.Flags().GetString("manifest")

		validation, err := validateCIManifestFile(manifestPath)
		if err != nil {
			return err
		}
		resolution, err := resolveCIManifestBaseline(cmd, rc, workspaceID, manifestPath, *validation.Manifest, time.Now().UTC())
		if err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(resolution)
		}

		rc.Output.PrintSuccess("Resolved AgentClash CI baseline")
		rc.Output.PrintDetail("Strategy", resolution.Strategy)
		rc.Output.PrintDetail("Source", resolution.Source)
		rc.Output.PrintDetail("Run", resolution.Baseline.RunID)
		if resolution.Baseline.RunAgentID != "" {
			rc.Output.PrintDetail("Run Agent", resolution.Baseline.RunAgentID)
		}
		if resolution.Baseline.DeploymentID != "" {
			rc.Output.PrintDetail("Deployment", resolution.Baseline.DeploymentID)
		}
		rc.Output.PrintDetail("Reason", resolution.Reason)
		rc.Output.PrintDetail("Refresh", fmt.Sprintf("%s - %s", resolution.Refresh.Mode, resolution.Refresh.NextAction))
		if resolution.Baseline.AgeDays > 0 {
			rc.Output.PrintDetail("Age", fmt.Sprintf("%d day(s)", resolution.Baseline.AgeDays))
		}
		for _, warning := range resolution.Warnings {
			rc.Output.PrintWarning(warning)
		}
		return nil
	},
}

type ciManifest struct {
	Version     int                   `yaml:"version" json:"version"`
	Trigger     ciManifestTrigger     `yaml:"trigger" json:"trigger"`
	Candidate   ciManifestCandidate   `yaml:"candidate" json:"candidate"`
	Evaluation  ciManifestEvaluation  `yaml:"evaluation" json:"evaluation"`
	Baseline    ciManifestBaseline    `yaml:"baseline" json:"baseline"`
	Gate        ciManifestGate        `yaml:"gate" json:"gate"`
	Regressions ciManifestRegressions `yaml:"regressions" json:"regressions"`
}

type ciManifestTrigger struct {
	Paths  []string `yaml:"paths" json:"paths"`
	Labels []string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

type ciManifestCandidate struct {
	Build      ciManifestCandidateBuild      `yaml:"build" json:"build"`
	Deployment ciManifestCandidateDeployment `yaml:"deployment" json:"deployment"`
}

type ciManifestCandidateBuild struct {
	AgentBuildID string `yaml:"agent_build_id" json:"agent_build_id"`
	SpecFile     string `yaml:"spec_file" json:"spec_file"`
}

type ciManifestCandidateDeployment struct {
	Name              string `yaml:"name,omitempty" json:"name,omitempty"`
	RuntimeProfileID  string `yaml:"runtime_profile_id" json:"runtime_profile_id"`
	ProviderAccountID string `yaml:"provider_account_id,omitempty" json:"provider_account_id,omitempty"`
	ModelAliasID      string `yaml:"model_alias_id,omitempty" json:"model_alias_id,omitempty"`
}

type ciManifestEvaluation struct {
	ChallengePackVersionID string   `yaml:"challenge_pack_version_id" json:"challenge_pack_version_id"`
	InputSetID             string   `yaml:"input_set_id,omitempty" json:"input_set_id,omitempty"`
	RegressionSuites       []string `yaml:"regression_suites,omitempty" json:"regression_suites,omitempty"`
	RegressionCases        []string `yaml:"regression_cases,omitempty" json:"regression_cases,omitempty"`
}

type ciManifestBaseline struct {
	RunID        string `yaml:"run_id,omitempty" json:"run_id,omitempty"`
	RunAgentID   string `yaml:"run_agent_id,omitempty" json:"run_agent_id,omitempty"`
	DeploymentID string `yaml:"deployment_id,omitempty" json:"deployment_id,omitempty"`
	Refresh      string `yaml:"refresh,omitempty" json:"refresh,omitempty"`
	MaxAgeDays   int    `yaml:"max_age_days,omitempty" json:"max_age_days,omitempty"`
}

type ciManifestGate struct {
	FailOn     string `yaml:"fail_on" json:"fail_on"`
	PolicyFile string `yaml:"policy_file,omitempty" json:"policy_file,omitempty"`
}

type ciManifestRegressions struct {
	PromoteFailures string `yaml:"promote_failures" json:"promote_failures"`
}

type ciManifestValidationResult struct {
	Path     string                            `json:"path" yaml:"path"`
	Valid    bool                              `json:"valid" yaml:"valid"`
	Errors   []string                          `json:"errors,omitempty" yaml:"errors,omitempty"`
	Manifest *ciManifest                       `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Remote   *ciManifestRemoteValidationResult `json:"remote,omitempty" yaml:"remote,omitempty"`
}

type ciManifestRemoteValidationResult struct {
	WorkspaceID string                  `json:"workspace_id" yaml:"workspace_id"`
	Valid       bool                    `json:"valid" yaml:"valid"`
	Checks      []ciManifestRemoteCheck `json:"checks" yaml:"checks"`
	Errors      []string                `json:"errors,omitempty" yaml:"errors,omitempty"`
}

type ciManifestRemoteCheck struct {
	Field    string `json:"field" yaml:"field"`
	Resource string `json:"resource" yaml:"resource"`
	ID       string `json:"id,omitempty" yaml:"id,omitempty"`
	Valid    bool   `json:"valid" yaml:"valid"`
	Code     string `json:"code,omitempty" yaml:"code,omitempty"`
	Message  string `json:"message,omitempty" yaml:"message,omitempty"`
}

type ciRegressionCaseRemoteSummary struct {
	ID                           string `json:"id"`
	SuiteID                      string `json:"suite_id"`
	WorkspaceID                  string `json:"workspace_id"`
	Status                       string `json:"status"`
	SourceChallengePackVersionID string `json:"source_challenge_pack_version_id"`
	SourceChallengeInputSetID    string `json:"source_challenge_input_set_id"`
}

type ciShouldRunPathMatch struct {
	Pattern string `json:"pattern" yaml:"pattern"`
	File    string `json:"file" yaml:"file"`
}

type ciShouldRunResult struct {
	Path             string                 `json:"path" yaml:"path"`
	ShouldRun        bool                   `json:"should_run" yaml:"should_run"`
	Reason           string                 `json:"reason" yaml:"reason"`
	ChangedFiles     []string               `json:"changed_files" yaml:"changed_files"`
	Labels           []string               `json:"labels" yaml:"labels"`
	CheckedPathGlobs []string               `json:"checked_path_globs" yaml:"checked_path_globs"`
	CheckedLabels    []string               `json:"checked_labels" yaml:"checked_labels"`
	MatchedPaths     []ciShouldRunPathMatch `json:"matched_paths,omitempty" yaml:"matched_paths,omitempty"`
	MatchedLabels    []string               `json:"matched_labels,omitempty" yaml:"matched_labels,omitempty"`
}

type ciBaselineResolution struct {
	ManifestPath string                      `json:"manifest_path" yaml:"manifest_path"`
	WorkspaceID  string                      `json:"workspace_id" yaml:"workspace_id"`
	Strategy     string                      `json:"strategy" yaml:"strategy"`
	Source       string                      `json:"source" yaml:"source"`
	Reason       string                      `json:"reason" yaml:"reason"`
	Baseline     ciBaselineRunResolution     `json:"baseline" yaml:"baseline"`
	Refresh      ciBaselineRefreshResolution `json:"refresh" yaml:"refresh"`
	Warnings     []string                    `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type ciBaselineRunResolution struct {
	RunID                  string `json:"run_id" yaml:"run_id"`
	RunAgentID             string `json:"run_agent_id,omitempty" yaml:"run_agent_id,omitempty"`
	DeploymentID           string `json:"deployment_id,omitempty" yaml:"deployment_id,omitempty"`
	RunName                string `json:"run_name,omitempty" yaml:"run_name,omitempty"`
	Status                 string `json:"status" yaml:"status"`
	ChallengePackVersionID string `json:"challenge_pack_version_id,omitempty" yaml:"challenge_pack_version_id,omitempty"`
	InputSetID             string `json:"input_set_id,omitempty" yaml:"input_set_id,omitempty"`
	CreatedAt              string `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	FinishedAt             string `json:"finished_at,omitempty" yaml:"finished_at,omitempty"`
	AgeDays                int    `json:"age_days,omitempty" yaml:"age_days,omitempty"`
}

type ciBaselineRefreshResolution struct {
	Mode       string `json:"mode" yaml:"mode"`
	NextAction string `json:"next_action" yaml:"next_action"`
}

const sampleCIManifestYAML = `version: 1
trigger:
  paths:
    - .agentclash/agent.json
    - prompts/**
    - tools/**
  labels:
    - agentclash/eval
candidate:
  build:
    agent_build_id: 00000000-0000-0000-0000-000000000001
    spec_file: .agentclash/agent.json
  deployment:
    name: pr-candidate
    runtime_profile_id: 00000000-0000-0000-0000-000000000002
    provider_account_id: 00000000-0000-0000-0000-000000000003
    model_alias_id: 00000000-0000-0000-0000-000000000004
evaluation:
  challenge_pack_version_id: 00000000-0000-0000-0000-000000000005
  input_set_id: 00000000-0000-0000-0000-000000000006
  regression_suites:
    - 00000000-0000-0000-0000-000000000007
baseline:
  run_id: 00000000-0000-0000-0000-000000000008
  refresh: manual
  max_age_days: 30
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`

func validateCIManifestFile(path string) (ciManifestValidationResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		result := ciManifestValidationResult{Path: path, Valid: false, Errors: []string{fmt.Sprintf("read manifest: %v", err)}}
		return result, fmt.Errorf("read manifest: %w", err)
	}

	var manifest ciManifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		result := ciManifestValidationResult{Path: path, Valid: false, Errors: []string{fmt.Sprintf("parse YAML: %v", err)}}
		return result, fmt.Errorf("parse YAML: %w", err)
	}

	errors := validateCIManifest(manifest)
	result := ciManifestValidationResult{
		Path:     path,
		Valid:    len(errors) == 0,
		Errors:   errors,
		Manifest: &manifest,
	}
	if len(errors) > 0 {
		return result, fmt.Errorf("ci manifest validation failed")
	}
	return result, nil
}

func validateCIManifestRemote(cmd *cobra.Command, rc *RunContext, workspaceID, manifestPath string, manifest ciManifest) (ciManifestRemoteValidationResult, error) {
	result := ciManifestRemoteValidationResult{
		WorkspaceID: workspaceID,
		Valid:       true,
	}

	if _, err := validateCIManifestRemoteAgentBuild(cmd, rc, &result, workspaceID, manifest.Candidate.Build.AgentBuildID); err != nil {
		result.addAPIError(err)
		return result, err
	}

	if _, err := validateCIManifestRemoteListResource(cmd, rc, &result, workspaceID, "candidate.deployment.runtime_profile_id", "runtime_profile", manifest.Candidate.Deployment.RuntimeProfileID, "/v1/workspaces/%s/runtime-profiles"); err != nil {
		result.addAPIError(err)
		return result, err
	}
	var err error
	var providerAccount map[string]any
	if manifest.Candidate.Deployment.ProviderAccountID != "" {
		providerAccount, err = validateCIManifestRemoteListResource(cmd, rc, &result, workspaceID, "candidate.deployment.provider_account_id", "provider_account", manifest.Candidate.Deployment.ProviderAccountID, "/v1/workspaces/%s/provider-accounts")
		if err != nil {
			result.addAPIError(err)
			return result, err
		}
	}
	var modelAlias map[string]any
	if manifest.Candidate.Deployment.ModelAliasID != "" {
		modelAlias, err = validateCIManifestRemoteListResource(cmd, rc, &result, workspaceID, "candidate.deployment.model_alias_id", "model_alias", manifest.Candidate.Deployment.ModelAliasID, "/v1/workspaces/%s/model-aliases")
		if err != nil {
			result.addAPIError(err)
			return result, err
		}
	}
	validateCIManifestRemoteModelProvider(&result, manifest, providerAccount, modelAlias)

	packID, versionID, err := validateCIManifestRemoteChallengeVersion(cmd, rc, &result, workspaceID, manifest.Evaluation.ChallengePackVersionID)
	if err != nil {
		result.addAPIError(err)
		return result, err
	}
	if err := validateCIManifestRemoteInputSet(cmd, rc, &result, workspaceID, versionID, manifest.Evaluation.InputSetID); err != nil {
		result.addAPIError(err)
		return result, err
	}
	var suites []regressionSuiteSummary
	var selectedSuiteIDs []string
	if len(manifest.Evaluation.RegressionSuites) > 0 || len(manifest.Evaluation.RegressionCases) > 0 {
		suites, selectedSuiteIDs, err = validateCIManifestRemoteRegressionSuites(cmd, rc, &result, workspaceID, packID, manifest.Evaluation.RegressionSuites)
		if err != nil {
			result.addAPIError(err)
			return result, err
		}
	}
	if err := validateCIManifestRemoteRegressionCases(cmd, rc, &result, workspaceID, versionID, manifest.Evaluation.InputSetID, suites, selectedSuiteIDs, len(manifest.Evaluation.RegressionSuites) > 0, manifest.Evaluation.RegressionCases); err != nil {
		result.addAPIError(err)
		return result, err
	}
	if err := validateCIManifestRemoteBaseline(cmd, rc, &result, workspaceID, manifestPath, manifest); err != nil {
		return result, err
	}

	return result, nil
}

func validateCIManifestRemoteAgentBuild(cmd *cobra.Command, rc *RunContext, result *ciManifestRemoteValidationResult, workspaceID, id string) (map[string]any, error) {
	field := "candidate.build.agent_build_id"
	resp, err := rc.Client.Get(cmd.Context(), "/v1/agent-builds/"+id, nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		if ciRemoteAPIErrorCanBeFieldFailure(apiErr) {
			result.addFailure(field, "agent_build", id, ciRemoteAPIErrorCode(apiErr), fmt.Sprintf("agent build %s is not accessible in workspace %s: %s", id, workspaceID, apiErr.Error()))
			return nil, nil
		}
		return nil, apiErr
	}

	var build map[string]any
	if err := resp.DecodeJSON(&build); err != nil {
		return nil, err
	}
	if !ciRemoteWorkspaceMatches(result, field, "agent_build", id, mapString(build, "workspace_id"), workspaceID) {
		return build, nil
	}
	result.addPass(field, "agent_build", id, "agent build is accessible in the selected workspace")
	return build, nil
}

func validateCIManifestRemoteListResource(cmd *cobra.Command, rc *RunContext, result *ciManifestRemoteValidationResult, workspaceID, field, resource, id, pathTemplate string) (map[string]any, error) {
	items, err := listCIManifestRemoteObjects(cmd, rc, fmt.Sprintf(pathTemplate, workspaceID), nil)
	if err != nil {
		return nil, err
	}
	item, ok := findCIManifestRemoteObjectByID(items, id)
	if !ok {
		result.addFailure(field, resource, id, "not_found", fmt.Sprintf("%s %s was not found in or is not accessible to workspace %s", resource, id, workspaceID))
		return nil, nil
	}
	if !ciRemoteWorkspaceMatches(result, field, resource, id, mapString(item, "workspace_id"), workspaceID) {
		return item, nil
	}
	result.addPass(field, resource, id, fmt.Sprintf("%s exists in the selected workspace", resource))
	return item, nil
}

func validateCIManifestRemoteModelProvider(result *ciManifestRemoteValidationResult, manifest ciManifest, providerAccount map[string]any, modelAlias map[string]any) {
	if providerAccount == nil || modelAlias == nil {
		return
	}
	providerAccountID := strings.TrimSpace(manifest.Candidate.Deployment.ProviderAccountID)
	aliasProviderID := strings.TrimSpace(mapString(modelAlias, "provider_account_id"))
	if providerAccountID == "" || aliasProviderID == "" || aliasProviderID == providerAccountID {
		return
	}
	result.addFailure(
		"candidate.deployment.model_alias_id",
		"model_alias",
		manifest.Candidate.Deployment.ModelAliasID,
		"incompatible",
		fmt.Sprintf("model alias %s uses provider account %s, not candidate.deployment.provider_account_id %s", manifest.Candidate.Deployment.ModelAliasID, aliasProviderID, providerAccountID),
	)
}

func validateCIManifestRemoteChallengeVersion(cmd *cobra.Command, rc *RunContext, result *ciManifestRemoteValidationResult, workspaceID, versionID string) (string, string, error) {
	packs, err := listChallengePacksForWorkflow(cmd, rc, workspaceID)
	if err != nil {
		return "", "", err
	}
	for _, pack := range packs {
		for _, version := range pack.Versions {
			if version.ID == versionID {
				result.addPass("evaluation.challenge_pack_version_id", "challenge_pack_version", versionID, "challenge pack version exists in the selected workspace")
				return pack.ID, version.ID, nil
			}
		}
	}
	result.addFailure("evaluation.challenge_pack_version_id", "challenge_pack_version", versionID, "not_found", fmt.Sprintf("challenge pack version %s was not found in workspace %s", versionID, workspaceID))
	return "", "", nil
}

func validateCIManifestRemoteInputSet(cmd *cobra.Command, rc *RunContext, result *ciManifestRemoteValidationResult, workspaceID, challengePackVersionID, inputSetID string) error {
	if strings.TrimSpace(inputSetID) == "" {
		return nil
	}
	if strings.TrimSpace(challengePackVersionID) == "" {
		result.addFailure("evaluation.input_set_id", "challenge_input_set", inputSetID, "dependency_failed", "input set cannot be validated because evaluation.challenge_pack_version_id did not resolve")
		return nil
	}
	inputSets, err := listChallengeInputSets(cmd, rc, workspaceID, challengePackVersionID)
	if err != nil {
		return err
	}
	for _, inputSet := range inputSets {
		if inputSet.ID == inputSetID {
			result.addPass("evaluation.input_set_id", "challenge_input_set", inputSetID, "input set exists for the selected challenge pack version")
			return nil
		}
	}
	result.addFailure("evaluation.input_set_id", "challenge_input_set", inputSetID, "not_found", fmt.Sprintf("input set %s was not found for challenge pack version %s", inputSetID, challengePackVersionID))
	return nil
}

func validateCIManifestRemoteRegressionSuites(cmd *cobra.Command, rc *RunContext, result *ciManifestRemoteValidationResult, workspaceID, packID string, suiteIDs []string) ([]regressionSuiteSummary, []string, error) {
	suites, err := listRegressionSuites(cmd, rc, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if len(suiteIDs) == 0 {
		return suites, nil, nil
	}

	var selected []string
	byID := make(map[string]regressionSuiteSummary, len(suites))
	for _, suite := range suites {
		byID[suite.ID] = suite
	}
	for _, suiteID := range suiteIDs {
		suite, ok := byID[suiteID]
		if !ok {
			result.addFailure("evaluation.regression_suites", "regression_suite", suiteID, "not_found", fmt.Sprintf("regression suite %s was not found in workspace %s", suiteID, workspaceID))
			continue
		}
		if suite.WorkspaceID != "" && suite.WorkspaceID != workspaceID {
			result.addFailure("evaluation.regression_suites", "regression_suite", suiteID, "wrong_workspace", fmt.Sprintf("regression suite %s belongs to workspace %s, not %s", suiteID, suite.WorkspaceID, workspaceID))
			continue
		}
		if suite.Status != "" && suite.Status != "active" {
			result.addFailure("evaluation.regression_suites", "regression_suite", suiteID, "inactive", fmt.Sprintf("regression suite %s status is %s, want active", suiteID, suite.Status))
			continue
		}
		if packID != "" && suite.SourceChallengePackID != "" && suite.SourceChallengePackID != packID {
			result.addFailure("evaluation.regression_suites", "regression_suite", suiteID, "incompatible", fmt.Sprintf("regression suite %s belongs to challenge pack %s, not %s", suiteID, suite.SourceChallengePackID, packID))
			continue
		}
		selected = append(selected, suite.ID)
		result.addPass("evaluation.regression_suites", "regression_suite", suiteID, "regression suite exists and is compatible with the selected workload")
	}
	return suites, selected, nil
}

func validateCIManifestRemoteRegressionCases(cmd *cobra.Command, rc *RunContext, result *ciManifestRemoteValidationResult, workspaceID, challengePackVersionID, inputSetID string, suites []regressionSuiteSummary, selectedSuiteIDs []string, suiteScopeExplicit bool, caseIDs []string) error {
	if len(caseIDs) == 0 {
		return nil
	}
	if suiteScopeExplicit && len(selectedSuiteIDs) == 0 {
		for _, caseID := range caseIDs {
			result.addFailure("evaluation.regression_cases", "regression_case", caseID, "dependency_failed", "regression case cannot be validated because no explicit evaluation.regression_suites entry resolved")
		}
		return nil
	}
	searchSuiteIDs := selectedSuiteIDs
	if len(searchSuiteIDs) == 0 {
		for _, suite := range suites {
			if suite.Status == "" || suite.Status == "active" {
				searchSuiteIDs = append(searchSuiteIDs, suite.ID)
			}
		}
	}
	cases, err := listCIManifestRemoteRegressionCases(cmd, rc, workspaceID, searchSuiteIDs)
	if err != nil {
		return err
	}
	byID := make(map[string]ciRegressionCaseRemoteSummary, len(cases))
	for _, regressionCase := range cases {
		byID[regressionCase.ID] = regressionCase
	}
	for _, caseID := range caseIDs {
		regressionCase, ok := byID[caseID]
		if !ok {
			result.addFailure("evaluation.regression_cases", "regression_case", caseID, "not_found", fmt.Sprintf("regression case %s was not found in the selected workspace or suites", caseID))
			continue
		}
		if regressionCase.WorkspaceID != "" && regressionCase.WorkspaceID != workspaceID {
			result.addFailure("evaluation.regression_cases", "regression_case", caseID, "wrong_workspace", fmt.Sprintf("regression case %s belongs to workspace %s, not %s", caseID, regressionCase.WorkspaceID, workspaceID))
			continue
		}
		if regressionCase.Status != "" && regressionCase.Status != "active" {
			result.addFailure("evaluation.regression_cases", "regression_case", caseID, "inactive", fmt.Sprintf("regression case %s status is %s, want active", caseID, regressionCase.Status))
			continue
		}
		if challengePackVersionID != "" && regressionCase.SourceChallengePackVersionID != "" && regressionCase.SourceChallengePackVersionID != challengePackVersionID {
			result.addFailure("evaluation.regression_cases", "regression_case", caseID, "incompatible", fmt.Sprintf("regression case %s belongs to challenge pack version %s, not %s", caseID, regressionCase.SourceChallengePackVersionID, challengePackVersionID))
			continue
		}
		if inputSetID != "" && regressionCase.SourceChallengeInputSetID != "" && regressionCase.SourceChallengeInputSetID != inputSetID {
			result.addFailure("evaluation.regression_cases", "regression_case", caseID, "incompatible", fmt.Sprintf("regression case %s belongs to input set %s, not %s", caseID, regressionCase.SourceChallengeInputSetID, inputSetID))
			continue
		}
		result.addPass("evaluation.regression_cases", "regression_case", caseID, "regression case exists and is compatible with the selected workload")
	}
	return nil
}

func validateCIManifestRemoteBaseline(cmd *cobra.Command, rc *RunContext, result *ciManifestRemoteValidationResult, workspaceID, manifestPath string, manifest ciManifest) error {
	_, err := resolveCIManifestBaseline(cmd, rc, workspaceID, manifestPath, manifest, time.Now().UTC())
	field, resource, id := ciManifestRemoteBaselineField(manifest, err)
	if err == nil {
		result.addPass(field, resource, id, "baseline resolves to a completed run compatible with the selected workload")
		return nil
	}
	if apiErr, ok := ciRemoteAPIError(err); ok {
		if ciRemoteAPIErrorCanBeFieldFailure(apiErr) {
			result.addFailure(field, resource, id, ciRemoteAPIErrorCode(apiErr), apiErr.Error())
			return nil
		}
		result.addAPIError(err)
		return err
	}
	result.addFailure(field, resource, id, "incompatible", err.Error())
	return nil
}

func ciManifestRemoteBaselineField(manifest ciManifest, err error) (string, string, string) {
	if strings.TrimSpace(manifest.Baseline.RunAgentID) != "" && err != nil && strings.Contains(err.Error(), "baseline.run_agent_id") {
		return "baseline.run_agent_id", "run_agent", strings.TrimSpace(manifest.Baseline.RunAgentID)
	}
	if runID := strings.TrimSpace(manifest.Baseline.RunID); runID != "" {
		return "baseline.run_id", "run", runID
	}
	return "baseline.deployment_id", "agent_deployment", strings.TrimSpace(manifest.Baseline.DeploymentID)
}

const ciRemoteValidationPageLimit = 100

func listCIManifestRemoteObjects(cmd *cobra.Command, rc *RunContext, path string, query url.Values) ([]map[string]any, error) {
	var items []map[string]any
	for offset := 0; ; offset += ciRemoteValidationPageLimit {
		pageQuery := cloneURLValues(query)
		pageQuery.Set("limit", fmt.Sprintf("%d", ciRemoteValidationPageLimit))
		if offset > 0 {
			pageQuery.Set("offset", fmt.Sprintf("%d", offset))
		}
		resp, err := rc.Client.Get(cmd.Context(), path, pageQuery)
		if err != nil {
			return nil, err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return nil, apiErr
		}
		var result struct {
			Items []map[string]any `json:"items"`
			Total *int64           `json:"total,omitempty"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return nil, err
		}
		items = append(items, result.Items...)
		if result.Total == nil || len(result.Items) == 0 || len(result.Items) < ciRemoteValidationPageLimit || int64(len(items)) >= *result.Total {
			break
		}
	}
	return items, nil
}

func listCIManifestRemoteRegressionCases(cmd *cobra.Command, rc *RunContext, workspaceID string, suiteIDs []string) ([]ciRegressionCaseRemoteSummary, error) {
	var cases []ciRegressionCaseRemoteSummary
	for _, suiteID := range suiteIDs {
		suiteCasesFetched := 0
		for offset := 0; ; offset += ciRemoteValidationPageLimit {
			query := url.Values{}
			query.Set("limit", fmt.Sprintf("%d", ciRemoteValidationPageLimit))
			if offset > 0 {
				query.Set("offset", fmt.Sprintf("%d", offset))
			}
			resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/regression-suites/"+suiteID+"/cases", query)
			if err != nil {
				return nil, err
			}
			if apiErr := resp.ParseError(); apiErr != nil {
				return nil, apiErr
			}
			var result struct {
				Items []ciRegressionCaseRemoteSummary `json:"items"`
				Total *int64                          `json:"total,omitempty"`
			}
			if err := resp.DecodeJSON(&result); err != nil {
				return nil, err
			}
			cases = append(cases, result.Items...)
			suiteCasesFetched += len(result.Items)
			if result.Total == nil || len(result.Items) == 0 || len(result.Items) < ciRemoteValidationPageLimit || int64(suiteCasesFetched) >= *result.Total {
				break
			}
		}
	}
	return cases, nil
}

func cloneURLValues(values url.Values) url.Values {
	out := url.Values{}
	for key, entries := range values {
		out[key] = append([]string(nil), entries...)
	}
	return out
}

func findCIManifestRemoteObjectByID(items []map[string]any, id string) (map[string]any, bool) {
	for _, item := range items {
		if mapString(item, "id") == id {
			return item, true
		}
	}
	return nil, false
}

func ciRemoteWorkspaceMatches(result *ciManifestRemoteValidationResult, field, resource, id, resourceWorkspaceID, selectedWorkspaceID string) bool {
	if resourceWorkspaceID == "" || selectedWorkspaceID == "" || resourceWorkspaceID == selectedWorkspaceID {
		return true
	}
	result.addFailure(field, resource, id, "wrong_workspace", fmt.Sprintf("%s %s belongs to workspace %s, not %s", resource, id, resourceWorkspaceID, selectedWorkspaceID))
	return false
}

func (r *ciManifestRemoteValidationResult) addPass(field, resource, id, message string) {
	r.Checks = append(r.Checks, ciManifestRemoteCheck{
		Field:    field,
		Resource: resource,
		ID:       id,
		Valid:    true,
		Code:     "ok",
		Message:  message,
	})
}

func (r *ciManifestRemoteValidationResult) addFailure(field, resource, id, code, message string) {
	r.Valid = false
	r.Checks = append(r.Checks, ciManifestRemoteCheck{
		Field:    field,
		Resource: resource,
		ID:       id,
		Valid:    false,
		Code:     code,
		Message:  message,
	})
	r.Errors = append(r.Errors, fmt.Sprintf("%s: %s", field, message))
}

func (r *ciManifestRemoteValidationResult) addAPIError(err error) {
	r.Valid = false
	r.Errors = append(r.Errors, fmt.Sprintf("remote API error: %v", err))
}

func ciRemoteAPIError(err error) (*cliapi.APIError, bool) {
	var apiErr *cliapi.APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

func ciRemoteAPIErrorCanBeFieldFailure(apiErr *cliapi.APIError) bool {
	switch apiErr.StatusCode {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound:
		return true
	default:
		return false
	}
}

func ciRemoteAPIErrorCode(apiErr *cliapi.APIError) string {
	if apiErr.Code != "" {
		return apiErr.Code
	}
	if apiErr.StatusCode > 0 {
		return fmt.Sprintf("http_%d", apiErr.StatusCode)
	}
	return "api_error"
}

func validateCIManifest(manifest ciManifest) []string {
	var errors []string
	if manifest.Version != 1 {
		errors = append(errors, "version must be 1")
	}
	if len(nonEmptyStrings(manifest.Trigger.Paths)) == 0 {
		errors = append(errors, "trigger.paths must include at least one path glob")
	}
	if hasBlankString(manifest.Trigger.Paths) {
		errors = append(errors, "trigger.paths cannot include blank entries")
	}
	if strings.TrimSpace(manifest.Candidate.Build.AgentBuildID) == "" {
		errors = append(errors, "candidate.build.agent_build_id is required")
	}
	if strings.TrimSpace(manifest.Candidate.Build.SpecFile) == "" {
		errors = append(errors, "candidate.build.spec_file is required")
	}
	if strings.TrimSpace(manifest.Candidate.Deployment.RuntimeProfileID) == "" {
		errors = append(errors, "candidate.deployment.runtime_profile_id is required")
	}
	if strings.TrimSpace(manifest.Evaluation.ChallengePackVersionID) == "" {
		errors = append(errors, "evaluation.challenge_pack_version_id is required")
	}
	if hasBlankString(manifest.Evaluation.RegressionSuites) {
		errors = append(errors, "evaluation.regression_suites cannot include blank entries")
	}
	if hasBlankString(manifest.Evaluation.RegressionCases) {
		errors = append(errors, "evaluation.regression_cases cannot include blank entries")
	}
	if strings.TrimSpace(manifest.Baseline.RunID) == "" && strings.TrimSpace(manifest.Baseline.DeploymentID) == "" {
		errors = append(errors, "baseline.run_id or baseline.deployment_id is required")
	}
	if strings.TrimSpace(manifest.Baseline.RunID) != "" && strings.TrimSpace(manifest.Baseline.DeploymentID) != "" {
		errors = append(errors, "baseline.run_id and baseline.deployment_id are mutually exclusive")
	}
	if strings.TrimSpace(manifest.Baseline.RunAgentID) != "" && strings.TrimSpace(manifest.Baseline.RunID) == "" {
		errors = append(errors, "baseline.run_agent_id requires baseline.run_id")
	}
	if refresh := strings.TrimSpace(manifest.Baseline.Refresh); refresh != "" && !allowedCIManifestValue(refresh, "manual", "propose", "auto_on_main") {
		errors = append(errors, "baseline.refresh must be one of manual, propose, auto_on_main")
	}
	if manifest.Baseline.MaxAgeDays < 0 {
		errors = append(errors, "baseline.max_age_days must be greater than or equal to 0")
	}
	if failOn := strings.TrimSpace(manifest.Gate.FailOn); failOn == "" {
		errors = append(errors, "gate.fail_on is required")
	} else if !allowedCIManifestValue(failOn, "regression", "warning", "insufficient_evidence") {
		errors = append(errors, "gate.fail_on must be one of regression, warning, insufficient_evidence")
	}
	if promote := strings.TrimSpace(manifest.Regressions.PromoteFailures); promote == "" {
		errors = append(errors, "regressions.promote_failures is required")
	} else if !allowedCIManifestValue(promote, "disabled", "proposed", "auto_on_main") {
		errors = append(errors, "regressions.promote_failures must be one of disabled, proposed, auto_on_main")
	}
	return errors
}

func resolveCIManifestBaseline(cmd *cobra.Command, rc *RunContext, workspaceID, manifestPath string, manifest ciManifest, now time.Time) (ciBaselineResolution, error) {
	mode := ciBaselineRefreshMode(manifest.Baseline.Refresh)
	resolution := ciBaselineResolution{
		ManifestPath: manifestPath,
		WorkspaceID:  workspaceID,
		Refresh: ciBaselineRefreshResolution{
			Mode:       mode,
			NextAction: ciBaselineRefreshNextAction(mode),
		},
	}

	runID := strings.TrimSpace(manifest.Baseline.RunID)
	deploymentID := strings.TrimSpace(manifest.Baseline.DeploymentID)
	switch {
	case runID != "":
		run, err := getRunSummaryByID(cmd, rc, runID)
		if err != nil {
			return resolution, fmt.Errorf("resolving baseline.run_id %s: %w", runID, err)
		}
		if err := validateCIBaselineRun(workspaceID, manifest, run, now); err != nil {
			return resolution, err
		}

		runAgentID := strings.TrimSpace(manifest.Baseline.RunAgentID)
		if runAgentID != "" {
			agent, err := resolveRunAgentSummary(cmd, rc, run.ID, runAgentID)
			if err != nil {
				return resolution, fmt.Errorf("resolving baseline.run_agent_id %s: %w", runAgentID, err)
			}
			runAgentID = agent.ID
		}

		resolution.Strategy = "locked_run"
		resolution.Source = "baseline.run_id"
		resolution.Reason = "baseline.run_id pins the exact accepted run for pull request gates"
		resolution.Baseline = buildCIBaselineRunResolution(run, runAgentID, "", now)
		return resolution, nil

	case deploymentID != "":
		resolved, err := resolveCIBaselineDeployment(cmd, rc, workspaceID, manifest, deploymentID, now)
		if err != nil {
			return resolution, err
		}
		resolution.Strategy = "deployment_latest_completed"
		resolution.Source = "baseline.deployment_id"
		resolution.Reason = "baseline.deployment_id resolved to the newest completed compatible run for that deployment"
		resolution.Baseline = resolved
		resolution.Warnings = append(resolution.Warnings, "deployment baselines move when a newer compatible run exists; use baseline.run_id for fully locked PR gates")
		return resolution, nil

	default:
		return resolution, fmt.Errorf("baseline.run_id or baseline.deployment_id is required")
	}
}

func ciBaselineRefreshMode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "manual"
	}
	return value
}

func ciBaselineRefreshNextAction(mode string) string {
	switch mode {
	case "propose":
		return "after a successful mainline run, open a reviewed change that updates baseline.run_id"
	case "auto_on_main":
		return "after a successful protected mainline run, automation may update baseline.run_id with an auditable commit"
	default:
		return "after a successful mainline run, update baseline.run_id intentionally in a reviewed change"
	}
}

const ciBaselineDeploymentRejectionLimit = 3

func resolveCIBaselineDeployment(cmd *cobra.Command, rc *RunContext, workspaceID string, manifest ciManifest, deploymentID string, now time.Time) (ciBaselineRunResolution, error) {
	runs, err := listCIBaselineRuns(cmd, rc, workspaceID)
	if err != nil {
		return ciBaselineRunResolution{}, fmt.Errorf("listing runs for baseline.deployment_id %s: %w", deploymentID, err)
	}
	sort.SliceStable(runs, func(i, j int) bool { return ciBaselineRunNewer(runs[i], runs[j]) })

	var rejectedCandidates []string
	recordRejection := func(err error) {
		if err == nil || len(rejectedCandidates) >= ciBaselineDeploymentRejectionLimit {
			return
		}
		rejectedCandidates = append(rejectedCandidates, err.Error())
	}

	for _, run := range runs {
		if err := validateCIBaselineRun(workspaceID, manifest, run, now); err != nil {
			recordRejection(err)
			if ciBaselineRunExhaustsFreshWindow(manifest.Baseline.MaxAgeDays, run, now) {
				break
			}
			continue
		}
		agents, err := listRunAgentsForWorkflow(cmd, rc, run.ID)
		if err != nil {
			return ciBaselineRunResolution{}, fmt.Errorf("listing agents for baseline candidate run %s: %w", run.ID, err)
		}
		matchedDeployment := false
		for _, agent := range agents {
			if agent.AgentDeploymentID != deploymentID {
				continue
			}
			matchedDeployment = true
			if agent.Status != "completed" {
				recordRejection(fmt.Errorf("baseline run %s agent %s for baseline.deployment_id %s status is %s, want completed", run.ID, agent.ID, deploymentID, ciBaselineStatusForError(agent.Status)))
				continue
			}
			return buildCIBaselineRunResolution(run, agent.ID, deploymentID, now), nil
		}
		if !matchedDeployment {
			recordRejection(fmt.Errorf("baseline run %s has no agent for baseline.deployment_id %s", run.ID, deploymentID))
		}
	}
	if len(rejectedCandidates) > 0 {
		return ciBaselineRunResolution{}, fmt.Errorf("no completed compatible baseline run found for baseline.deployment_id %s; newest rejected baseline candidates: %s", deploymentID, strings.Join(rejectedCandidates, "; "))
	}
	return ciBaselineRunResolution{}, fmt.Errorf("no completed compatible baseline run found for baseline.deployment_id %s", deploymentID)
}

func ciBaselineRunNewer(a, b runWorkflowSummary) bool {
	aTimestamp, aOK := ciBaselineRunTimestamp(a)
	bTimestamp, bOK := ciBaselineRunTimestamp(b)
	if aOK && bOK && !aTimestamp.Equal(bTimestamp) {
		return aTimestamp.After(bTimestamp)
	}
	if aOK != bOK {
		return aOK
	}
	if a.CreatedAt != b.CreatedAt {
		return a.CreatedAt > b.CreatedAt
	}
	return a.ID > b.ID
}

func listCIBaselineRuns(cmd *cobra.Command, rc *RunContext, workspaceID string) ([]runWorkflowSummary, error) {
	const pageLimit = 100

	var runs []runWorkflowSummary
	for offset := 0; ; offset += pageLimit {
		query := url.Values{}
		query.Set("limit", fmt.Sprintf("%d", pageLimit))
		if offset > 0 {
			query.Set("offset", fmt.Sprintf("%d", offset))
		}
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/runs", query)
		if err != nil {
			return nil, err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return nil, apiErr
		}

		var result struct {
			Items []runWorkflowSummary `json:"items"`
			Total int64                `json:"total"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return nil, err
		}

		runs = append(runs, result.Items...)
		if len(result.Items) == 0 {
			break
		}
		if result.Total > 0 && int64(len(runs)) >= result.Total {
			break
		}
		if len(result.Items) < pageLimit {
			break
		}
	}
	return runs, nil
}

func validateCIBaselineRun(workspaceID string, manifest ciManifest, run runWorkflowSummary, now time.Time) error {
	if run.WorkspaceID != "" && workspaceID != "" && run.WorkspaceID != workspaceID {
		return fmt.Errorf("baseline run %s belongs to workspace %s, not %s", run.ID, run.WorkspaceID, workspaceID)
	}
	if run.Status != "completed" {
		return fmt.Errorf("baseline run %s status is %s, want completed", run.ID, run.Status)
	}
	if run.ChallengePackVersionID != "" && manifest.Evaluation.ChallengePackVersionID != "" && run.ChallengePackVersionID != manifest.Evaluation.ChallengePackVersionID {
		return fmt.Errorf("baseline run %s uses challenge pack version %s, not %s", run.ID, run.ChallengePackVersionID, manifest.Evaluation.ChallengePackVersionID)
	}
	if manifest.Evaluation.InputSetID != "" && run.ChallengeInputSetID != manifest.Evaluation.InputSetID {
		return fmt.Errorf("baseline run %s uses input set %s, not %s", run.ID, run.ChallengeInputSetID, manifest.Evaluation.InputSetID)
	}
	if err := validateCIBaselineAge(manifest.Baseline.MaxAgeDays, run, now); err != nil {
		return err
	}
	return nil
}

func validateCIBaselineAge(maxAgeDays int, run runWorkflowSummary, now time.Time) error {
	if maxAgeDays <= 0 {
		return nil
	}
	timestamp, ok := ciBaselineRunTimestamp(run)
	if !ok {
		return fmt.Errorf("baseline run %s has no created_at or finished_at timestamp for max_age_days", run.ID)
	}
	if now.Sub(timestamp) > time.Duration(maxAgeDays)*24*time.Hour {
		return fmt.Errorf("baseline run %s is older than baseline.max_age_days (%d)", run.ID, maxAgeDays)
	}
	return nil
}

func ciBaselineRunExhaustsFreshWindow(maxAgeDays int, run runWorkflowSummary, now time.Time) bool {
	if maxAgeDays <= 0 {
		return false
	}
	timestamp, ok := ciBaselineRunTimestamp(run)
	if !ok {
		return true
	}
	return now.Sub(timestamp) > time.Duration(maxAgeDays)*24*time.Hour
}

func ciBaselineStatusForError(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "<missing>"
	}
	return fmt.Sprintf("%q", status)
}

func ciBaselineRunTimestamp(run runWorkflowSummary) (time.Time, bool) {
	for _, raw := range []string{run.FinishedAt, run.CreatedAt} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, raw)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func buildCIBaselineRunResolution(run runWorkflowSummary, runAgentID string, deploymentID string, now time.Time) ciBaselineRunResolution {
	out := ciBaselineRunResolution{
		RunID:                  run.ID,
		RunAgentID:             runAgentID,
		DeploymentID:           deploymentID,
		RunName:                run.Name,
		Status:                 run.Status,
		ChallengePackVersionID: run.ChallengePackVersionID,
		InputSetID:             run.ChallengeInputSetID,
		CreatedAt:              run.CreatedAt,
		FinishedAt:             run.FinishedAt,
	}
	if timestamp, ok := ciBaselineRunTimestamp(run); ok {
		out.AgeDays = int(now.Sub(timestamp).Hours() / 24)
	}
	return out
}

func allowedCIManifestValue(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func hasBlankString(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func evaluateCIShouldRun(path string, manifest ciManifest, changedFiles []string, labels []string) (ciShouldRunResult, error) {
	result := ciShouldRunResult{
		Path:             path,
		ChangedFiles:     normalizeCIValues(changedFiles),
		Labels:           normalizeCIValues(labels),
		CheckedPathGlobs: normalizeCIValues(manifest.Trigger.Paths),
		CheckedLabels:    normalizeCIValues(manifest.Trigger.Labels),
	}

	for _, pattern := range result.CheckedPathGlobs {
		if err := ciValidateGlob(pattern); err != nil {
			return result, err
		}
	}
	for _, pattern := range result.CheckedPathGlobs {
		for _, file := range result.ChangedFiles {
			matched, err := ciGlobMatches(pattern, file)
			if err != nil {
				return result, err
			}
			if matched {
				result.MatchedPaths = append(result.MatchedPaths, ciShouldRunPathMatch{
					Pattern: pattern,
					File:    file,
				})
			}
		}
	}

	allowedLabels := make(map[string]struct{}, len(result.CheckedLabels))
	for _, label := range result.CheckedLabels {
		allowedLabels[label] = struct{}{}
	}
	seenLabels := map[string]struct{}{}
	for _, label := range result.Labels {
		if _, ok := allowedLabels[label]; ok {
			if _, seen := seenLabels[label]; !seen {
				result.MatchedLabels = append(result.MatchedLabels, label)
				seenLabels[label] = struct{}{}
			}
		}
	}

	result.ShouldRun = len(result.MatchedPaths) > 0 || len(result.MatchedLabels) > 0
	result.Reason = ciShouldRunReason(result)
	return result, nil
}

func ciShouldRunReason(result ciShouldRunResult) string {
	switch {
	case len(result.MatchedPaths) > 0 && len(result.MatchedLabels) > 0:
		return "changed files matched trigger.paths and labels matched trigger.labels"
	case len(result.MatchedPaths) > 0:
		return "changed files matched trigger.paths"
	case len(result.MatchedLabels) > 0:
		return "labels matched trigger.labels"
	case len(result.ChangedFiles) == 0 && len(result.Labels) == 0:
		return "no changed files or labels were provided"
	default:
		return "no changed files or labels matched manifest triggers"
	}
}

func normalizeCIValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeCIPath(value string) string {
	path := filepath.ToSlash(strings.TrimSpace(value))
	path = strings.TrimPrefix(path, "./")
	return path
}

func ciValidateGlob(pattern string) error {
	normalized := normalizeCIPath(pattern)
	if normalized == "" {
		return fmt.Errorf("invalid trigger glob %q: cannot be blank", pattern)
	}
	if !doublestar.ValidatePattern(normalized) {
		return fmt.Errorf("invalid trigger glob %q", pattern)
	}
	return nil
}

func ciGlobMatches(pattern string, file string) (bool, error) {
	if err := ciValidateGlob(pattern); err != nil {
		return false, err
	}
	normalized := normalizeCIPath(pattern)
	matched, err := doublestar.Match(normalized, normalizeCIPath(file))
	if err != nil {
		return false, fmt.Errorf("invalid trigger glob %q: %w", pattern, err)
	}
	return matched, nil
}

func defaultCIShouldRunRefs(base string, head string) (string, string) {
	if strings.TrimSpace(base) == "" {
		if envBase := strings.TrimSpace(os.Getenv("AGENTCLASH_CI_BASE")); envBase != "" {
			base = envBase
		} else if githubBase := strings.TrimSpace(os.Getenv("GITHUB_BASE_REF")); githubBase != "" {
			base = "origin/" + githubBase
		}
	}
	if strings.TrimSpace(head) == "" {
		if envHead := strings.TrimSpace(os.Getenv("AGENTCLASH_CI_HEAD")); envHead != "" {
			head = envHead
		} else if githubSHA := strings.TrimSpace(os.Getenv("GITHUB_SHA")); githubSHA != "" {
			head = githubSHA
		} else if strings.TrimSpace(base) != "" {
			head = "HEAD"
		}
	}
	return strings.TrimSpace(base), strings.TrimSpace(head)
}

func gitChangedFiles(repo string, base string, head string) ([]string, error) {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(head) == "" {
		return nil, fmt.Errorf("--base and --head are required to derive changed files")
	}
	diffRange := fmt.Sprintf("%s...%s", base, head)
	cmd := exec.Command("git", "-C", repo, "diff", "--name-only", "--diff-filter=ACDMRTUXB", diffRange)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return nil, fmt.Errorf("deriving changed files with git diff: %s", output.SanitizeLine(stderr))
			}
		}
		return nil, fmt.Errorf("deriving changed files with git diff: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return normalizeCIValues(lines), nil
}
