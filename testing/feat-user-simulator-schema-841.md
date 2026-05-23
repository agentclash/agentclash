# feat/user-simulator-schema-841 — Test Contract

## Functional Behavior

- Challenge pack cases may declare a `user_simulator` block (schema v1, `kind: hybrid`) with phased actors: `scripted`, `llm`, `human`.
- `version.execution_mode: multi_turn` is accepted; every case in every input set must include a valid `user_simulator`.
- Non-`multi_turn` packs reject cases that declare `user_simulator`.
- Trigger values must be from the catalog: `always`, `on_assistant_mismatch`, `on_validator_fail`, `on_judge_below`, `on_agent_loop`, `on_max_llm_turns`, `manual`, `never`.
- Scripted phase turns require non-empty `message`; placeholders in messages are validated against each case context (via #840 templating).
- No execution behavior changes in this issue — validation only.

## Unit Tests

- `TestValidateUserSimulator_AcceptsHybridScriptedPhase` — minimal valid simulator
- `TestValidateUserSimulator_RejectsUnknownActor` — invalid actor
- `TestValidateUserSimulator_RejectsUnknownTrigger` — invalid trigger
- `TestValidateUserSimulator_ScriptedPhaseRequiresTurnMessages` — empty turns/messages fail
- `TestValidateUserSimulator_LLMPhaseRequiresPersona` — llm without persona fails
- `TestValidateBundle_MultiTurnRequiresUserSimulatorOnCases` — multi_turn pack validation
- `TestValidateBundle_NativeRejectsUserSimulator` — native + user_simulator fails
- `TestValidateBundleUserSimulatorTemplates_*` — unresolved `{{placeholder}}` in messages

## Integration / Functional Tests

- N/A — schema validation only.

## Smoke Tests

- `cd backend && go test -short -race -count=1 ./internal/challengepack/... -run 'UserSimulator|MultiTurn'`

## E2E Tests

- N/A — no executor wiring in this issue.

## Manual / cURL Tests

- N/A — `ValidateBundle` / pack upload path covered by unit tests.
