# cli-hardening-and-npm-distribution — Test Contract

## Functional Behavior
Lock the CLI behavior to the current backend API contracts for the overlapping hardening work on this branch.

- `agentclash compare gate` must interpret the backend's nested `release_gate` payload and exit according to the returned `verdict`, not stale top-level fields.
- `agentclash deployment create` flag-driven requests must send the backend's required deployment fields with the correct JSON keys, including `agent_build_id` and `build_version_id`.
- `agentclash run ranking` must render the current `{state, message, ranking}` payload shape and handle ready, pending, and errored states correctly.
- `agentclash run scorecard` must render the current scorecard payload shape, including state handling and the nested scorecard document.
- `agentclash replay get` must treat `202 Accepted` as a pending replay response and avoid assuming success-table fields that no longer exist.
- `agentclash compare runs` non-JSON output must summarize the current comparison payload (`key_deltas`, `regression_reasons`, summary/evidence fields) instead of stale dimension-array assumptions.
- Any fixes added on top of the open hardening PR must preserve its structured-output behavior for JSON/YAML users.
- The backend module path migration must be internally consistent: tracked backend packages in the PR branch must import `github.com/agentclash/agentclash/backend/...`, not the legacy `github.com/Atharva-Kanherkar/agentclash/backend/...` path.
- The `Backend CI` GitHub Actions workflow must get past package resolution and compile/test the backend module without `no required module provides package github.com/Atharva-Kanherkar/...` errors.

## Unit Tests
- `TestCompareGateUsesNestedReleaseGateVerdict` — pass/warn/fail/insufficient-evidence map to the correct CLI exit behavior.
- `TestDeploymentCreateBuildVersionFlagsUseCurrentRequestShape` — flag-based deployment creation posts `agent_build_id` and `build_version_id`.
- `TestRunRankingHandlesCurrentAPIShape` — ranking table output reads `ranking.items`.
- `TestRunRankingHandlesPendingOrErroredStates` — non-200 ranking states surface helpful output and/or structured payloads.
- `TestRunScorecardHandlesCurrentAPIShape` — scorecard view reads `state`, scalar scores, and nested `scorecard`.
- `TestReplayGetPendingAcceptedResponse` — `202 Accepted` pending responses do not fall through into stale rendering.
- `TestCompareRunsUsesKeyDeltasAndRegressionReasons` — non-JSON compare output reflects the current backend payload.
- `go test ./...` from `backend/` must succeed for the packages touched by the module-path cleanup.

## Integration / Functional Tests
- `go test ./cmd` covers the touched command handlers and response-shape parsing paths.
- Structured output tests continue to pass for the hardening branch's JSON/YAML formatter changes.
- No existing CLI auth, SSE, or output-format tests regress while updating the overlapping commands.
- The failing `Backend CI / Build & Vet` job on PR #353 should be reproducible locally as stale-import compile errors before the fix and disappear after the import-path cleanup.

## Smoke Tests
- `go test ./...` in `cli/` passes.
- `agentclash compare gate --help` still documents the exit-code behavior accurately.
- `agentclash run events --output yaml` still streams YAML documents as introduced by the hardening branch.
- `cd backend && go test -short -count=1 ./...` completes without any unresolved `github.com/Atharva-Kanherkar/...` package imports.

## E2E Tests
N/A — not applicable for this change. The work is limited to CLI command wiring, formatting, and unit/integration coverage.

## Manual / cURL Tests
Use the backend payloads below as fixtures for command-level verification.

```bash
# compare gate expected payload shape
cat <<'JSON'
{
  "baseline_run_id": "00000000-0000-0000-0000-000000000001",
  "candidate_run_id": "00000000-0000-0000-0000-000000000002",
  "release_gate": {
    "verdict": "pass",
    "reason_code": "all_checks_passed",
    "summary": "Candidate passed all gate checks.",
    "evidence_status": "sufficient"
  }
}
JSON

# deployment create expected request keys
cat <<'JSON'
{
  "name": "prod-deploy",
  "agent_build_id": "00000000-0000-0000-0000-000000000010",
  "build_version_id": "00000000-0000-0000-0000-000000000011",
  "runtime_profile_id": "00000000-0000-0000-0000-000000000012",
  "provider_account_id": "00000000-0000-0000-0000-000000000013",
  "model_alias_id": "00000000-0000-0000-0000-000000000014"
}
JSON

# run ranking expected payload shape
cat <<'JSON'
{
  "state": "ready",
  "ranking": {
    "items": [
      {
        "rank": 1,
        "label": "baseline",
        "composite_score": 0.91,
        "correctness_score": 0.95,
        "reliability_score": 0.90,
        "latency_score": 0.88,
        "cost_score": 0.91
      }
    ]
  }
}
JSON

# backend module path cleanup sanity check
cd backend
rg -n "github\\.com/Atharva-Kanherkar/agentclash/backend" .
# Expected after the fix: no matches in tracked source files used by CI
```
