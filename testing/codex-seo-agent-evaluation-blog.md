# codex/seo-agent-evaluation-blog - Test Contract

## Functional Behavior

- Add a public blog post targeting AI agent evaluation and AI agent regression testing search intent.
- Post should explain why real-task agent evals need replay evidence, scorecards, challenge packs, and CI gates.
- Post should include internal links to the platform pages and relevant docs.
- No homepage, main landing page UI, shared marketing footer, authenticated/internal UI, DB, or destructive changes.

## Unit Tests

- `pnpm -C web test -- blog.test.ts`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built blog route includes `/blog/ai-agent-evaluation-regression-testing`.
- Built blog page includes the post title and internal links to both platform pages.

## E2E Tests

- N/A - static blog content.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/blog/ai-agent-evaluation-regression-testing | rg "AI Agent Evaluation Needs Regression Testing|/platform/agent-evaluation|/platform/agent-regression-testing"
```

Expected: the new blog post is published and internally links to both platform pages.
