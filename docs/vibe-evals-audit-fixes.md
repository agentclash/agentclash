# Vibe Evals audit fixes

Implementation and verification record, 10 September 2026. Branch: `codex/vibe-audit-fixes`, based on `104fb64e8c441821409d47b227d5350a5304f4b8`. The prior reliability changes remain in the base. This change improves the text preview, offline test plans and documented handoffs; it adds no customer endpoint, audio or telephony execution.

| Finding | Implemented behavior |
| --- | --- |
| F01 | Authoring receives compact conversation context, stable active requirement IDs/status, accepted instructions, complete evaluation contracts and relevant evidence. Identical instructions and duplicate artifacts are represented once; administrative metadata and redundant schema prose are omitted. The conservative context count and one-repair limit remain unchanged. Oversize errors identify the largest section and preserve editing/export access. |
| F02 / F06 | The starter choice persists idea/existing-agent intent before authoring. Journey context records reported stack and evidence. Existing-agent authoring asks for missing setup/evidence or creates a separate `test_plan`; its scoped response schema excludes executable drafts until explicit preview consent. |
| F03 / F06 | A server-owned capability catalog supplies the assistant and UI with reviewed documentation. Plans receive a reviewed Python pytest template and concrete handoff steps from the server. The template requires a real invocation, local authentication and bounded timeouts; it cannot pass against hardcoded mock output. Supplied-text analysis, company discovery, mock tests and audio/telephony evidence remain distinct. `evaltest run` is documented as the smoke runner. |
| F04 | Effective preview instructions prohibit claims of connected bookings, transfers, notifications or future follow-up. New authoring checks explicit action promises and unsupported criteria. Revision summaries describe committed artifact changes; ignored attempts to alter accepted tests cannot appear as successful edits. These checks do not constitute a complete semantic classifier. |
| F05 | Add/replace/remove proposals reference stable requirement IDs, suppress whitespace-normalized duplicates and preserve revision history. Pending changes to confirmed rules need explicit confirmation. Reply, artifact, journey and requirement changes commit atomically. Product-support replies cannot create target-agent requirements. |
| F07 | Design, customer trials and evaluation have separate labels. Trial controls appear before acceptance with their prerequisite. Trial messages retain operation/artifact links and a distinct origin. Test plans expose review, export and docs, with no target-model or scorecard controls. |
| F08 | Drafts expose editable examples and the actual compiled criteria before acceptance, with requirement links and proposed assumptions identified. The current authoring contract requires positive, negative and insufficient-evidence inputs even on JSON-object routes. Evidence rules allow no qualifying recommendation. Test edits create a new contract; retests retain original cases, judges and evaluator. |

The existing compiler/composition architecture, Geist typography, `builder-*` styling and ClashMark are retained. The new integration guide is [Test an existing agent from Vibe Evals](../web/content/docs/guides/vibe-evals-existing-agent.mdx). Its SDK reference is the [AgentClash pytest integration](https://github.com/agentclash/agentclash-evals/blob/main/docs/evaltest/pytest.md).

## Compatibility and rollout

All additions use existing JSON documents; there is no database migration. Missing artifact kinds remain readable as `agent_draft`, and legacy messages render as design conversation. Existing saved-model receipts, exports and immutable evaluation data remain readable.

Queued operations retain their persisted authoring contract version. The backend supports legacy replies, version 2 replies and the current version 3 single-artifact contract. New version 3 operations reject legacy draft fields and missing example categories without weakening validation of old queued operations.

Release the compatible worker/backend handling before enabling the updated UI. Upgrade workers before allowing new API submissions to use the new authoring version; let active operations settle during the transition. The UI uses catalog-provided code and next steps for historical test plans as well as new ones, while the stored original conversation remains unchanged. Recheck profile conformance when its existing verification expires. No deployment was performed for this audit.

## Automated verification

| Check | Result |
| --- | --- |
| Vibe and relevant API Go/PostgreSQL race suites | 109 top-level tests, 273 including subtests; all pass, no skips. Dedicated `vibe_test` database. |
| Backend build and vet | Pass. |
| Web suite | 656 pass, 3 skipped. The 40 VibeClient tests also pass after the last handoff regression update. |
| Browser verification | 10 Playwright tests pass with mocked Vibe API responses; these use no model calls. Live UI, documentation, customer-trial labels and scorecard reload were also inspected. |
| TypeScript and ESLint | Type check passes. ESLint has no errors and five pre-existing warnings in unchanged files. |
| CLI and shared runtime | Build, vet and short race suites pass. |
| Contributor lifecycle checks | All 8 pass. |
| OpenAPI and patch checks | YAML parses with unique keys, all 1,501 local references resolve, and 209 operation IDs are unique. `git diff --check` passes. |

The three skipped web tests are the opt-in generic backend integration smoke tests (`INTEGRATION_TESTS` was not enabled). Existing local dependencies were reused; dependency installation was not repeated and both lockfiles remain unchanged.

The full repository check is **not green**. Five failures were reproduced against an untouched archive of `104fb64e`, using a separate `vibe_audit_repo_test` database:

- `TestRepositoryOrganizationEntitlementGatesBlockWorkspaceAndSeatWrites`: expected billing gate error is absent.
- `TestRepositoryListRunFailureReviewItemsBuildsPerCaseItems`: failure-class expectation differs.
- `TestRepositoryPromoteFailureFreezesContextAndIsIdempotent`: frozen JSON payload comparison differs.
- `TestRepositoryListRunRegressionCoverageCasesByRunID`: duplicate judge-result key in the fixture.
- `TestRepositoryEvaluateRunAgentReturnsPartialWhenChallengeInputIsAmbiguous`: persisted JSON payload comparison differs.

A Vibe normalized-reference test also timed out during the broad concurrent backend run; it subsequently passed in the isolated Vibe/API race runs. SQL generation was exercised, but unrelated generated-file drift from unchanged SQL inputs was discarded. The Vibe changes do not add or modify SQL query files.

Sanitized versions of the four recorded conversations are in [the regression fixtures](../backend/internal/vibe/testdata/README.md). Their complete initial requests have these conservative bounds, including the response schema and verified framing allowance:

| Recorded path | Initial bound | Initial and single repair fit 16,384 |
| --- | ---: | --- |
| Research idea | 15,198 | Yes |
| Existing research agent | 13,634 | Yes |
| Receptionist idea | 15,378 | Yes |
| Existing voice agent | 15,993 | Yes |

Tests reconstruct and compare every original evaluation field after compaction, exercise long repair diagnostics, and verify genuine overflow creates zero provider attempts while preserving accepted state. Further regressions cover concurrent requirement decisions, atomic rollback, legacy recovery, execution rejection for plans, immutable retests and the existing accounting/idempotency boundaries.

## Capped live verification

The trial used the previously conformed free route `dots-studio/dots-3-note-preview:free` on `atlas-cloud/fp8`. Configuration and ledger checks ran before every operation. No additional model-profile conformance request was needed. The cap was 24 AI requests, further constrained by retained prior usage. All original accounting records remain intact.

The original anonymous browser cookie was unavailable. A new QA browser identity was used, with an external guard carrying forward the original trial's consumption and limiting this audit to its remaining **eight design submissions**. No quota rows or accounting history were reset. The final ledger contains **19 new provider attempts**, all settled at **$0**, with no uncertain outcomes or disabled profiles. No more live calls were made after the requested evaluation completed.

| Path | Design submissions / AI requests | Observed outcome |
| --- | --- | --- |
| Research idea | 3 / 5 | The third turn continued without context overflow and produced a draft. The first two turns failed authoring validation after their repairs; the eventual fictional-company prototype had weak criteria. This is partial live coverage, not three successful revisions. Those observations led to the single-artifact response and explicit example categories. |
| Existing research agent | 1 / 2 | Produced a separate non-runnable plan with the reported Python/FastAPI/LangGraph stack. Its original scenarios and generated mock-based Python handoff were inadequate. The server-owned handoff replaced that code in the displayed/exported plan; the original raw history remains intact. No additional clean design call was spent on this path. |
| Receptionist idea | 1 / 1, then 1 trial and 6 evaluation requests | Produced a safe text draft with three editable examples. After review and acceptance, the customer trial truthfully declined booking, transfer and later texting. The persisted evaluation passed 3/3 cases and 6/6 checks; reload retained the scorecard and message links without another provider call. |
| Existing voice agent | 2 / 2 | Asked one compact setup/evidence question, then produced a non-runnable plan with the reviewed local handoff. Scenario quality still needs review: a booking-timeout scenario suggested a retry without first establishing the outcome, and stack/evidence fields were imperfectly classified. |
| Audit navigation mistake | 1 / 2 | A failed browser selector left the previous conversation selected and one research message was sent there. Both requests are counted, retained and excluded from the clean path results. Subsequent navigation used verified fresh-conversation controls. |

The local API/worker processes stopped after the customer trial and before the evaluation. The guard rejected evaluation submission while configuration was unavailable; no provider attempt occurred during that interruption. Restarting the services preserved state, and the evaluation then completed normally.

Final changes strengthened guidance on stack/evidence, separating scenario inputs from expected behavior, unknown-outcome retries, truthful revision summaries and server validation of all three example categories. Automated checks cover those changes. They were not given another live authoring trial because the retained design allowance was exhausted. The capped audit therefore does **not** establish that all four founder paths now produce consistently good plans.

Real WorkOS saving remains separately unverified. API tests cover saving with test authentication; there was no real sign-in/save, connected customer system, audio execution or production deployment. Raw audit transcripts, provider journals, browser state and credentials remain outside the patch.
