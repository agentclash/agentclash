# Prompt Eval Failure Promotion

Prompt eval CI now gives a clean red/green gate for prompt and model changes. Promotion is the next product step: when a prompt eval fails in CI, teams need a way to preserve that failure as regression coverage without turning the prompt-eval run path into a slow, stateful authoring workflow.

This document is the design contract for that follow-up. It intentionally does not implement promotion in the prompt-eval run loop.

## Decision

Promotion should be both a UI workflow and a CLI command, with the UI as the primary human review surface and the CLI as the CI/automation escape hatch.

- UI: review failed prompt-eval assertions, inspect playground experiment output, select failures, edit titles/expected contracts, and promote to a regression suite.
- CLI: `agentclash prompt-eval promote-failures <result-file> --suite <suite-id>` for batch promotion from a saved `prompt-eval run --follow --json` result.
- CI: never auto-promote by default. CI comments should link to the failed experiments and, later, to a prefilled promotion view.

Promotion is asynchronous and user-initiated. `prompt-eval run --follow` must remain a runner/gate command; it should not create regression cases during the native loop.

## Object Mapping

| Prompt Eval Object | Existing AgentClash Object | Promotion Meaning |
| --- | --- | --- |
| Prompt eval config | Playground plus test cases | Source authoring config for the prompt/model eval. |
| `tests[].key` | Playground test case `case_key` | Stable case identity inside one prompt eval config. |
| Assertion signature | Playground evaluation spec validator key/type/metric | Determines which playground group produced the result. |
| Result row | Playground experiment result validator row | Candidate failure evidence. |
| Failed assertion row | Regression case candidate | A behavior that should be preserved and rechecked later. |
| Playground experiment | Evidence run | Source run containing actual output, model alias, provider account, and timestamps. |
| Regression suite | Regression asset container | Durable home for promoted prompt-eval failures. |
| Challenge-pack case | Future executable representation | A promoted failure can later be compiled into challenge-pack-like coverage, but prompt eval should not pretend it already is one. |

Prompt eval failures are not native challenge-pack failures. They are playground failures with prompt/test/assertion provenance. Promotion should store that provenance explicitly instead of squeezing it into challenge-pack fields that imply a different runner.

## Ownership

- CLI owns local result parsing, deterministic payload creation, dry-run output, and non-interactive promotion.
- Backend owns idempotency, dedupe, persistence, audit metadata, and future inclusion in run/eval APIs.
- Web owns review UX, failure selection, prefilled edits, and links from CI comments/playground experiments.
- Prompt-eval runner owns no promotion side effects.

## Data Model Changes

Add a prompt-eval source shape to regression cases. This can be stored in existing JSON metadata initially, then normalized later if usage grows.

Recommended fields:

```json
{
  "source_kind": "prompt_eval",
  "source_workspace_id": "ws_...",
  "source_config_hash": "sha256...",
  "source_config_path": ".agentclash/prompt-eval.yaml",
  "source_playground_id": "pg_...",
  "source_playground_experiment_id": "pexp_...",
  "source_playground_test_case_id": "ptc_...",
  "source_case_key": "refund_denial",
  "source_assertion_key": "contains_correctness_1",
  "source_assertion_type": "contains",
  "source_metric": "correctness",
  "source_model_alias_id": "ma_...",
  "source_provider_account_id": "pa_..."
}
```

Regression case payloads should preserve:

- `payload_snapshot`: prompt variables, rendered prompt when available, model alias/provider account, actual output excerpt, and experiment/result ids.
- `expected_contract`: assertion type, expected value, metric, and threshold context.
- `failure_summary`: short human-readable assertion failure.
- `failure_class`: `prompt_eval_assertion_failure` or `prompt_eval_execution_error`.
- `evidence_tier`: `playground_result`.
- `promotion_mode`: `output_only` for assertion rows; execution errors usually start as `manual` unless the expected contract is clear.

## Idempotency And Dedupe

Backend should enforce idempotency using a deterministic source fingerprint:

```text
workspace_id
regression_suite_id
source_kind=prompt_eval
source_config_hash
source_case_key
source_assertion_key
source_model_alias_id
normalized_expected_contract_hash
```

The same failed assertion in repeated CI runs should update or return the existing proposed case, not create duplicates. If the prompt/test expected value changes, the expected-contract hash changes and the promotion can create a new case because the target behavior changed.

Suggested API behavior:

- `POST /v1/workspaces/{workspaceID}/regression-suites/{suiteID}/prompt-eval-failures`
- Request contains one or more failure candidates plus `idempotency_key` per candidate.
- Response groups `created`, `existing`, `blocked`, and `errors`.
- Replays with the same idempotency key are safe.
- Backend validates that referenced playground experiment/test-case resources belong to the workspace.

## CLI Follow-Up

Proposed command:

```bash
agentclash prompt-eval promote-failures agentclash-ci-result.json \
  --suite suite_123 \
  --status proposed \
  --severity warning \
  --dry-run
```

Required behavior:

- Read the JSON result emitted by `prompt-eval run --follow --json`.
- Select failed assertion rows and execution-error rows.
- Build deterministic promotion candidates with idempotency keys.
- `--dry-run` prints candidates and dedupe fingerprints without writing.
- Without `--dry-run`, POST candidates to the backend endpoint.
- Exit `0` when all candidates are created or already exist.
- Exit nonzero when any candidate is blocked or API/auth fails.

This command should not rerun prompt evals. It only consumes saved results.

## UI Workflow

Add entry points from:

- Prompt eval CI PR comment: link to the failed playground experiment and, later, a promotion view.
- Playground experiment results page: "Promote failed assertions".
- Regression suite detail: "Import from prompt eval result".

The review screen should show:

- case key, assertion type, metric, expected value, actual output excerpt
- model alias and provider account
- prompt variables and rendered prompt where available
- proposed regression title, severity, and failure summary
- dedupe status: new, already proposed, active, blocked

Users should be able to promote selected rows, edit titles/summaries, or discard noisy failures.

## Future CI Behavior

Promoted failures should show up in future CI only after they become durable regression assets:

1. Proposed cases are visible but do not block by default.
2. Active cases can be included by CI manifests or eval start flags.
3. Prompt eval CI continues to run prompt-eval gates independently.
4. Agent CI can include promoted cases through regression suites once the backend can compile prompt-eval regression cases into executable evaluation inputs.

This avoids a bad loop where one failing prompt eval immediately mutates future gates without review.

## Open Implementation Work

- Backend endpoint and repository idempotency for prompt-eval failure promotion.
- Regression case metadata/source-kind additions.
- CLI `prompt-eval promote-failures`.
- Web promotion review screen.
- CI comment link to prefilled promotion view once the route exists.
- Docs showing the human review workflow.

## Non-Goals

- No automatic promotion from `prompt-eval run --follow`.
- No challenge-pack conversion in the runner path.
- No requirement that prompt eval failures become executable agent challenges in V1.
- No mutation of shared playground resources during promotion beyond reading source evidence.
