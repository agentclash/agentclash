# codex/issue-187-strict-plan — Test Contract

## Functional Behavior
Implement composed-tool delegation chaining exactly as specified in issue #187 and its linked implementation plan.
- A composed tool may delegate to another composed tool through the existing `primitive` field; declaration order must not matter.
- Delegation resolution must use `resolveAny()` so hidden tools can still participate in the chain.
- Runtime execution must track the full delegation chain and enforce a hard maximum chain depth of `8`.
- Runtime execution must detect delegation cycles and surface them as structured tool errors.
- Failure metadata must preserve nested wrapping per level and include `ResolutionChain` plus `FailureDepth`.
- Parameter visibility must remain explicit passthrough only; delegated tools only receive args provided by their caller.
- Tool execution telemetry must include `resolution_chain` and `failure_depth` only when the chain length is greater than `1`.
- Validation must reject cyclic or over-depth composed-tool declarations before execution.

## Unit Tests
The following unit coverage must exist and pass.
- `TestComposedTool_ChainsComposedToComposed` — two-level composed delegation reaches the terminal primitive and records full chain metadata.
- `TestComposedTool_DetectsCycleAtRuntime` — runtime cycle detection returns a structured cycle failure.
- `TestComposedTool_EnforcesDepthCap` — runtime execution fails when delegation exceeds `MaxDelegationDepth`.
- `TestComposedTool_ReportsFailureDepthInChain` — resolution failures inside a chain report the correct failure depth.
- `TestBuildToolRegistry_TwoPassAllowsOutOfOrderComposedTools` — registry build succeeds when composed tools are declared out of order.
- `TestBuildToolRegistry_RejectsStaticCycle` — registry build rejects static composed-tool cycles.
- Existing `TestBuildToolRegistry*` coverage continues to pass after the two-pass registration change.

## Integration / Functional Tests
N/A — not applicable for this change per the locked implementation plan, which explicitly limits testing to unit, registry-build, and validation coverage.

## Smoke Tests
- `cd backend && go build ./...`
- `cd backend && go test ./internal/engine/... -run TestBuildToolRegistry`
- `cd backend && go test ./internal/engine/... ./internal/challengepack/... -v -count=1`

## E2E Tests
N/A — not applicable for this change. This work stays inside backend engine, validation, and native telemetry plumbing.

## Manual / cURL Tests
Manual verification is code-level for this backend-only change.

```bash
cd backend && go build ./...
cd backend && go test ./... -count=1
```

Expected:
- Build succeeds without compile errors.
- All backend tests pass.
- New chain metadata only appears for multi-level delegation events.
- Validation and registry build reject cyclic chains before runtime where applicable.
