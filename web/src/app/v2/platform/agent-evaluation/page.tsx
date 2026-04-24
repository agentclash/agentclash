import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { PageHeader } from "@/components/marketing/page-header";
import { SplitSection } from "@/components/marketing/split-section";
import { FeatureGrid } from "@/components/marketing/feature-grid";
import { ClosingCTA } from "@/components/marketing/closing-cta";
import { DemoButton } from "@/components/marketing/demo-button";
import { CodeCard } from "@/components/marketing/code-card";
import { FAQBlock } from "@/components/marketing/faq-block";
import { ProviderStrip } from "@/components/marketing/provider-strip";
import {
  JsonLd,
  breadcrumbSchema,
  productSchema,
} from "@/components/marketing/json-ld";

const PATH = "/v2/platform/agent-evaluation";

export const metadata: Metadata = {
  title: "AI agent evaluation platform",
  description:
    "AgentClash is the open-source AI agent evaluation platform. Race agents head-to-head on real tasks with the same tools, same time budget, live scoring, and CI regression gates.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AI agent evaluation platform — AgentClash",
    description:
      "Race AI agents head-to-head on real tasks. Same tools, same constraints, live scoring, CI regression gates.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Live",
    title: "Watch every race in real time.",
    body: "WebSocket event stream. Every think, tool call, observation, and scoring update appears the instant it happens — no batch post-processing.",
  },
  {
    label: "Replay",
    title: "Scrub back to where it broke.",
    body: "Full trace replay of prompts, outputs, and sandbox state. Step back to the moment a model went sideways and see exactly what it saw.",
  },
  {
    label: "Multi-vantage",
    title: "Correctness, cost, latency, behaviour.",
    body: "Verdicts score four vantage points on the same race so you can ship the model that wins on what your product actually optimizes for.",
  },
  {
    label: "Tool-use",
    title: "Real tools in a real sandbox.",
    body: "Isolated ephemeral E2B environments with pinned tools, network policy, secrets — not mocked function stubs that let everyone pretend to pass.",
  },
  {
    label: "Providers",
    title: "Every major provider, normalized.",
    body: "OpenAI, Anthropic, Gemini, xAI, Mistral, OpenRouter. Tool-call shapes and failure codes normalized so scoring is apples-to-apples.",
  },
  {
    label: "CI",
    title: "Block the merge, not the human.",
    body: "Fire races from CI with one CLI command. Verdicts post as GitHub checks so regressions show up in the review, not in prod.",
  },
];

const FAQ_ITEMS = [
  {
    question: "What is AI agent evaluation?",
    answer:
      "AI agent evaluation is the practice of running agents against realistic multi-step tasks — tool calls, sandbox actions, long-horizon reasoning — and scoring them on correctness, cost, latency, and behavior. It's distinct from prompt evaluation, which scores single responses from a single model.",
  },
  {
    question: "How is AgentClash different from LangSmith or Braintrust?",
    answer:
      "LangSmith, Braintrust, Promptfoo, and Langfuse are excellent at prompt evaluation — scoring single-call outputs against references. AgentClash is built for the next problem over: agents that take actions, use tools, run for minutes at a time, and can regress in ways a single-turn eval can't catch.",
  },
  {
    question: "What tasks should I race agents on?",
    answer:
      "Whatever your product actually does. Code review, SQL generation, customer support tickets, SRE runbooks, deep research. Challenge packs are small declarative bundles that describe the task, the tools, the scoring rubric, and the verdict pipeline.",
  },
  {
    question: "Do I need to write my own scorer?",
    answer:
      "No. AgentClash ships with reference scorers for correctness, cost, latency, and behavior. You can drop in custom scorers as tool-calling agents themselves — useful when \"correct\" needs domain-specific judgment.",
  },
  {
    question: "Is this open source?",
    answer:
      "Yes, under FSL-1.1-MIT. You can read the full scoring pipeline, fork it, and self-host the entire race engine. The managed cloud is an option, not a requirement.",
  },
];

export default function AgentEvaluationPage() {
  return (
    <>
      <JsonLd
        id="ld-ae-product"
        data={productSchema({
          name: "AgentClash — AI agent evaluation platform",
          description:
            "Open-source AI agent evaluation platform. Race agents head-to-head on real tasks with the same tools, same constraints, live scoring, and CI regression gates.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-ae-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Platform", url: "/v2/platform/agent-evaluation" },
          { name: "Agent evaluation", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Platform" },
            { label: "Agent evaluation" },
          ]}
          eyebrow="AI agent evaluation platform"
          title={
            <>
              Agent eval that scores
              <br />
              <span className="text-white/40">the whole trajectory.</span>
            </>
          }
          subtitle={
            <>
              AgentClash is the open-source AI agent evaluation platform.
              Race models head-to-head on real multi-turn tasks — same
              tools, same sandbox, same time budget — and gate CI on
              verdicts that actually reflect production.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton />
              <Link
                href="/v2/oss"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Self-host it
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="A race in one file"
              code={`# challenges/rag-qa.yaml
name: rag-qa-short
version: 1
agents: [gpt-5, claude-4.5, gemini-2.5]
tools: [search, fetch, quote]
time_budget: 120s
verdict:
  - correctness: rubric
  - cost: usd
  - latency: p95
  - behaviour: trajectory`}
            />
          }
        />

        <SplitSection
          eyebrow="Agent eval vs. prompt eval"
          title={
            <>
              Prompt eval scores text.
              <br />
              <span className="text-white/40">Agent eval scores outcomes.</span>
            </>
          }
          body={
            <>
              <p>
                A single-turn prompt eval can tell you that a model said
                the right thing once. It can&apos;t tell you whether an
                agent followed a plan across eight tool calls, stayed
                within a budget, handled a flaky search API, and
                terminated correctly when the task was done.
              </p>
              <p className="mt-4">
                AgentClash races whole trajectories. Each agent runs a
                full multi-step task in an isolated sandbox against the
                same tools — and we score what actually happened, not
                what the model claimed it would do.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Trajectory, not transcript"
              code={`think → tool:search("q") → observe
      ↳ tool:fetch(doc-42)     → observe
      ↳ tool:quote(p.7)        → observe
      → answer

verdict:
  correctness  9.2 / 10
  cost         $0.018
  latency      12.4 s
  behaviour    on-plan, no loops`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-40">
          <div className="mx-auto max-w-[1440px]">
            <div className="grid gap-10 md:grid-cols-2 md:items-end md:gap-16">
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Any model. Any provider.
              </h2>
              <p className="max-w-[42ch] text-[15px] leading-[1.65] text-white/55">
                First-class adapters for every major provider plus
                OpenRouter for the long tail — swap models without
                touching your challenge packs.
              </p>
            </div>
            <div className="mt-16">
              <ProviderStrip />
            </div>
          </div>
        </section>

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                Capabilities
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six things an agent eval platform should quietly do well.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock
          title="Agent evaluation, answered."
          items={FAQ_ITEMS}
          schemaId="ld-ae-faq"
        />

        <ClosingCTA
          title={
            <>
              Score the trajectory.
              <br />
              <span className="text-white/40">Ship the right agent.</span>
            </>
          }
          body={
            <p>
              Book a 20-minute call and we&apos;ll race your hardest task
              against the six models you were going to compare yourself.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            />
            <Link
              href="/v2/oss"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Self-host for free
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
