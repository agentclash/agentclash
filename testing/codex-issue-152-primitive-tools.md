# codex/issue-152-primitive-tools — Test Contract

## Functional Behavior
- Add primitive tools for `search_files`, `search_text`, `query_json`, `http_request`, `run_tests`, `build`, and `query_sql`.
- Keep the existing native executor and registry flow intact; new tools must fit through the current `Tool` interface and registry resolution path.
- New command-backed primitives must share a common execution path that:
  - runs fixed internal commands inside the sandbox
  - supports per-tool timeout handling
  - supports per-tool exit-code interpretation
  - supports consistent structured JSON responses
  - spills large outputs to `/workspace/.agentclash/spill/<tool>_<id>.txt` once inline payload exceeds `32KB`
- Tool visibility must honor the locked taxonomy:
  - `search_files`, `search_text` -> `file`
  - `query_json`, `query_sql` -> `data`
  - `http_request` -> `network`
  - `run_tests`, `build` -> `build`
  - raw `exec` remains separately gated by `AllowShell`
- Internal built-in primitives may use `Session.Exec()` without exposing raw `exec` to the agent.
- `http_request` must:
  - require `AllowNetwork`
  - support structured responses
  - support file downloads through `output_path`
  - enforce timeout defaults/caps
  - reject loopback, link-local, and private-network targets by default
- `query_sql` must:
  - accept a generic `engine` parameter
  - support only `sqlite` on day 1
  - return a structured unsupported-engine error for anything else
- `run_tests` and `build` must:
  - support explicit command override
  - support auto-detection for `package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`, and `Makefile`
  - return structured output with command used, working directory, exit code, stdout, and stderr at minimum
- `query_json` must support both file input and inline JSON.
- `search_text` must treat ripgrep exit code `1` as a successful empty result, not a tool error.
- Tool execution events and replay behavior must continue to work with the new tools through the existing observer path.

## Unit Tests
- `TestBuildToolRegistry_DefaultPrimitivesVisible` updated to cover newly visible primitives under matching tool kinds.
- `TestBuildToolRegistry_*` coverage for allow/deny behavior with the new tool-kind mapping.
- Shared command helper tests:
  - large output spills to file after `32KB`
  - hard sandbox failures are surfaced as engine failures
  - tool-specific non-zero exits can be treated as successful empty results
- `search_files` unit tests:
  - returns structured file matches
  - respects `max_results`
  - hidden when `file` tool kind is denied
- `search_text` unit tests:
  - returns structured text matches
  - exit code `1` becomes empty success
  - large output spills to file
- `query_json` unit tests:
  - works with `file_path`
  - works with inline `json`
  - invalid query becomes tool error
- `http_request` unit tests:
  - rejects when network is not allowed
  - rejects blocked/private targets
  - returns structured response payload
  - supports `output_path`
- `run_tests` unit tests:
  - detects command from supported project markers
  - honors explicit command override
  - returns structured execution result
- `build` unit tests:
  - detects command from supported project markers
  - honors explicit command override
  - returns structured execution result
- `query_sql` unit tests:
  - supports SQLite query execution
  - returns unsupported-engine error for non-SQLite engines
  - SQL syntax/runtime error returns tool error

## Integration / Functional Tests
- Native executor integration proves the new tool definitions are exposed to provider requests when allowed by tool policy.
- Native executor integration proves internal command-backed primitives execute successfully without requiring raw `exec` visibility.
- Observer/replay integration confirms tool execution events still record for new primitives.
- Sandbox request / policy tests confirm `http_request` depends on network policy and other tools respect tool-kind visibility.

## Smoke Tests
- `go test ./internal/engine ./internal/worker ./internal/sandbox/...`
- Focused package tests for any helper script or template-related updates added for `http_request`.
- Manual native tool-registry sanity check in tests: new tools appear only when their tool kind is allowed.

## E2E Tests
- N/A — not applicable for this change because this work is backend engine and sandbox primitive behavior, not a user-facing browser flow.

## Manual / cURL Tests
```bash
# After implementation, run backend engine-focused tests
cd /Users/atharva/agentclash/backend
go test ./internal/engine ./internal/worker ./internal/sandbox/...

# Optional focused runs while iterating
cd /Users/atharva/agentclash/backend
go test ./internal/engine -run 'TestBuildToolRegistry|TestPrimitive|TestNativeExecutor'

# Expected:
# - all listed tests pass
# - registry reflects the locked tool-kind taxonomy
# - query_sql supports sqlite only
# - http_request is blocked when network policy is disabled
```
