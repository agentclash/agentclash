# codex/fix-web-swr-lockfile — Test Contract

## Functional Behavior
- `web/package.json` and `web/pnpm-lock.yaml` stay in sync for the `swr` dependency so CI and Vercel frozen installs do not fail before build.
- The lockfile importer for `web/` explicitly lists `swr` under `dependencies`.
- No application source files change for this fix.

## Unit Tests
- N/A — lockfile-only dependency metadata fix.

## Integration / Functional Tests
- `pnpm install --frozen-lockfile` in `/Users/ayush.parihar/.codex/worktrees/2068/agentclash/web` completes successfully.

## Smoke Tests
- `npm test` in `web/` is known to have two unrelated existing failures in `create-run-dialog.test.tsx`; this fix must not introduce additional failures beyond that baseline.
- `npm run lint` in `web/` passes.
- `npm run build` in `web/` passes.

## E2E Tests
- N/A — no runtime behavior change.

## Manual / cURL Tests
- Inspect `web/pnpm-lock.yaml` and confirm `importers -> . -> dependencies -> swr` is present with specifier `^2.4.1`.
