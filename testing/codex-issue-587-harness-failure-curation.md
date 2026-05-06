# Codex Issue 587 Harness Failure Curation — Test Contract

## Functional Behavior
- Agent Harness failures expose a taxonomy read model without duplicating challenge-pack failure review, scorecard, validator, judge, replay, or regression-promotion primitives.
- Failure classes cover setup, auth, tool misuse, incomplete implementation, no-op diff, test failure, overbroad diff, no PR, judge failure, timeout, and policy/privacy failure.
- Classification uses existing harness execution events, execution snapshots, scorecards, run/replay evidence, and failure review taxonomy concepts where possible.
- Human-edited classifications are preserved separately from suggested/derived classifications and remain authoritative on later reads.
- Dashboard aggregation groups failure modes by repository, task type, harness, model, template, and suite.
- Users can promote a prior successful or failed harness execution into an Agent Harness suite/private task bank as a `prior_harness_run` task with sanitized prompts, validators, judges, artifacts, and source metadata.
- Promotion must not leak private hidden prompts or raw validators in public fields; hidden details stay in task/evaluation snapshots.

## Unit Tests
- Classification tests cover each required failure class from event types and scorecard/evaluation evidence.
- Annotation tests cover human override precedence over suggested classification.
- Summary tests group failures by repo, task type, harness, model, template, and suite.
- Promotion tests verify sanitized `public_prompt`, hidden `task_prompt`, preserved validators/judges, source snapshot provenance, and suite version/task creation.

## Integration / Functional Tests
- API tests cover authorization, not-found behavior, classification reads, annotation writes, dashboard summaries, and promotion into a suite/private task bank.
- Repository tests reuse existing Agent Harness execution/suite fixtures where possible.

## Smoke Tests
- `cd backend && go test ./internal/repository ./internal/api`
- `cd backend && go test ./...`
- `git diff --check`

## E2E Tests
- N/A — backend/API curation work. UI consumption can build on the added read/promote endpoints.

## Manual / cURL Tests
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/agent-harness-executions/$EXECUTION_ID/failure-review"
# Expected: classification with suggested and effective failure class.

curl -X POST "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/agent-harness-executions/$EXECUTION_ID/promote-task" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"suite_id":"'$SUITE_ID'","title":"Curated harness task","public_prompt":"Fix the failing repo task."}'
# Expected: suite task sourced from prior_harness_run with private snapshots preserved.
```
