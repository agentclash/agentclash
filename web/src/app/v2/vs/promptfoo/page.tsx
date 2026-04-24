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

const PATH = "/v2/vs/promptfoo";

export const metadata: Metadata = {
  title: "AgentClash vs Promptfoo",
  description:
    "Promptfoo is a great OSS CLI for git-committed prompt evals and red-teaming presets. AgentClash is built for the problem one step over: multi-turn agents with real tools, raced head-to-head, gating CI on trajectory verdicts.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AgentClash vs Promptfoo — agent eval vs prompt-config CLI",
    description:
      "Promptfoo evaluates YAML-described prompts in your repo. AgentClash races multi-turn agent trajectories with real tools. A direct comparison.",
    url: PATH,
  },
};

const COLUMNS: ComparisonColumn[] = [
  { name: "AgentClash", tag: "Agent eval", highlight: true },
  { name: "Promptfoo", tag: "OSS prompt-eval CLI" },
];

const ROWS: ComparisonRow[] = [
  {
    label: "Multi-turn agent loops",
    sub: "Scores plans, tool sequences, self-correction, and termination — not a single call.",
    cells: ["yes", "partial"],
  },
  {
    label: "Sandboxed tool execution",
    sub: "Ephemeral VM with real filesystem, subprocesses, and network-policy isolation.",
    cells: ["yes", "no"],
  },
  {
    label: "Durable orchestration for long runs",
    sub: "Temporal-backed workflows that survive provider timeouts, retries, and restarts.",
    cells: ["yes", "no"],
  },
  {
    label: "Head-to-head concurrent race",
    sub: "N models, same inputs, same budget, same scorer — a verdict, not a matrix.",
    cells: ["yes", "partial"],
  },
  {
    label: "Trajectory scoring",
    sub: "Plan adherence, tool-order signatures, self-correction, termination reason.",
    cells: ["yes", "partial"],
  },
  {
    label: "Flunk → regression suite",
    sub: "Failed runs promote into a permanent regression bar the next model has to clear.",
    cells: ["yes", "no"],
  },
  {
    label: "Git-committed YAML eval configs",
    sub: "Declarative prompt configs that live in your repo and diff cleanly in PRs.",
    cells: ["partial", "yes"],
  },
  {
    label: "Red-teaming + jailbreak presets",
    sub: "Canonical prompt-injection, PII, and jailbreak test suites out of the box.",
    cells: ["partial", "yes"],
  },
];

const FEATURES = [
  {
    label: "Sandbox",
    title: "Real tools executing in isolation.",
    body: "Each agent gets an ephemeral E2B environment with pinned tools, network policy, and secrets. Promptfoo-style providers fake the runtime; sandboxes make the runtime real.",
  },
  {
    label: "Durable",
    title: "Temporal under the hood.",
    body: "Agent runs survive provider 503s, flaky tools, and node restarts. Replays work months later because events are durably persisted, not inferred from stdout.",
  },
  {
    label: "Race",
    title: "Same-time, same-budget competition.",
    body: "Fire N providers at the same task concurrently. The verdict is a ranking, not a per-row pass/fail that you eyeball across a console dump.",
  },
  {
    label: "Trajectory",
    title: "Score the path, not the completion.",
    body: "Tool-order signatures, plan adherence, termination reason. A string-match assertion will not catch the model that looped three times to get there.",
  },
  {
    label: "CI gate",
    title: "Regressions block the merge.",
    body: "`agentclash regression run` posts verdicts as a GitHub check and links to the failing replay. Flunks freeze into the regression suite automatically.",
  },
  {
    label: "Multi-tenant",
    title: "Workspaces, SSO, role-based access.",
    body: "Built for teams beyond one engineer's laptop. WorkOS-backed auth, per-workspace secrets, shared runs, and durable replay archive.",
  },
];

const FAQ_ITEMS = [
  {
    question: "When should I use Promptfoo instead?",
    answer:
      "If you're a solo engineer or a small team that wants git-committed prompt evals running locally in a tight feedback loop, and your tests are mostly string assertions and red-team presets — Promptfoo is an excellent fit. It's free, fast, and gets out of your way. AgentClash is a heavier, team-shaped tool for the next problem: scoring full agent trajectories with real tools under durable orchestration.",
  },
  {
    question: "Can I migrate from Promptfoo?",
    answer:
      "Yes. Promptfoo YAML configs map cleanly to AgentClash challenge packs — both are declarative, git-friendly, and describe inputs + graders. You'll want to enrich them with tool specs, sandbox images, and trajectory rubrics to get the agent-eval benefits. Teams often keep Promptfoo for quick local prompt iteration and add AgentClash for the CI gate.",
  },
  {
    question: "Does AgentClash work from the CLI?",
    answer:
      "Yes. `agentclash run create`, `agentclash run follow`, and `agentclash regression run` are first-class commands. Runs can be fired from a developer laptop, a CI workflow, or a scheduled cron. The CLI ships via npm, Homebrew on macOS/Linux, and install scripts.",
  },
  {
    question: "What about red-teaming?",
    answer:
      "Promptfoo has the category-leading library of red-team presets. AgentClash is focused on agent behavior under real tool-use and isn't a drop-in replacement for its jailbreak suite. If safety-scanning prompts is your primary job, keep Promptfoo; add AgentClash when you need to race real agent trajectories.",
  },
  {
    question: "Is AgentClash open source?",
    answer:
      "Yes, under FSL-1.1-MIT. The full race engine, scorer pipeline, and CLI are self-hostable. Managed cloud is an option for teams that don't want to run Temporal and Postgres themselves.",
  },
];

export default function VsPromptfooPage() {
  return (
    <>
      <JsonLd
        id="ld-vs-promptfoo-product"
        data={productSchema({
          name: "AgentClash vs Promptfoo",
          description:
            "Side-by-side comparison of AgentClash (agent evaluation, trajectory scoring, durable orchestration) against Promptfoo (OSS CLI for git-committed prompt-eval configs and red-team presets).",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-vs-promptfoo-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Compare", url: "/v2/vs/promptfoo" },
          { name: "Promptfoo", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Compare" },
            { label: "Promptfoo" },
          ]}
          eyebrow="AgentClash vs Promptfoo"
          title={
            <>
              Sandboxes and verdicts.
              <br />
              <span className="text-white/40">Not local YAML asserts.</span>
            </>
          }
          subtitle={
            <>
              Promptfoo is one of the best OSS prompt-eval CLIs on the
              market — fast, git-native, and great at red-team presets.
              AgentClash is built for the next problem: racing multi-turn
              agents in a sandbox, with durable orchestration, and gating
              CI on the trajectory verdict.
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
              title="Promptfoo — typical config"
              language="yaml"
              code={`# promptfooconfig.yaml
prompts:
  - "Summarize: {{text}}"
providers:
  - openai:gpt-4o-mini
  - anthropic:claude-3-5-sonnet
tests:
  - vars: { text: "..." }
    assert:
      - type: contains
        value: "charter"
      - type: llm-rubric
        value: "concise, no speculation"`}
            />
          }
        />

        <SplitSection
          eyebrow="Where Promptfoo shines"
          title={
            <>
              Fast local loops.
              <br />
              <span className="text-white/40">Diffable prompt configs.</span>
            </>
          }
          body={
            <>
              <p>
                Promptfoo nails the &quot;evals in my repo&quot; workflow.
                Config in YAML, runs in seconds, diffs like code, and comes
                with a mature red-team preset library. For a solo engineer
                or a tight team iterating on prompts, it&apos;s hard to
                beat on feedback latency.
              </p>
              <p className="mt-4">
                AgentClash doesn&apos;t replace that. If your unit of work
                is a prompt + a set of string asserts in CI, keep using
                Promptfoo — it&apos;s great at that job.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Promptfoo — CLI"
              language="shell"
              code={`$ promptfoo eval

┌────────────────┬───────┬────────┐
│ provider       │ pass  │ rubric │
├────────────────┼───────┼────────┤
│ gpt-4o-mini    │ 12/14 │  8.2   │
│ claude-3-5     │ 13/14 │  9.1   │
└────────────────┴───────┴────────┘`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Where AgentClash wins"
          title={
            <>
              Agents using real tools.
              <br />
              <span className="text-white/40">Under durable orchestration.</span>
            </>
          }
          body={
            <>
              <p>
                Promptfoo is single-call-shaped by design. A multi-turn
                agent that runs for 90 seconds, calls five tools, recovers
                from one failure, and terminates on budget is a different
                unit of evaluation entirely.
              </p>
              <p className="mt-4">
                AgentClash runs those trajectories in ephemeral sandboxes
                with real tools, under Temporal-backed durable workflows.
                The verdict scores plan adherence, correctness, cost, and
                latency — and the flunks feed a CI regression gate.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="AgentClash — challenge pack"
              code={`$ agentclash run create \\
    --pack sre-runbook \\
    --agents gpt-5,claude-4.5,gemini-2.5 \\
    --follow

  gpt-5       ● on-plan   9.3  $0.031  22.1s
  claude-4.5  ● on-plan   9.0  $0.020  24.5s
  gemini-2.5  ◐ loop:1    6.8  $0.034  41.2s

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
                Where AgentClash pulls ahead
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six things a prompt CLI can&apos;t do for you.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock
          title="Promptfoo vs AgentClash, answered."
          items={FAQ_ITEMS}
          schemaId="ld-vs-promptfoo-faq"
        />

        <ClosingCTA
          title={
            <>
              Keep your prompt evals.
              <br />
              <span className="text-white/40">Race your agents.</span>
            </>
          }
          body={
            <p>
              Ship Promptfoo for local prompt iteration. Bring AgentClash
              in for the CI gate that actually fires real tools in a
              sandbox and blocks the merge when a trajectory regresses.
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
