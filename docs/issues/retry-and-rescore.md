# Retry failed runs and preview re-scores without mutating canonical results

## Problem

Today, if a run fails for any reason — provider 503, E2B lease expired, worker crash, judge LLM throttled — the only way forward is to manually create a brand-new run, re-picking the challenge pack version, agent roster, judge models, budgets, and spend policy. There is no "try again" button, no retry endpoint, no way to distinguish "the model couldn't do this task" from "our infrastructure dropped the request."

This conflates two very different concepts:

- A **run** as the user thinks of it: intent. "Pit these agents against this challenge with these limits."
- A **run** as the platform models it: a single physical execution. If it dies, the intent dies with it.

The immutable model is correct for reproducibility (a scorecard published as *the* result of attempt 1 should never silently mutate into the result of attempt 3). But forcing the user to reconstruct the entire configuration to work around someone else's outage is not an immutability trade-off — it's a missing feature.

Orthogonally: during pack authoring, users want to **re-score** an existing run against a tweaked evaluation spec without re-executing the agent. The scoring engine already supports this (`scoring.EvaluateRunAgent` is pure over persisted events), but `run_agent_evaluation.go:26` hard-asserts that the spec's pinned `challenge_pack_version_id` matches the run's, refusing to score under any newer version. That assertion is right for the *canonical* scorecard and wrong for a *preview*.

## Proposed shape

### Object model split

Separate the run's **specification** from its **attempts**:

- `runs` (existing) — stores the intent + canonical result. Add `attempt_count int` and `current_attempt_id uuid`.
- `run_attempts` (new) — one row per physical execution. Carries `run_id`, `attempt_number`, `status`, `stop_reason`, `failure_category` (see below), `started_at`, `completed_at`, `temporal_run_id`.
- `run_events.run_attempt_id` — every event belongs to exactly one attempt. The replay viewer scopes to the current attempt by default and offers an "attempt switcher" when prior attempts exist.
- `run_agent_scorecards.run_attempt_id` — scorecards likewise pin to a specific attempt.

This makes retries cheap: one insert into `run_attempts`, one Temporal workflow signal against the same `run_id`. The prior attempt remains visible in the UI, labelled as superseded, with its events preserved for forensics.

### API

1. **`POST /v1/runs/{id}/retry`** — for runs whose current attempt failed with a retryable category (see taxonomy below). Spawns a new attempt against the same intent. Rejects when the run completed or when the current attempt failed for a non-retryable reason.

2. **`POST /v1/runs/{id}/rescore`** with `{ "evaluation_spec_id": "<uuid>" }` — pure function over persisted events and captured files, produces a scorecard **labelled "preview"** that lives in its own column family and never replaces the canonical scorecard. Safe to call repeatedly during pack iteration.

## The hard part: failure classification

The whole retry story only works if we can answer "is this failure the model's fault, or ours?" The user-facing rule must be:

> **Failures that carry signal about the model stay. Failures that carry signal only about our infrastructure are retryable.**

When in doubt, default to "signal" — it is far worse to silently memory-hole a real model failure than to make a user manually confirm a retry of a genuine infra failure.

We already produce enough typed information to classify almost every failure at the source. What's missing is the rollup.

### Existing signals

- **`provider.FailureCode`** (in `backend/internal/provider/provider.go:86`):
  `auth`, `rate_limit`, `invalid_request`, `timeout`, `unavailable`, `malformed_response`, `credential_unavailable`, `unsupported_provider`, `unsupported_capability`.

- **`engine.StopReason`** (in `backend/internal/engine/native_executor.go:27`):
  `completed`, `timeout`, `step_limit`, `tool_limit`, `provider_error`, `sandbox_error`, `observer_error`.

- **`system.run.failed` event payload** (in `backend/internal/worker/native_event_observer.go:280`):
  `{ error, stop_reason, provider_failure: { provider_key, code, retryable, message } }`.

### Proposed `failure_category` rollup

Each attempt gets one of:

| Category | Meaning | Retryable? | Counts as result? |
|---|---|---|---|
| `completed` | Agent produced a final output, scoring ran. | N/A | Yes |
| `model_behavior` | The agent failed the task (ran out of steps/tokens/time, refused, emitted unrecoverable malformed tool calls, crashed the sandbox with its own code). | **No, auto-retry is wrong here.** User may manually retry with an explicit override. | **Yes.** Treated as a terminal result; keeps the failed scorecard. |
| `provider_infra` | Provider returned `rate_limit`, `timeout`, `unavailable`, or the HTTP client failed to reach the provider. | **Yes**, with exponential backoff and a cap (e.g. 3 auto-retries per attempt tree). | No |
| `provider_config` | Provider returned `auth`, `credential_unavailable`, `unsupported_provider`, `unsupported_capability`. | **Yes after user fixes credentials/config** — the retry endpoint is enabled but the failure banner explains why it won't succeed until the key is rotated / the deployment is edited. | No |
| `sandbox_infra` | E2B failed to create/connect/lease the sandbox; node unhealthy; disk exhausted before the agent ran a command. | **Yes.** | No |
| `judge_infra` | Agent completed, but the LLM judge LLM call failed (provider outage on the *judge* side). | **Partial retry via `/rescore`** — agent work is preserved, only the judge is re-invoked. | Canonical scorecard remains in `evaluating`/`partial` until the judge succeeds. |
| `platform_bug` | Scoring engine crashed, Temporal activity panicked, event emission returned observer_error. | **Yes via `/rescore`** when agent events are intact, via `/retry` when they aren't. | No, but we page on this category. |
| `unclassified` | Default when none of the above map cleanly. | **No auto-retry.** User override required. | Treated as signal (conservative default). |

### Classification rules (at the worker, before persisting `run_attempts.failure_category`)

The order of checks matters — first match wins:

1. **Agent ran and produced output** → `completed`.
2. **`StopReason in { step_limit, tool_limit, timeout }`** → `model_behavior`. These are budget exhaustions, which are legitimate signals about the model's efficiency.
3. **`StopReason == sandbox_error`** — needs further discrimination:
   - If the failure occurred **before** the first `tool.call.started` event (i.e. sandbox wasn't usable to begin with) → `sandbox_infra`.
   - If it occurred after the agent began issuing commands → `model_behavior` (agent misused the sandbox).
4. **`StopReason == provider_error`** with `provider_failure.code`:
   - `rate_limit`, `timeout`, `unavailable`, `malformed_response` → `provider_infra`.
   - `auth`, `credential_unavailable` → `provider_config`.
   - `invalid_request` is the **ambiguous one**: a well-formed prompt that the provider rejected because the model refused → `model_behavior`. A malformed request our adapter generated → `platform_bug`. We cannot always tell from the outside. **Default to `model_behavior`** (conservative: don't memory-hole a refusal).
5. **`StopReason == observer_error`** → `platform_bug`.
6. **Temporal activity error outside the engine** (e.g. DB write failed, Redis unreachable) → `platform_bug`.
7. **Judge LLM invocation failed after agent completed** → `judge_infra`.
8. **None of the above** → `unclassified`.

### UI implications

- Run detail page shows the current attempt's scorecard; a small "N prior attempts" affordance reveals the history with category badges.
- Retry button is disabled for `completed` and `model_behavior` (with tooltip explaining why), enabled otherwise.
- `model_behavior` failures render the failed scorecard prominently — this is a result, not an error.
- `provider_config` failures link directly to the deployment editor so the user can rotate credentials.
- `judge_infra` failures show a "retry judges only" action instead of a full retry.

### Open questions

1. **Auto-retry defaults.** Should `provider_infra` and `sandbox_infra` auto-retry once without user intervention, or always require a click? My lean: auto-retry once with backoff, then require a click. Bounded automation + user control.
2. **Attempt numbering on `model_behavior` retries.** If a user explicitly overrides and retries a `model_behavior` failure, do we display it alongside the original as "attempt 2, manual override"? Probably yes — transparency beats hiding.
3. **Spend policy.** A retry still draws from the workspace budget. Do we deduct a second time, or does the spend policy track "run budget" rather than "per-attempt"? My lean: per-run budget with a hard cap; retries eat into it.
4. **Historical runs.** Backfilling `run_attempts` for existing runs (1 attempt each, derived from their final state). Straightforward migration.
5. **Public immutability contract.** If a run is published to an arena / released via gate, later retries should be forbidden — the published result is the canonical one. Need a check on the retry endpoint against publication state.

## Scope for a first PR

Just the plumbing for `provider_infra`, `sandbox_infra`, and `completed` / `model_behavior`. That covers ~90% of the real pain (provider 5xx and E2B flakes) without touching judge retries or rescoring. Everything else layers on top.

## Related

- PR #310 (scorecard source pointer) — depends on stable replay steps; the attempt model would scope replay to the current attempt.
- PR #312 (source fix + json-schema integer fix) — surfaced this when running the fibonacci-e2e-showcase pack against live models.
