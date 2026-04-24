import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { PageHeader } from "@/components/marketing/page-header";
import { SplitSection } from "@/components/marketing/split-section";
import { FeatureGrid } from "@/components/marketing/feature-grid";
import {
  ComparisonTable,
  type ComparisonColumn,
  type ComparisonRow,
} from "@/components/marketing/comparison-table";
import { ClosingCTA } from "@/components/marketing/closing-cta";
import { DemoButton } from "@/components/marketing/demo-button";
import { CodeCard } from "@/components/marketing/code-card";
import { FAQBlock } from "@/components/marketing/faq-block";
import {
  JsonLd,
  breadcrumbSchema,
  productSchema,
} from "@/components/marketing/json-ld";

const PATH = "/v2/vs/langsmith";

export const metadata: Metadata = {
  title: "AgentClash vs LangSmith",
  description:
    "LangSmith is a first-class trace viewer and prompt-eval platform for LangChain apps. AgentClash scores multi-turn agent trajectories head-to-head with real tools in a real sandbox. Honest comparison.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AgentClash vs LangSmith — agent eval vs prompt eval",
    description:
      "LangSmith traces single calls. AgentClash races whole trajectories with real tools. A direct comparison.",
    url: PATH,
  },
};

const COLUMNS: ComparisonColumn[] = [
  { name: "AgentClash", tag: "Agent eval", highlight: true },
  { name: "LangSmith", tag: "Prompt eval + traces" },
];

const ROWS: ComparisonRow[] = [
  {
    label: "Multi-turn agent loops",
    sub: "Races a plan, tool calls, self-correction, and termination — not a single call.",
    cells: ["yes", "partial"],
  },
  {
    label: "Sandboxed tool execution",
    sub: "Ephemeral VM with pinned tools, network policy, and isolated filesystem.",
    cells: ["yes", "no"],
  },
  {
    label: "Head-to-head concurrent race",
    sub: "N models on the same inputs, same time budget, same scorer — side by side.",
    cells: ["yes", "no"],
  },
  {
    label: "Trajectory scoring",
    sub: "Plan adherence, tool-order signatures, self-correction, termination behavior.",
    cells: ["yes", "partial"],
  },
  {
    label: "Cross-provider tool normalization",
    sub: "Same challenge pack runs on OpenAI, Anthropic, Gemini, xAI, Mistral without code changes.",
    cells: ["yes", "partial"],
  },
  {
    label: "Flunks promoted into CI regression gate",
    sub: "Failed runs freeze into a permanent regression suite the next model has to clear.",
    cells: ["yes", "no"],
  },
  {
    label: "LangChain / LangGraph trace viewer",
    sub: "Rich production-trace observability for LangChain apps.",
    cells: ["partial", "yes"],
  },
  {
    label: "Prompt dataset + single-call grading",
    sub: "Classic prompt-eval UX over reference datasets.",
    cells: ["partial", "yes"],
  },
];

const FEATURES = [
  {
    label: "Sandbox",
    title: "Real tools, not stubbed functions.",
    body: "Each agent runs in an isolated E2B environment with real filesystems, real subprocesses, and real network policy. Tool mocks let every model fake the same answer; sandboxes make them earn it.",
  },
  {
    label: "Race",
    title: "Concurrent, same-inputs, same-budget.",
    body: "Fire five providers at the same task with the same tools and the same time budget. The verdict is a fair, reproducible ranking — not a single trace you eyeball after the fact.",
  },
  {
    label: "Trajectory",
    title: "Score the whole path, not the last token.",
    body: "Tool-order signature, plan adherence, self-correction, termination reason. If one model loops on a 500 and another recovers, the verdict shows it.",
  },
  {
    label: "Provider-neutral",
    title: "Not tied to LangChain.",
    body: "Challenge packs run against OpenAI, Anthropic, Gemini, xAI, Mistral, and OpenRouter with normalized tool-call shapes. Zero framework lock-in.",
  },
  {
    label: "CI gate",
    title: "Block the merge on behavior regression.",
    body: "A CLI `regression run` posts verdicts as a GitHub check. When a trajectory shape drifts, reviewers see it in the diff — not in prod a week later.",
  },
  {
    label: "Drift",
    title: "Catch silent provider updates.",
    body: "Nightly races replay archived trajectories against live endpoints. You find out the day a frontier model changed, not after a customer does.",
  },
];

const FAQ_ITEMS = [
  {
    question: "When should I use LangSmith instead?",
    answer:
      "If you're LangChain-native and your primary need is production trace observability, cost attribution across chains, and dataset-driven single-call evals — LangSmith is the right tool. AgentClash does not replace its trace viewer or its LangChain integrations.",
  },
  {
    question: "Can I migrate from LangSmith?",
    answer:
      "Partially. Prompt-eval datasets from LangSmith can be wrapped into AgentClash challenge packs, but you'll want to upgrade them to describe tools, sandboxes, and verdict rubrics. The LangChain trace data stays where it is — most teams keep LangSmith for production observability and add AgentClash for pre-merge agent evaluation.",
  },
  {
    question: "Do I have to drop LangChain to use AgentClash?",
    answer:
      "No. Your application code can keep using LangChain. AgentClash exercises your agent through its provider interface — it only cares that a model returns tool calls it can execute in a sandbox. Most teams run both: LangSmith for production traces, AgentClash for pre-merge races.",
  },
  {
    question: "Why a race instead of a dataset eval?",
    answer:
      "Datasets grade single outputs against a reference. Agents don't produce single outputs — they produce trajectories. Two models can reach the same answer via very different plans, and only one can be trusted to do it when the task gets harder. Racing exposes that delta.",
  },
  {
    question: "Is AgentClash open source?",
    answer:
      "Yes, under FSL-1.1-MIT. The race engine, scoring pipeline, and CLI are fully self-hostable. Managed cloud is an option, not a requirement.",
  },
];

export default function VsLangSmithPage() {
  return (
    <>
      <JsonLd
        id="ld-vs-langsmith-product"
        data={productSchema({
          name: "AgentClash vs LangSmith",
          description:
            "Side-by-side comparison of AgentClash (agent evaluation, trajectory scoring, CI gates) against LangSmith (prompt-eval datasets, LangChain trace observability).",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-vs-langsmith-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Compare", url: "/v2/vs/langsmith" },
          { name: "LangSmith", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Compare" },
            { label: "LangSmith" },
          ]}
          eyebrow="AgentClash vs LangSmith"
          title={
            <>
              Agent eval.
              <br />
              <span className="text-white/40">Not another trace viewer.</span>
            </>
          }
          subtitle={
            <>
              LangSmith is the canonical trace + prompt-eval platform for
              LangChain apps — and it&apos;s very good at that job. AgentClash is
              built for the problem one step over: racing multi-turn agents
              on real tools in a real sandbox and gating CI on the verdict.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton />
              <Link
                href="/v2/oss"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Self-host
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="LangSmith — typical eval"
              language="python"
              code={`# prompt-eval with a reference dataset
from langsmith import Client, evaluate

client = Client()

def target(inputs):
    return chain.invoke(inputs)

evaluate(
    target,
    data="qa-golden-v3",
    evaluators=[exact_match, llm_judge],
    experiment_prefix="gpt-4o-vs-4o-mini",
)`}
            />
          }
        />

        <SplitSection
          eyebrow="Where LangSmith shines"
          title={
            <>
              Production traces.
              <br />
              <span className="text-white/40">Dataset-driven prompt eval.</span>
            </>
          }
          body={
            <>
              <p>
                If you&apos;re already on LangChain or LangGraph, LangSmith is
                hard to beat for trace observability. You can see every chain
                step, cost-attribute tokens to specific nodes, and grade
                reference datasets with the LLM-judge UX that shipped the
                category.
              </p>
              <p className="mt-4">
                That&apos;s a real job, and AgentClash doesn&apos;t try to
                replace it. If your primary need is production observability
                for LangChain apps, LangSmith is the right tool — keep it.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="LangSmith — trace a chain"
              language="python"
              code={`from langsmith import traceable

@traceable(run_type="chain")
def answer(question: str) -> str:
    docs = retriever.invoke(question)
    return llm.invoke({"q": question, "docs": docs})

# Every run shows up in the LangSmith trace tree
answer("who signed the charter?")`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Where AgentClash wins"
          title={
            <>
              Whole trajectories.
              <br />
              <span className="text-white/40">Scored head-to-head.</span>
            </>
          }
          body={
            <>
              <p>
                Agents don&apos;t produce single answers — they produce
                trajectories. Plan, eight tool calls, one recovered error,
                one terminated budget. A dataset eval can&apos;t tell you
                which of three models followed the plan under load.
              </p>
              <p className="mt-4">
                AgentClash races them concurrently in isolated sandboxes
                with real tools, then scores trajectory shape, correctness,
                cost, and latency. Every flunked run freezes into a
                regression the next model has to clear.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="AgentClash — race + verdict"
              code={`$ agentclash run create \\
    --pack rag-qa-hard \\
    --agents gpt-5,claude-4.5,gemini-2.5 \\
    --follow

  gpt-5       ● on-plan    9.1  $0.019  11.2s
  claude-4.5  ● on-plan    8.8  $0.014  13.8s
  gemini-2.5  ◐ looped:2   6.4  $0.022  24.1s

verdict → winner: claude-4.5 (cost-weighted)`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                Side by side
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                What each tool is actually good at.
              </h2>
            </div>
            <div className="mt-16">
              <ComparisonTable columns={COLUMNS} rows={ROWS} />
            </div>
          </div>
        </section>

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                Why teams add AgentClash
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six things a trace viewer can&apos;t do for you.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock
          title="LangSmith vs AgentClash, answered."
          items={FAQ_ITEMS}
          schemaId="ld-vs-langsmith-faq"
        />

        <ClosingCTA
          title={
            <>
              Keep your traces.
              <br />
              <span className="text-white/40">Race your agents.</span>
            </>
          }
          body={
            <p>
              Bring us your hardest multi-turn task. We&apos;ll race it
              against three frontier models in a sandbox and show you the
              trajectory delta LangSmith&apos;s dataset graders can&apos;t
              surface.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors" />
            <Link
              href="/v2/platform/agent-evaluation"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Platform overview
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
