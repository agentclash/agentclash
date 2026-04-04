# agentclash-reasoning: Implementation Plan

> Research plan for [agentclash/agentclash#118](https://github.com/agentclash/agentclash/issues/118)
>
> This document answers the 8 research questions the issue requires before implementation begins.
> All recommendations have been validated against live documentation, academic sources, and the existing AgentClash codebase.

---

## Table of Contents

1. [Codebase Context](#1-codebase-context)
2. [Runtime Choice](#2-runtime-choice)
3. [Bridge Contract Design](#3-bridge-contract-design)
4. [Trace Normalization](#4-trace-normalization)
5. [Reasoning Strategy Design](#5-reasoning-strategy-design)
6. [Guardrail Design](#6-guardrail-design)
7. [Evaluation Harness](#7-evaluation-harness)
8. [Integration Boundaries](#8-integration-boundaries)
9. [Failure and Recovery](#9-failure-and-recovery)
10. [Phase Plan](#10-phase-plan)

---

## 1. Codebase Context

Before designing the Python service, here is what already exists in AgentClash and what the service plugs into.

### Spec Models

Defined in migration `00012_agent_spec_schema.sql`. All specs are `jsonb` columns on `agent_build_versions`, frozen into `agent_deployment_snapshots.source_agent_spec`:

- `reasoning_spec` — reasoning strategy/approach
- `guardrail_spec` — guardrail policies
- `memory_spec` — memory configuration
- `workflow_spec` — workflow definition
- `trace_contract` — tracing constraints
- `policy_spec` — instructions and tool policy
- `model_spec` — model configuration

### Execution Context

`RunAgentExecutionContext` (`backend/internal/repository/run_agent_execution_context.go`) carries:

- Run/RunAgent IDs and status
- ChallengePackVersion manifest + ChallengeInputSet items
- Deployment snapshot with all spec JSONs
- ProviderAccount credentials + ModelAlias (provider key, model ID)
- RuntimeProfile (timeouts, max iterations, tool call limits)

### Existing Hosted-Run Pattern

`backend/internal/hostedruns/contracts.go` defines the callback-based pattern the Python service will follow:

- Go worker sends `StartRequest` with task payload
- External service calls back via HTTP with events
- Temporal workflow waits on `HostedRunEventSignal` with a deadline timer
- Signal is sent from API layer via `TemporalHostedRunWorkflowSignaler.SignalRunAgentWorkflow`

### Event System

`backend/internal/runevents/envelope.go` defines 61 event types with:

- `event_id`, `schema_version`, `run_id`, `run_agent_id`, `sequence_number`
- `event_type`, `source`, `occurred_at`, `payload` (json), `summary_metadata`
- Sources: `native_engine`, `hosted_external`, `hosted_callback`, `worker_scoring`
- Evidence levels: `native_structured`, `hosted_structured`, `hosted_black_box`, `derived_summary`
- Unique constraint on `(run_agent_id, sequence_number)`

### Replay Builder

`backend/internal/repository/run_agent_replay_builder.go`:

- Reads all events via `ListRunEventsByRunAgentID`
- Groups events into steps (agent steps, model calls, tool calls, etc.)
- **Source-agnostic** — does not filter by source

### Scoring Engine

`backend/internal/scoring/engine.go`:

- `EvaluateRunAgent` operates on `[]scoring.Event`
- **Source-agnostic** — reads event types and payloads regardless of origin
- Critical events: `system.run.started`, `system.output.finalized`, `model.call.completed`, `system.run.completed`

### Routing Decision

`backend/internal/workflow/run_agent_workflow.go` line 58 branches on `DeploymentType`:

```go
if executionContext.Deployment.DeploymentType == "hosted_external" {
    return runHostedRunAgent(...)
}
// else: native executor
```

### Key Absence

- No Python code exists in the repository
- No proto/gRPC definitions — all HTTP/JSON
- No Docker service infrastructure for Python (docker-compose only has PostgreSQL)

---

## 2. Runtime Choice

### Decision: pydantic-ai is the right v0 runtime shell

**Justification:**

1. **Type safety is non-negotiable** for a reasoning/eval harness. Pydantic-ai gives validated structured outputs, typed tool parameters, and typed dependency injection out of the box.

2. **"Thin primitives" philosophy** matches the need for custom reasoning strategies. Pydantic-ai provides building blocks (agent loop, tools, structured output, retries) without forcing an opinionated orchestration model. LangGraph gives more built-in patterns but at the cost of a heavy abstraction that fights novel strategy designs.

3. **TestModel/FunctionModel** enable deterministic, zero-cost unit tests for reasoning pipelines. No other framework offers this as cleanly.

4. **OTEL-native instrumentation.** Pydantic-ai emits standard OpenTelemetry spans that can go to any backend (Jaeger, Datadog, Honeycomb, Grafana). Logfire is the recommended but not required backend. This means we get vendor-neutral tracing without needing to wrap the observability layer.

5. **The risk is manageable.** The library is pre-1.0 with potential API churn, but by wrapping the execution loop and eval orchestration behind our own interfaces, the blast radius of any breaking changes is contained to the adapter layer.

### What to use directly (stable, well-designed)

- `Agent` class definition — `Agent(model, system_prompt, result_type, tools)`
- `@agent.tool` decorator with `RunContext[DepsType]` for typed dependency injection
- Pydantic `BaseModel` result types for validated structured output
- `TestModel` and `FunctionModel` for deterministic testing
- Multi-model provider support (OpenAI, Anthropic, Gemini, etc.)

### What to wrap behind our own abstractions

- **Agent execution / the run loop** — wrap `agent.run()` and `agent.run_stream()` behind a `ReasoningEngine` abstraction to implement custom strategies (ReAct, plan-execute, reflect-once)
- **Model selection/routing** — wrap model configuration for provider switching, fallbacks, A/B testing
- **Eval orchestration** — pydantic-evals provides Dataset/Case/evaluate primitives but is early-stage; wrap it behind our own runner
- **Agent composition** — build our own orchestration for multi-agent patterns if needed

### What NOT to adopt as primary dependencies

- pydantic-ai's durable execution (experimental)
- pydantic-ai's handoff system (too early-stage)
- pydantic-evals as the sole eval framework (use for dataset management, build custom metric/trace infrastructure on top)

### Architecture

```
agentclash-reasoning/
  reasoning/
    engine.py              # ReasoningEngine abstraction (wraps agent.run)
    strategies/
      react.py             # ReAct strategy (tight loop)
      plan_execute.py      # Plan-then-execute strategy
      reflect_once.py      # Execute-then-reflect strategy
    tracing.py             # OTEL-native trace emission
  runtime/
    adapter.py             # Thin wrapper around pydantic-ai Agent
    models.py              # Model provider configuration
  guardrails/
    hooks.py               # Pre-run, pre-tool, post-tool, final-output hooks
    rules.py               # Deterministic rule implementations
  bridge/
    server.py              # HTTP server (FastAPI)
    contracts.py           # Request/response types
    events.py              # Event emission to AgentClash callback
  evals/
    runner.py              # Eval orchestrator (wraps pydantic-evals)
    multi_run.py           # Multi-run executor for pass@k
    metrics/
      final_output.py      # Exact match, fuzzy F1, schema validity
      trace.py             # Tool sequence, step count, goal decomposition
      tool_strategy.py     # Necessity, sufficiency, ordering
      cost.py              # Token accounting, cost calculation
      statistical.py       # pass@k, consistency rate, bootstrap CI
    datasets/              # YAML fixtures per strategy
    golden/                # Golden traces and answers
    price_table.yaml       # Model pricing
```

---

## 3. Bridge Contract Design

### Decision: 3 endpoints on Python + 1 callback endpoint on Go, HTTP/JSON, callback-based event delivery

### Protocol: HTTP/JSON (not gRPC)

- The existing codebase is 100% HTTP+JSON for inter-service communication
- The architecture doc (section 22) explicitly rules out gRPC-first for v1
- Event payloads use `json.RawMessage` throughout — protobuf's rigid schema would fight this

### Event delivery: Callback POST (Python pushes to Go)

The existing `waitForHostedRunTerminalEvent` pattern in `run_agent_workflow.go` already implements this:
- Workflow waits on a signal channel with a deadline timer
- External service calls back via HTTP
- API layer signals the Temporal workflow
- No long-lived connections to manage
- Worker restarts are transparent (Temporal replays signals)

Alternatives rejected:
- **gRPC streaming** — requires Go to hold open connection inside Temporal activity; complicates load balancing and worker restarts
- **SSE** — same open-connection problem; text-only encoding overhead
- **Polling** — adds latency, wastes requests during idle periods

### Endpoints

#### `POST /reasoning/runs` — StartReasoningRun

```json
{
  "run_id": "uuid",
  "run_agent_id": "uuid",
  "idempotency_key": "string",
  "strategy": "react | plan_execute | reflect_once",
  "strategy_config": {},
  "model_config": {
    "provider_key": "string",
    "provider_model_id": "string",
    "credential": "string"
  },
  "system_prompt": "string",
  "messages": [],
  "tools": [],
  "limits": {
    "max_iterations": 25,
    "max_tool_calls": 50,
    "step_timeout_seconds": 60,
    "run_timeout_seconds": 300
  },
  "callback_url": "string",
  "callback_token": "string"
}
```

Response: `{ "accepted": bool, "reasoning_run_id": "string", "error": "string?" }`

#### `POST /reasoning/runs/{id}/tool-results` — SubmitToolResult

```json
{
  "idempotency_key": "string",
  "tool_call_id": "string",
  "content": "string",
  "is_error": false
}
```

Response: `{ "accepted": bool }`

#### `POST /reasoning/runs/{id}/cancel` — CancelReasoningRun

```json
{ "reason": "string" }
```

Response: `{ "acknowledged": bool }`

#### FinalizeReasoningRun — NOT a separate endpoint

The terminal event (`reasoning.completed` or `reasoning.failed`) via callback IS finalization. Avoids two-phase commit.

### Event Envelope (Python to Go via callback POST)

```json
{
  "reasoning_run_id": "string",
  "run_id": "uuid",
  "run_agent_id": "uuid",
  "event_id": "string",
  "idempotency_key": "string",
  "sequence_number": 1,
  "event_type": "string",
  "occurred_at": "timestamp",
  "payload": {}
}
```

### Event Types (strategy-agnostic)

| Event Type | Maps to Existing | When |
|---|---|---|
| `reasoning.started` | `system.run.started` | Run begins |
| `reasoning.step.started` | `system.step.started` | Each reasoning iteration |
| `reasoning.model_call.started` | `model.call.started` | LLM call begins |
| `reasoning.model_call.completed` | `model.call.completed` | LLM call finishes (with usage) |
| `reasoning.tool_call.proposed` | `model.tool_calls.proposed` | Model proposes a tool call |
| `reasoning.tool_call.completed` | `tool.call.completed` | Tool result received |
| `reasoning.step.completed` | `system.step.completed` | Reasoning iteration ends |
| `reasoning.output.finalized` | `system.output.finalized` | Final answer produced |
| `reasoning.completed` | `system.run.completed` | Terminal success |
| `reasoning.failed` | `system.run.failed` | Terminal failure |

Strategy differences are expressed through `step_type` metadata in `step.started` payload: `"reason"`, `"plan"`, `"execute"`, `"replan"`, `"reflect"`, `"revise"`. No strategy-specific endpoints or event types needed.

### Temporal Integration

**Phase 1 — Short activity (~5s):** `StartReasoningRun` — POST to Python, get `reasoning_run_id`.

**Phase 2 — Workflow signal-wait loop:** Identical to existing `waitForHostedRunTerminalEvent`:
- When `tool_call.proposed` arrives → short activity to execute tool in sandbox → `SubmitToolResult`
- When terminal event arrives → loop exits
- Deadline timer handles Python service crashes

No long-running activities. The workflow is the durable coordinator.

### Idempotency

- All mutating requests carry `idempotency_key`
- Go side deduplicates events: `INSERT ... ON CONFLICT (run_agent_id, idempotency_key) DO NOTHING`
- Monotonic `sequence_number` per `reasoning_run_id`; Go rejects out-of-order delivery
- Event ID format: `reasoning:{run_agent_id}:{monotonic_sequence}`

### Cancellation

- Go worker POSTs to `/cancel`
- Python service stops reasoning loop at next safe point, emits `reasoning.failed` with `stop_reason: "cancelled"`
- If no terminal event within grace period (10s), Go worker treats as timeout (mirrors existing `markHostedRunTimedOut`)

---

## 4. Trace Normalization

### Decision: Flat event sequence with optional nesting metadata; OTEL spans emitted in parallel for ops

### Keep the flat event sequence

- Replay UI steps through events in order — flat sequence is simplest
- Scoring reads are range scans by type — flat is cheapest
- Span trees create fan-out complexity and can orphan children on partial failure

### Optional nesting fields on SummaryMetadata

```
parent_event_id     string   -- back-pointer for nesting reconstruction
scope_depth         int      -- 0=top-level, 1=within step, 2=within plan step
reasoning_strategy  string   -- "react" | "plan_execute" | "reflect_once"
```

### New Event Types

```
# Planning
reasoning.plan.created
reasoning.plan.step.started
reasoning.plan.step.completed
reasoning.plan.revised

# Reasoning steps
reasoning.step.started
reasoning.step.completed

# Tool selection
reasoning.tool.considered
reasoning.tool.selected
reasoning.tool.rejected

# Reflection
reasoning.reflection.started
reasoning.reflection.completed

# Guardrails
reasoning.guardrail.evaluated
reasoning.guardrail.passed
reasoning.guardrail.blocked
reasoning.guardrail.tripped

# Answer lifecycle
reasoning.answer.proposed
reasoning.answer.accepted
```

### New Source Value

```
reasoning_engine
```

Distinct from `native_engine`. Coexists alongside it. Replay builder and scoring engine are already source-agnostic — no changes needed.

### Streaming: Buffer-Then-Flush

- Do NOT persist per-token `model.output.delta` events to durable store
- Emit deltas ephemerally for live UI via Redis pub/sub
- Persist a single `model.call.completed` event with the full assembled response
- Include `time_to_first_token_ms` and `total_stream_duration_ms` in payload

### OTEL Parallel Path

The Python runtime emits OTEL spans for infrastructure observability (Grafana). The canonical envelope path feeds replay/scoring/comparison. Both emitted in parallel — this is our architectural decision, not an OTEL-prescribed pattern.

### Usage and Cost Metadata

**Per model call** (in `model.call.completed` payload):

```json
{
  "usage": {
    "input_tokens": 1200,
    "output_tokens": 350,
    "total_tokens": 1550,
    "details": { "cached_tokens": 800, "reasoning_tokens": 120 }
  },
  "latency_ms": 4500,
  "streaming": {
    "streamed": true,
    "time_to_first_token_ms": 230,
    "total_stream_duration_ms": 4500
  },
  "estimated_cost_usd": 0.0042
}
```

**Per run** (in `system.run.completed` payload):

```json
{
  "total_input_tokens": 5400,
  "total_output_tokens": 1200,
  "total_model_calls": 4,
  "total_tool_calls": 3,
  "total_steps": 4,
  "total_latency_ms": 18000,
  "total_estimated_cost_usd": 0.018
}
```

---

## 5. Reasoning Strategy Design

### Three strategies, deliberately scoped

The `reasoning_spec` defines the strategy. Each strategy maps to a distinct control flow. The strategies are named for this system — `reflect_once` in particular is a custom name for a constrained Evaluator-Optimizer pattern (not standard academic terminology).

### `react` — Reason + Act

A single-loop strategy interleaving reasoning and action (based on Yao et al., 2022/ICLR 2023).

```
REASON about current state
  → Done? YES → ANSWER → END
  → Done? NO  → ACT (tool call) → OBSERVE (tool result) → loop
```

- No explicit planning or reflection
- Self-correction is implicit (next Thought reasons about tool errors)
- Termination: final answer, `max_reasoning_steps`, or `max_iterations`

| Spec Field | Value |
|---|---|
| `planner.enabled` | `false` |
| `reflection.enabled` | `false` |
| `termination.max_iterations` | = `max_reasoning_steps` |

### `plan_execute` — Plan Then Execute

A two-phase strategy separating planning from execution (based on Wang et al., 2023 Plan-and-Solve).

```
PLAN (generate ordered steps)
  → EXECUTE step (may involve multiple tool calls)
  → EVALUATE step
    → success → next step
    → failure → RE-PLAN (revise remaining steps) → continue
  → All steps done → ANSWER
```

- Planning is mandatory, happens once at start
- Re-planning happens only on step failure or unexpected results
- EVALUATE phase is lightweight reflection (structurally required)
- Cap re-plans at `floor(max_reasoning_steps / 3)` or `termination.max_iterations`

| Spec Field | Value |
|---|---|
| `planner.enabled` | `true` (required) |
| `reflection.enabled` | controls deeper reflection at step boundaries |
| `reflection.on_tool_error_only` | meaningful here — narrows reflection to error cases |
| `termination.max_iterations` | 1-3 (full plan-execute cycles) |

### `reflect_once` — Act Then Reflect

A constrained Evaluator-Optimizer pattern bounded to one reflection cycle. This is a custom strategy name for this system — the academic Reflexion pattern (Shinn et al., 2023) is multi-trial; ours is deliberately single-pass.

```
EXECUTE (ReAct loop → draft answer)
  → REFLECT (structured self-review: correct? complete? quality?)
    → PASS → return draft
    → FAIL → REVISE (one more execution pass) → return regardless
```

- Reflection is exactly one structured, prompted LLM call — not implicit reasoning
- The REFLECT phase does NOT make tool calls — pure reasoning over observations
- Budget allocation: ~70% EXECUTE, ~30% REVISE

| Spec Field | Value |
|---|---|
| `planner.enabled` | `false` |
| `reflection.enabled` | `true` (required) |
| `reflection.on_tool_error_only` | `false` (reflection always triggers) |
| `termination.max_iterations` | 2 (execute + revise) |

### `max_reasoning_steps` is a global budget

All phases compete for the same pool. Reflection cannot be unbounded because it competes with execution for the step budget. Reserve at least 2 steps for final answer production.

### Bounding Reflection

- **Semantic similarity check:** If consecutive reflections identify the same issues (>80% overlap), stop
- **Score monotonicity:** If confidence doesn't improve after revision, stop
- **Budget reserve:** Always keep >=2 steps for final answer

### Strategy Degradation

Always toward simplicity: `plan_execute` → `react` → direct answer.

If planning produces an unparseable plan after 1 retry, fall back to `react` for the remainder of the budget. Log a `strategy_degradation` trace event. If the reflection step itself fails, skip revision and return the draft answer.

### `reasoning_spec` v0 Schema

```yaml
strategy: react | plan_execute | reflect_once
max_reasoning_steps: 25
planner:
  enabled: false
reflection:
  enabled: false
  on_tool_error_only: false
tool_strategy:
  prefer_structured_tools: true
  allow_shell_fallback: false
termination:
  max_iterations: 10
trace:
  emit_intermediate_reasoning_summary: true
```

`tool_strategy` is orthogonal to reasoning strategy — any combination works without special-case logic.

---

## 6. Guardrail Design

### Decision: Deterministic rules only in v0; design hook interface to accept async/model-based checks for v1

### Hook Points

#### Pre-Run Input Validation

Deterministic checks:
- Token/character length limits
- Blocked content patterns (regex blocklist for known-bad patterns)
- Schema validation (Pydantic) for structured inputs
- Character encoding validation (reject homoglyphs, invisible characters, RTL overrides)

**Violation behavior: Block.** Return structured error immediately. Retry makes no sense (input is from the user). Redaction risks passing partially-malicious input.

#### Pre-Tool Validation

Deterministic checks:
- **Tool allowlist** — only permit explicitly registered tools (single most important guardrail)
- Argument schema validation (types, ranges, required fields, string length limits)
- Dangerous argument patterns — regex for shell metacharacters (`;`, `|`, `&&`), path traversal (`../`), SQL injection
- Call frequency limits — per-tool and total maximums
- Resource limits — check argument quantities within bounds

**Violation behavior: Block** for allowlist/schema violations. **Fail** (abort entire run) for security violations. Do not retry — if the model generates malicious tool calls, re-prompting may produce subtler attacks.

#### Post-Tool Validation

Deterministic checks:
- Output size limits (truncate or reject)
- PII/secret pattern redaction — regex for API keys (`sk-...`, `AKIA...`), private keys, SSNs, credit card numbers (Luhn-validated)
- Stack trace stripping from error responses
- Content type validation (JSON parse check, HTML script tag stripping)

**Violation behavior: Redact** for secrets (replace with `[REDACTED]`). **Block** for size limits. The tool has already executed; the question is what the model sees.

#### Final-Output Validation

Deterministic checks:
- PII/secret scanning (same patterns as post-tool)
- Output format validation (JSON schema if structured output expected)
- Blocklist matching (system prompt fragments, guardrail config details, raw tool descriptions)
- Length limits

**Violation behavior: Redact** for PII. **Retry** (max 2) for format validation failures. **Block** for blocklist matches.

### `guardrail_spec` Structure

```yaml
pre_run:
  max_input_tokens: 4096
  blocked_patterns: ["ignore all previous", "you are now"]
  unicode_safety: true
  on_violation: block

pre_tool:
  allowed_tools: ["search", "calculator", "read_file"]
  max_calls_per_tool: { search: 10, read_file: 5 }
  max_total_calls: 25
  dangerous_patterns:
    read_file:
      path: { deny: ["\\.\\.", "/etc/", "/proc/"] }
  on_violation: block
  on_security_violation: fail

post_tool:
  max_output_size: 16384
  redact_patterns: ["sk-[a-zA-Z0-9]+", "AKIA[A-Z0-9]+", "-----BEGIN.*KEY"]
  strip_stack_traces: true
  on_violation: redact

final_output:
  redact_patterns: ["...same as post_tool..."]
  blocked_content: ["SYSTEM PROMPT:", "guardrail_spec"]
  validate_schema: null
  on_format_error: retry
  max_retries: 2
  on_violation: block
```

### Tripwire Behaviors

| Behavior | Semantics | When |
|---|---|---|
| **Block** | Stop current operation, return error, system remains available | Input validation, tool allowlist, size limits |
| **Retry** | Re-attempt with modification/feedback, up to configured limit | Output format errors only |
| **Fail** | Abort entire run, mark as failed in trace | Security violations, repeated retry exhaustion |

Retry has max count of 2. After exhaustion, escalate to Fail. Fail writes full context to trace but returns only category to caller (not details that would help an attacker).

### Safety: v0 Mitigations

- **Prompt injection:** Tool descriptions from trusted static registry only. Never interpolate user input.
- **Shell escalation:** No shell tool in v0. Expose narrow tools (`list_directory`, `read_file` with path constraints).
- **Instruction leakage:** Final-output blocklist for system prompt fragments.
- **Indirect injection via tool outputs:** Truncate, strip instruction-like patterns. Full model-based detection is highest-priority v1 item.

### Performance

- Pre-compile regex at initialization, not per-call
- Short-circuit on first violation within a hook
- Run input guardrails concurrently with first model call (zero added latency on happy path)
- Every guardrail check emits a trace event with timing

### Deferred to v1

- Semantic intent classification on inputs (model-based)
- Indirect prompt injection detection on tool outputs (model-based)
- Toxicity/harm classification on outputs
- Multi-step attack sequence detection
- Branch-aware guardrail scoping

---

## 7. Evaluation Harness

### Decision: pydantic-evals for dataset management + custom trace/multi-run/metric layers

### Metric Categories

#### A. Final-Output Metrics (v0)

| Metric | Calculation | Golden Data |
|---|---|---|
| Exact Match | `output == golden` (after normalization) | Golden answer per task |
| Fuzzy F1 | Token-level F1 between output and golden | Golden answer per task |
| Structured Output Validity | Pydantic model validation | Schema per task type |

#### B. Trace-Grading Metrics (v0)

| Metric | Calculation | Golden Data |
|---|---|---|
| Tool Call Sequence | Subsequence match (not exact) against golden tool sequence | Golden tool sequence per task |
| Reasoning Step Count | Check actual count falls within expected range | Min/max step bounds |
| Goal Decomposition Quality | Jaccard similarity of generated plan sub-goals vs golden | Golden sub-goal list (plan_execute only) |

#### C. Repeated-Run Metrics (v0)

| Metric | Calculation | Golden Data |
|---|---|---|
| pass@k | Unbiased estimator: `1 - C(n-c,k) / C(n,k)` (Chen et al., 2021) | Correctness function |
| Consistency Rate | Fraction of runs producing same correct answer | Correctness function |
| Strategy Variance | Std dev of step counts, tool calls, latency | None (computed from traces) |

#### Deferred to v1

- Hallucination Detection (requires LLM-as-judge)
- Trace Coherence (requires LLM-as-judge)
- Error Recovery Rate (requires error injection infrastructure)

### pass@k Methodology

Formula: `pass@k = 1 - C(n-c,k) / C(n,k)` (unbiased estimator from Chen et al., 2021)

- **pass@1:** n=20 samples per task (adequate)
- **pass@5:** n>=40 samples per task (n=20 is marginal — produces wide confidence intervals)
- Use production temperature (0.7), not temperature=0
- Regression detection: flag when new mean falls below previous 95% CI lower bound
- Bootstrap: **2000+ iterations** (not 1000) for stable percentile-based 95% CI bounds; prefer BCa over simple percentile method

### Tool-Strategy Scoring

Composite: `0.4 * necessity + 0.4 * sufficiency + 0.2 * ordering`

- **Necessity:** `1 - (unnecessary_calls / total_calls)` vs golden tool set
- **Sufficiency:** `called_required / total_required`
- **Ordering:** LCS of actual vs golden tool sequence / len(golden)

### Cost Accounting

- Track per-LLM-call: input/output/cached tokens, wall-clock latency, TTFT
- Static price table keyed by `(provider, model_id)` — no dynamic fetching
- Report **median** cost (not mean) for regression stability
- Regression: bootstrap 95% CI on median, flag when new median exceeds upper bound

### Non-Determinism Handling

- **Property-based assertions** — assert properties ("contains Y", "valid JSON matching schema Z") not exact strings
- **Functional correctness** — execute generated code against test cases when applicable
- **Snapshot tool call names + key args** — NOT reasoning text
- **Subsequence matching** — not exact sequence
- **Statistical assertions** — binomial test at alpha=0.05 for repeated-run metrics

Note: Property-based assertions are one valid approach among several (LLM-as-judge, functional correctness, semantic similarity). Trace-level grading is an essential complement to final-output checking, not optional.

### Eval Fixture Design Per Strategy

**react:**
```yaml
task_id: react-001
input: "What is the population of the capital of France?"
expected_output: "2.1 million"
golden_tool_sequence: [search, extract]
expected_step_range: [2, 5]
strategy: react
tags: [multi-hop, factual]
```

**plan_execute:**
```yaml
task_id: planexec-001
input: "Build a summary of Q3 earnings"
expected_output: { structured summary }
golden_plan: [fetch report, extract metrics, compute change, format]
golden_tool_sequence: [fetch_document, extract_table, calculate, format]
expected_step_range: [4, 8]
strategy: plan_execute
```

**reflect_once:**
```yaml
task_id: reflect-001
input: "Solve this logic puzzle..."
expected_output: "Answer: B"
golden_reflection_issues: [failed to account for constraint 3]
expected_step_range: [2, 3]
strategy: reflect_once
tags: [self-correction, logic]
```

### Architecture

```
eval/
  conftest.py              # pytest fixtures, trace capture setup
  metrics/
    final_output.py        # exact_match, fuzzy_f1, schema_validity
    trace.py               # tool_sequence_score, step_count_check
    tool_strategy.py       # necessity, sufficiency, ordering
    cost.py                # cost_per_task, latency_stats
    statistical.py         # pass_at_k, consistency_rate, bootstrap_ci
  datasets/
    react/                 # YAML fixtures
    plan_execute/           # YAML fixtures
    reflect_once/           # YAML fixtures
  runners/
    single_run.py          # wraps pydantic-evals evaluate()
    multi_run.py           # n-run executor for pass@k
  golden/
    traces/                # golden trace snapshots
    answers/               # golden answers
  price_table.yaml         # model pricing
```

---

## 8. Integration Boundaries

### Routing Decision

Route based on `agent_kind` + feature flag, not a new `deployment_type`.

The `AgentKind` field already exists on `AgentBuildVersionExecutionContext`. Add a value `"reasoning_v1"`. The routing check becomes:

```go
if executionContext.Deployment.DeploymentType == "hosted_external" {
    return runHostedRunAgent(...)
}
if isReasoningServiceCompatible(executionContext, flags) {
    return runReasoningServiceAgent(...)
}
// default: native executor
```

### Feature Flags (3-tier)

1. **Global kill switch:** `REASONING_SERVICE_ENABLED=bool`. If false, all builds use native executor.
2. **Workspace allowlist:** `reasoning_service_opt_in` on workspace. Only opted-in workspaces route to Python.
3. **Percentage rollout:** `hash(flag_name + run_agent_id) % 100 < rollout_percentage`. Hash input includes BOTH the flag name and entity ID (not just entity ID) to decorrelate flag assignments. Deterministic per run-agent so retries always hit the same executor.

Store routing decision as `execution_lane` column on `run_agents`: `"native"`, `"reasoning_v1"`, `"hosted_external"`.

### Data Flow

**Go to Python:** The full `RunAgentExecutionContext` serialized as JSON (same pattern as `buildHostedTaskPayload`):
- Run/RunAgent IDs
- ChallengePackVersion manifest + ChallengeInputSet items
- All spec JSONs (reasoning_spec, guardrail_spec, policy_spec, model_spec)
- Provider credentials (scoped token)
- RuntimeProfile (timeouts, limits)

**Python to Go:** Events conforming to `runevents.Envelope` schema. Critical events for scoring:
- `system.run.started` — latency metrics
- `model.call.completed` — token usage
- `system.output.finalized` — correctness validators
- `system.run.completed` — reliability dimension, usage totals

### No Changes Needed to Replay or Scoring

Both `BuildRunAgentReplay` and `EvaluateRunAgent` are source-agnostic. As long as the Python service emits conformant events, they work identically.

---

## 9. Failure and Recovery

### Python Service Crash Mid-Run

- Model as Temporal activity with `HeartbeatTimeout: 60s` — detects stuck activities
- Set `MaximumAttempts: 2` explicitly (Temporal defaults to 0 = unlimited; must always set this)
- Worker crashes are caught separately by task dispatch timeouts — both mechanisms needed together
- On crash: Temporal deadline timer fires → workflow marks run-agent as `failed` via existing `markRunAgentFailed` path

### Partial Event Delivery

- Accept partial event streams. Replay builder already handles incomplete streams (open steps get `incompleteReplayHeadline`). Scoring returns `EvaluationStatusPartial`.
- Go workflow emits an application-level failure event on activity failure to give replay a clean terminal state. Note: this is a custom pattern, not a Temporal primitive — Temporal's native answer to partial completion is saga/compensation.
- Python service should maintain a local WAL (append-only file or SQLite) for event durability.

### Idempotency Model

- Event ID format: `reasoning:{run_agent_id}:{monotonic_sequence}`
- Database: `INSERT ... ON CONFLICT (run_agent_id, idempotency_key) DO NOTHING`
- Note: `ON CONFLICT DO NOTHING` still consumes sequence values — gaps in auto-increment are expected
- On retry: check for existing terminal events before re-executing (check-before-act pattern)
- Combine check-before-act with DB constraint to cover TOCTOU race window

### Runs are NOT Resumable in v0

Restart from scratch on failure (idempotent start). Acceptable because runs are bounded by `max_iterations` and `run_timeout_seconds`. Checkpoint/resume for long-running runs is a v1 item.

---

## 10. Phase Plan

### Phase 1: Contracts (Week 1-2)

- Define `reasoning_spec` v0 JSON Schema with Pydantic models
- Define `guardrail_spec` v0 JSON Schema
- Define HTTP bridge request/response types
- Define event envelope types mapping to AgentClash canonical events
- Write shared JSON fixture tests for Go <-> Python payloads
- Write spec validation tests (valid/invalid fixtures, backwards-compatible defaulting)

### Phase 2: Runtime (Week 3-4)

- Implement pydantic-ai adapter layer (Agent wrapper, model configuration)
- Implement `ReasoningEngine` abstraction
- Implement `react` strategy with tool calling loop
- Implement `plan_execute` strategy with planning, execution, re-planning
- Implement `reflect_once` strategy with structured reflection
- Implement strategy degradation (`plan_execute` -> `react` on planning failure)
- Implement guardrail hooks (pre-run, pre-tool, post-tool, final-output)
- Write unit tests for strategy selection, termination, retry, guardrail handling

### Phase 3: Trace Normalization (Week 5)

- Implement trace emission (canonical event envelopes)
- Implement OTEL span emission (parallel path)
- Implement buffer-then-flush for streaming
- Write golden trace snapshots for each strategy
- Write trace -> AgentClash event mapping tests
- Write event ordering and idempotency tests

### Phase 4: Eval Harness (Week 6-7)

- Implement eval runner (wrapping pydantic-evals)
- Implement multi-run executor for pass@k
- Implement metric library (final-output, trace, tool-strategy, cost, statistical)
- Build eval fixtures for all three strategies
- Write eval metric calculation tests
- Write regression baseline infrastructure

### Phase 5: AgentClash Integration (Week 8)

- Add `reasoning_engine` source value to Go event system
- Add `execution_lane` column to `run_agents`
- Implement routing logic in `RunAgentWorkflow`
- Implement 3-tier feature flag evaluation
- Implement callback endpoint for receiving Python events
- Implement Temporal signal-wait loop for reasoning runs
- Write contract tests in both Go and Python
- Write integration tests for the full flow

### Success Criteria

- A build with `reasoning_spec.strategy: react` executes through the Python service from an AgentClash run
- The service emits normalized reasoning events visible in replay
- `react`, `plan_execute`, and `reflect_once` are covered by fixtures and golden traces
- Guardrail hooks can deterministically block or fail a run
- The eval harness reports pass@k, latency, token usage, and cost-per-task
- Contract tests pass in both Python and Go
- Clear follow-ups exist for MCP, A2A, memory, and multi-agent work
