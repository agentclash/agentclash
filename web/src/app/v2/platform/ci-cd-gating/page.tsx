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

const PATH = "/v2/platform/ci-cd-gating";

export const metadata: Metadata = {
  title: "Agent evaluation in CI/CD",
  description:
    "Block merges when agents regress. AgentClash runs full races from GitHub Actions, posts verdicts as checks, and fails the build on correctness, cost, or latency regressions.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AI agent evaluation in CI/CD — AgentClash",
    description:
      "Fire full agent races from CI. Block merges on regressions. Post verdicts as GitHub checks.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "GitHub checks",
    title: "Verdicts inline in the PR.",
    body: "Passing races turn the check green. Failing ones link directly to the failing replay, the diff on the trajectory, and the specific assertion that broke.",
  },
  {
    label: "Budget gates",
    title: "Cost and latency regress too.",
    body: "Block a PR that makes the agent twice as expensive or twice as slow — even when the correctness rubric would still pass.",
  },
  {
    label: "Matrix",
    title: "Race three models in parallel.",
    body: "Concurrent races across GPT, Claude, Gemini — matrix the verdict grid so you can see which model the change actually improved.",
  },
  {
    label: "Artifacts",
    title: "Every race is an artifact.",
    body: "Replays, trajectories, and verdicts are addressable by run ID. Attach them to the PR. Link them from your release notes.",
  },
  {
    label: "Zero glue",
    title: "One CLI, any runner.",
    body: "GitHub Actions, GitLab CI, CircleCI, Buildkite, Jenkins — same `agentclash run create` command. No provider-specific plugins.",
  },
  {
    label: "Cheap by default",
    title: "Short budgets for the PR loop.",
    body: "Tight default time budgets in CI mean races finish in under two minutes. Nightly suites can still run the heavy ones.",
  },
];

const FAQ_ITEMS = [
  {
    question: "How does AgentClash run in CI/CD?",
    answer:
      "Install the CLI (npm i -g agentclash), set AGENTCLASH_TOKEN and AGENTCLASH_WORKSPACE as secrets, and run `agentclash run create` as a job step. Verdicts post as GitHub checks; the CLI exits non-zero when the run fails its assertions.",
  },
  {
    question: "What does a verdict check look like?",
    answer:
      "A GitHub check named after the challenge pack with the overall verdict, a link to the replay, and per-vantage details (correctness, cost, latency, behaviour). Click-through goes to the trajectory viewer with the failing assertion highlighted.",
  },
  {
    question: "Can I block merges on cost and latency, not just correctness?",
    answer:
      "Yes. Challenge pack verdicts can include budget assertions: cost ceiling, latency p95, trajectory length. If the agent gets more expensive or slower, the check fails even if the answer is still right.",
  },
  {
    question: "Which CI providers are supported?",
    answer:
      "Any CI that can run a shell command. GitHub Actions is the best-supported because the CLI posts checks and comments via the GitHub token, but the same command runs fine on GitLab, CircleCI, Buildkite, Jenkins, or a cron box.",
  },
  {
    question: "Can I race multiple models per PR?",
    answer:
      "Yes — that's the default. Challenge packs name the lineup, and the races run concurrently in the background. The check fails if *any* required agent regresses.",
  },
];

export default function CICDGatingPage() {
  return (
    <>
      <JsonLd
        id="ld-cicd-product"
        data={productSchema({
          name: "AgentClash — CI/CD agent eval gating",
          description:
            "Fire AI agent races from CI. Block merges on correctness, cost, or latency regressions. Post verdicts as GitHub checks.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-cicd-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Platform", url: "/v2/platform/agent-evaluation" },
          { name: "CI/CD gating", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Platform" },
            { label: "CI/CD gating" },
          ]}
          eyebrow="Agent eval in CI/CD"
          title={
            <>
              Fail the build,
              <br />
              <span className="text-white/40">not the user.</span>
            </>
          }
          subtitle={
            <>
              Drop the AgentClash CLI into any CI. Every PR fires a real
              race in a real sandbox and posts a verdict back as a GitHub
              check. Merges block when the agent regresses on
              correctness, cost, or latency.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton />
              <Link
                href="/docs/guides/interpret-results"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Interpret the verdict
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title=".github/workflows/agent.yml"
              code={`- name: Race agents
  env:
    AGENTCLASH_TOKEN: \${{ secrets.AGENTCLASH_TOKEN }}
    AGENTCLASH_WORKSPACE: \${{ secrets.AGENTCLASH_WORKSPACE }}
  run: |
    npm i -g agentclash
    agentclash run create \\
      --pack coding-agents \\
      --agents gpt-5,claude-4.5 \\
      --follow`}
            />
          }
        />

        <SplitSection
          eyebrow="Verdict → check"
          title={
            <>
              Regressions show up
              <br />
              <span className="text-white/40">in the review.</span>
            </>
          }
          body={
            <>
              <p>
                The CLI posts back to GitHub the same way your test
                runner does — a check named after the challenge pack,
                passing or failing, with a link to the failing
                trajectory if there is one.
              </p>
              <p className="mt-4">
                Reviewers see agent regressions in the same pane as unit
                test failures. No one has to open a second tool, skim a
                dashboard, or remember to look.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Check output"
              code={`✓ coding-agents / correctness  9.1 → 9.3
✓ coding-agents / cost         $0.018 → $0.017
✗ coding-agents / latency      p95 14s → 22s
✗ coding-agents / behaviour    looped on tool error

  → replay: agentclash.dev/r/01H...`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                What CI gets
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six things the pipeline actually needs.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-cicd-faq" />

        <ClosingCTA
          title={
            <>
              Land the PR, or block it.
              <br />
              <span className="text-white/40">Not the in-between.</span>
            </>
          }
          body={
            <p>
              Most teams ship the CLI into CI on day one. Book a call
              and we&apos;ll wire it up live against your repo.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors" />
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
