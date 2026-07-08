# feat/local-m1-byo-keys — Test Contract

Parent: #1154 (Local M1 [2/5]: BYO keys — local credential resolution)
Epic: #1147

## Functional Behavior

- Add a **local-only** credential resolver in `runtime/provider/local` that resolves provider API keys entirely on the user's machine with this precedence:
  1. Process environment (`env://VAR`, `secret://name` → existing env candidate rules)
  2. Local provider-keys file (`~/.config/agentclash/provider_keys.yaml`, or `$XDG_CONFIG_HOME/agentclash/provider_keys.yaml`)
  3. Optional OS keychain (service `agentclash.local.providers`, account = provider key)
- Explicit provider → default credential reference / env var mapping for router providers: `openai`, `anthropic`, `gemini`, `xai`, `openrouter`, `mistral` (aligned with `NewDefaultRouter` + judge defaults).
- Fail closed:
  - Missing key → `FailureCodeCredentialUnavailable` with an actionable message listing tried sources (env var names, config path, keychain account).
  - `workspace-secret://…` is **rejected** by the local resolver (never loads hosted workspace secrets).
- Helper to construct a `provider.Router` for local runs using only the local resolver (`NewLocalRouter` / `NewDefaultLocalRouter`) — no hosted secret APIs, no Postgres/Temporal.
- Document the BYO key surface (env vars + provider-keys file + optional keychain) under `docs/evaluation/local-byo-keys.md`.
- Do **not** add `agentclash local run`, Docker changes, harness-builder, local UI, or hosted BYOK/workspace-secret sync in this PR.
- Hosted `EnvCredentialResolver` + `workspace-secret://` behavior remains unchanged for the control-plane path.

## Unit Tests

- Resolution order: env wins over config; config wins over keychain; keychain used when env+config empty.
- Missing key: fail-closed with `FailureCodeCredentialUnavailable` and message that mentions the expected env var / config path.
- `workspace-secret://` rejected by local resolver (`ErrHostedSecretRejected`).
- Provider default mapping covers all `NewDefaultRouter` provider keys.
- `NewLocalRouter` builds a router whose adapters use the local resolver; no network / AgentClash API calls in unit tests (fake keychain + temp config dir).
- `rg "backend/internal|postgres|pgx|sqlc|temporal" runtime/provider/local` returns no matches in production code.

## Integration / Functional Tests

- `cd runtime && go test -short -race -count=1 ./provider/...` passes.
- `cd runtime && go test -short -race -count=1 ./...` passes.
- Existing hosted env/workspace-secret tests in `runtime/provider` remain green (no behavior change to `EnvCredentialResolver`).

## Smoke Tests

N/A for live provider calls in this PR.

## E2E Tests

N/A — full laptop pack eval is #1157; CLI wiring is #1156. This issue only ships local credential resolution + docs.

## Manual / cURL Tests

N/A — library + docs. Maintainer verification:

1. Write `provider_keys.yaml` with a dummy key; confirm unit tests / resolver returns it when env unset.
2. Confirm docs list env vars for openai/anthropic/gemini/xai/openrouter/mistral.
