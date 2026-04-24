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

const PATH = "/v2/methodology";

export const metadata: Metadata = {
  title: "Evaluation methodology — AgentClash",
  description:
    "Four vantages per verdict: correctness, cost, latency, behavior. Explicit rubrics, explicit sandbox guarantees, explicit non-determinism handling. No hidden magic in the scoring pipeline.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AgentClash evaluation methodology",
    description:
      "Four-vantage verdicts, trajectory-signature scoring, and explicit non-determinism handling.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Correctness",
    title: "Rubric or ground-truth.",
    body: "Every challenge pack declares how correctness is judged: a YAML rubric scored by a judge model, a hand-written ground-truth checker, or a mix. The choice is in the repo — you can read it, fork it, and argue with it.",
  },
  {
    label: "Cost",
    title: "Token-accurate, provider-aware.",
    body: "Cost is measured from the provider's billed token counts, not estimated from heuristics. Per-run figures include input, output, cached, and tool-use tokens, broken down by model so cost regressions are attributable.",
  },
  {
    label: "Latency",
    title: "p50 and p95 wall-clock in sandbox.",
    body: "Wall-clock from the first tool call to the terminal event, measured inside the sandbox so network variance between you and our control plane doesn't pollute the number. Distributions, not averages.",
  },
  {
    label: "Behavior",
    title: "Plan adherence and termination.",
    body: "The trajectory scorer looks at plan adherence, tool-loop detection, self-correction patterns, and termination reason. A passing correctness score on a looping agent still fails behavior — and that shows up in the verdict.",
  },
  {
    label: "Non-determinism",
    title: "N-run cohorts, distributional.",
    body: "Single runs are noise. Every race in a cohort is rerun N times with fresh seeds and sandbox state. The verdict is a distribution — you see pass rate, variance, and outliers rather than a single coin flip.",
  },
  {
    label: "Transparency",
    title: "Scorers are readable Go.",
    body: "Every scorer lives as plain Go in the open-source repo. No binary blobs, no proprietary model weights gating your scores. If you disagree with how something is measured, you can fork the scorer and pin your own.",
  },
];

const FAQ_ITEMS = [
  {
    question: "Who authors the rubrics?",
    answer:
      "You do. A challenge pack ships with a YAML rubric (or a ground-truth checker, or both), and the rubric is part of the pack repo — reviewable, versioned, and diffable. When we help design partners author their first pack, we draft it together on a call and hand you the editable file.",
  },
  {
    question: "Are scorers deterministic?",
    answer:
      "Heuristic scorers (cost, latency, trajectory signature, termination reason) are fully deterministic — identical inputs yield identical scores. Judge-model scorers are non-deterministic by construction, which is why cohorts run N times and the verdict is reported as a distribution with variance, not a single number.",
  },
  {
    question: "Agent-as-scorer or hand-written checks?",
    answer:
      "Both, and you pick per challenge pack. Judge-model scoring is the right tool when the answer space is open-ended (summaries, plans, code explanations). Hand-written checks are the right tool when the answer is testable (unit tests passing, a file created with specific contents). You can combine them — rubric score gated by a ground-truth precondition is a common pattern.",
  },
  {
    question: "How is cost measured?",
    answer:
      "We read billed token counts from each provider response (input, output, cached tokens, and tool-use tokens where the provider breaks them out) and price them at the model's published rate. The resulting cost is attributable per step, per tool call, and per model — not a single aggregate at the end of the run.",
  },
  {
    question: "Why trajectory signatures instead of output matching?",
    answer:
      "Model wording drifts between versions in ways that don't matter for behavior — a new model says the same thing differently. Signatures (the sequence of tools used, the plan shape, the termination reason) stay stable across those shifts, so your assertions still mean what you wrote them to mean six months later.",
  },
];

export default function MethodologyPage() {
  return (
    <>
      <JsonLd
        id="ld-methodology-product"
        data={productSchema({
          name: "AgentClash — evaluation methodology",
          description:
            "Four-vantage verdicts (correctness, cost, latency, behavior) with explicit rubric authorship, sandbox guarantees, and non-determinism handling.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-methodology-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Methodology", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Methodology" },
          ]}
          eyebrow="Evaluation methodology"
          title={
            <>
              How we score.
              <br />
              <span className="text-white/40">On purpose.</span>
            </>
          }
          subtitle={
            <>
              Four vantages — correctness, cost, latency, behavior — on
              every race. Explicit rubric authorship, explicit sandbox
              guarantees, explicit non-determinism handling. No hidden
              magic in the scoring pipeline; every scorer is readable Go
              you can fork.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton label="Walk the rubric with us" />
              <Link
                href="/v2/platform/regression-testing"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Regression testing
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="Verdict shape"
              code={`verdict:
  correctness:  0.82  (rubric: rag-qa.yaml)
  cost:         $0.014 / run
  latency:      p50 4.1s · p95 11.3s
  behavior:     plan-adherent · terminated:success
cohort:
  runs:         25
  pass_rate:    0.76
  variance:     σ = 0.09`}
            />
          }
        />

        <SplitSection
          eyebrow="The four-vantage verdict"
          title={
            <>
              One number is a lie.
              <br />
              <span className="text-white/40">Four numbers are a verdict.</span>
            </>
          }
          body={
            <>
              <p>
                Collapsing agent performance into a single leaderboard
                number throws away the signal you actually need to ship.
                An agent that scores 90% on correctness at ten times the
                cost of the baseline is a regression, not a win.
              </p>
              <p className="mt-4">
                Every AgentClash verdict carries correctness, cost,
                latency, and behavior side by side. You can gate on any
                one of them, or compose a pass criterion across all four.
                The raw numbers stay attached to the run so you can argue
                about them later.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="A rendered verdict"
              code={`$ agentclash run show run_01H...

  agent:         claude-4.6-sonnet
  pack:          rag-qa-hard
  correctness:   0.82  ✓ (threshold 0.75)
  cost:          $0.014 ✓ (budget $0.05)
  latency p95:   11.3s  ✓ (budget 15s)
  behavior:      ✓ plan-adherent
                 ✓ no-loops
                 ✓ terminated:success

  verdict:       ✓ pass`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Why rubrics beat string-match"
          title={
            <>
              Assertions that survive
              <br />
              <span className="text-white/40">a model bump.</span>
            </>
          }
          body={
            <>
              <p>
                A rubric spells out what makes an answer correct — the
                facts it must contain, the reasoning steps it must take,
                the errors it must avoid — in YAML your team writes and
                reviews. That&apos;s a durable specification.
              </p>
              <p className="mt-4">
                String-matching a reference answer breaks the first time a
                new model says the same thing differently. Rubric scoring,
                paired with trajectory signatures for behavior, holds up
                across model swaps — which is the whole point of having a
                regression suite.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="rag-qa.yaml"
              code={`# packs/rag-qa-hard/rubric.yaml
rubric:
  must_include:
    - cites source document
    - names the customer tier
  must_avoid:
    - hallucinated policy numbers
    - refusal on in-scope question
  scoring:
    judge: gpt-5-mini
    scale: 0-1
    rationale_required: true
behavior:
  expected_tools: [search, fetch]
  terminated: success
  max_tool_calls: 8`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                Scoring pipeline
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six parts you can read, fork, and argue with.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-methodology-faq" />

        <ClosingCTA
          title={
            <>
              Bring us a case
              <br />
              <span className="text-white/40">you can&apos;t score.</span>
            </>
          }
          body={
            <p>
              We&apos;ll walk your actual task through the scoring
              pipeline on a 30-minute call — rubric draft, trajectory
              signature, cohort plan. If it doesn&apos;t fit, we&apos;ll
              say so.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            />
            <Link
              href="/v2/platform/regression-testing"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Regression testing
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
