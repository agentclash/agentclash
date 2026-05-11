# Roadmap #693: provider account smoke test

Issue: #718
Branch: codex/roadmap-693-provider-account-test

## Change

- Added `POST /v1/provider-accounts/{accountID}/test`.
- Added `agentclash infra provider-account test <id>`.
- The smoke test performs one bounded provider invocation with a small prompt.
- The response omits credential references and raw provider payloads; failure messages are redacted against loaded workspace secret values.

## Verification

- `cd backend && go test ./internal/api -run 'TestInfrastructureManagerTestProviderAccount|TestGetRuntimeProfile' -count=1`
- `cd cli && go test ./cmd -run 'TestInfraProviderAccountTest|TestInfraProviderAccountListCallsCorrectEndpoint' -count=1`
- `cd backend && go test ./internal/api ./internal/provider -count=1`
- `cd cli && go test ./...`
- `git diff --check`
