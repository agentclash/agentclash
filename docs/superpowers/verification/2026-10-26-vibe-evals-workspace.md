# Vibe Evals workspace verification — 2026-10-26

This records the implementation snapshot before the user-requested simplification. Further broad verification and review were stopped. The next sequence is a draft PR against main, parallel overengineering audits, removal of unnecessary complexity, then testing of the reduced implementation. This record does not assert approval or final validation.

## Scope and source milestones

The workspace retains authored test identities, fingerprints and provenance across canonical publication, connected CI execution, evidence, regression gating and explicit reimport. Reimport creates a new unapproved artifact; publication and CI baseline promotion are separate explicit actions. Imported scores remain historical. CLI `evaltest` is a smoke path; the connected canonical integration is the exercised application execution path.

- Base after reviewed Tasks1–9: `59f1670a0ee39d49b367869d5a722d8f2b6c4e25`.
- Product implementation and emitted return links: `054c2a0f3ff9c42b4f7dfa499e903efe0e5856bd`.
- Isolated canonical fixture compatibility: `05ff70faf91a413087a5298959c669ce7c94ec10`.
- GoReleaser configuration: `c6ffb0318682449e5bf0b54dd95047d1bd07645b`.
- SDK child lifecycle/current browser contracts: `f3bb0fe2affd8c93c6f6568d1f9fd5dfad9e5dbc`.
- Search/message visibility: `4ce144245f4e382be96e6a7de9f6002de52a30a0`.
- Instruction-only comparison continuity: `39a9a90a756573319cd2a382b94ba9d35455ddfb`.
- Single prototype completion owner: `84b755f254920fde3f5c66f5445b10cdaacf6ce5`.
- Nullable-session type correction: `f832977009ba3e3e51ffc15b37dd0f7f9f6765fc`.
- Final source/test milestone: `611db380dc382eda1845cd3c701885c2b0713fe9`.

No new migration was needed; additive migrations through92 remain intact. Original dirty developer checkout was preserved.

## Commands and results

Commands ran on the Linux server through `agent-run`. Backend database commands used `python3 scripts/vibe/fixture.py --` and a unique freshly migrated child. Every helper invocation set `VIBE_BROWSER_OWNERSHIP_LOG`. Counts below are Go passing test/subtest events, not unique top-level tests.

| Gate | Source | Exit / result |
| --- | --- | --- |
| Backend API/vibe race, prescribed workflow filter |054c|0;928 pass,4 explicit skips|
| `go -C backend build ./...`; `go -C backend vet ./...` |054c|0 each|
| `go -C backend test -race ./internal/enquiries ./internal/vibe/inputs -count=1` |054c|0;12 pass,0 skip|
| Canonical repository race, prescribed filter, `--database-env DATABASE_URL --label task7` |05ff|0;84 pass,0 skip|
| Canonical API/engine/workflow/worker race, prescribed filter |05ff|0;261 pass,0 skip|
| `go -C runtime test ./connectors ./provider ./scoring ./challengepack ./runner -count=1` |05ff|0;875 pass,0 skip|
| `python3 scripts/vibe/run_baseline.py --report <owned-report>` |05ff|0;139 mandatory tests,none missing/skipped|
| `python3 -m unittest discover -s scripts/vibe -p 'test_*.py' -v`; `python3 scripts/vibe/quality.py` |05ff|0 each|
| Frontend prescribed Vitest suite; `tsc --noEmit --incremental false` |611db380|0 each;42 files,389 tests|
| Offline fixture suite; corpus validation |611db380|0 each;26 tests;70 corpus cases|
| CLI `go build ./...`; `go vet ./...`; `go test -short -race -count=1 ./...` from cli/ |05ff|0 each;636 pass,0 skip|
| CLI `go run github.com/goreleaser/goreleaser/v2@latest check` |c6ff|0;v2.19.0,Go1.27.3|
| CLI `go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean` |c6ff|0;six OS/architecture binaries,no publish|
| `node scripts/publish-npm/assemble.mjs v0.0.0-rehearse cli/dist`; `npm pack --dry-run` in each package |c6ff|0;all seven package dry runs|

The offline quality command validates the reference corpus only:70 cases,0 human-reviewed. It is not a measured model-quality or release-readiness pass; funded held-out review remains external.

The four backend skips are the opt-in browser-server test (exercised separately) and three funded live-provider tests. The real existing isolated PDF reader passed extraction and secret/network/write/memory denial tests using Python3.14.4/pdfplumber0.11.7 and bubblewrap. No runtime was installed or sandbox bypassed.

## Meaningful failures retained

- Snapshot owner reproduced13 queries against bound12 before optimization. Green retains equal1/100-operation counts, maximum12, no raw snapshot evidence, and revocation inside writes.
- Reimport HTTP owner failed404 before implementation; negative proof retains foreign/partial/tampered/stale rejection and exact original bytes.
- CLI existing-deployment and gate-policy owners failed before implementation; real gate policy now reaches canonical request. Return-link owner failed404 before implementation and proves private actor/workspace authority, deleted originating draft, unavailable source and no other actor project discovery.
- Canonical repository initial run failed13 test/subtest events. Strict connector fixture prefix and duplicate case-validator fixture keys were repaired; guards and assertions retained.
- Actual concurrent cleanup failure is accepted only after the exact reserved database identity is absent; surviving/replaced identity still fails. No uncertain accounting was cleared.
- SDK child-loss proof initially lacked group absence due to an orphaned SDK zombie; the bounded existing supervisor now adopts and reaps its children. Original failure and exact cleanup receipts were retained.
- Original GoReleaser check exited1 on deprecated Homebrew settings; migrated configuration passed check and snapshot. macOS installation itself remains unverified.

## Resource and external boundaries

Before allocating children, the existing JSONL owner records exact database, directory, process and optional SDK devserver intent; database cleanup additionally requires the created OID. Successful cleanup records exact absence and process exit. Failed or ambiguous ownership fails safely; there is no prefix sweep or parent reset. Catchable interruption, API child loss and actual startup failure are separate executable lifecycle proofs. Killing the supervisor itself with uncatchable SIGKILL cannot promise automatic cleanup; retained exact receipts support bounded reconciliation.

SDK mode owns its in-memory Temporal process and files; its namespace lifetime ends with that process. Shared Temporal namespaces from earlier authorized tests retain metadata under the accepted policy;24-hour workflow-history retention is not namespace deletion. No shared service or namespace administration was performed by Task10.

Production/provider keys, funded targets, real WorkOS, public ports, deployments and external publication were not exercised. Native Mac Safari/VoiceOver and physical browser200% zoom require the Mac workstation. Linux coverage includes deterministic2x text enlargement and the prescribed headless width/theme/motion/forced-color/keyboard matrix, with final evidence recorded below when complete.

## Latest browser state before simplification

At `611db380`, the core real-stack suite completed 20 tests successfully. The latest material suite completed with exit1: one passed and two failed because the prototype displayed an empty new conversation instead of its retained initial trial. No product repair was applied for that issue before the user stopped the verification loop. The mocked browser child previously reported 82 passes and two opt-in skips, but its outer driver exited143, so it is not recorded as a complete successful gate. Production comparison was not run.

All 50 created test databases, 27 API directories and 20 frontend directories were reconciled absent. No owned test server or active command remained when work switched to simplification. Twelve shared Temporal namespace metadata records were retained without administration. Historical checks below describe their tested source only; validation of the reduced branch will be recorded separately.

## Exact backend commands

The following are the successful owning commands from the repository root; each ran inside `agent-run` with an explicit disposable parent DSN and credential-safe JSONL inventory. Output report paths are represented by `$REPORT` because they are local artifacts, not API inputs.

```bash
python3 scripts/vibe/fixture.py -- go -C backend test -p 1 -race -json ./internal/vibe ./internal/api -run 'Test(Vibe|Observed|PublicShare|Publication|GitHubIntegration|Regression|ReleaseGate|RunComparison|ControlledPreview|ControlledFleet|Suite|Consistency|Integration|Structured|YAML|Fanout|Money|FullContext|Unknown|Coverage|RunQuote|SampleIdentity|Requirement|StateMachine|MissingPricing|TrialProtectedAllocation|PaidTemporal|InvalidJudge|CORSMiddleware|BillingManager.*Webhook|AgentTryoutPromot|Generate|Evidence|Conversation|AuthoringPolicies|RetainedAuthoring)' -count=1
go -C backend build ./...
go -C backend vet ./...
python3 scripts/vibe/fixture.py -- go -C backend test -race -json ./internal/enquiries ./internal/vibe/inputs -count=1
python3 scripts/vibe/fixture.py --database-env DATABASE_URL --label task7 -- go -C backend test -race -json ./internal/repository -run '^TestControlled|EvalSession|TestConnector|GetRunAgentExecutionContext|WorkspaceSecret|CreateQueuedRun|CreateEvalSessionWithQueuedRuns|ListActiveAgentDeployments|Regression|ReleaseGate|CIProfile' -count=1
python3 scripts/vibe/fixture.py -- go -C backend test -race -json ./internal/api ./internal/engine ./internal/workflow ./internal/worker -run 'EvalSession|EvalSet|ControlledScoring|EvaluateLLMJudges|PromptEval|NativeExecutor|Connector|Hosted|RunCreation|CreateRun|AgentBuild|AgentDeployment|AuthorizeWorkspaceAction|WorkspaceSecret|Healthz|NativeModel|QuickCreateAgent|ExecuteNative' -count=1
go -C runtime test -json ./connectors ./provider ./scoring ./challengepack ./runner -count=1
python3 scripts/vibe/run_baseline.py --report "$REPORT"
python3 -m unittest discover -s scripts/vibe -p 'test_*.py' -v
python3 scripts/vibe/quality.py
```

## Frontend and CLI commands

The frontend command ran from `web/` after all Next fixture processes stopped:

```bash
./node_modules/.bin/tsc --noEmit --incremental false
./node_modules/.bin/vitest run 'src/app/(workspace)/workspaces/[workspaceId]/runs/[runId]/run-detail-client.test.tsx' src/lib/ci-setup.test.ts 'src/app/(workspace)/workspaces/[workspaceId]/ci-setup/ci-setup-client.test.tsx' src/components/share/report-snapshot.test.tsx src/components/share/workspace-reports.test.tsx src/app/vibe-evals src/components/vibe src/lib/vibe.test.ts src/lib/vibe-controllers.test.tsx src/lib/vibe-export.test.ts src/lib/vibe-reports.test.ts src/components/challenge-packs/vibe-return.test.tsx src/lib/vibe-build-analytics.test.ts src/lib/use-vibe-inputs.test.tsx src/lib/vibe-request-recovery.test.ts
```

CLI commands used the `cli/` module (`go -C cli` for the first three), with all five required validations:

```bash
go build ./...
go vet ./...
go test -short -race -json -count=1 ./...
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

## Implementation contracts and final UI triage

| Owner | Contract |
| --- | --- |
| `backend/internal/vibe/reimport.go`, API library handler, `reimport-review.tsx` | Preview/apply binds exact uploaded bytes, current revision and source artifact; stable-key semantic changes create another unapproved artifact. Foreign/duplicate/partial deletions, stale revisions and tampering fail; original history is retained. |
| `cli/cmd/ci_run.go`, canonical result lookup and project return controls | Existing deployment admission and policy file reach actual canonical CI/gate owners; emitted links resolve retained publication tuples only for the private owning actor with workspace access. |
| `validation_reviews.go` / snapshot store | Latest10 reviews and event cursor share one query; unchanged12-query maximum,1vs100 growth, no-raw and write-revocation invariants. |
| `fixture.py`, `browser_stack.py`, existing Playwright launchers | Independent floor/browser children, exact OID cleanup, catchable interruption and adopted owned SDK descendants; failure remains nonzero and uncertain ownership remains retained. |
| Existing Vibe CI workflows | Route canonical/run/CI/public-sharing owners and SDK-mode lifecycle failures; separate fresh broad/floor children and always-upload inventory. |

Deferred UI observations were addressed in their existing owners: stale metadata errors/busy state respect project/actor ownership; imported raw summaries are collapsed and marked unverified; library counts explicitly describe prototype checks; recovered Fleet controls reflect retained mode/count; observed participants retain sample/run identity. Connector async paths were expanded for readability without changing their access/receipt contracts.

Final integration also repaired actual visible/behavioral regressions: dialog focus after responsive Guide transitions, short viewport composer overflow, light-theme message contrast, sidebar search44px affordance/border, instruction-only comparison continuity, and duplicate prototype completion actions. The shared comparison helper preserves original parent/version/grading identity; it never promotes a CI profile baseline. Recorded replies retain their separate completion owner.

Meaningful retained browser failures include message contrast1.022<4.5, search height23.1875<44, and missing `baseline_id` after a completed instruction-only rerun. The repaired owners passed their unchanged bounds and identity assertions. Full real stack attempt03 had13pass/7fail: three duplicate-action failures, one journal/browser selection race and three stale navigation/entry expectations. The seven affected flows then passed. Typecheck subsequently exposed a nullable-session signature mismatch (exit2), corrected without runtime change. Vitest388pass/1fail exposed the historical-result test still seeking the removed duplicate panel; it now follows the actual prototype action flow and retains exact pinned-version quote/no-run assertions.
