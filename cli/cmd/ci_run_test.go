package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIRunCreatesCandidateRunAndEvaluatesPassingGate(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, nil))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1s"}, srv.URL)
	if err != nil {
		t.Fatalf("ci run error: %v", err)
	}

	if result.ExitCode != 0 || result.GateVerdict != "pass" {
		t.Fatalf("result = %+v, want passing gate", result)
	}
	if result.Candidate.BuildVersionID != "version-candidate" || result.Candidate.DeploymentID != "dep-candidate" || result.Candidate.RunID != "run-candidate" || result.Candidate.RunAgentID != "agent-candidate" {
		t.Fatalf("candidate = %+v, want created version/deployment/run/agent ids", result.Candidate)
	}
	if result.Baseline.RunID != "00000000-0000-0000-0000-000000000008" {
		t.Fatalf("baseline run = %q, want manifest baseline", result.Baseline.RunID)
	}
	if result.BaselineResolution == nil || result.BaselineResolution.Strategy != "locked_run" {
		t.Fatalf("baseline resolution = %+v, want locked_run", result.BaselineResolution)
	}
	if result.BaselineResolution.Source != "baseline.run_id" || result.BaselineResolution.Refresh.Mode != "manual" {
		t.Fatalf("baseline resolution = %+v, want source and refresh policy", result.BaselineResolution)
	}
	if captures.BuildVersionBody["agent_kind"] != "llm_agent" {
		t.Fatalf("build version body = %+v, want spec payload", captures.BuildVersionBody)
	}
	if captures.DeploymentBody["build_version_id"] != "version-candidate" || captures.DeploymentBody["runtime_profile_id"] != "00000000-0000-0000-0000-000000000002" {
		t.Fatalf("deployment body = %+v, want manifest deployment resources", captures.DeploymentBody)
	}
	if deployments, ok := captures.RunBody["agent_deployment_ids"].([]any); !ok || len(deployments) != 1 || deployments[0] != "dep-candidate" {
		t.Fatalf("run body deployments = %#v, want dep-candidate", captures.RunBody["agent_deployment_ids"])
	}
	if captures.RunBody["challenge_pack_version_id"] != "00000000-0000-0000-0000-000000000005" {
		t.Fatalf("run body = %+v, want manifest challenge pack version", captures.RunBody)
	}
	if captures.GateBody["baseline_run_id"] != "00000000-0000-0000-0000-000000000008" || captures.GateBody["candidate_run_agent_id"] != "agent-candidate" {
		t.Fatalf("gate body = %+v, want baseline and candidate ids", captures.GateBody)
	}
}

func TestCIRunWritesSummaryAndArtifacts(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, nil))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.md")
	artifactDir := filepath.Join(dir, "artifacts")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
		"--summary-file", summaryPath,
		"--artifact-dir", artifactDir,
	}, srv.URL)
	if err != nil {
		t.Fatalf("ci run error: %v", err)
	}
	if result.Reports == nil || result.Reports.ArtifactDir != artifactDir || len(result.Reports.Artifacts) != 5 {
		t.Fatalf("reports = %+v, want summary and five artifacts", result.Reports)
	}

	summary := readTextFile(t, summaryPath)
	for _, snippet := range []string{
		"AgentClash CI Gate: PASS",
		"Challenge Pack Version",
		"00000000-0000-0000-0000-000000000005",
		"Baseline Run",
		"Candidate run",
		"default / v1 / fp-123",
		"candidate passed",
	} {
		if !strings.Contains(summary, snippet) {
			t.Fatalf("summary missing %q\n---\n%s", snippet, summary)
		}
	}

	for _, name := range []string{"result.json", "run.json", "scorecard.json", "comparison.json", "gate.json"} {
		path := filepath.Join(artifactDir, name)
		var envelope map[string]any
		readTestJSONFile(t, path, &envelope)
		if envelope["schema_version"] != ciRunArtifactSchemaVersion {
			t.Fatalf("%s schema_version = %v, want %s", name, envelope["schema_version"], ciRunArtifactSchemaVersion)
		}
		if envelope["challenge_pack_version_id"] != "00000000-0000-0000-0000-000000000005" {
			t.Fatalf("%s challenge_pack_version_id = %v", name, envelope["challenge_pack_version_id"])
		}
		if envelope["payload"] == nil {
			t.Fatalf("%s payload is nil", name)
		}
	}

	var resultEnvelope struct {
		Payload ciRunResult `json:"payload"`
	}
	readTestJSONFile(t, filepath.Join(artifactDir, "result.json"), &resultEnvelope)
	if resultEnvelope.Payload.Reports == nil || len(resultEnvelope.Payload.Reports.Artifacts) != 5 {
		t.Fatalf("result artifact reports = %+v, want result.json included", resultEnvelope.Payload.Reports)
	}
}

func TestCIRunUsesGitHubStepSummary(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, nil))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	summaryPath := filepath.Join(t.TempDir(), "github-step-summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)

	if _, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1s"}, srv.URL); err != nil {
		t.Fatalf("ci run error: %v", err)
	}

	summary := readTextFile(t, summaryPath)
	if !strings.Contains(summary, "AgentClash CI Gate: PASS") || !strings.Contains(summary, "Candidate Run") {
		t.Fatalf("github step summary = %q, want gate summary", summary)
	}
}

func TestCIRunSummaryFileWinsOverGitHubStepSummaryAppend(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, nil))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	summaryPath := filepath.Join(t.TempDir(), "summary.md")
	if err := os.WriteFile(summaryPath, []byte("old summary\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(summary) error: %v", err)
	}
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)

	if _, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
		"--summary-file", summaryPath,
	}, srv.URL); err != nil {
		t.Fatalf("ci run error: %v", err)
	}

	summary := readTextFile(t, summaryPath)
	if strings.Contains(summary, "old summary") {
		t.Fatalf("summary-file should truncate even when it equals GITHUB_STEP_SUMMARY\n---\n%s", summary)
	}
	if strings.Count(summary, "AgentClash CI Gate: PASS") != 1 {
		t.Fatalf("summary = %q, want exactly one gate summary", summary)
	}
}

func TestCIRunTimeoutWritesSummary(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"GET /v1/runs/run-candidate": jsonHandler(200, map[string]any{
			"id":      "run-candidate",
			"status":  "running",
			"web_url": "https://app.agentclash.dev/runs/run-candidate",
		}),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.md")
	artifactDir := filepath.Join(dir, "artifacts")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1ms",
		"--summary-file", summaryPath,
		"--github-step-summary=false",
		"--artifact-dir", artifactDir,
	}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitTimeout {
		t.Fatalf("exit code = %d, want timeout %d", got, ciRunExitTimeout)
	}
	if result.Reports == nil || len(result.Reports.SummaryFiles) != 1 || len(result.Reports.Artifacts) != 5 {
		t.Fatalf("reports = %+v, want timeout summary and artifacts", result.Reports)
	}
	summary := readTextFile(t, summaryPath)
	if !strings.Contains(summary, "timed out waiting for candidate run run-candidate") || !strings.Contains(summary, "Exit Code") {
		t.Fatalf("timeout summary missing error/exit code\n---\n%s", summary)
	}
	for _, name := range []string{"result.json", "run.json", "scorecard.json", "comparison.json", "gate.json"} {
		var envelope map[string]any
		readTestJSONFile(t, filepath.Join(artifactDir, name), &envelope)
		if envelope["schema_version"] != ciRunArtifactSchemaVersion {
			t.Fatalf("%s schema_version = %v, want %s", name, envelope["schema_version"], ciRunArtifactSchemaVersion)
		}
	}
}

func TestCIRunReportWriteFailurePreservesPrimaryExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"GET /v1/runs/run-candidate": jsonHandler(200, map[string]any{
			"id":      "run-candidate",
			"status":  "running",
			"web_url": "https://app.agentclash.dev/runs/run-candidate",
		}),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	notDir := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(notDir, []byte("file"), 0o644); err != nil {
		t.Fatalf("WriteFile(not-dir) error: %v", err)
	}

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1ms",
		"--summary-file", filepath.Join(notDir, "summary.md"),
		"--github-step-summary=false",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitTimeout {
		t.Fatalf("exit code = %d, want timeout %d", got, ciRunExitTimeout)
	}
	if result.ExitCode != ciRunExitTimeout {
		t.Fatalf("result exit code = %d, want timeout %d", result.ExitCode, ciRunExitTimeout)
	}
	if !strings.Contains(strings.Join(result.Errors, "\n"), "create ci summary directory") {
		t.Fatalf("errors = %#v, want report write error recorded", result.Errors)
	}
}

func TestCIRunReportWriteFailurePreservesGateExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, "fail"),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	notDir := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(notDir, []byte("file"), 0o644); err != nil {
		t.Fatalf("WriteFile(not-dir) error: %v", err)
	}

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
		"--summary-file", filepath.Join(notDir, "summary.md"),
		"--github-step-summary=false",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate fail %d", got, gateExitFail)
	}
	if result.ExitCode != gateExitFail {
		t.Fatalf("result exit code = %d, want gate fail %d", result.ExitCode, gateExitFail)
	}
	if !strings.Contains(strings.Join(result.Errors, "\n"), "create ci summary directory") {
		t.Fatalf("errors = %#v, want report write error recorded", result.Errors)
	}
}

func TestCIRunWritesSummaryAndArtifactsForNonPassingVerdicts(t *testing.T) {
	cases := []struct {
		verdict string
		code    int
	}{
		{verdict: "warn", code: gateExitWarn},
		{verdict: "fail", code: gateExitFail},
		{verdict: "insufficient_evidence", code: gateExitInsufficientEvidence},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.verdict, func(t *testing.T) {
			target := writeCIRunManifest(t)
			captures := &ciRunRouteCaptures{}
			srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
				"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, tc.verdict),
				"GET /v1/compare": jsonHandler(200, map[string]any{
					"state":       "ready",
					"status":      "regressed",
					"reason_code": "candidate_regressed",
					"regression_reasons": []string{
						"latency regressed by 22%",
					},
				}),
			}))
			defer srv.Close()
			t.Setenv("AGENTCLASH_TOKEN", "test-token")
			dir := t.TempDir()
			summaryPath := filepath.Join(dir, "summary.md")
			artifactDir := filepath.Join(dir, "artifacts")

			result, err, _ := runCIRunJSON(t, []string{
				"ci", "run",
				"--manifest", target,
				"-w", "ws-1",
				"--json",
				"--poll-interval", "1ms",
				"--timeout", "1s",
				"--summary-file", summaryPath,
				"--artifact-dir", artifactDir,
			}, srv.URL)
			if got := exitCodeOf(t, err); got != tc.code {
				t.Fatalf("exit code = %d, want %d", got, tc.code)
			}
			if result.Reports == nil || len(result.Reports.Artifacts) != 5 {
				t.Fatalf("reports = %+v, want artifacts despite non-passing gate", result.Reports)
			}
			summary := readTextFile(t, summaryPath)
			if !strings.Contains(summary, "AgentClash CI Gate: "+strings.ToUpper(tc.verdict)) || !strings.Contains(summary, "Regression: latency regressed by 22%") {
				t.Fatalf("summary for %s missing verdict/failure\n---\n%s", tc.verdict, summary)
			}
			var gateArtifact map[string]any
			readTestJSONFile(t, filepath.Join(artifactDir, "gate.json"), &gateArtifact)
			if gateArtifact["gate_verdict"] != tc.verdict {
				t.Fatalf("gate artifact verdict = %v, want %s", gateArtifact["gate_verdict"], tc.verdict)
			}
		})
	}
}

func TestCIRunProposesRegressionCandidatesOnFailingGate(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, ciRunRegressionPromotionRoutes(t, captures, "fail", nil)))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
		"--ci-provider", "github_actions",
		"--ci-repository", "acme/agent",
		"--ci-pull-request", "42",
		"--ci-branch", "feature/gate",
		"--ci-default-branch", "main",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate fail %d", got, gateExitFail)
	}
	if result.RegressionPromotions == nil || result.RegressionPromotions.CaseStatus != "proposed" || len(result.RegressionPromotions.Created) != 1 {
		t.Fatalf("regression promotions = %+v, want one proposed candidate", result.RegressionPromotions)
	}
	if len(captures.PromotionBodies) != 1 {
		t.Fatalf("promotion bodies = %+v, want one request", captures.PromotionBodies)
	}
	body := captures.PromotionBodies[0]
	if body["status"] != "proposed" || body["suite_id"] != "00000000-0000-0000-0000-000000000007" || body["run_agent_id"] != "agent-candidate" {
		t.Fatalf("promotion body = %+v, want proposed candidate payload", body)
	}
	metadata := mapObject(body, "metadata")
	ciMetadata := mapObject(metadata, "ci_metadata")
	if metadata["source"] != "agentclash_ci" || ciMetadata["pull_request_number"] != float64(42) {
		t.Fatalf("metadata = %+v, want ci metadata and source", metadata)
	}
	if metadata["source_failure_fingerprint"] != "frf-test" || metadata["source_failure_cluster_key"] != "frc-test" {
		t.Fatalf("metadata = %+v, want failure identity metadata", metadata)
	}
}

func TestCIRunDisabledRegressionPromotionSkipsFailureAPIs(t *testing.T) {
	target := writeCIRunManifestWith(t, func(manifest string) string {
		return strings.Replace(manifest, "  promote_failures: proposed", "  promote_failures: disabled", 1)
	})
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, "fail"),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate fail %d", got, gateExitFail)
	}
	if result.RegressionPromotions == nil || len(result.RegressionPromotions.Skipped) != 1 || result.RegressionPromotions.Skipped[0].Reason != "policy_disabled" {
		t.Fatalf("regression promotions = %+v, want disabled skip", result.RegressionPromotions)
	}
}

func TestCIRunAutoOnMainBlocksPullRequestPromotion(t *testing.T) {
	target := writeCIRunManifestWith(t, func(manifest string) string {
		return strings.Replace(manifest, "  promote_failures: proposed", "  promote_failures: auto_on_main", 1)
	})
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, "fail"),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
		"--ci-provider", "github_actions",
		"--ci-event", "pull_request",
		"--ci-pull-request", "42",
		"--ci-branch", "feature/gate",
		"--ci-default-branch", "main",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate fail %d", got, gateExitFail)
	}
	if result.RegressionPromotions == nil || len(result.RegressionPromotions.Blocked) != 1 || result.RegressionPromotions.Blocked[0].Reason != "pull_request_event" {
		t.Fatalf("regression promotions = %+v, want pull_request block", result.RegressionPromotions)
	}
}

func TestCIRunAutoOnMainBlocksNonDefaultBranchPromotion(t *testing.T) {
	target := writeCIRunManifestWith(t, func(manifest string) string {
		return strings.Replace(manifest, "  promote_failures: proposed", "  promote_failures: auto_on_main", 1)
	})
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, "fail"),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
		"--ci-provider", "github_actions",
		"--ci-event", "push",
		"--ci-branch", "release-candidate",
		"--ci-default-branch", "main",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate fail %d", got, gateExitFail)
	}
	if result.RegressionPromotions == nil || len(result.RegressionPromotions.Blocked) != 1 || result.RegressionPromotions.Blocked[0].Reason != "non_default_branch" {
		t.Fatalf("regression promotions = %+v, want non-default branch block", result.RegressionPromotions)
	}
}

func TestCIRunAutoOnMainPromotesActiveOnDefaultBranch(t *testing.T) {
	target := writeCIRunManifestWith(t, func(manifest string) string {
		return strings.Replace(manifest, "  promote_failures: proposed", "  promote_failures: auto_on_main", 1)
	})
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, ciRunRegressionPromotionRoutes(t, captures, "fail", nil)))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
		"--ci-provider", "github_actions",
		"--ci-event", "push",
		"--ci-branch", "main",
		"--ci-default-branch", "main",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate fail %d", got, gateExitFail)
	}
	if result.RegressionPromotions == nil || result.RegressionPromotions.CaseStatus != "active" || len(result.RegressionPromotions.Created) != 1 {
		t.Fatalf("regression promotions = %+v, want active auto promotion", result.RegressionPromotions)
	}
	if len(captures.PromotionBodies) != 1 || captures.PromotionBodies[0]["status"] != "active" {
		t.Fatalf("promotion bodies = %+v, want active status", captures.PromotionBodies)
	}
}

func TestCIRunRegressionPromotionSkipsExistingCase(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, ciRunRegressionPromotionRoutes(t, captures, "fail", []map[string]any{{
		"id":                           "case-existing",
		"status":                       "proposed",
		"source_challenge_identity_id": "challenge-1",
		"title":                        "Existing proposal",
		"metadata": map[string]any{
			"source_failure_cluster_key": "frc-from-another-failure",
		},
	}})))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate fail %d", got, gateExitFail)
	}
	if result.RegressionPromotions == nil || len(result.RegressionPromotions.Existing) != 1 || result.RegressionPromotions.Existing[0].CaseID != "case-existing" {
		t.Fatalf("regression promotions = %+v, want existing case skip", result.RegressionPromotions)
	}
	if result.RegressionPromotions.Existing[0].ChallengeIdentityID != "challenge-1" {
		t.Fatalf("existing challenge identity = %q, want current failure challenge identity", result.RegressionPromotions.Existing[0].ChallengeIdentityID)
	}
	if len(captures.PromotionBodies) != 0 {
		t.Fatalf("promotion bodies = %+v, want no promote request for existing case", captures.PromotionBodies)
	}
}

func TestCIRunRegressionPromotionSkipsExistingCaseByClusterKey(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, ciRunRegressionPromotionRoutes(t, captures, "fail", []map[string]any{{
		"id":                           "case-existing-cluster",
		"status":                       "proposed",
		"source_challenge_identity_id": "challenge-from-prior-pack-version",
		"title":                        "Existing cluster proposal",
		"metadata": map[string]any{
			"source_failure_cluster_key": "frc-test",
		},
	}})))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate fail %d", got, gateExitFail)
	}
	if result.RegressionPromotions == nil || len(result.RegressionPromotions.Existing) != 1 || result.RegressionPromotions.Existing[0].CaseID != "case-existing-cluster" {
		t.Fatalf("regression promotions = %+v, want existing cluster case", result.RegressionPromotions)
	}
	if result.RegressionPromotions.Existing[0].ChallengeIdentityID != "challenge-1" {
		t.Fatalf("existing challenge identity = %q, want current failure challenge identity", result.RegressionPromotions.Existing[0].ChallengeIdentityID)
	}
	if result.RegressionPromotions.Existing[0].FailureClusterKey != "frc-test" {
		t.Fatalf("existing failure cluster key = %q, want frc-test", result.RegressionPromotions.Existing[0].FailureClusterKey)
	}
	if len(captures.PromotionBodies) != 0 {
		t.Fatalf("promotion bodies = %+v, want no promote request for existing cluster case", captures.PromotionBodies)
	}
}

func TestCIRunRegressionPromotionIgnoresArchivedClusterMatch(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, ciRunRegressionPromotionRoutes(t, captures, "fail", []map[string]any{{
		"id":                           "case-archived-cluster",
		"status":                       "archived",
		"source_challenge_identity_id": "challenge-from-prior-pack-version",
		"title":                        "Archived cluster proposal",
		"metadata": map[string]any{
			"source_failure_cluster_key": "frc-test",
		},
	}})))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
	}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate fail %d", got, gateExitFail)
	}
	if result.RegressionPromotions == nil || len(result.RegressionPromotions.Created) != 1 || result.RegressionPromotions.Created[0].FailureClusterKey != "frc-test" {
		t.Fatalf("regression promotions = %+v, want new case because archived cluster is ignored", result.RegressionPromotions)
	}
	if len(captures.PromotionBodies) != 1 {
		t.Fatalf("promotion bodies = %+v, want one promote request", captures.PromotionBodies)
	}
}

func TestCIRunAttachesGitHubActionsMetadata(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, nil))
	defer srv.Close()
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte(`{"pull_request":{"number":42}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(event) error: %v", err)
	}
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REPOSITORY", "acme/agent")
	t.Setenv("GITHUB_HEAD_REF", "feature/gate")
	t.Setenv("GITHUB_REF", "refs/pull/42/merge")
	t.Setenv("GITHUB_SHA", "abc123")
	t.Setenv("GITHUB_WORKFLOW", "AgentClash gate")
	t.Setenv("GITHUB_RUN_ID", "99")
	t.Setenv("GITHUB_RUN_ATTEMPT", "2")
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	t.Setenv("GITHUB_EVENT_PATH", eventPath)

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1s"}, srv.URL)
	if err != nil {
		t.Fatalf("ci run error: %v", err)
	}

	metadata, ok := captures.RunBody["ci_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("run body = %+v, want ci_metadata", captures.RunBody)
	}
	if metadata["repository"] != "acme/agent" || metadata["pull_request_number"] != float64(42) || metadata["workflow_run_url"] != "https://github.com/acme/agent/actions/runs/99" {
		t.Fatalf("ci metadata = %+v, want GitHub Actions metadata", metadata)
	}
	if result.Candidate.CIMetadata["repository"] != "acme/agent" || result.Candidate.CIMetadata["pull_request_number"] != float64(42) {
		t.Fatalf("result metadata = %+v, want persisted metadata", result.Candidate.CIMetadata)
	}
}

func TestCIRunManualMetadataFlagsOverrideGitHubActions(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, nil))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REPOSITORY", "acme/env")
	t.Setenv("GITHUB_RUN_ID", "99")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
		"--ci-repository", "manual/repo",
		"--ci-pull-request", "7",
		"--ci-branch", "manual-branch",
		"--ci-workflow-run-url", "https://ci.example/runs/7",
	}, srv.URL)
	if err != nil {
		t.Fatalf("ci run error: %v", err)
	}

	metadata, ok := captures.RunBody["ci_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("run body = %+v, want ci_metadata", captures.RunBody)
	}
	if metadata["repository"] != "manual/repo" || metadata["pull_request_number"] != float64(7) || metadata["branch"] != "manual-branch" || metadata["workflow_run_url"] != "https://ci.example/runs/7" {
		t.Fatalf("ci metadata = %+v, want manual override metadata", metadata)
	}
	if result.Candidate.CIMetadata["repository"] != "manual/repo" {
		t.Fatalf("result metadata = %+v, want manual repo", result.Candidate.CIMetadata)
	}
}

func TestCIRunRejectsNonPositivePullRequestMetadata(t *testing.T) {
	target := writeCIRunManifest(t)
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, stderr := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--ci-pull-request", "0",
	}, srv.URL)
	if err == nil {
		t.Fatal("ci run error = nil, want validation error")
	}
	if result.ExitCode != ciRunExitInvalidManifest {
		t.Fatalf("exit code = %d, want %d", result.ExitCode, ciRunExitInvalidManifest)
	}
	if !strings.Contains(stderr, "--ci-pull-request must be greater than 0") {
		t.Fatalf("stderr = %q, want ci-pull-request validation", stderr)
	}
	if called {
		t.Fatal("API server was called despite invalid CI metadata")
	}
}

func TestCIRunRejectsInvalidManifestBeforeAPIWrites(t *testing.T) {
	target := writeCIManifest(t, `version: 1
trigger: {}
candidate: {}
evaluation: {}
baseline: {}
gate: {}
regressions: {}
`)
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitInvalidManifest {
		t.Fatalf("exit code = %d, want invalid manifest %d (err %v)", got, ciRunExitInvalidManifest, err)
	}
	if called {
		t.Fatal("ci run made an API request after invalid local manifest validation")
	}
	if result.ExitCode != ciRunExitInvalidManifest || !strings.Contains(strings.Join(result.Errors, "\n"), "trigger.paths") {
		t.Fatalf("result = %+v, want invalid manifest errors", result)
	}
	if result.BaselineResolution != nil {
		t.Fatalf("baseline_resolution = %+v, want omitted on early failure", result.BaselineResolution)
	}
}

func TestCIRunMissingWorkspaceReturnsJSONExitCode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_WORKSPACE", "")
	target := writeCIRunManifest(t)

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "--json"}, "http://unused")
	if got := exitCodeOf(t, err); got != ciRunExitInvalidManifest {
		t.Fatalf("exit code = %d, want invalid manifest %d (err %v)", got, ciRunExitInvalidManifest, err)
	}
	if result.ExitCode != ciRunExitInvalidManifest || !strings.Contains(strings.Join(result.Errors, "\n"), "no workspace specified") {
		t.Fatalf("result = %+v, want workspace error", result)
	}
}

func TestCIRunRejectsUnsafeSpecFilePaths(t *testing.T) {
	for _, path := range []string{
		"/tmp/agent.json",
		"../agent.json",
		".",
		"..",
	} {
		t.Run(path, func(t *testing.T) {
			if err := validateCIRunSpecPath(path); err == nil {
				t.Fatalf("validateCIRunSpecPath(%q) error = nil, want rejection", path)
			}
		})
	}
}

func TestCIRunRemoteDecodeErrorUsesAPIExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"GET /v1/agent-builds/00000000-0000-0000-0000-000000000001": invalidJSONHandler(200),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitAPI {
		t.Fatalf("exit code = %d, want API %d (err %v)", got, ciRunExitAPI, err)
	}
	if result.ExitCode != ciRunExitAPI {
		t.Fatalf("result = %+v, want API exit code", result)
	}
}

func TestCIRunBuildVersionDecodeErrorUsesAPIExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/agent-builds/00000000-0000-0000-0000-000000000001/versions": invalidJSONHandler(201),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitAPI {
		t.Fatalf("exit code = %d, want API %d (err %v)", got, ciRunExitAPI, err)
	}
	if result.ExitCode != ciRunExitAPI {
		t.Fatalf("result = %+v, want API exit code", result)
	}
}

func TestCIRunGateFailReturnsGateExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, "fail"),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1s"}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate failure %d (err %v)", got, gateExitFail, err)
	}
	if result.GateVerdict != "fail" || result.ExitCode != gateExitFail || result.FailureReason != "regression_detected" {
		t.Fatalf("result = %+v, want failing gate result", result)
	}
}

func TestCIRunTimeoutDoesNotEvaluateGate(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"GET /v1/runs/run-candidate": jsonHandler(200, map[string]any{
			"id":      "run-candidate",
			"status":  "running",
			"web_url": "https://app.agentclash.dev/runs/run-candidate",
		}),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1ms"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitTimeout {
		t.Fatalf("exit code = %d, want timeout %d (err %v)", got, ciRunExitTimeout, err)
	}
	if result.ExitCode != ciRunExitTimeout || result.Candidate.RunStatus != "running" {
		t.Fatalf("result = %+v, want running timeout result", result)
	}
	if captures.GateBody != nil {
		t.Fatalf("gate body = %+v, want no gate evaluation after timeout", captures.GateBody)
	}
}

func TestCIRunAPIErrorUsesAPIExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/agent-builds/00000000-0000-0000-0000-000000000001/versions": jsonHandler(500, map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "boom"},
		}),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1s"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitAPI {
		t.Fatalf("exit code = %d, want API %d (err %v)", got, ciRunExitAPI, err)
	}
	if result.ExitCode != ciRunExitAPI || !strings.Contains(strings.Join(result.Errors, "\n"), "internal_error") {
		t.Fatalf("result = %+v, want API error details", result)
	}
}

type ciRunRouteCaptures struct {
	BuildVersionBody map[string]any
	DeploymentBody   map[string]any
	RunBody          map[string]any
	GateBody         map[string]any
	PromotionBodies  []map[string]any
}

func writeCIRunManifest(t *testing.T) string {
	return writeCIRunManifestWith(t, func(manifest string) string { return manifest })
}

func writeCIRunManifestWith(t *testing.T, mutate func(string) string) string {
	t.Helper()
	t.Setenv("GITHUB_ACTIONS", "")
	dir, err := os.MkdirTemp(".", "ci-run-spec-*")
	if err != nil {
		t.Fatalf("MkdirTemp(spec) error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	specPath := filepath.Join(dir, "agent.json")
	if err := os.WriteFile(specPath, []byte(`{"agent_kind":"llm_agent","prompt_spec":{"system":"be useful"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(spec) error: %v", err)
	}
	manifest := strings.Replace(sampleCIManifestYAML, "    spec_file: .agentclash/agent.json", "    spec_file: "+specPath, 1)
	manifest = strings.Replace(manifest, "  max_age_days: 30\n", "", 1)
	manifest = mutate(manifest)
	return writeCIManifest(t, manifest)
}

func invalidJSONHandler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{`))
	}
}

func runCIRunJSON(t *testing.T, args []string, apiURL string) (ciRunResult, error, string) {
	t.Helper()
	stdout := captureStdout(t)
	err := executeCommand(t, args, apiURL)
	out := stdout.finish()
	var result ciRunResult
	if decodeErr := json.Unmarshal([]byte(out), &result); decodeErr != nil {
		t.Fatalf("json output parse error: %v\n---\n%s", decodeErr, out)
	}
	return result, err, out
}

func ciRunRoutes(t *testing.T, captures *ciRunRouteCaptures, overrides map[string]http.HandlerFunc) map[string]http.HandlerFunc {
	t.Helper()
	routes := remoteCIValidateRoutes(t, map[string]http.HandlerFunc{
		"POST /v1/agent-builds/00000000-0000-0000-0000-000000000001/versions": func(w http.ResponseWriter, r *http.Request) {
			decodeJSONBody(t, r, &captures.BuildVersionBody)
			jsonHandler(201, map[string]any{"id": "version-candidate", "version_status": "draft"})(w, r)
		},
		"POST /v1/agent-build-versions/version-candidate/ready": jsonHandler(200, map[string]any{
			"id":             "version-candidate",
			"version_status": "ready",
		}),
		"POST /v1/workspaces/ws-1/agent-deployments": func(w http.ResponseWriter, r *http.Request) {
			decodeJSONBody(t, r, &captures.DeploymentBody)
			jsonHandler(201, map[string]any{
				"id":      "dep-candidate",
				"name":    "pr-candidate",
				"status":  "active",
				"web_url": "https://app.agentclash.dev/deployments/dep-candidate",
			})(w, r)
		},
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			decodeJSONBody(t, r, &captures.RunBody)
			response := map[string]any{
				"id":      "run-candidate",
				"status":  "queued",
				"web_url": "https://app.agentclash.dev/runs/run-candidate",
			}
			if metadata := captures.RunBody["ci_metadata"]; metadata != nil {
				response["ci_metadata"] = metadata
			}
			jsonHandler(201, response)(w, r)
		},
		"GET /v1/runs/run-candidate": jsonHandler(200, map[string]any{
			"id":      "run-candidate",
			"status":  "completed",
			"web_url": "https://app.agentclash.dev/runs/run-candidate",
		}),
		"GET /v1/runs/run-candidate/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":                  "agent-candidate",
				"run_id":              "run-candidate",
				"label":               "candidate",
				"status":              "completed",
				"agent_deployment_id": "dep-candidate",
			}},
		}),
		"GET /v1/scorecards/agent-candidate": jsonHandler(200, map[string]any{
			"run_agent_id":  "agent-candidate",
			"state":         "ready",
			"overall_score": 0.97,
			"web_url":       "https://app.agentclash.dev/scorecards/agent-candidate",
			"replay_url":    "https://app.agentclash.dev/replays/agent-candidate",
			"scorecard": map[string]any{
				"passed": true,
				"dimensions": map[string]any{
					"latency": map[string]any{
						"passed": true,
						"score":  0.94,
					},
				},
			},
		}),
		"GET /v1/compare": jsonHandler(200, map[string]any{
			"state":       "ready",
			"status":      "comparable",
			"reason_code": "candidate_comparable",
			"web_url":     "https://app.agentclash.dev/compare/run-baseline/run-candidate",
			"evidence_quality": map[string]any{
				"warnings": []string{"cost evidence incomplete"},
			},
		}),
		"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, "pass"),
	})
	for key, handler := range overrides {
		routes[key] = handler
	}
	return routes
}

func ciRunRegressionPromotionRoutes(t *testing.T, captures *ciRunRouteCaptures, verdict string, existingCases []map[string]any) map[string]http.HandlerFunc {
	t.Helper()
	if existingCases == nil {
		existingCases = []map[string]any{}
	}
	return map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, verdict),
		"GET /v1/workspaces/ws-1/runs/run-candidate/failures": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("agent_id"); got != "agent-candidate" {
				t.Fatalf("failure query agent_id = %q, want agent-candidate", got)
			}
			jsonHandler(200, map[string]any{
				"items": []map[string]any{{
					"run_agent_id":             "agent-candidate",
					"challenge_identity_id":    "challenge-1",
					"challenge_key":            "challenge.one",
					"failure_fingerprint":      "frf-test",
					"failure_cluster_key":      "frc-test",
					"failure_state":            "failed",
					"failure_class":            "policy_violation",
					"headline":                 "Policy regression",
					"detail":                   "Candidate wrote outside the allowed workspace",
					"promotable":               true,
					"promotion_mode_available": []string{"output_only", "full_executable"},
					"severity":                 "blocking",
				}},
			})(w, r)
		},
		"GET /v1/workspaces/ws-1/regression-suites/00000000-0000-0000-0000-000000000007/cases": jsonHandler(200, map[string]any{
			"items": existingCases,
		}),
		"POST /v1/workspaces/ws-1/runs/run-candidate/failures/challenge-1/promote": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode promotion body: %v", err)
			}
			captures.PromotionBodies = append(captures.PromotionBodies, body)
			jsonHandler(201, map[string]any{
				"id":                           "case-created",
				"suite_id":                     body["suite_id"],
				"status":                       body["status"],
				"source_challenge_identity_id": "challenge-1",
			})(w, r)
		},
	}
}

func ciRunGateHandler(t *testing.T, captures *ciRunRouteCaptures, verdict string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		decodeJSONBody(t, r, &captures.GateBody)
		reason := "gate_passed"
		summary := "candidate passed"
		if verdict != "pass" {
			reason = "regression_detected"
			summary = "candidate regressed"
		}
		jsonHandler(200, map[string]any{
			"release_gate": map[string]any{
				"verdict":            verdict,
				"reason_code":        reason,
				"summary":            summary,
				"evidence_status":    verdict,
				"policy_key":         "default",
				"policy_version":     1,
				"policy_fingerprint": "fp-123",
				"evaluation_details": map[string]any{
					"triggered_conditions": []string{"latency_delta"},
				},
			},
		})(w, r)
	}
}

func decodeJSONBody(t *testing.T, r *http.Request, target *map[string]any) {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	*target = body
}

func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitCodeError, got %T (%v)", err, err)
	}
	return exitErr.Code
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", path, err)
	}
	return string(data)
}

func readTestJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("Unmarshal(%s) error: %v\n---\n%s", path, err, data)
	}
}

func TestCIRunMarkdownSummarySanitizesServerStrings(t *testing.T) {
	hostile := "bad|cell\nforged\x1b[2J"
	summary := renderCIRunMarkdownSummary(ciRunResult{
		ManifestPath:  ".agentclash/ci.yaml",
		WorkspaceID:   "ws-1",
		GateVerdict:   "fail",
		FailureReason: hostile,
		Baseline: ciBaselineRunResolution{
			RunID: "run-b",
		},
		Candidate: ciRunCandidateResult{
			RunID:  "run-c",
			RunURL: "javascript:alert(1)",
		},
	}, ciManifest{
		Evaluation: ciManifestEvaluation{ChallengePackVersionID: "cpv-1"},
	}, nil, map[string]any{
		"regression_reasons": []any{hostile},
	}, map[string]any{
		"verdict":     "fail",
		"summary":     hostile,
		"reason_code": hostile,
	})

	for _, forbidden := range []string{"\x1b", "javascript:alert", "\nforged"} {
		if strings.Contains(summary, forbidden) {
			t.Fatalf("summary leaked %q\n---\n%s", forbidden, summary)
		}
	}
	if !strings.Contains(summary, "bad\\|cell") && !strings.Contains(summary, "bad\\|cell forged") {
		t.Fatalf("summary did not escape markdown table pipe\n---\n%s", summary)
	}
	if strings.Count(summary, "bad\\|cell") == 0 {
		t.Fatalf("summary missing printable sanitized content\n---\n%s", summary)
	}
	if strings.Contains(summary, "Candidate run") {
		t.Fatalf("unsafe candidate link rendered\n---\n%s", summary)
	}
}
