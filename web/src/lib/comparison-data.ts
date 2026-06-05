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

// One cell per COMPARISON_COLUMNS entry. Fixed-length tuple so a row with the
// wrong number of cells fails to compile (keep the length in sync with the
// number of columns if a competitor is ever added or removed).
export type Cells = readonly [
  MarkKind,
  MarkKind,
  MarkKind,
  MarkKind,
  MarkKind,
  MarkKind,
  MarkKind,
];

export type ComparisonRow = {
  label: string;
  sub: string;
  cells: Cells;
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
  // Per-row verdict for this competitor's own column, aligned to the
  // COMPARISON_ROWS order. Decoupled from COMPARISON_COLUMNS so a competitor can
  // have a dedicated /compare page without being added as a column to the
  // landing/hub capability matrix (which would make that table unreadably wide).
  verdicts: MarkKind[];
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

// Competitors that appear as columns in the landing/hub capability matrix.
// Their per-row verdicts are read straight out of COMPARISON_ROWS so the matrix
// and the per-competitor pages can never drift apart.
const matrixCompetitors: Competitor[] = COMPARISON_COLUMNS.map(
  (column, columnIndex) => ({ column, columnIndex }),
)
  .filter(({ column }) => !column.highlight)
  .map(({ column, columnIndex }) => ({
    name: column.name,
    tag: column.tag,
    slug: competitorSlug(column.name),
    verdicts: COMPARISON_ROWS.map((row) => row.cells[columnIndex]),
    whereItFits: COMPETITOR_NOTES[column.name] ?? "",
  }));

// Additional competitors with dedicated /compare pages but intentionally NOT
// shown as columns in the landing/hub capability matrix (that table stays at the
// six prompt-eval tools so it remains readable). Each verdict tuple is aligned to
// COMPARISON_ROWS order and reflects the tool's real, current capabilities —
// honest and complimentary, never overclaiming. Sandboxed tool execution and the
// head-to-head concurrent race are AgentClash-specific, so they read "no" here.
type ExtendedCompetitor = {
  name: string;
  tag: string;
  whereItFits: string;
  verdicts: Cells;
};

const EXTENDED_COMPETITORS: ExtendedCompetitor[] = [
  {
    name: "DeepEval",
    tag: "LLM eval framework",
    whereItFits:
      "DeepEval is an excellent open-source, Pytest-style LLM evaluation framework — 50+ research-backed metrics including tool correctness, task completion, and multi-turn simulation, run locally or in CI. Reach for it when you want code-first, metric-on-trace evals inside your existing test suite.",
    // multi-turn (sim, trace-based), no sandbox, no race, trajectory (tool
    // correctness/trace), cross-provider judges, composite (50+ metrics), CI
    // dataset regression.
    verdicts: ["partial", "no", "no", "partial", "partial", "partial", "partial"],
  },
  {
    name: "Galileo",
    tag: "LLM observability",
    whereItFits:
      "Galileo is a strong LLM observability and evaluation platform with purpose-built agentic metrics like tool selection quality and session success, plus real-time guardrails. Choose it when production observability and guardrailing of live agents matter most.",
    verdicts: ["partial", "no", "no", "partial", "partial", "partial", "no"],
  },
  {
    name: "Patronus AI",
    tag: "eval & guardrails",
    whereItFits:
      "Patronus AI is great for LLM evaluation, guardrails, and agent trace diagnosis — its Percival agent localizes reasoning, planning, and execution faults across long traces. Use it when you need to monitor and debug failing agents at scale.",
    verdicts: ["partial", "no", "no", "partial", "partial", "partial", "no"],
  },
  {
    name: "Ragas",
    tag: "RAG & agent eval",
    whereItFits:
      "Ragas is an excellent open-source evaluation framework for RAG and agentic workflows, with metrics like agent goal accuracy and tool-call accuracy over complete traces. Pick it when retrieval quality and trace-based agent metrics are the focus.",
    verdicts: ["partial", "no", "no", "partial", "partial", "no", "no"],
  },
];

export const COMPETITORS: Competitor[] = [
  ...matrixCompetitors,
  ...EXTENDED_COMPETITORS.map((competitor) => ({
    name: competitor.name,
    tag: competitor.tag,
    slug: competitorSlug(competitor.name),
    verdicts: [...competitor.verdicts],
    whereItFits: competitor.whereItFits,
  })),
];

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
  return COMPARISON_ROWS.map((row, index) => ({
    label: row.label,
    sub: row.sub,
    agentclash: row.cells[AGENTCLASH_COLUMN_INDEX],
    competitor: competitor.verdicts[index],
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
