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

const PATH = "/v2/platform/rag-evaluation";

export const metadata: Metadata = {
  title: "RAG agent evaluation",
  description:
    "Evaluate retrieval-augmented agents end-to-end: retrieval quality, grounding, answer correctness, and cost — all on the same race. Catch silent retrieval regressions before they ship.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "RAG agent evaluation — AgentClash",
    description:
      "End-to-end RAG evaluation: retrieval, grounding, correctness, cost. On the same trajectory.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Retrieval",
    title: "Score what got fetched.",
    body: "Recall and precision against a labeled corpus, plus trajectory-aware metrics: did the right doc arrive in the first three calls, or the eighth?",
  },
  {
    label: "Grounding",
    title: "Answers tied to sources.",
    body: "Each claim in the final answer is traced to the specific retrieved chunk it came from. Unsourced claims flag the verdict regardless of correctness.",
  },
  {
    label: "Correctness",
    title: "Rubric-scored final answers.",
    body: "Reference-free rubric scorers and reference-based scorers, both first-class. Mix them per challenge — ground-truth where you have it, rubric where you don't.",
  },
  {
    label: "Budget",
    title: "Cheap retrieval isn't free.",
    body: "Token, embedding, and vector-DB call budgets are all first-class verdict vantages. A more-correct but 3×-more-expensive pipeline still regresses.",
  },
  {
    label: "Reranking",
    title: "Test with and without.",
    body: "Matrix runs over reranker variants in the same race. Head-to-head verdicts tell you whether your fancy reranker was worth the latency.",
  },
  {
    label: "Drift",
    title: "Corpus changes. Agents notice.",
    body: "Schedule nightly races against your index. When a retrieval-dependent challenge suddenly starts failing, you get a drift alert — before customers do.",
  },
];

const FAQ_ITEMS = [
  {
    question: "What does 'RAG evaluation' cover?",
    answer:
      "Retrieval-augmented agent evaluation scores the whole retrieval+generation pipeline end-to-end: retrieval quality, grounding, answer correctness, cost, and drift. AgentClash scores these on the same trajectory, so you can see *which* part broke when a verdict dips.",
  },
  {
    question: "Do I need to bring a ground-truth corpus?",
    answer:
      "Not necessarily. Reference-free rubric scorers work when you only have ground-truth answers; reference-based scorers work when you have labeled retrievals. Most teams have some of each, and AgentClash mixes them per challenge pack.",
  },
  {
    question: "How does grounding scoring work?",
    answer:
      "The scorer inspects the final answer for claims, walks back through the trajectory to find the retrieved chunk that supports each claim, and flags any claim with no supporting retrieval. Grounding score is the ratio of supported claims.",
  },
  {
    question: "Can I test reranker variants head-to-head?",
    answer:
      "Yes. Challenge packs can name a matrix of retrieval configurations, each racing concurrently. The verdict table shows you whether the reranker earned its latency.",
  },
  {
    question: "What happens when the corpus changes?",
    answer:
      "Schedule a nightly regression race; AgentClash compares today's verdicts against the baseline and surfaces retrieval drift as a failing check. You find out before customers do.",
  },
];

export default function RagEvaluationPage() {
  return (
    <>
      <JsonLd
        id="ld-rag-product"
        data={productSchema({
          name: "AgentClash — RAG agent evaluation",
          description:
            "Evaluate retrieval-augmented agents end-to-end: retrieval quality, grounding, answer correctness, and cost.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-rag-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Platform", url: "/v2/platform/agent-evaluation" },
          { name: "RAG evaluation", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Platform" },
            { label: "RAG evaluation" },
          ]}
          eyebrow="RAG agent evaluation"
          title={
            <>
              Retrieval, grounding,
              <br />
              <span className="text-white/40">the whole pipeline.</span>
            </>
          }
          subtitle={
            <>
              Retrieval-augmented agents break in more places than
              single-call models. AgentClash scores retrieval quality,
              grounding, answer correctness, and cost on the same
              trajectory so you know which part regressed when the
              verdict drops.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton />
              <Link
                href="/v2/use-cases/deep-research"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Deep research use case
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="RAG verdict"
              code={`retrieval:    recall    0.92  precision 0.71
              first_doc@1          0.88
grounding:    supported            0.94
correctness:  rubric               9.1 / 10
cost:         embed + gen         $0.014
latency:      p95                  3.8 s`}
            />
          }
        />

        <SplitSection
          eyebrow="Where RAG actually breaks"
          title={
            <>
              Wrong chunk.
              <br />
              <span className="text-white/40">Confident answer.</span>
            </>
          }
          body={
            <>
              <p>
                The worst RAG bugs aren&apos;t hallucinations — they&apos;re
                confident answers grounded in the wrong chunk. A
                single-turn correctness eval can give both cases the
                same score.
              </p>
              <p className="mt-4">
                AgentClash scores retrieval and grounding separately, on
                the same run. When the overall verdict drops you can see
                whether the retriever, the reranker, or the generator
                was the weak link.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Where it went wrong"
              code={`answer:        "Yes — policy allows it."
grounding:     cited chunk #4
chunk #4:      "Exceptions allowed only..."
                  ↑
grounding:     unsupported (cherry-picked)
verdict:       ✗ hallucinated grounding`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                RAG vantages
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six things worth scoring separately.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-rag-faq" />

        <ClosingCTA
          title={
            <>
              Know which part broke.
              <br />
              <span className="text-white/40">Before customers do.</span>
            </>
          }
          body={
            <p>
              Send us your hardest RAG failure and we&apos;ll race three
              pipelines against it — retriever, reranker, and generator
              swapped independently.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors" />
            <Link
              href="/v2/use-cases/deep-research"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Deep-research workflow
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
