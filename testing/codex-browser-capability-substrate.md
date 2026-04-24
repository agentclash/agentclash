# codex/browser-capability-substrate — Test Contract

## Functional Behavior
- Native challenge packs can declare `browser` in `version.tool_policy.allowed_tool_kinds`.
- The native engine recognizes `browser` as a distinct tool kind while preserving existing file, data, network, build, and shell behavior.
- Browser-capable challenge packs can request browser runtime dependencies through the E2B template without changing run creation APIs.
- Prompt-eval packs continue to reject tools, sandbox settings, and tool policies.
- Documentation explains how browser-enabled packs should declare capability, network access, secrets, and evidence.

## Unit Tests
- `backend/internal/engine` policy tests cover `allowsBrowserTools` for unrestricted, allowed, and denied policies.
- `backend/internal/engine` sandbox request tests cover `allowed_tool_kinds: ["browser"]` passing through from the manifest.
- `backend/internal/challengepack` validation tests cover `browser` as an accepted tool kind and an unknown tool kind as rejected.
- Existing prompt-eval validation tests continue to pass.

## Integration / Functional Tests
- `go test ./internal/engine ./internal/challengepack` from `backend/` passes.
- The E2B template remains syntactically valid TypeScript.

## Smoke Tests
- `go test ./internal/engine -run Browser -count=1` from `backend/` passes.
- `go test ./internal/challengepack -run Tool -count=1` from `backend/` passes.

## E2E Tests
- N/A — this PR only adds the capability substrate. End-to-end browser task execution lands in the follow-up primitives PR.

## Manual / cURL Tests
- N/A — no API route changes in this PR.
