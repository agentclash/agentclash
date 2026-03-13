# Run Repository Layer

Status: implementation note for PR #3 and issue #2

Purpose: explain what the run-orchestration repository layer now does, why it exists, and which tradeoffs were made so the next workflow slice can build on it cleanly.

## Why This Exists

The architecture and build-order docs already said the database phase should produce a query/repository layer before workflow code starts:

- [`architecture.md`](../../architecture.md)
- [`docs/build-order/build-order.md`](./build-order.md)
- [`docs/database/rule.md`](../database/rule.md)
- [`backend/db/migrations/00006_run_orchestration.sql`](../../backend/db/migrations/00006_run_orchestration.sql)

This PR turns that requirement into actual code.

Before this change, the repository had:

- run schema migrations
- no typed query layer
- no Go constants for run states
- no repository method that owned transition validation
- no atomic guarantee between current-state writes and history writes

That would have pushed too much responsibility into the next Temporal workflow issue.

## What The PR Adds

### 1. `sqlc` query generation for run orchestration tables

Files:

- [`backend/sqlc.yaml`](../../backend/sqlc.yaml)
- [`backend/db/queries/runs.sql`](../../backend/db/queries/runs.sql)
- [`backend/db/queries/run_agents.sql`](../../backend/db/queries/run_agents.sql)
- generated code under [`backend/internal/repository/sqlc`](../../backend/internal/repository/sqlc)

Why:

- keeps SQL explicit and reviewable
- keeps Go call sites typed
- catches schema/query mismatches before runtime
- gives future workflow code a stable query surface

### 2. Typed domain status definitions

File:

- [`backend/internal/domain/run.go`](../../backend/internal/domain/run.go)

Why:

- status strings are now centralized
- callers do not need to scatter raw literals such as `"queued"` or `"executing"`
- allowed transitions are defined in one place

Important behavior:

- `Run` transitions allow the expected forward path plus failure/cancellation from in-progress states
- `RunAgent` transitions allow failure before full execution completes, which matches real worker behavior such as setup or provider failures

### 3. A thin repository that owns run mutation rules

File:

- [`backend/internal/repository/repository.go`](../../backend/internal/repository/repository.go)

Methods added:

- `GetRunByID`
- `ListRunAgentsByRunID`
- `SetRunTemporalIDs`
- `TransitionRunStatus`
- `TransitionRunAgentStatus`
- `InsertRunStatusHistory`
- `InsertRunAgentStatusHistory`

Why:

- workflow code should orchestrate, not invent DB mutation rules
- state transitions are business rules, not just `UPDATE` statements
- the repository is now the single place that decides whether a transition is valid

### 4. Integration tests against Postgres

File:

- [`backend/internal/repository/repository_integration_test.go`](../../backend/internal/repository/repository_integration_test.go)

Coverage includes:

- loading runs
- listing run agents
- setting Temporal IDs
- valid run transitions
- valid run-agent transitions
- invalid transition rejection
- rollback when history insert fails
- Temporal ID idempotency and conflict rejection

Why:

- this code is mostly about database correctness
- unit tests alone would miss FK, transaction, and trigger behavior

## Key Behavior Decisions

### Atomic status updates and history writes

For both `runs` and `run_agents`, the repository writes:

1. the current status row
2. the matching history row

inside one transaction.

Why:

- if those writes split, the product can show inconsistent run timelines
- a run page could claim one current state while the history trail says another
- rollback on failure keeps the timeline trustworthy

User-facing effect later:

- when live run pages and replay pages exist, the audit timeline should match the latest state shown in the UI

### Repository-level state-machine enforcement

The repository rejects invalid transitions before the database is mutated.

Why:

- the schema only constrains the allowed values, not the allowed transitions
- workflow callers should not be able to jump from `draft` to `running` or from `queued` to `executing`

User-facing effect later:

- the product should show believable progress, not impossible state jumps caused by a buggy worker or workflow

### Temporal identity attachment is now first-write-wins

`SetRunTemporalIDs` now behaves as:

- first attach succeeds when the run has no Temporal IDs yet
- repeating the same IDs is allowed and treated as idempotent
- trying to replace existing IDs with different values is rejected

Why:

- a run should not be silently rebound to a different workflow lineage
- duplicate workflow start handling should be safe
- idempotent retries should not rewrite `updated_at` or mutate the row unnecessarily

User-facing effect later:

- debugging a stuck or failed run should always point to the correct Temporal workflow
- audit trails and support tooling should not drift to the wrong workflow instance

## Tradeoffs

### Chosen tradeoff: explicit repository surface over generic CRUD

We chose a small method set instead of a generic store abstraction.

Pros:

- easier to understand
- closer to the product domain
- fewer accidental write paths

Cons:

- more methods will be added as workflow needs grow
- the repository will stay somewhat verbose

### Chosen tradeoff: SQL in `.sql` files instead of building queries in Go

Pros:

- SQL stays readable
- `sqlc` verifies shape against the schema
- query review is easier

Cons:

- regeneration is required after query changes
- there is generated code in the diff

### Chosen tradeoff: integration tests seed the minimum upstream graph

The tests create just enough tenancy, challenge, runtime, build, deployment, and snapshot data to make `runs` and `run_agents` valid.

Pros:

- tests reflect the real schema and ownership model
- they prove the repository works in the same FK environment the product expects

Cons:

- fixture setup is more verbose than isolated unit tests
- tests require a real Postgres database

## What This PR Does Not Do

It does not add:

- HTTP handlers
- Temporal workflow implementations
- provider execution logic
- replay builders
- scorecard generation
- public arena features

That is intentional. This PR is the persistence/control slice that those later pieces should call into.

## What The Next Workflow Issue Can Assume

The next issue can now safely assume there is one backend path to:

- load a run
- load its run agents
- attach Temporal identifiers
- move statuses forward
- record matching history rows atomically

That means the workflow issue can focus on orchestration logic rather than persistence design.
