# Review Checkpoint

Purpose:
- Reviewer-oriented running checklist for issue `#400`.
- Updated after each implementation slice.
- Each slice is a commit on this branch. Review one slice at a time — top to bottom.

Issue:
- `#400 feat: race-context — agents see live peer standings mid-run`
- https://github.com/agentclash/agentclash/issues/400

Branch:
- `issue-400-race-context`

Scope: **Phase 1 only** — backend + API + CLI. No UI changes. (Phase 2 UI lands in a separate follow-up PR.)

## Global Review Rules

Reviewer should always verify:
- No challenge-pack YAML schema changes. Packs remain untouched.
- No frontend changes. UI toggle + token-split render is Phase 2.
- `race_context: false` (the default) must be byte-identical to main in event stream and behavior.
- No new environment variables required to run existing tests.
- Redis is treated as an already-available worker dependency (per CLAUDE.md). No new infra.
- `race.standings.injected` tokens are never double-counted into the model's billable spend.

## Review Slices

### Slice 1: DB migration + runs.race_context columns

Status:
- completed

Reviewer should check:
- Migration file numbered `00029_` (next after `00028_public_share_links.sql`).
- Both `+goose Up` and `+goose Down` sections present.
- `race_context` defaults to `false` so backfill is implicit.
- `race_context_min_step_gap` is nullable with CHECK constraint enforcing `[1, 10]` when non-null.
- sqlc regenerated cleanly — new columns flow into `Run` sqlc struct and the `GetRunAgentExecutionContextByIDRow` worker view.
- Run domain struct in `backend/internal/domain/run.go` extended with `RaceContext bool` and `RaceContextMinStepGap *int32`.
- `mapRun` and the worker execution-context mapping both populate the new fields.
- New helper `cloneInt32Ptr` mirrors the existing `cloneInt64Ptr` pattern.

Relevant tests:
- `go test -short -race -count=1 ./internal/domain/... ./internal/repository/...` green.
- No behavior change in this slice. Existing tests still pass.

Files changed:
- `backend/db/migrations/00029_race_context.sql` (new)
- `backend/db/queries/worker_execution_context.sql` (select new columns)
- `backend/internal/domain/run.go` (struct fields)
- `backend/internal/repository/repository.go` (`mapRun`, `cloneInt32Ptr`)
- `backend/internal/repository/run_agent_execution_context.go` (worker mapping)
- `backend/internal/repository/sqlc/*` (regenerated)

### Slice 2: race.standings.injected event type + payload

Status:
- completed

Reviewer should check:
- `EventTypeRaceStandingsInjected = "race.standings.injected"` added in `envelope.go`.
- Added to the `isValidType` switch.
- Payload + trigger enum live in a new file `runevents/race.go` (payloads are per-event-type elsewhere in the package, so a sibling file keeps the package tidy without introducing a new cross-cutting abstraction).
- `RaceStandingsTrigger` values: `cadence`, `peer_submitted`, `peer_failed`, `peer_timed_out`, each validated by `IsValid()`.
- `RaceStandingsInjectedPayload` carries `tokens_added`, `standings_snapshot` (verbatim newswire), `triggered_by`, `self_step_index`, `min_step_gap`.
- No emitters wired yet — the event type is dormant until Slice 7.

Relevant tests:
- `TestRaceStandingsInjectedEventTypeIsValid` — new type accepted by `isValidType`.
- `TestRaceStandingsTriggerIsValid` — enum validity + rejection of bogus values.
- `TestRaceStandingsInjectedEnvelopeValidates` — full envelope with marshalled payload passes `ValidatePending`.

Files changed:
- `backend/internal/runevents/envelope.go`
- `backend/internal/runevents/race.go` (new)
- `backend/internal/runevents/race_test.go` (new)

### Slice 3: API field + OpenAPI spec + N<2 validation

Status:
- completed

Reviewer should check:
- `createRunRequest` accepts optional `race_context` (bool, default false) and `race_context_min_step_gap` (pointer int).
- Decoder validates cadence range `[1, 10]` with code `invalid_race_context_min_step_gap`.
- `RunCreationManager.CreateRun` rejects `race_context && len(agents) < 2` with code `invalid_race_context`.
- Fields flow through `CreateRunInput` → `CreateQueuedRunParams` → sqlc `CreateRunParams` → DB.
- Both `createRunResponse` and `getRunResponse` return the new fields. Omitted cadence in the response is `null` (via `omitempty` + pointer).
- OpenAPI `CreateRunRequest`, `CreateRunResponse`, `RunDetail` schemas updated. `@redocly/cli lint` passes.
- Backwards compat: omitted field defaults to false; existing `TestCreateRunEndpointReturnsCreated` still passes without changes.

Relevant tests (all green):
- `TestCreateRunEndpointPropagatesRaceContext` — race_context=true + N=2 + cadence=5 → 201, fake sees correct input, response body echoes fields.
- `TestCreateRunEndpointRejectsRaceContextCadenceOutOfRange` — cadence=11 → 400 with `invalid_race_context_min_step_gap`.
- `TestRunCreationManagerRejectsRaceContextWithSingleAgent` — race_context=true + N=1 → `invalid_race_context`.
- Full backend `go test ./...` green.

Files changed:
- `backend/db/queries/runs.sql` (INSERT columns + params)
- `backend/internal/repository/repository.go` (`CreateQueuedRunParams` + sqlc call)
- `backend/internal/api/runs.go` (request + input + response + decoder cadence check)
- `backend/internal/api/run_service.go` (N<2 validation + pass-through)
- `backend/internal/api/run_reads.go` (`getRunResponse` + builder)
- `backend/internal/api/runs_test.go` (two endpoint tests)
- `backend/internal/api/run_service_test.go` (manager test)
- `backend/internal/repository/sqlc/runs.sql.go` (regenerated)
- `docs/api-server/openapi.yaml` (three schemas)

### Slice 4: CLI --race-context flags

Status:
- completed

Reviewer should check:
- `cli/cmd/run.go` exposes `--race-context` (bool, default false) and `--race-context-cadence` (int, default 0 = backend default, [1, 10] otherwise).
- Both flags add their fields to the POST `/v1/runs` body **only when set** (preserves backwards-compatible request shape).
- Help text explains the 2+ agents requirement and the cadence range.
- `cli/cmd/cmd_test.go` `executeCommand` helper now resets `--race-context` and `--race-context-cadence` so absence-assertions work across tests (cobra stores flag state on the package-level command).
- `cd cli && go build ./...`, `go vet ./...`, `go test -short -race -count=1 ./...` all green.

Relevant tests (all green):
- `TestRunCreateRaceContextFlagsPropagate` — flags set → body contains `race_context: true` and `race_context_min_step_gap: 4`.
- `TestRunCreateWithoutRaceContextFlagsOmitsFields` — flags unset → neither key appears in the body.

Files changed:
- `cli/cmd/run.go` (flag registration + body building)
- `cli/cmd/run_create_interactive_test.go` (two new tests)
- `cli/cmd/cmd_test.go` (flag reset between tests)

### Slice 5: Redis standings writer (event subscriber)

Status:
- pending

Reviewer should check:
- New subscriber in worker subscribes to the existing run-event pub/sub channel.
- Writes to Redis hash `run:{run_id}:standings`, field `agent:{run_agent_id}`.
- Handles all documented event types (`system.run.started`, `system.step.started`, `tool.call.completed`, `model.call.completed`, `system.output.finalized`, `system.run.failed`).
- TTL = 1h applied on first write.
- No writes when `race_context: false` for the run (or writer handles per-run opt-in check cheaply).
- Graceful degradation if Redis is down: log + continue, do not block runs.

Relevant tests:
- Subscriber unit test with mocked Redis and a synthetic event stream.
- Integration test: simulated 2-agent run populates standings hash correctly.

Files changed:
- (to be filled)

### Slice 6: Newswire formatter

Status:
- pending

Reviewer should check:
- Pure function: `FormatStandings(snapshot, selfAgentID) (string, int)` returning formatted text + estimated token count.
- Ranked by step desc; submitters pinned to top by submission order.
- Self tagged as `you (<model>)`.
- Handles: peer FAILED, peer TIMED OUT, peer submitted · verifying, peer submitted · passed, peer submitted · failed, peer not_started, all-peers-submitted-you-are-last case.
- Zero network / IO in the function — fully unit-testable.

Relevant tests:
- Table-driven test covering every edge-case combination.
- Deterministic output (no timestamps, no randomness).

Files changed:
- (to be filled)

### Slice 7: Wire injection into native_executor loop

Status:
- pending

Reviewer should check:
- Injection lives in `backend/internal/engine/native_executor.go` at the step-boundary.
- Injection only fires when `run.race_context == true`.
- First injection earliest at step 3.
- Dedupe: never two consecutive injections without an agent-authored turn between.
- Cadence predicates: `(step - last_inject_step) >= min_step_gap` OR `peer_state_changed_since_last_inject`.
- Adds `role=user` message to agent message list.
- Emits `race.standings.injected` event with accurate `tokens_added`.
- Behavior identical to main when `race_context == false`.

Relevant tests:
- Unit test: cadence predicates in isolation.
- Integration test: multi-agent simulated run produces correct injection timing.

Files changed:
- (to be filled)

### Slice 8: Token accounting split

Status:
- pending

Reviewer should check:
- `run_agent_tokens` collector sums tokens from `model.call.completed` events.
- `run_race_context_tokens` collector sums `tokens_added` from `race.standings.injected` events.
- `run_total_tokens` = agent + race_context. Value unchanged for runs with `race_context: false`.
- No double-counting.

Relevant tests:
- Scoring test: run without race_context — `run_race_context_tokens == 0`, `run_total_tokens == run_agent_tokens`.
- Scoring test: run with race_context — both split metrics positive, sum matches total.

Files changed:
- (to be filled)

### Slice 9: Integration tests + acceptance check

Status:
- pending

Reviewer should check:
- E2E test: 6-agent run with `race_context: true` produces expected events in order.
- Cadence: first injection at step 3 earliest; subsequent at `>= min_step_gap` or peer-state change.
- Edge case: N=1 rejection at API.
- Edge case: peer crashed mid-run surfaces as `FAILED` in subsequent injections.
- Parity test: run with `race_context: false` produces same event stream as main (snapshot diff).

Files changed:
- (to be filled)

## Out of scope for this PR

- Frontend UI toggle + token-split rendering (Phase 2 follow-up PR).
- Pack YAML schema changes (intentionally not supported).
- Agent-initiated `get_standings` tool (push-only in v1).
- Historical replay / retroactive race context.
