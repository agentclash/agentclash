# codex/issue-448-challenge-pack-input-sets-skill — Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/challenge-pack-skills/agentclash-challenge-pack-input-sets/SKILL.md` for issue #448.
- The skill must let a coding agent design and edit AgentClash `input_sets` without reading the source repo.
- It must document exact current YAML fields for `input_sets[].cases[]`, including `challenge_key`, `case_key`, `payload`, `inputs`, `expectations`, `artifacts`, and `assets`.
- It must explain smoke/full/regression/CI grouping, edge-case coverage, stable keys, duplicate prevention, and case review guidance.
- It must describe how case inputs and expectations reference version assets via `artifact_key` and how expectation `source` values work.
- It must avoid unsupported fields and stale legacy-first guidance; legacy `items` may be mentioned only as old input normalized to `cases`.
- Examples must use hosted production defaults when CLI commands are shown.

## Unit Tests
- Add source-backed assertions to `web/src/lib/docs.test.ts` for the input sets skill.
- Assertions must cover: `input_sets`, `cases`, `case_key`, `challenge_key`, `payload`, `inputs`, `expectations`, `source: input:prompt`, smoke/full/regression/CI guidance, and `agentclash challenge-pack validate`.

## Integration / Functional Tests
- Run `npm test -- src/lib/docs.test.ts` from `web/`.
- Run `go test ./internal/challengepack` from `backend/` to reverify the parser/validator surface referenced by the skill.

## Smoke Tests
- Run `git diff --check`.
- Run keyword sanity against the changed skill for `input_sets`, `cases`, `case_key`, `challenge_key`, `payload`, `inputs`, `expectations`, `artifact_key`, `source: input:prompt`, and `agentclash challenge-pack validate`.

## E2E Tests
N/A — docs-only skill change. The PR-level blind skill harness will provide hosted self-containment coverage after the PR opens.

## Manual / cURL Tests
- Manually review against `backend/internal/challengepack/bundle.go` and `backend/internal/challengepack/validation.go`.
- Manually review CLI command claims against `cli/cmd/challenge_pack.go`.
