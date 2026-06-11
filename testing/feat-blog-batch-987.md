# feat/blog-batch-987 — Test Contract

## Functional Behavior

Five new MDX posts in `web/content/blog/` targeting comparison and agent-eval search intent:

1. Agent evaluation vs prompt evaluation (Braintrust workflow fit)
2. Benchmark AI agents on your own data (not public leaderboards)
3. pass@k / pass^k for enterprise teams (extends existing pass-at-k post)
4. Evaluating coding agents on private repos (checklist)
5. AgentClash vs LangSmith vs Braintrust for production testing (links to `/compare`)

Each post includes:
- Frontmatter: `title`, `date`, `description`, `author`
- Internal links to SEO pages: `/agent-evals`, `/llm-agent-evaluation`, `/use-cases/coding-agent-evaluation`, `/compare`
- Link to `/enterprise` and conversion path (benchmarks or compare where relevant)
- Related resources block at end

At least 2 posts link to `/compare/agentclash-vs-*` competitor pages.

Blog index auto-lists via `getAllPosts()` (no manual index edit required).

## Unit Tests

- Existing `public-canonical-metadata.test.ts` blog post fixture still passes with new slugs if referenced
- `getAllPosts()` returns 11 posts (6 existing + 5 new) when parsed

## Integration / Functional Tests

- `web/src/app/blog/page.tsx` renders new posts in index (sorted by date)
- Blog post pages render MDX without frontmatter errors

## Smoke Tests

```bash
cd web && pnpm build
cd web && pnpm exec vitest run src/app/public-canonical-metadata.test.ts
```

## E2E Tests

N/A — content-only change.

## Manual / cURL Tests

- Open `/blog` and confirm 5 new titles appear
- Open each new slug URL and verify internal links resolve
- Confirm posts 1 and 5 link to `/compare/agentclash-vs-braintrust` or `/compare/agentclash-vs-langsmith`
