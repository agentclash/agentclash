# Agent Harness Execution Data Model

Agent Harnesses should evaluate coding agents without asking users to author eval packs. Internally, they still need to reuse AgentClash's mature evaluation primitives: run events, artifacts, replays, evaluation specs, deterministic validators, LLM judges, metric results, scorecards, eval sessions, comparisons, release gates, rankings, and failure review.

## Decision

Use `agent_harness_executions` as the product-facing task object and bridge each scoreable execution to one canonical `runs` / `run_agents` record when replay or scoring is enabled.

The harness execution remains the stable UI/API object for chat-style task launches, live activity, retries, cancellations, repository setup, and provider-specific execution state. The linked `run_agent` becomes the canonical evaluation subject for everything the existing scoring stack already knows how to do.

Harness runs are first-class canonical runs, not user-visible eval packs. A harness run uses `runs.source_type = 'agent_harness'` with no `eval_pack_version_id` or input set. Its single linked lane uses `run_agents.source_type = 'agent_harness'` with no agent deployment or deployment snapshot. Existing eval-pack runs keep the default `eval_pack` / `agent_deployment` shape.

## Bridge Fields

`agent_harness_executions` has nullable bridge identifiers:

- `run_id`: the canonical run containing the harness attempt.
- `run_agent_id`: the canonical run-agent lane for replay, judge results, metric results, scorecards, ranking, comparisons, release gates, and failure review.
- `evaluation_spec_id`: the evaluation spec generated from the harness evaluation configuration.

They are nullable so existing harness executions continue to read normally. New API-started executions create the linked canonical run and run-agent projection at execution creation time.

## Reuse Map

Use the existing primitives instead of adding harness-specific copies:

- Live activity: keep `agent_harness_execution_events` for product-specific setup and provider activity; mirror scoreable execution events into `run_events` once the bridge exists.
- Artifacts: store diffs, logs, summaries, screenshots, and replay blobs in `artifacts`; associate scoreable artifacts with `run_id` / `run_agent_id`.
- Replays: build harness replay views from `run_agent_replays` and `run_replays`.
- Validators and LLM judges: translate `evaluation_config` into an `evaluation_specs.definition`; persist results in `metric_results`, `judge_results`, `llm_judge_results`, and `run_agent_scorecards`.
- Pass@k and multi-turn aggregation: use `eval_sessions` / `eval_session_results` for repeated harness attempts and reliability reporting.
- Comparisons and rankings: compare linked `run_agents` through run comparisons, run rankings, and ranking insights.
- Release gates and failure review: consume linked run-agent scorecards, failure reasons, regression cases, and failure review read models.

## Migration Path

1. Keep old executions with null bridge fields visible through existing Agent Harness APIs.
2. For new scoreable executions, create or resolve the canonical run/run-agent projection before sandbox execution begins.
3. Translate harness `evaluation_config` into an `evaluation_specs` row instead of inventing harness-only validator or judge storage.
4. Persist setup and provider-specific events in `agent_harness_execution_events`; persist replay/scoring events in `run_events`.
5. Attach diff/log/artifact records to the linked `run_id` / `run_agent_id` after redaction.
6. Build scorecards with the existing scoring repositories, then expose them from Agent Harness UI by following the bridge IDs.
7. Use eval sessions for repeated attempts, pass@k, variance, and multi-turn judge aggregation.

## Legacy Run Lists

Harness canonical runs should not appear in existing eval-pack run lists. The run-list read model filters to `runs.source_type = 'eval_pack'`; harness UI follows `agent_harness_executions` and then bridge IDs for replay/scoring artifacts.

## Authorization

All harness reads must remain workspace scoped through `agent_harness_executions.workspace_id`. Bridge reads must additionally verify that linked `runs.workspace_id` and `run_agents.workspace_id` match the same workspace. The schema bridge enforces run workspace scope with `(run_id, organization_id, workspace_id)` and run-agent membership with `(run_agent_id, run_id)`.

## Backwards Compatibility

Null bridge fields mean historical executions are still listable and replayable through their existing harness event timeline. UI should treat missing bridge IDs as "scoring not available yet" rather than an execution error.
