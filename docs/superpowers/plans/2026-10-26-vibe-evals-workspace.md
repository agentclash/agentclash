# Vibe Evals workspace implementation plan

> Execute the approved specification with subagent-driven development. Do not stop between releases.

Spec: [approved product specification](../specs/2026-10-26-vibe-evals-workspace.md).
Audit evidence: /tmp/agentclash-hig-audit-20261026/audit.json and its referenced reports, produced using apple-human-interface-skills commit 6edb27c8324e4145fa2e98b09136f6f573a9f101.

## Global Constraints

- Keep actor-private raw sessions, server authoritative journals, existing budget ceilings and billing policy. Do not invent production credentials, funding, approved models or deployment access.
- Reuse canonical execution, eval sessions, public shares and CI. New connector secrets must be workspace scoped and approved for the target origin; no process env fallback.
- Preserve existing URLs, API clients and stored artifacts through additive migrations and explicit compatibility parsing.
- Work in the managed isolated checkout; preserve unrelated original-checkout edits. Bind development servers to 127.0.0.1. Heavy commands use agent-run. No production deployment, paid probe or external messages.
- Meaningful behavior regressions belong at their strongest owner boundary; record RED/GREEN. Do not add tests for source spelling or duplicate implementation. Visual-only changes use browser inspection.
- Read web/AGENTS.md before frontend edits. Use existing components and semantic tokens, scoped appearance, no new global marketing theme.
- A release is complete only when its specification is implemented and reviewed; report external verification constraints explicitly.

### Task 1: Backend readiness and truthful public capabilities

Implement Release 1 backend/config/deployment runbook requirements. Inspect backend/internal/vibe/models.go, backend/internal/api/vibe.go, health.go, worker dispatch, Helm readiness, .env.example and docs/vibe-evals-ubuntu.md. Separate liveness/dependency readiness/inference availability; include database migration and usable Redis operations, Temporal pollers and accounting/outbox readiness where owned. Return additive availability.guest/workspace, server_time and material_build capabilities without leaking secrets/balances. Keep historical operations/reconciliation usable when inference is down. Validate stable configuration and preserve all existing budget limits and quote durations; expose matching UTF-8 limits and truthful contact/model capabilities. Correct deployment samples and document exactly which production settings require an operator. Add owner API/config regressions that fail before repair, run focused tests and build. No frontend edits in this task.

### Task 2: Draft, quote, trial, entry and list reliability

Implement remaining Release 1 behavior and Release 2 draft/navigation foundations in existing Vibe controllers/components. Read approved spec Releases 1–2. Preserve editor drafts across unmount/project/reload/storage failure with stable schema 2 keys and actual source conflict detection; migrate old drafts. Quotes have explicit preparation/current/expired/failure, max price/funding/scope and nonexecuting refresh using server time with existing durations. Trial dock exposes New/history and grouped thread boundaries. Align UTF-8 limits, touch/IME-safe send semantics, analytics attempts/failures before admission and data-use notices before estimates/upload/submission. Implement availability/example/account/help/contact gating and truthful list states retaining previous rows. Canonical URL views and meaningful history navigation, stale/foreign session protection and one subscription. Regression tests own lifecycle/transport risks rather than source strings. Coordinate additive frontend types with backend output. Do not redesign unrelated marketing.

### Task 3: Agent, Tests, Results workspace and accessible materials

Implement Release 2 workspace/layout/material/accessibility requirements in the specification. Persistent Agent/Tests/Results tabs, stable project/version/evidence header, explicit Guide reference and mutually exclusive Guide/Details utility surface. Use specified responsive breakpoints and sheet/drawer behaviors. Add scoped System/Light/Dark provider including portals without global theme changes. Dialog geometry/typography/motion/focus and full titles/response expansion/announcements/semantics. Material roles, Paste default when no PDF, explicit rules/passages/partial extraction acknowledgement, dedup and unattached draft retention, per-source failure and stalled recovery. Agent and Tests editing in main area; preserve drafts and selected context through tab switches without inference. Browser inspect required widths, focus, motion and theme; Mac-only tests remain an honest limitation.

### Task 4: Results validation, provenance, comparison and recovery

Implement Release 3 results/recovery requirements using existing suite validators and authoritative execution state. Show case input/expectation/rules before Run, detailed validation outcomes with targeted fixes, preserve prior suite. Stable outcome, filters/full explanation, before/after replies, immutable provenance and fingerprints, changed-variable comparison and controlled comparison checks. Append-only grade dispute reviews separate from policy edits/regrades with additive backend persistence/endpoint. Last confirmed phase progress, elapsed time, safe Stop, billing unknown and completed evidence. Reload/retry unresolved/full rerun; quoted subset case_keys only after reconciliation produces a separate run, never merged evidence. Meaningful regression proof at API/state/UI owner boundaries.

### Task 5: Project library, save/export and advanced handoff

Implement Release 3 library/save/export requirements. Add metadata endpoint with revision/title/archived and archived_at migration; expose title/updated_at frontend, rename/archive/search/run metadata/sidebar preference. Actor ownership remains. Save summary pins exact version/tests/baseline/workspace/visibility/credits, preserves auth and advanced return and explains no deployment. Selected run/version export default, explicit full history and exact/confirmed partial manifest with missing list/progress/retry/names; preserve v1 import compatibility and complex original bytes. Delete project naming/retained-copy disclosure; Help matches actual controls. Verify migration, metadata ownership/conflicts, export failure/selection and auth-return behavior.

### Task 6: Immutable SDK report intake

Implement Release 4 report lane: POST /v1/vibe/sessions/{id}/reports and additive storage. Accept existing v1 SDK report/agent-result in strict bounded metadata envelope with application revision/fingerprints/capture scope, ownership and case identity validation. Original bytes/grades immutable, explicit imported provenance and review-before-promoting expectations, separate hosted regrade. Provide usable report import UI in project Results. Reuse canonical references rather than new scheduler. Test malformed/oversized/foreign/unknown-case imports and preserved originals.

### Task 7: Approved live connector adapters

Implement Release 4 connector lane in canonical deployments/runtime/UI. Add native default/http_json/hosted discriminators; preserve native requirements. Admin approves HTTPS target-origin+workspace-secret binding and explicit JSON request/response pointer mappings; members can execute approved connectors, guests text only. Enforce SSRF/private network/redirect protections, response/body/time bounds, no env/global secret fallback, durable authenticated deduplicated dispatch/callback and unknown-outcome no-auto-retry. Reuse signed hosted callback protocol, existing canonical billing/entitlements/spend controls, distinguish external charges and acknowledged cancellation. Extend schema/types/UI and test protocol/security boundaries against local deterministic fixtures (no real network/provider spend).

### Task 8: Tool/retrieval evidence and repeated experiments

Implement Release 5 observed evidence and experiment requirements. Normalize run/replay/SDK tool_calls and retrieval_context with unknown/partial/complete capture; preserve observed-none versus uninstrumented and reference corpus versus observed retrieval. Surface evidence in Results. Reuse eval-session repetitions/Fleet for controlled fixed-case experiments with scope/price/sample size/variance/changed variables and acknowledged seed only. Preserve source fingerprints and billing contracts; test normalization and experiment request/result semantics.

### Task 9: Scoped shared reports and annotations

Implement Release 5 collaboration requirements with additive selected-report snapshots, read-only viewers, append-only member/admin annotations and immutable grades. Raw actor-private sessions stay private. Reuse existing public shares with explicit selected evidence, default private/unindexed, expiry and revocation; exclude raw data unless deliberately selected. Provide usable sharing/annotation controls and test cross-actor/workspace role, expired/revoked, original grade immutability and selected scope.

### Task 10: CI round trip and final verification

Implement Release 6 using existing pack compile/publish/canonical deployment/GitHub CI/regression resources. Round trip preserves IDs, expectations, validators, provenance and fingerprints; semantic reimport diff and explicit publish/baseline promotion. Historic imported scores stay historical; label CLI evaltest as smoke and document actual SDK/pytest or connected runtime. Verify reviewed test→CI→evidence→regression→project using deterministic integration fixtures. Update user help and deployment/rollback docs. Run all prescribed CI gates plus responsive/browser checks; record exact commands, skipped/unavailable dependencies and Mac/production prerequisites honestly. Final independent whole-branch review and fixes before claiming completion.
