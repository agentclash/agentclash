# codex/seo-platform-structured-data - Test Contract

## Functional Behavior

- Add SoftwareApplication JSON-LD to the two public platform pages only.
- Structured data uses the existing marketing JSON-LD helper and matches each page's title, description, canonical path, and product category.
- Preserve existing breadcrumb and FAQ JSON-LD.
- No homepage, main landing page UI, shared marketing footer, authenticated/internal UI, DB, or destructive changes.

## Unit Tests

- `pnpm -C web test -- json-ld.test.ts platform-pages.test.ts`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built platform page HTML includes one SoftwareApplication JSON-LD object for each platform page.
- Built platform page HTML preserves FAQPage and BreadcrumbList JSON-LD.

## E2E Tests

- N/A - structured metadata only.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/platform/agent-evaluation | rg "SoftwareApplication|AI Agent Evaluation Platform"
curl -s https://www.agentclash.dev/platform/agent-regression-testing | rg "SoftwareApplication|AI Agent Regression Testing"
```

Expected: both public pages expose SoftwareApplication structured data without visible UI changes.
