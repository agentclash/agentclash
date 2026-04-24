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
import {
  JsonLd,
  breadcrumbSchema,
  productSchema,
} from "@/components/marketing/json-ld";

const PATH = "/v2/platform/multi-turn-evaluation";

export const metadata: Metadata = {
  title: "Multi-turn agent evaluation",
  description:
    "Score whole conversations and multi-step plans, not one-shot replies. AgentClash evaluates tool use, plan adherence, self-correction, and termination behavior across the full trajectory.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "Multi-turn agent evaluation — AgentClash",
    description:
      "Evaluate conversations and multi-step plans. Score tool use, plan adherence, self-correction, termination.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Plan adherence",
    title: "Did it follow the plan it made?",
    body: "Agents that state a plan and quietly drift from it are hard to catch. Plan-adherence scoring flags when the trajectory stops matching the stated intent.",
  },
  {
    label: "Self-correction",
    title: "Recover gracefully or flail?",
    body: "When a tool returns an error, the good agents replan. The bad ones loop or bail. Self-correction is a first-class vantage in the verdict.",
  },
  {
    label: "Termination",
    title: "Stop when done. Stop when stuck.",
    body: "An agent that runs to the time budget when the task finished in 3 steps costs you real money. Termination behavior gets scored on both sides.",
  },
  {
    label: "Tool budget",
    title: "Cheap for the task, not cheap overall.",
    body: "Budget assertions are per-challenge-pack. Research tasks can afford 40 tool calls; a support reply shouldn't cross 6.",
  },
  {
    label: "Memory",
    title: "Long context, honestly tested.",
    body: "Challenges can carry state across turns — ticket threads, codebase context, prior conversation. Score whether the agent actually used it.",
  },
  {
    label: "Rubrics",
    title: "Rubrics written by humans, scored by agents.",
    body: "Scoring agents are themselves tool-calling agents, with their own prompts and rubrics — you can inspect, fork, and fine-tune them per challenge pack.",
  },
];

const FAQ_ITEMS = [
  {
    question: "What is multi-turn agent evaluation?",
    answer:
      "Multi-turn evaluation scores an agent across an entire trajectory: multiple model calls, tool uses, observations, replans, and a termination condition. It's the right shape for real products, because real agents don't solve things in one shot.",
  },
  {
    question: "How is this different from a chat benchmark?",
    answer:
      "A chat benchmark usually scores one-shot responses from a single model. Multi-turn agent evaluation scores plans, tool sequences, budget adherence, and termination behavior — the things that break in production but look fine in a single-turn transcript.",
  },
  {
    question: "Can I bring my own rubric?",
    answer:
      "Yes. Rubrics are markdown-driven challenge-pack fields that a scoring agent consults. You can fork the default scorer, swap the scoring model, or write a domain-specific rubric from scratch.",
  },
  {
    question: "How long can a race be?",
    answer:
      "Challenge packs set a time budget and a tool-call budget. Short races (under 60 seconds) work well in CI; longer research or SRE-style races can run for many minutes. Temporal keeps everything durable regardless of length.",
  },
  {
    question: "How do you handle non-determinism?",
    answer:
      "Run the same challenge N times and score distributions, not single runs. The default pipeline captures every seed and lets you compare median / p90 verdicts across a cohort.",
  },
];

export default function MultiTurnEvaluationPage() {
  return (
    <>
      <JsonLd
        id="ld-mt-product"
        data={productSchema({
          name: "AgentClash — multi-turn agent evaluation",
          description:
            "Evaluate whole agent trajectories — tool use, plan adherence, self-correction, and termination — not one-shot replies.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-mt-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Platform", url: "/v2/platform/agent-evaluation" },
          { name: "Multi-turn evaluation", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Platform" },
            { label: "Multi-turn evaluation" },
          ]}
          eyebrow="Multi-turn agent evaluation"
          title={
            <>
              The eight tool calls
              <br />
              <span className="text-white/40">between hello and done.</span>
            </>
          }
          subtitle={
            <>
              Production agents fail in the middle, not at the start or
              the end. AgentClash scores whole trajectories — plan
              adherence, self-correction, tool budget, termination — so
              the right model wins for the reason it won.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton />
              <Link
                href="/v2/platform/agent-evaluation"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Platform overview
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="A trajectory"
              code={`turn 1  plan:   ["search", "fetch", "verify"]
turn 2  tool:   search("arxiv 2506...")
turn 3  obs:    3 results
turn 4  tool:   fetch(doc-1)
turn 5  tool:   fetch(doc-2)     ✗ 500
turn 6  think:  "retry with fallback"
turn 7  tool:   fetch(doc-1.alt) ✓
turn 8  answer  verified: true   ✓`}
            />
          }
        />

        <SplitSection
          eyebrow="Plan vs reality"
          title={
            <>
              Plans are easy.
              <br />
              <span className="text-white/40">Following them is the job.</span>
            </>
          }
          body={
            <>
              <p>
                Agents routinely state a plan and quietly deviate. A
                single-turn eval never catches it; the first reply
                already sounds fine. Plan-adherence scoring compares the
                stated plan with the executed trajectory and flags
                silent drift.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Plan adherence"
              code={`stated plan:    [search, fetch, verify]
executed:       [search, fetch, answer]
                                 ↑
adherence:      0.66 (skipped verify)
verdict note:   unverified claim shipped
                in final answer`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                Capabilities
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six vantages for whole-trajectory scoring.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-mt-faq" />

        <ClosingCTA
          title={
            <>
              One reply is easy.
              <br />
              <span className="text-white/40">An hour of them is the job.</span>
            </>
          }
          body={
            <p>
              Bring a real multi-step task and we&apos;ll race three
              models against it end-to-end.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors" />
            <Link
              href="/v2/use-cases/coding-agents"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              See a coding-agent race
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
