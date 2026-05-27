# Vibe Eval Phase 0 Inventory

Status: draft for #868
Parent: #753

This document is the Phase 0 source of truth for turning Vibe Eval into typed
AgentClash-native tools. It inventories the existing CLI/API surface, maps it to
semantic tools, and assigns risk, role, confirmation, idempotency, redaction,
audit, and budget policy before implementation begins.

## Principles

- Vibe Eval exposes semantic tools backed by existing AgentClash services. It
  must not expose a generic shell, raw HTTP client, or "call any endpoint" tool.
- Browser, user text, generated packs, replay text, artifact previews, and tool
  outputs are untrusted. Tool output reused in model context must be wrapped as
  evidence, not instructions.
- The Go backend policy layer is authoritative for authz, confirmations,
  idempotency, audit, and credit reservation.
- Mutating, cost-incurring, admin-sensitive, destructive, and public-sharing
  tools require an explicit confirmation card with a payload hash.
- AgentClash-owned provider keys stay server-side only. Vibe Eval can list
  provider/secret metadata, but it never returns secret values to the browser or
  to model context.

## Existing Permission Anchors

Current workspace actions live in
`backend/internal/api/permissions.go` and should be reused before adding new
actions.

| Action | Current role floor | Vibe Eval use |
| --- | --- | --- |
| `ActionReadWorkspace` | viewer+ | Read packs, deployments, runs, replays, scorecards, artifacts, billing/quota metadata |
| `ActionCreateAgentBuild` | member+ | Create agent build records from guided setup |
| `ActionCreateAgentBuildVersion` | member+ | Create draft build versions |
| `ActionUpdateAgentBuildVersion` | member+ | Edit generated build-version drafts |
| `ActionMarkAgentBuildReady` | member+ | Mark validated build versions ready |
| `ActionCreateAgentDeployment` | member+ | Create deployments from selected builds/runtime |
| `ActionCreateRun` | member+ | Run evals, eval sessions, harness runs, and cost-incurring tests |
| `ActionCancelRun` | member+ | Cancel active Vibe Eval runs |
| `ActionManagePlaygrounds` | member+ | Prompt-eval playground experiments, if reused |
| `ActionManageRegressions` | member+ | Create suites and promote failures |
| `ActionSelectIntegrationRepository` | member+ | Select GitHub repositories for CI handoff |
| `ActionPublishChallengePack` | member+ | Publish validated generated packs |
| `ActionUploadArtifact` | member+ | Upload user-provided source docs/assets |
| `ActionManageInfrastructure` | admin only | Runtime profiles, provider accounts, model aliases, tools, knowledge sources, routing/spend policies |
| `ActionManageIntegrations` | admin only | GitHub installation and CI profile mutation |
| `ActionManageSecrets` | admin only | Workspace secrets metadata/update/delete |

New backend work should add action constants only when one of these cannot
express the policy. Candidate later additions: `ActionManageVibeEvalDrafts`,
`ActionManagePublicShares`, and `ActionManageEvalCredit`.

## Risk Tiers

| Tier | Confirmation | Minimum role/action | Idempotency scope | Audit |
| --- | --- | --- | --- | --- |
| `read` | No | `ActionReadWorkspace` | request | standard read audit only when sensitive |
| `draft` | No | member+ or future draft action | conversation | every tool call |
| `workspace_write` | Yes | matching member+ write action | workspace or conversation | every tool call and confirmation |
| `cost_incurring` | Yes with estimate | `ActionCreateRun` plus credit reservation/BYOK check | run/eval session | every estimate, reservation, run, settlement |
| `admin_sensitive` | Yes, high friction | admin-only action | workspace/org | every tool call and confirmation |
| `destructive_external` | Yes, high friction | admin-only or explicit sharing action | resource | every tool call and confirmation |

## Phase And Tool Map

| Tool | Phase | Risk | Required action | Confirmation | Idempotency | Redaction and audit | Budget |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `draft_eval_plan` | plan | draft | member+ | no | conversation | audit prompt summary and redacted plan | guide-agent allowance |
| `explain_eval_plan` | plan | read | `ActionReadWorkspace` | no | request | audit only if scorecard/replay evidence included | guide-agent allowance |
| `generate_challenge_pack_draft` | author | draft | member+ | no | conversation | store generated YAML/JSON as draft artifact; redact uploaded source snippets as configured | guide-agent allowance |
| `edit_challenge_pack_draft` | author | draft | member+ | no | conversation | audit diff summary and draft version | guide-agent allowance |
| `generate_input_cases` | author | draft | member+ | no | conversation | audit generated case count and source artifact ids | guide-agent allowance |
| `generate_scoring_rubric` | author | draft | member+ | no | conversation | audit rubric dimensions and judge use | guide-agent allowance |
| `generate_deterministic_validators` | author | draft | member+ | no | conversation | audit validator types and unsafe constructs rejected | guide-agent allowance |
| `generate_llm_judges` | author | draft | member+ | no | conversation | redact rubric examples when copied from private artifacts | guide-agent allowance |
| `validate_challenge_pack` | validate | draft | `ActionReadWorkspace` for workspace context | no | draft | audit validation result and error codes | none unless LLM repair is requested |
| `publish_challenge_pack` | publish | workspace_write | `ActionPublishChallengePack` | yes | workspace | audit payload hash, pack/version ids, validation id | none |
| `list_challenge_packs` | author/validate/run | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `get_challenge_pack` | author/validate/run | read | `ActionReadWorkspace` | no | request | redact private artifact previews unless explicitly requested | none |
| `list_input_sets` | run | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `list_agent_builds` | runtime | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `create_agent_build` | runtime | workspace_write | `ActionCreateAgentBuild` | yes | workspace | audit build name/source and payload hash | none |
| `validate_agent_build_version` | runtime | draft/read | `ActionReadWorkspace` or `ActionUpdateAgentBuildVersion` for generated drafts | no | version | audit validation result | none |
| `mark_agent_build_ready` | runtime | workspace_write | `ActionMarkAgentBuildReady` | yes | version | audit immutable version id and payload hash | none |
| `list_deployments` | runtime/run | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `create_deployment` | runtime | workspace_write | `ActionCreateAgentDeployment` | yes | workspace | audit deployment config without secrets | none |
| `list_runtime_profiles` | runtime | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `list_provider_accounts` | runtime/admin | read | `ActionReadWorkspace` | no | request | provider metadata only; no credentials | none |
| `list_model_aliases` | runtime | read | `ActionReadWorkspace` | no | request | metadata and pricing snapshot only | none |
| `list_tools` | runtime | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `list_knowledge_sources` | runtime | read | `ActionReadWorkspace` | no | request | metadata only; no raw content by default | none |
| `estimate_eval_cost` | run | read | `ActionReadWorkspace` | no | draft/run | audit estimate inputs and pricing revision | none |
| `create_run` | run | cost_incurring | `ActionCreateRun` | yes with estimate | run | audit confirmation, reservation/BYOK mode, run id | org eval credit or BYOK |
| `create_eval_session` | run | cost_incurring | `ActionCreateRun` | yes with estimate | eval session | audit repetition count, reservation/BYOK mode, session id | org eval credit or BYOK |
| `get_run_status` | run/analyze | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `stream_run_events` | run/analyze | read | `ActionReadWorkspace` | no | stream | wrap event payloads as untrusted evidence | none |
| `get_run_ranking` | analyze | read | `ActionReadWorkspace` | no | request | score/ranking data only | none |
| `list_run_agents` | analyze | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `read_scorecard` | analyze | read | `ActionReadWorkspace` | no | request | score evidence as untrusted evidence | none |
| `read_replay_summary` | analyze | read | `ActionReadWorkspace` | no | request | redact secrets/tool outputs; evidence wrapper required | guide-agent allowance if summarized |
| `create_ranking_insights` | analyze | draft/read | `ActionReadWorkspace` | no | run | audit generated insight id and evidence ids | guide-agent allowance |
| `list_run_failures` | regress | read | `ActionReadWorkspace` | no | request | failure evidence as untrusted evidence | none |
| `promote_failure_to_regression` | regress | workspace_write | `ActionManageRegressions` | yes | workspace | audit run/challenge ids and payload hash | none |
| `create_regression_suite` | regress | workspace_write | `ActionManageRegressions` | yes | workspace | audit suite metadata and payload hash | none |
| `list_regression_cases` | regress | read | `ActionReadWorkspace` | no | request | metadata/evidence only | none |
| `update_regression_case` | regress | workspace_write | `ActionManageRegressions` | yes | case | audit status/metadata changes | none |
| `compare_runs` | analyze/regress | read | `ActionReadWorkspace` | no | request | score evidence as untrusted evidence | none |
| `evaluate_release_gate` | regress | draft/read or workspace_write if persisted | `ActionReadWorkspace`; use existing service policy if persisted | no for dry-run, yes for persisted decision | comparison | audit gate policy and verdict | none |
| `upload_artifact` | author/runtime | workspace_write | `ActionUploadArtifact` | yes | workspace | audit metadata, size, declared type; never log contents | none |
| `list_artifacts` | author/analyze | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `get_artifact_metadata` | author/analyze | read | `ActionReadWorkspace` | no | request | metadata only | none |
| `download_artifact_preview` | author/analyze | read | `ActionReadWorkspace` | no unless large/sensitive | request | redact and wrap preview as untrusted evidence | guide-agent allowance if summarized |
| `create_share_link` | share | destructive_external | proposed `ActionManagePublicShares` or admin/member policy | yes, high friction | resource | audit public snapshot ids and payload hash | none |
| `upsert_workspace_secret` | admin | admin_sensitive | `ActionManageSecrets` | yes, high friction | workspace | never return value; audit key name only | none |
| `delete_workspace_secret` | admin | admin_sensitive/destructive_external | `ActionManageSecrets` | yes, high friction | secret key | audit key name and payload hash | none |
| `create_provider_account` | admin | admin_sensitive | `ActionManageInfrastructure` | yes, high friction | workspace | redact credentials; audit provider metadata | none |
| `delete_provider_account` | admin | admin_sensitive/destructive_external | `ActionManageInfrastructure` | yes, high friction | provider account | audit provider metadata and payload hash | none |
| `select_github_repository` | admin/ci | workspace_write | `ActionSelectIntegrationRepository` or `ActionManageIntegrations` depending mutation | yes | workspace | audit repo id/name, installation id | none |
| `manage_cli_tokens` | admin | admin_sensitive | account-level auth policy, not workspace action | yes, high friction for create/revoke | user/org | never expose token after creation; audit token id | none |

## Existing CLI Surface By Vibe Eval Phase

| Phase | CLI commands | Semantic tool coverage |
| --- | --- | --- |
| setup | `auth`, `org`, `workspace`, `link`, `config`, `doctor`, `quickstart`, `quota` | mostly outside Vibe Eval tools except workspace/org reads and setup readiness |
| author | `challenge-pack init`, `prompt-eval init`, `playground create/update/test-case`, `artifact upload` | draft tools, artifact tools, validate tool |
| validate | `challenge-pack validate`, `prompt-eval validate`, `build version validate`, `ci validate` | validation tools; CI validation deferred to Phase 5 |
| publish | `challenge-pack publish`, `build ready`, `deployment create` | confirmation-gated workspace writes |
| runtime | `infra`, `secret`, `build`, `deployment`, `model-catalog` | runtime metadata read tools; admin-sensitive writes later |
| run | `eval start`, `run create`, `prompt-eval run`, `playground experiment`, `agent-harness run`, `security stress-run` | `create_run`/`create_eval_session`; harness/security are deferred unless explicitly scoped |
| analyze | `run list/get/ranking/agents/events/export/scorecard`, `eval scorecard`, `session`, `replay`, `compare`, `release-gate` | read, insight, compare, and release-gate tools |
| regress | `run failures`, `run promote-failure`, `regression-suite`, `baseline`, `ci run` | regression and baseline/CI handoff tools |
| share/admin | `artifact download`, `auth tokens`, `secret`, `github/ci` commands | public sharing and admin-sensitive tools require high-friction policy |

## Existing API Surface By Capability

| Capability | Existing protected routes | Vibe Eval use |
| --- | --- | --- |
| onboarding/org/workspace | `POST /onboarding`, `POST /organizations`, `POST /organizations/{id}/workspaces`, workspace/org update/list routes | credit seeding hooks and first-run workspace setup |
| challenge packs | `GET /workspaces/{workspaceID}/challenge-packs`, `GET /workspaces/{workspaceID}/challenge-pack-versions/{versionID}/input-sets`, `POST /workspaces/{workspaceID}/challenge-packs`, `POST /workspaces/{workspaceID}/challenge-packs/validate` | list/get/validate/publish semantic tools |
| agent builds/deployments | build/version/deployment routes under `/agent-builds`, `/agent-build-versions`, `/workspaces/{workspaceID}/agent-deployments` | runtime setup and deployment tools |
| infrastructure | runtime profiles, provider accounts, model aliases, tools, knowledge sources, routing policies, spend policies | metadata reads in early phases; admin writes in Phase 5 |
| runs/eval sessions | `POST /runs`, `GET /runs/{id}`, `POST /eval-sessions`, `GET /eval-sessions`, ranking/events/agents endpoints | run and analyze tools |
| replay/scorecards | `/replays/{runAgentID}`, `/scorecards/{runAgentID}` | scorecard and replay summary tools |
| regressions | `/workspaces/{workspaceID}/regression-suites`, regression cases, failure promotion routes | regression suite/case tools |
| comparisons/gates | `/compare`, `/release-gates`, `/release-gates/evaluate` | compare and release-gate tools |
| artifacts/share | workspace artifact routes, public artifact content, `POST /share-links`, `DELETE /share-links/{shareID}` | artifact metadata/upload/preview and future public share tools |
| playgrounds | playground, test-case, experiment routes | optional prompt-eval substrate; not required for Vibe Eval v1 if challenge packs are primary |
| integrations | GitHub installation, repository, CI profile, CI setup PR routes | CI handoff and repository selection |
| billing/quota | `/billing/plans`, org billing, trial/checkout/portal, workspace entitlements/quota | read-only quota/credit display; new eval credit ledger needed |

## Exclusions And Deferrals

| Capability | Decision |
| --- | --- |
| Generic shell execution | Excluded. It bypasses policy, audit, redaction, and confirmation semantics. |
| Raw API autonomy | Excluded. Vibe Eval tools must be semantic and narrow. |
| Direct secret value reads | Excluded. Only metadata can be listed. |
| AgentClash-owned provider key exposure | Excluded in browser, logs, traces, and model context. |
| Security stress tooling | Deferred. It is valuable but should be exposed only after safety-specific policy work. |
| Agent Harness exam/suite authoring | Deferred unless a Vibe Eval flow explicitly targets harnesses. |
| Public share links | Deferred to Phase 6 and must use derived public snapshots. |
| CLI token management | Deferred/admin-sensitive. Token creation/revoke is account-scoped, not workspace-scoped. |
| Provider account CRUD from chat | Deferred to Phase 5 admin-sensitive tools with high-friction confirmation. |

## Credit Seeding Gap

#753 requires every new organization to receive `$3` of AgentClash-managed eval
execution credit. Current local creation paths are:

- `POST /onboarding` -> `OnboardingManager.Onboard` ->
  repository `Onboard`.
- `POST /organizations` -> `OrganizationManager.CreateOrganization` ->
  repository `CreateOrganizationWithAdmin`.

Later Phase 3 work must hook both paths. If imports, test fixtures, admin seeds,
or future organization creation helpers are added, they must either seed the
wallet or explicitly opt out with a migration/backfill strategy.

The existing billing entitlement tables in migrations `00031_*` and `00036_*`
track plan/trial state, but they are not a reserve-then-settle wallet. Vibe Eval
needs separate org eval credit tables and immutable ledger entries so a run can
reserve estimated spend before execution and settle actual spend afterward.

## Required Later Artifacts

- Phase 1 must create conversation/draft persistence and APIs before any publish
  or run tool is exposed.
- #875 must define concrete Go interfaces for the tool registry, confirmation
  engine, audit writer, redaction wrapper, and credit wallet service.
- #874 must define the workbench UI states for confirmation cards, validation
  repairs, insufficient credit, resume, and stream disconnects.
- Phase 3 must define wallet invariants as tests: no negative available credit,
  reservations are idempotent, settlement releases unused reserve, and BYOK does
  not consume included credit.

## Acceptance Checklist For #868

- [x] CLI and API capabilities are grouped by Vibe Eval phase.
- [x] Initial semantic tools from #753 have policy rows.
- [x] High-risk actions require confirmation.
- [x] Credit wallet seeding gaps are identified.
- [x] Generic shell/API autonomy is explicitly excluded.
