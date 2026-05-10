# Roadmap #693: model alias create flags

Issue: #717
Branch: codex/roadmap-693-model-alias-create-flags

## Change

- Added non-interactive flags to `agentclash infra model-alias create`:
  - `--alias-key`
  - `--display-name`
  - `--model-catalog-entry-id`
  - `--provider-account-id` (optional, matching the backend create contract)
- Flags populate the JSON request body sent to `/v1/workspaces/{workspace}/model-aliases`.
- Existing `--from-file` support is preserved; explicitly supplied flags override matching file fields.

## Verification

- `cd cli && go test ./cmd -run 'TestInfraModelAliasCreateBuildsRequestBodyFromFlags|TestInfraModelAliasCreateMergesFromFileAndFlagOverrides|TestInfraModelAliasCreateAllowsOmittingProviderAccount|TestInfraModelAliasCreateValidatesRequiredFields|TestInfraProviderAccountListCallsCorrectEndpoint|TestInfraModelCatalogListCallsCorrectEndpoint' -count=1`
- `cd cli && go test ./...`
- `git diff --check`
