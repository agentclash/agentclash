# codex/seo-llms-blog-posts - Test Contract

## Functional Behavior

- Add public blog posts to `/llms.txt` so AI tools can discover the blog corpus.
- Add public blog post content to `/llms-full.txt` with source URLs and normalized internal links.
- Preserve existing docs, agent skills, and platform page coverage.
- No homepage, main landing page UI, shared marketing footer, authenticated/internal UI, DB, or destructive changes.

## Unit Tests

- `pnpm -C web test -- docs.test.ts`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built `/llms.txt` includes `/blog/ai-agent-evaluation-regression-testing`.
- Built `/llms-full.txt` includes the blog title and links to both platform pages.

## E2E Tests

- N/A - machine-readable text routes only.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/llms.txt | rg "/blog/ai-agent-evaluation-regression-testing"
curl -s https://www.agentclash.dev/llms-full.txt | rg "AI Agent Evaluation Needs Regression Testing|/platform/agent-evaluation|/platform/agent-regression-testing"
```

Expected: blog posts are discoverable in both machine-readable files.
