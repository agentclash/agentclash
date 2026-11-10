# Refund report v2 import — Test Contract

Base: 03e448d0c7246b6c82a934e2b6157bfacdfaa64c. Scope: existing workspace importer; no execution, deployment, SDK merger, or new service.

## Functional Behavior
- Envelope 1 accepts exactly eval_report/1, agent_result/1 and explicit eval_report/2. Reject relabeling, agent_result/2, unknown structural fields and inconsistent reports.
- Keep original UTF-8 bytes, SHA256, report identity, source version and all raw grades immutable. Preserve unknown/null cost, capture and settings. Version 2 uses a separate internal view, never a v1 adapter.
- Validate bounded structure, unique case/check IDs, runner/suite/target identity, required coverage, invocation/observation flags, summary/status/check/failure/exit consistency. Retain valid incomplete, skipped, config-error and zero-case evidence with UNKNOWN diagnostics.
- Retain exact human case mapping, artifact/published version/workspace access and suspended-user guards, source quotas, idempotent original receipt/time and one durable analytics event. Changed original/metadata conflicts.
- Imported grades are uploader claims. Retain source suite/target/revision/check/fixture provenance. Surface conflicting declared claims. Block v2 hosted regrade/promotion before quota/provider work; retain v1 behavior and exact original export.
- Expose additive supported import formats in existing config. UI offers v2 only with explicit server support; old servers retain v1. Preview safe allowlisted summaries; no raw prompts/tool arguments/replies. Explicit authenticated save retains original file. Unknown capture starts unknown; retry body immutable; owner/workspace changes cancel; reopen identity checked.

## Unit Tests
- DecodeSDKReport: unchanged canonical v1 fixtures; four inspected scripted refund CI reports accepted as v2; wrong labels, duplicates, unknown fields, summary/coverage/exit/check inconsistencies rejected.
- sdk v2 projection: safe case/check/provenance, incomplete UNKNOWN, null cost retained, revision disagreement diagnostics.
- Canonical v2 schema equals backend mirror; existing v1 schema hashes unchanged.
- Frontend preview/discovery/mapping/save: v2 supported and old-server unavailable, escaped metadata, no full trace summary, immutable retry and auth/workspace pending cancellation; v1 tests stay passing.

## Integration / Functional Tests
- Real owned loopback PostgreSQL with all current migrations; HTTP import/read/original where harness allows. Exact original bytes/hash and retry ID/time; changed source/metadata conflicts; wrong workspace/artifact/case/published binding and readonly/suspended actors rejected; quota enforced.
- Regrade/promotion v2 rejected before admission/compiler/provider side effects; v1 remains supported.
- Focused backend tests/race/build/vet; affected frontend tests/types/lint; schema parity; OpenAPI errors compared with existing baseline (3 Vibe errors and 2 tryouts-client warnings).

## Smoke Tests
- Local controlled HTTP import/read/original only. Deployment smoke is out of scope; source readiness does not imply deployed support.

## E2E Tests
- Add focused controlled authenticated browser import/map/save/reopen/original journey if existing harness permits, without new auth infrastructure. Distinguish controlled auth from WorkOS/live ownership.
- Real customer agent report/save acceptance remains OPEN: supplied reports are actual SCRIPTED synthetic demo executions, not customer agents. ACQ-07/#8 is not closed.

## Manual / cURL Tests
- Use owned test server/database harness to POST envelope 1 eval_report/2, GET receipt and GET original as an authorized member, retry identical request, then reject changed metadata. Record exact commands/results in implementation report.
- No hosted calls, provider/model runs, credential reads, automatic uploads or sharing. Frozen private evidence stays unchanged; inspected synthetic fixtures only may be copied with their own recorded digest.
