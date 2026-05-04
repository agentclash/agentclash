package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	ciInitCmd.Flags().Bool("force", false, "Overwrite an existing manifest")
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
		result, err := validateCIManifestFile(args[0])
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
	DeploymentID string `yaml:"deployment_id,omitempty" json:"deployment_id,omitempty"`
}

type ciManifestGate struct {
	FailOn     string `yaml:"fail_on" json:"fail_on"`
	PolicyFile string `yaml:"policy_file,omitempty" json:"policy_file,omitempty"`
}

type ciManifestRegressions struct {
	PromoteFailures string `yaml:"promote_failures" json:"promote_failures"`
}

type ciManifestValidationResult struct {
	Path     string      `json:"path" yaml:"path"`
	Valid    bool        `json:"valid" yaml:"valid"`
	Errors   []string    `json:"errors,omitempty" yaml:"errors,omitempty"`
	Manifest *ciManifest `json:"manifest,omitempty" yaml:"manifest,omitempty"`
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
