# ADR: Pre-Deploy Eval SDK — Repository and Package Strategy

**Status:** Accepted  
**Date:** 2026-06-24  
**Parent epic:** [#1104](https://github.com/agentclash/agentclash/issues/1104)  
**Related issues:** #1105 (this ADR), #1106–#1114 (implementation)

## Context

AgentClash needs a low-friction pre-deploy evaluation layer for agents and LLM apps that works like normal tests before release. The wedge must:

- run locally with no AgentClash account required;
- run in CI/CD with stable exit codes, JSON, and JUnit output;
- be driven from a CLI;
- be authored by coding agents;
- promote failures into eval packs, regression suites, and hosted scorecards.

DeepEval-style adoption proves demand for Python-first, pytest-style eval libraries. AgentClash's opportunity is a smaller, safer, local-first SDK with a direct path into sandboxed agent evaluation, replay, and scorecards — not a DeepEval clone.

## Decision

**Start inside this monorepo.** Do not create a separate SDK repository for v0.

Package boundaries, release cadence, and API surface will stabilize here first. A future split into a dedicated SDK repo remains possible once:

1. the public API has been stable for at least one minor release cycle;
2. contract-sync automation between repos is proven in CI; and
3. release ownership (PyPI, npm, GoReleaser) is clearly separated.

### Rationale for monorepo-first

| Factor | Monorepo now | Separate repo now |
|--------|--------------|-------------------|
| Eval-pack promotion | Same schemas, validators, and examples | Cross-repo contract drift risk |
| CLI glue (`agentclash evaltest`) | Already in `cli/` | Duplicate release pipeline |
| Docs and examples | Co-located with eval packs | Fragmented onboarding |
| Early API churn | Cheap to refactor across packages | Expensive sync overhead |
| CI contract validation | Single workflow | Requires dedicated sync job from day one |

**Blocker check:** No blocker was found that requires a separate repo for v0. Tight coupling to eval-pack YAML, regression suite shapes, and CLI exit-code registry favors monorepo-first.

### Future split trigger

Split when **all** of the following hold:

- Python and TypeScript SDKs emit a versioned report schema that has not changed for ≥ 4 weeks.
- PyPI/npm publish cadence diverges from AgentClash backend/CLI releases.
- External contributors need SDK-only access without backend credentials.

Proposed future repo name: `agentclash-evals` (GitHub org: `agentclash/agentclash-evals`).

---

## Package layout (v0)

```
agentclash/                          # this monorepo
├── sdk/
│   ├── python/
│   │   └── agentclash_eval/         # PyPI package: agentclash-evals
│   │       ├── pyproject.toml
│   │       ├── src/agentclash_eval/
│   │       └── tests/
│   └── typescript/
│       └── evals/                   # npm package: @agentclash/evals (Phase 2)
│           ├── package.json
│           └── src/
├── schemas/
│   └── evaltest/                    # language-neutral JSON schemas
│       ├── eval-report.schema.json
│       ├── agent-result.schema.json
│       └── fixtures/                # golden report examples
├── cli/
│   └── cmd/
│       └── evaltest.go              # agentclash evaltest ...
├── examples/
│   └── evaltest/                    # local eval examples
│       ├── python/
│       └── typescript/              # added in Phase 2
└── docs/
    └── evaltest/                    # user-facing eval SDK docs
```

### Python package

- **PyPI name:** `agentclash-evals`
- **Import path:** `agentclash_eval`
- **Location:** `sdk/python/agentclash_eval/`
- **Optional extras:**
  - `agentclash-evals[pytest]` — opt-in pytest plugin
  - `agentclash-evals[openai]` — OpenAI/Anthropic result adapters
  - `agentclash-evals[langchain]` — LangChain/LangGraph adapters
  - `agentclash-evals[otel]` — OpenTelemetry trace ingestion (later)

Core package has **zero** hard dependencies on OpenAI, LangChain, pytest, or hosted AgentClash.

### TypeScript package (Phase 2)

- **npm name:** `@agentclash/evals`
- **Location:** `sdk/typescript/evals/`
- **Subpath exports:**
  - `@agentclash/evals` — core
  - `@agentclash/evals/vitest` — Vitest helpers
  - `@agentclash/evals/jest` — Jest helpers
  - `@agentclash/evals/vercel-ai` — Vercel AI SDK adapter
  - `@agentclash/evals/langchain` — LangChain JS adapter

### Shared schemas

- **Location:** `schemas/evaltest/`
- **Authority:** JSON Schema (draft 2020-12), same convention as `docs/schemas/prompt-eval-result.schema.json`.
- **Versioning:** `schema_version` integer field in every report; breaking changes bump version and retain golden fixtures.

### CLI glue

- **Owner:** existing `cli/` Go module (`github.com/agentclash/agentclash/cli`).
- **Command namespace:** `agentclash evaltest` (distinct from hosted `prompt-eval`, `eval`, and `ci run`).
- **Responsibilities:**
  - test discovery and orchestration;
  - JSON/JUnit/SARIF report emission;
  - stable exit codes (see #1106);
  - eval-pack promotion (`evaltest promote-failures`);
  - GitHub Action integration (docs + example workflow).

Go remains the control plane for CI orchestration; Python/TS SDKs are the authoring surface.

---

## Scope boundaries

### In scope — local SDK v0

| Capability | Owner |
|------------|-------|
| `assert_agent` / `evaluate` Python API | `sdk/python/agentclash_eval` |
| 10 deterministic + judge-backed metrics | `sdk/python/agentclash_eval/metrics` |
| Plain-function and framework adapters | `sdk/python/agentclash_eval/adapters` |
| Versioned JSON report + JUnit output | `schemas/evaltest`, `cli/cmd/evaltest` |
| Opt-in pytest plugin | `sdk/python/agentclash_eval/pytest` |
| `agentclash evaltest init/run/promote-failures` | `cli/cmd/evaltest` |
| Local examples and CI docs | `examples/evaltest`, `docs/evaltest` |
| Failure → eval-pack YAML promotion | `cli/cmd/evaltest` + existing pack validation |

### Out of scope — remains in hosted AgentClash

| Capability | Owner |
|------------|-------|
| Sandbox provisioning (E2B) | `backend/internal/sandbox` |
| Temporal run workflows | `backend/internal/workflow` |
| Live scoring and replay | `backend/internal/engine` |
| Workspace auth and tenancy | `backend/internal/api` |
| Hosted eval sessions and scorecards | `backend/internal/api/eval_sessions` |
| Agent-vs-agent races | Run engine |
| `--upload` to hosted workspace | v1.1+ bridge (not v0) |

Local evals are the adoption wedge; hosted AgentClash is the graduation path.

---

## Non-negotiables

These apply to every SDK and CLI surface in v0:

1. **No auth required** for local eval execution.
2. **No telemetry by default** — no analytics, crash reporting, or usage beacons in core packages.
3. **No hidden network calls** — network access only when the user configures a judge/model provider.
4. **No implicit test-runner side effects** — importing the SDK must not register pytest/Jest plugins.
5. **Minimal dependency core** — framework and provider SDKs are optional extras only.
6. **Bounded async** — cancellation-safe execution with deterministic timeouts in CI.
7. **Stable exit codes** (evaltest runner):

   | Code | Meaning |
   |------|---------|
   | 0 | All evals passed |
   | 1 | Eval assertions failed |
   | 2 | Config/test authoring error |
   | 3 | Provider/runtime error |
   | 4 | Internal SDK/runner error |

8. **JSON and JUnit output from day one** — every run produces machine-readable artifacts.
9. **Reproducible failure evidence** — every failure includes enough context to debug without hosted AgentClash.

---

## Contract-sync plan

### Schema authority

- **Source of truth:** `schemas/evaltest/*.schema.json` in this repo.
- Python and TypeScript SDKs **must** emit reports validating against these schemas.
- Golden fixtures in `schemas/evaltest/fixtures/` are the regression oracle.

### CI validation (monorepo)

1. **Schema lint:** JSON Schema files pass `@redocly/cli` or equivalent validation.
2. **Golden fixture validation:** each fixture in `schemas/evaltest/fixtures/` validates against `eval-report.schema.json`.
3. **Python round-trip:** SDK unit tests produce reports that match golden fixtures (pass, metric failure, provider error, malformed config, multi-turn).
4. **CLI round-trip:** `agentclash evaltest run` output validates against the same schema.
5. **TypeScript parity (Phase 2):** TS SDK reports validate against identical schema.

### Cross-language parity

- Field names use `snake_case` in JSON reports (matches existing AgentClash conventions in eval packs and prompt-eval schemas).
- `schema_version` is an integer; consumers reject unknown versions with exit code 2.
- Metric results, tool calls, and retrieval context shapes are defined once in JSON Schema and mirrored as typed structs in Python (dataclasses/Pydantic) and TypeScript (Zod or equivalent).

### Future split sync (when triggered)

If/when the SDK moves to `agentclash/agentclash-evals`:

1. Schemas are copied or submodule-linked; this repo imports released schema versions as a dev dependency.
2. A weekly CI job in **both** repos validates fixture parity.
3. Eval-pack promotion logic stays in this repo's CLI; the SDK repo ships only report generation.
4. Release Please manages independent semver for SDK packages.

---

## Language priority

1. **Python SDK first** — largest eval-framework adoption signal; ships in v0.
2. **TypeScript SDK second** — after Python MVP and schema stabilize (#1114 spike → alpha).
3. **Go as CLI/control plane only** — not a first-class SDK authoring surface.
4. **Java/C# later** — only with explicit enterprise pull.

---

## Initial metric catalog (v0)

Ten metrics at launch — deterministic first, then judge-backed:

| # | Metric | Type |
|---|--------|------|
| 1 | TaskCompletion | judge |
| 2 | ToolCorrectness (ToolCalled) | deterministic |
| 3 | ToolArgumentCorrectness | judge |
| 4 | OutputSchema | deterministic |
| 5 | Contains / RegexMatch / JSONPath | deterministic |
| 6 | SafetyPolicy | judge |
| 7 | RetrievalGrounding | judge |
| 8 | CostLimit | deterministic |
| 9 | LatencyLimit | deterministic |
| 10 | StepEfficiency | judge |

Custom metrics must be easy to add; avoid launching with 50+ built-in metrics.

---

## Agent result data model (summary)

All adapters map framework traces into a common shape (full schema in #1106):

```json
{
  "input": "...",
  "output": "...",
  "messages": [],
  "tool_calls": [],
  "retrieval_context": [],
  "metadata": {
    "model": "...",
    "latency_ms": 1234,
    "cost_usd": 0.01
  }
}
```

---

## Success criteria (epic-level)

- Time to first local passing eval < 5 minutes.
- No account required for first eval.
- CI setup < 10 lines of YAML.
- Python package reaches meaningful adoption before TS launch.
- At least one end-to-end failure promotion path used in the wild.

---

## Consequences

### Positive

- Single PR can land schema + SDK + CLI + docs + examples atomically.
- Eval-pack promotion reuses existing validation in `cli/cmd/eval_pack.go`.
- Agents authoring evals can reference one repo for skills and examples.

### Negative / trade-offs

- Monorepo CI grows slightly (Python + Go test matrix for evaltest).
- PyPI/npm publish cadence is coupled to AgentClash releases until split.
- SDK contributors need clone of full monorepo (mitigated by sparse checkout later).

### Neutral

- `prompt-eval` CLI command remains for hosted prompt experiments; `evaltest` is the local pre-deploy path. Names are intentionally distinct.

---

## References

- Epic: [#1104](https://github.com/agentclash/agentclash/issues/1104)
- Existing prompt-eval schema: `docs/schemas/prompt-eval-result.schema.json`
- Eval pack v0 contract: `docs/evaluation/eval-pack-v0.md`
- CLI exit code registry: `cli/cmd/exit_codes.go`
