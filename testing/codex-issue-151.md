# codex/issue-151 — Test Contract

## Functional Behavior
- The native executor builds a tool registry once per run and uses it for both provider-visible tool definitions and tool-call dispatch.
- The registry exposes a common `Tool` interface shared across primitive, composed, and mock tool categories.
- The registry preserves the current native primitives: `submit`, `read_file`, `write_file`, `list_files`, and `exec`.
- The pack manifest may declare a top-level `tools` block separate from `tool_policy`.
- Registry construction applies configuration in this order:
  1. Load all primitive tools.
  2. Apply pack `tools.allowed` if present.
  3. Apply pack `tools.denied` if present.
  4. Load pack `tools.custom`.
  5. Apply target-level deny-only `tool_overrides` from deployment snapshot config.
- Custom tool names must not silently shadow primitive or previously registered tool names; registry construction fails instead.
- Composed tools may delegate to hidden primitives internally even when those primitives are not agent-visible.
- Unknown tool calls do not crash the run; they return a structured tool error to the model.
- Tool execution events include the agent-facing tool name plus tool category metadata, and composed tools include the resolved underlying tool name/category when applicable.
- Existing sandbox capability enforcement still applies to primitives after the registry refactor.

## Unit Tests
- `TestBuildToolRegistry_DefaultPrimitivesVisible` — registry exposes the native primitive set when no extra filters are configured.
- `TestBuildToolRegistry_AppliesAllowedDeniedAndSnapshotOverridesInOrder` — visibility follows the required precedence and target overrides are deny-only.
- `TestBuildToolRegistry_RejectsCustomToolNameCollision` — duplicate custom/primitive names fail registry construction.
- `TestRegistryToolDefinitions_OnlyReturnsVisibleTools` — provider-facing tool definitions match the registry-visible set.
- `TestRegistryResolve_ReturnsStructuredUnknownToolErrorPath` — unknown tool calls become recoverable tool errors.
- `TestPrimitiveToolImplementations_PreserveCurrentBehavior` — `submit`, `read_file`, `write_file`, `list_files`, and `exec` behave the same after implementing the interface.
- `TestDecodeManifestToolsConfig` — the new pack `tools` block is parsed correctly and absent blocks keep current behavior.
- `TestDecodeSnapshotToolOverrides_DenyOnly` — deployment snapshot config can deny tools by name and cannot introduce new tools.

## Integration / Functional Tests
- `TestNativeExecutorHappyPathWritesFileThenSubmits` continues to pass with registry-backed dispatch.
- `TestNativeExecutorRecoversFromToolErrorAndEventuallySubmits` continues to pass with registry-backed dispatch.
- Add an executor integration test covering a tool hidden by config so it is omitted from provider tool definitions and rejected at execution time with a structured error.
- Add an executor integration test covering a snapshot-denied tool override.
- Add an observer test verifying tool events include `tool_category`, and for composed tools, `resolved_tool_name` and `resolved_tool_category`.

## Smoke Tests
- `go test ./backend/internal/engine ./backend/internal/worker ./backend/internal/challengepack`
- Confirm the updated executor still constructs provider requests successfully with tool definitions from the registry.
- Confirm replay/event tests remain green after adding category metadata.

## E2E Tests
- N/A — this change is executor and backend-internal, with no browser or external end-user workflow in this branch.

## Manual / cURL Tests
- N/A — there is no stable HTTP surface for this issue alone; verification is through Go tests and run-event assertions.
