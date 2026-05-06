# Codex Issue 584 Harness Rankings — Test Contract

## Functional Behavior
- Agent Harness suites expose reusable ranking evidence without duplicating challenge-pack judges, validators, scorecards, replay, or budget accounting.
- Ranking data is derived from canonical harness executions, immutable harness/suite/task/config snapshots, scorecards, and existing run/replay artifacts where available.
- Suite results report success@1, pass@k, pass^k, score, confidence interval, cost, latency, failure modes, retry/budget metadata, and task/harness grouping.
- Head-to-head race comparisons only compare attempts that share fair immutable constraints: suite version/task, repository/base branch, task prompt, budget/config fingerprint, and comparable execution grouping.
- Pass@k and pass^k are unavailable when fewer than k attempts exist, instead of fabricating certainty.
- Experiment snapshots remain stable even if harness, model, template, or suite config changes later.
- Long-running execution controls from prior work are reused and surfaced as ranking/budget context, not reimplemented.

## Unit Tests
- Repository aggregation tests cover:
  - success@1 and Wilson confidence intervals.
  - pass@k and pass^k math for repeated trials.
  - pass@k/pass^k unavailable when there are too few attempts.
  - ranking ordering by score, then success/cost/latency where appropriate.
  - fair pairwise grouping excludes mismatched repository/base branch/task/budget fingerprints.
  - immutable snapshots are read from execution/config snapshots rather than current harness mutable fields.
- API tests cover authorization, response shape, and workspace scoping for ranking endpoints.
- Tests should reuse existing scorecard and harness execution fixture patterns where possible.

## Integration / Functional Tests
- A suite with multiple tasks and repeated harness attempts can be aggregated into per-harness results and pairwise race summaries.
- Existing harness execution, suite, scorecard, replay, and budget repositories remain the sources of truth.
- No new duplicate evaluator/judge/validator pipeline is introduced for ranking.

## Smoke Tests
- `cd backend && go test ./internal/repository ./internal/api`
- `cd backend && go test ./...`
- `git diff --check`

## E2E Tests
- N/A — backend/API ranking work. End-to-end harness execution behavior is already covered by existing Agent Harness workflows; this PR adds aggregation over those records.

## Manual / cURL Tests
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/agent-harness-suites/$SUITE_ID/rankings"
# Expected: 200 with ranked harness results, metric summaries, fair pairwise comparisons, and immutable snapshot metadata.
```
