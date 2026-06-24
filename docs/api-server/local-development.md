# API Server Local Development

Purpose: give the repo one short path for starting the first control-plane HTTP server introduced in issue `#7`.

## What exists in this step

This API server slice is intentionally thin.

It currently provides:

- config bootstrapping from environment
- `chi`-based router setup
- baseline JSON success and error envelopes
- request logging and panic recovery middleware
- `GET /healthz`
- request-scoped caller identity for `/v1/*`
- workspace authorization boundary for future run handlers
- `POST /v1/runs` backed by Postgres persistence and Temporal workflow start
- `GET /v1/runs/{id}` for durable run-detail bootstrap reads
- `GET /v1/runs/{id}/agents` for participant-lane bootstrap reads
- `GET /v1/replays/{runAgentId}` for per-lane replay bootstrap reads
- `GET /v1/replays/{runAgentId}/viewer` for a minimal HTML replay viewer wired to the replay API
- `GET /v1/scorecards/{runAgentId}` for per-lane scorecard bootstrap reads
- `POST /v1/integrations/hosted-runs/{runID}/events` for hosted external completion ingestion
- development-only header-backed auth endpoints:
  - `GET /v1/auth/session`
  - `GET /v1/workspaces/{workspaceID}/auth-check`

This server step now connects to both `Postgres` and `Temporal` so the first control-plane write path can create durable queued runs and then start `RunWorkflow`.

## Local startup

From the repository root:

```bash
cp backend/.env.example backend/.env
cd backend
set -a
source .env
set +a
go run ./cmd/api-server
```

The server listens on `:8080` by default, so health can be checked with:

```bash
curl http://localhost:8080/healthz
```

Expected response:

```json
{"ok":true,"service":"api-server"}
```

## Endpoint Contracts

### `POST /v1/runs`

Request body:

```json
{
  "workspace_id": "uuid",
  "challenge_pack_version_id": "uuid",
  "challenge_input_set_id": "uuid-or-null",
  "name": "optional human label",
  "agent_deployment_ids": ["uuid"]
}
```

Success response:

```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "challenge_pack_version_id": "uuid",
  "challenge_input_set_id": "uuid-or-null",
  "status": "queued",
  "execution_mode": "single_agent | comparison",
  "created_at": "timestamp",
  "queued_at": "timestamp",
  "links": {
    "self": "/v1/runs/{id}",
    "agents": "/v1/runs/{id}/agents"
  }
}
```

Status codes:

- `201` when the queued run was created and the workflow start succeeded
- `400` for malformed JSON or invalid benchmark/deployment references
- `401` when auth headers are missing or invalid
- `403` when the caller lacks workspace access
- `413` when the JSON body exceeds 1 MiB
- `415` when `Content-Type` is not `application/json`
- `502` when the run row was created but Temporal workflow start failed

### `GET /v1/runs/{id}`

Success response fields:

- `id`
- `workspace_id`
- `challenge_pack_version_id`
- `challenge_input_set_id`
- `name`
- `status`
- `execution_mode`
- `temporal_workflow_id`
- `temporal_run_id`
- `queued_at`
- `started_at`
- `finished_at`
- `cancelled_at`
- `failed_at`
- `created_at`
- `updated_at`
- `links`

Status codes:

- `200` when the run exists and belongs to one of the caller's workspaces
- `400` when `{id}` is not a UUID
- `401` when auth headers are missing or invalid
- `403` when the caller lacks workspace access
- `404` when the run does not exist

### `GET /v1/runs/{id}/agents`

Success response shape:

```json
{
  "items": [
    {
      "id": "uuid",
      "run_id": "uuid",
      "lane_index": 0,
      "label": "lane label",
      "agent_deployment_id": "uuid",
      "agent_deployment_snapshot_id": "uuid",
      "status": "queued",
      "queued_at": "timestamp",
      "started_at": "timestamp",
      "finished_at": "timestamp",
      "failure_reason": "string-or-null",
      "created_at": "timestamp",
      "updated_at": "timestamp"
    }
  ]
}
```

Status codes:

- `200` when the run exists and the caller is authorized
- `400`, `401`, `403`, and `404` with the same meanings as `GET /v1/runs/{id}`

### `GET /v1/replays/{runAgentId}`

Query params:

- `limit` optional step page size, defaults to `50`, max `200`
- `cursor` optional zero-based replay-step offset

Success response fields:

- `state`
- `message`
- `run_agent_id`
- `run_id`
- `run_agent_status`
- `replay`
- `steps`
- `pagination`

Status codes:

- `200` when a replay row exists for the requested run agent
- `202` when replay generation is still pending
- `409` when the run agent is terminal but replay generation failed or is unavailable
- `400` when `{runAgentId}` is not a UUID
- `401` when auth headers are missing or invalid
- `403` when the caller lacks workspace access
- `404` when the run agent does not exist

### `GET /v1/replays/{runAgentId}/viewer`

Purpose:

- render a small HTML replay page for first-user testing
- fetch replay data through `GET /v1/replays/{runAgentId}`
- make provider/tool/system boundaries and pending or errored replay states legible

Query params:

- `limit` optional step page size, defaults to `50`, max `200`
- `cursor` optional zero-based replay-step offset

Status codes:

- `200` when the HTML viewer shell is rendered
- `400` when `{runAgentId}` or pagination params are malformed
- `401` when auth headers are missing or invalid

Local example:

```bash
RUN_AGENT_ID=replace-with-real-run-agent-id
WORKSPACE_ID=11111111-1111-1111-1111-111111111111
USER_ID=22222222-2222-2222-2222-222222222222

curl \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_member" \
  http://localhost:8080/v1/replays/${RUN_AGENT_ID}/viewer
```

### `GET /v1/scorecards/{runAgentId}`

Success response fields:

- `state`
- `message`
- `run_agent_status`
- `id`
- `run_agent_id`
- `run_id`
- `evaluation_spec_id`
- `overall_score`
- `correctness_score`
- `reliability_score`
- `latency_score`
- `cost_score`
- `llm_judge_results`
- `scorecard`
- `created_at`
- `updated_at`

Status codes:

- `200` when a scorecard row exists for the requested run agent
- `202` when the run agent is still executing or awaiting scorecard generation
- `409` when the run agent is terminal but the scorecard is unavailable
- `400` when `{runAgentId}` is not a UUID
- `401` when auth headers are missing or invalid
- `403` when the caller lacks workspace access
- `404` when the run agent does not exist

Protected development endpoints currently use a temporary header-backed auth contract so the API-side tenancy boundary can be exercised before full WorkOS integration exists.

Example:

```bash
WORKSPACE_ID=11111111-1111-1111-1111-111111111111
USER_ID=22222222-2222-2222-2222-222222222222

curl \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  http://localhost:8080/v1/auth/session
```

Current development headers:

- `X-Agentclash-User-Id`: required UUID for authenticated requests
- `X-Agentclash-WorkOS-User-Id`: optional future-facing external identity reference
- `X-Agentclash-User-Email`: optional caller email
- `X-Agentclash-User-Display-Name`: optional caller display name
- `X-Agentclash-Workspace-Memberships`: optional comma-separated workspace access list in the form `workspace_uuid:role`

Error behavior:

- missing or invalid identity headers return `401`
- authenticated callers without the requested workspace membership return `403`
- missing replay or scorecard rows return `404`
- hosted callback requests require `Authorization: Bearer <token>` signed from `HOSTED_RUN_CALLBACK_SECRET`

To exercise the run-create path locally, start the local database first and point `TEMPORAL_HOST_PORT` at a reachable Temporal dev server or namespace, then call:

```bash
WORKSPACE_ID=11111111-1111-1111-1111-111111111111
CHALLENGE_PACK_VERSION_ID=33333333-3333-3333-3333-333333333333
DEPLOYMENT_ID=44444444-4444-4444-4444-444444444444
USER_ID=22222222-2222-2222-2222-222222222222

curl \
  -X POST http://localhost:8080/v1/runs \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  -d '{
    "workspace_id": "'"${WORKSPACE_ID}"'",
    "challenge_pack_version_id": "'"${CHALLENGE_PACK_VERSION_ID}"'",
    "agent_deployment_ids": ["'"${DEPLOYMENT_ID}"'"]
  }'
```

Once a run exists, the initial run-page bootstrap reads are:

```bash
RUN_ID=55555555-5555-5555-5555-555555555555

curl \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  http://localhost:8080/v1/runs/${RUN_ID}

curl \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  http://localhost:8080/v1/runs/${RUN_ID}/agents
```

```bash
RUN_AGENT_ID=66666666-6666-6666-6666-666666666666

curl \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  http://localhost:8080/v1/replays/${RUN_AGENT_ID}

curl \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  http://localhost:8080/v1/scorecards/${RUN_AGENT_ID}
```

## Config

- `API_SERVER_BIND_ADDRESS`: HTTP bind address. Default `:8080`
- `DATABASE_URL`: Postgres connection string used by the run-create API path. Default local dev connection string
- `TEMPORAL_HOST_PORT`: Temporal target used when starting `RunWorkflow`. Default `localhost:7233`
- `TEMPORAL_NAMESPACE`: Temporal namespace used by the API server. Default `default`
