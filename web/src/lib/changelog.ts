import { CHANGELOG_PULL_REQUESTS } from "./changelog-pull-requests";

export type ChangelogCategory = "added" | "improved" | "fixed" | "security";

export interface ChangelogPullRequest {
  number: number;
  title: string;
}

export interface ChangelogEntry {
  category: ChangelogCategory;
  text: string;
  detail?: string;
  href?: string;
}

export interface ChangelogPeriod {
  id: string;
  startDate: string;
  endDate: string;
  label: string;
  headline: string;
  summary: string;
  themes: string[];
  entries: ChangelogEntry[];
}

export const CHANGELOG_REPO = "agentclash/agentclash" as const;

export const CHANGELOG_CATEGORY_LABELS: Record<ChangelogCategory, string> = {
  added: "Added",
  improved: "Improved",
  fixed: "Fixed",
  security: "Security",
};

export const CHANGELOG_PERIODS: ChangelogPeriod[] = [
  {
    id: "2026-04-15",
    startDate: "2026-04-15",
    endDate: "2026-04-24",
    label: "Apr 15 – Apr 24, 2026",
    headline: "Scoring depth, failure review, and the first regression suite UI",
    summary:
      "The evaluation engine gained deterministic validators, LLM judge dimensions, and behavioral scoring. Failure review and regression suites landed in the workspace UI, alongside CLI distribution hardening and the xAI provider adapter.",
    themes: [
      "Scoring & validators",
      "Failure review",
      "Regression suites",
      "CLI distribution",
    ],
    entries: [
      {
        category: "added",
        text: "Sandbox code-execution validator and math-equivalence scoring for deterministic checks.",
      },
      {
        category: "added",
        text: "LLM judge dimensions with n-wise comparisons, persisted results, and scorecard rationale cards in the UI.",
      },
      {
        category: "added",
        text: "Behavioral spec scoring — signal extraction, scorecards, and dimension contribution metadata.",
      },
      {
        category: "added",
        text: "Text-generation metrics and token F1 validator with evidence linked back to run events.",
      },
      {
        category: "added",
        text: "Failure review API and workspace Failures page with filters, detail drawer, and run links.",
      },
      {
        category: "added",
        text: "Regression suite CRUD, failure promotion flow, and full regression UI (suites, cases, promotion dialog).",
      },
      {
        category: "added",
        text: "xAI provider adapter wired into the execution engine.",
      },
      {
        category: "added",
        text: "Run replay step detail now surfaces model output and tool results.",
      },
      {
        category: "added",
        text: "Workspace invite emails via Resend.",
      },
      {
        category: "improved",
        text: "Run-agent scorecard redesigned with an inspector-style layout.",
      },
      {
        category: "improved",
        text: "Cross-platform CLI install scripts hardened for macOS, Linux, and Windows.",
      },
      {
        category: "improved",
        text: "Authenticated users redirect from the landing page straight to the dashboard.",
      },
    ],
  },
  {
    id: "2026-04-25",
    startDate: "2026-04-25",
    endDate: "2026-05-04",
    label: "Apr 25 – May 04, 2026",
    headline: "Peer standings, CLI distribution, and a redesigned public site",
    summary:
      "Live peer standings injected into running agents, the CLI shipped through npm with production defaults, and the marketing site got a full redesign — /why, pricing, docs foundation, and broadcast-style run views.",
    themes: [
      "Peer standings",
      "CLI & npm",
      "Public site redesign",
      "Docs foundation",
    ],
    entries: [
      {
        category: "added",
        text: "Live peer standings — injection at step boundaries, newswire formatting, and token split between agents vs comparison context.",
      },
      {
        category: "added",
        text: "CLI `--peer-standings` flags and UI toggle for live comparison standings during runs.",
      },
      {
        category: "added",
        text: "Workflow-first eval commands in the CLI.",
      },
      {
        category: "added",
        text: "Dedicated /why manifesto page and pricing section with tier cards and trial CTA.",
      },
      {
        category: "added",
        text: "Released CLI binaries now default to https://api.agentclash.dev.",
      },
      {
        category: "improved",
        text: "AgentClash-branded auth with liquid-glass login, starfield hero, and 3D clash mark on landing.",
      },
      {
        category: "improved",
        text: "Workspace navigation made instant with client-side prefetching.",
      },
      {
        category: "improved",
        text: "Run detail and replay pages restyled with instrument-panel aesthetics.",
      },
      {
        category: "improved",
        text: "Playground comparison workspace refactored for clearer side-by-side evals.",
      },
    ],
  },
  {
    id: "2026-05-05",
    startDate: "2026-05-05",
    endDate: "2026-05-14",
    label: "May 05 – May 14, 2026",
    headline: "CI intelligence, prompt eval, and GitHub-native workflows",
    summary:
      "Failure clusters, regression provenance, and CI setup generators connected the eval loop to GitHub. Prompt eval CLI commands, the PR comment bot, and E2B harness runners closed the gap between local runs and production gates.",
    themes: [
      "CI & GitHub integration",
      "Failure taxonomy",
      "Prompt eval CLI",
      "Harness runners",
    ],
    entries: [
      {
        category: "added",
        text: "Failure cluster rollups, identity keys, taxonomy classification, and trend charts in the web UI.",
      },
      {
        category: "added",
        text: "Regression provenance, validation signals, proposed-case queue, and remediation hints.",
      },
      {
        category: "added",
        text: "CI setup generators with workspace page and one-click setup pull request creation.",
      },
      {
        category: "added",
        text: "Prompt eval CLI commands — config validation, remote preflight, compile, follow, and GitHub Action mode.",
      },
      {
        category: "added",
        text: "AgentClash PR comment bot with links back to failure review in the UI.",
      },
      {
        category: "added",
        text: "E2B harness runners for Claude Code and OpenClaw agent execution.",
      },
      {
        category: "added",
        text: "Production failure capture and explicit validation runs for proposed regression cases.",
      },
      {
        category: "improved",
        text: "CLI preserves CI curation metadata and surfaces failure taxonomy in workflow output.",
      },
      {
        category: "improved",
        text: "Evaluator validity signals exposed in scorecards for clearer trust boundaries.",
      },
      {
        category: "fixed",
        text: "Dozens of CI, regression, and failure-review edge cases hardened across API and web.",
      },
    ],
  },
  {
    id: "2026-05-15",
    startDate: "2026-05-15",
    endDate: "2026-05-24",
    label: "May 15 – May 24, 2026",
    headline: "Security packs, multi-turn eval, and replay polish",
    summary:
      "Security-family challenge packs, stress-run CLI, and vault boundary harnesses established AgentClash as a security eval surface. Multi-turn evaluation with human takeover and case templating extended the engine beyond single-shot runs.",
    themes: [
      "Security eval",
      "Multi-turn eval",
      "Case templating",
      "Replay polish",
    ],
    entries: [
      {
        category: "security",
        text: "SecurityPolicy schema and SecurityScore dimension for security-family challenge packs.",
      },
      {
        category: "security",
        text: "Canonical secret-hygiene and prompt-injection challenge packs shipped.",
      },
      {
        category: "security",
        text: "CLI `stress-run` subcommand with Anthropic Messages provider and --no-system-guard leak surfacing.",
      },
      {
        category: "security",
        text: "agent-vault-stress harness with real Vault SDK, function calling, campaign mode, and bundled HTTP mock.",
      },
      {
        category: "security",
        text: "Infisical and HashiCorp Vault boundary packs for vault-framed canary leak testing.",
      },
      {
        category: "added",
        text: "Multi-turn evaluation — user simulators, hybrid executor, human takeover, calibration reviews, and post-run arena.",
      },
      {
        category: "added",
        text: "Case template renderer and bundle validation for code-execution challenge packs.",
      },
      {
        category: "added",
        text: "Multi-turn conversation events with transcript helpers in the event model.",
      },
      {
        category: "improved",
        text: "Replay trace output upgraded with IDE-level syntax highlighting and rich text rendering.",
      },
      {
        category: "improved",
        text: "Planted secrets from security packs wired into sandbox provisioning.",
      },
    ],
  },
  {
    id: "2026-05-25",
    startDate: "2026-05-25",
    endDate: "2026-06-01",
    label: "May 25 – Jun 01, 2026",
    headline: "Datasets, /try demos, multi-turn transcripts, and Hermes harness",
    summary:
      "The datasets platform shipped end-to-end — import, generation, trace ingest, eval gates, and full workspace UI. /try CLI demos, multi-turn transcript exports, Hermes harness support, and PostHog analytics rounded out the release window.",
    themes: [
      "Datasets platform",
      "/try CLI demos",
      "Multi-turn transcripts",
      "Agent harnesses",
    ],
    entries: [
      {
        category: "added",
        text: "Datasets foundation — interop import/export, version snapshots, eval runs, baselines, CI gate verdicts, and JUnit output.",
      },
      {
        category: "added",
        text: "Self-Instruct synthetic dataset generation with async jobs and CLI parity.",
      },
      {
        category: "added",
        text: "Production trace ingest, trace candidates, and promote-to-example workflow.",
      },
      {
        category: "added",
        text: "Full dataset management UI in the workspace — import mapping, eval wait, dataset test flow, and generation history.",
      },
      {
        category: "added",
        text: "/try interactive terminal demos with E2B sandbox, free-trial gateway, and Kimi/Qwen/OpenCode integrations.",
      },
      {
        category: "added",
        text: "OpenAI Responses API as a first-class execution mode.",
      },
      {
        category: "added",
        text: "Multi-turn conversation transcript API with replay rendering, scorecard view, and PDF export.",
      },
      {
        category: "added",
        text: "Vibe eval draft persistence for in-progress eval configuration.",
      },
      {
        category: "added",
        text: "Hermes agent harness support in E2B templates and harness execution workflow.",
      },
      {
        category: "added",
        text: "PostHog usage analytics across backend, CLI, and web.",
      },
      {
        category: "improved",
        text: "SEO and AEO discoverability expanded across public marketing and docs surfaces.",
      },
      {
        category: "improved",
        text: "Landing page bar promoting /try CLI demos.",
      },
      {
        category: "fixed",
        text: "Dataset generation provider validation returns clear 400 errors instead of opaque failures.",
      },
    ],
  },
  {
    id: "2026-06-02",
    startDate: "2026-06-02",
    endDate: "2026-06-11",
    label: "Jun 02 – Jun 11, 2026",
    headline: "Portable agent skills and one-command host install",
    summary:
      "Twenty-five portable Agent Skills shipped with docs, an embedded CLI snapshot, and a standalone agent-skills bundle repo. Install or doctor skills on Claude, Codex, Cursor, OpenClaw, Hermes, and OpenCode with one command, or export an offline bundle as a directory or tar.gz.",
    themes: [
      "Portable agent skills",
      "CLI integration install",
      "Skills distribution",
      "CLI ergonomics",
    ],
    entries: [
      {
        category: "added",
        text: "25 portable Agent Skills — hub, quickstart, compare-and-triage, eval runner, challenge-pack authoring, harness setup, multi-turn operator, dataset workflows, CI release gate, security evaluation, and workspace admin.",
        href: "/docs/guides/use-with-ai-tools",
      },
      {
        category: "added",
        text: "`agentclash integration <host> install|doctor` for Claude, Codex, Cursor, OpenClaw, Hermes, and OpenCode — copies the embedded snapshot into each host's skills directory.",
      },
      {
        category: "added",
        text: "`agentclash skills export` — offline install bundles as a flat directory or tar.gz, with optional host-specific layout.",
      },
      {
        category: "added",
        text: "Standalone agent-skills distribution repo with manifest verification and sync from canonical docs source.",
        href: "https://github.com/agentclash/agent-skills",
      },
      {
        category: "added",
        text: "`agentclash schema` — machine-readable CLI command tree for agents and tooling.",
      },
      {
        category: "improved",
        text: "Shell completion now validates its `--shell` argument instead of silently accepting invalid values.",
      },
      {
        category: "improved",
        text: "IndexNow pings and sitemap image entries for faster search-engine discovery.",
      },
      {
        category: "added",
        text: "First public coding-agent benchmark report with frozen Expression Evaluator Arena pack, monthly blog summary, and reproducible scorecard export workflow.",
        href: "/blog/coding-agent-benchmark-june-2026",
      },
      {
        category: "added",
        text: "/benchmarks hub summarizes the measured GPT generations comparison with replay-backed scoreboard and methodology appendix.",
        href: "/benchmarks/gpt-generations-expression-evaluator",
      },
    ],
  },
  {
    id: "2026-06-22",
    startDate: "2026-06-22",
    endDate: "2026-07-01",
    label: "Jun 22 – Jul 01, 2026",
    headline: "DataSmith launch: weak-vs-strong synthetic data for agents",
    summary:
      "DataSmith ships as an open-source Python SDK for high-signal synthetic dataset generation with web-grounded seed construction, OTLP trace ingestion, and SFT/DPO export. AgentClash adds platform marketing, docs, and SEO surfaces for the hosted Agentic Self-Instruct loop.",
    themes: [
      "DataSmith SDK",
      "Synthetic data generation",
      "Agentic Self-Instruct",
      "SEO and discoverability",
    ],
    entries: [
      {
        category: "added",
        text: "DataSmith — open-source Python SDK for weak-vs-strong Agentic Self-Instruct synthetic data generation, inspired by Meta FAIR Autodata.",
        href: "https://github.com/Atharva-Kanherkar/datasmith",
      },
      {
        category: "added",
        text: "/platform/datasmith landing page for the SDK + hosted generation story with FAQ and JSON-LD product schema.",
        href: "/platform/datasmith",
      },
      {
        category: "added",
        text: "Synthetic dataset generation docs guide covering Fast Self-Instruct, Agentic Self-Instruct, CLI, and DataSmith export.",
        href: "/docs/guides/synthetic-dataset-generation",
      },
      {
        category: "added",
        text: "Launch blog: Introducing DataSmith with pipeline overview, trace ingestion, and AgentClash integration table.",
        href: "/blog/introducing-datasmith-synthetic-agent-data",
      },
      {
        category: "added",
        text: "SEO landing pages for synthetic data generation, Agentic Self-Instruct, trace-to-dataset workflows, and glossary definition.",
        href: "/synthetic-data-generation-agents",
      },
      {
        category: "improved",
        text: "Datasets overview and sitemap updated with DataSmith and synthetic generation discovery links.",
        href: "/docs/guides/datasets-overview",
      },
    ],
  },
];

export function getChangelogPeriods(): ChangelogPeriod[] {
  return [...CHANGELOG_PERIODS].sort(
    (a, b) => b.startDate.localeCompare(a.startDate),
  );
}

export function getAllChangelogPeriodSlugs(): string[] {
  return CHANGELOG_PERIODS.map((period) => period.id);
}

export function getChangelogPeriodBySlug(
  slug: string,
): ChangelogPeriod | undefined {
  return CHANGELOG_PERIODS.find((period) => period.id === slug);
}

export function getChangelogPeriodHref(periodId: string): string {
  return `/changelog/${periodId}`;
}

export function getChangelogPullRequestUrl(number: number): string {
  return `https://github.com/${CHANGELOG_REPO}/pull/${number}`;
}

export function getChangelogLatestModified(): string {
  const [latest] = getChangelogPeriods();
  return latest?.endDate ?? CHANGELOG_PERIODS[0]?.startDate ?? "2026-04-15";
}

export const CHANGELOG_FAQ = [
  {
    question: "What is the AgentClash changelog?",
    answer:
      "The AgentClash changelog lists product updates shipped since launch — features, improvements, fixes, and security work — grouped into ten-day release periods with category badges.",
  },
  {
    question: "How often is the AgentClash changelog updated?",
    answer:
      "AgentClash groups changelog entries into ten-day windows. Each period summarizes everything merged during that window, from scoring and regression tooling to datasets, security packs, and CLI releases.",
  },
  {
    question: "Where can I find AgentClash release notes?",
    answer:
      "Visit https://www.agentclash.dev/changelog for the public release timeline. For engineering deep dives, see the AgentClash blog at https://www.agentclash.dev/blog.",
  },
] as const;

export function getChangelogPeriodPullRequests(
  periodId: string,
): ChangelogPullRequest[] {
  return CHANGELOG_PULL_REQUESTS[periodId] ?? [];
}

export function renderChangelogMarkdown(origin = "https://www.agentclash.dev"): string {
  const periods = getChangelogPeriods();
  const lines = [
    "# AgentClash Changelog",
    "",
    "Product updates shipped in ten-day periods since April 15, 2026.",
    "",
    `Source: ${origin}/changelog`,
    "",
  ];

  for (const period of periods) {
    lines.push(
      `## ${period.label}`,
      "",
      period.headline,
      "",
      period.summary,
      "",
      `Period page: ${origin}${getChangelogPeriodHref(period.id)}`,
      "",
    );
    for (const entry of period.entries) {
      lines.push(
        `- **${CHANGELOG_CATEGORY_LABELS[entry.category]}**: ${entry.text}`,
      );
    }
    lines.push("");
  }

  return lines.join("\n").trim();
}
