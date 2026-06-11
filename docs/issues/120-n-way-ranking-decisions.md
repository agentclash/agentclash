# Issue 120: N-Way Ranking Decisions

Issue: `#120 N-way comparison: support ranking 3+ agents in a single view`

Scope note:
- This document is backend-only.
- UI is intentionally out of scope for this implementation.
- The goal is to add a run-level ranking read model without changing execution behavior.

## Final Decisions

### 1. Ranking is computed on read

Decision:
- Do not materialize rank order or a separate ranking document in the database.
- Compute ranking on read from existing persisted run-level and per-agent scorecard data.

Why:
- Per-agent scored facts are already persisted.
- Ranking is just ordering a small agent list in memory.
- Formula and sorting behavior are likely to change during early iterations.
- Materializing ranking would create unnecessary invalidation and backfill work.

Implications:
- No new table.
- No new ranking write path.
- No migration for ranking persistence.

### 2. No new persistence object

Decision:
- Do not create a `run_rankings` table.
- Do not create a separate ranking materialization workflow activity.
- Do not add a repository write method for ranking.

Why:
- Existing `run_scorecards` already contains the run-level agent summaries needed for ranking.
- A second persisted read model would create drift risk with little value.

### 3. New read endpoint

Decision:
- Add a new endpoint: `GET /v1/runs/{runID}/ranking`.

Why:
- `/v1/compare` is pairwise and baseline/candidate-specific.
- Ranking is an intra-run listing problem, not a pairwise comparison problem.
- Keeping ranking separate avoids contaminating compare and release-gate semantics.

### 4. Default ordering

Decision:
- Default ordering will match the current run winner semantics:
  - `correctness` descending
  - `reliability` descending as the tiebreaker
  - stable fallback by `lane_index`

Why:
- This preserves current backend meaning.
- It avoids introducing a new hidden ranking formula as the default.
- It keeps `winning_run_agent_id` and default rank ordering aligned.

### 5. Explicit sort support

Decision:
- Support query-based sorting by scored dimension:
  - `correctness`
  - `reliability`
  - `latency`
  - `cost`
- Default sort remains the winner-compatible ordering above.

Why:
- This satisfies the strongest low-risk requirement in the issue.
- It is easy to explain and easy to test.

### 6. Composite score is deferred

Decision:
- Do not implement composite score in this first backend iteration.
- Do not add `composite_score` or `delta_from_top` based on a synthetic weighted formula yet.

Why:
- There is no canonical composite formula in the current scoring spec.
- `overall_score` exists in storage but is not actually populated by the current scoring path.
- Adding composite now would force a product decision that is not yet stable.

Follow-up:
- Composite can be added later once product semantics are agreed and versioned.

### 7. Partial agents remain visible

Decision:
- Agents with missing score data remain in the response.
- If the requested sort field is unavailable, they sort last.
- They receive `rank: null`.
- Their score state remains explicit in the payload.

Why:
- This preserves observability.
- It avoids pretending incomplete evidence is rankable.
- It aligns with current scorecard behavior that surfaces missing evidence instead of hiding it.

### 8. Ranking is strictly intra-run

Decision:
- Ranking is only supported for agents within a single run.

Why:
- The issue is about "compare my 5 agents in a single run".
- Cross-run ranking would inherit compatibility checks from pairwise compare and expand scope substantially.

### 9. Pairwise compare remains unchanged

Decision:
- No semantic or contract changes to:
  - `GET /v1/compare`
  - release-gate evaluation
  - run comparison persistence

Why:
- Ranking and pairwise regression analysis solve different problems.
- Existing compare logic is intentionally baseline/candidate shaped.

### 10. Current winner semantics remain unchanged

Decision:
- Keep `run_scorecards.winning_run_agent_id` semantics as-is.
- Do not redefine the winner as a composite winner.

Why:
- Current winner logic is already implemented and tested.
- Changing it now would be a hidden behavior change unrelated to the minimal ranking endpoint.

## Non-Goals For This PR

- No UI work
- No composite formula
- No release-gate changes
- No cross-run ranking
- No database migrations
- No backfill job

## Exit Criteria For This Planning

Before implementation starts, all code changes must remain consistent with these constraints:
- ranking is read-only
- no new ranking persistence
- compare stays pairwise
- winner logic stays unchanged
- partial agents are visible but not rankable for unavailable sort keys
