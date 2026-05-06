# Local Test Log: codex/issue-610-harness-chat-ui

Timestamp: 2026-05-06T15:28:09+0530

## Dependency Setup

Command:

```bash
cd web
npm install
```

Result: passed. npm reported existing dependency advisories; no dependency files were intentionally changed for this PR.

## Focused Component Tests

Command:

```bash
cd web
npm test -- --run 'src/app/(workspace)/workspaces/[workspaceId]/agent-harnesses/agent-harnesses-client.test.tsx'
```

Result: passed.

Latest summary:

```text
Test Files  1 passed (1)
Tests       19 passed (19)
```

Reviewer follow-up coverage added:

- runner output events do not render raw `message` or `raw` payloads
- failed POST preserves the user's prompt for retry
- setup sidebar includes repository and base branch context

## Focused Lint

Command:

```bash
cd web
npm run lint -- 'src/app/(workspace)/workspaces/[workspaceId]/agent-harnesses/agent-harnesses-client.tsx' 'src/app/(workspace)/workspaces/[workspaceId]/agent-harnesses/agent-harnesses-client.test.tsx'
```

Result: passed.
