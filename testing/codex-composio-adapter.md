# Review Checkpoint Contract

Task: add a Composio workspace-tool adapter for native AgentClash eval runs on a fresh branch from `main`.

## Functional Expectations

1. Native execution must be able to expose bound workspace tools to the model in addition to existing primitive, composed, and mock tools.
2. The native runtime must load workspace-tool bindings from the frozen `source_agent_spec.tools` payload and resolve the referenced workspace tool records by ID at run start.
3. A new runtime tool category for workspace tools must be recorded in the tool registry and emitted in native tool-execution events.
4. Support one capability initially: `capability_key = "composio.execute"`.
5. The Composio adapter must use direct tool execution, not Tool Router sessions or MCP:
   - `POST /api/v3/tools/execute/{tool_slug}`
   - `x-api-key` header for auth
   - request body supports `arguments` plus `user_id` or `connected_account_id`, with optional `version`
6. The workspace tool definition contract for `composio.execute` must support:
   - required: `tool_slug`, `credential_reference`
   - at least one auth selector: `user_id` or `connected_account_id` in definition or binding config
   - optional: `description`, `parameters`, `base_url`, `version`
   - optional binding override: `tool_name`
7. Workspace tool execution must return structured JSON content to the model. Non-2xx responses or JSON payloads with `successful: false` must be marked as tool errors.
8. Prompt-eval execution must remain unchanged and tool-less.

## Tests To Add Or Run

- `go test ./internal/engine ./internal/repository ./internal/worker`
- Add engine coverage for workspace-tool registration and Composio execution against an `httptest` server.
- Add repository coverage for decoding frozen tool bindings from `source_agent_spec`.
- Add native executor or worker coverage proving a bound Composio workspace tool is visible and executable in a native run.

## Manual Verification

- Verify the worktree path is `/tmp/agentclash-composio-adapter`.
- Verify the branch is `codex/composio-adapter`.
- Verify the final PR targets `main`.
- Verify the main checkout at `/home/atharva/agentclash` remains untouched.
