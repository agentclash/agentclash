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

const PATH = "/v2/use-cases/deep-research";

export const metadata: Metadata = {
  title: "Evaluate deep research agents",
  description:
    "Evaluate deep research agents on multi-source retrieval, grounding, and verification. Score sub-query planning, citations, and cross-checks with AgentClash.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "Evaluate deep research agents — AgentClash",
    description:
      "Grade long-horizon research agents on source quality, citation fidelity, and cross-check behavior — not just output polish.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Sub-query planning",
    title: "Score the research tree.",
    body: "A good researcher decomposes the question before it starts searching. We grade the sub-query plan — breadth, ordering, and whether it revisits gaps — alongside the final answer.",
  },
  {
    label: "Citations",
    title: "Quote or it didn't happen.",
    body: "Every claim needs a URL and a passage. We check that cited passages exist in the fetched page and actually support the claim — not just that a link was dropped next to it.",
  },
  {
    label: "Cross-check",
    title: "Two independent sources.",
    body: "For load-bearing facts, grade whether the agent cross-verified against a second source — not two mirrors of the same press release.",
  },
  {
    label: "Source quality",
    title: "Primary over aggregator.",
    body: "Agents that cite SEC filings, arXiv, or official docs score above agents that cite content farms. Domain-level scoring is part of the verdict.",
  },
  {
    label: "Freshness",
    title: "Date-aware grounding.",
    body: "Old sources quietly poison answers about moving targets. We flag claims that cite pages older than the declared freshness window.",
  },
  {
    label: "Budget",
    title: "Depth inside a time box.",
    body: "Deep research burns tokens. Budget budgets for search calls, fetch size, and wall-clock so you can find agents that go deep without going broke.",
  },
];

const FAQ_ITEMS = [
  {
    question: "What is a deep research agent evaluation?",
    answer:
      "It grades an agent that plans sub-queries, browses, reads, and writes a cited report against a research question — on the quality of its process, not just the polish of its final paragraph. AgentClash scores plan shape, source quality, citation fidelity, and cross-checking behavior.",
  },
  {
    question: "How do you grade citations?",
    answer:
      "For every claim the agent makes, we verify that the cited URL was actually fetched during the run, that the quoted passage exists on the fetched page, and that the passage semantically supports the claim. A link without a supporting passage is graded as an unsupported claim.",
  },
  {
    question: "Can you catch hallucinated sources?",
    answer:
      "Yes. Citations that don't appear in the fetch history are hallucinations by definition — the agent couldn't have read them. Those cases are flagged and heavily penalized in the verdict.",
  },
  {
    question: "How do you score cross-checking?",
    answer:
      "Load-bearing facts (numbers, names, dates, claims about living people) are extracted from the report and checked for independent secondary sources in the fetch trajectory. Two citations to the same press release don't count as a cross-check.",
  },
  {
    question: "What tools do research agents get?",
    answer:
      "Search, fetch, quote, and citation. Search returns ranked results; fetch pulls the full page; quote extracts passages with offsets; citation binds a claim to a (url, passage) pair. The tool policy is configurable per pack.",
  },
];

export default function DeepResearchPage() {
  return (
    <>
      <JsonLd
        id="ld-dr-product"
        data={productSchema({
          name: "AgentClash — deep research agent evaluation",
          description:
            "Grade long-horizon research agents on source quality, citation fidelity, and cross-check behavior.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-dr-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Use cases", url: "/v2/use-cases/coding-agents" },
          { name: "Deep research", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Use cases" },
            { label: "Deep research" },
          ]}
          eyebrow="Deep research agents"
          title={
            <>
              Grade the research,
              <br />
              <span className="text-white/40">not the prose.</span>
            </>
          }
          subtitle={
            <>
              Deep research agents write beautiful reports. That&apos;s the
              problem. AgentClash strips the polish and scores what&apos;s
              underneath: the sub-query plan, the sources fetched, the
              passages actually quoted, and whether the agent bothered to
              cross-check the numbers.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton />
              <Link
                href="/v2/platform/rag-evaluation"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                RAG evaluation
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="A research case"
              code={`# packs/q1-earnings-rollup.yaml
question: |
  Summarize Q1-2026 cloud revenue for the
  three hyperscalers. Flag any segment
  reclassifications since Q4-2025.
tools: [search, fetch, quote, citation]
require:
  - cross_check_min: 2
  - primary_source_ratio: 0.6
  - citation_fidelity: 0.95
  - freshness_days_max: 60`}
            />
          }
        />

        <SplitSection
          eyebrow="Plan → fetch → quote"
          title={
            <>
              Good research has shape.
              <br />
              <span className="text-white/40">Most agents skip straight to prose.</span>
            </>
          }
          body={
            <>
              <p>
                The first failure mode we see: an agent answers the
                question from memory and back-fills a few links to make
                it look grounded. The trajectory gives it away — writes
                before reads, citations that weren&apos;t fetched, a
                suspiciously short plan.
              </p>
              <p className="mt-4">
                AgentClash scores the plan step explicitly. Did the agent
                decompose the question into sub-queries? Did it visit
                primary sources? Did it quote passages that actually
                support the claims? Agents that skip the method get the
                verdict they deserve.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Trajectory — grounded vs hallucinated"
              code={`# agent_a.trajectory
  plan: 6 sub-queries; primary > aggregator
  1 tool: search("AWS Q1-2026 revenue 10-Q")
  2 tool: fetch("https://ir.aws/.../10q")
  3 tool: quote(§"Cloud segment revenue...")
  4 tool: citation(claim_42, quote_1, source_2)
  ...
  verdict: ✓ 9.1 / 10   0 hallucinated citations

# agent_b.trajectory
  plan: 1 sub-query
  1 tool: search("hyperscaler earnings")
  2 output: <1200 words, 8 citations>
  verdict: ✗ 3.4 / 10   4 citations not in fetch log`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Cross-check"
          title={
            <>
              One source is a rumor.
              <br />
              <span className="text-white/40">Two is a fact.</span>
            </>
          }
          body={
            <>
              <p>
                Numbers, names, and claims about living people need to
                survive a second opinion. AgentClash extracts
                load-bearing claims from the report and checks whether
                each one was independently verified in the fetch
                trajectory. Two links to the same wire story do not
                count.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Claim extraction"
              code={`$ agentclash run show run_01H... --claims

  claim_17: "AWS Q1 op income $13.2B"
    sources: [sec.gov/10q, reuters.com/...]
    independent: ✓ 2 domains
    cross_check: passed

  claim_31: "Azure overtook AWS in API latency"
    sources: [medium.com/@blogpost]
    independent: ✗ 1 domain (opinion)
    cross_check: failed
    verdict_impact: -1.4`}
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
                Evaluate the research, not the executive summary.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-dr-faq" />

        <ClosingCTA
          title={
            <>
              Ship researchers
              <br />
              <span className="text-white/40">that cite their work.</span>
            </>
          }
          body={
            <p>
              Let us race three deep research agents against your
              hardest question and show you the fetch trajectories,
              claim-by-claim grounding, and cross-check verdicts.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            />
            <Link
              href="/v2/platform/agent-evaluation"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Evaluation primitives
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
