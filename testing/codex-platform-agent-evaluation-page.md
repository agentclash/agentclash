# codex/platform-agent-evaluation-page - Test Contract

## Functional Behavior

- Add an indexable `/platform/agent-evaluation` page targeting `AI agent evaluation platform`.
- Page has one visible H1, route-scoped metadata, canonical URL, OpenGraph URL, breadcrumb JSON-LD, and FAQ JSON-LD for visible FAQ content.
- Page explains the AgentClash wedge: real-task agent races, same tools/constraints, replay, scorecards, challenge packs, and CI regression gates.
- Page links to relevant docs and trial/demo paths without changing authenticated product behavior.
- Sitemap and public marketing navigation expose the new page.

## Unit Tests

- N/A - static marketing page and metadata.

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built output includes `/platform/agent-evaluation`.
- Built page output includes the H1 text, canonical metadata, BreadcrumbList JSON-LD, and FAQPage JSON-LD.
- Sitemap output includes `https://www.agentclash.dev/platform/agent-evaluation`.

## E2E Tests

- N/A - no authenticated workflow changes.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/platform/agent-evaluation | rg "AI agent evaluation platform|BreadcrumbList|FAQPage|canonical"
curl -s https://www.agentclash.dev/sitemap.xml | rg "platform/agent-evaluation"
```

Expected: page HTML exposes the target phrase, structured data, canonical tag, and sitemap URL.
