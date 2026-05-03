# codex/issue-491-pack-slug-resolve - Test Contract

## Functional Behavior
- `GET /v1/workspaces/{workspaceID}/challenge-packs` includes each pack's durable `slug` alongside `id`, `name`, `description`, `versions`, and timestamps.
- `agentclash eval start --pack <slug>` can resolve a published pack by exact slug because the CLI receives the slug field in the list response.
- Existing ID and exact-name resolution behavior remains unchanged.

## Unit Tests
- `TestListChallengePacksHandlerIncludesSlug` - the challenge-pack list API response serializes a `slug` field.
- Existing CLI eval selector tests continue to pass, including `TestEvalStartResolvesSelectorsAndCreatesRun`.

## Integration / Functional Tests
- `go test ./internal/api ./internal/repository ./cmd` from `cli/` or relevant module roots where applicable.

## Smoke Tests
- `cd cli && go test ./cmd -run TestEvalStartResolvesSelectorsAndCreatesRun -count=1`.
- `cd backend && go test ./internal/api -run TestListChallengePacksHandlerIncludesSlug -count=1`.

## E2E Tests
N/A - this is a response shape and selector-resolution fix covered by API and CLI tests.

## Manual / cURL Tests
```bash
curl -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
  "$AGENTCLASH_API_URL/v1/workspaces/$AGENTCLASH_WORKSPACE/challenge-packs"
# Expected: each item contains "slug": "<pack-slug>".
```
