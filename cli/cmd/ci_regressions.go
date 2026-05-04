package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

type ciRunRegressionPromotionResult struct {
	Policy     string                         `json:"policy" yaml:"policy"`
	CaseStatus string                         `json:"case_status,omitempty" yaml:"case_status,omitempty"`
	Created    []ciRunRegressionPromotionCase `json:"created,omitempty" yaml:"created,omitempty"`
	Existing   []ciRunRegressionPromotionCase `json:"existing,omitempty" yaml:"existing,omitempty"`
	Skipped    []ciRunRegressionPromotionSkip `json:"skipped,omitempty" yaml:"skipped,omitempty"`
	Blocked    []ciRunRegressionPromotionSkip `json:"blocked,omitempty" yaml:"blocked,omitempty"`
	Errors     []string                       `json:"errors,omitempty" yaml:"errors,omitempty"`
}

type ciRunRegressionPromotionCase struct {
	SuiteID             string `json:"suite_id" yaml:"suite_id"`
	CaseID              string `json:"case_id,omitempty" yaml:"case_id,omitempty"`
	ChallengeIdentityID string `json:"challenge_identity_id,omitempty" yaml:"challenge_identity_id,omitempty"`
	ChallengeKey        string `json:"challenge_key,omitempty" yaml:"challenge_key,omitempty"`
	Status              string `json:"status,omitempty" yaml:"status,omitempty"`
	Created             bool   `json:"created" yaml:"created"`
}

type ciRunRegressionPromotionSkip struct {
	SuiteID             string `json:"suite_id,omitempty" yaml:"suite_id,omitempty"`
	ChallengeIdentityID string `json:"challenge_identity_id,omitempty" yaml:"challenge_identity_id,omitempty"`
	ChallengeKey        string `json:"challenge_key,omitempty" yaml:"challenge_key,omitempty"`
	Reason              string `json:"reason" yaml:"reason"`
	Message             string `json:"message" yaml:"message"`
}

type ciRunFailureReviewItem struct {
	RunAgentID             string   `json:"run_agent_id"`
	ChallengeIdentityID    string   `json:"challenge_identity_id"`
	ChallengeKey           string   `json:"challenge_key"`
	FailureFingerprint     string   `json:"failure_fingerprint"`
	FailureClusterKey      string   `json:"failure_cluster_key"`
	FailureState           string   `json:"failure_state"`
	FailureClass           string   `json:"failure_class"`
	Headline               string   `json:"headline"`
	Detail                 string   `json:"detail"`
	Promotable             bool     `json:"promotable"`
	PromotionModeAvailable []string `json:"promotion_mode_available"`
	Severity               string   `json:"severity"`
}

type ciRunRegressionCaseSummary struct {
	ID                  string `json:"id"`
	Status              string `json:"status"`
	ChallengeIdentityID string `json:"source_challenge_identity_id"`
	Title               string `json:"title"`
}

func promoteCIRunRegressionFailures(cmd *cobra.Command, rc *RunContext, workspaceID string, manifest ciManifest, result ciRunResult, releaseGate map[string]any) *ciRunRegressionPromotionResult {
	policy := strings.TrimSpace(manifest.Regressions.PromoteFailures)
	if policy == "" || result.GateVerdict != "fail" {
		return nil
	}

	summary := &ciRunRegressionPromotionResult{Policy: policy}
	if policy == "disabled" {
		summary.Skipped = append(summary.Skipped, ciRunRegressionPromotionSkip{
			Reason:  "policy_disabled",
			Message: "regressions.promote_failures is disabled",
		})
		return summary
	}

	caseStatus := "proposed"
	if policy == "auto_on_main" {
		if block := ciRunAutoMainPromotionBlock(result.Candidate.CIMetadata); block != nil {
			summary.Blocked = append(summary.Blocked, *block)
			return summary
		}
		caseStatus = "active"
	}
	summary.CaseStatus = caseStatus

	if len(manifest.Evaluation.RegressionSuites) == 0 {
		summary.Blocked = append(summary.Blocked, ciRunRegressionPromotionSkip{
			Reason:  "no_regression_suites",
			Message: "evaluation.regression_suites must include at least one target suite before CI can propose regression candidates",
		})
		return summary
	}

	failures, err := listCIRunFailures(cmd, rc, workspaceID, result.Candidate.RunID, result.Candidate.RunAgentID)
	if err != nil {
		summary.Errors = append(summary.Errors, fmt.Sprintf("list failure review items: %v", err))
		return summary
	}
	if len(failures) == 0 {
		summary.Skipped = append(summary.Skipped, ciRunRegressionPromotionSkip{
			Reason:  "no_failure_review_items",
			Message: "candidate run has no promotable failure review items",
		})
		return summary
	}

	for _, suiteID := range manifest.Evaluation.RegressionSuites {
		existingCases, err := listCIRunRegressionCases(cmd, rc, workspaceID, suiteID)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("list regression cases for suite %s: %v", suiteID, err))
			continue
		}
		existingByChallenge := ciRunExistingCaseByChallenge(existingCases)
		for _, failure := range failures {
			if !failure.Promotable {
				summary.Skipped = append(summary.Skipped, ciRunRegressionPromotionSkip{
					SuiteID:             suiteID,
					ChallengeIdentityID: failure.ChallengeIdentityID,
					ChallengeKey:        failure.ChallengeKey,
					Reason:              "not_promotable",
					Message:             "failure review item is not promotable",
				})
				continue
			}
			if strings.TrimSpace(failure.ChallengeIdentityID) == "" {
				summary.Skipped = append(summary.Skipped, ciRunRegressionPromotionSkip{
					SuiteID:      suiteID,
					ChallengeKey: failure.ChallengeKey,
					Reason:       "missing_challenge_identity",
					Message:      "failure review item has no challenge identity id",
				})
				continue
			}
			if existing, ok := existingByChallenge[failure.ChallengeIdentityID]; ok {
				summary.Existing = append(summary.Existing, ciRunRegressionPromotionCase{
					SuiteID:             suiteID,
					CaseID:              existing.ID,
					ChallengeIdentityID: failure.ChallengeIdentityID,
					ChallengeKey:        failure.ChallengeKey,
					Status:              existing.Status,
				})
				continue
			}

			promotionMode := ciRunPreferredPromotionMode(failure.PromotionModeAvailable)
			if promotionMode == "" {
				summary.Skipped = append(summary.Skipped, ciRunRegressionPromotionSkip{
					SuiteID:             suiteID,
					ChallengeIdentityID: failure.ChallengeIdentityID,
					ChallengeKey:        failure.ChallengeKey,
					Reason:              "no_supported_promotion_mode",
					Message:             "failure review item does not expose a supported promotion mode",
				})
				continue
			}

			created, err := promoteCIRunFailure(cmd, rc, workspaceID, suiteID, caseStatus, promotionMode, failure, result, releaseGate, manifest)
			if err != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("promote challenge %s in suite %s: %v", failure.ChallengeIdentityID, suiteID, err))
				continue
			}
			if created.Created {
				summary.Created = append(summary.Created, created)
			} else {
				summary.Existing = append(summary.Existing, created)
			}
		}
	}
	return summary
}

func ciRunAutoMainPromotionBlock(metadata map[string]any) *ciRunRegressionPromotionSkip {
	eventName := strings.ToLower(strings.TrimSpace(str(metadata["event_name"])))
	ref := strings.TrimSpace(str(metadata["ref"]))
	if _, ok := metadata["pull_request_number"]; ok || strings.HasPrefix(eventName, "pull_request") || strings.HasPrefix(ref, "refs/pull/") {
		return &ciRunRegressionPromotionSkip{
			Reason:  "pull_request_event",
			Message: "auto_on_main refuses to promote regression candidates from pull request events",
		}
	}

	defaultBranch := strings.TrimSpace(str(metadata["default_branch"]))
	if defaultBranch == "" {
		return &ciRunRegressionPromotionSkip{
			Reason:  "missing_default_branch",
			Message: "auto_on_main requires default branch metadata; pass --ci-default-branch when your CI provider cannot expose it automatically",
		}
	}
	branch := strings.TrimSpace(str(metadata["branch"]))
	if branch == "" && strings.HasPrefix(ref, "refs/heads/") {
		branch = strings.TrimPrefix(ref, "refs/heads/")
	}
	if branch != defaultBranch {
		return &ciRunRegressionPromotionSkip{
			Reason:  "non_default_branch",
			Message: fmt.Sprintf("auto_on_main only promotes on default branch %q; current branch is %q", defaultBranch, branch),
		}
	}
	return nil
}

func listCIRunFailures(cmd *cobra.Command, rc *RunContext, workspaceID, runID, runAgentID string) ([]ciRunFailureReviewItem, error) {
	q := url.Values{}
	if strings.TrimSpace(runAgentID) != "" {
		q.Set("agent_id", runAgentID)
	}
	q.Set("limit", "200")
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/runs/"+runID+"/failures", q)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var result struct {
		Items []ciRunFailureReviewItem `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func listCIRunRegressionCases(cmd *cobra.Command, rc *RunContext, workspaceID, suiteID string) ([]ciRunRegressionCaseSummary, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/regression-suites/"+suiteID+"/cases", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var result struct {
		Items []ciRunRegressionCaseSummary `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func ciRunExistingCaseByChallenge(items []ciRunRegressionCaseSummary) map[string]ciRunRegressionCaseSummary {
	out := make(map[string]ciRunRegressionCaseSummary)
	for _, item := range items {
		switch strings.TrimSpace(item.Status) {
		case "archived", "rejected":
			continue
		}
		if id := strings.TrimSpace(item.ChallengeIdentityID); id != "" {
			out[id] = item
		}
	}
	return out
}

func ciRunPreferredPromotionMode(modes []string) string {
	for _, want := range []string{"full_executable", "output_only"} {
		for _, mode := range modes {
			if strings.TrimSpace(mode) == want {
				return want
			}
		}
	}
	return ""
}

func promoteCIRunFailure(cmd *cobra.Command, rc *RunContext, workspaceID, suiteID, caseStatus, promotionMode string, failure ciRunFailureReviewItem, result ciRunResult, releaseGate map[string]any, manifest ciManifest) (ciRunRegressionPromotionCase, error) {
	body := map[string]any{
		"run_agent_id":        failure.RunAgentID,
		"suite_id":            suiteID,
		"promotion_mode":      promotionMode,
		"title":               ciRunPromotionTitle(failure),
		"failure_summary":     ciRunPromotionFailureSummary(failure, result),
		"status":              caseStatus,
		"validator_overrides": nil,
		"metadata":            ciRunPromotionMetadata(failure, result, releaseGate, manifest),
	}
	if severity := strings.TrimSpace(failure.Severity); severity != "" {
		body["severity"] = severity
	}

	resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+workspaceID+"/runs/"+result.Candidate.RunID+"/failures/"+failure.ChallengeIdentityID+"/promote", body)
	if err != nil {
		return ciRunRegressionPromotionCase{}, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return ciRunRegressionPromotionCase{}, apiErr
	}
	var payload map[string]any
	if err := resp.DecodeJSON(&payload); err != nil {
		return ciRunRegressionPromotionCase{}, err
	}
	regressionCase := mapObject(payload, "case")
	if regressionCase == nil {
		regressionCase = payload
	}
	return ciRunRegressionPromotionCase{
		SuiteID:             suiteID,
		CaseID:              str(regressionCase["id"]),
		ChallengeIdentityID: failure.ChallengeIdentityID,
		ChallengeKey:        failure.ChallengeKey,
		Status:              firstNonEmptyString(str(regressionCase["status"]), caseStatus),
		Created:             resp.StatusCode == 201,
	}, nil
}

func ciRunPromotionTitle(failure ciRunFailureReviewItem) string {
	if headline := strings.TrimSpace(failure.Headline); headline != "" {
		return "CI regression: " + headline
	}
	if key := strings.TrimSpace(failure.ChallengeKey); key != "" {
		return "CI regression: " + key
	}
	return "CI regression candidate"
}

func ciRunPromotionFailureSummary(failure ciRunFailureReviewItem, result ciRunResult) string {
	if detail := strings.TrimSpace(failure.Detail); detail != "" {
		return detail
	}
	if result.FailureReason != "" {
		return result.FailureReason
	}
	return "Candidate failed the AgentClash CI release gate."
}

func ciRunPromotionMetadata(failure ciRunFailureReviewItem, result ciRunResult, releaseGate map[string]any, manifest ciManifest) map[string]any {
	metadata := map[string]any{
		"source":                       "agentclash_ci",
		"manifest_path":                result.ManifestPath,
		"promote_failures_policy":      manifest.Regressions.PromoteFailures,
		"gate_verdict":                 result.GateVerdict,
		"gate_reason":                  result.FailureReason,
		"challenge_pack_version_id":    manifest.Evaluation.ChallengePackVersionID,
		"source_challenge_identity_id": failure.ChallengeIdentityID,
		"source_challenge_key":         failure.ChallengeKey,
		"source_failure_fingerprint":   failure.FailureFingerprint,
		"source_failure_cluster_key":   failure.FailureClusterKey,
		"source_failure_state":         failure.FailureState,
		"source_failure_class":         failure.FailureClass,
		"source_failure_severity":      failure.Severity,
	}
	if result.Candidate.CIMetadata != nil {
		metadata["ci_metadata"] = result.Candidate.CIMetadata
	}
	if policyKey := mapString(releaseGate, "policy_key"); policyKey != "" {
		metadata["gate_policy_key"] = policyKey
	}
	if policyVersion := releaseGate["policy_version"]; policyVersion != nil {
		metadata["gate_policy_version"] = policyVersion
	}
	if fingerprint := mapString(releaseGate, "policy_fingerprint"); fingerprint != "" {
		metadata["gate_policy_fingerprint"] = fingerprint
	}
	return metadata
}
