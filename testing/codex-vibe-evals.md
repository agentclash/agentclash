# Vibe Evals implementation contract

This is the implementation contract for the product and safety plan reviewed on 2026-09-06. Changes stay on `codex/vibe-evals`; no deployment or provider calls are part of development verification.

## Functional behavior

- A public, current-theme conversation supports exploration, building a text agent, importing an editable evaluation, checking behavior, improvements and fair retests. Incomplete briefs get a useful question; complete briefs can produce a draft immediately. No score is invented for chat or an unexecuted draft.
- Assistant, target and evaluator model policies are independent and persisted. The coordinator proposes typed artifacts; it cannot execute arbitrary tools, URLs, code, deploy, publish, charge a card, or change permissions.
- Reuse the merged #1246 blueprint/compiler and #1245 editable pack conversion. Discontinued guide agents are not dependencies. No public tryout rows are fabricated.
- Private conversations, idempotent messages, immutable artifact revisions, structured requirement provenance, operation events, attempts, results and billing survive reconnects and crashes. Reconnecting never dispatches work.
- Every funded invocation requires a full-context bound, explicit output bound, current authorization, durable attempt journal and atomic reservation. Missing accounting, price profiles or Redis rejects new hosted work.
- Anonymous lifetime allowance is $1, 20 messages, 40 model calls, three initial cases and one retest. $0.75 is protected for authoring/check/retest. Global subsidy caps are required and reserve atomically. All limits are centralized, server enforced, with no silent truncation.
- Root operations have finite graph/call/time/cost limits; no recursive repair loop. Paid activities do not retry ambiguous provider calls. Stop ends future dispatch but keeps uncertain holds in reconciliation.
- Money is integer nano-USD. Concurrent reservations cannot overspend. Grants and verified payment credits are idempotent. In-flight reservations never expire by TTL.
- PASS/FAIL/UNKNOWN are independent of technical execution status. Code aggregates persisted evidence. Incomplete judge coverage is explicit. Imported adversarial strings remain data, never coordinator authority. No raw HTML/MDX or remote image rendering.
- Save preserves evidence and produces an editable canonical pack; it does not deploy, publish, establish a baseline or enable monitoring.

## Unit tests

Limits: oversized/chunked bodies, malformed UTF-8, nesting, duplicate keys, aliases, refs, array/string/case limits. Context includes system/history/artifacts/tool definitions. Graph fan-out and overflow fail before calling a provider. Role routing never falls back implicitly.

State transitions, cancellation/billing independence, deterministic unknown handling, immutable requirement acceptance, import coverage errors, price parsing, concurrent reservation invariants and synthetic trial economics.

## Integration / functional tests

Real isolated PostgreSQL: duplicate messages; two revisions/tabs; wallet row contention; two credit reservations; claim ownership; denied workspace IDs; permission revoked between queue and dispatch; duplicate grants/webhooks; durable event snapshots and attempt journal recovery.

Fake provider plus real service: invalid judge JSON; timeout; provider success then worker crash; late billing after Stop; unavailable evaluator; missing pricing/Redis; malicious imported instructions. No live API charges.

Temporal replay/duplicate activity must reuse persisted attempt outcomes or reconcile uncertainty, never invoke twice. Delayed outbox delivery must not create another operation.

## Smoke tests

Go package/build checks; frontend TypeScript, lint and focused unit tests. Start local UI and walk the first-use interaction using mocked HTTP responses. Public entry links to the new conversation route and uses AgentClash fonts/theme.

## E2E tests

Browser: describe app, receive a draft, select target model independently, check three cases, inspect evidence/unknown states, suggest improvement, accept revision, fair retest, refresh during run, reconnect with an old cursor, double-click send, cancel during a call and see reconciling billing. Unsafe Markdown renders inertly.

## Manual / cURL tests

The implementation README must include local startup, safe configuration, example session/message requests and isolated test commands. Model execution remains disabled until explicit hosted price and funding configuration is supplied.

## V1 exclusions

Arbitrary URLs/connectors and code execution; archives/PDF/image understanding; recursive/remote schemas; repetitions/consensus; autonomous production deployment; continuous production ingestion/notifications; automatic card recharge. Existing monitoring primitives remain reachable through the saved workspace, with no claim that a preview is production monitoring.
