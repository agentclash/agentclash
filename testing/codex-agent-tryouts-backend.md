# codex/agent-tryouts-backend - Test Contract

## Functional Behavior
- Agent tryout templates are available through a backend catalog with stable slugs, input schema, tool policy, evaluation spec, anonymous availability, and per-template cost/time caps.
- Creating an agent tryout validates template slug, JSON object input, anonymous eligibility, input byte caps, and per-template cost/duration limits.
- Anonymous tryouts store no organization/workspace/user ownership, store only an anonymous fingerprint hash, and expire by default.
- Signed-in tryouts require a workspace, authorize that workspace, and persist organization/workspace/user ownership.
- Tryout status supports a narrow lifecycle: `queued`, `running`, `completed`, `failed`, `cancelled`.
- Tryout reads return template metadata plus tryout state, input snapshot, backing run id when present, cost/latency summary, and timestamps.
- Claiming an anonymous tryout requires an authenticated workspace user, authorizes the destination workspace, writes ownership exactly once, and records claim metadata.
- Private share creation for tryouts requires an authenticated owner/workspace user and creates a `public_share_links` row with `search_indexing=false` by default.
- Backend routes are registered under `/v1` and OpenAPI documents the new resources.
- Public indexed sharing is not enabled by default.

## Unit Tests
- `TestAgentTryoutCatalogListsAnonymousTemplates` - returns active public templates and hides disabled ones.
- `TestAgentTryoutManagerCreateAnonymousValidatesTemplateAndInput` - accepts valid anonymous template input and stores limits/snapshot.
- `TestAgentTryoutManagerRejectsAnonymousWhenTemplateDisabled` - returns validation error for signed-in-only templates.
- `TestAgentTryoutManagerRejectsOversizedInput` - rejects input snapshots over the template cap.
- `TestAgentTryoutManagerCreateSignedInAuthorizesWorkspace` - loads organization id and persists workspace ownership.
- `TestAgentTryoutManagerClaimAnonymousTryout` - moves an anonymous tryout into a workspace and records claim metadata.
- `TestAgentTryoutManagerClaimRejectsAlreadyOwnedTryout` - prevents double-claim/ownership overwrite.
- `TestAgentTryoutManagerCreateShareDefaultsNoIndex` - share params always default `search_indexing=false`.
- `TestAgentTryoutHandlers` coverage for list/create/get/claim/share happy paths and validation/auth errors.

## Integration / Functional Tests
- SQLC generation succeeds for new agent tryout queries.
- Repository integration test covers create, get, list, claim, status update, and share-resource compatibility.
- Migration creates `agent_tryouts`, claim tracking, indexes, status checks, and extends `public_share_links.resource_type` to include `agent_tryout`.

## Smoke Tests
- `cd backend && go test -short -count=1 ./internal/api -run AgentTryout`
- `cd backend && go test -short -count=1 ./internal/repository -run AgentTryout`
- `cd backend && go test -short -count=1 ./...`

## E2E Tests
- N/A for this backend PR. The public UI and live worker execution will add browser-level E2E coverage when `/try/agent` exists.

## Manual / cURL Tests
- List templates:
  `curl -sS "$AGENTCLASH_API_URL/v1/agent-tryout-templates"`
- Create anonymous tryout:
  `curl -sS -X POST "$AGENTCLASH_API_URL/v1/agent-tryouts" -H 'Content-Type: application/json' -d '{"template_slug":"meeting-minutes","input":{"notes":"Alice will ship the backend by Friday."}}'`
- Get tryout:
  `curl -sS "$AGENTCLASH_API_URL/v1/agent-tryouts/<id>"`
- Claim signed-in tryout:
  `curl -sS -X POST "$AGENTCLASH_API_URL/v1/agent-tryouts/<id>/claim" -H "Authorization: Bearer $AGENTCLASH_TOKEN" -H 'Content-Type: application/json' -d '{"workspace_id":"<workspace-id>"}'`
- Create private share:
  `curl -sS -X POST "$AGENTCLASH_API_URL/v1/agent-tryouts/<id>/share" -H "Authorization: Bearer $AGENTCLASH_TOKEN" -H 'Content-Type: application/json' -d '{}'`
