# codex/seo-llms-platform-pages - Test Contract

## Functional Behavior

- Add public platform page entries to machine-readable discovery surfaces only.
- `/llms.txt` includes `/platform/agent-evaluation` and `/platform/agent-regression-testing` with concise descriptions for AI agent evaluation discovery.
- `/llms-full.txt` includes the same public product page links in its introductory bundle metadata.
- No homepage, main landing page UI, shared marketing footer, authenticated/internal UI, DB, or destructive changes.

## Unit Tests

- `pnpm -C web test -- docs.test.ts`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built `/llms.txt` output includes both platform page URLs.
- Built `/llms-full.txt` output includes both platform page URLs.

## E2E Tests

- N/A - machine-readable text routes only.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/llms.txt | rg "platform/agent-evaluation|platform/agent-regression-testing"
curl -s https://www.agentclash.dev/llms-full.txt | rg "platform/agent-evaluation|platform/agent-regression-testing"
```

Expected: both machine-readable files expose the public platform URLs.
