# codex-agent-tryout-quota-spend-caps - Test Contract

## Functional Behavior
- Anonymous tryouts are gated before persistence and before paid execution using a hashed fingerprint and a rolling 24 hour window.
- Anonymous quota exhaustion returns a stable product-facing `anonymous_quota_exhausted` error that the UI can render as sign-in or try-again-later.
- Global hosted anonymous spend is checked before persistence/dispatch using existing anonymous tryout rows as the durable ledger.
- Hosted spend accounting fails closed: if the repository cannot read quota/spend state, the API returns a stable `hosted_spend_unavailable` error and does not create a tryout.
- Global spend cap exhaustion returns a stable `hosted_spend_exhausted` error and does not create a tryout.
- Per-template `max_cost_usd` must not exceed the configured anonymous per-run cap; oversized templates fail with `tryout_cost_cap_exceeded` before persistence.
- Existing workspace tryout behavior remains authorized by workspace permissions and is not blocked by anonymous fingerprint quota.
- Tryout snapshots continue to persist server-owned cost limits and runtime/tool/evaluation policy.

## Unit Tests
- `TestAgentTryoutManagerAllowsAnonymousWithinQuotaAndSpendCaps` - creates one anonymous tryout when quota and spend checks pass.
- `TestAgentTryoutManagerRejectsAnonymousQuotaExhaustedBeforeCreate` - second anonymous tryout within the window fails before persistence/dispatch.
- `TestAgentTryoutManagerRejectsHostedSpendUnavailableBeforeCreate` - repository quota/spend read errors fail closed.
- `TestAgentTryoutManagerRejectsHostedSpendCapExceededBeforeCreate` - projected daily spend over cap fails before persistence/dispatch.
- `TestAgentTryoutManagerRejectsPerRunCostCapExceededBeforeCreate` - template cost above configured per-run cap fails before persistence/dispatch.
- `TestCreateAnonymousAgentTryoutHandlerMapsQuotaAndSpendErrors` - handler returns stable UI error codes for quota/spend/cost cap failures.

## Integration / Functional Tests
- Repository tests verify counting anonymous tryouts by fingerprint and summing anonymous cost-limit ledger rows across a time window.
- Existing agent tryout repository tests continue to pass.

## Smoke Tests
- `go test -short -count=1 ./internal/api -run 'AgentTryout|Quota|Spend|CostCap'`
- `go test -short -count=1 ./internal/repository -run AgentTryout`
- `go test -short -count=1 ./...`
- `go vet ./...`

## E2E Tests
- N/A - full E2B/provider execution requires external credentials. This change must still curl local API routes to verify quota, spend, and cost-cap responses.

## Manual / cURL Tests
- With default config, `POST /v1/agent-tryouts` for valid `meeting-minutes` input returns `201` and preserves `cost_limit_usd`.
- With anonymous quota set to zero, `POST /v1/agent-tryouts` returns `429 anonymous_quota_exhausted`.
- With daily hosted spend cap set below the template cost limit, `POST /v1/agent-tryouts` returns `429 hosted_spend_exhausted`.
- With per-run cap set below the template cost limit, `POST /v1/agent-tryouts` returns `400 tryout_cost_cap_exceeded`.
- Repeating the same anonymous fingerprint after one successful create returns `429 anonymous_quota_exhausted`.
