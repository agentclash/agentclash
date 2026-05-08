# codex/seo-docs-main-entity-schema - Test Contract

## Functional Behavior
- Docs page `TechArticle` JSON-LD describes `mainEntityOfPage` as a `WebPage` node with the canonical docs URL.
- Existing docs structured data remains intact, including breadcrumbs, headline, description, author, publisher, and docs website relationship.
- The change must be crawler-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/components/marketing/json-ld.test.ts` verifies `docsPageSchema` emits the richer `mainEntityOfPage` object.
- `web/src/app/docs/docs-page-schema.test.tsx` verifies rendered docs page JSON-LD uses the same shape.

## Integration / Functional Tests
- `pnpm -C web test -- docs-page-schema.test.tsx json-ld.test.ts`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- N/A - structured data only.

## E2E Tests
- N/A - structured data only.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/docs/getting-started/quickstart | rg 'mainEntityOfPage|WebPage'
# Expected: docs article structured data includes a WebPage mainEntityOfPage node.
```
