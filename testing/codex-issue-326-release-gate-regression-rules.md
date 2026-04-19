# codex/issue-326-release-gate-regression-rules — Test Contract

## Functional Behavior
- Release-gate policy accepts an optional `regression_gate_rules` object without breaking existing policy payloads or legacy persisted gate snapshots.
- A policy with `regression_gate_rules: null` or the field omitted behaves exactly like today’s release-gate evaluation.
- `regression_gate_rules.no_blocking_regression_failure=true` fails the gate when the candidate has any failing regression case with `blocking` severity in scope.
- `regression_gate_rules.no_new_blocking_failure_vs_baseline=true` fails the gate only for blocking-severity regression cases that passed on the baseline and failed on the candidate.
- `regression_gate_rules.max_warning_regression_failures=N` warns or fails the gate when candidate warning-severity regression failures exceed `N`, and the violation payload includes the observed count.
- `regression_gate_rules.suite_ids` limits rule evaluation to active regression cases in the referenced suites; omitted `suite_ids` evaluates all regression suites represented on the candidate run.
- Runs with no regression-case scoring results pass the regression rules trivially.
- Missing candidate regression evidence for rules that depend on candidate scoring results produces `insufficient_evidence` instead of silently passing the release gate.
- Missing baseline evidence for `no_new_blocking_failure_vs_baseline` does not fail the gate; it records a warning that the rule was not evaluable.
- Regression violations include enough evidence to deep-link the offending case: `regression_case_id`, fired rule, candidate scoring-result id, and replay step refs from the promoted regression case evidence when available.
- Regression evidence must stay inside the authorized workspace; cross-workspace regression case ids are rejected as invalid evidence and must not leak foreign suite ids or case ids into `evaluation_details`.
- Release-gate evaluate/list responses surface regression violations as typed data in `evaluation_details`, and persisted release-gate rows retain the same typed payload.

## Unit Tests
- `TestNormalizePolicyPreservesLegacyPoliciesWithoutRegressionRules` — omitted `regression_gate_rules` does not change legacy normalization/fingerprints.
- `TestNormalizePolicyNormalizesRegressionGateRules` — suite ids are sorted/deduped and negative thresholds are rejected.
- `TestEvaluateRegressionRulesNoBlockingFailure` — blocking candidate failures create violations and fail the gate.
- `TestEvaluateRegressionRulesNoNewBlockingFailureVsBaseline` — only baseline-pass to candidate-fail transitions create violations.
- `TestEvaluateRegressionRulesWarningThreshold` — warning failures over threshold emit typed violations with observed count.
- `TestEvaluateRegressionRulesSuiteScope` — only cases inside selected suites are considered.
- `TestEvaluateRegressionRulesMissingBaselineWarns` — rule #2 adds a warning and does not fail when baseline evidence is unavailable.
- `TestEvaluateMergesStandardAndRegressionResults` — regression rule failures/warnings compose correctly with existing threshold-based release-gate verdicts.
- `TestMergeEvaluationPreservesPrimaryThresholdFailure` — an existing threshold failure remains the primary reason when regression rules also fail.

## Integration / Functional Tests
- `TestReleaseGateManagerEvaluatePersistsRegressionViolations` — manager loads regression evidence, stores typed violations in `evaluation_details`, and updates the persisted release gate verdict/reason.
- `TestReleaseGateManagerEvaluateScopesRegressionRulesToSelectedSuites` — repository-backed evaluation ignores out-of-scope suites.
- `TestReleaseGateManagerEvaluateWarnsWhenBaselineRegressionEvidenceMissing` — manager preserves warning-only outcome when the baseline rule is not evaluable.
- `TestReleaseGateManagerEvaluateReturnsInsufficientEvidenceWhenCandidateRegressionEvidenceMissing` — candidate-side scorecard/evaluation metadata gaps block the regression gate with insufficient evidence.
- `TestReleaseGateManagerEvaluateRejectsCrossWorkspaceRegressionCaseEvidence` — foreign-workspace regression cases are not loaded into typed violations.

## Smoke Tests
- `go test ./backend/internal/releasegate ./backend/internal/api ./backend/internal/repository`
- `go test ./backend/...`

## E2E Tests
- N/A — backend-only release-gate behavior change; endpoint coverage and OpenAPI schema validation are sufficient for this issue.

## Manual / cURL Tests
```bash
curl -X POST http://localhost:8080/v1/release-gates/evaluate \
  -H "Content-Type: application/json" \
  -H "X-User-Id: <user-id>" \
  -H "X-Workspace-Memberships: <workspace-id>:workspace_member" \
  -d '{
    "baseline_run_id":"<baseline-run-id>",
    "candidate_run_id":"<candidate-run-id>",
    "policy":{
      "policy_key":"default",
      "policy_version":1,
      "require_comparable":true,
      "require_evidence_quality":true,
      "fail_on_candidate_failure":true,
      "fail_on_both_failed_differently":true,
      "required_dimensions":["correctness"],
      "dimensions":{"correctness":{"warn_delta":0.02,"fail_delta":0.05}},
      "regression_gate_rules":{
        "no_blocking_regression_failure":true,
        "no_new_blocking_failure_vs_baseline":true,
        "max_warning_regression_failures":0
      }
    }
  }'
# Expected: 200. response.release_gate.evaluation_details.regression_violations is a typed array.
# Expected: blocking candidate regressions flip verdict to fail and include regression_case_id + evidence refs.
```
