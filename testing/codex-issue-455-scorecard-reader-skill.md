# codex/issue-455-scorecard-reader-skill — Test Contract

## Functional Behavior
- Replace the scorecard reader stub with source-aligned instructions for interpreting AgentClash rankings, scorecards, replay steps, artifacts, judge rationale, and failure evidence.
- Use hosted production examples by default with `AGENTCLASH_API_URL="https://api.agentclash.dev"`.
- Document the exact CLI commands:
  - `agentclash run ranking <RUN_ID> [--sort-by ...]`
  - `agentclash run agents <RUN_ID>`
  - `agentclash run failures <RUN_ID>`
  - `agentclash run scorecard <RUN_AGENT_ID>`
  - `agentclash eval scorecard [RUN_ID] --agent <RUN_AGENT_ID_OR_LABEL>`
  - `agentclash replay get <RUN_AGENT_ID>`
  - `agentclash artifact list`
  - `agentclash artifact download <ARTIFACT_ID>`
- Do not document nonexistent forms such as `run scorecard <RUN_ID> <RUN_AGENT_ID>`, `replay get <RUN_ID> <RUN_AGENT_ID>`, or `artifact list --run <RUN_ID>`.
- Document the exact important JSON envelopes and fields from the CLI/API: ranking `state`, `message`, `ranking.sort`, `ranking.winner`, `ranking.evidence_quality`, `ranking.items`; scorecard top-level scores, nested `scorecard.dimensions`, `validator_details`, `metric_details`, `llm_judge_results`; replay `steps` and `pagination`; failures `items`, `clusters`, and `next_cursor`.
- Explain pending and errored stateful reads: 202 returns `state: "pending"` and exits successfully in human/JSON reads; 409 returns `state: "errored"` and exits with code 1.
- Include evidence-first interpretation guidance and a report-back template that avoids treating LLM judge rationale as ground truth without replay/artifact support.

## Unit Tests
- `web/src/lib/docs.test.ts` should assert the scorecard reader skill page exists and includes source-backed command names, important field names, failure mode wording, and related skills.

## Integration / Functional Tests
- `npm test -- src/lib/docs.test.ts` from `web/` must pass.
- `go test ./cmd -run 'TestContractAlignment|TestEval'` from `cli/` must pass to keep the documented CLI scorecard/replay/ranking behavior aligned.

## Smoke Tests
- `git diff --check` must pass.

## E2E Tests
- N/A — documentation-only skill update; no browser or hosted API calls are required.

## Manual / cURL Tests
- Read the final `SKILL.md` and verify every command, flag, field, state, and failure-mode claim is traceable to:
  - `cli/cmd/run.go`
  - `cli/cmd/replay.go`
  - `cli/cmd/artifact.go`
  - `cli/cmd/scorecard_helpers.go`
  - `cli/cmd/eval.go`
  - `backend/internal/api/replay_reads.go`
  - `backend/internal/api/run_ranking.go`
  - `backend/internal/api/failure_reviews.go`
  - `backend/internal/failurereview/read_model.go`
