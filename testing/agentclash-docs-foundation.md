---
title: AgentClash Docs Foundation Contract
description: Review-checkpoint contract for the first public docs implementation in the web app.
---

# Scope

Implement the first production-facing docs foundation for AgentClash inside the existing Next.js app at `/docs`, using the current MDX toolchain already present in `web/`, and extend that foundation so the docs are publicly reachable, easier to navigate, and less hand-maintained.

# Functional expectations

1. The web app exposes a dedicated docs experience at `/docs` with a docs home page and nested docs pages.
2. Docs content is file-backed in the repo and stored separately from blog content.
3. Docs pages support frontmatter with `title` and `description`.
4. The docs shell includes a left-hand navigation for the initial docs sections and highlights the current page.
5. The docs shell is visually distinct from the blog and matches the current dark AgentClash aesthetic.
6. The landing page header exposes a `Docs` entry so the new docs area is discoverable.
7. Sitemap output includes the docs landing page and the initial shipped docs pages.
8. Initial docs content only documents features and workflows that are already grounded in the current repo:
   - product/docs landing
   - hosted quickstart
   - self-host quickstart
   - runs/evals concept
   - first eval walkthrough
   - generated CLI reference
   - generated config reference
   - architecture overview
   - orchestration architecture
   - frontend architecture
   - contributor setup
9. The implementation must reuse the existing MDX/content pattern where practical instead of introducing a second docs framework dependency in this first step.
10. `/docs` and nested docs routes are publicly reachable without tripping the AuthKit middleware redirect-URI requirement in a local environment with no WorkOS redirect env configured.
11. The docs shell includes inline search/filtering across the shipped docs set.
12. The docs shell includes a heading-based table of contents for pages that have section headings.
13. At least two reference pages are generated from current source inputs rather than being fully hand-maintained.
14. The follow-up implementation remains isolated in the docs worktree branch and is prepared for a GitHub PR.

# Tests to add or run

1. Run `pnpm build` in `web/` to catch routing, metadata, and MDX compilation issues.
2. Run the built app locally and verify that `/docs` returns successfully without the AuthKit redirect-URI runtime error.
3. Verify that a generated reference page renders successfully.

# Manual verification

1. Open `/docs` and confirm the docs landing page renders.
2. Open at least one nested page in each implemented section and confirm the sidebar navigation works.
3. Confirm the docs search/filter surfaces relevant pages.
4. Confirm the table of contents appears on a page with headings.
5. Confirm the landing page header links to `/docs`.
6. Confirm docs URLs are emitted by the sitemap source.
7. Confirm `/docs` is reachable locally even when WorkOS redirect configuration is absent.

# Non-goals for this change

1. Do not add a new third-party docs framework in this pass.
2. Do not try to fully solve the broader public-marketing auth surface beyond making docs reachable.
3. Do not publish roadmap or compare pages unless they are grounded in existing shipped behavior.
