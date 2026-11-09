# codex/agent-integration-analytics — Test Contract

## Functional Behavior
- Browser acquisition_channel is claude/codex/unknown from first-touch utm_source only, immutable across later login visits; persisted attribution is validated/sanitized on read. It never establishes active host use.
- Explicit opt-in AGENTCLASH_ANALYTICS_HOST=claude|codex adds a bounded host marker to existing hosted CLI requests. Other hosts stay neutral; missing/invalid values remain unknown; no new requests/profile edits.
- Request context and canonical recorder propagate observed_host with a strict allowlist; crawlers/UTM/providers are never host evidence. Reserved canonical IDs/context cannot be overridden by arbitrary properties.
- product.tryout.claimed follows successful repository claim; product.sdk_report.imported follows successful committed import. Stable internal caller/entity/workspace IDs and deterministic retry UUIDs; failures cannot emit success; nil/noop recorders preserve behavior.
- Separate platform_demo, imported_local_evidence and hosted_run buckets. No inference of customer-application execution, installs or unobserved local SDK/MCP use.
- Reproducible offline query artifacts require explicit UTC cutoff/as-of/exclusion review; seven-day genuine-signup denominators are mature, deduplicated internal UUIDs joined to person acquisition traits; primary denominator requires signup_kind=new_user; invited_activation and missing/unrecognized kinds remain separately auditable; existing-user login is never signup. Weekly observed active authenticated host connections/workspaces exclude failures and unknowns from tagged counts, display unknown separately.
- No production access, credentials/env-file reads, network provisioning, deployment or invented baseline; actual deployment cutoff and observed baseline remain pending.

## Unit Tests
- attribution: first-touch Claude/Codex/unknown, redirect/later login immutability, tampered persisted state/unsafe fields and expiry.
- identity bridge: tagged acquisition retained on identify with internal UUID; auth-completed event never a signup.
- CLI BuildUserAgent: opt-in allowlist, hostile marker, neutral non-hosted destinations.
- productanalytics recorder: bounded context, reserved fields, workflow values, deterministic retry identity/noop.
- API middleware: explicit host parsing/context, crawler unknown; canonical state-transition tests for successful claims/retries/failures.
- Offline query tests: mandatory boundaries/exclusions, deterministic output, no credentials/network in dry-run, exact join/window/dedup/bucket definitions and synthetic fixture cohorts.

## Integration / Functional Tests
- Owned isolated PostgreSQL SDK import: new receipt, idempotent retry, rejected conflict, transaction failure; emitted analytics must correspond to durable stored receipt. No provider calls.
- Tagged Claude/Codex synthetic web attribution/auth fixtures and actual backend success fixtures share documented synthetic identity semantics; never described as production onboarding journeys.
- Existing signup-versus-login tests and canonical run transition tests remain passing.

## Smoke Tests
- Local query generator/provisioner dry-run with synthetic UTC parameters emits JSON without credentials or network.
- Affected-module Go build/vet/race checks and web focused Vitest/TypeScript/lint.
- Production smoke: pending deployment and dataset access, not run here.

## E2E Tests
Controlled component/manager/real PostgreSQL fixtures only; no claim of live provider execution or production end-to-end coverage. Production tagged journeys and baseline remain explicit closure gates.

## Manual / cURL Tests
No production cURL. Reproducible node dry-run/query commands and owned PostgreSQL fixture commands will be documented in docs/analytics.md and task report. Baseline template must retain pending cutoff, exclusion coverage, cohort counts and period.
