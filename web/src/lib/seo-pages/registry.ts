import { AGENT_EVALUATION_FEATURES } from "@/lib/seo-features";
import type { SeoPageConfig } from "./types";

const sharedProofPoints = AGENT_EVALUATION_FEATURES;

const sharedDocsLinks = [
  {
    title: "Datasets overview",
    text: "Import examples, record baselines, sync regression suites, and gate CI.",
    href: "/docs/guides/datasets-overview",
  },
  {
    title: "Dataset CI gates",
    text: "Fail builds when a candidate regresses against a pinned baseline.",
    href: "/docs/guides/dataset-ci-gates",
  },
  {
    title: "CI/CD agent gates",
    text: "Block pull requests when agent behavior gets worse.",
    href: "/docs/guides/ci-cd-agent-gates",
  },
] as const;

const quickstartDocsLinks = [
  {
    title: "Quickstart",
    text: "Validate the CLI and get to your first runnable command.",
    href: "/docs/getting-started/quickstart",
  },
  {
    title: "Write a challenge pack",
    text: "Turn a real task into a repeatable agent evaluation.",
    href: "/docs/guides/write-a-challenge-pack",
  },
  {
    title: "CI/CD agent gates",
    text: "Fail a pull request when an agent regresses.",
    href: "/docs/guides/ci-cd-agent-gates",
  },
] as const;

const seoHubLinks = [
  {
    title: "Agent evals",
    text: "Real-task agent evals with replay evidence and CI gates.",
    href: "/agent-evals",
  },
  {
    title: "LLM agent evaluation",
    text: "Evaluate LLM agents on full trajectories, not one-shot answers.",
    href: "/llm-agent-evaluation",
  },
  {
    title: "Compare tools",
    text: "See how AgentClash differs from prompt-eval platforms.",
    href: "/compare",
  },
] as const;

function seoHubLinksFor(path: string): SeoPageConfig["relatedLinks"] {
  return seoHubLinks.filter((link) => link.href !== path);
}

function dedupeRelatedLinks(
  links: SeoPageConfig["relatedLinks"],
): SeoPageConfig["relatedLinks"] {
  const seen = new Set<string>();

  return links.filter((link) => {
    if (seen.has(link.href)) return false;
    seen.add(link.href);
    return true;
  });
}

function page(
  config: Omit<
    SeoPageConfig,
    "proofPoints" | "relatedLinks" | "workflow" | "faqItems"
  > & {
    proofPoints?: string[];
    relatedLinks?: SeoPageConfig["relatedLinks"];
    workflow?: SeoPageConfig["workflow"];
    faqItems: SeoPageConfig["faqItems"];
  },
): SeoPageConfig {
  const { relatedLinks: customRelatedLinks, ...rest } = config;

  return {
    proofPoints: sharedProofPoints,
    relatedLinks: dedupeRelatedLinks([
      ...seoHubLinksFor(config.path),
      ...(customRelatedLinks ?? sharedDocsLinks),
    ]),
    workflow: [
      {
        title: "Import the evidence",
        text: "Start from OpenTelemetry traces, curated datasets, support transcripts, or a real failure your team already saw.",
      },
      {
        title: "Pin the baseline",
        text: "Record the current accepted behavior so every prompt, model, RAG, or tool change has a fair comparison point.",
      },
      {
        title: "Replay the evidence",
        text: "Inspect tool calls, outputs, artifacts, latency, cost, and judge evidence when a candidate gets worse.",
      },
      {
        title: "Gate the release",
        text: "Compare candidate and baseline runs, then fail CI before a regression reaches users.",
      },
    ],
    ...rest,
  };
}

export const SEO_PAGE_REGISTRY: SeoPageConfig[] = [
  page({
    path: "/open-source-ai-agent-evaluation",
    tier: "S",
    keyword: "open source AI agent evaluation",
    intent: "OSS users/founders",
    pageTitle: "Open Source AI Agent Regression Testing - AgentClash",
    metaDescription:
      "Open-source AI agent regression testing for production traces, pinned datasets, replay evidence, baselines, and CI gates.",
    socialImageAlt: "AgentClash open source AI agent evaluation social preview.",
    eyebrow: "Open source",
    h1: "Open source AI agent evaluation for real tasks",
    heroDescription:
      "AgentClash is MIT-licensed and self-hostable. Import traces or curated datasets, keep replay evidence, and turn failures into reusable CI gates without a black-box vendor loop.",
    proofSectionTitle: "What open-source regression testing should include",
    proofSectionDescription:
      "Open source matters when your agent test stack needs to run inside your repo, your CI, and your sandbox policy, not just inside someone else's hosted UI.",
    workflowSectionTitle: "From first eval to team-wide gate",
    docsSectionTitle: "Start with the repo",
    docsSectionDescription:
      "Clone AgentClash, boot the local stack, and wire the same challenge packs into hosted runs or pull request gates.",
    faqSectionTitle: "Questions OSS teams ask before adopting",
    applicationSubCategory: "Open source AI agent evaluation platform",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Open source AI agent evaluation", url: "/open-source-ai-agent-evaluation" },
    ],
    schemaId: "agentclash-open-source-ai-agent-evaluation-schema",
    searchKeywords:
      "open source AI agent regression testing OSS agent eval platform self-hosted agent evaluation OpenTelemetry traces datasets CI gates",
    sitemapTitle: "Open source AI agent evaluation",
    sitemapDescription:
      "MIT-licensed, self-hostable AI agent evaluation with replay, scorecards, and CI gates.",
    faqItems: [
      {
        question: "Is AgentClash actually open source?",
        answer:
          "Yes. AgentClash is MIT-licensed. You can self-host the API server, worker, and web app, or use the hosted product when that is faster for your team.",
      },
      {
        question: "Can we run evals entirely on our own infrastructure?",
        answer:
          "Yes. AgentClash supports local stacks, self-hosted deployments, and sandbox providers you control. Challenge packs and scorecards work the same whether the run is local or hosted.",
      },
      {
        question: "How does open source help with agent regression testing?",
        answer:
          "You can inspect scoring rules, pack definitions, replay artifacts, and CI gate manifests in git. That makes agent eval auditable instead of a dashboard-only black box.",
      },
    ],
    relatedLinks: [
      {
        title: "Self-host guide",
        text: "Boot Postgres, Temporal, API, and worker locally or in your cluster.",
        href: "/docs/getting-started/self-host",
      },
      ...quickstartDocsLinks,
    ],
  }),
  page({
    path: "/agent-evals",
    tier: "S",
    keyword: "agent evals",
    intent: "Dev shorthand",
    pageTitle: "Agent Evals from Traces, Datasets, and CI Gates - AgentClash",
    metaDescription:
      "Run agent evals from production traces and pinned datasets. Compare baselines, replay failures, and block regressions in CI.",
    socialImageAlt: "AgentClash agent evals social preview.",
    eyebrow: "Agent evals",
    h1: "Agent evals that catch production regressions",
    heroDescription:
      "Agent evals should answer whether a change made your agent worse. AgentClash imports traces or curated examples, compares candidates to baselines, and turns failures into release gates.",
    proofSectionTitle: "What useful agent evals capture",
    proofSectionDescription:
      "If your agent eval only checks the final string, you miss the tool misuse, RAG drift, latency spikes, and artifact gaps that show up in production.",
    workflowSectionTitle: "From one eval to a reusable gate",
    docsSectionTitle: "Run your first agent eval",
    docsSectionDescription:
      "Use datasets and challenge packs for repeatable workloads, then promote failures into regression cases your team can run in CI.",
    faqSectionTitle: "Agent eval FAQ",
    applicationSubCategory: "Agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Agent evals", url: "/agent-evals" },
    ],
    schemaId: "agentclash-agent-evals-schema",
    searchKeywords:
      "agent evals agent evaluation LLM evals production traces datasets golden test cases replay scorecards CI gates",
    sitemapTitle: "Agent evals",
    sitemapDescription:
      "Real-task agent evals with replay evidence, scorecards, and CI regression gates.",
    faqItems: [
      {
        question: "What is an agent eval?",
        answer:
          "An agent eval is a repeatable test that runs an agent on a task, scores the full trajectory, and compares the result to a baseline, dataset, or competitor.",
      },
      {
        question: "How is AgentClash different from prompt evals?",
        answer:
          "Prompt evals score one model response. AgentClash evals multi-turn agents that use tools in a sandbox and scores correctness, cost, latency, and evidence quality across the run.",
      },
      {
        question: "Can agent evals run in CI?",
        answer:
          "Yes. AgentClash can compare candidate and baseline scorecards, including dataset baselines, and fail a pull request when the configured release gate regresses.",
      },
    ],
  }),
  page({
    path: "/llm-agent-evaluation",
    tier: "S",
    keyword: "LLM agent evaluation",
    intent: "Broad category",
    pageTitle: "LLM Agent Evaluation on Real Workloads - AgentClash",
    metaDescription:
      "Evaluate LLM agents on real tasks with sandboxed tool use, same-task eval runs, replay trails, scorecards, and CI regression gates.",
    socialImageAlt: "AgentClash LLM agent evaluation social preview.",
    eyebrow: "LLM agents",
    h1: "LLM agent evaluation beyond single-turn answers",
    heroDescription:
      "LLM agents plan, call tools, inspect results, and recover from mistakes. AgentClash evaluates that full loop on your workloads so model swaps and prompt changes do not hide regressions.",
    proofSectionTitle: "Evaluate the agent loop, not just the model",
    proofSectionDescription:
      "LLM agent evaluation needs the same task, same tools, and preserved evidence — otherwise you are comparing demos, not systems.",
    workflowSectionTitle: "A practical LLM agent eval workflow",
    docsSectionTitle: "Bring your workload into AgentClash",
    docsSectionDescription:
      "Start with one real failure, encode it as a challenge pack, then scale to model comparisons and CI gates.",
    faqSectionTitle: "LLM agent evaluation FAQ",
    applicationSubCategory: "LLM agent evaluation platform",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "LLM agent evaluation", url: "/llm-agent-evaluation" },
    ],
    schemaId: "agentclash-llm-agent-evaluation-schema",
    searchKeywords:
      "LLM agent evaluation language model agent eval tool-using agents sandboxed workloads replay scorecards challenge packs",
    sitemapTitle: "LLM agent evaluation",
    sitemapDescription:
      "Evaluate tool-using LLM agents on real tasks with replay and scorecards.",
    faqItems: [
      {
        question: "What should LLM agent evaluation measure?",
        answer:
          "At minimum: task success, tool strategy, artifacts produced, cost, latency, and whether the agent stayed inside policy. AgentClash captures all of that in a scorecard.",
      },
      {
        question: "Can we compare multiple LLM agents fairly?",
        answer:
          "Yes. AgentClash runs candidates on the same challenge pack with the same tool policy, time budget, and sandbox resources.",
      },
      {
        question: "Does AgentClash work with hosted model providers?",
        answer:
          "Yes. AgentClash routes to major LLM providers and normalizes tool-call shapes so evals stay comparable across models.",
      },
    ],
  }),
  page({
    path: "/agent-evaluation-framework",
    tier: "A",
    keyword: "agent evaluation framework",
    intent: "Devs comparing tools",
    pageTitle: "Agent Evaluation Framework for Real Tasks - AgentClash",
    metaDescription:
      "Compare agent evaluation frameworks on what matters: sandboxed execution, replay evidence, scorecards, challenge packs, and CI regression gates.",
    socialImageAlt: "AgentClash agent evaluation framework social preview.",
    eyebrow: "Framework",
    h1: "An agent evaluation framework built for production tasks",
    heroDescription:
      "Teams comparing agent evaluation frameworks should look past leaderboard scores. AgentClash gives you repeatable workloads, same-task eval runs, replay evidence, and release gates you can audit in git.",
    proofSectionTitle: "What a serious framework includes",
    proofSectionDescription:
      "A useful agent evaluation framework packages tasks, enforces fair constraints, scores trajectories, and makes failures reusable.",
    workflowSectionTitle: "Framework workflow",
    docsSectionTitle: "Evaluate before you commit",
    docsSectionDescription:
      "Use AgentClash alongside prompt-eval tools when you need end-to-end agent behavior, not single-call scoring.",
    faqSectionTitle: "Framework comparison FAQ",
    applicationSubCategory: "Agent evaluation framework",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Agent evaluation framework", url: "/agent-evaluation-framework" },
    ],
    schemaId: "agentclash-agent-evaluation-framework-schema",
    searchKeywords:
      "agent evaluation framework compare agent eval tools agent testing framework replay scorecards challenge packs CI gates",
    sitemapTitle: "Agent evaluation framework",
    sitemapDescription:
      "Framework for real-task agent evaluation with replay, scorecards, and CI gates.",
    faqItems: [
      {
        question: "How is AgentClash different from prompt-evaluation frameworks?",
        answer:
          "Prompt-evaluation frameworks score isolated model outputs. AgentClash is an agent-evaluation framework for multi-turn tool-using runs in a sandbox.",
      },
      {
        question: "Can we compare AgentClash with other tools?",
        answer:
          "Yes. See the compare hub for side-by-side notes with Braintrust, LangSmith, Promptfoo, Langfuse, Arize Phoenix, and OpenAI Evals.",
      },
      {
        question: "Does the framework support custom scoring?",
        answer:
          "Yes. Challenge packs carry scoring rules, validators, and judge configuration so teams can encode domain-specific pass conditions.",
      },
    ],
    relatedLinks: [...sharedDocsLinks],
  }),
  page({
    path: "/ai-agent-testing",
    tier: "A",
    keyword: "AI agent testing",
    intent: "Simpler language",
    pageTitle: "AI Agent Testing from Production Traces to CI Gates - AgentClash",
    metaDescription:
      "Test AI agents with production traces, pinned datasets, replay evidence, scorecards, and CI gates that block regressions.",
    socialImageAlt: "AgentClash AI agent testing social preview.",
    eyebrow: "Agent testing",
    h1: "AI agent testing that starts from real failures",
    heroDescription:
      "AI agent testing should feel closer to software testing than prompt tweaking. AgentClash turns traces, golden datasets, and escaped failures into repeatable tests with baseline comparisons.",
    proofSectionTitle: "What production-grade agent testing covers",
    proofSectionDescription:
      "Testing agents means checking behavior across the whole run, including tool calls, retrieval, cost, latency, and artifacts.",
    workflowSectionTitle: "Test loop",
    docsSectionTitle: "Start testing with docs",
    docsSectionDescription:
      "Import a trace or dataset, run an eval, inspect replay, then wire the same workload into CI.",
    faqSectionTitle: "AI agent testing FAQ",
    applicationSubCategory: "AI agent testing software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "AI agent testing", url: "/ai-agent-testing" },
    ],
    schemaId: "agentclash-ai-agent-testing-schema",
    searchKeywords:
      "AI agent testing test AI agents production traces golden datasets LLM regression testing replay scorecards CI gates",
    sitemapTitle: "AI agent testing",
    sitemapDescription:
      "Test AI agents on real tasks with replay evidence and CI regression gates.",
    faqItems: [
      {
        question: "Is AI agent testing different from LLM testing?",
        answer:
          "Yes. LLM testing often means scoring one response. AI agent testing evaluates plans, tool calls, artifacts, recovery behavior, and whether the task actually finished.",
      },
      {
        question: "Can non-ML engineers review agent test failures?",
        answer:
          "Yes. Replay timelines, artifacts, and scorecards are designed for reviewers who need to understand what changed without reading raw model traces alone.",
      },
      {
        question: "How do we keep tests from getting stale?",
        answer:
          "Promote escaped production failures into datasets, challenge packs, and regression suites so the same mistake stays covered after the next model swap.",
      },
    ],
  }),
  page({
    path: "/agent-trajectory-evaluation",
    tier: "A",
    keyword: "agent trajectory evaluation",
    intent: "Technical buyer",
    pageTitle: "Agent Trajectory Evaluation and Replay Evidence - AgentClash",
    metaDescription:
      "Evaluate agent trajectories with replay trails, tool-call evidence, scorecards, challenge packs, and CI gates for baseline versus candidate decisions.",
    socialImageAlt: "AgentClash agent trajectory evaluation social preview.",
    eyebrow: "Trajectories",
    h1: "Agent trajectory evaluation with reviewable evidence",
    heroDescription:
      "The final answer is not enough. AgentClash evaluates the trajectory — tool choices, observations, retries, artifacts, and stop conditions — then preserves replay evidence for auditors and release owners.",
    proofSectionTitle: "Why trajectories matter",
    proofSectionDescription:
      "Two agents can return the same answer while taking wildly different paths. Trajectory evaluation catches unsafe shortcuts, runaway loops, and brittle tool strategies.",
    workflowSectionTitle: "Trajectory eval workflow",
    docsSectionTitle: "Inspect runs with docs",
    docsSectionDescription:
      "Use replay and scorecards to debug trajectory regressions, then encode the workload as a challenge pack for CI.",
    faqSectionTitle: "Trajectory evaluation FAQ",
    applicationSubCategory: "Agent trajectory evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Agent trajectory evaluation", url: "/agent-trajectory-evaluation" },
    ],
    schemaId: "agentclash-agent-trajectory-evaluation-schema",
    searchKeywords:
      "agent trajectory evaluation trajectory scoring tool call replay agent trace evaluation scorecards challenge packs",
    sitemapTitle: "Agent trajectory evaluation",
    sitemapDescription:
      "Score full agent trajectories with replay evidence and release gates.",
    faqItems: [
      {
        question: "What is agent trajectory evaluation?",
        answer:
          "Trajectory evaluation scores the sequence of actions and observations an agent took to complete a task, not just the final output string.",
      },
      {
        question: "How does AgentClash store trajectory evidence?",
        answer:
          "Each run keeps replay events, tool calls, logs, artifacts, and scorecards so reviewers can reconstruct the path that produced the result.",
      },
      {
        question: "Can trajectory evals gate releases?",
        answer:
          "Yes. Compare candidate and baseline trajectories via scorecards, then fail CI when correctness, cost, latency, or evidence quality regresses.",
      },
    ],
    relatedLinks: [
      {
        title: "Agent replay",
        text: "See how AgentClash preserves replay evidence for reviewers.",
        href: "/features/agent-replay",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/ci-cd-agent-evaluation",
    tier: "A",
    keyword: "agent evaluation CI/CD",
    intent: "Release engineering",
    pageTitle: "CI/CD Agent Regression Testing and Gates - AgentClash",
    metaDescription:
      "Wire AI agent regression testing into CI/CD with dataset baselines, trace replay, scorecards, and pull request gates.",
    socialImageAlt: "AgentClash CI/CD agent evaluation social preview.",
    eyebrow: "CI/CD",
    h1: "CI/CD regression gates for AI agents",
    heroDescription:
      "Model, prompt, RAG, and tool changes should not ship on vibes. AgentClash runs repeatable agent workloads in CI, compares candidates to baselines, and blocks merges when behavior regresses.",
    proofSectionTitle: "What CI/CD agent eval needs",
    proofSectionDescription:
      "Release engineering needs pinned datasets, stable scoring, and enough replay evidence to debug a failed gate without reproducing the issue manually.",
    workflowSectionTitle: "CI gate workflow",
    docsSectionTitle: "Wire gates with docs",
    docsSectionDescription:
      "Start with dataset or challenge-pack gates, then promote real failures into cases your pipeline can rerun on every change.",
    faqSectionTitle: "CI/CD agent evaluation FAQ",
    applicationSubCategory: "CI/CD agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "CI/CD agent evaluation", url: "/ci-cd-agent-evaluation" },
    ],
    schemaId: "agentclash-ci-cd-agent-evaluation-schema",
    searchKeywords:
      "CI/CD agent regression testing agent eval CI pull request gates dataset baselines production traces release gates",
    sitemapTitle: "CI/CD agent evaluation",
    sitemapDescription:
      "Run agent evaluation in CI/CD with scorecard gates and replay evidence.",
    faqItems: [
      {
        question: "How do agent eval gates fit into CI/CD?",
        answer:
          "A dataset or challenge pack runs on every candidate change, AgentClash compares the scorecard to a baseline, and the pipeline fails when configured thresholds regress.",
      },
      {
        question: "What happens when a gate fails?",
        answer:
          "Reviewers open replay evidence, inspect tool calls and artifacts, and either fix the regression or update the baseline intentionally.",
      },
      {
        question: "Can gates cover cost and latency budgets?",
        answer:
          "Yes. Scorecards include correctness and evidence quality plus cost and latency signals you can enforce in release policy.",
      },
    ],
    relatedLinks: [
      {
        title: "Agent regression testing",
        text: "Product overview for trace and dataset-driven regression testing.",
        href: "/platform/agent-regression-testing",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/ai-agent-benchmark",
    tier: "A",
    keyword: "AI agent benchmark",
    intent: "Benchmark traffic",
    pageTitle: "AI Agent Benchmark on Real Tasks - AgentClash",
    metaDescription:
      "Run AI agent benchmarks on real workloads with fair constraints, replay evidence, scorecards, and regression gates — not leaderboard-only snapshots.",
    socialImageAlt: "AgentClash AI agent benchmark social preview.",
    eyebrow: "Benchmarks",
    h1: "AI agent benchmarks grounded in real workloads",
    heroDescription:
      "Leaderboards are a starting point, not a release decision. AgentClash lets teams benchmark agents on the tasks they actually ship — with the same tools, budgets, and evidence requirements.",
    proofSectionTitle: "Better than a one-number benchmark",
    proofSectionDescription:
      "A useful AI agent benchmark reports correctness, cost, latency, tool strategy, and artifact quality on workloads your team owns.",
    workflowSectionTitle: "Benchmark workflow",
    docsSectionTitle: "Build a benchmark you can reuse",
    docsSectionDescription:
      "Encode workloads as challenge packs so benchmark runs become regression gates instead of one-off demos.",
    faqSectionTitle: "AI agent benchmark FAQ",
    applicationSubCategory: "AI agent benchmark software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "AI agent benchmark", url: "/ai-agent-benchmark" },
    ],
    schemaId: "agentclash-ai-agent-benchmark-schema",
    searchKeywords:
      "AI agent benchmark agent benchmark real task benchmark coding agent benchmark replay scorecards challenge packs",
    sitemapTitle: "AI agent benchmark",
    sitemapDescription:
      "Benchmark AI agents on real tasks with replay and scorecards.",
    faqItems: [
      {
        question: "How is AgentClash different from public leaderboards?",
        answer:
          "Public leaderboards summarize generic tasks. AgentClash benchmarks your agents on your tools, repositories, APIs, and release constraints.",
      },
      {
        question: "Can we benchmark multiple agents on the same task?",
        answer:
          "Yes. AgentClash runs candidates on the same challenge pack with the same sandbox policy and produces comparable scorecards.",
      },
      {
        question: "Can a benchmark become a regression test?",
        answer:
          "Yes. The same challenge pack can power ad-hoc benchmarks and CI gates once your team trusts the scoring rules.",
      },
    ],
  }),
  page({
    path: "/agent-reliability-benchmark",
    tier: "A",
    keyword: "agent reliability benchmark",
    intent: "Better than generic eval",
    pageTitle: "Agent Reliability Benchmark and Regression Gates - AgentClash",
    metaDescription:
      "Measure agent reliability with repeatable workloads, pass-rate tracking, replay evidence, scorecards, and CI gates for baseline versus candidate runs.",
    socialImageAlt: "AgentClash agent reliability benchmark social preview.",
    eyebrow: "Reliability",
    h1: "Agent reliability benchmarks your team can ship on",
    heroDescription:
      "Reliability is repeatability under constraint. AgentClash benchmarks how often agents finish real tasks correctly, how much evidence they produce, and whether new changes make outcomes worse.",
    proofSectionTitle: "Reliability signals that matter",
    proofSectionDescription:
      "Track success rate, cost stability, latency drift, tool misuse, and promoted failures — not just whether one demo looked impressive.",
    workflowSectionTitle: "Reliability workflow",
    docsSectionTitle: "Turn reliability into gates",
    docsSectionDescription:
      "Use challenge packs and regression suites to keep reliability benchmarks current as models and tools change.",
    faqSectionTitle: "Agent reliability FAQ",
    applicationSubCategory: "Agent reliability benchmark software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Agent reliability benchmark", url: "/agent-reliability-benchmark" },
    ],
    schemaId: "agentclash-agent-reliability-benchmark-schema",
    searchKeywords:
      "agent reliability benchmark reliability eval pass rate regression testing replay scorecards challenge packs",
    sitemapTitle: "Agent reliability benchmark",
    sitemapDescription:
      "Benchmark agent reliability on real tasks with regression gates.",
    faqItems: [
      {
        question: "What makes an agent reliability benchmark useful?",
        answer:
          "It reruns the same real workloads over time, tracks pass rates and cost/latency drift, and preserves evidence when a run fails.",
      },
      {
        question: "How does AgentClash handle flaky agent behavior?",
        answer:
          "Teams can rerun workloads, inspect replay differences, and encode pass@k-style reliability policies in challenge packs and release gates.",
      },
      {
        question: "Can reliability benchmarks block release?",
        answer:
          "Yes. Compare candidate and baseline scorecards in CI and fail the gate when reliability metrics cross your threshold.",
      },
    ],
    relatedLinks: [
      {
        title: "pass@k vs pass^k",
        text: "Read when strict success and independent retries measure different things.",
        href: "/blog/pass-at-k-vs-pass-power-k",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/use-cases/coding-agent-evaluation",
    tier: "B",
    keyword: "evaluate coding agents",
    intent: "Use-case page",
    pageTitle: "Coding Agent Evaluation on Real Repos - AgentClash",
    metaDescription:
      "Evaluate coding agents on real repositories with sandboxed execution, replay evidence, scorecards, challenge packs, and CI regression gates.",
    socialImageAlt: "AgentClash coding agent evaluation social preview.",
    eyebrow: "Use case",
    h1: "Evaluate coding agents on real repositories",
    heroDescription:
      "Coding agents fail on messy repos, flaky tests, and tool limits — not on polished benchmark prompts. AgentClash evaluates patches, test runs, artifacts, and cost on workloads your team actually ships.",
    proofSectionTitle: "What coding agent eval should check",
    proofSectionDescription:
      "Look for correct fixes, sane tool usage, reproducible artifacts, and stable cost/latency — not just a plausible diff in a demo.",
    workflowSectionTitle: "Coding eval workflow",
    docsSectionTitle: "Start with a real bug",
    docsSectionDescription:
      "Promote a failed coding-agent run into a challenge pack, then eval model and harness changes before they reach users.",
    faqSectionTitle: "Coding agent evaluation FAQ",
    applicationSubCategory: "Coding agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Use cases", url: "/use-cases" },
      { name: "Coding agent evaluation", url: "/use-cases/coding-agent-evaluation" },
    ],
    schemaId: "agentclash-coding-agent-evaluation-schema",
    searchKeywords:
      "coding agent evaluation evaluate coding agents software engineering agents repo eval replay scorecards challenge packs",
    sitemapTitle: "Coding agent evaluation",
    sitemapDescription:
      "Evaluate coding agents on real repositories with replay and CI gates.",
    faqItems: [
      {
        question: "Can AgentClash evaluate agents that edit code?",
        answer:
          "Yes. Challenge packs can run agents in sandboxes with repository fixtures, test commands, and artifact checks for patches and logs.",
      },
      {
        question: "How do teams compare coding agents fairly?",
        answer:
          "Run every candidate on the same repo state, tool policy, and time budget, then compare scorecards and replay evidence.",
      },
      {
        question: "Can coding agent evals run in CI?",
        answer:
          "Yes. Wire challenge packs into pull request gates so model, prompt, or tool changes cannot merge when correctness regresses.",
      },
    ],
  }),
  page({
    path: "/use-cases/research-agent-evaluation",
    tier: "B",
    keyword: "evaluate research agents",
    intent: "Use-case page",
    pageTitle: "Research Agent Evaluation with Evidence Quality - AgentClash",
    metaDescription:
      "Evaluate research agents on real investigation tasks with replay evidence, artifact checks, scorecards, challenge packs, and regression gates.",
    socialImageAlt: "AgentClash research agent evaluation social preview.",
    eyebrow: "Use case",
    h1: "Evaluate research agents with evidence quality",
    heroDescription:
      "Research agents live or die on sourcing, synthesis, and artifact quality. AgentClash scores whether an agent found the right evidence, cited it correctly, and finished the investigation under budget.",
    proofSectionTitle: "Research eval signals",
    proofSectionDescription:
      "Measure coverage, citation quality, artifact completeness, and whether the agent stopped with a useful deliverable.",
    workflowSectionTitle: "Research eval workflow",
    docsSectionTitle: "Encode investigations as packs",
    docsSectionDescription:
      "Turn recurring research workflows into challenge packs so every model or prompt change reruns the same investigation fairly.",
    faqSectionTitle: "Research agent evaluation FAQ",
    applicationSubCategory: "Research agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Use cases", url: "/use-cases" },
      { name: "Research agent evaluation", url: "/use-cases/research-agent-evaluation" },
    ],
    schemaId: "agentclash-research-agent-evaluation-schema",
    searchKeywords:
      "research agent evaluation evaluate research agents investigation agents evidence quality replay scorecards challenge packs",
    sitemapTitle: "Research agent evaluation",
    sitemapDescription:
      "Evaluate research agents on investigation tasks with replay and scorecards.",
    faqItems: [
      {
        question: "What should research agent evaluation score?",
        answer:
          "Task completion, evidence quality, artifact completeness, tool discipline, and whether the final synthesis matches the sources collected.",
      },
      {
        question: "Can evals include web or file tools?",
        answer:
          "Yes. Challenge packs define which tools agents may use and which artifacts must be produced for a pass.",
      },
      {
        question: "How do teams debug a failed research run?",
        answer:
          "Replay shows each search, fetch, note, and synthesis step so reviewers can see where the investigation went off track.",
      },
    ],
  }),
  page({
    path: "/use-cases/support-agent-evaluation",
    tier: "B",
    keyword: "evaluate customer support agents",
    intent: "Use-case page",
    pageTitle: "Customer Support Agent Evaluation - AgentClash",
    metaDescription:
      "Evaluate customer support agents on real resolution workflows with replay evidence, scorecards, challenge packs, and CI regression gates.",
    socialImageAlt: "AgentClash support agent evaluation social preview.",
    eyebrow: "Use case",
    h1: "Evaluate customer support agents on real resolutions",
    heroDescription:
      "Support agents must resolve tickets safely, use the right tools, and leave an audit trail. AgentClash evaluates full support trajectories and keeps replay evidence when tone, policy, or resolution quality regresses.",
    proofSectionTitle: "Support eval signals",
    proofSectionDescription:
      "Score resolution correctness, policy adherence, tool usage, escalation behavior, and customer-facing artifact quality.",
    workflowSectionTitle: "Support eval workflow",
    docsSectionTitle: "Start with escaped tickets",
    docsSectionDescription:
      "Promote real support failures into challenge packs and regression suites so the same mistake cannot return after a model update.",
    faqSectionTitle: "Support agent evaluation FAQ",
    applicationSubCategory: "Customer support agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Use cases", url: "/use-cases" },
      { name: "Support agent evaluation", url: "/use-cases/support-agent-evaluation" },
    ],
    schemaId: "agentclash-support-agent-evaluation-schema",
    searchKeywords:
      "customer support agent evaluation support agent eval ticket resolution replay scorecards challenge packs regression testing",
    sitemapTitle: "Support agent evaluation",
    sitemapDescription:
      "Evaluate support agents on resolution workflows with replay and gates.",
    faqItems: [
      {
        question: "Can AgentClash evaluate multi-turn support flows?",
        answer:
          "Yes. Multi-turn challenge packs support scripted, simulated, and human phases for realistic support conversations.",
      },
      {
        question: "How do teams measure policy adherence?",
        answer:
          "Challenge packs encode required actions, forbidden tool use, and validator checks so scorecards reflect policy—not just friendly language.",
      },
      {
        question: "Can support evals gate releases?",
        answer:
          "Yes. Compare candidate and baseline scorecards in CI before deploying a new support agent or model route.",
      },
    ],
  }),
  page({
    path: "/industries/banking",
    tier: "B",
    keyword: "banking agent evaluation",
    intent: "Industry vertical",
    pageTitle: "Banking Agent Evaluation with Replay Evidence - AgentClash",
    metaDescription:
      "Evaluate banking and financial services agents on real workflows with replay evidence, scorecards, challenge packs, and release gates your risk team can review.",
    socialImageAlt: "AgentClash banking agent evaluation social preview.",
    eyebrow: "Industry",
    h1: "Agent evaluation for banking and financial services",
    heroDescription:
      "Banking agents touch payments, account changes, and policy-heavy workflows. AgentClash gives platform teams replay evidence and scorecards to compare candidates before a model change reaches production.",
    proofSectionTitle: "What banking eval should prove",
    proofSectionDescription:
      "Score resolution correctness, tool discipline, artifact quality, and cost stability on workloads your compliance reviewers can replay.",
    workflowSectionTitle: "Banking eval workflow",
    docsSectionTitle: "Encode regulated workflows as packs",
    docsSectionDescription:
      "Turn escaped incidents and approval flows into challenge packs, then gate releases when scorecards regress.",
    faqSectionTitle: "Banking agent evaluation FAQ",
    applicationSubCategory: "Banking agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Industries", url: "/industries" },
      { name: "Banking", url: "/industries/banking" },
    ],
    schemaId: "agentclash-industries-banking-schema",
    searchKeywords:
      "banking agent evaluation financial services agent eval regulated agent testing replay scorecards release gates challenge packs",
    sitemapTitle: "Banking agent evaluation",
    sitemapDescription:
      "Evaluate financial services agents with replay evidence and release gates.",
    faqItems: [
      {
        question: "Does AgentClash certify banking compliance?",
        answer:
          "No. AgentClash provides evaluation evidence, replay, and release gates your team can use in internal approval workflows. Compliance decisions stay with your risk and legal stakeholders.",
      },
      {
        question: "Can we evaluate agents that call internal banking APIs?",
        answer:
          "Yes. Challenge packs run agents in sandboxes with your fixtures, tool policy, and validators so candidates face the same constraints.",
      },
      {
        question: "How do teams gate model changes in banking?",
        answer:
          "Compare candidate and baseline scorecards in CI, attach replay links to change tickets, and fail merges when correctness or policy checks regress.",
      },
    ],
    relatedLinks: [
      {
        title: "Enterprise pilot",
        text: "Start a governed eval program with replay and gates.",
        href: "/enterprise",
      },
      {
        title: "Support agent evaluation",
        text: "Evaluate service workflows with policy checks and replay.",
        href: "/use-cases/support-agent-evaluation",
      },
      {
        title: "Release gate glossary",
        text: "What a release gate means in AgentClash.",
        href: "/glossary/release-gate",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/industries/insurance",
    tier: "B",
    keyword: "insurance agent evaluation",
    intent: "Industry vertical",
    pageTitle: "Insurance Agent Evaluation for Support and Compliance - AgentClash",
    metaDescription:
      "Evaluate insurance support and compliance agents on real claim and policy workflows with replay evidence, scorecards, and regression gates.",
    socialImageAlt: "AgentClash insurance agent evaluation social preview.",
    eyebrow: "Industry",
    h1: "Agent evaluation for insurance support and compliance",
    heroDescription:
      "Insurance agents must follow policy, escalate correctly, and leave an auditable trail. AgentClash evaluates full support trajectories and preserves replay when resolution quality or compliance signals regress.",
    proofSectionTitle: "Insurance eval signals",
    proofSectionDescription:
      "Measure policy adherence, escalation behavior, artifact completeness, multilingual quality where needed, and whether the agent finished with a defensible resolution.",
    workflowSectionTitle: "Insurance eval workflow",
    docsSectionTitle: "Start with escaped claims",
    docsSectionDescription:
      "Promote real claim or policy failures into challenge packs so the same mistake cannot return after a prompt or model update.",
    faqSectionTitle: "Insurance agent evaluation FAQ",
    applicationSubCategory: "Insurance agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Industries", url: "/industries" },
      { name: "Insurance", url: "/industries/insurance" },
    ],
    schemaId: "agentclash-industries-insurance-schema",
    searchKeywords:
      "insurance agent evaluation support agent compliance eval claims workflow replay scorecards challenge packs release gates",
    sitemapTitle: "Insurance agent evaluation",
    sitemapDescription:
      "Evaluate insurance support agents with policy checks and replay.",
    faqItems: [
      {
        question: "Can AgentClash evaluate multi-turn claims conversations?",
        answer:
          "Yes. Multi-turn challenge packs support scripted, simulated, and human phases for realistic insurance support flows.",
      },
      {
        question: "How do teams measure policy adherence?",
        answer:
          "Challenge packs encode required actions, forbidden tool use, and validator checks so scorecards reflect policy, not just friendly language.",
      },
      {
        question: "Does AgentClash replace compliance sign-off?",
        answer:
          "No. AgentClash supplies evaluation evidence and gates. Final compliance and underwriting decisions remain with your organization.",
      },
    ],
    relatedLinks: [
      {
        title: "Enterprise pilot",
        text: "Stand up governed eval for support and compliance agents.",
        href: "/enterprise",
      },
      {
        title: "Support agent evaluation",
        text: "Use-case overview for ticket resolution eval.",
        href: "/use-cases/support-agent-evaluation",
      },
      {
        title: "Challenge pack glossary",
        text: "How packs encode insurance workflows.",
        href: "/glossary/challenge-pack",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/industries/government",
    tier: "B",
    keyword: "government agent evaluation",
    intent: "Industry vertical",
    pageTitle: "Government Agent Evaluation with Audit Trails - AgentClash",
    metaDescription:
      "Evaluate public-sector agents with replay evidence, artifact bundles, scorecards, and release gates that support audit-ready review workflows.",
    socialImageAlt: "AgentClash government agent evaluation social preview.",
    eyebrow: "Industry",
    h1: "Agent evaluation for government and public sector",
    heroDescription:
      "Public-sector agents need traceable decisions, artifact bundles, and repeatable tests before deployment. AgentClash captures replay evidence and scorecards reviewers can attach to change records.",
    proofSectionTitle: "Government eval signals",
    proofSectionDescription:
      "Track task completion, evidence quality, tool discipline, artifact exports, and whether candidate runs regress against an approved baseline.",
    workflowSectionTitle: "Government eval workflow",
    docsSectionTitle: "Build audit-ready packs",
    docsSectionDescription:
      "Encode citizen service workflows as challenge packs with validators and replay links your program office can review.",
    faqSectionTitle: "Government agent evaluation FAQ",
    applicationSubCategory: "Government agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Industries", url: "/industries" },
      { name: "Government", url: "/industries/government" },
    ],
    schemaId: "agentclash-industries-government-schema",
    searchKeywords:
      "government agent evaluation public sector agent eval audit trail evidence bundle replay scorecards release gates challenge packs",
    sitemapTitle: "Government agent evaluation",
    sitemapDescription:
      "Evaluate public-sector agents with replay and evidence bundles.",
    faqItems: [
      {
        question: "Can reviewers export evidence from a run?",
        answer:
          "Yes. Replay captures tool calls, artifacts, and scorecard dimensions so reviewers can attach evidence to internal change and approval workflows.",
      },
      {
        question: "Does AgentClash guarantee FedRAMP or IL compliance?",
        answer:
          "No. AgentClash provides evaluation infrastructure and evidence. Deployment, accreditation, and authority to operate decisions are yours. Enterprise can discuss dedicated deployment during architecture review.",
      },
      {
        question: "How do teams compare vendors or model routes fairly?",
        answer:
          "Run every candidate on the same frozen challenge pack with identical tools and budgets, then compare scorecards and replay side by side.",
      },
    ],
    relatedLinks: [
      {
        title: "Enterprise pilot",
        text: "Discuss residency, deployment, and governed eval for public sector.",
        href: "/enterprise",
      },
      {
        title: "Agent replay feature",
        text: "Inspect trajectories and artifact bundles after each run.",
        href: "/features/agent-replay",
      },
      {
        title: "Agent evaluation glossary",
        text: "Core terms for standing up an eval program.",
        href: "/glossary/agent-evaluation",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/glossary/agent-evaluation",
    tier: "B",
    keyword: "agent evaluation definition",
    intent: "Glossary",
    pageTitle: "What Is Agent Evaluation? - AgentClash Glossary",
    metaDescription:
      "Agent evaluation runs AI agents on repeatable real tasks, scores full trajectories, and produces replay evidence and scorecards for release decisions.",
    socialImageAlt: "AgentClash agent evaluation glossary social preview.",
    eyebrow: "Glossary",
    h1: "What is agent evaluation?",
    heroDescription:
      "Agent evaluation measures whether an AI agent completes a real task correctly under constraint. Unlike prompt tests, it scores the whole trajectory: tools, artifacts, cost, latency, and evidence quality.",
    proofSectionTitle: "How agent evaluation differs",
    proofSectionDescription:
      "Prompt eval checks text from one call. Agent evaluation reruns multi-step work in a sandbox and preserves replay when something fails.",
    workflowSectionTitle: "Typical eval workflow",
    docsSectionTitle: "Go deeper",
    docsSectionDescription:
      "Read the platform overview, then author a challenge pack for your first repeatable eval.",
    faqSectionTitle: "Agent evaluation FAQ",
    applicationSubCategory: "Agent evaluation glossary",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Glossary", url: "/glossary" },
      { name: "Agent evaluation", url: "/glossary/agent-evaluation" },
    ],
    schemaId: "agentclash-glossary-agent-evaluation-schema",
    searchKeywords:
      "agent evaluation definition what is agent evaluation AI agent eval trajectory scoring replay scorecards",
    sitemapTitle: "Agent evaluation (glossary)",
    sitemapDescription:
      "Definition of agent evaluation vs prompt testing.",
    faqItems: [
      {
        question: "Is agent evaluation the same as LLM benchmarking?",
        answer:
          "Benchmarks compare models on fixed tasks. Agent evaluation also covers your prompts, tools, harness, and release gates on workloads you own.",
      },
      {
        question: "What outputs does an agent evaluation produce?",
        answer:
          "A scorecard, replay of the trajectory, artifacts from the run, and a pass or fail against validators and gates you define.",
      },
      {
        question: "Where should teams start?",
        answer:
          "Promote one escaped failure into a challenge pack, establish a baseline run, then compare the next candidate in CI or a benchmark eval.",
      },
    ],
    relatedLinks: [
      {
        title: "Agent evaluation platform",
        text: "Product overview for real-task eval.",
        href: "/platform/agent-evaluation",
      },
      {
        title: "Agent evals landing page",
        text: "SEO hub for evaluation workflows.",
        href: "/agent-evals",
      },
      {
        title: "Glossary index",
        text: "More AgentClash terms.",
        href: "/glossary",
      },
      ...sharedDocsLinks.slice(0, 2),
    ],
  }),
  page({
    path: "/glossary/challenge-pack",
    tier: "B",
    keyword: "challenge pack definition",
    intent: "Glossary",
    pageTitle: "What Is a Challenge Pack? - AgentClash Glossary",
    metaDescription:
      "A challenge pack is a versioned YAML bundle that defines an agent evaluation task: inputs, tools, sandbox, scoring rules, and pass conditions.",
    socialImageAlt: "AgentClash challenge pack glossary social preview.",
    eyebrow: "Glossary",
    h1: "What is a challenge pack?",
    heroDescription:
      "Challenge packs are AgentClash's unit of repeatable agent evaluation. Encode the task once so every model, prompt, or harness change reruns the same workload with the same constraints.",
    proofSectionTitle: "What packs contain",
    proofSectionDescription:
      "Inputs, tool policy, sandbox resources, validators, judges, artifacts, and pass conditions that together define a fair eval or regression test.",
    workflowSectionTitle: "From pack to gate",
    docsSectionTitle: "Authoring resources",
    docsSectionDescription:
      "Use the challenge pack docs and authoring guide to publish your first pack.",
    faqSectionTitle: "Challenge pack FAQ",
    applicationSubCategory: "Challenge pack glossary",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Glossary", url: "/glossary" },
      { name: "Challenge pack", url: "/glossary/challenge-pack" },
    ],
    schemaId: "agentclash-glossary-challenge-pack-schema",
    searchKeywords:
      "challenge pack definition agent evaluation YAML pack scoring rules sandbox tools validators",
    sitemapTitle: "Challenge pack (glossary)",
    sitemapDescription:
      "Definition of AgentClash challenge packs.",
    faqItems: [
      {
        question: "How is a challenge pack versioned?",
        answer:
          "Packs are versioned YAML bundles in your workspace. Pin a version for benchmarks and CI so comparisons stay reproducible.",
      },
      {
        question: "Can one pack power both benchmarks and CI?",
        answer:
          "Yes. The same frozen pack can back a public benchmark eval and an internal release gate once your team trusts the scoring rules.",
      },
      {
        question: "Where are examples?",
        answer:
          "See example packs in the repository and the challenge pack reference docs for field-by-field authoring.",
      },
    ],
    relatedLinks: [
      {
        title: "Challenge packs feature",
        text: "Feature overview for pack-based eval.",
        href: "/features/challenge-packs",
      },
      {
        title: "Challenge pack docs",
        text: "Reference hub for pack authors.",
        href: "/docs/challenge-packs",
      },
      {
        title: "Benchmarks hub",
        text: "Public eval runs on frozen packs.",
        href: "/benchmarks",
      },
      ...sharedDocsLinks.slice(1, 3),
    ],
  }),
  page({
    path: "/glossary/release-gate",
    tier: "B",
    keyword: "release gate definition",
    intent: "Glossary",
    pageTitle: "What Is a Release Gate? - AgentClash Glossary",
    metaDescription:
      "A release gate compares a candidate agent run to a baseline on a challenge pack and blocks promotion when scorecards or validators regress.",
    socialImageAlt: "AgentClash release gate glossary social preview.",
    eyebrow: "Glossary",
    h1: "What is a release gate?",
    heroDescription:
      "A release gate is the pass or fail boundary between a candidate agent run and an approved baseline. AgentClash evaluates both on the same pack and fails CI when correctness, cost, or evidence quality regresses.",
    proofSectionTitle: "What gates enforce",
    proofSectionDescription:
      "Scorecard thresholds, validator failures, and baseline comparisons that stop a model or harness change from reaching users.",
    workflowSectionTitle: "Gate workflow",
    docsSectionTitle: "Wire gates in CI",
    docsSectionDescription:
      "Use the CI/CD agent gates guide to connect manifests, baselines, and pull request checks.",
    faqSectionTitle: "Release gate FAQ",
    applicationSubCategory: "Release gate glossary",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Glossary", url: "/glossary" },
      { name: "Release gate", url: "/glossary/release-gate" },
    ],
    schemaId: "agentclash-glossary-release-gate-schema",
    searchKeywords:
      "release gate definition agent release gate CI regression gate baseline candidate scorecard",
    sitemapTitle: "Release gate (glossary)",
    sitemapDescription:
      "Definition of agent release gates and CI regression checks.",
    faqItems: [
      {
        question: "What is a baseline in a release gate?",
        answer:
          "A baseline is the approved run you compare against, usually pinned by run ID in a CI manifest after a green eval on main.",
      },
      {
        question: "Can gates run outside CI?",
        answer:
          "Yes. Teams also gate manually before production promotion, using the same scorecard and replay evidence produced in CI.",
      },
      {
        question: "How is this different from a linter?",
        answer:
          "Linters check static code. Release gates execute the agent on real tasks and score the full trajectory with replay attached.",
      },
    ],
    relatedLinks: [
      {
        title: "CI/CD agent evaluation",
        text: "Landing page for eval in CI.",
        href: "/ci-cd-agent-evaluation",
      },
      {
        title: "Eval workflows and gates",
        text: "Docs on baselines and manifests.",
        href: "/docs/challenge-packs/eval-workflows-and-gates",
      },
      {
        title: "Enterprise pilot",
        text: "Stand up governed gates with platform support.",
        href: "/enterprise",
      },
      ...sharedDocsLinks.slice(2, 3),
    ],
  }),
  page({
    path: "/features/agent-scorecards",
    tier: "B",
    keyword: "agent scorecard",
    intent: "Feature keyword",
    pageTitle: "Agent Scorecards for Correctness, Cost, and Evidence - AgentClash",
    metaDescription:
      "Agent scorecards summarize correctness, latency, cost, tool strategy, and evidence quality so teams can compare runs and gate releases.",
    socialImageAlt: "AgentClash agent scorecards social preview.",
    eyebrow: "Feature",
    h1: "Agent scorecards that justify a release decision",
    heroDescription:
      "Scorecards turn long agent runs into a reviewable verdict. AgentClash aggregates correctness, cost, latency, tool strategy, and validator evidence so baselines and candidates are easy to compare.",
    proofSectionTitle: "What scorecards include",
    proofSectionDescription:
      "A scorecard should be more than a single number — it should explain why a run passed or failed.",
    workflowSectionTitle: "From run to scorecard",
    docsSectionTitle: "Interpret results",
    docsSectionDescription:
      "Read the interpret-results guide, then wire scorecard thresholds into CI gates and release policy.",
    faqSectionTitle: "Agent scorecard FAQ",
    applicationSubCategory: "Agent scorecard software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Features", url: "/features" },
      { name: "Agent scorecards", url: "/features/agent-scorecards" },
    ],
    schemaId: "agentclash-agent-scorecards-schema",
    searchKeywords:
      "agent scorecard scorecards agent eval results correctness cost latency evidence quality release gates",
    sitemapTitle: "Agent scorecards",
    sitemapDescription:
      "Scorecards for correctness, cost, latency, and evidence quality.",
    faqItems: [
      {
        question: "What appears on an AgentClash scorecard?",
        answer:
          "Correctness signals, validator evidence, cost, latency, tool usage summaries, and dimension scores your challenge pack defines.",
      },
      {
        question: "Can scorecards compare two runs?",
        answer:
          "Yes. Baseline versus candidate comparisons are first-class for regression testing and CI gates.",
      },
      {
        question: "Can scorecards be exported or shared?",
        answer:
          "Yes. Runs keep scorecard views in the product and support shareable evidence for reviewers.",
      },
    ],
    relatedLinks: [
      {
        title: "Interpret results",
        text: "Learn how to read scorecards and replay evidence after a run.",
        href: "/docs/guides/interpret-results",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/features/agent-replay",
    tier: "B",
    keyword: "agent replay",
    intent: "Feature keyword",
    pageTitle: "Agent Replay for Debugging and Review - AgentClash",
    metaDescription:
      "Agent replay preserves tool calls, observations, artifacts, and scorecard evidence so teams can debug agent failures without reproducing them manually.",
    socialImageAlt: "AgentClash agent replay social preview.",
    eyebrow: "Feature",
    h1: "Agent replay that makes failures reviewable",
    heroDescription:
      "When an agent gate fails, reviewers need the path — not a summary. AgentClash replay timelines show tool calls, observations, artifacts, and scorecard context in one place.",
    proofSectionTitle: "Why replay matters",
    proofSectionDescription:
      "Replay turns agent eval from a score into an auditable story of what the agent tried and what the environment returned.",
    workflowSectionTitle: "Replay workflow",
    docsSectionTitle: "Debug with docs",
    docsSectionDescription:
      "Pair replay with interpret-results guidance and challenge packs so every debugged failure becomes a reusable test.",
    faqSectionTitle: "Agent replay FAQ",
    applicationSubCategory: "Agent replay software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Features", url: "/features" },
      { name: "Agent replay", url: "/features/agent-replay" },
    ],
    schemaId: "agentclash-agent-replay-schema",
    searchKeywords:
      "agent replay replay evidence tool call timeline agent trace debugging scorecards challenge packs",
    sitemapTitle: "Agent replay",
    sitemapDescription:
      "Replay tool calls, artifacts, and evidence for agent debugging.",
    faqItems: [
      {
        question: "What does AgentClash replay include?",
        answer:
          "Tool calls, observations, logs, artifacts, timing, and links to the scorecard dimensions that passed or failed.",
      },
      {
        question: "Can replay be shared with reviewers?",
        answer:
          "Yes. Runs support shareable views so PMs, support leads, and engineers can review the same evidence.",
      },
      {
        question: "How does replay help CI failures?",
        answer:
          "When a gate fails, replay shows exactly which step diverged from baseline so you do not guess from a single error string.",
      },
    ],
    relatedLinks: [
      {
        title: "Trajectory evaluation",
        text: "See how replay supports full-path agent scoring.",
        href: "/agent-trajectory-evaluation",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/features/challenge-packs",
    tier: "B",
    keyword: "challenge packs agent evaluation",
    intent: "Your unique term",
    pageTitle: "Challenge Packs for Agent Evaluation - AgentClash",
    metaDescription:
      "Challenge packs turn real agent tasks into repeatable evaluations with tools, scoring rules, artifacts, and CI-ready regression gates.",
    socialImageAlt: "AgentClash challenge packs social preview.",
    eyebrow: "Feature",
    h1: "Challenge packs for repeatable agent evaluation",
    heroDescription:
      "Challenge packs are AgentClash's unit of agent evaluation: a real task, tool policy, scoring rules, and artifacts encoded once so every model or harness change reruns the same workload.",
    proofSectionTitle: "What challenge packs encode",
    proofSectionDescription:
      "Inputs, sandbox resources, allowed tools, validators, judges, and pass conditions — everything needed for a fair, repeatable agent eval.",
    workflowSectionTitle: "Pack lifecycle",
    docsSectionTitle: "Author packs with docs",
    docsSectionDescription:
      "Read the challenge pack docs and authoring guide, then promote escaped failures into packs your whole team can run.",
    faqSectionTitle: "Challenge packs FAQ",
    applicationSubCategory: "Challenge pack agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Features", url: "/features" },
      { name: "Challenge packs", url: "/features/challenge-packs" },
    ],
    schemaId: "agentclash-challenge-packs-schema",
    searchKeywords:
      "challenge packs agent evaluation repeatable agent tasks scoring rules validators CI regression packs",
    sitemapTitle: "Challenge packs",
    sitemapDescription:
      "Repeatable agent evaluation workloads with scoring and CI gates.",
    faqItems: [
      {
        question: "What is a challenge pack?",
        answer:
          "A challenge pack is a versioned agent evaluation workload with inputs, tool policy, scoring rules, and expected artifacts.",
      },
      {
        question: "Can challenge packs run locally and in CI?",
        answer:
          "Yes. The same pack can power exploratory evals, hosted runs, and pull request gates.",
      },
      {
        question: "How do teams create challenge packs?",
        answer:
          "Start from a real failure or release risk, encode it as YAML, and iterate with replay evidence until the scoring rules match what reviewers expect.",
      },
    ],
    relatedLinks: [
      {
        title: "Challenge packs docs",
        text: "Overview of pack structure, scoring, and execution modes.",
        href: "/docs/challenge-packs",
      },
      {
        title: "Write a challenge pack",
        text: "Step-by-step authoring guide for your first pack.",
        href: "/docs/guides/write-a-challenge-pack",
      },
      ...sharedDocsLinks.slice(0, 2),
    ],
  }),
];

const SEO_PAGE_BY_PATH = new Map(
  SEO_PAGE_REGISTRY.map((entry) => [entry.path, entry]),
);

export function getSeoPageByPath(path: string): SeoPageConfig | undefined {
  return SEO_PAGE_BY_PATH.get(path);
}

export function getSeoPagesByPrefix(prefix: string): SeoPageConfig[] {
  return SEO_PAGE_REGISTRY.filter((entry) => entry.path.startsWith(`${prefix}/`));
}

export function getAllSeoPagePaths(): string[] {
  return SEO_PAGE_REGISTRY.map((entry) => entry.path);
}
