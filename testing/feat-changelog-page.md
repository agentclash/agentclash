# feat/changelog-page — Test Contract

## Functional Behavior

- Public route `/changelog` renders without authentication.
- Changelog content is grouped into **ten-day periods** from **2026-04-15** through the latest period ending **2026-06-01**.
- Periods sort **newest first**; the latest period shows a **Latest** badge.
- Each period shows: date label, headline, and bullet entries with category badges (**Added**, **Improved**, **Fixed**, **Security**).
- Each period card exposes an anchor id (`#2026-05-25`, etc.) for deep links and structured data.
- Marketing header and footer link to `/changelog`.
- HTML sitemap page (`/sitemap`) lists the changelog with description.
- XML sitemap (`/sitemap.xml`) includes `/changelog` with weekly change frequency.

## Unit Tests

- `getChangelogPeriods` returns periods sorted descending by `startDate`.
- `getChangelogLatestModified` returns the newest period end date.
- `changelogIndexSchema` emits BreadcrumbList, WebPage, ItemList, and FAQPage nodes with absolute URLs.
- `renderChangelogMarkdown` exports all periods as markdown with source link.
- Metadata tests in `public-canonical-metadata.test.ts` and `secondary-pages-metadata.test.ts` lock canonical and social tags.

## Integration / Functional Tests

- `buildLlmsIndex` includes a Changelog section and `/changelog` link.
- `getDocsSearchIndex` includes a searchable changelog entry.
- `buildLlmsFull` embeds the rendered changelog markdown bundle section.
- Changelog page renders JSON-LD script `#agentclash-changelog-index-schema`.

## Smoke Tests

- `cd web && pnpm exec vitest run src/lib/changelog.test.ts src/components/marketing/json-ld.test.ts src/app/changelog/changelog-index-schema.test.tsx src/app/public-canonical-metadata.test.ts src/app/secondary-pages-metadata.test.ts src/app/sitemap.test.ts src/lib/docs.test.ts`
- `cd web && pnpm lint`

## E2E Tests

- N/A — static marketing page; covered by unit/render tests and manual curl checks.

## Manual / cURL Tests

1. Fetch page HTML (after deploy or local dev):
   ```bash
   curl -sS http://localhost:3000/changelog | grep -E 'Changelog|Latest|Added|application/ld\\+json'
   ```
   Expected: page title, timeline content, JSON-LD script present.

2. Verify sitemap entry:
   ```bash
   curl -sS http://localhost:3000/sitemap.xml | grep changelog
   ```
   Expected: `https://www.agentclash.dev/changelog`.

3. Verify llms discovery:
   ```bash
   curl -sS http://localhost:3000/llms.txt | grep -i changelog
   ```
   Expected: changelog link in index.

4. Verify metadata (canonical):
   ```bash
   curl -sS http://localhost:3000/changelog | grep 'rel="canonical"'
   ```
   Expected: canonical href `/changelog`.
