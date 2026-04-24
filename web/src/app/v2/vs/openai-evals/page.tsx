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

const PATH = "/v2/vs/openai-evals";

export const metadata: Metadata = {
  title: "AgentClash vs OpenAI Evals",
  description:
    "OpenAI Evals is a free, bare-bones OSS eval framework optimized for OpenAI models and text grading. AgentClash races multi-turn agent trajectories across every major provider with real tools in a sandbox and gates CI on the verdict.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AgentClash vs OpenAI Evals — agent eval vs text-grading CLI",
    description:
      "OpenAI Evals grades single OpenAI completions. AgentClash races multi-provider agent trajectories with real tools. A direct comparison.",
    url: PATH,
  },
};

const COLUMNS: ComparisonColumn[] = [
  { name: "AgentClash", tag: "Agent eval", highlight: true },
  { name: "OpenAI Evals", tag: "OSS text-grading CLI" },
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
    label: "Cross-provider head-to-head",
    sub: "OpenAI, Anthropic, Gemini, xAI, Mistral, OpenRouter — normalized tool-call shapes.",
    cells: ["yes", "no"],
  },
  {
    label: "Trajectory scoring",
    sub: "Plan adherence, tool-order signature, termination reason — not string match.",
    cells: ["yes", "no"],
  },
  {
    label: "Live event stream + replay",
    sub: "WebSocket tailing while the run happens; full replay months later.",
    cells: ["yes", "no"],
  },
  {
    label: "Flunk → CI regression gate",
    sub: "Failed runs freeze into a regression suite the next model must clear.",
    cells: ["yes", "no"],
  },
  {
    label: "Free + barebones text-grading",
    sub: "Classic `oaieval` CLI for comparing completions on reference datasets.",
    cells: ["partial", "yes"],
  },
  {
    label: "OpenAI-first completion format",
    sub: "Direct wiring to OpenAI's APIs, optimized for their model set.",
    cells: ["partial", "yes"],
  },
];

const FEATURES = [
  {
    label: "Sandbox",
    title: "Real tools, not stubbed functions.",
    body: "Every agent gets an ephemeral E2B environment — real filesystem, real subprocesses, real network policy. Text-grading CLIs can only score what a completion said; sandboxes test what the agent did.",
  },
  {
    label: "Providers",
    title: "Not OpenAI-only.",
    body: "First-class adapters for OpenAI, Anthropic, Gemini, xAI, Mistral, and OpenRouter. Tool-call shapes and failure codes normalized, so a challenge pack runs across every provider without code changes.",
  },
  {
    label: "Trajectory",
    title: "Score the path, not the string.",
    body: "Tool-order signatures, plan adherence, self-correction, termination. A grader that string-matches a completion cannot tell you the model looped three times to say it.",
  },
  {
    label: "Live",
    title: "Watch it run in real time.",
    body: "WebSocket event stream: every think, tool call, observation, and scoring update in the browser as it happens. No batch log parsing after the fact.",
  },
  {
    label: "CI gate",
    title: "Block the merge on drift.",
    body: "CLI ships a `regression run` command designed for CI — verdicts post as GitHub checks and link straight to the failing replay.",
  },
  {
    label: "Durable",
    title: "Temporal-backed workflows.",
    body: "Long-running agent runs survive provider 503s, node restarts, and flaky tools. Replays work months later because events are durably persisted.",
  },
];

const FAQ_ITEMS = [
  {
    question: "When should I use OpenAI Evals instead?",
    answer:
      "If you only ship on OpenAI, your tests are mostly reference text-grading (`completion == expected` or a rubric), and you want a free, simple, hackable framework — OpenAI Evals is the right starting point. AgentClash is built for the next shape of problem: multi-turn agents with real tool use, raced across every major provider.",
  },
  {
    question: "Can I migrate from OpenAI Evals?",
    answer:
      "Yes. OpenAI Evals YAML registry entries map naturally into AgentClash challenge packs — both are declarative and describe inputs + graders. You'll want to enrich them with tool specs, sandbox images, and trajectory rubrics to get the agent-eval benefits. Most teams keep OpenAI Evals for quick text-grading experiments and add AgentClash for the CI gate.",
  },
  {
    question: "Does AgentClash only score OpenAI models?",
    answer:
      "No — the opposite. First-class adapters for OpenAI, Anthropic, Gemini, xAI, Mistral, and OpenRouter. The whole point is that one challenge pack runs across providers with normalized tool-call shapes and failure codes, so the verdict is apples-to-apples.",
  },
  {
    question: "Is AgentClash free?",
    answer:
      "The engine is open source under FSL-1.1-MIT — free to run, fork, and self-host. A managed cloud is available for teams that don't want to operate Temporal and Postgres themselves; it's an option, not a requirement.",
  },
  {
    question: "What do I lose by leaving OpenAI Evals?",
    answer:
      "The built-in OpenAI-shaped registry, the shared CLI muscle memory if your team already uses it, and some text-grading presets. You gain real tool execution, trajectory scoring, multi-provider racing, durable orchestration, and a CI regression story. It's a trade — worth it once your tests are agent-shaped, not completion-shaped.",
  },
];

export default function VsOpenAIEvalsPage() {
  return (
    <>
      <JsonLd
        id="ld-vs-openai-evals-product"
        data={productSchema({
          name: "AgentClash vs OpenAI Evals",
          description:
            "Side-by-side comparison of AgentClash (multi-provider agent evaluation, trajectory scoring, CI gates) against OpenAI Evals (OSS text-grading CLI optimized for OpenAI models).",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-vs-openai-evals-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Compare", url: "/v2/vs/openai-evals" },
          { name: "OpenAI Evals", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Compare" },
            { label: "OpenAI Evals" },
          ]}
          eyebrow="AgentClash vs OpenAI Evals"
          title={
            <>
              Cross-provider races.
              <br />
              <span className="text-white/40">Not OpenAI-only text grading.</span>
            </>
          }
          subtitle={
            <>
              OpenAI Evals is a free, bare-bones OSS framework — perfect
              when you only ship on OpenAI and your tests are text-grading
              against a reference. AgentClash is built for the next shape:
              multi-turn agents with real tools, raced across every major
              provider, gating CI on the trajectory verdict.
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
              title="OpenAI Evals — oaieval CLI"
              language="shell"
              code={`$ oaieval gpt-4o-mini match \\
    --registry_path ./evals \\
    --record_path ./results.jsonl

# runs the "match" grader against
# reference completions and dumps a
# line-delimited JSON result file`}
            />
          }
        />

        <SplitSection
          eyebrow="Where OpenAI Evals shines"
          title={
            <>
              Free. Simple.
              <br />
              <span className="text-white/40">OpenAI-native.</span>
            </>
          }
          body={
            <>
              <p>
                OpenAI Evals is one of the most honest options on the
                market: a small YAML registry, a `oaieval` CLI, and a
                handful of canonical graders. If you ship exclusively on
                OpenAI and your eval needs are &quot;grade completions
                against a reference dataset,&quot; it does that job well
                and it&apos;s free.
              </p>
              <p className="mt-4">
                AgentClash doesn&apos;t try to out-cheap a GitHub repo. If
                that&apos;s your workflow, keep it — it&apos;s a good
                starting point.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="OpenAI Evals — registry entry"
              language="yaml"
              code={`# evals/qa-match.yaml
qa-match:
  id: qa-match.v0
  metrics: [accuracy]
  description: Short-answer match eval

qa-match.v0:
  class: evals.elsuite.basic.match:Match
  args:
    samples_jsonl: qa/test.jsonl`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Where AgentClash wins"
          title={
            <>
              Multi-provider.
              <br />
              <span className="text-white/40">Multi-turn. Sandboxed.</span>
            </>
          }
          body={
            <>
              <p>
                Completions grading is one-shot. Real agents make eight
                tool calls, recover from a flaky API, and terminate on
                budget. Measuring them needs a sandbox, a concurrent race,
                and a trajectory-aware scorer — not a string comparison.
              </p>
              <p className="mt-4">
                AgentClash races agents from OpenAI, Anthropic, Gemini,
                xAI, Mistral, and OpenRouter on the same challenge pack
                with normalized tool-call shapes, and gates CI on the
                verdict. Flunks freeze into a regression suite the next
                model has to clear.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="AgentClash — cross-provider race"
              code={`$ agentclash run create \\
    --pack code-review-hard \\
    --agents gpt-5,claude-4.5,gemini-2.5,grok-4 \\
    --follow

  gpt-5       ● on-plan   9.2  $0.021  12.8s
  claude-4.5  ● on-plan   9.0  $0.016  14.1s
  gemini-2.5  ◐ loop:1    7.1  $0.024  22.4s
  grok-4      ● on-plan   8.4  $0.012  11.0s

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
                Six things a text-grading CLI can&apos;t do.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock
          title="OpenAI Evals vs AgentClash, answered."
          items={FAQ_ITEMS}
          schemaId="ld-vs-openai-evals-faq"
        />

        <ClosingCTA
          title={
            <>
              Outgrow text grading.
              <br />
              <span className="text-white/40">Race real agents.</span>
            </>
          }
          body={
            <p>
              Bring us one task that OpenAI Evals can&apos;t tell you the
              answer to — something multi-turn, tool-using, or
              cross-provider. We&apos;ll race it in a sandbox in 20 minutes
              and show you the delta.
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
