# Browser Agent Challenge Packs

Browser-capable packs evaluate agents that can use a real browser, not just HTTP
fetches. AgentClash uses the same native run loop and scoring pipeline as other
agentic packs, with a dedicated `browser` tool kind gating browser primitives.

## Pack Contract

Declare browser access on native packs:

```yaml
version:
  execution_mode: native
  sandbox:
    network_access: true
  tool_policy:
    allowed_tool_kinds:
      - browser
```

`browser` is intentionally separate from `network`. Browser tools operate a
remote browser session through Browser Use browser-harness, while `network`
continues to mean direct HTTP request tools from the sandbox.

Do not put secrets such as `BROWSER_USE_API_KEY` in `version.sandbox.env_vars`.
Sandbox environment variables are literal-only. Browser primitives resolve the
workspace secret at execution time and inject it only into the harness command
that needs it.

## Runtime

The E2B template installs `browser-harness` and its Python dependencies. The
browser itself should be a Browser Use cloud browser so each run agent can get an
isolated session namespace. The follow-up browser primitives use:

- `BU_NAME=<run_agent_id>` for per-agent isolation
- `BROWSER_USE_API_KEY` from workspace secrets
- structured JSON tool results for navigation state, screenshots, and errors

## Authoring Guidance

Prefer benchmark tasks that have a deterministic goal state:

- navigate to a page and extract visible facts
- fill a form and verify confirmation text
- interact with dropdowns, dialogs, tabs, or iframes
- download or generate a file and validate its contents
- recover from a wrong page, stale session, or transient page error

Score with existing evidence before adding custom graders:

- `final_output` JSON validators for extracted answers
- post-execution file captures for downloads or saved screenshots
- code-execution checks for generated artifacts
- latency, tool-call count, token, and behavioral metrics

Avoid tasks that require personal accounts, production purchases, CAPTCHA
solving, or unbounded live-web browsing unless the pack provides explicit test
accounts and teardown instructions.
