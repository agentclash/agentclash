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

const PATH = "/v2/vs/braintrust";

export const metadata: Metadata = {
  title: "AgentClash vs Braintrust",
  description:
    "Braintrust is a polished SDK-first prompt-eval and experiment-tracking tool — great for hill-climbing a single prompt. AgentClash races multi-turn agent trajectories with real tools in a sandbox and gates CI on the verdict.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AgentClash vs Braintrust — agent eval vs prompt iteration",
    description:
      "Braintrust hill-climbs a single prompt. AgentClash races whole trajectories with real tools. A direct comparison.",
    url: PATH,
  },
};

const COLUMNS: ComparisonColumn[] = [
  { name: "AgentClash", tag: "Agent eval", highlight: true },
  { name: "Braintrust", tag: "Prompt eval + experiments" },
];

const ROWS: ComparisonRow[] = [
  {
    label: "Multi-turn agent loops",
    sub: "Scores plans, tool sequences, self-correction, and termination — not a single model call.",
    cells: ["yes", "partial"],
  },
  {
    label: "Sandboxed tool execution",
    sub: "Ephemeral VM with real filesystems, subprocesses, and network-policy isolation.",
    cells: ["yes", "no"],
  },
  {
    label: "Head-to-head concurrent race",
    sub: "N models on the same inputs, same budget, same scorer — in parallel.",
    cells: ["yes", "partial"],
  },
  {
    label: "Trajectory scoring",
    sub: "Plan adherence, tool-order signature, termination reason. Not just the final string.",
    cells: ["yes", "partial"],
  },
  {
    label: "Failures as CI regressions",
    sub: "Flunks freeze into a permanent regression suite that blocks future merges.",
    cells: ["yes", "no"],
  },
  {
    label: "Open source, self-hostable",
    sub: "Race engine, scorers, and CLI available under FSL-1.1-MIT.",
    cells: ["yes", "no"],
  },
  {
    label: "Single-prompt hill climbing UX",
    sub: "Scorecards, experiment diffs, and iterate-on-one-prompt workflows.",
    cells: ["partial", "yes"],
  },
  {
    label: "Custom scorers + LLM-as-judge SDK",
    sub: "Rich SDK for writing dataset-based scorers and custom graders.",
    cells: ["partial", "yes"],
  },
];

const FEATURES = [
  {
    label: "Sandbox",
    title: "Tool calls that actually execute.",
    body: "Each agent runs in an isolated E2B environment. No mocked tool stubs, no memorized outputs — the scorer sees what the tool actually returned.",
  },
  {
    label: "Race",
    title: "Concurrent competition, not sequential trials.",
    body: "Fire all your candidates at the same inputs with the same time budget in parallel. The verdict is a true ranking, not a sequence of experiment rows you line up by eye.",
  },
  {
    label: "Trajectory",
    title: "Score the path, not just the output.",
    body: "Plan shape, tool-order signature, self-correction, termination reason. Two models can reach the same string via very different competence.",
  },
  {
    label: "Provider-neutral",
    title: "Every major provider, normalized.",
    body: "OpenAI, Anthropic, Gemini, xAI, Mistral, OpenRouter. Tool-call shapes and failure codes normalized, so verdicts are apples-to-apples.",
  },
  {
    label: "CI native",
    title: "Regressions block the merge.",
    body: "A CLI designed for CI: posts verdicts as GitHub checks, blocks the PR when a trajectory regresses, links straight to the failing replay.",
  },
  {
    label: "Open",
    title: "Read the scorer. Fork the engine.",
    body: "Everything under FSL-1.1-MIT. The full race engine, scoring pipeline, and CLI are self-hostable — cloud is a convenience, not a lock-in.",
  },
];

const FAQ_ITEMS = [
  {
    question: "When should I use Braintrust instead?",
    answer:
      "If your primary workflow is iterating on a single prompt with a scorecard — comparing temperatures, tweaking system prompts, hill-climbing against a golden dataset — Braintrust has the best UX in the category. AgentClash is built for a different shape of problem: multi-turn agents with tool use, where the unit of evaluation is a trajectory, not a completion.",
  },
  {
    question: "Can I migrate from Braintrust?",
    answer:
      "Yes, gradually. Your golden datasets map into AgentClash challenge packs, but you'll want to upgrade them to describe tools, sandbox images, and trajectory rubrics instead of just input/reference pairs. Most teams run both: Braintrust for single-prompt iteration, AgentClash for agent-level CI gates.",
  },
  {
    question: "Do you support custom scorers?",
    answer:
      "Yes. AgentClash ships reference scorers for correctness, cost, latency, and behavior, and lets you drop in custom scorers as tool-calling agents themselves. The SDK surface is smaller than Braintrust's today — if you need a rich LLM-judge authoring workflow right now, Braintrust is ahead.",
  },
  {
    question: "Is AgentClash open source?",
    answer:
      "Yes, under FSL-1.1-MIT. Every part of the race engine, scorer pipeline, and CLI is self-hostable. Managed cloud is an option, not a requirement.",
  },
  {
    question: "How is the CI story different?",
    answer:
      "Braintrust leans on SDK invocations during your test runs. AgentClash ships a first-class CLI that spawns durable workflows, waits on verdicts, and posts GitHub checks — the flunk archive becomes a regression suite automatically, so the next model ships against a higher bar than the last.",
  },
];

export default function VsBraintrustPage() {
  return (
    <>
      <JsonLd
        id="ld-vs-braintrust-product"
        data={productSchema({
          name: "AgentClash vs Braintrust",
          description:
            "Side-by-side comparison of AgentClash (agent evaluation, trajectory scoring, CI gates) against Braintrust (SDK-first prompt eval, experiment tracking, custom scorers).",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-vs-braintrust-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Compare", url: "/v2/vs/braintrust" },
          { name: "Braintrust", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Compare" },
            { label: "Braintrust" },
          ]}
          eyebrow="AgentClash vs Braintrust"
          title={
            <>
              Races and regressions.
              <br />
              <span className="text-white/40">Not experiment rows.</span>
            </>
          }
          subtitle={
            <>
              Braintrust is one of the cleanest SDK-first prompt-eval tools
              you can buy — great for iterating on a single prompt with a
              scorecard. AgentClash races whole agent trajectories with
              real tools, concurrent, in a sandbox, and gates CI on the
              verdict. Different shape of problem.
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
              title="Braintrust — typical experiment"
              language="typescript"
              code={`import { Eval } from "braintrust";
import { LLMClassifier } from "autoevals";

Eval("rag-qa", {
  data: () => dataset,
  task: async ({ input }) => {
    return await chain({ question: input });
  },
  scores: [LLMClassifier("correct", {
    prompt: "Did the answer cite p.7?",
  })],
});`}
            />
          }
        />

        <SplitSection
          eyebrow="Where Braintrust shines"
          title={
            <>
              Iterate on a prompt.
              <br />
              <span className="text-white/40">With a polished scorecard.</span>
            </>
          }
          body={
            <>
              <p>
                If the unit of work is a single prompt — system message,
                few-shot examples, temperature — Braintrust has the most
                polished experiment-tracking UX in the category. Custom
                scorers are a pleasure to write, the diff view is tasteful,
                and hill-climbing a golden dataset feels right.
              </p>
              <p className="mt-4">
                AgentClash doesn&apos;t try to out-prompt-eval Braintrust.
                If your job is to get one prompt 2% better against a
                reference set, keep using it.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Braintrust — experiment SDK"
              language="typescript"
              code={`import { initExperiment } from "braintrust";

const exp = initExperiment("prompt-a-vs-b");

for (const row of goldenSet) {
  const out = await llm(row.input);
  exp.log({
    input: row.input,
    output: out,
    expected: row.expected,
    scores: { exact: row.expected === out ? 1 : 0 },
  });
}`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Where AgentClash wins"
          title={
            <>
              Trajectories.
              <br />
              <span className="text-white/40">Concurrent. In a sandbox.</span>
            </>
          }
          body={
            <>
              <p>
                Experiments are sequential by design. Trajectories
                are not. When an agent makes eight tool calls, handles a
                flaky search, and decides to terminate — that whole path
                is the unit of evaluation. Racing it head-to-head against
                three other providers is what tells you who to ship.
              </p>
              <p className="mt-4">
                AgentClash does that in a real sandbox, not a mock harness.
                Tools execute. Files change. Subprocesses run. The verdict
                is based on what actually happened.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="AgentClash — race + verdict"
              code={`$ agentclash run create \\
    --pack sql-refactor \\
    --agents gpt-5,claude-4.5,gemini-2.5,grok-4 \\
    --follow

  gpt-5       ● on-plan   9.1  $0.019  11.2s
  claude-4.5  ● on-plan   8.8  $0.014  13.8s
  gemini-2.5  ◐ loop:2    6.4  $0.022  24.1s
  grok-4      ● on-plan   8.2  $0.011  10.4s

verdict → winner: grok-4 (cost-weighted)`}
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
                Six things an experiment tracker can&apos;t do.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock
          title="Braintrust vs AgentClash, answered."
          items={FAQ_ITEMS}
          schemaId="ld-vs-braintrust-faq"
        />

        <ClosingCTA
          title={
            <>
              Keep your scorecards.
              <br />
              <span className="text-white/40">Race your agents.</span>
            </>
          }
          body={
            <p>
              We&apos;ll set up a head-to-head race on your hardest
              multi-turn task in 20 minutes. If an experiment row would
              have told you the same thing, we&apos;ll say so out loud.
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
