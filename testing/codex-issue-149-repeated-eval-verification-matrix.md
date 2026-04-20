# #149 Repeated-Eval Verification Matrix

This matrix is the trust contract for repeated eval sessions. It separates low-cost deterministic coverage that should stay green in everyday development from opt-in manual scale checks that prove the session read surfaces remain trustworthy as repetition counts grow.

## Read Surfaces Under Test
- `POST /v1/eval-sessions`
- `GET /v1/eval-sessions/{evalSessionID}`
- `GET /v1/eval-sessions?workspace_id={workspaceID}&limit={n}`
- `./scripts/smoke/eval-session-read.sh`

## Expected Invariants
- Every created eval session has exactly `repetitions` attached child runs.
- Child runs are returned in deterministic creation order.
- `summary.run_counts.total` equals the number of attached child runs.
- Freshly created sessions surface queued child-run counts explicitly.
- `aggregate_result` remains `null` until session-level aggregation persistence lands.
- `evidence_warnings` explain missing aggregate evidence instead of leaving the field ambiguous.
- Single-run mode (`repetitions=1`) uses the same read surfaces and summary math as repeated mode.

## CI Matrix
| Tier | Repetitions | Coverage | Expected Outcome |
|---|---:|---|---|
| Single-run compatibility | 1 | API/repository tests plus local smoke when needed | Session read returns one child run, `total=1`, `queued=1`, `aggregate_result=null`, and an aggregate-evidence warning. |
| Small repeated run | 3 | API/repository tests plus `REPETITIONS=3 ./scripts/smoke/eval-session-read.sh` | Session read returns three child runs, ordered consistently, with `total=3`, `queued=3`. |
| Small comparison-style inspection | 3 then 5 | `REPETITIONS=3 SECOND_REPETITIONS=5 ./scripts/smoke/eval-session-read.sh` | List read shows both sessions so a developer can inspect session-vs-session state side by side without querying raw tables. |

## Manual Scale Matrix
| Tier | Repetitions | Command | Expected Outcome |
|---|---:|---|---|
| Medium | 10 | `REPETITIONS=10 ./scripts/smoke/eval-session-read.sh` | Detail and list reads remain responsive; summary counts stay exact; warnings remain deterministic. |
| Medium-high | 30 | `REPETITIONS=30 ./scripts/smoke/eval-session-read.sh` | No missing runs; no ordering drift; list surface still includes the created session. |
| Large | 50 | `REPETITIONS=50 ./scripts/smoke/eval-session-read.sh` | Same invariants as medium, used as the default manual stress rehearsal. |
| Very large | 100 | `REPETITIONS=100 ./scripts/smoke/eval-session-read.sh` | Used sparingly to prove attachment/read paths hold at the top end of the planned scale tier. |

## Manual Checklist
1. Start the local stack with `./scripts/dev/start-local-stack.sh`.
2. Run the CI-matrix smoke commands above and confirm the script reports success.
3. Run one medium or larger repetition tier and confirm:
   - detail read returns the requested repetition count
   - list read contains the created session
   - `aggregate_result` is still `null`
   - `evidence_warnings` explains the missing aggregate result
4. Run the two-session smoke command (`REPETITIONS=3 SECOND_REPETITIONS=5`) and confirm both sessions appear in list order for side-by-side inspection.
5. Shut the local stack back down when finished.

## Notes
- Until `#361` lands, the read-model contract intentionally reports `aggregate_result: null` and uses `evidence_warnings` to make that absence explicit.
- When `#361` and `#362` land, extend this matrix rather than replacing it so single-run compatibility and scale-tier inspection stay locked.
