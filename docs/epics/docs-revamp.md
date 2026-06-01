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

- [ ] Sidebar / TOC / search styling pass (changelog mono labels, white active states)
- [ ] Mobile docs nav drawer
- [ ] Docs home hero aligned with changelog index density

### Phase 2 — Content migration (38 MDX files)

Track per-section completion in `web/content/docs/`:

| Section | Files | Status |
| --- | --- | --- |
| Getting started | 3 | Not started |
| Concepts | 8 | Not started |
| Challenge packs | 6 | Not started |
| Guides | 5 | Not started |
| Architecture | 6 | Not started |
| Reference | 4 | Not started |
| Contributing | 4 | Not started |
| Index | 1 | Not started |

Migration checklist per page:

1. Verify tables render (GFM)
2. Verify fenced code blocks highlight
3. Replace stale diagrams or add presets where helpful
4. Align callout tone with changelog voice
5. Add cross-links to new product surfaces (datasets, multi-turn, security harnesses)

### Phase 3 — New docs coverage

- [ ] Datasets overview (generation, CI gates, regression suites)
- [ ] Multi-turn challenge packs (human takeover, calibration)
- [ ] Security evaluation harnesses

### Phase 4 — Agent exports

- [ ] Regenerate `llms-full.txt` after content migration
- [ ] Per-page markdown export smoke test

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

Manual: open `/docs/challenge-packs/bundle-yaml-reference` and confirm the `version` field table renders with borders and header row.
