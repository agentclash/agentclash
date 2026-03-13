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

## Config

- `API_SERVER_BIND_ADDRESS`: HTTP bind address. Default `:8080`
- `DATABASE_URL`: reserved for upcoming Postgres-backed API work. Default local dev connection string
- `TEMPORAL_HOST_PORT`: reserved for upcoming workflow-start integration. Default `localhost:7233`
- `TEMPORAL_NAMESPACE`: reserved for upcoming Temporal client setup. Default `default`
