# Tryout rollout capabilities — Test Contract

## Functional Behavior

- GET /v1/agent-tryout-templates retains items and announces creator_access_protocol: cookie-v1 with Cache-Control: no-store on the creator-proof backend.
- Public frontend operations discover capabilities without cookies using fresh no-store requests. Only an absent field on a valid catalog selects legacy transport; unknown, null, malformed or failed discovery fails boundedly without a write. No sticky mode across clients/hosts, no downgrade or write retry.
- Anonymous create sends cookie-v1 and includes credentials only after explicit fresh support. New backend rejects missing/unsupported protocol with a structured reload/update error before service/quota/reservation/row/provider work or cookie issuance. Workspace creates remain unchanged.
- Public reads/events/turns use negotiated transport. Legacy results never advertise Save, even if an older server returns save_eligible. Claim requires fresh cookie-v1 support and never reaches older handlers. Existing terminal, workspace and creator-proof gates remain enforced.
- All public callers, including server-rendered/static demo helpers, use the focused tryout boundary. No generic credentials change, permissive CORS, database/SDK/evaluator/analytics change.

## Unit Tests

- Backend catalog announcement/no-store; missing, unknown and null create protocol reject before fake service invocation and Set-Cookie; supported create retains cookie privacy and existing error mapping.
- Frontend legacy and supported catalog transport for create/read/events/turns; fresh discovery on every operation; unsupported/malformed/network failure makes no dependent request; claim refuses legacy/unknown; 403 and network failures never retry without credentials.
- Legacy save_eligible normalized false; terminal Save UI and workspace/saved authentication unchanged; independent clients/API hosts discover independently.

## Integration / Functional Tests

- Existing creator ownership, CORS, workspace isolation, claim, event and quota focused tests pass.
- Affected frontend tests, TypeScript and lint; backend focused race tests, build and vet; validate OpenAPI, disclosing existing baseline findings only.
- OpenAPI documents additive discovery and anonymous-only required protocol without changing workspace creation input requirements.

## Smoke Tests

- Synthetic loopback fixtures only. No production mutations, deployments, provider execution, real identity/customer data or external database required.

## E2E Tests

- Preserve the original seven browser journeys against actual new handlers and fake identity provider.
- Controlled old-CORS server without Access-Control-Allow-Credentials demonstrates unconditional include failure, then actual frontend legacy create/read/events/turns success with no Save promise and no provider work.

## Manual / cURL Tests

- Covered by loopback browser/HTTP fixtures: inspect catalog/no-store, send stale create without protocol and verify structured rejection/no cookie/no service work, send supported create and verify creator cookie protection. Do not probe production.
- Source readiness is distinct from the separately staged/promoted backend release; no production rollout is claimed.
