import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  Code2,
  GitBranch,
  Play,
  ShieldCheck,
  Sparkles,
  Terminal,
} from "lucide-react";
import { ClashMark } from "@/components/marketing/clash-mark";
import {
  JsonLd,
  breadcrumbSchema,
  faqSchema,
  productSchema,
} from "@/components/marketing/json-ld";
import { AGENT_EVALUATION_FEATURES } from "@/lib/seo-features";

const PAGE_PATH = "/platform/agent-evaluation";
const PAGE_TITLE = "AI Agent Evaluation Platform for Real Tasks - AgentClash";
const PAGE_DESCRIPTION =
  "Evaluate AI agents on real tasks with same-tools races, sandboxed execution, replay, scorecards, challenge packs, and CI regression gates.";
const SOCIAL_IMAGE_ALT =
  "AgentClash AI agent evaluation platform social preview.";

const faqItems = [
  {
    question: "What is an AI agent evaluation platform?",
    answer:
      "An AI agent evaluation platform runs agents against repeatable tasks, captures what they did, scores outcomes, and helps teams decide whether an agent or model is ready to ship.",
  },
  {
    question: "How is AgentClash different from static benchmarks?",
    answer:
      "AgentClash runs your agents on your tasks with the same tools, constraints, and time budget, then records replay evidence and scorecards that can become regression gates.",
  },
  {
    question: "Can AgentClash run in CI?",
    answer:
      "Yes. AgentClash can compare a candidate run against a baseline and fail CI when the candidate regresses on the scorecard or release gate you define.",
  },
  {
    question: "How is AgentClash different from prompt-evaluation tools?",
    answer:
      "Prompt-evaluation tools score the text a model returns from a single call. AgentClash evaluates multi-turn agents that take actions in a sandbox and scores the whole trajectory. See agentclash.dev/compare for a side-by-side with Braintrust, LangSmith, Promptfoo, Langfuse, Arize Phoenix, and OpenAI Evals.",
  },
];

const workflow = [
  {
    icon: Code2,
    title: "Package the task",
    text: "Describe the workload as a challenge pack with inputs, tools, scoring rules, and artifacts.",
  },
  {
    icon: Play,
    title: "Race the agents",
    text: "Run every candidate against the same task at the same time with the same constraints.",
  },
  {
    icon: Sparkles,
    title: "Replay the evidence",
    text: "Inspect tool calls, outputs, artifacts, latency, cost, and judge evidence after the run.",
  },
  {
    icon: GitBranch,
    title: "Gate the release",
    text: "Compare candidate and baseline runs, then fail CI before a regression reaches users.",
  },
];

const proofPoints = AGENT_EVALUATION_FEATURES;

export const metadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  alternates: {
    canonical: PAGE_PATH,
  },
  openGraph: {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    url: PAGE_PATH,
    type: "website",
    locale: "en_US",
    siteName: "AgentClash",
    images: [
      {
        url: "/og-image.png",
        width: 1200,
        height: 630,
        alt: SOCIAL_IMAGE_ALT,
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    images: [
      {
        url: "/twitter-image.png",
        alt: SOCIAL_IMAGE_ALT,
      },
    ],
  },
};

function StaticHeader() {
  return (
    <header className="border-b border-white/[0.06] px-5 py-5 sm:px-12 sm:py-6">
      <div className="mx-auto flex max-w-[1440px] items-center justify-between gap-4">
        <Link
          href="/"
          className="inline-flex items-center gap-2.5 text-white/90"
        >
          <ClashMark className="size-6" />
          <span className="font-[family-name:var(--font-display)] text-xl tracking-normal">
            AgentClash
          </span>
        </Link>
        <nav className="flex items-center gap-2 text-xs">
          <Link
            href="/docs"
            className="hidden px-3 py-1.5 text-white/55 transition-colors hover:text-white/85 sm:inline-flex"
          >
            Docs
          </Link>
          <a
            href="https://github.com/agentclash/agentclash"
            target="_blank"
            rel="noopener noreferrer"
            className="hidden px-3 py-1.5 text-white/55 transition-colors hover:text-white/85 sm:inline-flex"
          >
            GitHub
          </a>
          <Link
            href="/auth/login"
            className="inline-flex items-center gap-1.5 rounded-md bg-white px-3 py-1.5 font-medium text-[#060606] transition-colors hover:bg-white/90"
          >
            Start
            <ArrowRight className="size-3.5" />
          </Link>
        </nav>
      </div>
    </header>
  );
}

function StaticFooter() {
  return (
    <footer className="border-t border-white/[0.06] bg-[#060606] px-6 py-10 sm:px-12">
      <div className="mx-auto flex max-w-[1440px] flex-col gap-5 text-sm text-white/45 sm:flex-row sm:items-center sm:justify-between">
        <Link href="/" className="font-medium text-white/65">
          AgentClash
        </Link>
        <nav className="flex flex-wrap gap-5">
          <Link href="/docs" className="transition-colors hover:text-white/75">
            Docs
          </Link>
          <Link href="/blog" className="transition-colors hover:text-white/75">
            Blog
          </Link>
          <Link href="/team" className="transition-colors hover:text-white/75">
            Team
          </Link>
          <a
            href="https://github.com/agentclash/agentclash"
            target="_blank"
            rel="noopener noreferrer"
            className="transition-colors hover:text-white/75"
          >
            GitHub
          </a>
        </nav>
      </div>
    </footer>
  );
}

function ProductVisual() {
  return (
    <div className="relative overflow-hidden rounded-md border border-white/[0.08] bg-[#090909] shadow-2xl shadow-cyan-950/20">
      <div className="border-b border-white/[0.08] px-4 py-3">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <span className="size-2 rounded-full bg-emerald-300" />
            <span className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/45">
              live race
            </span>
          </div>
          <span className="rounded-md border border-white/[0.08] px-2 py-1 font-[family-name:var(--font-mono)] text-2xs text-white/45">
            gate: pass
          </span>
        </div>
      </div>
      <div className="grid gap-px bg-white/[0.06] sm:grid-cols-3">
        {[
          ["Codex", "92", "correct patch, low cost"],
          ["Claude", "88", "strong reasoning"],
          ["Gemini", "73", "missed edge case"],
        ].map(([name, score, note]) => (
          <div key={name} className="bg-[#090909] p-4">
            <p className="text-sm font-medium text-white">{name}</p>
            <div className="mt-5 flex items-end justify-between gap-3">
              <span className="font-[family-name:var(--font-display)] text-5xl text-white">
                {score}
              </span>
              <span className="mb-1 text-right text-xs leading-5 text-white/45">
                {note}
              </span>
            </div>
          </div>
        ))}
      </div>
      <div className="grid gap-px bg-white/[0.06] md:grid-cols-[1.1fr_0.9fr]">
        <div className="bg-[#090909] p-5">
          <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
            replay timeline
          </p>
          <div className="mt-4 space-y-3">
            {[
              "cloned repo and installed dependencies",
              "ran failing test and inspected trace",
              "patched parser, reran unit suite",
              "attached diff, logs, and scorecard",
            ].map((item, index) => (
              <div key={item} className="flex items-start gap-3">
                <span className="mt-1 flex size-5 items-center justify-center rounded-md border border-cyan-300/25 bg-cyan-300/10 font-[family-name:var(--font-mono)] text-2xs text-cyan-100">
                  {index + 1}
                </span>
                <span className="text-sm leading-6 text-white/68">{item}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="bg-[#090909] p-5">
          <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
            ci verdict
          </p>
          <div className="mt-4 rounded-md border border-emerald-300/15 bg-emerald-300/[0.06] p-4">
            <div className="flex items-center gap-2 text-sm font-medium text-emerald-100">
              <ShieldCheck className="size-4" />
              Candidate clears release gate
            </div>
            <p className="mt-3 text-xs leading-5 text-emerald-50/60">
              Correctness +7, latency -11%, cost unchanged. No promoted
              failures exceeded the configured threshold.
            </p>
          </div>
          <pre className="mt-4 overflow-hidden rounded-md border border-white/[0.08] bg-black px-4 py-3 font-[family-name:var(--font-mono)] text-2xs leading-5 text-white/55">
            agentclash ci run --manifest .agentclash/ci.yaml
          </pre>
        </div>
      </div>
    </div>
  );
}

export default function AgentEvaluationPage() {
  return (
    <>
      <JsonLd
        id="agentclash-platform-agent-evaluation-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "AI Agent Evaluation Platform", url: PAGE_PATH },
          ]),
          faqSchema(faqItems),
          productSchema({
            name: PAGE_TITLE,
            description: PAGE_DESCRIPTION,
            url: PAGE_PATH,
            applicationSubCategory: "AI agent evaluation platform",
            featureList: proofPoints,
          }),
        ]}
      />
      <StaticHeader />
      <main className="min-h-screen bg-[#060606] text-white">
        <section className="px-6 py-20 sm:px-12 sm:py-28">
          <div className="mx-auto grid max-w-[1440px] gap-14 lg:grid-cols-[0.88fr_1.12fr] lg:items-center">
            <div>
              <nav className="flex items-center gap-2 text-xs text-white/35">
                <Link href="/" className="transition-colors hover:text-white/70">
                  Home
                </Link>
                <span>/</span>
                <span>Platform</span>
              </nav>
              <p className="mt-10 font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-cyan-200/70">
                AgentClash platform
              </p>
              <h1 className="mt-5 max-w-[11ch] font-[family-name:var(--font-display)] text-5xl font-normal leading-none tracking-normal text-white sm:text-7xl">
                AI agent evaluation platform for real tasks
              </h1>
              <p className="mt-8 max-w-[58ch] text-base leading-8 text-white/62 sm:text-lg">
                Race agents against the same workload with the same tools and
                same constraints. AgentClash captures replay evidence,
                scorecards, artifacts, and release-gate verdicts so evals turn
                into decisions instead of dashboard archaeology.
              </p>
              <div className="mt-9 flex flex-col gap-3 sm:flex-row">
                <Link
                  href="/auth/login"
                  className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
                >
                  Start first race
                  <ArrowRight className="size-4" />
                </Link>
                <Link
                  href="/docs/getting-started/quickstart"
                  className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
                >
                  Read quickstart
                  <Terminal className="size-4" />
                </Link>
              </div>
            </div>
            <ProductVisual />
          </div>
        </section>

        <section className="border-y border-white/[0.06] px-6 py-20 sm:px-12">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-3xl">
              <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
                What the platform evaluates
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
                The whole agent, not just the final answer
              </h2>
              <p className="mt-5 text-sm leading-7 text-white/55 sm:text-base">
                AgentClash looks at the trajectory that produced the answer:
                tool choices, runtime behavior, artifacts, cost, latency, and
                whether the agent actually satisfied the task.
              </p>
            </div>
            <div className="mt-12 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {proofPoints.map((point) => (
                <div
                  key={point}
                  className="rounded-md border border-white/[0.08] bg-white/[0.03] p-5"
                >
                  <CheckCircle2 className="size-5 text-emerald-200" />
                  <p className="mt-4 text-sm leading-6 text-white/78">
                    {point}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="px-6 py-20 sm:px-12 sm:py-28">
          <div className="mx-auto max-w-[1440px]">
            <div className="grid gap-12 lg:grid-cols-[0.7fr_1fr]">
              <div>
                <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
                  Workflow
                </p>
                <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
                  From one failed task to a reusable gate
                </h2>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                {workflow.map((item) => (
                  <div
                    key={item.title}
                    className="rounded-md border border-white/[0.08] bg-white/[0.03] p-5"
                  >
                    <item.icon className="size-5 text-cyan-200" />
                    <h3 className="mt-5 text-lg font-semibold tracking-normal text-white">
                      {item.title}
                    </h3>
                    <p className="mt-3 text-sm leading-6 text-white/55">
                      {item.text}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section className="border-y border-white/[0.06] px-6 py-20 sm:px-12">
          <div className="mx-auto grid max-w-[1440px] gap-10 lg:grid-cols-[0.8fr_1fr] lg:items-start">
            <div>
              <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
                Start with docs
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
                Bring your first workload into the loop
              </h2>
              <p className="mt-5 text-sm leading-7 text-white/55 sm:text-base">
                Use challenge packs for repeatable tasks, then wire the same
                workload into local runs, hosted runs, or pull request gates.
              </p>
            </div>
            <div className="grid gap-3">
              {[
                [
                  "Quickstart",
                  "Validate the CLI and get to your first runnable command.",
                  "/docs/getting-started/quickstart",
                ],
                [
                  "Write a challenge pack",
                  "Turn a real task into a repeatable agent evaluation.",
                  "/docs/guides/write-a-challenge-pack",
                ],
                [
                  "CI/CD agent gates",
                  "Fail a pull request when an agent regresses.",
                  "/docs/guides/ci-cd-agent-gates",
                ],
              ].map(([title, text, href]) => (
                <Link
                  key={href}
                  href={href}
                  className="group rounded-md border border-white/[0.08] bg-white/[0.03] p-5 transition-colors hover:border-white/20 hover:bg-white/[0.05]"
                >
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <h3 className="text-base font-semibold tracking-normal text-white">
                        {title}
                      </h3>
                      <p className="mt-2 text-sm leading-6 text-white/50">
                        {text}
                      </p>
                    </div>
                    <ArrowRight className="size-4 shrink-0 text-white/35 transition-colors group-hover:text-white/75" />
                  </div>
                </Link>
              ))}
            </div>
          </div>
        </section>

        <section className="px-6 py-20 sm:px-12 sm:py-28">
          <div className="mx-auto max-w-[960px]">
            <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
              FAQ
            </p>
            <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
              Questions teams ask before replacing benchmark-only evals
            </h2>
            <div className="mt-10 divide-y divide-white/[0.08] border-y border-white/[0.08]">
              {faqItems.map((item) => (
                <section key={item.question} className="py-6">
                  <h3 className="text-lg font-semibold tracking-normal text-white">
                    {item.question}
                  </h3>
                  <p className="mt-3 text-sm leading-7 text-white/55">
                    {item.answer}
                  </p>
                </section>
              ))}
            </div>
            <div className="mt-10 flex flex-col gap-3 sm:flex-row">
              <Link
                href="/auth/login"
                className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
              >
                Start first race
                <ArrowRight className="size-4" />
              </Link>
              <a
                href="https://github.com/agentclash/agentclash"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
              >
                View on GitHub
                <ArrowRight className="size-4" />
              </a>
            </div>
          </div>
        </section>
      </main>
      <StaticFooter />
    </>
  );
}
