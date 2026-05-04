# codex/issue-450-challenge-pack-artifacts-skill — Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/challenge-pack-skills/agentclash-challenge-pack-artifacts/SKILL.md` for issue #450.
- The skill must document exact YAML fields for `assets`, `artifact_refs`, case `artifacts`, input/expectation `artifact_key`, and artifact evidence.
- It must explain version assets versus challenge/case assets and which references must point to version assets.
- It must document produced file capture through scoring `post_execution_checks` and file/artifact evidence references without inventing unsupported CLI upload/download commands.
- It must include hosted validation commands and review-only evidence patterns.

## Unit Tests
- Add docs assertions covering: `version.assets`, `artifact_refs`, `artifacts`, `artifact_key`, `path`, `media_type`, `artifact_id`, `post_execution_checks`, `file_capture`, `artifact.<path>`, and `agentclash challenge-pack validate`.

## Integration / Functional Tests
- Run `npm test -- src/lib/docs.test.ts` from `web/`.
- Run `go test ./internal/challengepack ./internal/scoring` from `backend/`.

## Smoke Tests
- Run `git diff --check`.
- Keyword sanity for artifact-related terms.

## E2E Tests
N/A locally — PR blind harness covers hosted self-containment.

## Manual / cURL Tests
- Review against `backend/internal/challengepack/bundle.go`, `backend/internal/challengepack/validation.go`, `backend/internal/scoring/spec.go`, and `backend/internal/scoring/validation.go`.
