# SEO remediation implementation plan

User-approved design: the five-stage SEO remediation plan in the conversation.
Deliver one review-ready PR. Execute stages sequentially with independent review.

## Global Constraints

- Work in the existing isolated worktree. All workloads on THINKBOOK use `/home/ayushparihar/.local/bin/agent-run`; one heavy build/test at a time. Servers bind to `127.0.0.1`.
- Follow root and web AGENTS.md. New marketing headings use Geist, not Instrument Serif. New marketing copy has no em dashes.
- Preserve intentional private/noindex exclusions, publication opt-in, Markdown negotiation, and anonymous public access.
- Add focused observable regression tests with the owning change, not source-text mirrors or duplicate inventory assertions.
- No invented dates, results, customer claims, certifications, integrations, legal policies, or publishing cadence.
- One implementer at a time; fresh task reviewer checks spec and code quality. Resolve blocking findings before advancing.
- Final validation: typecheck, lint, relevant tests, production build, anonymous HTTP crawl with deterministic upstream fixtures, and two independent final reviews.

### Task 1: Repair internal links and Markdown exports

Want: docs HTML and exported Markdown lead to correct destinations.

- Migrate authored internal docs links to canonical root-relative `/docs/...` destinations, resolving from each source file directory, including index.mdx.
- Repair eleven challenge-pack-index destinations and blog RSS `/blog/rss.xml` to `/feed.xml`.
- Replace regex link rewriting in Markdown export with parsed destination handling; use absolute `/md/docs/...` URLs for recognized docs targets. Preserve queries/fragments, assets, external/mailto links, inline/fenced code. Handle inline links, reference definitions, and MDX anchors. Generated skill docs need their own source context.
- Add an authoring check rejecting relative internal page links in authored docs. Use AST positions when possible to avoid unrelated serialization churn. Declare any directly imported parser packages explicitly and keep lockfiles consistent.
- Regression checks: source index and leaf docs, inline/reference/MDX links, query/fragment preservation, assets/external/code unaffected; real export URLs resolve. Verify baseline failures before the fix and focused tests afterward.
- Review: docs/link-resolution reviewer. Commit task and write evidence report.

### Task 2: Connect isolated pages and public navigation

Want: every indexable page is reachable from actual anonymous homepage HTML; key product/collection/tool entries within three links.

- Link `/features`, `/use-cases`, `/industries`, `/glossary` from public navigation.
- Link `/ai-agent-testing` from evaluation content; `/trace-to-dataset` from DataSmith; `/agent-opportunity` and `/tryouts` through public tools; `/publications` from benchmarks and footer.
- Connect `/docs/guides/vibe-evals-existing-agent` and `/docs/vibe-evals/data-use` from related docs with accurate availability context. Preserve Vibe app noindex.
- Update actual homepage navigation/footer as well as applicable shared components. Essential links must appear in initial HTML, not only after an interaction.
- Add focused rendered-link regressions. Full graph crawl belongs to Task 5.
- Review: navigation/SEO reviewer. Commit task and report.

### Task 3: Truthful modification dates

Want: unchanged builds do not advance dates; unknown dates omitted; explicit dates consistent.

- Make `PublicContentAdapter.lastModified` and static page dates optional; remove blanket static date, docs build-time fallback, and homepage changelog-inferred modification date.
- Add optional blog `dateModified` through parser/listing, Open Graph metadata, JSON-LD, sitemap. Retain publication date for feeds and sorting. Explicit known publication dates may serve as initial modification dates.
- Validate authored ISO calendar dates/order/future dates with filename-specific diagnostics through an explicit authoring validation path; do not silently drop malformed files. Preserve API-owned publication updated_at without making API timestamps build-blocking.
- No runtime Git or filesystem mtime inference. New content dates represent the actual authored change; unknown historical dates omitted.
- Regression checks: unchanged builds/frozen clocks, missing dates, valid edits, malformed/future/reversed dates, feed/sort compatibility, optional-date consumers.
- Review: metadata reviewer. Commit task and report.

### Task 4: Distinct useful content and evidence-backed additions

Want: each of 26 SEO pages has a distinct reader question and useful answer, reflected in HTML and Markdown.

- Intents: `/agent-evals` first evaluation; `/llm-agent-evaluation` trajectory versus final answer; `/agent-evaluation-framework` component/selection checklist; `/ai-agent-testing` deterministic checks/judges/regressions; `/ci-cd-agent-evaluation` gate configuration/failure interpretation.
- Other pages: specific proof/workflow/example/limitations; feature inputs/outputs; use-case fixtures/failure checks; glossary definitions/examples; hypothetical industry scenarios without customer/compliance claims. Improve hubs with selection guidance.
- Make hardcoded shared section headings configurable, label illustrative scores, use compliant marketing typography. Share substantive content data between HTML and Markdown.
- Correct unsupported benchmark publishing cadence and evidence/reproduction claims.
- Add `/benchmarks/methodology` as explicit static page/content adapter, reserve report slug, exclude from report feeds/Dataset schema. Explain frozen inputs, constraints, scoring, repetitions, token efficiency versus money, interpretation; existing report remains one task/one run per model.
- Add `/security`: factual vulnerability reporting sourced from SECURITY.md plus links to sandbox/secrets/security-evaluation docs; no invented hosted guarantees. Register both pages in canonical metadata, sitemap, Markdown, public-http negotiation; link from relevant existing pages.
- Privacy/terms, new reports, framework tutorials, case studies remain evidence-dependent follow-ups with explicit source requirements, no indexed placeholders.
- Add meaningful focused rendering/representation tests and verify claims against repository sources.
- Review: content/SEO and factual accuracy. Commit task and report.

### Task 5: Independent coverage, production crawl, CI, rollout notes

Want: prevent recurrence at actual public-output boundaries.

- Independently derive static routes/content expectations from filesystem conventions and dynamic providers. Registry remains authoritative; small reasoned private/machine exclusions only, no second hand-maintained public inventory. No fixed assertion that count is 181.
- Check sitemap canonical origin, unique/indexable permitted paths, optional valid dates; compare baseline and document additions/exclusions.
- Crawl built anonymous HTML on localhost: actual 200 without silent redirects, canonical/meta/header robots agreement, internal destinations, initial-HTML links, homepage graph, key entrypoints <=3 clicks. Handle query/private/sample exclusions.
- Fixture upstream server API before build/server where needed, reuse existing WorkOS fixture, no production bypass flags or live service dependency. Test publication sitemap success/empty/pagination/upstream503 at owning boundaries.
- Integrate focused SEO tests + production smoke into existing frontend CI with paths for external docs inputs (CLI command/config sources, backend API/worker config/.env.example, bundled demos, SECURITY.md as needed). Respect npm lock used by CI.
- Run typecheck/lint/relevant tests/build/crawl; attach results and regression evidence. Final fresh technical and SEO reviews. Record owner-required legal/content inputs and post-deploy sitemap/GSC checks with 28-day indexing/query/click/conversion monitoring, not a PR completion prerequisite.
- Commit task and report.
