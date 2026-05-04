package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(ciCmd)
	ciCmd.AddCommand(ciInitCmd)
	ciCmd.AddCommand(ciValidateCmd)

	ciInitCmd.Flags().Bool("force", false, "Overwrite an existing manifest")
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
	if err := yaml.Unmarshal(data, &manifest); err != nil {
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
