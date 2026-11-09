export const BENCHMARKS_HUB_FAQ = [
  { question: "What does the public report compare?", answer: "The expression evaluator report compares four OpenAI models on one task, with one run per model. It is not a general model ranking." },
  { question: "Can I reproduce the exact results?", answer: "The report describes its configuration, but the fixture link points to mutable main and raw replay artifacts are not attached. Treat it as a starting point for your own evaluation." },
  { question: "How often are reports published?", answer: "Reports are published when available. Check the hub or RSS feed for updates." },
] as const;

export const BENCHMARKS_METHODOLOGY = [
  { title: "Freeze inputs", text: "For a controlled comparison, record the pack commit, inputs, models, tool policy and budgets." },
  { title: "Interpret scoring", text: "Read deterministic checks and judge dimensions separately. Token efficiency is not dollar spend." },
  { title: "Repeat runs", text: "Multiple runs and reported variation help assess reliability; a single run does not estimate a pass rate." },
  { title: "Read the caveats", text: "The expression evaluator report describes one task and one run per model. Its mutable fixture link is not an exact reproduction recipe." },
] as const;

export const BENCHMARKS_CHILD_PAGES = [
  {
    href: "/ai-agent-benchmark",
    title: "AI agent benchmark",
    text: "Benchmark agents on workloads your team owns, with replay and scorecards instead of leaderboard-only snapshots.",
  },
  {
    href: "/agent-reliability-benchmark",
    title: "Agent reliability benchmark",
    text: "Track pass rates, cost drift, and promoted failures with repeatable packs and CI regression gates.",
  },
] as const;

export const BENCHMARKS_MONTHLY_BLOG = {
  href: "/blog/coding-agent-benchmark-june-2026",
  title: "Coding agent benchmark: June 2026",
  text: "Summary of the reported expression evaluator results and their limits.",
} as const;

export const BENCHMARKS_RUNBOOK_HREF =
  "https://github.com/agentclash/agentclash/blob/main/docs/marketing/model-benchmark-workflow.md" as const;

export const BENCHMARKS_READING = [
  {
    href: BENCHMARKS_MONTHLY_BLOG.href,
    title: BENCHMARKS_MONTHLY_BLOG.title,
  },
  {
    href: "/blog/benchmark-ai-agents-on-your-own-data",
    title: "Benchmark AI agents on your own data",
  },
  {
    href: "/blog/how-agentclash-scores-agent-trajectories",
    title: "How AgentClash scores agent trajectories",
  },
  {
    href: "/blog/pass-k-reliability-enterprise-teams",
    title: "Pass@k reliability for enterprise teams",
  },
  {
    href: "/blog/why-agentclash-races-agents-head-to-head",
    title: "Why AgentClash finds agent failures on real tasks",
  },
] as const;

export const BENCHMARKS_PACK_LINKS = [
  {
    href: "/docs/challenge-packs",
    title: "Challenge pack reference",
    text: "Author versioned YAML packs with scoring, tools, sandbox policy, and eval workflows.",
  },
  {
    href: "/docs/challenge-packs/eval-workflows-and-gates",
    title: "Eval workflows and gates",
    text: "Wire baseline versus candidate comparisons into CI release policy.",
  },
  {
    href: "https://github.com/agentclash/agentclash/tree/main/examples/challenge-packs",
    title: "Example packs in the repo",
    text: "Start from expression evaluators, refund recovery, incident response, and security stress packs.",
    external: true,
  },
] as const;

export const BENCHMARKS_MONTHLY_PROCESS = [
  "Record the pack version, commit, inputs, models, and runtime constraints.",
  "Run candidates under the same tools, network policy, and budgets.",
  "Preserve scorecards and evidence, then report sample sizes and limitations.",
] as const;
