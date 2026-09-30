# Vibe Evals V1 cleanup and fixes

## Product and preservation boundary

Keep the existing dark/minimal Build and Improve journeys. Both accept a desired behavior, run supported examples, explain real replies, support improvement against a fixed baseline, and preserve/export the work. Build accepts any task, not a list of hardcoded workflows. An interactive prototype runs supplied instructions here; business systems, live apps, OCR and network tools are not connected.

Casual chat never becomes a requirement. Material is factual input unless the user explicitly adopts a quoted rule. Text-based PDFs are limited to one per submission, 30 pages and 10 MB; extraction is isolated and bounded, with pasted text as recovery. Guest files expire after seven days; claiming before expiry preserves them. Project deletion removes stored content, unlike deleting a single file. Accounting and independently saved workspace packs remain.

Keep the progressive disclosure, compact conversation, usable output, checks and contact action. Extra runs require one bounded spending confirmation. Downloads do not require sign-in; workspace saving does. Contact reuses the reviewed summary and must not claim delivery without a configured recipient.

**Retire only pre-V1 sessions whose document format is not `1`.** Improve authoring version 15 is current V1 and must stay. Historical V1 results remain readable. Historical migrations remain.

## Implementation order

### 1. Reliable deletion

- Calculate quotes outside transactions. `Store.persistQuote` locks the active session, checks owner, current access and revision, then inserts both Build and run quotes.
- Filter cleanup eligibility before the batch limit; order by deletion time and ID; lock and recheck the parent before scrubbing.
- Preserve unresolved costs, reservations and sending leases. Blocked projects must not starve eligible ones.
- Acceptance: no late quote content after completed deletion; an eligible project completes behind twenty unresolved projects.
- Verification: isolated PostgreSQL race tests for late persistence, revision/ownership and fair cleanup, plus existing file/enquiry deletion coverage.

Status: implemented; focused isolated PostgreSQL race tests passed.

### 2. Frontend ownership

- Session controller owns active server snapshot, loading and stream. One reconciliation boundary rejects inactive IDs and older revision **or** cursor for GET, mutation, action and SSE responses.
- Draft controller owns guide/trial drafts by session, restores both together before persistence, preserves typing during loading, and retains version-1 storage.
- Navigation controller alone owns/writes pane, artifact, trial thread and run selection; restore on refresh and browser navigation.
- Delete competing caches, direct replacements and child URL/run synchronization. Workspace receives typed display data and explicit callbacks. Keep content-bound submission IDs, uncertain admission and late-acknowledgement edit guards.
- Acceptance: drafts survive switch/refresh; delayed responses cannot roll back progress; selecting result B survives refresh; production components stay below 1,000 lines.

Status: implemented. Three controllers own snapshot reconciliation, atomic drafts and URL selection. Browser regression preserves historical result B and the chosen pane on refresh; restoring a run no longer forces Results. The main client/workspace owners are 448/898/928 lines.

### 3. Retire old chats and authoring pipelines

- Inventory sessions by document format, operation states, pending outbox and Temporal execution. Drain retiring contracts; require zero pending execution before code removal.
- Add an idempotent administrative command, dry-run by default. Apply to the local pilot only after inventory/drain. Reuse project deletion through internal pre-V1 retirement, including containers; preserve accounting and canonical workspace packs.
- Remove archive UI/continuation and obsolete executors. Share candidate preparation, review, repair and completion in one executor.
- Explicit policies retain Improve 15, Build 18 without material support, and Build 20. One policy supplies stage names, repair/fallback edges, call counts and spending bounds.
- Preserve serialized requests/hashes/journals for retained contracts. New admissions use current policies; do not replay retired plans. Remove exclusively obsolete tests/config branches while retaining import, recorded-conversation, provenance and accounting coverage.
- Acceptance: repeatable retirement; one current executor; request identities and spending limits unchanged.

Status: implemented. Local inventory found 191 pre-V1 projects and 30 V1 projects, with zero pending retiring operations/outbox or running Vibe workflows. Retirement removed the old projects from use; 185 finished content cleanup, while six retain unresolved accounting/file-cleanup holds. All 30 V1 projects remain. Improve 15 and Build 18/20 request fingerprints match the original PR head. Archive/executor-only fixtures and the unused legacy API harness are removed. Every other deployment requires its own inventory/drain.

Operator command: `go -C backend run ./cmd/vibe-retire` inventories the database selected by `DATABASE_URL` and defaults to local Temporal/default namespace. Set `--temporal` and `--namespace` for the target deployment. Add `--apply` only after its inventory shows zero pending execution; repeating it is safe. Physical file cleanup stays with the input worker.

### 4. Authorization separate from presentation

- Use a small authorized-session lookup for file and enquiry endpoints. Full snapshots belong to conversation reads/streaming.
- Batch case summaries and retry metadata across operations; reuse loaded preparation data while rechecking write authorization under locks.
- Acceptance: file/download/delete reads stay constant with 1 versus 100 operations; snapshots stay bounded; revoked access fails.

Status: implemented. File and contact endpoints use `access.Lookup`; file deletion and enquiry creation repeat permissions under the parent lock. Snapshot case summaries and retry metadata use batched queries. HTTP regression verifies identical query counts with 1 and 100 operations, hides full evidence, and rejects revoked access inside writes.

### 5. Scoped locking

- Pass the five-second transaction context into every callback and database operation.
- Lock order: affected sessions sorted by ID → required capacity scopes sorted → operations → attempts → funding accounts sorted.
- Session locks protect project writes; operation locks protect completion/settlement; account locks protect grants/reservations. Serialize actor/workspace/hosted-pool/daily-count checks only within their required scopes.
- Reconciliation resolves the owner, locks in order and rechecks accounting. Remove the universal advisory lock only after all dependent invariants migrate.
- Acceptance: unrelated projects can edit/cancel independently; no double charge, capacity breach or premature release of uncertain holds.

Status: implemented. Project/operation/account locks replace the universal advisory lock; bounded contexts reach every transaction callback. Provider content and diagnostics recheck deletion under the parent lock. Race regressions cover independent edits/cancellation, exact settlement/grants, retained uncertain holds and cross-project capacity counting.

## Required verification and rollout

Use isolated migrated test databases and fake inference for the automated floor. Use `agent-run` for heavy checks.

- Deletion: pause/resume quote persistence across deletion, fair cleanup, repeated retirement and preserved accounting/files.
- Frontend: delayed GET/PATCH/action after newer SSE, switching during a request, unsent guide/trial drafts, blocked storage, typing during load, result selection and refresh/back/forward.
- Authoring: retained-contract stage keys, request hashes, journals and spend bounds through repairs, fallback, manual edits and interrupted recovery.
- Accounting: race reservation, completion, cancellation, deletion, grants and reconciliation. Assert exact balances, capacity limits and retained uncertain holds.
- Product: Build clear/vague/casual chat/text/PDF; Improve instructions, recorded conversations and imported tests. Keep expectations fixed during improvement comparisons.
- Browser: desktop 1440×900; mobile 390×844 and 360×800; loading, active context, scrolling, results/history, Stop, retry, export and contact.
- Run backend race tests, frontend tests, TypeScript and Playwright API/Temporal-worker journeys. Make the new regressions mandatory in CI; report failures explicitly.

Make each phase a separate reviewable commit after its regression floor passes. Retirement requires inventory and drain on **each deployment target**; local retirement does not authorize skipping another target's gate. Content cleanup is an explicit operation, not a migration. Completion requires all seven review findings covered, current journeys passing, obsolete code removed, and net deletion of duplication.

### Verification completed locally

- Full backend race suite (`vibe`, `api`, `enquiries`, `vibe/inputs`), Go build and vet passed. The mandatory floor passed all 117 required tests without missing/skipped regressions; CI enforces it.
- 210 frontend tests and TypeScript passed. CI includes controller tests and watches every `use-vibe-*` owner.
- Playwright: 74 mocked-browser checks, 14 real API/PostgreSQL/Temporal-worker journeys, and three material journeys passed. Two optional local-cookie smoke tests were skipped; real worker journeys exercised guest persistence and authenticated saving. Inference was fake, so these results establish application behavior, not model accuracy.
- Actual isolated PDF extraction, corrupted-PDF recovery, output/export/contact/deletion, Stop/retry, context isolation, drafts and fixed-baseline improvement passed. Screenshots and layout checks covered 1440×900, 390×844 and 360×800. Contact correctly reports unavailable delivery without configuration.
- Each phase is committed separately. Net removal exceeds 4,000 lines. No remaining test failures; other deployments still require retirement inventory/drain before rollout.
