# State machines

This page lists every status lifecycle in AgentClash, where its rules live, and
the known gaps. Read it before you add a status, write a status, or make a
client that polls a status.

## The rule: one owner per lifecycle

Each lifecycle has one owner: a Go transition table. Everything else reads that
table.

- The **server** enforces the table. A write checks `CanTransitionTo` in Go and
  guards the old status in SQL (`WHERE status = @from_status`). If the guard
  matches no row, the repository returns `ErrInvalidTransition`, and the API
  returns HTTP 409.
- The **CLI** publishes the table through `agentclash schema --json`
  (`status_enums`). The registry comes from `domain.Lifecycles()`
  (`runtime/domain/lifecycle.go`), so it cannot drift from the server.
- **Agents and scripts** read `status_enums` to learn the states, the terminal
  states, and the legal next states. They do not keep their own list.
- The **web app** reads the tables from
  `web/src/lib/generated/status-lifecycles.json`. A golden test in
  `runtime/domain` generates this file from `domain.Lifecycles()`, so it cannot
  drift from the server. `web/src/lib/status-lifecycles.ts` imports the file
  and gives typed helpers: `states`, `terminalStates`, `nonTerminalStates`,
  `isTerminal`, and `nextStatuses`. Type checking fails when a status union in
  `web/src/lib/api/types.ts` does not match the file. Feature modules such as
  `web/src/lib/run-status.ts` build their sets from these helpers. Vibe
  operations are not in the file; their set stays in `web/src/lib/vibe.ts`.
  Components import the sets and do not write inline arrays.

## Explicit lifecycles

These lifecycles have a transition table, a guard on the old status, and
tests.

| Resource | Owner | Terminal states | Published in `agentclash schema` |
|---|---|---|---|
| Run | `runtime/domain/run.go` (`runTransitions`) | completed, failed, cancelled | yes, `run` |
| Run agent | `runtime/domain/run.go` (`runAgentTransitions`) | completed, failed | yes, `run_agent` |
| Eval session | `runtime/domain/eval_session.go` | completed, failed, cancelled | yes, `eval_session` |
| Eval set | `runtime/domain/eval_set.go` | completed, failed, cancelled, budget_exceeded | yes, `eval_set` |
| Regression suite | `runtime/domain/regression.go` | none (archived can go back to active) | yes, `regression_suite` |
| Regression case | `runtime/domain/regression.go` | archived, rejected | yes, `regression_case` |
| Agent harness execution | `backend/internal/repository/agent_harness_executions.go` | completed, failed, cancelled | no |
| Agent tryout | `backend/internal/repository/agent_tryouts.go` | completed, failed, cancelled | no |
| Organization membership | `backend/internal/repository/membership_status.go` | none (re-invite moves archived to invited) | no |
| Workspace membership | `backend/internal/repository/membership_status.go` | none (re-invite moves archived to invited) | no |
| Device auth code | `backend/internal/repository/device_auth_status.go` | denied, expired, consumed | no |
| Billing checkout intent | `backend/internal/repository/billing_checkout_intent_status.go` | completed | no |
| Hosted run execution | `backend/internal/repository/hosted_run_status.go` | completed, failed, timed_out | no |
| Connector dispatch | `backend/internal/repository/connector_dispatch_status.go` | completed, rejected, unknown | no |
| Billing subscription | `backend/internal/billing/subscription_status.go` and the event-time guard in `UpsertBillingSubscription` | none (Dodo owns the transitions) | no |
| Race standings entry (Redis) | `backend/internal/racecontext/types.go` (`StandingsState`) | submitted, failed, timed_out | no |
| Vibe operation | `backend/internal/vibe/types.go` (`CanTransition`, `terminalStates`) | COMPLETED, PARTIAL, FAILED, CANCELLED, EXPIRED | no |

The tables in `runtime/domain` are published to the CLI and the web app. The
tables in the backend module are not published, because no client polls them
through a shared list.

Facts that the tables do not show:

- Run agents have no cancelled status. A cancelled run leaves its agents in
  their last status. To know when a run ends, follow the run status.
- `budget_exceeded` ends an eval set before it finishes. The CLI exits with
  code 1 for it.
- A billing checkout intent expires 24 hours after `created_at`. This is the
  default lifetime of a Dodo checkout session. An hourly worker sweep writes
  `expired`. `WORKER_BILLING_CHECKOUT_INTENT_REAPER_INTERVAL=0` stops the
  sweep.
- A late payment still completes an expired checkout intent. `completed`
  means that a verified Dodo webhook named the checkout. It does not prove a
  payment.
- A repeat complete of a checkout intent is a no-op, not a 409. A 409 would
  roll back the subscription and entitlement writes of the same webhook.
- A hosted run execution in `unknown` has an outcome that nobody knows. An
  owner must reconcile it. A callback that arrives after the wait ended is
  kept as evidence and does not change the status.
- When a callback and the deadline timer of a hosted run race, the first write
  to the execution row wins. The run agent follows the status of that row.
- A connector call or a hosted start that fails before the send, or that gets
  a definite refusal (for example HTTP 401), fails at once with its real
  reason. Other failures, for example HTTP 409, 5xx or a lost response, stay
  `unknown`. `accepted` is in the CHECK constraint, but a synchronous call has
  no accept phase, so nothing writes it.
- A race standings entry becomes `submitted` on `system.output.finalized` or
  on `system.run.completed`. Native runs emit only the second event.
- Three paths do not send their events through the standings recorder of the
  worker. The API stores hosted events. A connector stores its events in one
  transaction. A case fan-out run records no `system.run.*` event for a case.
  For these paths, the run agent workflow writes the standings entry through
  `workflow.update_race_standings`. No persisted event changes. The write is
  best effort: a failure only logs a warning, and a cancel writes nothing. A
  case fan-out agent is `submitted` if one or more cases passed.
- If the worker of a public tryout stops, the workflow marks the tryout
  failed. The web app then stops its wait. The activity sends a heartbeat
  each 10 seconds, so a crash shows in about 60 seconds.
- Vibe operations keep AWAITING_INPUT in the CHECK constraint, but
  `CanTransition` refuses it. The question flow uses the build phase
  `clarifying`.
- Vibe SQL gets the terminal operation states from `terminalStates()` as a
  `text[]` parameter. Do not write the terminal states as a list in SQL.
- The vibe build quota (the cycle total and the guest trial) counts a terminal
  operation, also a PARTIAL one, at its `model_calls`. It counts each other
  operation at the calls that it reserved (`input.calls`).
- Membership changes cascade from the organization to its workspaces in one
  transaction. A workspace membership can become active only while the
  organization membership is active or invited.
- A billing subscription has no transition table, because Dodo owns its
  transitions. The webhook stores the status that Dodo sends. The guard on
  the old state is the Dodo event time: an older event does not change the
  row.
- A Dodo event with no time can insert a missing subscription row, but it
  does not change a row. The webhook stores the event and logs a warning. If
  the event also has no subscription ID, it does not change the
  entitlements.
- Only a subscription payload gives the subscription status. For a payment
  or dunning event, the event type gives the status. For example,
  `payment.succeeded` gives `active`.
- A Dodo status that is not in the set becomes `unknown`. The webhook logs a
  warning and keeps the current entitlements. `renewed` becomes `active`, and
  `canceled` becomes `cancelled`.

## Guarded lifecycles

These columns have no transition table, but each write guards the old status
or a lease.

| Column | Guard |
|---|---|
| `multi_turn_human_turns.status` (awaiting, submitted, expired) | Submit and expire guard on awaiting. The upsert does not change a submitted turn. |
| `workspace_arena_tasks.status` (pending, completed) | The completing UPDATE guards on pending. A late vote returns 409. |
| `dataset_generation_jobs.status` (queued, running, completed, failed) | completed and failed are final. A failed workflow start marks the job failed. |
| `organization_github_installations.status` (active, suspended, deleted) | deleted is final. |
| `agent_tryout_turns.status` (pending, processing, done) | A claim takes a 20-minute lease. A later claim takes a turn whose lease ran out. Only a processing turn becomes done. |
| `challenge_pack_drafts.status` (draft, published, discarded) | The publish transaction writes published. |
| `dataset_examples.status` (active, archived, muted) | An import or a promotion with a default status keeps an archived or muted example. |
| `vibe_enquiries.status` | The review sweep skips a send with a live lease. A lost acceptance keeps its provider ID. |
| `vibe_inputs.status` | Lease guards on extraction. The failure sweep runs before every claim. An expired input stays expired on delete. |
| Vibe conversation questions (JSONB) | Only an active question becomes answered. |
| Vibe build phase (`Document.build.phase`, JSONB) | An expired or stopped operation ends the build phase. |

## Open gaps

These gaps remain. Each needs an owner decision or a larger change.

| Area | Gap |
|---|---|
| `vibe_operations.billing` | A later cost conflict moves a settled operation back to RECONCILING and freezes the account for review. Confirm that this is the intended review flow. |
| `vibe_attempts.state` | A worker crash can leave an attempt in DISPATCHING. RECONCILED has two meanings. |
| `challenge_pack_drafts.status` | A published draft can change and publish again. A test does this on purpose. Decide if published must be final. |
| Public tryout | A message that arrives after the turn loop ends, but before the completed write, is accepted and never processed. |

The money paths (billing, `vibe_attempts`, the build quota) need an owner
decision before a change. Do not add a guard there as a side change.

## Add or change a lifecycle

1. Put the states and the transition table in `runtime/domain`, next to the
   existing tables.
2. Add the state order list and a `describeLifecycle` line in
   `runtime/domain/lifecycle.go`.
3. Make the CHECK constraint in the migration name the same states.
4. Write each status change through one repository function that checks
   `CanTransitionTo` and guards the old status in SQL.
5. Map `ErrInvalidTransition` to HTTP 409 in the API, and document the 409 in
   `docs/api-server/openapi.yaml`.
6. Regenerate the CLI schema snapshot:
   `cd cli && go test ./cmd -run TestSchemaJSONMatchesGoldenSnapshot -update`.
   Regenerate the web lifecycles:
   `cd runtime && go test ./domain -run TestWebStatusLifecyclesMatchDomain -update`.
7. In the web app, update the status union in `web/src/lib/api/types.ts` and
   the union records in `web/src/lib/status-lifecycles.test.ts`. Read status
   sets from `web/src/lib/status-lifecycles.ts` (or the module of the feature,
   for example `web/src/lib/run-status.ts`). Type each status map as
   `Record<Status, ...>`, so the compiler finds a missing status.
8. Add the lifecycle to the explicit table above, and remove it from the
   guarded table or the open gaps.
