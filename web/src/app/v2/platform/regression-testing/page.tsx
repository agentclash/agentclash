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

const PATH = "/v2/platform/regression-testing";

export const metadata: Metadata = {
  title: "AI agent regression testing",
  description:
    "Archive every failure as a regression test. Replay them on every model change. AgentClash turns each production near-miss into a permanent bar the next agent has to clear.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AI agent regression testing — AgentClash",
    description:
      "Every failed trajectory becomes a permanent regression test. Catch model drift before it ships.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Auto-capture",
    title: "Flunks become tests.",
    body: "Any run whose verdict dips below your threshold freezes into a regression suite — with the exact prompt, tools, sandbox image, and expected trajectory signature.",
  },
  {
    label: "Replay",
    title: "Re-run on every model bump.",
    body: "Swap GPT-5 for Claude 4.6, Gemini 3, or an internal fine-tune — your regression suite replays all of them against the archived traces with no code change.",
  },
  {
    label: "Drift detection",
    title: "Catch provider-side regressions.",
    body: "Providers silently update models. Nightly regression races compare today's behavior against the archived baseline so drift surfaces the same day.",
  },
  {
    label: "Signature",
    title: "Behaviour beats string match.",
    body: "Trajectory signatures — tools used, plan shape, termination reason — give you stable assertions even when model wording shifts between versions.",
  },
  {
    label: "Budget aware",
    title: "Regress costs and latency too.",
    body: "A correct-but-5×-more-expensive agent is still a regression. Budget checks are first-class assertions alongside correctness.",
  },
  {
    label: "CI native",
    title: "Block the merge, not the hotfix.",
    body: "Run regressions in CI on every PR. Block when a regression appears — post the failing trace and verdict straight into the GitHub check.",
  },
];

const FAQ_ITEMS = [
  {
    question: "What is AI agent regression testing?",
    answer:
      "Agent regression testing replays a fixed set of known-hard tasks (usually archived from past failures) against a candidate agent, and fails the build if behavior regresses on correctness, cost, latency, or trajectory shape.",
  },
  {
    question: "How does AgentClash build a regression suite?",
    answer:
      "Any run whose verdict drops below your threshold is automatically promoted into the regression suite for that challenge pack. You can also hand-curate cases from the replay UI. Each case stores the full input, tool set, sandbox image, and expected trajectory signature.",
  },
  {
    question: "How do I keep regressions stable across model bumps?",
    answer:
      "We score against trajectory signatures (tools used, plan shape, termination conditions) instead of string-matching model output. This keeps assertions meaningful even when a new model says the same thing differently.",
  },
  {
    question: "Can I run regressions in CI?",
    answer:
      "Yes. The CLI has a `regression run` command designed for CI: it blocks on verdict changes, posts results as a GitHub check, and links directly to the failing replay.",
  },
  {
    question: "What about provider-side drift?",
    answer:
      "Providers ship silent model updates regularly. Schedule a nightly regression race and AgentClash will alert you when a baseline trajectory starts failing — that's the earliest signal you'll get that a provider shifted under you.",
  },
];

export default function RegressionTestingPage() {
  return (
    <>
      <JsonLd
        id="ld-rt-product"
        data={productSchema({
          name: "AgentClash — agent regression testing",
          description:
            "Archive every agent failure as a replayable regression test. Block merges on drift, cost blowouts, and trajectory regressions.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-rt-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Platform", url: "/v2/platform/agent-evaluation" },
          { name: "Regression testing", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Platform" },
            { label: "Regression testing" },
          ]}
          eyebrow="Agent regression testing"
          title={
            <>
              Every failure becomes
              <br />
              <span className="text-white/40">a permanent test.</span>
            </>
          }
          subtitle={
            <>
              Agents that passed yesterday regress quietly today — a new
              model ship, a provider update, a subtle sandbox change.
              AgentClash freezes every flunked trajectory into a
              regression suite that replays on every model change and
              blocks the merge when drift shows up.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton />
              <Link
                href="/v2/platform/ci-cd-gating"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Wire into CI
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="A regression case"
              code={`# regressions/pr-42-sandbox-lock.yaml
from_run: run_01H...
tools: [fs, exec, git]
expected_signature:
  - tool_order: [git:status, fs:read, exec:go_test]
  - terminated: success
  - cost_max: $0.02
  - p95_max: 14s`}
            />
          }
        />

        <SplitSection
          eyebrow="Why archive flunks"
          title={
            <>
              The hardest cases
              <br />
              <span className="text-white/40">rarely repeat themselves.</span>
            </>
          }
          body={
            <>
              <p>
                A good eval set is rare and expensive to build by hand.
                A failing agent run, on the other hand, is a naturally
                curated hard case — it exposed a real weakness in the
                shape of the model or the tooling.
              </p>
              <p className="mt-4">
                Every time one of those happens in AgentClash, we freeze
                it: the inputs, tools, sandbox image, and expected
                trajectory signature. Your regression suite grows on its
                own, weighted toward the cases that actually matter.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Flunk → regression"
              code={`$ agentclash run show run_01H...

  verdict:       ✗ failed
  correctness:   3.1 / 10
  behaviour:     looped_on_tool_error

$ agentclash regression promote run_01H...
  added to suite: rag-qa-hard
  12 cases now guarded in CI`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Drift detection"
          title={
            <>
              Providers ship quietly.
              <br />
              <span className="text-white/40">You find out loudly.</span>
            </>
          }
          body={
            <>
              <p>
                Frontier models change behavior between versions — sometimes
                between the same version. Nightly regression races replay
                your archived suite against live endpoints so silent drift
                surfaces as a red build, not a production incident.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Nightly drift sweep"
              code={`# .github/workflows/nightly-drift.yml
- name: Replay regression suite
  run: |
    agentclash regression run \\
      --suite production-hard \\
      --agents gpt-5,claude-4.5 \\
      --alert-on drift`}
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
                Regression testing that survives model bumps.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-rt-faq" />

        <ClosingCTA
          title={
            <>
              Ship upgrades.
              <br />
              <span className="text-white/40">Not regressions.</span>
            </>
          }
          body={
            <p>
              Let us replay your last painful agent bug against three new
              models so you can see where they break before you ship them.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            />
            <Link
              href="/v2/platform/ci-cd-gating"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Gate your CI
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
