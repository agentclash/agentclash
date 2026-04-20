# Issue 362 Research And Locked Design

Status: locked
Issue: `#362`
Parent: `#149`
Locked on: `2026-04-21`

## Goal

Implement repeated-eval `pass@k`, `pass^k`, metric routing, composite `AgentScore`, and comparison semantics that refuse to overstate noisy winners.

This document is the pre-implementation decision record. The implementation contract will be created separately under `testing/` and treated as immutable during coding.

## Research Inputs

### Open Source Codebases

1. OpenAI HumanEval repository
   - Repo: `https://github.com/openai/human-eval`
   - Raw implementation: `https://raw.githubusercontent.com/openai/human-eval/master/human_eval/evaluation.py`
   - Relevant takeaway:
     - `pass@k` is treated as a probability over repeated samples.
     - The codebase uses a dedicated estimator because it evaluates multiple generated completions per problem and aggregates over problems.

### SOTA OSS Codebases

1. EvalPlus
   - Repo: `https://github.com/evalplus/evalplus`
   - Raw implementation: `https://raw.githubusercontent.com/evalplus/evalplus/master/evalplus/evaluate.py`
   - Relevant takeaway:
     - rigorous task-level evaluation matters more than headline means
     - `pass@k` remains the primary repeated-sampling summary for code tasks
     - hidden-test rigor changes the usefulness of any reported pass metric

### Engineering Blogs

1. Anthropic, "Demystifying evals for AI agents"
   - URL: `https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents`
   - Relevant takeaway:
     - use repeated trials for nondeterministic agents
     - `pass@k` answers capability ceiling
     - `pass^k` answers reliability floor
     - the metric to emphasize depends on deployment context, especially side effects, autonomy, and whether one success is enough

### Research Papers

1. Chen et al., "Evaluating Large Language Models Trained on Code"
   - URL: `https://arxiv.org/abs/2107.03374`
   - Relevant takeaway:
     - repeated sampling changes measured capability materially
     - `pass@k` should be computed per task, then aggregated

2. Bjarnason et al., "On Randomness in Agentic Evals"
   - URL: `https://arxiv.org/abs/2602.07150`
   - Relevant takeaway:
     - small apparent improvements are often noise without repeated runs
     - agentic evals should report performance envelopes, not just a single central estimate
     - `pass@k` and `pass^k` are recommended together for nondeterministic agents

## Current Repo Constraints

1. `#361` already persists eval-session aggregate documents in `eval_session_results`.
2. Current aggregate output is run-level only:
   - mean, median, stddev, min, max, interval
   - participant aggregates by lane/label
   - top-level aggregate chosen from sole agent or per-run winner
3. Current repeated-eval storage already includes the inputs needed for `#362`:
   - `success_threshold_config`
   - `routing_task_snapshot`
   - child run membership
   - per-run scorecards
   - per-run-agent `judge_results`
   - per-run-agent execution-context challenge metadata
4. There is no existing eval-session comparison surface beyond the current winner-derived aggregate behavior.
5. Challenge-level scoring truth exists in `judge_results`, not in the run scorecard aggregate.

## Locked Decisions

### 1. Request Surface

Decision:
- Add optional `eval_session.aggregation.reliability_weight`.
- Keep `routing_task_snapshot` as the source of inferred task properties.

Why:
- Manual override is explicitly required by `#362` and by the parent issue examples.
- Making the override explicit in `aggregation` is clearer and more discoverable than hiding it inside the opaque routing snapshot.
- `routing_task_snapshot` already exists and is flexible enough to carry task metadata without adding a second new typed structure in this issue.

Locked shape:
- `eval_session.aggregation.reliability_weight`: optional float in `[0, 1]`
- inferred task properties are read from `routing_task_snapshot.task.task_properties`
- supported properties in this issue:
  - `has_side_effects` bool
  - `autonomy` string: `human`, `semi`, `full`
  - `step_count` integer
  - `output_type` string: `artifact`, `action`

### 2. Pass Metric Estimator

Decision:
- Treat repeated eval-session child runs as independent Bernoulli trials.
- Compute per-task empirical success rate `p = successes / observed_trials`.
- Compute:
  - `pass@k = 1 - (1 - p)^k`
  - `pass^k = p^k`

Why:
- HumanEval's combinatorial estimator is appropriate for its generated-completions setting.
- This repository stores actual repeated trials for the same task, so the direct Bernoulli estimator matches the data model better.
- This keeps the implementation explainable and aligned with Anthropic's agent-eval framing.

Locked `k` set:
- Always compute `{1, 3, 5, 10}`.
- Also compute `session.repetitions` if it is not already in the set.
- Routing/comparison semantics use `effective_k = session.repetitions`.

### 3. Task Unit

Decision:
- Use `challenge_identity_id` as the per-task unit.
- Resolve display metadata from execution context challenge definitions.

Why:
- Challenge identity is the stable challenge-level key already persisted alongside judge results.
- The current scoring persistence does not carry challenge-level dimension aggregates or case-key-scoped score verdicts.
- Using challenge identity avoids inventing a new persistence layer in `#362`.

Fallback:
- If no challenge-level task outcomes can be resolved for a participant, fall back to a synthetic suite task derived from the run-agent scorecard.
- Emit an evidence warning when fallback is used.

### 4. Task Success Derivation

Decision:
- Prefer challenge-level `judge_results` when available.
- Binary mode:
  - if any verdict-bearing judge says `fail`, the task fails
  - otherwise, if at least one verdict-bearing judge exists and none fail, the task passes
- Continuous mode:
  - if no verdict-bearing judges exist but normalized scores exist, use the mean normalized score for that challenge
  - compare that mean against the configured threshold
- Fallback suite mode:
  - use scorecard `passed` when present
  - otherwise use scorecard `overall_score` against the configured threshold

Threshold rule:
- configured threshold = `success_threshold.min_pass_rate` when present
- otherwise default threshold = `0.8`

Known limitation, locked:
- `success_threshold.require_all_dimensions` will only be enforced in the suite-level scorecard fallback path.
- This issue will not recompute per-challenge dimension results from raw evaluator internals.

Why:
- The repo already stores challenge-level judge outputs but not challenge-level dimension scores.
- Recomputing full per-challenge dimensions would turn `#362` into a much larger scoring-engine refactor.

### 5. Metric Routing

Decision:
- Use the routing heuristic already sketched in `#149`.

Locked weight formula:
- `+0.35` if `has_side_effects`
- `+0.25` if `step_count > 3`
- `+0.10` if `step_count > 1 && step_count <= 3`
- `+0.30` if `autonomy == "full"`
- `+0.15` if `autonomy == "semi"`
- `+0.10` if `output_type == "action"`
- clamp final weight to `1.0`

Primary metric rule:
- `pass^k` when `reliability_weight >= 0.5`
- otherwise `pass@k`

Composite:
- `AgentScore = (1 - w) * pass@k + w * pass^k`
- use the suite-level `effective_k` means for the score

Reasoning output:
- persist human-readable reasoning listing the factors that contributed to the inferred weight
- if manual override is used, state that explicitly and skip inference-based reasoning

### 6. Comparison Semantics

Decision:
- Add repeated-eval participant comparison semantics into the eval-session aggregate document.
- Do not reuse the old "winner of each child run" heuristic as the final repeated-session verdict.

Locked comparison rule:
- comparison operates on the `effective_k` suite-level metric for each participant
- compare the top two participants by `AgentScore`
- emit:
  - `clear_winner` only when both sides have sufficient evidence and their chosen primary-metric intervals do not overlap
  - `no_clear_winner` when evidence exists but intervals overlap
  - `insufficient_evidence` when there are too few scored child runs or too few resolved tasks

Why:
- This directly addresses the issue's requirement to stop declaring noisy winners as definitive.
- It keeps the semantics additive to the current aggregate document rather than introducing a separate endpoint in this issue.

### 7. Top-Level Aggregate Behavior

Decision:
- Single-agent sessions keep a top-level aggregate.
- Comparison sessions only expose winner-derived top-level `overall` and `dimensions` when the repeated-session comparison is `clear_winner`.
- Otherwise:
  - omit winner-derived top-level aggregate fields
  - keep participant aggregates
  - emit evidence warnings explaining why the top-level winner summary is absent

Why:
- The current top-level winner aggregation can overstate noisy comparison outcomes.
- Preserving participant aggregates keeps the read model useful without encoding a misleading winner.

### 8. Output Shape

Decision:
- Extend `aggregate_result` rather than adding a new endpoint.

Locked additions:
- participant-level:
  - `pass_at_k`
  - `pass_pow_k`
  - `metric_routing`
- top-level:
  - single-agent mirrors of those fields when applicable
  - repeated-session `comparison` block for multi-participant sessions

OpenAPI will be updated to describe the richer aggregate document instead of leaving it effectively opaque.

## Implementation Plan

### Step 1. Config And Aggregate Schema

- extend eval-session aggregation config validation and persistence to accept optional `reliability_weight`
- extend aggregate document types for:
  - pass metrics
  - task summaries
  - routing metadata
  - comparison summary

### Step 2. Task Outcome Extraction

- add repository-side helpers that:
  - load run agents for a child run
  - load execution context for challenge metadata
  - load challenge-level judge results
  - derive per-task success outcomes

### Step 3. Pass Metrics And Routing

- compute per-task success rate and `k` series
- compute suite-level aggregates over task-level probabilities
- compute routing weight, primary metric, and `AgentScore`

### Step 4. Comparison Semantics

- derive repeated-session comparison summary from participant routing + suite metrics
- remove winner-derived top-level comparison output when repeated evidence is insufficient

### Step 5. Read Surface And OpenAPI

- surface the richer aggregate document through the existing eval-session read endpoints
- update OpenAPI to describe:
  - `reliability_weight`
  - pass metrics
  - routing metadata
  - comparison semantics

### Step 6. Tests

- repository unit tests for:
  - Bernoulli pass metric math at `k = 1, 3, 5, 10`
  - monotonicity
  - routing inference and manual override precedence
  - clear winner vs overlap vs insufficient evidence
  - binary and continuous task-success derivation
- integration tests for persisted aggregate documents
- API/read-model tests for surfaced aggregate output and warnings

## Explicit Non-Goals For This Issue

1. No new top-level eval-session comparison endpoint.
2. No full per-challenge dimension recomputation engine.
3. No typed task-properties schema migration.
4. No Vibe Eval auto-classification work from `#172`.

## Files Likely To Change

- `backend/internal/api/eval_sessions.go`
- `backend/internal/api/eval_session_service.go`
- `backend/internal/api/eval_session_reads.go`
- `backend/internal/api/eval_session_reads_test.go`
- `backend/internal/repository/eval_session_aggregation.go`
- `backend/internal/repository/eval_session_aggregation_test.go`
- `backend/internal/repository/eval_session_aggregation_integration_test.go`
- `docs/api-server/openapi.yaml`
- `testing/codex-issue-362-passk-comparisons.md`

## Lock Rule

No implementation code should be edited against `#362` until the review-checkpoint contract is written from this locked design.
