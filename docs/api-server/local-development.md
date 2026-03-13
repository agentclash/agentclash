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
- development-only header-backed auth endpoints:
  - `GET /v1/auth/session`
  - `GET /v1/workspaces/{workspaceID}/auth-check`

It does not yet connect to Postgres or Temporal. This step only establishes the process boundary and shared HTTP plumbing for later `/v1/*` work.

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

## Config

- `API_SERVER_BIND_ADDRESS`: HTTP bind address. Default `:8080`
- `DATABASE_URL`: reserved for upcoming Postgres-backed API work. Default local dev connection string
- `TEMPORAL_HOST_PORT`: reserved for upcoming workflow-start integration. Default `localhost:7233`
- `TEMPORAL_NAMESPACE`: reserved for upcoming Temporal client setup. Default `default`
