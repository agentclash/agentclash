# feat/docs-revamp — Test Contract

## Functional Behavior

Docs site matches changelog editorial typography (sans + mono, white/opacity palette).

- GFM markdown (tables, task lists, strikethrough) renders on all `/docs/*` pages.
- Fenced code blocks syntax-highlight with language label and copy button.
- No Tailwind `prose` wrapper conflicts with `.prose-agentclash-docs`.
- Headings use sans semibold, not `--font-display`.
- Diagram presets use neutral borders (no emerald gradient chrome).
- Mobile docs navigation drawer works below `lg` breakpoint.
- Three new doc pages exist and appear in sidebar nav:
  - `/docs/guides/datasets-overview`
  - `/docs/eval-packs/multi-turn`
  - `/docs/guides/security-evaluation`
- All 38 existing MDX pages carry `dateModified` frontmatter and cross-link to new surfaces where relevant.

## Unit Tests

- `web/src/app/docs/docs-page-schema.test.tsx` — JSON-LD for docs home and detail pages passes.
- `web/src/lib/docs.test.ts` — `resolves every docs navigation item` passes for expanded nav.
- `web/src/lib/docs.test.ts` — `llms.txt` and `llms-full.txt` builders include new pages.

## Integration / Functional Tests

- `pnpm exec vitest run src/app/docs src/lib/docs.test.ts` — all pass.
- `pnpm build` in `web/` — compiles without type errors.

## Smoke Tests

- `/docs/eval-packs/bundle-yaml-reference` — pipe table under `version` renders with header row and borders.
- `/docs/eval-packs/multi-turn` — operator API table renders.
- `/docs/guides/security-evaluation` — stress harness code block highlights.

## E2E Tests

N/A — visual verification via dev server; no Playwright suite for docs in repo.

## Manual / cURL Tests

```bash
cd web
pnpm exec vitest run src/app/docs src/lib/docs.test.ts
pnpm lint
pnpm build
pnpm dev
# Browser: /docs/eval-packs/bundle-yaml-reference (table)
# Browser: /docs (home cards + FAQ)
# Browser: resize to mobile width — menu opens sidebar drawer
curl -sS http://localhost:3000/docs-md/guides/datasets-overview | head
curl -sS http://localhost:3000/llms-full.txt | rg "datasets-overview|multi-turn|security-evaluation"
```
