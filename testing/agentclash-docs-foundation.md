---
title: AgentClash Docs Foundation Contract
description: Review-checkpoint contract for the public docs implementation and AI-ingest exports in the web app.
---

# Scope

Implement a production-facing docs system for AgentClash inside the existing Next.js app at `/docs`, then extend it into a much more detailed A-to-Z docs surface that covers onboarding, core mental models, results interpretation, architecture, contributor workflows, and the currently shipped resource model for challenge packs, deployments, tools, artifacts, runtime profiles, provider accounts, model aliases, and secrets. Add AI-ingest endpoints so the docs can be consumed as `llms.txt`, a bundled full-text export, and per-page markdown exports without introducing a separate docs framework.

# Functional expectations

1. The web app exposes a dedicated docs experience at `/docs` with a docs home page and nested docs pages.
2. Docs content is file-backed in the repo and stored separately from blog content.
3. Docs pages support frontmatter with `title` and `description`.
4. The docs shell includes a left-hand navigation for the shipped docs sections and highlights the current page.
5. The docs shell is visually distinct from the blog and matches the current dark AgentClash aesthetic.
6. The landing page header exposes a `Docs` entry so the new docs area is discoverable.
7. Sitemap output includes the docs landing page, the shipped docs pages, and the AI-ingest endpoints.
8. Detailed docs content only documents features and workflows that are already grounded in the current repo, docs, examples, tests, or generated source readers. The shipped set for this phase includes:
   - product/docs landing
   - hosted quickstart
   - self-host quickstart
   - first eval walkthrough
   - runs and evals concept
   - agents and deployments concept
   - challenge packs and inputs concept
   - replay and scorecards concept
   - tools, network, and secrets concept
   - artifacts concept
   - interpret results guide
   - write a challenge pack guide
   - configure runtime resources and deployments guide
   - use with AI tools guide
   - generated CLI reference
   - generated config reference
   - architecture overview
   - orchestration architecture
   - sandbox architecture
   - data model architecture
   - evidence loop architecture
   - frontend architecture
   - contributor setup
   - contributor codebase tour
   - contributor testing workflow
9. The revised docs must explicitly answer the current codebase-backed questions a new user would ask, including:
   - what a challenge pack is
   - how to validate and publish a challenge pack
   - what a deployment is
   - what runtime profiles, provider accounts, and model aliases do in deployment setup
   - what a tool definition is versus a primitive implementation inside a challenge pack
   - how outbound internet access is controlled
   - how secrets are stored and where secret references resolve
   - what artifacts are and how they participate in packs, runs, and downloads
10. The implementation must reuse the existing MDX/content pattern where practical instead of introducing a second docs framework dependency.
11. `/docs`, `/docs-md`, `/llms.txt`, and `/llms-full.txt` are publicly reachable without tripping the AuthKit middleware redirect-URI requirement in a local environment with no WorkOS redirect env configured.
12. The docs shell includes inline search/filtering across the shipped docs set.
13. The docs shell includes a heading-based table of contents for pages that have section headings.
14. At least two reference pages are generated from current source inputs rather than being fully hand-maintained.
15. The app exposes `GET /llms.txt` as a concise machine-oriented index of the docs set with stable links.
16. The app exposes `GET /llms-full.txt` as a single bundled plain-text/markdown export of the shipped docs set.
17. The app exposes per-page markdown/plain-text exports under `/docs-md/...` for file-backed and generated docs pages.
18. The AI-ingest surfaces point at AgentClash docs content and markdown exports without claiming first-party support or plugin integration from third-party assistant products.
19. The implementation remains isolated in the docs worktree branch and updates the existing GitHub PR rather than opening a second PR.

# Tests to add or run

1. Run `pnpm build` in `web/` to catch routing, metadata, and MDX compilation issues.
2. Run the built app locally and verify that `/docs` returns successfully without the AuthKit redirect-URI runtime error.
3. Verify that a generated reference page renders successfully.
4. Verify that the new concept and guide pages render successfully.
5. Verify that `/llms.txt` returns successfully and includes links into the shipped docs set.
6. Verify that `/llms-full.txt` returns successfully and contains multiple concatenated docs sections.
7. Verify that at least one nested `/docs-md/...` route returns markdown/plain text successfully.

# Manual verification

1. Open `/docs` and confirm the docs landing page renders.
2. Open at least one nested page in each implemented section and confirm the sidebar navigation works.
3. Confirm the docs search/filter surfaces relevant pages.
4. Confirm the table of contents appears on a page with headings.
5. Confirm the landing page header links to `/docs`.
6. Confirm docs URLs are emitted by the sitemap source.
7. Confirm `/docs` is reachable locally even when WorkOS redirect configuration is absent.
8. Confirm `/docs/guides/write-a-challenge-pack` explains validate and publish flows grounded in the current CLI/API.
9. Confirm `/docs/concepts/tools-network-and-secrets` explains primitive/composed tools, network controls, and secret references grounded in the current engine.
10. Confirm `/docs/concepts/artifacts` explains upload, reference, and download behavior grounded in the current UI/API.
11. Confirm `/llms.txt` lists the docs index, the full bundle, and section-level markdown exports.
12. Confirm `/llms-full.txt` contains readable bundled markdown for multiple pages.
13. Confirm `/docs-md/guides/write-a-challenge-pack` returns a plain-text/markdown version of the new guide.

# Non-goals for this change

1. Do not add a new third-party docs framework in this pass.
2. Do not try to fully solve the broader public-marketing auth surface beyond making docs reachable.
3. Do not publish roadmap or compare pages unless they are grounded in existing shipped behavior.
4. Do not claim official first-party plugin or deep-link integration with ChatGPT, Codex, Claude Code, or similar tools; provide portable AI-friendly endpoints instead.
5. Do not document speculative future challenge-pack or deployment behavior that is not visible in the current repo surface.
