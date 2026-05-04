package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	ciCmd.AddCommand(ciRunCmd)

	ciRunCmd.Flags().String("manifest", ".agentclash/ci.yaml", "Path to the AgentClash CI manifest")
	ciRunCmd.Flags().Bool("follow", false, "Stream run events while waiting for the candidate run")
	ciRunCmd.Flags().Duration("timeout", 30*time.Minute, "Maximum time to wait for the candidate run; 0 disables the timeout")
	ciRunCmd.Flags().Duration("poll-interval", 5*time.Second, "Polling interval while waiting for run completion")
	ciRunCmd.Flags().String("ci-provider", "", "CI provider metadata override")
	ciRunCmd.Flags().String("ci-repository", "", "Repository metadata override, for example owner/repo")
	ciRunCmd.Flags().Int("ci-pull-request", 0, "Positive pull request number metadata override")
	ciRunCmd.Flags().String("ci-branch", "", "Branch metadata override")
	ciRunCmd.Flags().String("ci-ref", "", "Git ref metadata override")
	ciRunCmd.Flags().String("ci-commit", "", "Commit SHA metadata override")
	ciRunCmd.Flags().String("ci-workflow", "", "Workflow name metadata override")
	ciRunCmd.Flags().String("ci-workflow-run-id", "", "Workflow run id metadata override")
	ciRunCmd.Flags().String("ci-workflow-run-attempt", "", "Workflow run attempt metadata override")
	ciRunCmd.Flags().String("ci-workflow-run-url", "", "Workflow run URL metadata override")
	ciRunCmd.Flags().String("ci-event", "", "CI event name metadata override")
}

const (
	ciRunExitInvalidManifest = 10
	ciRunExitAPI             = 20
	ciRunExitTimeout         = 30
	ciRunExitRunFailed       = 31
)

var ciRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the AgentClash CI workflow described by a manifest",
	Long: `Run the AgentClash CI workflow described by a manifest.

The command validates the manifest, creates a candidate build version and
deployment, starts the manifest workload, waits for the candidate run to finish,
resolves the baseline, and evaluates the release gate. It is intended for
GitHub Actions and other non-interactive CI jobs after checkout and auth setup.

Exit codes:
  0   pass
  1   release gate failed
  2   release gate warning
  3   insufficient gate evidence
  10  invalid manifest or local candidate spec
  20  API/auth failure
  30  candidate run timed out
  31  candidate run failed before gate evaluation

Missing workspace configuration exits 10 for this command so --json callers
still receive a machine-readable envelope.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		result, err := executeCIRun(cmd, rc)
		if rc.Output.IsStructured() {
			if printErr := rc.Output.PrintRaw(result); printErr != nil {
				return printErr
			}
		} else {
			renderCIRunResult(rc, result, err)
		}
		if err != nil {
			return ciRunExitForError(err)
		}
		return nil
	},
}

type ciRunResult struct {
	ManifestPath       string                            `json:"manifest_path" yaml:"manifest_path"`
	WorkspaceID        string                            `json:"workspace_id" yaml:"workspace_id"`
	RemoteValidation   *ciManifestRemoteValidationResult `json:"remote_validation,omitempty" yaml:"remote_validation,omitempty"`
	Candidate          ciRunCandidateResult              `json:"candidate" yaml:"candidate"`
	BaselineResolution *ciBaselineResolution             `json:"baseline_resolution,omitempty" yaml:"baseline_resolution,omitempty"`
	Baseline           ciBaselineRunResolution           `json:"baseline" yaml:"baseline"`
	ReleaseGate        map[string]any                    `json:"release_gate,omitempty" yaml:"release_gate,omitempty"`
	GateVerdict        string                            `json:"gate_verdict,omitempty" yaml:"gate_verdict,omitempty"`
	FailureReason      string                            `json:"failure_reason,omitempty" yaml:"failure_reason,omitempty"`
	ExitCode           int                               `json:"exit_code" yaml:"exit_code"`
	Errors             []string                          `json:"errors,omitempty" yaml:"errors,omitempty"`
}

type ciRunCandidateResult struct {
	AgentBuildID   string         `json:"agent_build_id" yaml:"agent_build_id"`
	BuildVersionID string         `json:"build_version_id,omitempty" yaml:"build_version_id,omitempty"`
	DeploymentID   string         `json:"deployment_id,omitempty" yaml:"deployment_id,omitempty"`
	RunID          string         `json:"run_id,omitempty" yaml:"run_id,omitempty"`
	RunAgentID     string         `json:"run_agent_id,omitempty" yaml:"run_agent_id,omitempty"`
	RunStatus      string         `json:"run_status,omitempty" yaml:"run_status,omitempty"`
	RunURL         string         `json:"run_url,omitempty" yaml:"run_url,omitempty"`
	DeploymentName string         `json:"deployment_name,omitempty" yaml:"deployment_name,omitempty"`
	CIMetadata     map[string]any `json:"ci_metadata,omitempty" yaml:"ci_metadata,omitempty"`
}

func executeCIRun(cmd *cobra.Command, rc *RunContext) (ciRunResult, error) {
	manifestPath, _ := cmd.Flags().GetString("manifest")
	workspaceID := strings.TrimSpace(rc.Workspace)
	if workspaceID == "" {
		msg := "no workspace specified. Run 'agentclash link' to choose a default workspace, or pass --workspace, set AGENTCLASH_WORKSPACE, or create .agentclash.yaml with 'agentclash init'."
		return ciRunResult{ManifestPath: manifestPath, ExitCode: ciRunExitInvalidManifest, Errors: []string{msg}}, &ExitCodeError{Code: ciRunExitInvalidManifest, Message: msg}
	}
	follow, _ := cmd.Flags().GetBool("follow")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
	if timeout < 0 {
		err := fmt.Errorf("--timeout must be greater than or equal to 0")
		return ciRunResult{ManifestPath: manifestPath, WorkspaceID: workspaceID, ExitCode: ciRunExitInvalidManifest, Errors: []string{err.Error()}}, &ExitCodeError{Code: ciRunExitInvalidManifest, Message: err.Error()}
	}
	if pollInterval <= 0 {
		err := fmt.Errorf("--poll-interval must be greater than 0")
		return ciRunResult{ManifestPath: manifestPath, WorkspaceID: workspaceID, ExitCode: ciRunExitInvalidManifest, Errors: []string{err.Error()}}, &ExitCodeError{Code: ciRunExitInvalidManifest, Message: err.Error()}
	}
	ciMetadata, err := ciMetadataFromFlags(cmd)
	if err != nil {
		return ciRunResult{ManifestPath: manifestPath, WorkspaceID: workspaceID, ExitCode: ciRunExitInvalidManifest, Errors: []string{err.Error()}}, &ExitCodeError{Code: ciRunExitInvalidManifest, Message: err.Error()}
	}

	result := ciRunResult{
		ManifestPath: manifestPath,
		WorkspaceID:  workspaceID,
	}

	validation, err := validateCIManifestFile(manifestPath)
	if err != nil {
		result.ExitCode = ciRunExitInvalidManifest
		result.Errors = append(result.Errors, validation.Errors...)
		return result, &ExitCodeError{Code: ciRunExitInvalidManifest, Message: err.Error()}
	}
	manifest := *validation.Manifest
	result.Candidate.AgentBuildID = manifest.Candidate.Build.AgentBuildID
	result.Candidate.DeploymentName = ciRunDeploymentName(manifest)

	remoteValidation, remoteErr := validateCIManifestRemote(cmd, rc, workspaceID, manifestPath, manifest)
	result.RemoteValidation = &remoteValidation
	if remoteErr != nil {
		result.ExitCode = ciRunExitAPI
		result.Errors = append(result.Errors, remoteValidation.Errors...)
		return result, &ExitCodeError{Code: result.ExitCode, Message: remoteErr.Error()}
	}
	if !remoteValidation.Valid {
		result.ExitCode = ciRunExitInvalidManifest
		result.Errors = append(result.Errors, remoteValidation.Errors...)
		return result, &ExitCodeError{Code: ciRunExitInvalidManifest, Message: "ci manifest remote validation failed"}
	}

	buildVersion, err := createCIBuildVersion(cmd, rc, manifest)
	if err != nil {
		result.ExitCode = ciRunExitAPI
		if ciRunIsSpecError(err) {
			result.ExitCode = ciRunExitInvalidManifest
		}
		result.Errors = append(result.Errors, err.Error())
		return result, &ExitCodeError{Code: result.ExitCode, Message: err.Error()}
	}
	result.Candidate.BuildVersionID = mapString(buildVersion, "id")

	readyVersion, err := markCIBuildVersionReady(cmd, rc, result.Candidate.BuildVersionID)
	if err != nil {
		result.ExitCode = ciRunExitAPI
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	if id := mapString(readyVersion, "id"); id != "" {
		result.Candidate.BuildVersionID = id
	}

	deployment, err := createCIDeployment(cmd, rc, workspaceID, manifest, result.Candidate.BuildVersionID)
	if err != nil {
		result.ExitCode = ciRunExitAPI
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	result.Candidate.DeploymentID = mapString(deployment, "id")
	if name := mapString(deployment, "name"); name != "" {
		result.Candidate.DeploymentName = name
	}

	baseline, err := resolveCIManifestBaseline(cmd, rc, workspaceID, manifestPath, manifest, time.Now().UTC())
	if err != nil {
		result.ExitCode = ciRunExitInvalidManifest
		if ciRunIsAPIError(err) {
			result.ExitCode = ciRunExitAPI
		}
		result.Errors = append(result.Errors, err.Error())
		return result, &ExitCodeError{Code: result.ExitCode, Message: err.Error()}
	}
	result.BaselineResolution = &baseline
	result.Baseline = baseline.Baseline

	run, err := createCIRun(cmd, rc, workspaceID, manifest, result.Candidate.DeploymentID, ciMetadata)
	if err != nil {
		result.ExitCode = ciRunExitAPI
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	result.Candidate.RunID = mapString(run, "id")
	result.Candidate.RunStatus = mapString(run, "status")
	result.Candidate.RunURL = ciRunLink(run)
	if metadata := mapObject(run, "ci_metadata"); metadata != nil {
		result.Candidate.CIMetadata = metadata
	} else if ciMetadata != nil {
		result.Candidate.CIMetadata = ciMetadata
	}

	if follow && !rc.Output.IsStructured() {
		fmt.Fprintln(os.Stderr)
		if err := streamRunEvents(cmd, rc, result.Candidate.RunID); err != nil {
			result.ExitCode = ciRunExitAPI
			result.Errors = append(result.Errors, err.Error())
			return result, err
		}
	}

	completedRun, err := waitForCIRunCompletion(cmd, rc, result.Candidate.RunID, timeout, pollInterval)
	if err != nil {
		result.Candidate.RunStatus = mapString(completedRun, "status")
		if link := ciRunLink(completedRun); link != "" {
			result.Candidate.RunURL = link
		}
		result.ExitCode = ciRunExitForError(err).(*ExitCodeError).Code
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	result.Candidate.RunStatus = mapString(completedRun, "status")
	if link := ciRunLink(completedRun); link != "" {
		result.Candidate.RunURL = link
	}

	candidateAgent, err := resolveCIRunCandidateAgent(cmd, rc, result.Candidate.RunID, result.Candidate.DeploymentID)
	if err != nil {
		result.ExitCode = ciRunExitAPI
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	result.Candidate.RunAgentID = candidateAgent.ID

	gateEnvelope, gateVerdict, err := evaluateCIRunReleaseGate(cmd, rc, result.Baseline.RunID, result.Candidate.RunID, result.Baseline.RunAgentID, result.Candidate.RunAgentID)
	if err != nil {
		result.ExitCode = ciRunExitAPI
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	result.ReleaseGate, _ = gateEnvelope["release_gate"].(map[string]any)
	result.GateVerdict = gateVerdict.ReleaseGate.Verdict
	result.FailureReason = ciRunGateFailureReason(result.ReleaseGate)
	result.ExitCode = ciRunExitCodeForGate(result.GateVerdict)
	if result.ExitCode != 0 {
		return result, &ExitCodeError{Code: result.ExitCode}
	}
	return result, nil
}

func createCIBuildVersion(cmd *cobra.Command, rc *RunContext, manifest ciManifest) (map[string]any, error) {
	body, err := ciRunSpecBody(manifest.Candidate.Build.SpecFile)
	if err != nil {
		return nil, err
	}
	resp, err := rc.Client.Post(cmd.Context(), "/v1/agent-builds/"+manifest.Candidate.Build.AgentBuildID+"/versions", body)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var version map[string]any
	if err := resp.DecodeJSON(&version); err != nil {
		return nil, err
	}
	return version, nil
}

func ciRunSpecBody(path string) (map[string]any, error) {
	if err := validateCIRunSpecPath(path); err != nil {
		return nil, &ciRunSpecError{err: err}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &ciRunSpecError{err: fmt.Errorf("reading candidate.build.spec_file: %w", err)}
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, &ciRunSpecError{err: fmt.Errorf("parsing candidate.build.spec_file: %w", err)}
	}
	return body, nil
}

type ciRunSpecError struct {
	err error
}

func (e *ciRunSpecError) Error() string {
	return e.err.Error()
}

func (e *ciRunSpecError) Unwrap() error {
	return e.err
}

func ciRunIsSpecError(err error) bool {
	var specErr *ciRunSpecError
	return errors.As(err, &specErr)
}

func validateCIRunSpecPath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("candidate.build.spec_file must be a relative path inside the repository")
	}
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("candidate.build.spec_file must stay inside the repository")
	}
	return nil
}

func markCIBuildVersionReady(cmd *cobra.Command, rc *RunContext, versionID string) (map[string]any, error) {
	if strings.TrimSpace(versionID) == "" {
		return nil, fmt.Errorf("created build version response did not include id")
	}
	resp, err := rc.Client.Post(cmd.Context(), "/v1/agent-build-versions/"+versionID+"/ready", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var version map[string]any
	if err := resp.DecodeJSON(&version); err != nil {
		return nil, err
	}
	return version, nil
}

func createCIDeployment(cmd *cobra.Command, rc *RunContext, workspaceID string, manifest ciManifest, buildVersionID string) (map[string]any, error) {
	body := map[string]any{
		"name":               ciRunDeploymentName(manifest),
		"agent_build_id":     manifest.Candidate.Build.AgentBuildID,
		"build_version_id":   buildVersionID,
		"runtime_profile_id": manifest.Candidate.Deployment.RuntimeProfileID,
	}
	if manifest.Candidate.Deployment.ProviderAccountID != "" {
		body["provider_account_id"] = manifest.Candidate.Deployment.ProviderAccountID
	}
	if manifest.Candidate.Deployment.ModelAliasID != "" {
		body["model_alias_id"] = manifest.Candidate.Deployment.ModelAliasID
	}
	resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+workspaceID+"/agent-deployments", body)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var deployment map[string]any
	if err := resp.DecodeJSON(&deployment); err != nil {
		return nil, err
	}
	return deployment, nil
}

func ciRunDeploymentName(manifest ciManifest) string {
	if name := strings.TrimSpace(manifest.Candidate.Deployment.Name); name != "" {
		return name
	}
	return fmt.Sprintf("agentclash-ci-%d", time.Now().UTC().Unix())
}

func createCIRun(cmd *cobra.Command, rc *RunContext, workspaceID string, manifest ciManifest, deploymentID string, ciMetadata map[string]any) (map[string]any, error) {
	request := runCreateRequest{
		ChallengePackVersionID: manifest.Evaluation.ChallengePackVersionID,
		ChallengeInputSetID:    manifest.Evaluation.InputSetID,
		DeploymentIDs:          []string{deploymentID},
		OfficialPackMode:       "full",
		RegressionSuiteIDs:     manifest.Evaluation.RegressionSuites,
		RegressionCaseIDs:      manifest.Evaluation.RegressionCases,
		Name:                   ciRunName(manifest),
		CIMetadata:             ciMetadata,
	}
	body, err := buildRunCreateBody(workspaceID, request)
	if err != nil {
		return nil, err
	}
	return createRun(cmd, rc, body)
}

func ciRunName(manifest ciManifest) string {
	if name := strings.TrimSpace(manifest.Candidate.Deployment.Name); name != "" {
		return "CI gate: " + name
	}
	return "CI gate"
}

func waitForCIRunCompletion(cmd *cobra.Command, rc *RunContext, runID string, timeout, pollInterval time.Duration) (map[string]any, error) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for {
		run, err := getCIRun(cmd, rc, runID)
		if err != nil {
			return run, err
		}
		terminal, ok := ciRunTerminalStatus(mapString(run, "status"))
		if terminal {
			if ok {
				return run, nil
			}
			return run, &ExitCodeError{Code: ciRunExitRunFailed, Message: fmt.Sprintf("candidate run %s finished with status %s", runID, mapString(run, "status"))}
		}
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			return run, &ExitCodeError{Code: ciRunExitTimeout, Message: fmt.Sprintf("timed out waiting for candidate run %s", runID)}
		}
		sleepFor := pollInterval
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining < sleepFor {
				sleepFor = remaining
			}
		}
		if sleepFor > 0 {
			if err := sleepCIRunPoll(cmd.Context(), sleepFor); err != nil {
				return run, err
			}
		}
	}
}

func sleepCIRunPoll(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func getCIRun(cmd *cobra.Command, rc *RunContext, runID string) (map[string]any, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+runID, nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var run map[string]any
	if err := resp.DecodeJSON(&run); err != nil {
		return nil, err
	}
	return run, nil
}

func ciRunTerminalStatus(status string) (terminal bool, success bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "succeeded", "success":
		return true, true
	case "failed", "error", "errored", "canceled", "cancelled", "aborted", "timed_out", "timeout", "expired":
		return true, false
	default:
		return false, false
	}
}

func resolveCIRunCandidateAgent(cmd *cobra.Command, rc *RunContext, runID, deploymentID string) (runAgentWorkflowSummary, error) {
	agents, err := listRunAgentsForWorkflow(cmd, rc, runID)
	if err != nil {
		return runAgentWorkflowSummary{}, err
	}
	for _, agent := range agents {
		if agent.AgentDeploymentID == deploymentID {
			return agent, nil
		}
	}
	if len(agents) == 1 {
		return agents[0], nil
	}
	return runAgentWorkflowSummary{}, fmt.Errorf("candidate run %s has no agent for deployment %s", runID, deploymentID)
}

func evaluateCIRunReleaseGate(cmd *cobra.Command, rc *RunContext, baselineRunID, candidateRunID, baselineAgentID, candidateAgentID string) (map[string]any, releaseGateVerdict, error) {
	body := map[string]any{
		"baseline_run_id":  baselineRunID,
		"candidate_run_id": candidateRunID,
	}
	if baselineAgentID != "" {
		body["baseline_run_agent_id"] = baselineAgentID
	}
	if candidateAgentID != "" {
		body["candidate_run_agent_id"] = candidateAgentID
	}
	resp, err := rc.Client.Post(cmd.Context(), "/v1/release-gates/evaluate", body)
	if err != nil {
		return nil, releaseGateVerdict{}, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, releaseGateVerdict{}, apiErr
	}
	var verdict releaseGateVerdict
	if err := resp.DecodeJSON(&verdict); err != nil {
		return nil, releaseGateVerdict{}, fmt.Errorf("decoding release gate response: %w", err)
	}
	var raw map[string]any
	if err := resp.DecodeJSON(&raw); err != nil {
		return nil, releaseGateVerdict{}, fmt.Errorf("decoding raw release gate response: %w", err)
	}
	return raw, verdict, nil
}

func ciRunExitCodeForGate(verdict string) int {
	switch verdict {
	case "pass":
		return gateExitPass
	case "warn":
		return gateExitWarn
	case "fail":
		return gateExitFail
	case "insufficient_evidence":
		return gateExitInsufficientEvidence
	default:
		return gateExitFail
	}
}

func ciRunGateFailureReason(releaseGate map[string]any) string {
	if releaseGate == nil {
		return ""
	}
	if reason := mapString(releaseGate, "reason_code"); reason != "" {
		return reason
	}
	return mapString(releaseGate, "summary")
}

func ciRunLink(run map[string]any) string {
	return mapString(run, "url", "web_url", "html_url")
}

func ciRunExitForError(err error) error {
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr
	}
	if ciRunIsAPIError(err) {
		return &ExitCodeError{Code: ciRunExitAPI, Message: err.Error()}
	}
	return &ExitCodeError{Code: 1, Message: err.Error()}
}

func ciRunIsAPIError(err error) bool {
	var apiErr *cliapi.APIError
	return errors.As(err, &apiErr)
}

func renderCIRunResult(rc *RunContext, result ciRunResult, err error) {
	if err != nil {
		rc.Output.PrintError("AgentClash CI run failed")
		for _, msg := range result.Errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", output.SanitizeLine(msg))
		}
		return
	}
	rc.Output.PrintSuccess("AgentClash CI run completed")
	rc.Output.PrintDetail("Run", result.Candidate.RunID)
	if result.Candidate.RunURL != "" {
		rc.Output.PrintDetail("Run URL", result.Candidate.RunURL)
	}
	rc.Output.PrintDetail("Candidate Agent", result.Candidate.RunAgentID)
	rc.Output.PrintDetail("Baseline", result.Baseline.RunID)
	rc.Output.PrintDetail("Gate", result.GateVerdict)
	switch result.GateVerdict {
	case "pass":
		rc.Output.PrintDetail("Next", "merge or promote when ready")
	case "warn", "insufficient_evidence":
		rc.Output.PrintDetail("Next", "review gate evidence before merging")
	case "fail":
		rc.Output.PrintDetail("Next", "fix the regression before merging")
	}
}
