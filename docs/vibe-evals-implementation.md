# Vibe Evals V1 cleanup and fixes

## Product and preservation boundary

Keep the existing dark/minimal Build and Improve journeys. Both accept a desired behavior, run supported examples, explain real replies, support improvement against a fixed baseline, and preserve/export the work. Build accepts any task, not a list of hardcoded workflows. An interactive prototype runs supplied instructions here; business systems, live apps, OCR and network tools are not connected.

Casual chat never becomes a requirement. Material is factual input unless the user explicitly adopts a quoted rule. Text-based PDFs are limited to one per submission, 30 pages and 10 MB; extraction is isolated and bounded, with pasted text as recovery. Guest files expire after seven days; claiming before expiry preserves them. Project deletion removes stored content, unlike deleting a single file. Accounting and independently saved workspace packs remain.

Keep the progressive disclosure, compact conversation, usable output, checks and contact action. Extra runs require one bounded spending confirmation. Downloads do not require sign-in; workspace saving does. Contact reuses the reviewed summary and must not claim delivery without a configured recipient.

**Retire only pre-V1 sessions whose document format is not `1`.** Improve authoring version 15 is current V1 and must stay. Historical V1 results remain readable. Historical migrations remain.

## Review-fix implementation (PR #1278)

Keep the current UI and journeys. No new framework, historical-result rewrite, or replay of paid calls. Existing cleanup, session/draft/navigation ownership and scoped locking remain the foundation.

| Phase | Change and acceptance | Status |
| --- | --- | --- |
| 1 | CI uses its migrated database directly; mandatory fake-inference regressions run from tracked files. | Complete: 117 required tests passed |
| 2 | One artifact execution-context owner; regeneration preserves references. New material Build/manual review uses contract 21; retained 15/18/20 identities remain unchanged. | Passed: regeneration, material manual review and frozen-wire regression tests |
| 3 | Current session owner authorizes claim/continuation/retry/regrade. Temporary admission failures back off without replay; deleted Builds finalize accounting without new content. | Passed: signup during inline preparation, backoff/single continuation, claimed retry/regrade, deletion finalization with uncertain holds |
| 4 | Provider cost evidence does not finish a reply. Receipt/response orderings preserve output, settle once and quarantine conflicting costs. | Passed: both callback orders, concurrent duplicates/conflicts, Stop and accounting/deletion races |
| 5 | One immutable-request recovery owner; explicit proven rejection unlocks editing. Attachment selection is atomic across upload/attach/restore/polling. | Pending |
| 6 | PDF jobs/downloads/completion are bounded to 50/25/5 seconds; heartbeat/cleanup run independently. Expiry is independent of four-way, bounded provider reconciliation. | Pending |

### Verification

Add desired-behavior regressions with each fix and require them in the committed baseline; missing/skipped required tests fail. Use isolated migrated PostgreSQL, fake inference and fake email transport. Run heavy checks through `agent-run`.

- Context: PDF/text references through regeneration and manual review; target and judge share the factual corpus; trial data/answer keys never become instructions.
- Ownership/lifecycle: signup during preparation/inline material/continuation; retry/regrade after claim; former guest denied; transient gates recover once; deletion finalization repeats with uncertain holds.
- Accounting: both receipt/response orders, duplicates/conflicts, cancellation/deletion, exact balances and concurrent settlement.
- Frontend: lost acknowledgement then 403 retains the request ID; rejected stale material is editable; attachment races/restore/polling preserve newer selections, drafts and active contexts.
- Workers: stalled blob/provider requests, lease takeover, maintenance deadlines and shutdown; no uncertain paid work released by TTL.
- Product/browser: Build clear/vague/casual/text/PDF and Improve instructions/conversations/imports; fixed baselines, loading, Stop/retry, historical result refresh, export/contact. Desktop 1440×900; mobile 390×844 and 360×800.

Record actual results below. Earlier local verification predates these fixes and is not proof of clean-checkout CI or current completion.

### Verified results

Pending this implementation. Tests use fake inference and establish application behavior, not model accuracy. No real enquiries are sent.

### Rollout

Deploy rejection metadata before the frontend recovery change. Missing metadata stays uncertain. Apply the additive accounting migration, then coordinate worker replacement after active paid execution drains; never overlap old reconciliation with new attempt handling. Preserve V1 projects, original funding identities/holds and independent saved packs. Lost historical references require explicit reattachment; lost replies cannot be invented.

Pre-V1 retirement remains an explicit per-deployment inventory/drain operation, not a migration: `go -C backend run ./cmd/vibe-retire` is dry-run by default; `--apply` requires zero pending retiring execution. The previous local run retired 191 pre-V1 projects, preserved 30 V1 projects, and left six cleanup holds requiring accounting/file resolution.

Completion requires all twelve review findings covered (including two conditional timeout risks), passing clean-checkout-equivalent CI commands and current journeys, and separate reviewable phase commits. Report remaining failures and deployment drain/configuration requirements explicitly.
