# Docs revamp epic

Revamp all AgentClash documentation to match the changelog editorial system: sans + mono only, white/opacity palette, working GFM tables, syntax-highlighted code, and refreshed diagrams.

## Problem

- Markdown tables, task lists, and strikethrough did not compile (`remark-gfm` missing from `MDXRemote`).
- Tailwind `prose` wrapper in `DocsShell` fought with `.prose-agentclash-docs` styles.
- Headings used `--font-display` (Instrument Serif) while changelog moved to sans-only.
- Code blocks rendered as plain `<pre>` without Prism highlighting.
- Diagram presets used heavy emerald gradients that felt off-brand vs changelog.

## Phases

### Phase 0 — Rendering foundation (this branch)

- [x] Add `remark-gfm` and wire `mdxRemoteOptions` into docs `MDXRemote`
- [x] Syntax-highlight fenced code via shared `CodeBlock`
- [x] Remove conflicting Tailwind `prose` wrapper from `DocsShell`
- [x] Align `.prose-agentclash-docs` typography with changelog (sans headings)
- [x] Refresh diagram preset chrome (neutral borders, no display serif)

### Phase 1 — Shell & navigation polish

- [x] Sidebar / TOC / search styling pass (changelog mono labels, white active states)
- [x] Mobile docs nav drawer
- [x] Docs home hero aligned with changelog index density

### Phase 2 — Content migration (41 MDX files)

Track per-section completion in `web/content/docs/`:

| Section | Files | Status |
| --- | --- | --- |
| Getting started | 3 | Complete |
| Concepts | 8 | Complete |
| Eval packs | 7 | Complete |
| Guides | 8 | Complete |
| Architecture | 6 | Complete |
| Reference | 2 | Complete |
| Contributing | 3 | Complete |
| Index | 1 | Complete |

Migration checklist per page:

1. Verify tables render (GFM)
2. Verify fenced code blocks highlight
3. Replace stale diagrams or add presets where helpful
4. Align callout tone with changelog voice
5. Add cross-links to new product surfaces (datasets, multi-turn, security harnesses)

### Phase 3 — New docs coverage

- [x] Datasets overview (generation, CI gates, regression suites)
- [x] Multi-turn eval packs (human takeover, calibration)
- [x] Security evaluation harnesses

### Phase 4 — Agent exports

- [x] Regenerate `llms-full.txt` (generated at build via docs.ts) after content migration
- [x] Per-page markdown export smoke test (covered in test contract)

## Reference

- Changelog typography: `web/src/components/marketing/changelog/`
- Docs rendering: `web/src/app/docs/[[...slug]]/page.tsx`
- MDX components: `web/src/components/docs/mdx-components.tsx`
- Prose tokens: `web/src/app/globals.css` (`.prose-agentclash-docs`)

## Verification

```bash
cd web
pnpm exec vitest run src/app/docs
pnpm lint
pnpm build
```

Manual: open `/docs/eval-packs/bundle-yaml-reference` and confirm the `version` field table renders with borders and header row.
