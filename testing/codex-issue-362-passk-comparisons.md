# codex/issue-362-passk-comparisons — Test Contract

Locked from: `research-docs/issues/issue-362-passk-comparison-plan.md`

## Functional Behavior

- Extend repeated-eval config decoding and persistence to accept optional `eval_session.aggregation.reliability_weight` as a float in `[0, 1]`.
- Preserve backward compatibility for existing eval-session requests that omit `reliability_weight`.
- Keep inferred task metadata flowing through the existing `routing_task_snapshot` document rather than introducing a new typed storage object.
- Compute repeated-eval task outcomes per participant using challenge-level `judge_results` grouped by `challenge_identity_id`.
- When challenge-level task outcomes cannot be resolved for a participant, fall back to a synthetic suite-level task derived from the run-agent scorecard and emit evidence warnings.
- Compute participant-level `pass@k` and `pass^k` for `k = 1, 3, 5, 10`, plus `session.repetitions` when it is not already in that set.
- Ensure `k = 1` yields the same rate for `pass@k`, `pass^k`, and the observed task success rate.
- Use `success_threshold.min_pass_rate` as the continuous task-success threshold when provided; otherwise default to `0.8`.
- Prefer manual `aggregation.reliability_weight` over inferred routing weight when supplied.
- Infer routing weight from persisted task properties using the locked heuristic for:
  - `has_side_effects`
  - `autonomy`
  - `step_count`
  - `output_type`
- Persist and surface `metric_routing` with:
  - `source`
  - `reliability_weight`
  - `reasoning`
  - `primary_metric`
  - `effective_k`
  - `composite_agent_score`
- For single-agent sessions, surface top-level pass metrics and routing metadata in `aggregate_result`.
- For comparison sessions, surface per-participant pass metrics and a repeated-session `comparison` block.
- The repeated-session `comparison` block must return:
  - `clear_winner` only when evidence is sufficient and the effective-k primary-metric intervals do not overlap
  - `no_clear_winner` when evidence exists but the primary-metric intervals overlap
  - `insufficient_evidence` when scored child runs or resolved tasks are insufficient
- Comparison sessions must not surface a winner-derived top-level aggregate when the repeated-session comparison is `no_clear_winner` or `insufficient_evidence`.
- Update the eval-session read model and OpenAPI description to expose the richer aggregate document shape.

## Unit Tests

- Config decoding tests for:
  - valid `aggregation.reliability_weight`
  - invalid `aggregation.reliability_weight`
  - backward-compatible requests without the new field
- Repository aggregation tests for:
  - `pass@k` and `pass^k` at `k = 1, 3, 5, 10`
  - monotonic increase for `pass@k`
  - monotonic decrease for `pass^k`
  - `k = 1` equivalence with observed task success rate
  - binary verdict-driven task success
  - continuous score threshold-driven task success
  - suite-level fallback task success when challenge-level evidence is unavailable
  - inferred routing weights for side effects, autonomy, step count, and output type
  - manual override precedence over inferred routing
  - clear winner comparison
  - overlapping-interval no-clear-winner comparison
  - insufficient-evidence comparison
  - omission of comparison-session top-level winner summary when evidence is insufficient

## Integration / Functional Tests

- Repository integration test proving the persisted eval-session aggregate document includes:
  - pass metrics
  - task summaries
  - metric routing
  - comparison semantics when the session has multiple participants
- Repository integration test proving `aggregation.reliability_weight` persists through create -> read.
- API/read-model test proving eval-session reads surface the richer aggregate document without leaking synthetic top-level winner output for noisy comparison sessions.
- API/read-model test proving evidence warnings are returned when the aggregate falls back to suite-level task derivation or lacks enough evidence for a definitive repeated-session comparison.

## Smoke Tests

- `cd /home/atharva/agentclash-issue-362/backend && go test ./internal/api ./internal/repository`
- `cd /home/atharva/agentclash-issue-362/backend && go test ./...`
- `cd /home/atharva/agentclash-issue-362/backend && go vet ./...`
- `cd /home/atharva/agentclash-issue-362 && npx @redocly/cli lint docs/api-server/openapi.yaml`

## E2E Tests

- N/A — this slice is backend aggregation, API read-surface, and schema work, not a browser or CLI journey.

## Manual / cURL Tests

```bash
cd /home/atharva/agentclash-issue-362

curl -i -X POST http://localhost:8080/v1/eval-sessions \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: <user-id>" \
  -H "X-Agentclash-Workspace-Memberships: <workspace-id>:workspace_admin" \
  -d '{
    "workspace_id": "<workspace-id>",
    "challenge_pack_version_id": "<pack-version-id>",
    "challenge_input_set_id": "<input-set-id>",
    "participants": [
      {
        "agent_build_version_id": "<baseline-build-version-id>",
        "label": "Baseline"
      },
      {
        "agent_build_version_id": "<candidate-build-version-id>",
        "label": "Candidate"
      }
    ],
    "execution_mode": "comparison",
    "name": "Issue 362 repeated eval",
    "eval_session": {
      "repetitions": 5,
      "aggregation": {
        "method": "mean",
        "report_variance": true,
        "confidence_interval": 0.95,
        "reliability_weight": 0.85
      },
      "success_threshold": {
        "min_pass_rate": 0.8
      },
      "routing_task_snapshot": {
        "routing": { "mode": "comparison" },
        "task": {
          "pack_version": "v1",
          "task_properties": {
            "has_side_effects": true,
            "autonomy": "full",
            "step_count": 4,
            "output_type": "action"
          }
        }
      },
      "schema_version": 1
    }
  }'

curl -i http://localhost:8080/v1/eval-sessions/<eval-session-id> \
  -H "X-Agentclash-User-Id: <user-id>" \
  -H "X-Agentclash-Workspace-Memberships: <workspace-id>:workspace_admin"

# Expected:
# - create request accepts optional aggregation.reliability_weight
# - read response returns aggregate_result with participant pass metrics,
#   metric_routing, and comparison semantics
# - noisy comparisons do not expose a definitive top-level winner summary
```
