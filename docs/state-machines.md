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
- The **web app** keeps its status sets in `web/src/lib/run-status.ts` and
  `web/src/lib/vibe.ts`. Components import these sets and do not write inline
  arrays.

## Explicit lifecycles

These lifecycles have a transition table, a SQL guard, and tests.

| Resource | Owner | Terminal states | Published in `agentclash schema` |
|---|---|---|---|
| Run | `runtime/domain/run.go` (`runTransitions`) | completed, failed, cancelled | yes, `run` |
| Run agent | `runtime/domain/run.go` (`runAgentTransitions`) | completed, failed | yes, `run_agent` |
| Eval session | `runtime/domain/eval_session.go` | completed, failed, cancelled | yes, `eval_session` |
| Eval set | `runtime/domain/eval_set.go` | completed, failed, cancelled, budget_exceeded | yes, `eval_set` |
| Regression suite | `runtime/domain/regression.go` | none (archived can go back to active) | yes, `regression_suite` |
| Regression case | `runtime/domain/regression.go` | archived, rejected | yes, `regression_case` |
| Agent harness execution | `backend/internal/repository/agent_harness_executions.go` | completed, failed, cancelled | no (the table is in the backend module) |
| Vibe operation | `backend/internal/vibe/types.go` (`CanTransition`) | COMPLETED, PARTIAL, FAILED, CANCELLED, EXPIRED | no |

Two facts that the tables do not show:

- Run agents have no cancelled status. A cancelled run leaves its agents in
  their last status. To know when a run ends, follow the run status.
- `budget_exceeded` ends an eval set before it finishes. The CLI exits with
  code 1 for it.

## Implicit lifecycles

These columns change over time, but no transition table owns them. The writes
are scattered, and some writes have no guard on the old status. The list comes
from a code audit on 2026-11-10.

| Column | States | Guard on the old status | Known gaps |
|---|---|---|---|
| `agent_tryouts.status`, `redaction_status` | queued, running, completed, failed, cancelled | none in `UpdateAgentTryoutStatus` | A terminal tryout can change to another terminal status. A failed tryout gets `not_required` redaction, so it counts as ready to share. |
| `organization_memberships`, `workspace_memberships` `.membership_status` | invited, active, suspended, archived | checked in the API layer only | The UPDATE has no guard, so two requests can race. The cascade writes invited to suspended, which the API table forbids. An invalid transition returns 400, not 409. |
| `multi_turn_human_turns.status` | awaiting, submitted, expired | submit and expire guard on awaiting | The upsert in `MarkAwaitingHuman` resets a submitted turn. |
| `workspace_arena_tasks.status` | pending, completed | SELECT check, no row lock | A vote on a completed task returns 500. |
| `hosted_run_executions.status` | starting, accepted, running, completed, failed, timed_out, unknown | partial | `timed_out` is never written. `unknown` has two meanings. `ApplyHostedRunEvent` has no guard and no callers. |
| `dataset_generation_jobs.status` | queued, running, completed, failed | none | If the workflow does not start, the job stays queued. |
| `organization_github_installations.status` | active, suspended, deleted | none | A webhook that arrives late can change deleted back to active. |
| `challenge_pack_drafts.status` | draft, published, discarded | none | Publish does not check the status. `discarded` is never written. |
| `dataset_examples.status` | active, archived, muted | none | A re-import can bring back an archived example. |
| `billing_subscriptions.status` | values from the payment provider | event-time order only | No CHECK constraint. A missing event time falls back to `now()`. |
| `billing_checkout_intents.status` | created, pending, completed, cancelled, expired | guards on completed | Only completed is written, so an abandoned checkout stays created. |
| `device_auth_codes.status` | pending, approved, denied, expired | guards on pending | No consumed status. |
| `agent_tryout_turns.status` | pending, processing, done | claim with `SKIP LOCKED` | A crash leaves a turn in processing. |
| Race standings (`racecontext.StandingsState`, Redis) | not_started, running, submitted, failed, timed_out | none | A late event can change submitted back to running. |
| `vibe_operations.billing` | UNRESERVED, RESERVED, RECONCILING, SETTLED, RELEASED | callers check that the operation is terminal | A later cost conflict moves a settled operation back to RECONCILING and freezes the account for review. Confirm that this is the intended review flow. |
| `vibe_attempts.state` | DISPATCHING, SUCCEEDED, UNCERTAIN, RECONCILED | partial | A worker crash can leave an attempt in DISPATCHING. RECONCILED has two meanings. |
| `vibe_inputs.status` | uploaded, extracting, ready, unreadable, failed, deleted, expired | lease guards on extraction | Delete overwrites expired. The read path can report expired while the row says otherwise. |
| `vibe_enquiries.status` | received, sending, provider_accepted, needs_review, cancelled | lease guards on send | The sweeper can move a row that another worker sends. |
| Vibe build phase (`Document.build.phase`, JSONB) | preparing, checking, trying, ready, clarifying, waiting, blocked, error, results | partial | No CHECK constraint. The phase names are copied into SQL. |

The money paths (`vibe_operations.billing`, `vibe_attempts`) and access control
(memberships) need an owner decision before a change. Do not add a guard there
as a side change.

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
7. In the web app, import the status sets from `web/src/lib/run-status.ts` (or
   the module of the feature). Type each status map as
   `Record<Status, ...>`, so the compiler finds a missing status.
8. Remove the row of the lifecycle from the implicit table above.
