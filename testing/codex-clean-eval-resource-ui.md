# codex/clean-eval-resource-ui — Test Contract

## Functional Behavior
- `/resources/eval-checklist` should present the lead magnet without the "Search intent covered" section.
- `/resources/eval-checklist` should not show the global footer link columns after the content.
- `/resources/eval-checklist/thank-you` should not show the global footer link columns.
- The email form should use clean, direct copy: no awkward separate "WORK EMAIL" label above the button area and no spammy product-updates sentence.
- Layout should not leave large empty sections after removed content.

## Unit Tests
- N/A — this is a presentational cleanup with existing metadata tests unchanged.

## Integration / Functional Tests
- `pnpm exec vitest run src/app/public-canonical-metadata.test.ts src/app/sitemap.test.ts` should continue to pass.

## Smoke Tests
- `pnpm build` should pass.
- Browser check `/resources/eval-checklist` at desktop and mobile widths.
- Browser check `/resources/eval-checklist/thank-you` at desktop width.

## E2E Tests
- N/A — live form submission is covered manually in production after deploy.

## Manual / cURL Tests
- Confirm landing page no longer contains "Search intent covered".
- Confirm resource and thank-you pages no longer contain footer column headings like Product, Guides, Company.
- Confirm form CTA reads cleanly and no longer shows the disliked copy.
