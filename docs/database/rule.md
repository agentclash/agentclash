# Database Rules

Status: canonical rules for database-level changes

Purpose: tell future coding agents how to design, extend, and migrate the database without drifting away from the domain model in [`docs/domains/domains.md`](../domains/domains.md).

This is not the final schema. It is the rulebook that all schema work must follow.

## Source of Truth Order

When making any database change, use this order of authority:

1. [`docs/domains/domains.md`](../domains/domains.md)
2. this file
3. the actual migration set
4. repository code

If the code and the domain doc disagree, the agent should assume the database needs to move toward the domain doc unless a newer canonical doc says otherwise.

## Core Rule

The database exists to preserve domain ownership, historical reproducibility, and fair comparison.

For AgentClash, that means the schema must optimize for:

- stable private ownership boundaries
- immutable benchmark references
- durable run history
- replayability
- scoring reproducibility
- safe public projection

The schema must not optimize first for convenience, generic flexibility, or fewer tables.

## What The Database Must Guarantee

Every database change should preserve these guarantees:

1. A private object always belongs to an `Organization` or `Workspace`.
2. A `Run` always points to a frozen benchmark context.
3. A `RunAgent` always points to a frozen agent/deployment context.
4. Historical runs remain interpretable after builds, deployments, tools, models, or providers change later.
5. Public objects are derived from private objects, not direct exposure of private rows.
6. Replay and scoring data can be traced back to a specific run context.
7. Shared resources such as `Tool`, `Knowledge Source`, and provider accounts remain reusable without losing tenancy boundaries.

If a proposed schema change weakens any of those guarantees, it is the wrong change.

## Database Philosophy

### 1. Postgres Is The Relational Source Of Truth

The database owns:

- identities
- ownership boundaries
- version references
- state
- summaries
- indexes
- queryable relationships

The database does not need to store every large payload inline.

Large artifacts can live outside Postgres, but Postgres must still own:

- artifact identity
- artifact type
- run association
- storage location
- retention state

### 2. Model Domain Truth, Not UI Convenience

Do not shape core tables around screens or temporary API responses.

Examples:

- do not store “selected agents” as a JSON array inside `runs`
- do not store “published replay card” as a field on a private run row
- do not collapse `Agent Build` and `Agent Deployment` just because a UI form edits them together

If the UI needs a shortcut, create a query, view, or read model later.

### 3. Prefer Explicit Relationships Over Generic Config Blobs

Use real tables and join tables for real domain relationships.

Allowed uses for `jsonb`:

- provider-specific metadata
- judge raw outputs
- tool payload details
- replay event details
- external integration payloads
- flexible diagnostic metadata

Do not use `jsonb` to hide core relational structure such as:

- which tools an agent can use
- which knowledge sources are attached
- which models or providers are active
- which challenges were selected
- which objects are public

If the product reasons about it, queries it, filters on it, or joins through it, it should usually be relational.

### 4. Reproducibility Beats Mutable Convenience

AgentClash is an evaluation system. Historical reproducibility matters more than minimizing schema duplication.

That means the schema should intentionally preserve frozen references to:

- challenge-pack versions
- challenge identities
- deployment snapshots
- runtime profiles
- model/provider selections
- evaluation specs

Runs must not depend on “whatever the current build or deployment now looks like.”

## Global Table And Column Rules

### Naming

Use:

- `snake_case` for tables and columns
- plural table names
- singular foreign-key columns ending in `_id`

Examples:

- `organizations`
- `agent_build_versions`
- `challenge_pack_versions`
- `run_agents`
- `public_run_snapshots`

### Primary Keys

Default rule:

- every root table gets `id uuid primary key`

Do not use natural keys as primary keys.

Natural business identifiers may still exist as unique columns such as:

- `slug`
- `key`
- `provider_model_id`

but they do not replace the primary key.

### Timestamps

Every durable root table should usually have:

- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`

Append-only tables may omit `updated_at` if rows are never changed after insert.

All time values should be stored as `timestamptz`.

### Status Fields

Use lifecycle status columns where the domain has state transitions.

Prefer:

- constrained `text` columns with application constants
- or dedicated lookup/reference tables when the status set carries metadata

Avoid introducing PostgreSQL enum types for rapidly evolving workflow states unless the state set is already stable and unlikely to change.

### Soft Delete And Archival

Default rule:

- do not soft-delete everything by habit

Use:

- `archived_at` when the object should disappear from active use but remain referentially valid
- hard deletes only for records that are clearly safe to remove

Never hard-delete historical rows that affect:

- runs
- scorecards
- replays
- publications
- leaderboard provenance

## Domain-To-Database Rules

### 1. Identity And Tenancy

This domain defines the ownership root for the private product.

Tables in this area will likely include:

- `organizations`
- `workspaces`
- `users`
- `memberships`
- `roles`

Rules:

- every private root object must be owned either directly by a workspace or by an organization
- if an object is workspace-scoped, the schema must still allow proving which organization owns that workspace
- cross-workspace leakage must be prevented by foreign-key paths and query design, not just application code

Practical rule:

- favor `workspace_id` on workspace-owned product objects
- use `organization_id` for organization-shared infrastructure objects

Do not create orphan product rows that only become scoped indirectly through several optional joins.

### 2. Challenge Catalog

This domain is a core product pillar, not a side table collection.

Tables in this area will likely include:

- `challenge_packs`
- `challenge_pack_versions`
- `challenges`
- `challenge_input_sets`
- `challenge_version_links` or an equivalent join structure

Rules:

- `Challenge Pack` is the long-lived product identity
- `Challenge Pack Version` is immutable once it becomes runnable
- each `Challenge` inside a pack must have its own addressable identity
- challenge identity must survive across pack versions
- run records must point to the exact pack version and selected input set used

Design implications:

- do not store a whole pack version as one opaque JSON blob
- do not assume a run result only needs pack-level identity
- preserve both pack-level and challenge-level references

Recommended shape:

- stable challenge identity row
- versioned pack row
- join rows that bind a challenge identity into a specific pack version
- input sets versioned or otherwise frozen by reference

### 3. Agent Registry

This is where many schema mistakes are most likely, so be strict.

Tables in this area will likely include:

- `agent_builds`
- `agent_build_versions`
- `agent_deployments`
- `agent_deployment_snapshots`
- `tools`
- `agent_build_tools`
- `knowledge_sources`
- `agent_build_knowledge_sources`
- `runtime_profiles`

Rules:

- `Agent Build` and `Agent Deployment` are different nouns and must not be merged
- mutable edit surfaces belong on the build or deployment root
- benchmarked history must point to immutable version or snapshot rows
- `Tool` is a reusable first-class resource
- `Knowledge Source` is a reusable first-class resource
- tools and knowledge sources should be attached through join tables, not stored as inline arrays

Specific rules:

- `agent_builds` hold durable identity and ownership
- `agent_build_versions` hold immutable build definitions
- `agent_deployments` represent runnable targets
- `agent_deployment_snapshots` preserve the exact execution context used by runs

Knowledge-source scoping rule:

- support both organization scope and workspace scope
- organization-scoped knowledge sources may be reused by many workspaces in that organization
- workspace-scoped knowledge sources remain isolated

Implement this explicitly. Do not hide scope in free-form metadata.

### 4. Provider And Model Infrastructure

Provider data is shared infrastructure and should not be buried inside agent tables.

Tables in this area will likely include:

- `provider_accounts`
- `model_catalog_entries`
- `model_aliases`
- `routing_policies`
- `spend_policies`

Rules:

- provider credentials belong to provider-account rows or secret references, not agent rows
- model catalog rows should be reusable across many builds and deployments
- routing and spend policy should be independently attachable

The database should make it possible to answer:

- which provider account powered this deployment
- which logical model alias mapped to which provider model at the time
- which spend policy or routing policy applied

If that cannot be reconstructed later, the schema is too loose.

### 5. Run Orchestration

This domain is the center of the product’s operational history.

Tables in this area will likely include:

- `runs`
- `run_agents`
- `run_state_history`
- `execution_plans`

Rules:

- a `Run` is an experiment envelope
- a `Run` may contain one or many `RunAgent` entries
- `RunAgent` is the primary unit of execution for a participating build/deployment
- all `RunAgent` entries inside one `Run` must share the same challenge-pack version and challenge input set

Recommended invariants:

- `runs.challenge_pack_version_id` is required
- `runs.challenge_input_set_id` is required when the run targets a frozen input set
- `run_agents.run_id` is required
- `run_agents.agent_deployment_snapshot_id` is required for reproducibility

Do not make fairness depend on application convention alone.

If the product says a run compares participants on the same benchmark context, the schema should encode that expectation.

### Status History

For important long-running objects such as `Run` and `RunAgent`, store:

- current status on the main row
- optional status-history rows for auditability and debugging

Do not force the event stream to be the only source for understanding lifecycle changes.

### 6. Replay And Telemetry

This domain needs both append-only durability and efficient read models.

Tables in this area will likely include:

- `run_events`
- `run_replays`
- `run_agent_replays`
- `artifacts`

Rules:

- event rows are append-only
- replay summary/index rows are separate from raw events
- large payloads should be referenced, not always stored inline
- database rows should preserve enough normalized structure to filter and replay meaningfully

Store relationally:

- event type
- run and run-agent ownership
- sequence/order
- timestamps
- major references such as tool, provider call, artifact, challenge, or judge step

Store flexibly:

- raw payload details
- provider response fragments
- diagnostic metadata

Do not store an entire replay as one unstructured text/blob field if the product needs step-level filtering and comparison.

### 7. Evaluation And Scoring

Scoring is a reusable system, not a postscript.

Tables in this area will likely include:

- `evaluation_specs`
- `judge_results`
- `metric_results`
- `scorecards`
- `run_comparisons`

Rules:

- scorecards must exist at both `RunAgent` scope and `Run` comparison scope
- evaluation specs must be versionable or otherwise frozen by reference
- judge outputs must be traceable to a specific run context

Important modeling rule:

- avoid generic polymorphic `owner_type` / `owner_id` designs for critical scoring provenance if explicit foreign keys are practical

Prefer:

- explicit scope columns with clear constraints
- or separate tables when the shapes meaningfully diverge

The goal is that an engineer can reconstruct why a score exists without decoding a generic ownership abstraction.

### 8. Publication And Arena

Public content is not private content with a visibility flag.

Tables in this area will likely include:

- `public_agent_profiles`
- `public_run_snapshots`
- `publications`
- `arena_submissions`
- `leaderboards`
- `leaderboard_entries`

Rules:

- public rows are projections derived from private rows
- public identity should point to `Public Agent Profile`, not private build/deployment rows
- `Publication` and `Arena Submission` are different nouns and need different records
- `Official Arena` and `Community Arena` should remain separate concepts in the schema

Do not solve publication by adding:

- `is_public` to `runs`
- `is_public` to `agent_builds`
- a nullable `public_url` on private rows

Those shortcuts collapse domain boundaries and create leakage risk.

Instead:

- preserve source references from public rows back to private provenance
- store sanitization or redaction state explicitly
- materialize leaderboard-friendly read models

## Schema Patterns To Prefer

### Prefer Version Rows For Benchmark-Relevant Definitions

Use separate version tables when the product needs historical reproducibility.

Good candidates:

- challenge packs
- build definitions
- evaluation specs
- deployment snapshots

### Prefer Join Tables For Many-To-Many Attachments

Use join tables for:

- build-to-tool
- build-to-knowledge-source
- pack-version-to-challenge
- publication-to-public-snapshot

Do not hide many-to-many relationships inside arrays or `jsonb`.

### Prefer Derived Read Models For Public And Summary Views

If the product needs fast read-heavy summaries, create derived tables intentionally.

Good candidates:

- leaderboard entries
- replay summaries
- run comparison summaries

Do not denormalize core write tables just to make dashboards easier.

## Schema Patterns To Avoid

Avoid these unless there is a very strong reason:

- giant `jsonb` config columns for core product structure
- `owner_type` / `owner_id` polymorphic patterns across critical tables
- nullable foreign keys that mean “one of these three things”
- mixing private and public concerns in the same table
- mutating version rows after they have been used by a run
- using event tables as the only source of lifecycle truth
- collapsing build, deployment, and snapshot into one table

## Migration Rules

Every database change should be shipped as a migration set that is safe, reviewable, and reversible in practice.

### 1. One Logical Change Per Migration Set

Do not combine unrelated domain changes into one migration just because they touch the database.

Each migration set should state:

- domain owner
- business reason
- compatibility risk
- backfill need

### 2. Use Expand-Then-Contract

When changing an existing schema:

1. add new tables or columns
2. backfill or dual-write
3. switch reads
4. remove old structure later

Do not drop or rename active columns in the same step that introduces replacements.

### 3. Add Constraints In The Right Order

Do not introduce a `NOT NULL` or strict foreign key until the data can satisfy it.

Usual order:

1. add nullable column
2. backfill
3. validate
4. make not null
5. add stronger checks if needed

### 4. Separate Heavy Index Operations When Needed

For large tables, index creation may need a separate migration strategy.

Agents should not assume every index change is safe inside one normal transactional migration.

### 5. Preserve Historical Meaning

If a schema change would make old runs uninterpretable, the migration is incomplete.

Historical compatibility work may require:

- snapshot tables
- backfilled reference rows
- legacy mapping columns
- compatibility views

## Review Checklist For Any Database Change

Before writing migrations, the coding agent should answer:

1. Which domain owns this change?
2. Which product noun is being introduced or modified?
3. Is the change mutable config, immutable versioned state, or historical output?
4. Does the table need workspace scope, organization scope, or public scope?
5. Does this relationship deserve a real table or join table instead of `jsonb`?
6. Could this change break run reproducibility?
7. Could this change accidentally mix public and private concerns?
8. What indexes will the main read paths need?
9. Is there a safe migration path for existing data?
10. Will a future engineer be able to explain the row provenance from the schema alone?

If several answers are unclear, the database change is not ready.

## Minimum Expectations For Agents Writing Schema

When an agent makes database-level changes, it should also provide:

- the migration files
- a short schema rationale tied to the owning domain
- any required backfill steps
- indexes needed by the intended query path
- notes about immutability, versioning, or public/private boundaries if relevant

The agent should not open a database PR that only says “added table for feature X.”

## What Comes Next

This document should guide the next implementation docs in this order:

1. concrete schema design
2. state-machine persistence rules
3. repository/query layer conventions
4. API contracts that sit on top of the schema

If those are built without these rules, the schema will drift toward shortcuts early.
