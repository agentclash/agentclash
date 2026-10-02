# Vibe Evals: product integration follow-up PR

## Scope and preservation

**Product integration implemented for review; PDF/release verification remains blocked.** Isolated worktree from fresh `origin/main`, branch `codex/vibe-product-integration`, title `fix(vibe): complete product entry, auth and deployment integration`. PR #1278 is merged (`b56ec7d8`); its old draft/conflict/merge instructions are obsolete. Audit baseline: main `b839fa18`.

Keep the current Build/Improve design, progressive disclosure, session/draft/navigation owners and scoped database locks. Build accepts any task; prototypes execute only supported supplied-text/document behavior. Preserve casual-chat isolation, sample labels, fixed expectations, unsupported-capability explanations, spending limits, immutable request IDs and financial holds. Retain authoring contracts **15/18/20/21**, frozen requests/stage keys and historical results. No new UI framework, authoring pipeline, OCR or live integration tools.

Downloads remain available to guests; workspace saving requires sign-in and permission. PDFs remain text-based, at most one per submission, 30 pages and 10 MB; no OCR. Extra runs retain their bounded spending confirmation. Contact reuses the reviewed summary without attaching raw files/history or claiming unconfigured delivery. Guest files expire after seven days; claim before expiry preserves them. File deletion and project deletion remain distinct; accounting and independently saved packs survive content deletion. Retirement still applies only to pre-V1 document formats, never authoring version alone.

Prior #1278 verification is historical evidence for that revision, not proof of this PR or production. Follow-up verification now reproduces and repairs disabled-feature creation, lost import context, failed callback continuation and Redis admission denial. Current public probes from this development host: Vibe HTML 200, API `/healthz`, `/readyz` and `/v1/vibe/config` 403. This does not identify an upstream outage cause; production logs/deployed state remain a rollout prerequisite.

## Complete journey and ownership

Homepage Vibe CTA -> guest Build/Improve -> supported prototype/output/checks -> fresh or previous trial -> sign-in only when saving -> claim original work -> workspace setup if needed -> exact save dialog -> explicit Save. Downloads/imports preserve scope; missing references require explicit attachment before execution. Useful output/results lead to the existing reviewed contact action. Preserve the developer/CLI and `/tryouts` journeys as distinct entry intents.

| Responsibility | Canonical owner |
| --- | --- |
| Entry/navigation | Shared marketing navigation data/presentation; homepage retains tracking. Vibe shell provides Home/dashboard/account actions. |
| Identity/return | AuthKit and `lib/auth/return-to.ts`; token failure never becomes guest identity. No second auth store. |
| Save/setup | `use-vibe-saving.ts` owns exact artifact/baseline resume; dashboard/onboarding carry sanitized intent only. Writes recheck membership/ownership. |
| Conversation | `use-vibe-session.ts` owns HTTP/SSE snapshots, `use-vibe-drafts.ts` buffers, `use-vibe-navigation.ts` URL selection. Trial controls use existing actions. |
| Capabilities | Existing config endpoint, backend admission and real worker evidence. UI reflects availability, never grants permission. |
| Portability | Browser `vibe-export.ts` builds payloads; focused backend `portable.go` adapts formats; `artifact_execution.go` owns execution context/readiness; inputs repository resolves authorized files. |
| Runtime/release | Existing worker coordinator manages Vibe start/stop/drain. Delivery owns actual images, ACLs, shared configuration and promotion readiness. |

## Phases, acceptance and verification

Commits follow ownership boundaries in the same follow-up PR: admission/configuration, worker/release, backend portability, frontend integration, and CI/evidence. Frontend phases 3–5 share callbacks and imports and land together so intermediate commits remain buildable. Reproduce each primary defect before its owner fix, then pass focused checks. Exclude unrelated working-tree changes; do not reuse the old audit branch as the new PR base.

### 0. Baseline and operational diagnosis

Fetch fresh main into an isolated worktree; recheck findings and record deployed frontend/API/worker revisions. Privately correlate DNS/TLS/edge, Caddy upstream errors, direct liveness/readiness, startup logs, exits/OOM/restarts, resource pressure and dependencies. `/healthz` is dependency-free: Redis admission denial does not explain its failure by itself.

**Acceptance:** identify an evidence-backed operational cause or retain an explicit rollout blocker. Record any recovery separately from code changes; do not restart production, rewrite secrets or reset caches during discovery. Independent implementation can proceed while availability remains unresolved.

### 1. Admission, capabilities and configuration (P1/P2)

**Files:** `backend/internal/vibe/{gateway.go,models.go}`, `backend/internal/api/vibe.go`, `deploy/aws/scripts/{render.py,secret_store.py}`, `backend/.env.example`, runbooks; `web/src/lib/{vibe.ts,use-vibe-workspace.ts}`, `app/vibe-evals/vibe-client.tsx`.

- Add only `~vibe:rate:*` to API/worker Redis ACLs. Preserve terminal isolation, unrelated-key restrictions and administrative denials; no live-key rename or wildcard widening.
- Add `material_build` to the existing config response/type. Keep `pdf_uploads` as functioning-reader evidence. Material enabled/PDF unavailable still accepts pasted text.
- Separate execution availability from reading/loading/navigation. `enabled:false` prevents creation, quotes and inference, without blanket-blocking authorized history/download/delete/save. Enforce execution/capability rules at backend admission as well as UI dispatch. Gate guide/trial materials; restored incompatible attachments require explicit recovery, never silent detachment. Preserve admitted contracts.
- Validate API/worker shared feature/model/budget configuration together before release staging. Enabled execution requires usable defaults/credentials/funding. Cookie signing remains API-only; parser runtime worker-only. Diagnostics never reveal secrets.
- Replace obsolete interpreted-authoring/JEV/two-door switches and stale version guidance with current configuration and disabled-mode behavior.

**Proof/acceptance:** actual `Gate.Check` against generated API/worker TLS Valkey ACLs admits, rate-limits and sets bounded TTL; forbidden keys/flush and terminal Vibe access stay denied. Mounted client cases assert zero disabled creation/quote/inference requests, continued authorized history/save/delete, restored-selection rejection and text-on/PDF-off. Deployment validation rejects incomplete/mismatched service configurations.

### 2. Images, PDF isolation and managed workers (P1/P2)

**Files:** `deploy/aws/Dockerfile.backend{,.dockerignore}`, `delivery/{build.py,maintained.py,image_scan.py,health.py}`, affected platform locks/scanners, `deploy/aws/compose.yaml`, `backend/cmd/worker/main.go`, `backend/internal/vibe/temporal.go`, `backend/internal/worker/{service.go,activity_drain.go}`, drain inventories/runbooks.

- Explicitly retain embedded `inputs/parser.py` in the AWS build context and the requirements lock where consumed. A clean baseline AWS build succeeded: the earlier predicted embed compile failure was not reproduced. The genuine deployment gap was the absent worker-only reader runtime.
- Package the parser through the **actual AWS worker image graph**, not only the optional development Dockerfile. Prefer a worker-specific maintained Alpine runtime if compatible; pin/hash OS/Python/native dependencies and preserve SBOM/licensing and secret/HIGH/CRITICAL scan gates. Require scanner coverage of the installed Python and native dependency graph: today's Alpine/Go inventories alone do not establish it. API/migrator need no parser runtime.
- Prove self-check/extraction inside the exact image with production UID, mounts, tmpfs, capabilities, profiles and target kernel. Preserve no-new-privileges, dropped capabilities, private parser filesystem/network, empty environment and current resource limits. Never use privileged containers, host network/mounts or unconfined profiles to pass.
- Construct Vibe with configuration-derived concurrency/identity/grace and the existing activity drain. Manage it in the common coordinator; stop queue workers together and join outbox/materials/cleanup/enquiry/reconciliation work before closing clients. Keep paid-call no-retry and uncertain holds.
- Require fresh Vibe workflow/activity pollers at startup/promotion when enabled. Expected capabilities come from pinned release configuration: a PDF-required release fails promotion on missing self-check/heartbeat.
- Drain inventories include Vibe operations/outbox/continuation, attempts/holds, input leases and cleanup; Temporal workflow counts alone miss database work.

**Proof/acceptance:** clean Git export builds the real AWS API/worker/migrator images, including a no-cache affected-stage regression. Image rehearsal proves extraction/isolation and truthful fallback. Actual production-constructed Temporal workers respect configured slots, finish during grace or cancel/finalize safely, handle partial startup failure and never repeat uncertain paid calls. Missing/stale Vibe pollers block enabled promotion; disabled execution keeps maintenance alive.

### 3. Discovery, identity and exact save continuation (P1/P2)

**Files:** `web/src/app/landing.tsx`, shared marketing/nav components, `components/vibe/evaluation-navigation.tsx`, `app/vibe-evals/vibe-client.tsx`, `lib/use-vibe-{workspace,session,saving}.ts`, `lib/vibe-keep.ts`, `lib/auth/return-to.ts`, `app/auth/{login/actions.ts,callback/route.ts}`, `app/dashboard/page.tsx`, `app/onboard/{page.tsx,onboarding-wizard.tsx}`.

- Share marketing nav data/presentation with usable mobile access; never import async server `MarketingHeader` into client landing. Add a feature-gated Vibe CTA with preserved auth destination; retain developer/billing/invite/device/GitHub destinations and distinct tryout copy.
- Add Home/dashboard and existing account-menu behavior to Vibe. Guest sign-in keeps selection. Signed-in Home may retain the existing dashboard redirect; do not route all accounts to Vibe.
- Return absent credentials only for a settled guest. Auth loading/known-user token failure blocks dispatch with recovery. Move token retrieval inside GET/SSE/workspace-loading error handling and audit async callers for unhandled rejection. Keep drafts and pending subject/body/ID; never replay uncertain work under another account.
- Reuse `rememberSave`/`keepReturnURL`; carry sanitized exact artifact/baseline intent through setup, including early redirects, retry and already-onboarded paths. Reopen the dialog and require explicit Save. No automatic new organization for users without create/write permission; provide access guidance/downloads.
- AuthKit's error callback lacks verified return state. Use its public authorization URL API to store a bounded signed, server-written HttpOnly intent per auth attempt: sanitized destination, mode, expiry and correlation to SDK-generated state. Ten-minute expiry, production Secure, SameSite=Lax. Recover only the matching valid intent on the trusted callback origin and clear it. Concurrent attempts stay separate; invalid/expired intent falls back safely. No unchecked state decoding or SDK private imports.

**Proof/acceptance:** rendered desktop/mobile links reach Vibe and back. Signed-in token failure produces no guest requests, unhandled promises or lost drafts; retry retains request identity. Extend the existing local AuthKit/API stack with an unonboarded user and real auth-session/onboarding handlers: guest -> login -> setup -> exact dialog -> explicit persistence, with unchanged inference count. Inject callback failure/retry, concurrent/invalid intents, cancelled login, access denial and existing-workspace recovery. Do not fabricate workspace/save receipts.

### 4. Portable definitions with explicit missing references (P2)

**Files:** `web/src/lib/{vibe.ts,vibe-export.ts}`, `components/vibe/{agent-settings.tsx,evaluation-outcome.tsx}`, import/input presentation; `backend/internal/vibe/{portable.go,service.go,types.go,artifact_execution.go,runner.go,build_cycle.go}`, `backend/internal/api/vibe.go`, saved-source presentation.

- Introduce `agentclash-vibe-v2` as a selected definition: unchanged prompt/blueprint, title, input contract, capabilities, scope, sample designation and bounded model preferences. Reference manifest uses export-local keys and existing hash/usage/name/format/page metadata. No source IDs/URLs, raw files/extracted text, historical evidence or permission grants.
- Consolidate definition payloads in `vibe-export.ts`; remove competing `exportAgent` from `vibe.ts` and update callers. Label historical conversation/results archives separately; incomplete evidence export remains an error.
- Backend format adapters retain legacy formats and JSON/YAML packs. Rich legacy exports preserve available context and become unresolved references, not copied private IDs. Plain legacy exports cannot recover absent metadata. Import creates fresh identity, no operations/scores/policy grants and no active model override.
- Add unresolved requirements to artifact JSON. Preserve imported checks; do not regenerate or claim a runnable target. Canonical readiness rejects quotes/admission/execution before spending until references are bound.
- Reuse input upload/paste/download APIs. Add one revision-bound binding mutation through the authorized edit transaction: resolve a ready destination-session input, verify ownership/hash/page acknowledgement, create a new artifact version, return via snapshot reconciliation. Stale/foreign/deleted/expired/wrong-hash inputs reject atomically.
- Matching reattachment preserves the definition. Different material requires an explicit changed-reference version/check decision, not a same-baseline improvement. Expected answers/initial trial data never enter target instructions.
- Source downloads remain deliberate/file-specific. Saved work retains immutable Vibe source/baseline receipts and links back to its material-backed target. Packs remain tests: allow valid test-only publication, but do not claim an advanced target can execute dependencies it cannot bind. No expansion of deployment capabilities.

**Proof/acceptance:** definition download -> real import preserves context/fixed expectations; missing dependencies cause zero reservations/calls. Matching authorized attachment grounds target/judge without answer/trial leakage. Foreign/mismatched/expired binding fails atomically. Legacy packs retain coverage without invented results. Default download omits private IDs, raw materials/unrelated artifacts. Original saved source remains accessible.

### 5. Build trial controls and route consistency (P2/P3)

**Files:** `web/src/components/vibe/{prototype-trial.tsx,build-conversation.tsx,build-next-actions.tsx}`, existing workspace/navigation owners, `lib/public-http.ts`, middleware tests.

Expose compact accessible New conversation/history in the Build dock, calling existing `newTrial`/`onThread`. Preserve thread-specific buffers, model/artifact/URL and follow-ups; no second store or silent reset. Remove Vibe's false Markdown negotiation/alternate while keeping HTML AuthKit/noindex and private-content exclusion.

**Proof/acceptance:** actual Build dock creates independent threads, returns to earlier history and refreshes selected model/draft. Middleware GET/HEAD with Markdown negotiation enabled serves the HTML application without rewrite/alternate; no public private-session adapter is added.

### 6. Mandatory complete-story verification and CI

**Files:** existing backend/deployment test owners, `web/e2e/vibe{,-stack}` fixtures/specs, `scripts/vibe/run_baseline.py`, `.github/workflows/{vibe-evals.yml,vibe-contract-baseline.yml,aws-checks.yml}`, `delivery/{changes.py,checks.py}` as needed, this document.

- Use one primary regression per owning boundary. Record invariant, credible failure, coverage gap and red-before/green-after evidence. No copied inventories/source greps, mocks supplying the asserted outcome, or test-only production hooks.
- Extend Vibe CI triggers to marketing/auth/dashboard/onboarding/nav, deployment/build inputs and portability; include auth/export/onboarding tests and new real-stack specs in explicit selections. Keep required-test skip/missing failures and dependency-aware AWS gates.
- Reuse isolated databases, fake inference/email, local AuthKit, generated Redis permissions and actual Temporal/image rehearsals. Never exercise production projects, paid models or real enquiry delivery for automation.
- Through `agent-run` where practical: focused regressions, relevant backend race suites/build/vet, runtime provider/scoring/challenge-pack suites, frontend tests/typecheck/lint/build, Python delivery checks, mandatory baseline and actual image scans/rehearsals. Final proof uses a clean committed checkout, not ignored local wrappers.
- Browser: **1440x900**, **390x844**, **360x800**. Cover landing -> Build/Improve -> output/checks -> trial/history -> login/setup/save -> download/import/reattach -> reviewed contact. Include loading/API failure, disabled/re-enabled flags, vague input, casual chat, pasted text, valid/failed PDF, unsupported capabilities, Stop/retry, refresh/context switching, historical selection and fixed-baseline improvement. Check focus/keyboard, margins/overflow, context label, scrolling and screenshots after settled layout/persisted completion. Zero unexpected page errors or duplicate/charged work from auth/navigation.

Primary regression routing (extend existing owners, not duplicate the same proof):

| Contract | Primary test/rehearsal owner and required observation |
| --- | --- |
| Deployed permissions/build/PDF | `deploy/aws/tests/rehearse.py`, image lane in `delivery/checks.py`, existing isolated reader tests. Real role credentials admit rate checks; actual allowlisted build succeeds; candidate container extracts and denies secret/network access. |
| Worker lifecycle/readiness | Production SDK/worker harness and `delivery/tests/test_release.py`. Block dependency calls, observe configured concurrency, signal shutdown, verify persisted finalization/holds before clients close; stop only Vibe pollers and require promotion refusal. |
| Config/token recovery | `web/src/app/vibe-evals/vibe-client.test.tsx` and mounted session-controller tests. Assert network/identity/draft outcomes, not booleans alone; fix guest fixtures to use `user:null`. |
| Landing/auth/setup/save | Existing `web/e2e/vibe-stack` AuthKit fixture plus real auth-session/onboarding handlers. Persist exact artifact/baseline after explicit Save; login/setup cause no provider calls. Existing return-to unit tests independently protect redirect sanitization. |
| Portable materials | Extend `TestVibeIntegrationTwoDoorImportsPreserveEvidence`, material execution fixture and `web/src/lib/vibe-export.test.ts`. Observe imported metadata, blocked spending, bound provider requests and actual download bytes. |
| Build trial/history | Extend existing conversation/navigation Playwright cases using the Build dock; assert independent messages and restored selected thread on refresh. |
| Private route | `web/src/middleware.authkit.test.ts`: actual GET/HEAD response retains HTML/AuthKit and excludes Markdown alternate/rewrite. |

Final command groups (run in an isolated clean checkout with fixture services, through the existing launcher): backend `go test -race ./internal/vibe/... ./internal/api ./internal/worker ./internal/enquiries -count=1`, `go build ./...`, `go vet ./...`; runtime provider/scoring/challenge-pack tests; web `npm test`, `npx tsc --noEmit --incremental false`, lint/build, both Playwright configs; `python3 scripts/vibe/run_baseline.py --report <artifact>` and selected `python3 delivery/checks.py <group>` including images. Record exact selections and evidence, rather than promising a test count.

## Verification record (this revision)

- Relevant backend race suites (`vibe/...`, API, worker, enquiries), backend build/vet and shared provider/scoring/challenge-pack race suites pass against isolated PostgreSQL and fake inference. Optional reader tests in the broad host suite were not exercised; see the exact-image failure below.
- Persisted admission/import/binding regressions pass: disabled creation with historical read/delete, preserved legacy context, missing/unsupported references with no quotes, existing foreign-session/wrong-hash/expired rejection, fixed-blueprint binding and actual grounded target/judge requests. Existing admitted material work drains after a new-admission flag change.
- Actual generated TLS Valkey ACLs were red on the baseline and green with the narrow namespace change: rate admission/TTL/limits and forbidden key/admin/terminal denials. The production worker factory passed a disposable PostgreSQL/Temporal rehearsal from clean commit `8c667103`: configured identity/slots, observed execution during grace, cancellation at its expiry, replacement finalization, uncertain holds and all nine SQL drain fixtures. Setting SDK stop grace to zero made both intended assertions fail. Delivery checks (79) and AWS guards (17) pass; mocked readiness decisions alone are not SDK or image execution proof.
- Real AuthKit/API/PostgreSQL/Temporal: normal Build/Improve matrix passes all 15 cases, including casual-chat isolation, sample continuation, Stop/retry, fixed-test improvement, independent trial/history and cancelled-login recovery -> real onboarding -> exact explicit save, with unchanged inference count and no page errors. Authored-reference upload/import/bind/download/reimport/reattachment also passes, without private IDs, raw files or fabricated results. Desktop 1440×900 and mobile 390×844/360×800 screenshots are local artifacts. An initial combined material-enabled run failed the normal sample fixture because it selects a different retained authoring contract; rerunning the intended separate matrix passed without changing that fixture. The separate material matrix passes pasted-text output/checks/contact/reload/deletion and authored-reference portability (2 passed); its two real-PDF cases are explicitly skipped on this host.
- Frontend: 858 unit tests pass (three existing skips), TypeScript and the 191-page production build pass. The mounted client owner passes 73 cases after the final account-switch regression: removing the identity guard produces an unintended second POST; restoring it retains the draft and permits only the identical request under its original identity. Mocked Playwright passes 74 cases (two existing skips). No file newly exceeds 1,000 lines and the dedicated quality review found no remaining high-confidence structural/test defect.
- Python baseline-runner checks pass (20); the 70-case quality command validates fixtures (zero human-reviewed cases), not model intelligence. The 141-test mandatory floor passes from clean backend commit `8c667103` against a freshly migrated isolated database. Its initial reused-database run failed because retained receipt fixtures produced 12 observed receipts instead of the expected 10; no production guard or test assertion was weakened.
- Actual AWS API, worker and migrator images build. Updated pdfplumber/pdfminer removes two HIGH findings; OS/Python scan passes. Full native PDFium/OpenSSL scan coverage is **not established**. Under unchanged production UID/read-only/cap-drop/default-seccomp/no-new-privileges restrictions, extraction self-check **fails creating a namespace on this development host**; the AWS target kernel has not been verified. Optional PDF capability is false and PDF-required startup fails. No profile relaxation or scanner waiver was introduced.

## Rollout, unresolved decisions and risks

- Merge only after exact-revision required CI, image/security rehearsal and regressions pass. Production launch separately requires healthy edge/API, matched configuration, fresh pollers and the deployed full journey; keep acquisition exposure off until those gates pass.
- Prepare and approve matching bootstrap assets, secret generation and changed platform-image hashes first. ACL/rendering and delivery-driver changes affect pinned host assets; routine application-image delivery alone cannot apply them. Then deploy narrow ACL/config/backend additions, replace compatible workers after paid execution drains, and enable frontend exposure. Missing `material_build` is unavailable to new clients. Legacy exports remain readable; old clients reject v2 cleanly. Preserve rollback image/data/uncertain holds; never replay paid calls on rollback.
- **Outage:** logs/deployed state are needed before specifying recovery; failed public API probes are not proof of a merge regression.
- **PDF blocker:** exact maintained runtime/ABI and narrow namespace compatibility require real image/target-host proof. Alpine is preferred; Debian or a separate extraction service requires explicit platform/security design and scan coverage. Text fallback does not count as completing PDF support.
- **Export boundary:** this plan proposes metadata plus explicit reattachment, not full binary backup. A later full-file archive needs a privacy/size/retention decision. Lost old metadata cannot be invented. Additive artifact JSON should avoid a migration; verify document/request bounds before committing that choice.
- **Auth/access:** per-attempt intent must fit real state/cookie limits and survive concurrent flows; setup cannot bypass membership policy. Historical access remains separate from execution availability.
- Hosted CI remains separate from these local results. The image lane remains blocked by incomplete native inventory; PDF-required rehearsal fails rather than reporting false availability. No deployment, merge, paid inference or real enquiry was performed. Report only actual checks/revisions/failures/skips/screenshots; prior counts do not satisfy new integration gates. Safari and physical iOS keyboard behavior remain unverified until tested.
