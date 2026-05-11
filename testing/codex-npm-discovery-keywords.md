# codex/npm-discovery-keywords - Test Contract

## Functional Behavior

- Published `agentclash` npm metadata describes the CLI as an AI agent evaluation tool.
- npm keywords include high-intent discovery terms from issue #633.
- npm README opener matches the updated positioning.
- Package contents and binary wiring remain unchanged.

## Unit Tests

- N/A - package metadata and README copy only.

## Integration / Functional Tests

- Parse `npm/cli/package.json` as JSON.
- `npm pack --dry-run --json` from `npm/cli`.

## Smoke Tests

- Dry-run package output should still include `bin/agentclash.js`, `README.md`, and `package.json`.

## E2E Tests

- N/A - no CLI runtime behavior changes.

## Manual / cURL Tests

After the next npm release, inspect:

```bash
npm view agentclash description keywords
```

Expected: description and keywords include AI agent evaluation / agent eval discovery terms.
