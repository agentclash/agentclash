# feat/provider-connection-service — Test Contract

## Functional Behavior

- OpenRouter pricing is marked `live` only when both prompt and completion prices are finite, non-negative numbers; otherwise the complete static fallback is used.
- Migration 00061 converts legacy dataset-generation `config.model_alias_id` values to `config.model` before alias/catalog rows are dropped.
- Native deployments, native snapshots, and playground experiments cannot persist a blank provider model; hosted-external historical rows remain migratable.
- Workspace members who can create deployments, playground experiments, and dataset generations can list models for a provider account in their workspace.
- Switching provider accounts while a model-list request is in flight cannot apply the old account's response to the new selection.
- CLI schemas, agent skills, developer fixtures, and E2E flows use provider model IDs and do not invoke removed model-alias/model-catalog commands or tables.

## Unit Tests

- `TestOpenRouterListModelsFallsBackWhenLivePricingIsPartial` — partial live pricing never produces a mixed live/zero price.
- `TestParseUSDPerTokenRejectsNonFiniteValues` — NaN and infinity are rejected.
- Provider-account model-list handler authorization tests cover workspace members and reject callers outside the workspace.
- Existing connection, provider, API, CLI, and web test suites remain green.

## Integration / Functional Tests

- Apply all backend migrations to a fresh PostgreSQL database.
- Apply migration 00061 to legacy rows containing deployments, snapshots, playground experiments, and dataset-generation JSON with `model_alias_id`; verify all provider model IDs survive.
- Run the CLI contract/schema tests and verify generated skill snapshots match their web-content sources.
- Run the local fixture scripts against the post-00061 schema when PostgreSQL is available.

## Smoke Tests

- Backend and CLI build successfully.
- Provider model list returns a JSON `items` array for an authorized workspace user.
- Deployment, playground, dataset-generation, ranking-insights, and CI payloads send `model` rather than `model_alias_id`.

## E2E Tests

- The CLI E2E resource flow creates a provider account, uses a raw model ID for deployments and experiments, and never calls removed catalog/alias commands.
- Full remote E2E execution is environment-dependent; local structural validation must still prove the script contains no removed surface.

## Manual / cURL Tests

- `GET /v1/provider-accounts/{accountID}/models` as a workspace member returns 200; a user from another workspace is denied.
- Create two rapid provider-account selections in each changed picker and verify only the latest request controls the visible models.
- Inspect migration 00061 and verify dataset job JSON is rewritten before `DROP TABLE model_aliases`.
