# Vibe Eval Backend Design: Guide Agent, Tool Registry, Confirmations, Audit, and Credit Ledger

Status: draft for #875
Parent: #753
Depends on: #868 (Phase 0 inventory — `docs/vibe-eval-phase-0-inventory.md`)

This document is the authoritative backend design for the Vibe Eval **guide agent**: the
LLM-driven agent that turns a plain-English product description into a real AgentClash
evaluation loop (plan → challenge pack → validate → publish → run → analyze → regress).
It must be agreed before implementation spreads across the API server, worker, policy,
and billing code.

It specifies, in order:

1. What exists today and what this design adds.
2. Orchestration boundaries and trusted/untrusted boundaries.
3. LLM and tool-calling integration (reuse vs. new).
4. The typed tool registry and phase-based tool loading.
5. Policy authorization, the confirmation engine, payload hashing, and idempotency.
6. The audit schema.
7. Redaction and untrusted-tool-output handling.
8. The credit wallet: reservation/settlement APIs and invariants.
9. Threat model.
10. Implementation sequencing.
11. Resolved decisions.

## 1. Principles (carried from Phase 0)

These are non-negotiable and constrain every section below.

- Vibe Eval exposes **narrow semantic tools** backed by existing AgentClash services.
  It never exposes a generic shell, a raw HTTP client, or a "call any endpoint" tool.
- Browser input, user text, generated packs, replay text, artifact previews, and tool
  outputs are **untrusted**. Any tool output reused in model context is wrapped as
  *evidence*, never as instructions.
- The Go backend policy layer is **authoritative** for authz, confirmations,
  idempotency, audit, and credit reservation. The model proposes; the backend disposes.
- Mutating, cost-incurring, admin-sensitive, destructive, and public-sharing tools
  require an explicit confirmation with a **payload hash**.
- AgentClash-owned provider keys stay server-side only. Vibe Eval may list
  provider/secret *metadata*, but never returns secret values to the browser or to model
  context.

## 2. What exists today vs. what this design adds

### Already on `main`
- **Conversation + draft persistence** (Phase 1): `vibe_eval_conversations` and
  `vibe_eval_drafts` tables (migration `00049_vibe_eval_drafts.sql`), CRUD service and
  handlers in `backend/internal/api/vibe_eval.go`, repository in
  `backend/internal/repository/vibe_eval.go`. Drafts are typed JSON blobs
  (`eval_plan | challenge_pack | input_cases | scoring | runtime`) with a
  `validation_state`. `ActionManageVibeEvalDrafts` exists in
  `backend/internal/api/permissions.go` (member+).
- **Reusable execution substrate**: `provider.Router` / `provider.Client`
  (`backend/internal/provider/provider.go`) with normalized `Message`, `ToolCall`,
  `ToolDefinition`, `Usage`; the agentic loop in `backend/internal/engine`; the
  budget reserve/record model in `backend/internal/budget`; the SSE + Redis live-event
  pipeline (`backend/internal/pubsub`, `run_events_sse.go`).

### In flight (review-only; `origin/codex/vibe-eval-phase-2-validate-publish`, not merged)
- `ValidateDraft` / `PublishDraft` service methods, a nascent
  `VibeEvalConfirmationRequiredError`, a `vibeEvalPayloadHash` helper, and a
  `vibe_eval_draft_events` audit table (`action`, `payload_hash`, `request_payload`,
  `result_payload`).
- **Integration note:** that branch numbers the audit migration
  `00050_vibe_eval_draft_events.sql`, which **collides** with `00050_datasets.sql` on
  `main`. It must be renumbered to the next free slot (≥ `00056`) on rebase. This design
  treats that table as the *seed* of the general audit log (section 6) and recommends
  generalizing it rather than adding a parallel table.

### What this design adds (the agent)
- A **guide-agent orchestrator**: a bounded LLM tool-calling loop that drives a
  conversation, calls semantic tools, and streams progress to the browser.
- **Conversation messages** persistence (turns + tool calls/results), which Phase 1 does
  not have — today only conversations and drafts are stored, not the dialogue.
- A **typed tool registry** with phase-based loading and per-tool policy metadata.
- A **confirmation engine** (propose → confirm with payload hash) generalized from the
  Phase 2 seed.
- A general **tool-invocation audit log** generalized from `vibe_eval_draft_events`.
- An **eval credit wallet** (reserve → settle ledger) for AgentClash-managed `$3`
  execution credit, separate from billing entitlements.
- A **redaction/evidence wrapper** for tool outputs entering model context.

## 3. Orchestration boundaries

### 3.1 Where the agent runs

The guide agent is a **control-plane** concern, not a Temporal workflow. Rationale:

- Its work is short, interactive, and request-scoped (a conversation turn), unlike the
  long-running, durable run execution that Temporal owns (per `architecture.md` §7.3).
- It calls existing API-layer managers/services synchronously and must enforce policy,
  confirmations, and credit holds in the request path.
- The *expensive, durable* things it triggers — runs, eval sessions — already go through
  Temporal via the existing `POST /runs` / `POST /eval-sessions` managers. The agent is a
  client of those, not a replacement.

Therefore the agent loop lives in **`backend/internal/vibeeval`** (decided — §11.1; it must
not import `api`), with the HTTP handler + tool adapters in `backend/internal/api`, invoked
by a new turn endpoint (section 3.3). A single agent turn may itself make
several model calls (tool-calling loop) but is bounded (max steps, max tool calls, wall
clock, and credit/allowance ceiling).

### 3.2 Trusted vs. untrusted boundaries

| Zone | Trust | Rule |
| --- | --- | --- |
| User chat text | untrusted | Treated as a request, never as authorization. Cannot widen scope. |
| Model output (assistant text, tool-call args) | untrusted | Tool args are validated against the tool's JSON schema and policy before execution. |
| Tool *inputs* the backend constructs | trusted | Built by Go code from validated args + caller identity. |
| Tool *outputs* re-entering model context | untrusted | Wrapped as evidence (section 7); secrets/PII redacted. |
| Policy layer (authz, confirmation, credit, audit) | authoritative | Runs in Go, keyed off the authenticated `Caller`, never off model claims. |
| AgentClash provider keys / workspace secrets | server-only | Never serialized to browser, model context, logs, or traces. |

The model can only *request* actions. Every action is re-authorized in Go against the
`Caller` (section 5), independent of anything the model said.

### 3.3 API surface (proposed)

Reuse the existing workspace-scoped, data-aware-authz pattern. New routes under the
existing `/v1/workspaces/{workspaceID}/vibe-eval/...` group:

| Method | Path | Purpose | Risk |
| --- | --- | --- | --- |
| `POST` | `/conversations/{conversationID}/turns` | Submit a user message; runs one bounded agent turn; streams events (SSE) | draft/cost-incurring depending on tools used |
| `GET` | `/conversations/{conversationID}/messages` | List persisted turns + tool calls/results | read |
| `POST` | `/conversations/{conversationID}/confirmations/{confirmationID}` | Approve/deny a pending confirmation by payload hash; resumes the turn | matches deferred action |

The turn endpoint streams over the existing SSE substrate (`run_events_sse.go` style) so
the browser sees token deltas, tool-call cards, confirmation cards, and tool results
live. (LangGraph-style interrupt/resume is the reference pattern for the confirmation
pause — see #753 research notes.)

## 4. LLM and tool-calling integration

### 4.1 Reuse decision

**Reuse** `provider.Router` and the normalized `provider.{Request,Response,Message,
ToolCall,ToolDefinition,Usage}` types. **Do not reuse** `internal/engine`'s loop directly:
the engine is built for sandboxed challenge execution (its tools are `exec`, `read_file`,
`submit` in E2B; it has step/sandbox semantics the guide agent does not want). The guide
agent gets its **own small loop** in `vibeeval`, sharing only the provider abstraction.

Shared (reused):
- `provider.Router.InvokeModel(ctx, Request) (Response, error)`
- Message/tool-call normalization across OpenAI/Anthropic/Gemini/xAI/OpenRouter/Mistral.
- Failure classification (`provider.Failure` / `FailureCode`).

New (guide-agent-specific):
- The loop, the semantic tool registry, confirmation/audit/credit/redaction plumbing.

### 4.2 The guide-agent loop (proposed)

```go
// package vibeeval

type AgentLoop struct {
    router    provider.Router      // reused
    registry  ToolRegistry         // section 4.3
    policy    ToolPolicyEnforcer   // section 5
    confirm   ConfirmationEngine   // section 5.3
    audit     AuditWriter          // section 6
    redactor  EvidenceRedactor     // section 7
    wallet    CreditWallet         // section 8
    messages  MessageStore         // conversation turn persistence
    limits    AgentLimits          // max steps, max tool calls, wall clock, allowance
}

type TurnResult struct {
    AssistantText      string
    ToolInvocations    []ToolInvocationRecord
    PendingConfirmation *PendingConfirmation // non-nil => loop paused awaiting approval
    StopReason         string                // completed | awaiting_confirmation | limit | error
    Usage              provider.Usage
}

func (l *AgentLoop) RunTurn(ctx context.Context, caller Caller, conv Conversation, userMessage string) (TurnResult, error)
```

Loop sketch (bounded; deterministic control flow in Go, not model-driven):

1. Load conversation history from `MessageStore`; append the user message.
2. Build `provider.Request`: system prompt (phase-aware), history, and the
   `ToolDefinition`s for tools loaded for the conversation's current phase (section 4.3).
3. Charge the model call against the **guide-agent allowance** (separate from eval credit;
   section 8). Enforce per-turn step/tool/wall-clock limits.
4. `router.InvokeModel`. Persist the assistant message.
5. For each tool call the model emits:
   a. Resolve the tool in the registry; reject unknown/not-loaded tools.
   b. Validate arguments against the tool's JSON schema.
   c. `policy.Authorize(caller, tool, args)` — re-authorize in Go (section 5).
   d. If the tool's risk tier requires confirmation: compute the payload hash, persist a
      `PendingConfirmation`, emit a confirmation card, and **stop the turn**
      (`awaiting_confirmation`). The turn resumes via the confirmations endpoint.
   e. If cost-incurring: reserve credit (section 8) before execution.
   f. Execute the tool (calls an existing manager/service). Wrap the output as evidence
      (section 7). Write an audit record (section 6).
   g. Append the tool result to history.
6. Repeat from step 2 until the model returns no tool calls, or a limit is hit.
7. Return `TurnResult`; persist final state.

### 4.3 Typed tool registry and phase-based loading

Tools are static, typed, and registered at startup. The model only ever sees the
`ToolDefinition`s for the conversation's **current phase**, which caps agency and keeps
prompts small.

```go
// package vibeeval

type RiskTier string // read | draft | workspace_write | cost_incurring | admin_sensitive | destructive_external

type Tool interface {
    Name() string
    Phases() []string          // plan|author|validate|publish|run|analyze|regress|admin
    RiskTier() RiskTier
    RequiredAction() api.Action // re-authorized in Go before execution
    Definition() provider.ToolDefinition // name + description + JSON-schema params
    // Execute runs the semantic action against an existing manager/service.
    // args are already schema-validated and policy-authorized by the loop.
    Execute(ctx context.Context, caller Caller, conv Conversation, args json.RawMessage) (ToolOutput, error)
}

type ToolOutput struct {
    Result        any           // wrapped as evidence before re-entering model context
    AuditResult   map[string]any // metadata-only summary for the audit log (no secrets/contents)
    CostEstimate  *CostEstimate  // set by cost_incurring tools at propose time
    DraftMutation *DraftRef      // set when a tool writes a vibe_eval_draft
}

type ToolRegistry interface {
    Register(t Tool)
    // ForPhase returns the tools loaded for a conversation phase, in stable order.
    ForPhase(phase string) []Tool
    Resolve(name string) (Tool, bool)
    // Definitions returns provider.ToolDefinition for ForPhase(phase) to send to the model.
    Definitions(phase string) []provider.ToolDefinition
}
```

The concrete tools are the Phase 0 tool map rows. Each is a thin adapter over an existing
manager (e.g. `list_challenge_packs` → `ChallengePackReadManager`, `create_run` →
`RunCreationManager`, `publish_challenge_pack` → `ChallengePackAuthoringManager`). Tools
add **no** new business logic or data access of their own — they translate validated args
into existing service calls, which keeps the policy/authz surface unchanged.

### 4.4 Conversation messages

Phase 1 persists conversations and drafts but **not the dialogue**. Add a
`vibe_eval_messages` table (proposed, next free migration ≥ `00056`):

```
vibe_eval_messages (
  id, organization_id, workspace_id, conversation_id,
  seq               bigint,        -- monotonic per conversation
  role              text,          -- user | assistant | tool
  content           text,          -- assistant/user text; tool result content (redacted)
  tool_call_id      text,          -- for role=tool
  tool_name         text,
  tool_args         jsonb,         -- for assistant tool calls (validated, redacted)
  usage             jsonb,         -- token usage for assistant turns
  created_at        timestamptz
)
```

The browser reconstructs the workbench from messages + drafts. Drafts remain the durable
artifacts; messages are the transcript.

## 5. Policy, confirmation, idempotency

### 5.1 Authorization — reuse, do not reinvent

Every tool declares a required `api.Action`. Execution calls the existing
`AuthorizeWorkspaceAction(ctx, authorizer, caller, workspaceID, action)`
(`backend/internal/api/permissions.go`), which already encodes the role matrix and the
org_admin override. The model's request never bypasses this.

Tool → action mapping comes from the Phase 0 matrix. Reuse existing actions wherever
possible. Two new actions are required (and only these two are new):

| New action | Floor | Used by |
| --- | --- | --- |
| `ActionManageEvalCredit` | admin | credit-wallet administration (manual grants/adjustments) |
| `ActionManagePublicShares` | member | `create_share_link` (Phase 6) |

`ActionManageVibeEvalDrafts` already exists and covers draft/validate writes. Add both new
constants to `permissions.go` and to `permissionMatrix` in the same change that first
exposes a tool needing them — not before.

### 5.2 Risk tiers (from Phase 0) drive confirmation and idempotency

| Tier | Confirmation | Idempotency scope | Credit |
| --- | --- | --- | --- |
| `read` | no | request | none |
| `draft` | no | conversation | none (allowance only) |
| `workspace_write` | yes | workspace/conversation | none |
| `cost_incurring` | yes, with estimate | run / eval session | reserve → settle |
| `admin_sensitive` | yes, high friction | workspace/org | none |
| `destructive_external` | yes, high friction | resource | none |

Each `Tool` carries its tier as static metadata (section 4.3 registry), so the loop can
decide confirmation/credit behavior generically.

### 5.3 Confirmation engine (propose → confirm)

Generalize the Phase 2 `VibeEvalConfirmationRequiredError` + `vibeEvalPayloadHash` into a
reusable two-phase protocol:

```go
type PendingConfirmation struct {
    ID            uuid.UUID
    ConversationID uuid.UUID
    ToolName      string
    Action        api.Action
    RiskTier      RiskTier
    PayloadHash   string          // sha256 of canonical-JSON(tool, normalized args)
    Summary       string          // human-readable card body (what will happen)
    Estimate      *CostEstimate   // for cost_incurring
    ExpiresAt     time.Time
}

type ConfirmationEngine interface {
    Require(ctx, conv Conversation, tool Tool, args json.RawMessage, est *CostEstimate) (PendingConfirmation, error)
    Resolve(ctx, caller Caller, confirmationID uuid.UUID, approve bool, presentedHash string) error
}
```

Rules:
- The payload hash binds the confirmation to *exactly* the args presented to the user.
  On resolve, the client echoes the hash; a mismatch (model re-proposed different args)
  rejects the confirmation. This blocks bait-and-switch.
- Confirmations are single-use and expire.
- Approving resumes the turn and executes the bound action with the bound args.
- `admin_sensitive` / `destructive_external` use high-friction confirmation (e.g. typed
  resource name), not a single click.

### 5.4 Idempotency

Mutating and cost-incurring tools take an idempotency key scoped per the tier table.
Reuse existing idempotency where the underlying manager already has it (e.g. run
creation). For new writes, the key is `(conversation_id, tool, payload_hash)` so a
resumed/retried turn cannot double-publish or double-charge.

## 6. Audit schema

Generalize `vibe_eval_draft_events` (Phase 2, validate/publish only) into a single
**tool-invocation audit log** covering every non-trivial tool call:

```
vibe_eval_tool_invocations (
  id, organization_id, workspace_id, conversation_id,
  message_id          uuid,          -- links to the assistant tool-call message
  actor_user_id       uuid,          -- the authenticated Caller, never the model
  tool_name           text,
  action              text,          -- api.Action enforced
  risk_tier           text,
  payload_hash        text,
  confirmation_id     uuid null,     -- set for confirmed tiers
  request_payload     jsonb,         -- validated args (redacted)
  result_payload      jsonb,         -- outcome metadata (redacted; never secrets/contents)
  credit_reservation_id uuid null,   -- set for cost_incurring
  outcome             text,          -- ok | denied | error | confirmation_required
  created_at          timestamptz
)
```

- Every `draft`+ tier tool call writes one row, regardless of success.
- `read`-tier calls are audited only when they touch sensitive evidence (Phase 0 rule).
- Never log secret values, raw artifact contents, or provider keys — metadata and hashes
  only.
- The existing `vibe_eval_draft_events` data, if any exists pre-merge, migrates into this
  table; otherwise the Phase 2 branch should be re-pointed at this table on rebase to
  avoid two parallel audit logs.

## 7. Redaction and untrusted-tool-output handling

Tool outputs that re-enter model context pass through an `EvidenceRedactor`:

```go
type EvidenceRedactor interface {
    // Wrap renders tool output as clearly-delimited, untrusted evidence and
    // strips secrets/PII. The returned string is safe to place in model context.
    Wrap(toolName string, raw any) (evidence string, err error)
}
```

Rules:
- Output is wrapped in explicit BEGIN/END EVIDENCE delimiters with a notice that content
  inside is data, not instructions (mirrors the anti-injection envelope the judge prompts
  already use in `workflow/judges.go`).
- Known secret shapes (provider keys, workspace secret values, signed URLs) are redacted
  before wrapping; secret *metadata* (names, providers) may pass.
- Replay text, artifact previews, and run outputs are truncated and summarized; large
  payloads are referenced by id, not inlined.
- Browser-facing renders and model-context renders use the same redaction so nothing
  sensitive leaks to either surface.

## 8. Credit wallet (reserve → settle)

`#753` requires every new org to receive `$3` of **AgentClash-managed eval execution
credit**. Phase 0 confirmed billing entitlement tables (`00031_*`, `00036_*`) track
plan/trial state but are **not** a reserve-then-settle wallet. This is a new subsystem,
modeled on the existing `budget` reserve/record shape
(`backend/internal/budget/budget.go`) but with an immutable ledger.

### 8.1 Tables (proposed, next free migrations ≥ `00056`)

```
org_eval_credit_wallets (
  organization_id   uuid pk,
  currency_code     text,            -- 'USD'
  granted_micros    bigint,          -- total granted (e.g. $3 = 3_000_000 micros)
  reserved_micros   bigint,          -- currently held by open reservations
  settled_micros    bigint,          -- consumed by settled runs
  updated_at        timestamptz
)
-- available = granted - reserved - settled  (must always be >= 0)

org_eval_credit_ledger (              -- immutable, append-only
  id, organization_id,
  entry_type        text,            -- grant | reserve | settle | release | adjust
  amount_micros     bigint,          -- signed
  reservation_id    uuid null,       -- groups reserve/settle/release
  run_id            uuid null,
  eval_session_id   uuid null,
  reason            text,
  actor_user_id     uuid null,       -- null for system grants
  created_at        timestamptz
)
```

### 8.2 API (proposed)

```go
type CreditWallet interface {
    // Reserve holds estimated spend before a run/eval session starts.
    // Idempotent on reservationKey. Fails if available < amount and not BYOK.
    Reserve(ctx, orgID uuid.UUID, reservationKey string, micros int64, ref CreditRef) (Reservation, error)
    // Settle converts a reservation to actual spend (<= reserved); releases the remainder.
    Settle(ctx, reservationID uuid.UUID, actualMicros int64) error
    // Release cancels a reservation in full (run never started / cancelled before spend).
    Release(ctx, reservationID uuid.UUID) error
    // Grant adds credit (seeding, admin). Idempotent on grantKey.
    Grant(ctx, orgID uuid.UUID, grantKey string, micros int64, reason string) error
    Balance(ctx, orgID uuid.UUID) (WalletBalance, error)
}
```

### 8.3 Invariants (must be encoded as tests — #875 acceptance)

1. `available = granted - reserved - settled` is **never negative**.
2. `Reserve` is **idempotent** on `reservationKey`: a retry returns the same reservation,
   never double-holds.
3. `Settle` releases unused reserve: after settle, `reserved` drops by the full
   reservation and `settled` rises by the actual (≤ reserved) amount.
4. `Release` returns the full hold; no settle may follow a release (and vice-versa).
5. **BYOK does not consume included credit**: runs using a workspace's own provider
   account/secret bypass the wallet entirely (no reserve/settle).
6. The ledger is append-only and reconstructs the wallet totals exactly
   (sum of ledger entries == wallet columns).

### 8.4 Seeding (the `$3` gap from Phase 0)

`$3` is granted on org creation. Both current creation paths must call `Grant` with a
deterministic `grantKey` (so re-runs/imports don't double-grant):

- `POST /onboarding` → `OnboardingManager.Onboard` → repository `Onboard`.
- `POST /organizations` → `OrganizationManager.CreateOrganization` →
  `CreateOrganizationWithAdmin`.

A backfill migration grants existing orgs once. Any future org-creation helper (fixtures,
admin seeds, imports) must either seed or explicitly opt out — enforced by a test that
fails if an org exists without a wallet row (unless flagged).

### 8.5 Loop integration

- A `cost_incurring` tool (`create_run`, `create_eval_session`) first produces a
  `CostEstimate`, shown in the confirmation card.
- On approval: `Reserve` against the org wallet (or skip for BYOK) using the run/session
  id as `reservationKey`, then create the run via the existing manager.
- After the run completes, the existing post-run cost path (which already computes actual
  cost via `budget.RecordRunCost` and model pricing) calls `Settle` with actual spend.
- The guide-agent's own model calls are charged against a **separate internal
  allowance**, not the eval credit wallet (Phase 0: "guide-agent usage has a separate
  internal allowance"). Keep these two budgets distinct in code and audit.

## 9. Threat model (OWASP LLM/MCP top risks → mitigation)

| Risk | Mitigation in this design |
| --- | --- |
| Prompt injection via user/tool text | Tool outputs wrapped as evidence (section 7); model claims never authorize; all actions re-authorized in Go. |
| Excessive agency | No generic shell/HTTP; only registered semantic tools; per-phase tool loading; bounded loop. |
| Secret/token exposure | Provider keys & secret values never leave the server; redaction before model/browser; audit logs hashes/metadata only. |
| Tool poisoning / scope creep | Static tool registry with fixed schemas + required actions; args validated; confirmations bound by payload hash. |
| Unauthorized cost | `cost_incurring` requires confirmation with estimate + credit reservation; wallet invariants prevent overspend. |
| Lack of audit/telemetry | Every `draft`+ tool call audited; confirmations and credit ledger immutable. |
| Bait-and-switch on confirm | Payload-hash binding; mismatched re-proposal is rejected on resolve. |

## 10. Implementation sequencing

Each is a reviewable PR; later PRs depend on earlier interfaces.

1. **This design doc** (#875) — interfaces + schema agreed. (no runtime code)
2. **Tool registry + agent loop skeleton + messages table + read-only tools**
   (`list_challenge_packs`, `get_run_status`, `read_scorecard`). No mutations, no credit.
   Proves the loop, streaming, redaction, and audit end-to-end.
3. **Confirmation engine + audit generalization**; rebase/absorb the Phase 2
   validate/publish branch onto the generalized audit table; add `draft` author tools.
4. **Credit wallet + seeding + backfill**; wire `cost_incurring` tools
   (`estimate_eval_cost`, `create_run`, `create_eval_session`).
5. **Analyze/regress tools** (`read_replay_summary`, `compare_runs`,
   `promote_failure_to_regression`, `create_ranking_insights`).
6. **Admin-sensitive + public-share tools** (gated behind `ActionManagePublicShares`,
   high-friction confirmation).

The workbench UI (#874) consumes steps 2+ incrementally.

## 11. Resolved decisions

All six open decisions were resolved via a code-grounded Claude↔Codex review
(full record + citations in `discussion/agreed-direction-2026-06-09.md` and the per-question
`discussion/qN-*.md` threads).

1. **Package placement — DECIDED.** Core in **`backend/internal/vibeeval`** (loop, registry,
   reader/redactor/audit/message-store interfaces; **must not import `api`**). HTTP handler +
   concrete tool adapters in `backend/internal/api`. Identity via an adapter shim:
   `vibeeval.Actor{UserID}` + `vibeeval.WorkspaceAuthorizer.Authorize(ctx, workspaceID,
   action string)` bridged per-turn to `AuthorizeWorkspaceAction`; action constants stay in
   `api`; `api` validates each tool's action string at registry construction. No
   `internal/authz` package yet.
2. **Turn execution — DECIDED.** Synchronous, request-scoped, **SSE-streamed**, bounded; not
   a Temporal workflow. Cost-incurring tools start the existing Temporal run, emit the
   reference, and the turn ends (browser follows via existing run SSE). Confirmation =
   **end + resume** (`POST /confirmations/{id}` streams a continuation turn). SSE, not
   WebSocket (`architecture.md:30`'s "WebSockets" is stale vs. the implemented path).
3. **Allowance accounting — DECIDED.** A **monthly quota counter, not a wallet** — extend
   `workspace_usage_windows` with `guide_agent_turn_count`; count **accepted turns**
   per-workspace/month (increment at accept-time); limit in `EffectiveEntitlements`
   (`GuideAgentTurnsPerWorkspaceMonth`); `CheckGuideAgentAllowance` mirrors `CheckRaceQuota`.
   Tokens recorded observationally, not gated.
4. **Provider/model — DECIDED.** Reuse the api-server `providerRouter`; guide manager depends
   on a **streaming-capable** provider interface. Dedicated server config
   `VIBEEVAL_GUIDE_{PROVIDER_KEY,MODEL,CREDENTIAL_REFERENCE}` (`secret://`/`env://` only;
   reject `workspace-secret://`/BYOK). **Product decisions (user):** default
   `claude-sonnet-4-6`; guide chat is **allowance-metered, never the `$3` eval credit** (runs
   it launches debit the wallet).
5. **Phase 2 reconciliation — DECIDED.** Absorb, don't ship two audit tables: drop
   `vibe_eval_draft_events`; lift validate/publish behavior into the confirmation/audit PR
   writing **`vibe_eval_tool_invocations`** (migration ≥ `00056`); canonical-JSON payload
   hashing; preserve the private-pack entitlement gate on `publish`.
6. **Messages retention/redaction — DECIDED.** Cascade retention (no v1 TTL). user/assistant
   text **verbatim except a narrow known-secret-shape scrub** (high-precision: credentials +
   signed URLs only, never PII), living in a shared `internal/redaction` helper (so
   `vibeeval` doesn't import `engine`). Tool text **redacted only**, via a single redaction
   boundary that fans out to persistence + SSE + model context. Single `content` column +
   `redaction_state`; no dual raw/redacted columns.

## 12. Acceptance checklist for #875

- [x] A checked-in backend design doc exists and links back to #753 and #868.
- [x] The design names authoritative services and trusted boundaries (sections 3, 5, 9).
- [x] Confirmation, audit, redaction, and idempotency behavior is specified before code
      (sections 5, 6, 7).
- [x] Credit ledger invariants are explicit and testable (section 8.3).
- [x] Typed tool registry interfaces and phase-based loading are defined (section 4).
- [x] Reuse vs. new boundaries (provider router reuse; new loop) are stated (section 4.1).
- [x] Implementation sequencing is defined (section 10).
- [x] Open decisions resolved (section 11) via code-grounded Claude↔Codex review —
      record in `discussion/agreed-direction-2026-06-09.md`.
- [ ] Final review/approval by Atharva.
