# codex/browser-primitives-pack — Test Contract

## Functional Behavior
- Native agents see browser tools only when `tool_policy.allowed_tool_kinds` allows `browser`.
- Browser tools execute Browser Use browser-harness commands inside the sandbox with `BU_NAME` set to the run-agent ID.
- Browser tools inject `BROWSER_USE_API_KEY` from workspace secrets when available without requiring literal sandbox env vars.
- Browser tool results are structured JSON with useful fields for navigation state, screenshots, action success, and errors.
- Browser tools return policy-denied tool results when browser access is not allowed.
- A starter browser challenge pack demonstrates how to score a simple browser navigation task.

## Unit Tests
- `TestBrowserTools_VisibleOnlyWhenBrowserKindAllowed` — registry exposes browser tools only for browser-enabled policies.
- `TestBrowserOpenTool_ExecutesHarnessWithRunAgentNamespace` — command environment includes `BU_NAME` and browser secret.
- `TestBrowserOpenTool_DeniedWithoutBrowserKind` — policy denial returns a tool error instead of executing.
- `TestBrowserEvalTool_ReturnsStructuredResult` — successful harness stdout is wrapped as structured JSON.

## Integration / Functional Tests
- `go test ./internal/engine -run Browser -count=1` from `backend/` passes.
- `go test ./internal/challengepack -run Browser -count=1` from `backend/` passes.

## Smoke Tests
- Parse the starter browser challenge pack with the challenge-pack parser.

## E2E Tests
- N/A — this PR adds primitives and a sample pack. Live Browser Use cloud execution requires `BROWSER_USE_API_KEY` and should be smoke-tested manually in staging.

## Manual / cURL Tests
```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"
export BROWSER_USE_API_KEY="..."
cd cli
go run . challenge-pack publish ../examples/challenge-packs/browser-navigation-smoke.yaml
go run . run create --follow
```

Expected: a browser-enabled run can call browser tools and submit a scored final answer.
