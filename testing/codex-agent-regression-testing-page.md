# codex/agent-regression-testing-page - Test Contract

## Functional Behavior

- Add an indexable `/platform/agent-regression-testing` page targeting `AI agent regression testing` and `AI agent CI gates`.
- Page has one visible H1, route-scoped metadata, canonical URL, OpenGraph URL, BreadcrumbList JSON-LD, and FAQ JSON-LD for visible FAQ content.
- Page explains the AgentClash wedge for regression testing: baseline versus candidate runs, replay evidence, scorecards, promoted failures, challenge packs, and pull request gates.
- Page links to relevant docs and trial paths without changing authenticated product behavior.
- Sitemap exposes the new page without changing the homepage or shared marketing footer.

## Unit Tests

- N/A - static marketing page and metadata.

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built output includes `/platform/agent-regression-testing`.
- Built page output includes the H1 text, canonical metadata, BreadcrumbList JSON-LD, and FAQPage JSON-LD.
- Sitemap output includes `https://www.agentclash.dev/platform/agent-regression-testing`.

## E2E Tests

- N/A - no authenticated workflow changes.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/platform/agent-regression-testing | rg "AI agent regression testing|BreadcrumbList|FAQPage|canonical"
curl -s https://www.agentclash.dev/sitemap.xml | rg "platform/agent-regression-testing"
```

Expected: page HTML exposes the target phrase, structured data, canonical tag, and sitemap URL.
