# codex/issue-451-challenge-pack-scoring-validators-skill — Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/challenge-pack-skills/agentclash-challenge-pack-scoring-validators/SKILL.md` for issue #451.
- The skill must document deterministic validator fields, exact supported validator types, scorecard dimension fields, evidence references, pass/fail output expectations, metrics, and safe examples.
- It must explain file validators and `post_execution_checks` consistently with the artifact skill and scoring source.
- It must use hosted defaults and validation commands that match the current CLI.
- It must not invent validator types, scorecard sources, evidence prefixes, command flags, or JSON/YAML fields.

## Unit Tests
- Add docs assertions covering: `version.evaluation_spec.validators`, `exact_match`, `contains`, `regex_match`, `json_schema`, `json_path_match`, `file_json_schema`, `directory_structure`, `code_execution`, `target`, `expected_from`, `literal:`, `file:<post_execution_check_key>`, `scorecard.dimensions`, `source: validators`, and `agentclash challenge-pack validate`.

## Integration / Functional Tests
- Run `npm test -- src/lib/docs.test.ts` from `web/`.
- Run `go test ./internal/scoring` from `backend/`.

## Smoke Tests
- Run `git diff --check`.
- Keyword sanity for validator and scorecard terms.

## E2E Tests
N/A locally — PR blind harness covers hosted self-containment.

## Manual / cURL Tests
- Review against `backend/internal/scoring/spec.go`, `backend/internal/scoring/validation.go`, validator implementation files in `backend/internal/scoring/*_validators.go`, and `cli/cmd/challenge_pack.go`.
