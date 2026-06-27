# codex/agentic-self-instruct-phase-1-2 - Test Contract

## Functional Behavior

- Existing `self_instruct` / `self-instruct` generation requests remain backward compatible.
- New `agentic_self_instruct` / `agentic-self-instruct` strategy is accepted at parsing/API/CLI/UI boundaries.
- Agentic generation config supports judge-only semantic filtering fields:
  - `judge_provider_account_id`
  - `judge_model`
  - `max_rounds_per_example`
  - `acceptance_mode`
  - `min_gap`
  - `max_weak_score`
  - `min_strong_score`
- Phase 2 behavior is judge-only filtering before full weak/strong deployment execution:
  - challenger generates a candidate using the existing generation provider/model.
  - for `agentic_self_instruct`, a judge model evaluates candidate quality after parse/schema/duplicate checks.
  - judge verdict `accept` allows upsert.
  - judge verdict `improve` or `reject` records a semantic rejection and continues generation attempts.
  - malformed judge output records a judge parse rejection and continues generation attempts.
- Agentic accepted examples are tagged with `synthetic` and `agentic`.
- Agentic accepted example metadata includes generator, job id, challenger model, judge model, verdict summary, scores, gap, and capability tags when available.
- Existing schema, duplicate, provider error, and version snapshot behavior still works.

## Unit Tests

- `generation.ParseStrategy` accepts `agentic_self_instruct` and `agentic-self-instruct`.
- `generation.DecodeJobConfig` validates judge fields for agentic mode while preserving self-instruct validation.
- `generation.ParseAgenticJudgeResponse` accepts valid JSON with `accept`, `improve`, and `reject`.
- `generation.ParseAgenticJudgeResponse` rejects malformed JSON, missing verdict, invalid verdict, and out-of-range scores.
- `generation.ShouldAcceptJudgeVerdict` respects judge mode and threshold guardrails.
- Existing self-instruct generation tests continue to pass.

## Integration / Functional Tests

- API generation tests cover:
  - existing self-instruct request still succeeds.
  - agentic request requires judge provider/model.
  - agentic request writes the expected config JSON.
- Workflow generation tests cover:
  - agentic judge `accept` creates an example with `agentic` tag and metadata.
  - judge `reject` records a semantic rejection and does not upsert that candidate.
  - judge malformed output records a judge parse rejection.

## Smoke Tests

- `go test ./internal/datasets/generation ./internal/api ./internal/workflow` from `backend/` passes.
- `go test ./cmd` from `cli/` passes or targeted dataset generate tests pass if full suite is too slow.
- Relevant web type/test command passes for touched dataset generation UI files, or TypeScript typecheck passes.

## E2E Tests

N/A - Phase 1/2 does not execute full AgentClash weak/strong deployments. E2E coverage belongs to the later deployment-loop phase.

## Manual / cURL Tests

- Start self-instruct generation request still accepts:

```bash
curl -X POST "$API/v1/workspaces/$WS/datasets/$DATASET/generations" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"strategy":"self_instruct","target_count":1,"provider_account_id":"'$PROVIDER'","model":"gpt-4.1-mini"}'
```

- Start agentic judge-only generation request accepts:

```bash
curl -X POST "$API/v1/workspaces/$WS/datasets/$DATASET/generations" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"strategy":"agentic_self_instruct","target_count":1,"provider_account_id":"'$PROVIDER'","model":"gpt-4.1-mini","judge_provider_account_id":"'$PROVIDER'","judge_model":"gpt-4.1-mini","max_rounds_per_example":3,"acceptance_mode":"judge"}'
```
