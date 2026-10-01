# Vibe Evals V1 cleanup and fixes

## Product and preservation boundary

Keep the existing dark/minimal Build and Improve journeys. Both accept a desired behavior, run supported examples, explain real replies, support improvement against a fixed baseline, and preserve/export the work. Build accepts any task, not a list of hardcoded workflows. An interactive prototype runs supplied instructions here; business systems, live apps, OCR and network tools are not connected.

Casual chat never becomes a requirement. Material is factual input unless the user explicitly adopts a quoted rule. Text-based PDFs are limited to one per submission, 30 pages and 10 MB; extraction is isolated and bounded, with pasted text as recovery. Guest files expire after seven days; claiming before expiry preserves them. Project deletion removes stored content, unlike deleting a single file. Accounting and independently saved workspace packs remain.

Keep the progressive disclosure, compact conversation, usable output, checks and contact action. Extra runs require one bounded spending confirmation. Downloads do not require sign-in; workspace saving does. Contact reuses the reviewed summary and must not claim delivery without a configured recipient.

**Retire only pre-V1 sessions whose document format is not `1`.** Improve authoring version 15 is current V1 and must stay. Historical V1 results remain readable. Historical migrations remain.

## Review-fix foundation (PR #1278)

Preserve retained authoring contracts **15/18/20/21**, frozen requests/stage keys, spending limits and historical results. No historical-result rewrite or replay of paid calls. Keep the existing session/draft/navigation owners and scoped database locks.

| Area | Implemented responsibility and required protection |
| --- | --- |
| CI | Tracked runner uses the provisioned database directly; missing/skipped required regressions fail. |
| Artifact context | One owner preserves instructions, reference bindings, input contract, capabilities and scope through regeneration. Material-enabled Build/manual review uses 21; previously admitted 15/18/20 retain their identities. |
| Ownership/continuation | Current session owner authorizes claim, inline material, retry and regrade. Temporary admission failures wait for their retry deadline, then create one deterministic child. Deletion ends content updates while unresolved costs remain held. |
| Accounting | Receipt evidence never completes an executing reply. Both arrival orders preserve output and settle once; conflicting costs freeze affected funding and preserve evidence. |
| Request recovery | Immutable body/ID survives uncertain acknowledgement and later access failures. Proven rejection unlocks editing; upload/attach/restore/polling share atomic material selection. |
| Workers | PDF job/download/completion bounds are 50/25/5 seconds, with independent heartbeat/cleanup. Real queue maintenance runs separately from bounded, rotating, four-way receipt polling. |

## Test-audit cleanup

Three reviewable commits on `codex/vibe-audit-fixes`; keep PR #1278 and its visual design. No new frameworks, migrations, public APIs or test-only production hooks.

1. **Remove noise/dead paths.** Move full-policy rejection and the shared proposal fixture to retained 15; remove retired 12 replay success, 16 assertion tails and their unreachable decoder arms/constants. Delete the direct snapshot-helper test/export, random price simulation, unused reconciliation lookup and PDF wrapper. Keep actual protected-budget coverage. Reduce reviewer parsing to supported/veto cases with repair evidence; analytics asserts its envelope/deduplication, without claiming privacy from empty fixtures. Frozen prompt text remains untouched.
2. **Prove real owner behavior.** Split polling/rotation/hold coverage from a test invoking the actual `ReconcileLoop`, using the production minute interval, a 90-second bound and fake transport. Hold the queued project's row until receipt polling starts, then require expiry before polling finishes and verify shutdown/uncertain holds. Extend admission recovery to call both continuation entry points before backdating. Test historical instruction selection in the mounted workspace; key its `PromptChange` by artifact ID, matching V1. Keep exact clipboard-content coverage separately.
3. **Require and document proof.** Baseline requires the protected-allocation assertion, renamed reviewer/polling tests and actual-loop regression. CI selects the renamed economics test and Build analytics. Browser evidence waits for persisted completion and settled dialog bounds rather than early card visibility or a resize transition. Run focused checks, full suites, clean committed archive and browser journeys before pushing. Exclude unrelated work. The subsequent merge-verification pass integrates `main` under the required release checks.

### Demonstrated regression failures

- Historical workspace: copy v2, select v3 in the same session. Before the key fix, the test failed because v2's copied feedback remained. Afterward, feedback resets and copying uses exact v3 instructions.
- Worker scheduling: removing the maintenance launch **or** serializing expiry behind receipt polling makes the actual-loop test fail with queued work still reserved. Production scheduling passes while the receipt request remains blocked. The fixture does not launch maintenance itself.
- Early continuation: removing `AdvanceBuild`'s deadline guard fails budget-guard, capacity and rate-limit cases before a retry is due. Production preserves provider-call/reservation counts and then admits exactly one child after the deadline.

## Verification recorded on 2026-10-01

Through `agent-run`, with isolated migrated PostgreSQL, fake inference and fake email transport:

- **139 mandatory regressions passed**, none missing/skipped, from committed code `a589d5e7` archived without the ignored database wrapper or local secrets. Database provisioning is external; the tracked runner executes directly. **20 Python tooling tests** passed.
- Full backend race suites passed for Vibe, API, enquiries, materials and interaction. Backend build/vet and shared provider/scoring/challenge-pack tests passed. Actual isolated PDF extraction/security tests ran with the pinned reader. A temporary-directory quota failure was resolved by using disk-backed test storage.
- **220 frontend tests** passed; TypeScript passed. Retained wire-identity and admitted v15 edit/undo checks passed. Broader testing found and repaired the shared proposal fixture's remaining retired-13 assumption.
- **74 mocked browser scenarios** and **17 real API/PostgreSQL/Temporal journeys** passed. Two opt-in local cookie-smoke tests were skipped in the mocked suite; no required baseline tests were skipped. Coverage includes Build/Improve, casual chat isolation, PDF/text and extraction failure, Stop/retry, context/draft/version selection, fixed-baseline improvements, exports and contact recovery. No real enquiry was sent.
- Desktop **1440×900** and mobile **390×844/360×800** were checked; screenshots reviewed for results, composer/context visibility and contact. Browser assertions now await execution completion and dialog reflow before reading/capturing them.

The previous expiry test started maintenance itself and did not prove worker launch/shutdown. The new actual-loop test and its two failing controls replace that overstated claim. Reviewer parsing tests do not establish semantic model accuracy; synthetic economics is not a spend benchmark. No paid benchmark ran. Safari and a physical iOS keyboard remain unverified.

### Integration and merge verification

Integrated `main` at `3ab86fdf` in an isolated worktree, preserving the original checkout and unrelated edits. Fixed integration issues found by hosted CI: retirement now uses validated Temporal connection configuration; the migrator accepts balanced statement markers in unchanged historical SQL; the browser job compiles its API fixture before starting the readiness deadline. URL selection now has one external-store owner instead of mirrored component state. Offline draft quoting waits for configuration; the browser fixture distinguishes quotes from execution and rejects page errors.

After these changes, local proof includes **849 frontend tests passed** (three pre-existing backend-dependent smoke tests skipped), TypeScript, ESLint (zero errors, eight warnings), all Vibe backend race suites, backend build/vet, shared runtime tests, migrator unit tests and its isolated-database rehearsal. **74 mocked browser scenarios plus 17 actual API/database/worker journeys passed**; two optional cookie-smoke cases remain skipped. Desktop and phone entry, loading, results and contact screenshots were inspected. No paid inference or real email was used.

GitHub-hosted CI is a separate merge gate: push the fixes, require passing release and Vibe safety checks on that exact revision, then merge PR #1278. Never bypass branch protection. The image rehearsal runs the actual application migrator, so its initial failure must also be rechecked with the parser correction. The integrated clean archive passed all **139 mandatory regressions** with no skips. Hosted safety then exposed fixture contamination when the deletion test ran twice against one database: its ordinary cleanup helper released intentionally unresolved synthetic operations. The test now retains those holds and scopes its assertions to its own projects; it failed before the fixture correction and passed three repetitions afterward. Public package-name progress identified a retired Alpine OpenSSL download in the hosted image job. Updated all six SSL/crypto pins for Alpine 3.22/3.23/3.24 to published **3.5.9-r0**; bytes match the official CDN and kernel mirror, package metadata matches the declared version/architecture, and offline `apk verify` accepts upstream signatures. All **76 delivery tests** pass. Image scanning/rehearsal and the required release gate remain mandatory on the updated pins.

## Rollout and deferred work

Deploy rejection metadata before frontend recovery; absent metadata remains uncertain. Existing additive migration **00086** supplies reconciliation evidence. Replace workers only after active paid execution drains; do not mix old/new reconciliation behavior. Preserve funding identities, uncertain holds and independently saved packs. Missing historical references need explicit reattachment; missing replies cannot be invented.

Pre-V1 retirement is an explicit inventory/drain operation per deployment, not a migration: `go -C backend run ./cmd/vibe-retire` is dry-run by default (set `APP_ENV=development` for local plaintext Temporal; other environments require validated TLS configuration); `--apply` requires zero pending retiring execution. The previous local pilot retired 191 pre-V1 projects, preserved 30 V1 projects and left six cleanup holds needing accounting/file resolution. This test cleanup does not resolve those holds.
