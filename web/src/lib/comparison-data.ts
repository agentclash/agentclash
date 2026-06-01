// Single source of truth for the AgentClash competitor comparison.
//
// These columns and rows power BOTH the landing-page comparison section and the
// /compare pages (hub + per-competitor). Keeping them here prevents the two from
// drifting apart. Verdicts are the positioning AgentClash already publishes on
// the landing page: prompt-eval tools score single model calls; AgentClash races
// tool-using agents head-to-head in a sandbox and scores the whole trajectory.

export type MarkKind = "yes" | "partial" | "no";

export type ComparisonColumn = {
  name: string;
  tag: string;
  highlight?: boolean;
};

export const COMPARISON_COLUMNS: ComparisonColumn[] = [
  { name: "AgentClash", tag: "agent eval", highlight: true },
  { name: "Braintrust", tag: "prompt eval" },
  { name: "LangSmith", tag: "prompt eval" },
  { name: "Promptfoo", tag: "prompt eval" },
  { name: "Langfuse", tag: "prompt eval" },
  { name: "Arize Phoenix", tag: "prompt eval" },
  { name: "OpenAI Evals", tag: "prompt eval" },
];

export type ComparisonRow = {
  label: string;
  sub: string;
  cells: readonly MarkKind[];
};

export const COMPARISON_ROWS: ComparisonRow[] = [
  {
    label: "Multi-turn agent loops",
    sub: "Think → tool → observe → repeat, for minutes, with a fresh environment. Not one prompt → one response.",
    cells: ["yes", "partial", "partial", "no", "partial", "partial", "partial"],
  },
  {
    label: "Sandboxed tool execution",
    sub: "A fresh microVM per agent — real files, real shell, real network, real side effects.",
    cells: ["yes", "no", "no", "no", "no", "no", "no"],
  },
  {
    label: "Head-to-head concurrent race",
    sub: "Every model runs the same task at the same time, on the same budget. No staggered runs, no warm caches.",
    cells: ["yes", "no", "no", "no", "no", "no", "no"],
  },
  {
    label: "Trajectory scoring",
    sub: "Judges the path, not just the final answer — tool-choice efficiency, recovery from error, scope discipline.",
    cells: ["yes", "partial", "partial", "no", "partial", "partial", "no"],
  },
  {
    label: "Cross-provider tool-call normalisation",
    sub: "One schema across OpenAI, Anthropic, Gemini, xAI, Mistral, OpenRouter. Errors classified, retries sane.",
    cells: ["yes", "partial", "partial", "partial", "partial", "partial", "no"],
  },
  {
    label: "Four-vantage composite verdict",
    sub: "Deterministic + mathematic + behavioural + LLM, with consensus aggregation and weights you control.",
    cells: ["yes", "partial", "partial", "partial", "partial", "partial", "partial"],
  },
  {
    label: "Failures auto-promote to regression",
    sub: "Flunked traces freeze into permanent tests and replay in every future race, by default.",
    cells: ["yes", "partial", "partial", "partial", "partial", "partial", "no"],
  },
];

// Human-readable, crawlable cell labels for the semantic comparison <table> on
// the /compare pages (the landing page renders dot glyphs instead).
export const MARK_LABEL: Record<MarkKind, string> = {
  yes: "Yes",
  partial: "Partial",
  no: "No",
};

export const AGENTCLASH_COLUMN_INDEX = COMPARISON_COLUMNS.findIndex(
  (column) => column.highlight,
);

function toKebabCase(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function competitorSlug(name: string): string {
  return `agentclash-vs-${toKebabCase(name)}`;
}

export type Competitor = {
  name: string;
  tag: string;
  slug: string;
  columnIndex: number;
  // Honest, category-level note on where the competitor is the stronger fit.
  // Complimentary by design — establishes credibility and avoids overclaiming.
  whereItFits: string;
};

const COMPETITOR_NOTES: Record<string, string> = {
  Braintrust:
    "Braintrust is a strong choice for prompt and LLM evaluation — datasets, scoring functions, and logging across your app's model calls. Reach for it when the unit you evaluate is a prompt or a single model response.",
  LangSmith:
    "LangSmith excels at tracing, debugging, and evaluating LLM and LangChain apps — deep observability over your chains and prompts. Pick it when tracing and prompt/dataset evals are the priority.",
  Promptfoo:
    "Promptfoo is an excellent open-source, config-first tool for prompt testing and red-teaming across providers. It's a great fit for fast, declarative assertions over model outputs.",
  Langfuse:
    "Langfuse is a strong open-source LLM observability and tracing platform with evals layered on top. Choose it when tracing and analytics over production LLM calls matter most.",
  "Arize Phoenix":
    "Arize Phoenix is great for LLM and ML observability, tracing, and evaluation — especially RAG inspection and production monitoring. Use it when observability and offline evals are the focus.",
  "OpenAI Evals":
    "OpenAI Evals is a solid open framework for building and running model and prompt evals, especially within the OpenAI ecosystem. It fits when you're scoring model outputs against datasets.",
};

export const COMPETITORS: Competitor[] = COMPARISON_COLUMNS.map(
  (column, columnIndex) => ({ column, columnIndex }),
)
  .filter(({ column }) => !column.highlight)
  .map(({ column, columnIndex }) => ({
    name: column.name,
    tag: column.tag,
    slug: competitorSlug(column.name),
    columnIndex,
    whereItFits: COMPETITOR_NOTES[column.name] ?? "",
  }));

export function getCompetitorBySlug(slug: string): Competitor | undefined {
  return COMPETITORS.find((competitor) => competitor.slug === slug);
}

export type CompetitorRow = {
  label: string;
  sub: string;
  agentclash: MarkKind;
  competitor: MarkKind;
};

// Per-row AgentClash-vs-competitor verdict pairs for a single competitor page.
export function competitorRows(competitor: Competitor): CompetitorRow[] {
  return COMPARISON_ROWS.map((row) => ({
    label: row.label,
    sub: row.sub,
    agentclash: row.cells[AGENTCLASH_COLUMN_INDEX],
    competitor: row.cells[competitor.columnIndex],
  }));
}

// Answer-shaped Q&A for a competitor page (rendered visibly + as FAQPage JSON-LD).
export function competitorFaq(
  competitor: Competitor,
): Array<{ question: string; answer: string }> {
  return [
    {
      question: `Is AgentClash a ${competitor.name} alternative?`,
      answer: `AgentClash and ${competitor.name} overlap but solve different problems. ${competitor.name} is a ${competitor.tag} tool, while AgentClash is an agent-evaluation engine that races agents head-to-head on real tasks in a sandbox, scores the full trajectory, and gates CI on regressions. If you need to evaluate tool-using agents end-to-end, AgentClash is the closer fit; for single-call prompt and output scoring, ${competitor.name} may be all you need.`,
    },
    {
      question: `What is the difference between AgentClash and ${competitor.name}?`,
      answer: `${competitor.whereItFits} AgentClash focuses on multi-turn agents that take actions: each model gets a fresh microVM, real tools, the same time budget, and a head-to-head race, and the verdict scores the trajectory — not just the final text.`,
    },
    {
      question: `Can I use AgentClash and ${competitor.name} together?`,
      answer: `Yes. Many teams keep ${competitor.name} for prompt-level evaluation and observability and add AgentClash for end-to-end, sandboxed agent races and CI regression gates. They are complementary layers of an evaluation stack.`,
    },
  ];
}
