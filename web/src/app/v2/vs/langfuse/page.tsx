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

const PATH = "/v2/vs/langfuse";

export const metadata: Metadata = {
  title: "AgentClash vs Langfuse",
  description:
    "Langfuse is an excellent OSS LLM observability platform — production traces, cost attribution, dataset grading. AgentClash is built one room over: racing multi-turn agents with real tools and gating CI on the verdict.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AgentClash vs Langfuse — agent eval vs LLM observability",
    description:
      "Langfuse traces production LLM calls. AgentClash races pre-merge agent trajectories in a sandbox. A direct comparison.",
    url: PATH,
  },
};

const COLUMNS: ComparisonColumn[] = [
  { name: "AgentClash", tag: "Agent eval", highlight: true },
  { name: "Langfuse", tag: "LLM observability + evals" },
];

const ROWS: ComparisonRow[] = [
  {
    label: "Multi-turn agent loops",
    sub: "Scores plans, tool calls, self-correction, and termination — not a single LLM call.",
    cells: ["yes", "partial"],
  },
  {
    label: "Sandboxed tool execution",
    sub: "Ephemeral VM with real filesystem, subprocesses, and network-policy isolation.",
    cells: ["yes", "no"],
  },
  {
    label: "Head-to-head concurrent race",
    sub: "N models on the same inputs, same budget, same scorer — a single verdict.",
    cells: ["yes", "no"],
  },
  {
    label: "Trajectory scoring",
    sub: "Plan adherence, tool-order signatures, self-correction, termination reason.",
    cells: ["yes", "partial"],
  },
  {
    label: "CI regression gate",
    sub: "Flunked runs freeze into a permanent regression suite the next model has to clear.",
    cells: ["yes", "partial"],
  },
  {
    label: "Cross-provider tool normalization",
    sub: "One challenge pack runs on OpenAI, Anthropic, Gemini, xAI, Mistral without code changes.",
    cells: ["yes", "partial"],
  },
  {
    label: "Production trace ingestion",
    sub: "High-volume SDK ingest of every LLM call from your live app.",
    cells: ["partial", "yes"],
  },
  {
    label: "Cost & token attribution dashboards",
    sub: "Production spend analytics sliced by user, feature, or trace.",
    cells: ["partial", "yes"],
  },
];

const FEATURES = [
  {
    label: "Sandbox",
    title: "Tools that actually execute.",
    body: "Agents run in ephemeral E2B environments with real filesystems, subprocesses, and network policy. Observability platforms trace what happened; sandboxes produce what should happen.",
  },
  {
    label: "Race",
    title: "Concurrent, same-budget competition.",
    body: "Fire N providers at the same inputs in parallel. The verdict is a single ranking — not a pile of traces to cross-filter after the fact.",
  },
  {
    label: "Trajectory",
    title: "Score the path, not the token stream.",
    body: "Tool-order signature, plan adherence, self-correction, termination reason. A trace viewer shows you the what; trajectory scoring tells you which agent to ship.",
  },
  {
    label: "CI native",
    title: "Block merges on behavior drift.",
    body: "`agentclash regression run` posts verdicts as GitHub checks and links straight to the failing replay. Flunks become permanent regressions.",
  },
  {
    label: "Durable",
    title: "Temporal-backed workflows.",
    body: "Long-running agent runs survive provider 503s, node restarts, and flaky tools. Replays work months later because events are durably stored.",
  },
  {
    label: "Provider-neutral",
    title: "Every major provider, normalized.",
    body: "OpenAI, Anthropic, Gemini, xAI, Mistral, OpenRouter. Tool-call shapes and failure codes normalized, so scoring is apples-to-apples across the board.",
  },
];

const FAQ_ITEMS = [
  {
    question: "When should I use Langfuse instead?",
    answer:
      "If your primary job is production observability — ingesting every LLM call your app makes, attributing cost, debugging latency, and running lightweight dataset evals against traced outputs — Langfuse is an excellent, open-source choice. AgentClash doesn't try to replace its trace ingestion or its cost dashboards.",
  },
  {
    question: "Can I migrate from Langfuse?",
    answer:
      "Partially. Langfuse dataset evals can be lifted into AgentClash challenge packs, but you'll want to upgrade them to describe tools, sandboxes, and trajectory rubrics. The trace ingestion story stays where it is — most teams run Langfuse for production observability and add AgentClash for pre-merge agent evaluation.",
  },
  {
    question: "Does AgentClash do production observability?",
    answer:
      "Not as its primary job. We emit durable events for every race and ship a full replay viewer, but we're not built to be the firehose for every LLM call your live app makes. If that's what you need, keep Langfuse.",
  },
  {
    question: "Is AgentClash open source?",
    answer:
      "Yes, under FSL-1.1-MIT. The race engine, scoring pipeline, and CLI are fully self-hostable. Managed cloud is an option, not a requirement.",
  },
  {
    question: "What's the CI story?",
    answer:
      "The CLI is designed for CI. `agentclash run create` kicks off durable workflows, `agentclash regression run` gates the PR, and verdicts post as GitHub checks that link to the failing replay. Flunks freeze into a permanent regression suite, so the next model ships against a higher bar than the last.",
  },
];

export default function VsLangfusePage() {
  return (
    <>
      <JsonLd
        id="ld-vs-langfuse-product"
        data={productSchema({
          name: "AgentClash vs Langfuse",
          description:
            "Side-by-side comparison of AgentClash (agent evaluation, trajectory scoring, CI gates) against Langfuse (OSS LLM observability, production trace ingestion, cost attribution).",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-vs-langfuse-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Compare", url: "/v2/vs/langfuse" },
          { name: "Langfuse", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Compare" },
            { label: "Langfuse" },
          ]}
          eyebrow="AgentClash vs Langfuse"
          title={
            <>
              Pre-merge verdicts.
              <br />
              <span className="text-white/40">Not post-hoc traces.</span>
            </>
          }
          subtitle={
            <>
              Langfuse is an excellent OSS LLM observability platform — it
              ingests traces from your live app, attributes cost, and runs
              lightweight dataset evals. AgentClash is built one room over:
              racing multi-turn agents with real tools in a sandbox and
              gating CI on the trajectory verdict.
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
              title="Langfuse — trace ingest"
              language="python"
              code={`from langfuse import Langfuse

lf = Langfuse()

trace = lf.trace(name="answer-question")
gen = trace.generation(
    name="rag",
    model="gpt-4o",
    input={"q": question, "docs": docs},
)
reply = llm.complete(question, docs)
gen.end(output=reply, usage={"input": 412, "output": 87})`}
            />
          }
        />

        <SplitSection
          eyebrow="Where Langfuse shines"
          title={
            <>
              Live traces.
              <br />
              <span className="text-white/40">Cost attribution. Trends.</span>
            </>
          }
          body={
            <>
              <p>
                Langfuse does the hardest part of LLM observability well:
                ingest every call from your live app, link them into
                session traces, attribute spend, and flag latency tails.
                For teams whose top problem is &quot;what is my production
                LLM actually doing,&quot; it&apos;s one of the cleanest
                open-source answers on the market.
              </p>
              <p className="mt-4">
                AgentClash doesn&apos;t try to be a trace ingestion
                pipeline. If that&apos;s your primary job, keep Langfuse —
                it&apos;s the right tool.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Langfuse — OTel ingest"
              language="shell"
              code={`# OpenTelemetry sidecar sends
# every LLM call to Langfuse
export LANGFUSE_PUBLIC_KEY="pk-lf-..."
export LANGFUSE_SECRET_KEY="sk-lf-..."
export OTEL_EXPORTER_OTLP_ENDPOINT=\\
  https://cloud.langfuse.com/api/public/otel

# Your app runs unchanged; traces stream
# into the Langfuse dashboard`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Where AgentClash wins"
          title={
            <>
              Score the agent.
              <br />
              <span className="text-white/40">Before it reaches prod.</span>
            </>
          }
          body={
            <>
              <p>
                Observability is a post-mortem tool — useful, necessary,
                and one step too late. The job AgentClash is built for is
                the one before prod: racing three or five agents against
                the same real task, in a sandbox, and deciding which one
                earns the merge.
              </p>
              <p className="mt-4">
                Flunked runs archive into a regression suite automatically.
                The next model change runs against a higher bar than the
                last one shipped against — that&apos;s the loop observability
                can&apos;t close on its own.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="AgentClash — verdict + regression"
              code={`$ agentclash run create \\
    --pack support-triage \\
    --agents gpt-5,claude-4.5,gemini-2.5 \\
    --follow

  gpt-5       ● on-plan   9.4  $0.012  8.1s
  claude-4.5  ● on-plan   9.1  $0.010  9.4s
  gemini-2.5  ✗ failed    3.2  $0.018  32.0s

$ agentclash regression promote run_01HX...
  added to suite: support-triage-hard`}
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
                Where AgentClash pulls ahead
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six things trace ingestion can&apos;t do for you.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock
          title="Langfuse vs AgentClash, answered."
          items={FAQ_ITEMS}
          schemaId="ld-vs-langfuse-faq"
        />

        <ClosingCTA
          title={
            <>
              Keep your observability.
              <br />
              <span className="text-white/40">Add a CI verdict.</span>
            </>
          }
          body={
            <p>
              Langfuse tells you what your production agent did. AgentClash
              tells you which candidate agent deserves to replace it. We&apos;ll
              set up the race on your hardest task in 20 minutes.
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
