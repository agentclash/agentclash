import {
  COMPETITORS,
  MARK_LABEL,
  competitorFaq,
  competitorRows,
} from "@/lib/comparison-data";
import {
  DOCS_ORIGIN,
  getAllDocSlugs,
  getDocBySlug,
  renderBlogMarkdown,
  renderDocMarkdown,
} from "@/lib/docs";
import { getAllPosts, getPostBySlug } from "@/lib/blog";
import {
  getAllReports,
  getReportBySlug,
  hasPublishedBenchmarks,
} from "@/lib/benchmarks";
import {
  CHANGELOG_CATEGORY_LABELS,
  getChangelogLatestModified,
  getChangelogPeriodBySlug,
  getChangelogPeriodHref,
  getChangelogPeriods,
  renderChangelogMarkdown,
} from "@/lib/changelog";
import { PRICING_TIERS } from "@/lib/pricing-data";
import { SEO_PAGE_REGISTRY, type SeoPageConfig } from "@/lib/seo-pages";
import { getBundledTryCliDemos } from "@/lib/try-cli/catalog.server";
import type { DemoMeta } from "@/lib/try-cli/types";
import { PUBLIC_MACHINE_CONTRACTS } from "@/lib/public-contracts";

export const PUBLIC_ORIGIN = DOCS_ORIGIN;

export type PublicContentKind =
  | "marketing"
  | "docs"
  | "blog"
  | "changelog"
  | "benchmark"
  | "comparison"
  | "seo"
  | "interactive"
  | "publication";

export type PublicContentInclusion = {
  sitemap: boolean;
  llms: boolean;
  llmsFull: boolean;
  search: boolean;
  indexNow: boolean;
};

// Node-only adapter contract. This module imports filesystem-backed MDX and
// content readers and must never be imported by a Client Component.
export type PublicContentAdapter = {
  canonicalPath: string;
  markdownPath: string;
  title: string;
  description: string;
  kind: PublicContentKind;
  lastModified?: string;
  indexable: boolean;
  includeIn: PublicContentInclusion;
  sitemapPriority?: number;
  changeFrequency?: "daily" | "weekly" | "monthly" | "yearly";
  renderMarkdown: (origin?: string) => string;
};

export type PublicContentDescriptor = PublicContentAdapter;

type StaticSection = {
  heading: string;
  body?: string;
  bullets?: string[];
  links?: Array<{ label: string; href: string }>;
};

type StaticPage = {
  path: string;
  title: string;
  description: string;
  kind?: PublicContentKind;
  lastModified?: string;
  priority: number;
  changeFrequency?: "weekly" | "monthly";
  sections: StaticSection[];
};

const DEFAULT_INCLUDE: PublicContentInclusion = {
  sitemap: true,
  llms: true,
  llmsFull: true,
  search: true,
  indexNow: true,
};

const PRIVATE_PREFIXES = [
  "/api",
  "/auth",
  "/dashboard",
  "/github",
  "/ingest",
  "/invites",
  "/onboard",
  "/orgs",
  "/share",
  "/workspaces",
  "/_next",
] as const;

const STATIC_PAGES: StaticPage[] = [
  {
    path: "/",
    title: "AgentClash: AI Agent Evaluation with Replay Evidence",
    description:
      "Run AI agents on repeatable real tasks, compare trajectories, inspect replay evidence, and turn failures into regression gates.",
    priority: 1,
    changeFrequency: "weekly",
    sections: [
      {
        heading: "What AgentClash does",
        bullets: [
          "Runs agents on the same task, tools, budget, and isolated sandbox.",
          "Scores correctness, reliability, latency, cost, and tool strategy.",
          "Captures replay evidence and promotes failures into reusable regression tests.",
          "Supports hosted evaluation and MIT-licensed self-hosting.",
        ],
      },
      {
        heading: "Start here",
        links: [
          { label: "Quickstart", href: "/docs/getting-started/quickstart" },
          { label: "Run a demo", href: "/try" },
          { label: "Pricing", href: "/pricing" },
          { label: "Compare tools", href: "/compare" },
        ],
      },
    ],
  },
  {
    path: "/why",
    title: "Why AgentClash",
    description:
      "Agent evaluation should test what an agent does across a complete trajectory, not only what one model call says.",
    priority: 0.7,
    sections: [
      {
        heading: "The evaluation gap",
        body: "Prompt tests miss tool choice, recovery, side effects, timing, cost, and multi-turn execution. AgentClash evaluates the complete run with evidence that teams can inspect and replay.",
      },
      {
        heading: "The operating principle",
        bullets: [
          "Same task and budget for every candidate.",
          "Fresh isolated environment for every run.",
          "Deterministic and model-based scoring with visible evidence.",
          "Failures become regression coverage instead of disappearing into dashboards.",
        ],
      },
    ],
  },
  {
    path: "/team",
    title: "AgentClash Team",
    description:
      "Meet the team building open-source infrastructure for repeatable AI agent evaluation.",
    priority: 0.5,
    sections: [
      {
        heading: "What we are building",
        body: "AgentClash gives engineering teams a repeatable way to compare agents, inspect their behavior, and gate releases on evidence.",
      },
      {
        heading: "Work with us",
        links: [
          { label: "GitHub", href: "https://github.com/agentclash/agentclash" },
          { label: "Contact", href: "mailto:hello@agentclash.dev" },
        ],
      },
    ],
  },
  {
    path: "/enterprise",
    title: "Enterprise AI Agent Evaluation",
    description:
      "Governed agent release gates, private evaluation infrastructure, audit evidence, and rollout support for platform teams.",
    priority: 0.82,
    sections: [
      {
        heading: "Enterprise outcomes",
        bullets: [
          "Standardize evidence before agents reach production.",
          "Keep challenge packs, credentials, and replay data within approved boundaries.",
          "Connect regression suites and release gates to CI.",
          "Support SSO, audit logs, retention controls, and custom rollout terms.",
        ],
      },
      {
        heading: "Next step",
        links: [{ label: "Contact AgentClash", href: "mailto:hello@agentclash.dev" }],
      },
    ],
  },
  {
    path: "/services",
    title: "Agent Evaluation Services",
    description:
      "Fixed-scope services for challenge-pack design, regression setup, evaluation pilots, and platform adoption.",
    priority: 0.78,
    sections: [
      {
        heading: "Available engagements",
        bullets: [
          "Evaluation readiness and architecture review.",
          "Challenge-pack and scoring design.",
          "Regression-suite and CI release-gate implementation.",
          "Team enablement for hosted or self-hosted AgentClash.",
        ],
      },
    ],
  },
  {
    path: "/resources/eval-checklist",
    title: "Enterprise AI Agent Evaluation Checklist",
    description:
      "A practical checklist for evaluation design, release gates, pilot evidence, and procurement review.",
    priority: 0.77,
    sections: [
      {
        heading: "Checklist coverage",
        bullets: [
          "Define representative tasks, inputs, tools, and failure boundaries.",
          "Choose deterministic, behavioral, and model-based evaluators.",
          "Capture replay evidence and promote escaped failures.",
          "Set release thresholds, ownership, retention, and audit requirements.",
        ],
      },
    ],
  },
  {
    path: "/platform/agent-evaluation",
    title: "AI Agent Evaluation Platform",
    description:
      "Evaluate tool-using AI agents on real tasks with isolated sandboxes, scorecards, replay evidence, and CI regression gates.",
    priority: 0.85,
    sections: [
      {
        heading: "Evaluation workflow",
        bullets: [
          "Package tasks, inputs, tools, policies, and evaluators as challenge packs.",
          "Race candidate agents under the same constraints.",
          "Inspect scorecards and replay evidence.",
          "Promote failures and gate future releases.",
        ],
      },
    ],
  },
  {
    path: "/platform/agent-regression-testing",
    title: "AI Agent Regression Testing",
    description:
      "Compare baselines and candidates against pinned agent failures and datasets with replay evidence, release thresholds, and CI release gates.",
    priority: 0.82,
    sections: [
      {
        heading: "Regression loop",
        bullets: [
          "Capture failed production or evaluation traces.",
          "Curate them into repeatable cases.",
          "Run baseline and candidate agents on the same suite.",
          "Block regressions in CI with evidence-linked gates.",
        ],
      },
    ],
  },
  {
    path: "/platform/datasmith",
    title: "DataSmith Synthetic Agent Data",
    description:
      "Generate diverse agent evaluation datasets with weak-versus-strong Agentic Self-Instruct workflows.",
    priority: 0.84,
    sections: [
      {
        heading: "DataSmith workflow",
        bullets: [
          "Seed generation from real tasks and constraints.",
          "Create diverse candidate cases with weaker models.",
          "Refine, filter, and score cases with stronger models.",
          "Export approved cases into evaluation datasets and regression suites.",
        ],
      },
    ],
  },
  ...[
    ["/use-cases", "Agent Evaluation Use Cases", "Evaluation patterns for coding, research, and customer-support agents."],
    ["/features", "AgentClash Features", "Challenge packs, scorecards, replay, datasets, regression suites, and CI release gates."],
    ["/industries", "AI Agent Evaluation by Industry", "Evaluation guidance for regulated and evidence-sensitive industries."],
    ["/glossary", "Agent Evaluation Glossary", "Definitions for agent evaluation, challenge packs, release gates, replay evidence, and synthetic data."],
  ].map(([path, title, description]) => ({
    path,
    title,
    description,
    priority: 0.78,
    sections: [
      {
        heading: "Explore",
        body: description,
        links: SEO_PAGE_REGISTRY.filter((page) => page.path.startsWith(`${path}/`)).map(
          (page) => ({ label: page.sitemapTitle, href: page.path }),
        ),
      },
    ],
  })),
  {
    path: "/blog",
    title: "AgentClash Blog",
    description:
      "Engineering guides, benchmark analysis, product updates, and practical AI agent evaluation advice.",
    priority: 0.8,
    changeFrequency: "weekly",
    sections: [
      {
        heading: "Recent articles",
        links: getAllPosts().map((post) => ({
          label: post.title,
          href: `/blog/${post.slug}`,
        })),
      },
    ],
  },
  {
    path: "/benchmarks",
    title: "AI Agent Benchmarks",
    description:
      "Measured same-task agent comparisons with frozen challenge packs, scorecards, and replay evidence.",
    priority: 0.8,
    changeFrequency: "weekly",
    sections: [
      {
        heading: "Reports",
        links: getAllReports()
          .filter((report) => !report.sample)
          .map((report) => ({ label: report.title, href: `/benchmarks/${report.slug}` })),
      },
    ],
  },
  {
    path: "/publications",
    title: "Published Agent Evaluation Artifacts",
    description:
      "Browse explicitly published, redacted challenge packs, scorecards, replays, and agent tryouts.",
    kind: "publication",
    priority: 0.72,
    changeFrequency: "weekly",
    sections: [
      {
        heading: "Publication policy",
        bullets: [
          "Only artifacts whose owners explicitly enable search indexing appear in the catalog.",
          "Every artifact is rendered through a type-specific, redacted public serializer.",
          "Capability-token shares remain private, noindex, and absent from this catalog.",
          "Revoked, expired, unsupported, or indexing-disabled artifacts return 404.",
        ],
      },
    ],
  },
  {
    path: "/try",
    title: "Try AgentClash",
    description:
      "Browse public agent demos and start an isolated terminal only when you explicitly choose a demo.",
    kind: "interactive",
    priority: 0.88,
    changeFrequency: "weekly",
    sections: [
      {
        heading: "How demos work",
        bullets: [
          "Choose a public demo from the server-rendered catalog.",
          "Review its task, constraints, and suggested commands.",
          "Start an isolated sandbox with an explicit action.",
        ],
      },
    ],
  },
  {
    path: "/tryouts",
    title: "AI Agent Tryouts",
    description:
      "Run public task-gated agent tryouts with visible tools, inputs, completed output, and trace export.",
    kind: "interactive",
    priority: 0.9,
    changeFrequency: "weekly",
    sections: [
      {
        heading: "Tryout flow",
        bullets: [
          "Select a public task template and permitted tools.",
          "Review the input before launching the run.",
          "Inspect the completed result and export its trace.",
        ],
      },
    ],
  },
  {
    path: "/agent-opportunity",
    title: "AI Agent Opportunity Report",
    description:
      "Assess a company workflow for agent automation opportunities, evaluation needs, and build-versus-buy tradeoffs.",
    kind: "interactive",
    priority: 0.91,
    changeFrequency: "weekly",
    sections: [
      {
        heading: "POST contract",
        body: "Report generation is an explicit POST operation. Submit a company URL and optional context. The response contains an assessment, candidate workflows, risks, evidence requirements, and next steps. A GET or Markdown request never starts generation.",
      },
      {
        heading: "Publication",
        body: "Generated reports remain transient and non-indexable unless the user explicitly publishes a redacted result.",
      },
    ],
  },
];

function markdownPathFor(canonicalPath: string): string {
  return canonicalPath === "/" ? "/md" : `/md${canonicalPath}`;
}

function absoluteHref(href: string, origin: string): string {
  return href.startsWith("/") ? `${origin}${href}` : href;
}

function renderStaticPage(page: StaticPage, origin: string): string {
  const lines = [
    `# ${page.title}`,
    "",
    page.description,
    "",
    `Source: ${origin}${page.path === "/" ? "" : page.path}`,
    `Markdown export: ${origin}${markdownPathFor(page.path)}`,
  ];

  for (const section of page.sections) {
    lines.push("", `## ${section.heading}`, "");
    if (section.body) lines.push(section.body, "");
    for (const bullet of section.bullets ?? []) lines.push(`- ${bullet}`);
    for (const link of section.links ?? []) {
      lines.push(`- [${link.label}](${absoluteHref(link.href, origin)})`);
    }
  }

  return lines.join("\n").trim();
}

function renderPricing(origin: string): string {
  const lines = [
    "# AgentClash Pricing",
    "",
    "Start free with hosted evaluation or self-host the MIT-licensed engine. Bring your own LLM provider keys on every tier.",
    "",
    `Source: ${origin}/pricing`,
    `Markdown export: ${origin}/md/pricing`,
    "",
    "## Plans",
  ];

  for (const tier of PRICING_TIERS) {
    lines.push(
      "",
      `### ${tier.name}`,
      "",
      tier.blurb,
      "",
      `- Monthly: ${tier.prices.monthly.value}${tier.prices.monthly.suffix}`,
      `- Annual billing: ${tier.prices.yearly.value}${tier.prices.yearly.suffix}${tier.prices.yearly.note ? ` (${tier.prices.yearly.note})` : ""}`,
      ...tier.features.map((feature) => `- ${feature}`),
      `- Action: [${tier.cta.label}](${absoluteHref(tier.cta.href, origin)})`,
    );
  }

  return lines.join("\n").trim();
}

function renderTryCliDemo(demo: DemoMeta, origin: string): string {
  const canonicalPath = `/try/${demo.slug}`;
  const lines = [
    `# Try ${demo.name} in a browser sandbox`,
    "",
    demo.tagline ?? `Run ${demo.name} without installing it locally.`,
    "",
    `Source: ${origin}${canonicalPath}`,
    `Markdown export: ${origin}${markdownPathFor(canonicalPath)}`,
    `Session limit: ${demo.sessionMinutes} minutes`,
    "",
    "A page visit does not create a sandbox. Starting a disposable session requires an explicit action on the HTML page.",
    "",
    "## Suggested commands",
    "",
    ...demo.commands.map((command) => `- **${command.label}**: \`${command.run}\``),
  ];
  if (demo.auth) {
    lines.push(
      "",
      "## Authentication",
      "",
      demo.auth.summary,
      "",
      ...demo.auth.steps.map((step, index) => `${index + 1}. ${step}`),
    );
  }
  if (demo.docs || demo.github) {
    lines.push("", "## Links", "");
    if (demo.docs) lines.push(`- [Documentation](${demo.docs})`);
    if (demo.github) lines.push(`- [Source repository](${demo.github})`);
  }
  return lines.join("\n").trim();
}

function renderSeoPage(page: SeoPageConfig, origin: string): string {
  const lines = [
    `# ${page.h1}`,
    "",
    page.heroDescription,
    "",
    `Source: ${origin}${page.path}`,
    `Markdown export: ${origin}${markdownPathFor(page.path)}`,
    "",
    `## ${page.proofSectionTitle}`,
    "",
    page.proofSectionDescription,
    "",
    ...page.proofPoints.map((point) => `- ${point}`),
    "",
    `## ${page.workflowSectionTitle}`,
    "",
    ...page.workflow.flatMap((step, index) => [
      `### ${index + 1}. ${step.title}`,
      "",
      step.text,
      "",
    ]),
    `## ${page.docsSectionTitle}`,
    "",
    page.docsSectionDescription,
    "",
    ...page.relatedLinks.map(
      (link) => `- [${link.title}](${absoluteHref(link.href, origin)}): ${link.text}`,
    ),
    "",
    `## ${page.faqSectionTitle}`,
    "",
    ...page.faqItems.flatMap((item) => [
      `### ${item.question}`,
      "",
      item.answer,
      "",
    ]),
  ];

  return lines.join("\n").trim();
}

function renderCompareHub(origin: string): string {
  const lines = [
    "# AgentClash vs prompt-evaluation tools",
    "",
    "Compare end-to-end, sandboxed agent evaluation with prompt and model-output evaluation tools.",
    "",
    `Source: ${origin}/compare`,
    `Markdown export: ${origin}/md/compare`,
    "",
    "## Capability matrix",
    "",
    "| Capability | AgentClash | " + COMPETITORS.slice(0, 6).map((item) => item.name).join(" | ") + " |",
    "| --- | --- | " + COMPETITORS.slice(0, 6).map(() => "---").join(" | ") + " |",
  ];

  for (const [index, row] of competitorRows(COMPETITORS[0]).entries()) {
    const competitorValues = COMPETITORS.slice(0, 6).map(
      (competitor) => MARK_LABEL[competitor.verdicts[index]],
    );
    lines.push(`| ${row.label} | ${MARK_LABEL[row.agentclash]} | ${competitorValues.join(" | ")} |`);
  }

  lines.push("", "## Detailed comparisons", "");
  for (const competitor of COMPETITORS) {
    lines.push(`- [AgentClash vs ${competitor.name}](${origin}/md/compare/${competitor.slug})`);
  }
  return lines.join("\n").trim();
}

function renderCompetitor(slug: string, origin: string): string {
  const competitor = COMPETITORS.find((item) => item.slug === slug);
  if (!competitor) return "";
  const lines = [
    `# AgentClash vs ${competitor.name}`,
    "",
    `${competitor.name} is categorized here as ${competitor.tag}. AgentClash focuses on complete, tool-using agent trajectories in isolated sandboxes.`,
    "",
    `Source: ${origin}/compare/${competitor.slug}`,
    `Markdown export: ${origin}/md/compare/${competitor.slug}`,
    "",
    "## Where each tool fits",
    "",
    competitor.whereItFits,
    "",
    "## Capability comparison",
    "",
    `| Capability | AgentClash | ${competitor.name} |`,
    "| --- | --- | --- |",
    ...competitorRows(competitor).map(
      (row) => `| ${row.label} | ${MARK_LABEL[row.agentclash]} | ${MARK_LABEL[row.competitor]} |`,
    ),
    "",
    "## Common questions",
    "",
    ...competitorFaq(competitor).flatMap((item) => [
      `### ${item.question}`,
      "",
      item.answer,
      "",
    ]),
  ];
  return lines.join("\n").trim();
}

function renderChangelogPeriod(slug: string, origin: string): string {
  const period = getChangelogPeriodBySlug(slug);
  if (!period) return "";
  return [
    `# ${period.headline}`,
    "",
    period.summary,
    "",
    `Period: ${period.label}`,
    `Source: ${origin}${getChangelogPeriodHref(period.id)}`,
    `Markdown export: ${origin}/md${getChangelogPeriodHref(period.id)}`,
    "",
    "## Changes",
    "",
    ...period.entries.map(
      (entry) => `- **${CHANGELOG_CATEGORY_LABELS[entry.category]}**: ${entry.text}`,
    ),
  ].join("\n").trim();
}

function formatScore(value: number | null): string {
  return value === null ? "N/A" : `${Math.round(value * 100)}%`;
}

function renderBenchmark(slug: string, origin: string): string {
  const report = getReportBySlug(slug);
  if (!report) return "";
  const lines = [
    `# ${report.title}`,
    "",
    report.description,
    "",
    `Verdict: ${report.verdict}`,
    `Published: ${report.date}`,
    `Author: ${report.author}`,
    `Source: ${origin}/benchmarks/${report.slug}`,
    `Markdown export: ${origin}/md/benchmarks/${report.slug}`,
    "",
    "## Results",
    "",
    "| Rank | Model | Provider | Composite | Correctness | Reliability | Latency | Cost |",
    "| --- | --- | --- | --- | --- | --- | --- | --- |",
    ...report.results.map(
      (row) =>
        `| ${row.rank} | ${row.model} | ${row.provider || "N/A"} | ${formatScore(row.composite)} | ${formatScore(row.correctness)} | ${formatScore(row.reliability)} | ${formatScore(row.latency)} | ${formatScore(row.cost)} |`,
    ),
  ];
  if (report.tasks.length > 0) {
    lines.push("", "## Tasks", "", ...report.tasks.map((task) => `- **${task.name}**: ${task.summary}`));
  }
  lines.push("", report.content.trim());
  return lines.join("\n").trim();
}

function descriptorFromStatic(page: StaticPage): PublicContentDescriptor {
  return {
    canonicalPath: page.path,
    markdownPath: markdownPathFor(page.path),
    title: page.title,
    description: page.description,
    kind: page.kind ?? "marketing",
    lastModified: page.lastModified,
    indexable: page.path !== "/benchmarks" || hasPublishedBenchmarks(),
    includeIn: { ...DEFAULT_INCLUDE },
    sitemapPriority: page.priority,
    changeFrequency: page.changeFrequency ?? "monthly",
    renderMarkdown: (origin = PUBLIC_ORIGIN) =>
      page.path === "/pricing" ? renderPricing(origin) : renderStaticPage(page, origin),
  };
}

function buildStaticRegistry(): PublicContentDescriptor[] {
  const items: PublicContentDescriptor[] = STATIC_PAGES.map(descriptorFromStatic);

  for (const demo of getBundledTryCliDemos()) {
    const canonicalPath = `/try/${demo.slug}`;
    items.push({
      canonicalPath,
      markdownPath: markdownPathFor(canonicalPath),
      title: `Try ${demo.name} in browser`,
      description: demo.tagline ?? `Run ${demo.name} in a disposable browser sandbox.`,
      kind: "interactive",
      indexable: true,
      includeIn: { ...DEFAULT_INCLUDE },
      sitemapPriority: 0.72,
      changeFrequency: "monthly",
      renderMarkdown: (origin = PUBLIC_ORIGIN) => renderTryCliDemo(demo, origin),
    });
  }

  items.push({
    canonicalPath: "/pricing",
    markdownPath: "/md/pricing",
    title: "AgentClash Pricing",
    description:
      "Free hosted and self-hosted options, plus Pro, Team, and Enterprise plans with bring-your-own LLM keys.",
    kind: "marketing",
    indexable: true,
    includeIn: { ...DEFAULT_INCLUDE },
    sitemapPriority: 0.8,
    changeFrequency: "monthly",
    renderMarkdown: (origin = PUBLIC_ORIGIN) => renderPricing(origin),
  });

  for (const page of SEO_PAGE_REGISTRY) {
    items.push({
      canonicalPath: page.path,
      markdownPath: markdownPathFor(page.path),
      title: page.sitemapTitle,
      description: page.sitemapDescription,
      kind: "seo",
      indexable: true,
      includeIn: { ...DEFAULT_INCLUDE },
      sitemapPriority: page.tier === "S" ? 0.8 : page.tier === "A" ? 0.72 : 0.64,
      changeFrequency: "monthly",
      renderMarkdown: (origin = PUBLIC_ORIGIN) => renderSeoPage(page, origin),
    });
  }

  items.push({
    canonicalPath: "/compare",
    markdownPath: "/md/compare",
    title: "AgentClash vs prompt-evaluation tools",
    description:
      "Compare AgentClash with prompt evaluation, observability, and agent evaluation tools.",
    kind: "comparison",
    indexable: true,
    includeIn: { ...DEFAULT_INCLUDE },
    sitemapPriority: 0.8,
    changeFrequency: "monthly",
    renderMarkdown: (origin = PUBLIC_ORIGIN) => renderCompareHub(origin),
  });

  for (const competitor of COMPETITORS) {
    const canonicalPath = `/compare/${competitor.slug}`;
    items.push({
      canonicalPath,
      markdownPath: markdownPathFor(canonicalPath),
      title: `AgentClash vs ${competitor.name}`,
      description: `Compare AgentClash end-to-end agent evaluation with ${competitor.name}, a ${competitor.tag} tool.`,
      kind: "comparison",
      indexable: true,
      includeIn: { ...DEFAULT_INCLUDE },
      sitemapPriority: 0.75,
      changeFrequency: "monthly",
      renderMarkdown: (origin = PUBLIC_ORIGIN) => renderCompetitor(competitor.slug, origin),
    });
  }

  items.push({
    canonicalPath: "/changelog",
    markdownPath: "/md/changelog",
    title: "AgentClash Changelog",
    description: "Product updates, improvements, fixes, and security changes.",
    kind: "changelog",
    lastModified: getChangelogLatestModified(),
    indexable: true,
    includeIn: { ...DEFAULT_INCLUDE },
    sitemapPriority: 0.75,
    changeFrequency: "weekly",
    renderMarkdown: (origin = PUBLIC_ORIGIN) => renderChangelogMarkdown(origin),
  });

  for (const period of getChangelogPeriods()) {
    const canonicalPath = getChangelogPeriodHref(period.id);
    items.push({
      canonicalPath,
      markdownPath: markdownPathFor(canonicalPath),
      title: period.headline,
      description: period.summary,
      kind: "changelog",
      lastModified: period.endDate,
      indexable: true,
      includeIn: { ...DEFAULT_INCLUDE },
      sitemapPriority: 0.65,
      changeFrequency: "monthly",
      renderMarkdown: (origin = PUBLIC_ORIGIN) => renderChangelogPeriod(period.id, origin),
    });
  }

  for (const post of getAllPosts()) {
    const canonicalPath = `/blog/${post.slug}`;
    items.push({
      canonicalPath,
      markdownPath: markdownPathFor(canonicalPath),
      title: post.title,
      description: post.description,
      kind: "blog",
      lastModified: post.dateModified ?? post.date,
      indexable: true,
      includeIn: { ...DEFAULT_INCLUDE },
      sitemapPriority: 0.7,
      changeFrequency: "monthly",
      renderMarkdown: (origin = PUBLIC_ORIGIN) => {
        const fullPost = getPostBySlug(post.slug);
        return fullPost ? renderBlogMarkdown(fullPost, origin) : "";
      },
    });
  }

  for (const slug of getAllDocSlugs()) {
    const doc = getDocBySlug(slug);
    if (!doc) continue;
    items.push({
      canonicalPath: doc.href,
      markdownPath: markdownPathFor(doc.href),
      title: doc.title,
      description: doc.description,
      kind: "docs",
      lastModified: doc.dateModified ?? doc.datePublished,
      indexable: true,
      includeIn: { ...DEFAULT_INCLUDE },
      sitemapPriority: doc.href === "/docs" ? 0.85 : 0.75,
      changeFrequency: "weekly",
      renderMarkdown: (origin = PUBLIC_ORIGIN) => renderDocMarkdown(doc, origin),
    });
  }

  for (const report of getAllReports()) {
    const canonicalPath = `/benchmarks/${report.slug}`;
    const indexable = !report.sample;
    items.push({
      canonicalPath,
      markdownPath: markdownPathFor(canonicalPath),
      title: report.title,
      description: report.description,
      kind: "benchmark",
      lastModified: report.date,
      indexable,
      includeIn: {
        sitemap: indexable,
        llms: indexable,
        llmsFull: indexable,
        search: indexable,
        indexNow: indexable,
      },
      sitemapPriority: 0.75,
      changeFrequency: "monthly",
      renderMarkdown: (origin = PUBLIC_ORIGIN) => renderBenchmark(report.slug, origin),
    });
  }

  const seen = new Set<string>();
  for (const item of items) {
    if (seen.has(item.canonicalPath)) {
      throw new Error(`Duplicate public content path: ${item.canonicalPath}`);
    }
    seen.add(item.canonicalPath);
  }
  return items;
}

let staticRegistry: PublicContentDescriptor[] | undefined;

export function getAllPublicContent(): PublicContentDescriptor[] {
  staticRegistry ??= buildStaticRegistry();
  return [...staticRegistry];
}

export function normalizePublicPath(input: string): string | null {
  if (!input.startsWith("/") || input.includes("?") || input.includes("#")) return null;
  if (input.includes("\\") || input.includes("//")) return null;
  if (/%2f|%5c/i.test(input)) return null;
  let decoded: string;
  try {
    decoded = decodeURIComponent(input);
  } catch {
    return null;
  }
  if (decoded.split("/").some((part) => part === ".." || part === ".")) return null;
  const normalized = decoded.length > 1 ? decoded.replace(/\/+$/, "") : decoded;
  if (
    PRIVATE_PREFIXES.some(
      (prefix) => normalized === prefix || normalized.startsWith(`${prefix}/`),
    )
  ) {
    return null;
  }
  return normalized;
}

export function resolvePublicContent(pathname: string): PublicContentDescriptor | null {
  const normalized = normalizePublicPath(pathname);
  if (!normalized) return null;
  return getAllPublicContent().find((item) => item.canonicalPath === normalized) ?? null;
}

export function canonicalPathFromMarkdownSegments(segments: string[] = []): string | null {
  if (segments.some((segment) => !segment || segment === "." || segment === "..")) return null;
  return normalizePublicPath(segments.length === 0 ? "/" : `/${segments.join("/")}`);
}

function stripMarkdown(value: string): string {
  return value
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/[`*_>#|[\]()~-]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

export function getPublicSearchIndex() {
  return getAllPublicContent()
    .filter((item) => item.indexable && item.includeIn.search)
    .map((item) => ({
      title: item.title,
      description: item.description,
      href: item.canonicalPath,
      searchText: stripMarkdown(
        `${item.title} ${item.description} ${item.renderMarkdown(PUBLIC_ORIGIN).slice(0, 1400)}`,
      ).toLowerCase(),
    }));
}

const KIND_LABELS: Record<PublicContentKind, string> = {
  marketing: "Product and company",
  docs: "Documentation",
  blog: "Blog",
  changelog: "Changelog",
  benchmark: "Benchmarks",
  comparison: "Comparisons",
  seo: "Guides and reference pages",
  interactive: "Interactive tools",
  publication: "Publications",
};

export function buildPublicLlmsIndex(origin = PUBLIC_ORIGIN): string {
  const items = getAllPublicContent().filter((item) => item.indexable && item.includeIn.llms);
  const lines = [
    "# AgentClash",
    "",
    "> AgentClash evaluates AI agents on repeatable real tasks, captures replay evidence, and turns failures into regression gates.",
    "",
    "Use the exact Markdown URLs below instead of guessing routes. Fetch /llms-full.txt for the bundled static corpus.",
    "",
    "## Core entrypoints",
    "",
    `- [Homepage](${origin}/md)`,
    `- [Documentation](${origin}/md/docs)`,
    `- [Quickstart](${origin}/md/docs/getting-started/quickstart)`,
    `- [Pricing](${origin}/md/pricing)`,
    `- [Try AgentClash](${origin}/md/try)`,
    `- [Publications](${origin}/md/publications)`,
    `- [Full static bundle](${origin}/llms-full.txt)`,
    "",
    "## Machine contracts",
    "",
    ...PUBLIC_MACHINE_CONTRACTS.map(
      (contract) => `- [${contract.title}](${origin}${contract.path})`,
    ),
    "",
  ];

  for (const kind of Object.keys(KIND_LABELS) as PublicContentKind[]) {
    const group = items.filter((item) => item.kind === kind);
    if (group.length === 0 || kind === "publication") continue;
    lines.push(`## ${KIND_LABELS[kind]}`, "");
    for (const item of group) {
      lines.push(`- [${item.title}](${origin}${item.markdownPath}) - ${item.description}`);
    }
    lines.push("");
  }

  return lines.join("\n").trim();
}

export function buildPublicLlmsFull(origin = PUBLIC_ORIGIN): string {
  const items = getAllPublicContent().filter(
    (item) => item.indexable && item.includeIn.llmsFull && item.kind !== "publication",
  );
  const lines = [
    "# AgentClash Public Content Bundle",
    "",
    `Canonical site: ${origin}`,
    `Machine-readable index: ${origin}/llms.txt`,
    `Publications catalog: ${origin}/md/publications`,
    "",
    "This bundle contains static public AgentClash content. User-generated publications are intentionally excluded; use the publications catalog for explicitly published artifacts.",
  ];
  for (const item of items) {
    const rendered = item.renderMarkdown(origin).trim();
    if (rendered) lines.push("", "---", "", rendered);
  }
  return lines.join("\n").trim();
}
