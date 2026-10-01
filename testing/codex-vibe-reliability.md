# Vibe reliability regressions (Ubuntu continuation)

## Scope and acceptance contract

Continue `codex/vibe-evals` after `281f1f9c6d19668130ed6a617de5a2893d3bf2cb` without replacing the merged blueprint/composition/compiler. Preserve immutable evaluation contracts, integer accounting, no provider retries, fixed capability bounds, and Geist/builder theme/ClashMark.

Required regressions:

- Source-backed expectations resolve their source, not serialized `null`; explicitly supplied values retain precedence.
- Withhold only an expected-answer leaf (including supported array paths), preserve sibling instructions and original evidence, reject unresolved/unsafe paths explicitly.
- Reject unsupported metrics, derived/behavioral dimensions and post-execution checks. Never remove coverage to make an import fit Vibe. Preserve supported legacy `correctness` dimensions that the shared scorer normalizes to validators.
- Store save-time model receipts in Vibe-owned immutable records. Editing arbitrary canonical builder `model_spec` JSON must neither forge an old save receipt nor break session hydration.
- Distinguish definite initial pre-admission rejection from an ambiguous response. Permit correcting an initially rejected profile; after any earlier ambiguous send, later auth/rate/profile rejection must not release the original request identity.
- Bound the resolved fuzzy expected operand before inference and target output before scoring; unrelated metadata is not an operand.
- Reconciliation of an over-ceiling attempt freezes every backing account, even if total operation cost still fits the larger reservation. Other model routes cannot spend frozen funds.
- A pending approval locks requirement edits just as a running operation does.
- Initial authoring and its sole bounded repair both include the actual accepted agent even when the latest proposal is unrelated. Its evaluation remains pinned.
- An uncertain browser POST retry retains the entire original payload, not just its client ID. SSE/revision/model updates must not change that retry or create another operation.
- Existing-session hydration gates submission; failed hydration must not silently create a fresh conversation.
- Unsaved instruction edits block execution, playground and saving the old artifact.
- Open evidence refreshes when persisted case data changes, without requiring a collapse/reopen. Redacted case summaries are insufficient: pass the persisted session event cursor so output-only crash recovery also invalidates the cache, including after cancellation without a state/verdict change. Identical snapshots must not refetch, and refreshing must not submit inference.
- Requirement-only replies remain confirmable/dismissible without a draft.
- Acknowledging a POST does not erase a newer message typed while it was pending.
- Saving accepts and validates explicit model choices, stores the target/evaluator in the canonical builder without inference, and returns revision-scoped save receipts on reconnect. A repeat with changed/unrecorded models fails explicitly instead of returning a misleading prior save. Legacy requests without model choices retain idempotency.

## Verification categories

1. Tight RED→GREEN unit/database/component regressions for the defects above.
2. CI-selected Go/PostgreSQL race tests, including historical tryout promotion and plain-English pack generation; backend build/vet; shared provider/scoring tests.
3. Vibe TypeScript, component tests (including safe Markdown), focused lint, and mocked Playwright journeys. CI now runs all Vibe component tests, not only Markdown.
4. Real PostgreSQL + fake provider onboarding fixtures for casual chat, sufficient brief, the reported five-turn sparse conversation, delegated defaults, existing-agent description, and imported pack. These verify exact input transport, compiler/persistence, explicit acceptance and absence of fabricated scorecards; their replies are authored fixtures, **not measured model quality**.
5. The existing PostgreSQL journey exercises accept → technical UNKNOWN evidence → coaching/improvement → fair retest → canonical save using fixture authentication/provider responses. Mocked browser journeys cover UI interactions separately. Neither establishes real WorkOS sign-in.

## Live boundary

No production deployment, paid model, top-up, automatic retry, trial reset or identity rotation. A new localhost DB is not a new provider allowance. Obtain the previous pilot ledger/remaining allowance before using a key. The two historical unresolved attempts remain unresolved; this checkout has not reconciled or replayed them.

On September 9, 2026, the public OpenRouter endpoint API still advertised `dots-studio/dots-3-note-preview:free`, `atlas-cloud/fp8`, zero pricing and structured-output support. This is metadata only, not accounting conformance or a renewed usable profile. No key was configured during offline validation; inference was left disabled with an empty model catalog. Actual clarification loops, hallucinated business claims, live schema failure rate, latency and provider accounting remain unmeasured in this continuation.

Ubuntu setup and verified command results are recorded in [vibe-evals-ubuntu.md](../docs/vibe-evals-ubuntu.md).
