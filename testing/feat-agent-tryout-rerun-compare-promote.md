# feat/agent-tryout-rerun-compare-promote — Test Contract

Implements issue #947 (build order 6 of 6 under epic #940): rerun, compare, and
promote-to-eval conversion flows for Agent Tryouts. Locked before implementation.

## Scope decisions (v1)

- **Rerun lineage** is tracked with a new nullable `agent_tryouts.parent_tryout_id`.
- **Compare** is a read-time aggregation over 2–4 workspace tryouts (no new
  persistence) — reruns are immutable evidence, so compare never mutates them.
- **Promote-to-eval** targets a **Vibe Eval draft** (the lowest-friction durable
  workspace draft). `vibe_eval` is the only supported target in v1; any other
  target returns a stable product error.
- **Billing/provider enforcement** is an optional injected gate
  (`AgentTryoutRerunGate`, mirroring `WithExecution`/`WithQuota`). When configured
  and it denies, rerun returns a product-facing error; when unconfigured, rerun
  proceeds (dev/default), consistent with existing optional-service wiring.

## Functional Behavior

### Rerun — `POST /v1/agent-tryouts/{tryoutID}/rerun`
- Auth: signed-in workspace caller. Body: `{ "selected_model_policy": {...} }`.
- Loads the source tryout; authorizes the caller against the source tryout's
  workspace.
- Anonymous/unclaimed source tryout (no workspace) → `401`-class sign-in-required
  product error (`ErrAgentTryoutSignInRequired`).
- Cross-workspace caller → `403` (ErrForbidden via AuthorizeWorkspace).
- Validates the requested model policy. Invalid shape → `400`
  (`ErrAgentTryoutModelPolicyInvalid`). Unknown provider/model →
  `422`/`400` (`ErrAgentTryoutModelUnavailable`).
- Optional rerun gate denial → product error (provider key required / insufficient
  credits) mapped to `402`/`403`/`429`.
- On success: creates a NEW tryout that clones the source's `template_slug`,
  `InputSnapshot`, `TemplateSnapshot`, `ToolPolicySnapshot`,
  `EvaluationSpecSnapshot`, `CostLimitUSD`, `MaxDurationSeconds`, sets
  `SelectedModelPolicy` to the requested policy, `ParentTryoutID` = source id,
  `RedactionStatus = pending`, `Status = queued`, same workspace/org. Dispatches
  via the existing execution path. Returns the new tryout (201).
- The rerun is independent immutable evidence — it does not mutate the source.

### Compare — `POST /v1/workspaces/{workspaceID}/agent-tryouts/compare`
- Auth: signed-in workspace member. Body: `{ "tryout_ids": ["...", "..."] }`.
- Requires 2–4 ids → otherwise `400` (`ErrAgentTryoutCompareCardinality`).
- Every id must belong to `{workspaceID}`; any id in another workspace or missing
  → `404` (not found) / `403`. (Fail closed; don't leak existence cross-workspace.)
- Returns a side-by-side payload: per tryout `{ id, template_slug,
  selected_model_policy, status, redaction_status, run_id, cost_limit_usd,
  actual_cost_usd, latency_ms, summary, events_url }`. `events_url` links to the
  workspace events endpoint for full evidence.

### Promote-to-eval — `POST /v1/agent-tryouts/{tryoutID}/promote-to-eval`
- Auth: signed-in workspace caller. Body: `{ "target": "vibe_eval", "title": "..." (optional) }`.
- Anonymous/unclaimed source → sign-in-required product error.
- Cross-workspace caller → `403`.
- Unsupported target (anything != `vibe_eval`) → `400`
  (`ErrAgentTryoutPromotionTargetUnsupported`).
- On success: creates a Vibe Eval conversation + an `eval_plan` draft whose content
  captures `source_tryout_id`, `template_slug`, `input_snapshot`,
  `tool_policy_snapshot`, `evaluation_spec_snapshot`, and the template's
  `expected_artifacts`. Returns `{ conversation_id, draft_id, target }` (201).
- The draft is durable workspace data with enough to re-run later.

## Unit Tests (api package, fake repo)
- `TestAgentTryoutRerunClonesSnapshotsWithNewModelPolicy` — new tryout clones
  source snapshots, overrides model policy, sets parent_tryout_id, dispatches.
- `TestAgentTryoutRerunRejectsAnonymousSource` — unclaimed/anonymous source →
  ErrAgentTryoutSignInRequired.
- `TestAgentTryoutRerunRejectsCrossWorkspace` — non-member caller → ErrForbidden.
- `TestAgentTryoutRerunValidatesModelPolicy` — invalid policy →
  ErrAgentTryoutModelPolicyInvalid; unknown provider → ErrAgentTryoutModelUnavailable.
- `TestAgentTryoutRerunGateDenial` — injected gate denial → mapped product error.
- `TestAgentTryoutCompareAggregatesParticipants` — 2–4 tryouts → per-tryout fields
  present; events_url populated.
- `TestAgentTryoutCompareRejectsCardinality` — <2 or >4 ids → ErrAgentTryoutCompareCardinality.
- `TestAgentTryoutCompareRejectsCrossWorkspace` — id in another workspace → not found/forbidden.
- `TestAgentTryoutPromoteCreatesVibeEvalDraft` — creates conversation + eval_plan
  draft with snapshot content.
- `TestAgentTryoutPromoteRejectsUnsupportedTarget` — bad target →
  ErrAgentTryoutPromotionTargetUnsupported.
- `TestAgentTryoutPromoteRejectsAnonymousSource` — anonymous source → sign-in-required.
- `TestValidateTryoutModelPolicy` — table-driven: valid/invalid shapes, unknown providers.
- Handler-level: rerun/compare/promote return correct HTTP status codes for each error.

## Integration / Functional Tests (repository, DB-gated)
- `TestRepositoryCreateAgentTryoutWithParent` — create a tryout with
  `ParentTryoutID` set; round-trips and `GetAgentTryoutByID` returns it.
- (Rerun creating a linked tryout is exercised at the manager level with the fake
  repo asserting parent linkage + dispatch; full DB round-trip of parent linkage
  is covered by the repo test above.)

## Smoke Tests
- `go build ./...`, `go vet ./...`, `go test -short -race ./internal/api/... ./internal/repository/...` pass.
- `sqlc generate` produces no unexpected diff beyond the new query/column.

## E2E Tests
- N/A for this backend PR — covered by the frontend conversion flows separately.

## Manual / cURL Tests (against a local stack)
```bash
# Rerun with a different model policy
curl -sX POST "$API/v1/agent-tryouts/$TRYOUT/rerun" -H "Authorization: Bearer $JWT" \
  -d '{"selected_model_policy":{"mode":"hosted_default","max_models":1}}'
# Compare 2 tryouts
curl -sX POST "$API/v1/workspaces/$WS/agent-tryouts/compare" -H "Authorization: Bearer $JWT" \
  -d '{"tryout_ids":["'$T1'","'$T2'"]}'
# Promote to a Vibe Eval draft
curl -sX POST "$API/v1/agent-tryouts/$TRYOUT/promote-to-eval" -H "Authorization: Bearer $JWT" \
  -d '{"target":"vibe_eval"}'
# Anonymous (no auth) → sign-in-required
curl -sX POST "$API/v1/agent-tryouts/$TRYOUT/rerun" -d '{"selected_model_policy":{}}'
```

## OpenAPI
- New paths: rerun, compare, promote-to-eval, with request/response schemas and
  the new error responses (sign-in-required, model unavailable, unsupported target,
  compare cardinality).
