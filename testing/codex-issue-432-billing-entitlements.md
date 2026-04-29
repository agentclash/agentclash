# codex/issue-432-billing-entitlements - Test Contract

Issue: https://github.com/agentclash/agentclash/issues/432

## Functional Behavior

- Every organization resolves to an effective billing entitlement state. Existing and new organizations default to the `free` plan until an active Dodo-backed subscription materializes paid entitlements.
- The canonical backend plan catalog exposes `free`, `pro`, `team`, and `enterprise` with the landing-page limits from issue #432:
  - `free`: 1 seat, 1 workspace, 25 races per workspace per month, 4 max models per race, 7 day replay retention, concurrency 1.
  - `pro`: minimum 5 seats, 1 workspace, 500 races per paid seat per workspace per month, 8 max models per race, 30 day retention, concurrency 3.
  - `team`: paid seats, multiple workspaces, 2000 races per paid seat per workspace per month, 12 max models per race, 90 day retention, concurrency 10.
  - `enterprise`: contract/custom limits, contract retention, feature flags for SSO/SAML, org audit logs, SLA, dedicated support, custom billing.
- Dodo product and add-on IDs map deterministically to backend `plan_key`, billing period, seat quantity, and feature flags. No Stripe-specific tables, env vars, docs, or API names are introduced.
- Checkout creates a durable local intent and returns a hosted Dodo checkout URL. Return URLs and query params never provision paid access by themselves.
- Dodo webhooks are verified with Standard Webhooks headers, persisted idempotently by `webhook-id`, tolerate duplicates and stale out-of-order events, and update local materialized organization entitlements.
- Workspace entitlement APIs return effective workspace limits, usage, remaining quota, reset time, and structured gate summaries.
- Workspace role authorization remains separate from billing: viewers still cannot mutate even on paid plans; active member/admin callers inherit the organization plan for every workspace they can access.
- Workspace creation enforces the plan workspace limit.
- Organization and workspace member invitation/activation enforce billable seat limits using active organization members without double-counting the same user across multiple workspaces.
- Run creation and eval-session creation enforce:
  - max models per race,
  - monthly race quota pooled by workspace,
  - active concurrency cap across queued/provisioning/running/scoring runs,
  - structured entitlement errors with `plan_key`, `upgrade_target`, `limit`, `used`, `remaining`, and `reset_at` where applicable.
- Quota/concurrency check and increment is race-safe in the same repository transaction that queues the run/session.
- Replay retention is represented in the entitlement read model. Cleanup can be a follow-up worker if no replay cleanup worker exists in this slice, but API output must expose the retention limit.

## Unit Tests

- `backend/internal/billing`
  - plan catalog returns all four plans with the expected limits and feature flags.
  - Dodo product/add-on/status mapping resolves plan key, billing period, active/inactive state, and seat quantity.
  - invalid plan keys, below-minimum seat quantities, unknown Dodo product IDs, and inactive statuses fail with stable errors.
  - entitlement resolution falls back to Free when no active subscription exists.
  - gate decisions produce stable machine-readable codes for feature, quota, concurrency, workspace, and seat blocks.
- `backend/internal/api`
  - run creation blocks Free at 5 models, Pro at 9, and Team at 13.
  - run creation blocks quota exhausted and concurrency exhausted.
  - eval-session creation applies the same participant, quota, and concurrency rules as single-run creation.
  - billing handlers validate input and return structured JSON errors.
  - webhook handler rejects missing/invalid Standard Webhooks signatures and accepts valid signed events.
- Membership/workspace managers
  - workspace creation blocks when the active workspace count equals the plan limit.
  - org invite and workspace invite/activation block when active org members equal the plan seat limit.
  - repeated workspace membership for an existing org member does not consume an extra seat.

## Integration / Functional Tests

- Repository integration tests with `DATABASE_URL` set:
  - migrations create billing accounts, subscriptions, checkout intents, webhook events, materialized entitlements, and workspace usage windows.
  - migration/backfill makes existing organizations resolvable as Free.
  - Dodo webhook event insert is idempotent by `webhook-id`.
  - active subscription upsert updates materialized organization entitlements.
  - run and eval-session creation atomically consume quota and refuse concurrent over-limit attempts.
- API integration/handler tests:
  - `GET /v1/billing/plans` returns the canonical catalog.
  - `GET /v1/organizations/{organizationID}/billing` returns admin-visible subscription and entitlement state.
  - `POST /v1/organizations/{organizationID}/billing/checkout` creates an intent and returns a checkout URL without provisioning paid access.
  - `GET /v1/workspaces/{workspaceID}/entitlements` returns the inherited plan plus workspace usage.
  - `POST /v1/dodo/webhooks` verifies signatures, deduplicates events, and updates local entitlements.

## Smoke Tests

- `go test -short -race -count=1 ./...` from `backend/`.
- Local stack:
  - `make db-up`
  - `make db-migrate`
  - start `go run ./cmd/api-server` from `backend/`
  - `curl http://localhost:8080/healthz` returns HTTP 200 and `{"ok":true,"service":"api-server"}`.
- API smoke:
  - authenticated `GET /v1/billing/plans` returns HTTP 200 with `free`, `pro`, `team`, and `enterprise`.
  - authenticated `GET /v1/workspaces/{workspaceID}/entitlements` returns HTTP 200 for an authorized workspace member.

## E2E Tests

- With the local stack running and dev auth headers:
  - create or use an organization/workspace that has Free entitlements.
  - verify `GET /v1/workspaces/{workspaceID}/entitlements` reports Free limits.
  - attempt to create a run with more than 4 deployments and verify HTTP 400/403 equivalent structured entitlement error `plan_limit_exceeded`.
  - seed/apply a Pro subscription entitlement for the organization, verify the same workspace reports Pro limits, and verify up to 8 deployments is allowed while 9 is rejected.
  - fill the workspace usage window to the plan quota and verify the next run is rejected as `quota_exceeded`.
  - seed active queued/running runs up to the concurrency limit and verify the next run is rejected as `concurrency_limit_exceeded`.

## Manual / cURL Tests

Use local dev auth headers:

```bash
export API=http://localhost:8080
export USER_ID=22222222-2222-2222-2222-222222222222
export ORG_ID=33333333-3333-3333-3333-333333333333
export WORKSPACE_ID=11111111-1111-1111-1111-111111111111
export AUTH_ORG="${ORG_ID}:org_admin"
export AUTH_WS="${WORKSPACE_ID}:workspace_admin"
```

Health:

```bash
curl -i "${API}/healthz"
# Expected: HTTP 200, body contains {"ok":true,"service":"api-server"}
```

Plan catalog:

```bash
curl -i \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Org-Memberships: ${AUTH_ORG}" \
  -H "X-Agentclash-Workspace-Memberships: ${AUTH_WS}" \
  "${API}/v1/billing/plans"
# Expected: HTTP 200, body contains plan keys free, pro, team, enterprise.
```

Workspace entitlements:

```bash
curl -i \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Org-Memberships: ${AUTH_ORG}" \
  -H "X-Agentclash-Workspace-Memberships: ${AUTH_WS}" \
  "${API}/v1/workspaces/${WORKSPACE_ID}/entitlements"
# Expected: HTTP 200, body contains plan_key and gate summaries.
```

Billing state:

```bash
curl -i \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Org-Memberships: ${AUTH_ORG}" \
  -H "X-Agentclash-Workspace-Memberships: ${AUTH_WS}" \
  "${API}/v1/organizations/${ORG_ID}/billing"
# Expected: HTTP 200 for org admin, body contains entitlement and subscription summary.
```

Checkout intent:

```bash
curl -i -X POST \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Org-Memberships: ${AUTH_ORG}" \
  -H "X-Agentclash-Workspace-Memberships: ${AUTH_WS}" \
  -d '{"plan_key":"pro","billing_period":"monthly","seat_quantity":5,"return_url":"http://localhost:3000/billing/return"}' \
  "${API}/v1/organizations/${ORG_ID}/billing/checkout"
# Expected: HTTP 201, body contains checkout_url and checkout_intent_id. Organization remains Free until webhook/reconciliation.
```

Dodo webhook:

```bash
export WEBHOOK_BODY='{"type":"subscription.active","data":{"subscription_id":"sub_test","customer_id":"cus_test","product_id":"agentclash_pro_monthly","status":"active","quantity":5},"created_at":"2026-04-29T00:00:00Z"}'
curl -i -X POST \
  -H "Content-Type: application/json" \
  -H "webhook-id: wh_test_432" \
  -H "webhook-timestamp: 1777420800" \
  -H "webhook-signature: replace-with-valid-signature" \
  --data "${WEBHOOK_BODY}" \
  "${API}/v1/dodo/webhooks"
# Expected with valid signature: HTTP 202/204 and entitlement materialized. With invalid signature: HTTP 401.
```

Run gate:

```bash
curl -i -X POST \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Org-Memberships: ${AUTH_ORG}" \
  -H "X-Agentclash-Workspace-Memberships: ${AUTH_WS}" \
  -d '{"workspace_id":"'"${WORKSPACE_ID}"'","challenge_pack_version_id":"replace-with-version","agent_deployment_ids":["a","b","c","d","e"]}' \
  "${API}/v1/runs"
# Expected for Free workspace after replacing real UUIDs with 5 valid deployments: structured entitlement error code plan_limit_exceeded.
```

## Local Stack Shutdown

- After verification, stop the API server process and run `make db-down` from the repository root.
