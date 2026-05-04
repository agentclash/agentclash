# codex-issue-452-challenge-pack-llm-judges-skill — Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/challenge-pack-skills/agentclash-challenge-pack-llm-judges/SKILL.md` for issue #452.
- The skill must document exact `llm_judges` fields, supported judge modes, evidence input fields, rubric/assertion/reference/n_wise prompts, model/sample/consensus settings, scorecard pairing, judge limits, and abstention/safety guidance.
- It must explain when to pair judges with deterministic validators and when to avoid subjective judges.
- It must use hosted validation commands that match the current CLI.
- It must not invent judge modes, consensus modes, evidence prefixes, model/provider fields, command flags, or output fields.

## Unit Tests
- Add docs assertions covering: `llm_judges`, `judge_mode: hybrid`, `rubric`, `assertion`, `reference`, `n_wise`, `context_from`, `reference_from`, `score_scale`, `consensus`, `models`, `samples`, `judge_limits`, `max_samples_per_judge`, `source: llm_judge`, `judge_key`, `anti_gaming_clauses`, and `agentclash challenge-pack validate`.

## Integration / Functional Tests
- Run `npm test -- src/lib/docs.test.ts` from `web/`.
- Run `go test ./internal/scoring` from `backend/`.

## Smoke Tests
- Run `git diff --check`.
- Keyword sanity for judge-related terms.

## E2E Tests
N/A locally — PR blind harness covers hosted self-containment.

## Manual / cURL Tests
- Review against `backend/internal/scoring/spec.go`, `backend/internal/scoring/validation_judges.go`, `backend/internal/scoring/validation.go`, and LLM judge evaluator files under `backend/internal/scoring`.
