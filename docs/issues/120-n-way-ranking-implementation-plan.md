# Issue 120: N-Way Ranking Implementation Plan

Branch:
- `issue-120-ranking-prep`

Primary goal:
- Add a backend endpoint that returns an intra-run ranking view for all agents in a run, without changing execution or score persistence semantics.

## Planned Endpoint

- `GET /v1/runs/{runID}/ranking`

Query parameters:
- `sort_by`
  - allowed: `correctness`, `reliability`, `latency`, `cost`
  - omitted means default ordering

Default ordering:
- `correctness` desc
- `reliability` desc
- `lane_index` asc as a stable fallback

Partial sorting behavior:
- agents with unavailable values for the selected sort field sort last
- those agents return `rank: null`

## Proposed Response Shape

```json
{
  "run_id": "uuid",
  "evaluation_spec_id": "uuid",
  "sort": {
    "field": "correctness",
    "direction": "desc",
    "default_order": false
  },
  "winner": {
    "run_agent_id": "uuid",
    "strategy": "correctness_then_reliability",
    "status": "winner",
    "reason_code": "best_correctness"
  },
  "evidence_quality": {
    "missing_fields": [],
    "warnings": []
  },
  "items": [
    {
      "rank": 1,
      "run_agent_id": "uuid",
      "lane_index": 0,
      "label": "agent-a",
      "status": "completed",
      "sort_value": 0.94,
      "sort_state": "available",
      "overall_score": null,
      "correctness_score": 0.94,
      "reliability_score": 0.88,
      "latency_score": 0.62,
      "cost_score": 0.71,
      "dimensions": {
        "correctness": { "state": "available", "score": 0.94 },
        "reliability": { "state": "available", "score": 0.88 },
        "latency": { "state": "available", "score": 0.62 },
        "cost": { "state": "available", "score": 0.71 }
      }
    }
  ]
}
```

Notes:
- `overall_score` remains present only as a passthrough field from stored run-agent scorecards.
- No new composite value is computed in this PR.

## Exact Files To Edit

### API layer

1. Edit [backend/internal/api/routes.go](/home/atharva/agentclash/backend/internal/api/routes.go)
- Register `GET /runs/{runID}/ranking`

2. Edit [backend/internal/api/run_reads.go](/home/atharva/agentclash/backend/internal/api/run_reads.go)
- Extend repository interface to load run scorecards
- Add ranking service interface methods
- Add request parsing for `sort_by`
- Add response structs
- Add handler

3. Edit [backend/internal/api/server.go](/home/atharva/agentclash/backend/internal/api/server.go)
- Wire the new ranking-capable run read service into route registration if needed by current construction pattern

4. Edit [backend/internal/api/server_test.go](/home/atharva/agentclash/backend/internal/api/server_test.go)
- Update test doubles if interface signatures change

5. Edit [backend/internal/api/run_reads_test.go](/home/atharva/agentclash/backend/internal/api/run_reads_test.go)
- Add handler/service tests for ranking success and failure cases

### Repository/read support

6. Edit [backend/internal/repository/run_scorecard.go](/home/atharva/agentclash/backend/internal/repository/run_scorecard.go)
- Export or add decode helpers as needed for run scorecard documents
- Keep behavior unchanged; only expose reusable read helpers if necessary

7. Edit [backend/internal/repository/errors.go](/home/atharva/agentclash/backend/internal/repository/errors.go)
- Reuse existing `ErrRunScorecardNotFound` if available
- Add nothing unless a new explicit error is truly needed

8. Edit [backend/internal/repository/repository.go](/home/atharva/agentclash/backend/internal/repository/repository.go)
- No behavioral change expected
- Only edit if the run read repository interface needs an already-existing method surfaced

### Tests

9. Edit [backend/internal/repository/run_scorecard_test.go](/home/atharva/agentclash/backend/internal/repository/run_scorecard_test.go)
- Add pure sorting helper tests if shared logic lives here or in a nearby package

10. Edit [backend/internal/repository/repository_integration_test.go](/home/atharva/agentclash/backend/internal/repository/repository_integration_test.go)
- Only if integration coverage for existing run scorecard reads needs extension

## Exact Files To Create

1. Create [backend/internal/api/run_ranking.go](/home/atharva/agentclash/backend/internal/api/run_ranking.go)
- Preferred location for ranking-specific service, parsing, sorting, and response assembly
- Keep `run_reads.go` from becoming overly crowded if needed

2. Create [backend/internal/api/run_ranking_test.go](/home/atharva/agentclash/backend/internal/api/run_ranking_test.go)
- Focused tests for ranking logic if split into a dedicated file

3. Create [docs/issues/120-n-way-ranking-decisions.md](/home/atharva/agentclash/docs/issues/120-n-way-ranking-decisions.md)
- Decision record

4. Create [docs/issues/120-n-way-ranking-implementation-plan.md](/home/atharva/agentclash/docs/issues/120-n-way-ranking-implementation-plan.md)
- This plan

5. Create [testing/issue-120-ranking-prep.md](/home/atharva/agentclash/testing/issue-120-ranking-prep.md)
- Pre-PR validation checklist
- Not intended to be pushed later if the user chooses not to

6. Create [review-checkpoint.md](/home/atharva/agentclash/review-checkpoint.md)
- Rolling reviewer guidance updated after each implementation slice

## Planned Implementation Order

### Slice 1: API contract and read path

- Add ranking request parsing
- Add ranking response structs
- Add handler
- Add service method
- Reuse existing run + run scorecard loading

Reviewer checkpoint:
- Endpoint exists
- Authz path matches existing run read rules
- No new persistence

### Slice 2: Sorting logic

- Add default ordering
- Add explicit dimension ordering
- Add partial-agent handling
- Add stable tie-breaking

Reviewer checkpoint:
- Sorting semantics match the decisions doc
- Partial agents sort last
- Rank numbering only applies to available-sort-value rows

### Slice 3: Tests and docs

- Add unit tests
- Add handler tests
- Add curl-based validation checklist
- Update review checkpoint file

Reviewer checkpoint:
- Tests cover success, invalid sort, missing scorecard, authz, and partial data cases

## Things Explicitly Forbidden During Implementation

- No DB migration
- No ranking persistence
- No compare endpoint changes
- No release-gate logic changes
- No UI code changes
- No composite formula

## PR Description Plan

When the PR is eventually created, the description should include:

### 1. Summary
- what endpoint was added
- what behavior was intentionally not changed

### 2. Mermaid diagram

```mermaid
flowchart TD
    A[GET /v1/runs/{runID}/ranking] --> B[Authorize caller via run workspace]
    B --> C[Load run]
    C --> D[Load run_scorecard]
    D --> E[Decode run_scorecard agents]
    E --> F[Apply requested sort]
    F --> G[Assign ranks to available rows]
    G --> H[Return ranking response]
```

### 3. Reviewer guide
- review slice 1: endpoint and authz
- review slice 2: sorting semantics
- review slice 3: tests and edge cases

### 4. Validation
- list of unit tests run
- list of manual curl checks run
- any known limitations
