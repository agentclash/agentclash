import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  GitCompare,
  GitPullRequest,
  ListChecks,
  ShieldCheck,
  Terminal,
  TimerReset,
} from "lucide-react";
import { ClashMark } from "@/components/marketing/clash-mark";
import {
  JsonLd,
  breadcrumbSchema,
  faqSchema,
  productSchema,
} from "@/components/marketing/json-ld";

const PAGE_PATH = "/platform/agent-regression-testing";
const PAGE_TITLE = "AI Agent Regression Testing from Traces to CI Gates - AgentClash";
const PAGE_DESCRIPTION =
  "Turn production traces and pinned datasets into AI agent regression tests with baselines, replay evidence, scorecards, and pull request gates.";
const SOCIAL_IMAGE_ALT =
  "AgentClash AI agent regression testing social preview.";

const faqItems = [
  {
    question: "What is AI agent regression testing?",
    answer:
      "AI agent regression testing reruns production-inspired tasks against a candidate agent, compares the result to a baseline, and flags behavior that got worse before it ships.",
  },
  {
    question: "Can AgentClash use production traces or datasets?",
    answer:
      "Yes. AgentClash supports OTel-compatible trace import and pinned datasets so real failures, golden examples, and curated cases can become repeatable regression coverage.",
  },
  {
    question: "How do AgentClash CI gates work?",
    answer:
      "AgentClash can run a challenge pack in CI, compare candidate and baseline scorecards, then fail a pull request when correctness, cost, latency, or required artifacts cross the threshold you set.",
  },
  {
    question: "Can teams debug a failed agent gate?",
    answer:
      "Yes. Each run keeps replay evidence, tool calls, logs, artifacts, and scorecards so reviewers can see why the candidate failed instead of guessing from a final answer alone.",
  },
];

const workflow = [
  {
    icon: ListChecks,
    title: "Import traces or datasets",
    text: "Start from OTel-compatible production traces, curated examples, or escaped failures that already matter to the business.",
  },
  {
    icon: GitCompare,
    title: "Compare against a baseline",
    text: "Run the candidate under the same constraints and compare correctness, latency, cost, and evidence against known-good behavior.",
  },
  {
    icon: ShieldCheck,
    title: "Block risky changes",
    text: "Fail the pull request when the scorecard regresses past the release gate threshold.",
  },
  {
    icon: TimerReset,
    title: "Promote failures",
    text: "Convert failed traces and dataset rows into reusable regression cases so the same mistake stays covered.",
  },
];

const gateSignals = [
  "OTel-compatible trace import",
  "Pinned datasets and golden examples",
  "Baseline versus candidate scorecards",
  "Replay timelines for every failed gate",
  "Artifact checks for files, logs, and evidence",
  "Cost and latency thresholds for production budgets",
  "Pull request gates for model, prompt, RAG, and tool changes",
];

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
          <Link
            href="/platform/agent-evaluation"
            className="transition-colors hover:text-white/75"
          >
            Agent evaluation
          </Link>
          <Link href="/docs" className="transition-colors hover:text-white/75">
            Docs
          </Link>
          <Link href="/blog" className="transition-colors hover:text-white/75">
            Blog
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

function GateVisual() {
  return (
    <div className="overflow-hidden rounded-md border border-white/[0.08] bg-[#090909] shadow-2xl shadow-emerald-950/20">
      <div className="border-b border-white/[0.08] px-4 py-3">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <span className="size-2 rounded-full bg-emerald-300" />
            <span className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/45">
              pull request gate
            </span>
          </div>
          <span className="rounded-md border border-rose-300/20 bg-rose-300/[0.06] px-2 py-1 font-[family-name:var(--font-mono)] text-2xs text-rose-100/75">
            blocked
          </span>
        </div>
      </div>
      <div className="grid gap-px bg-white/[0.06] md:grid-cols-[0.92fr_1.08fr]">
        <div className="bg-[#090909] p-5">
          <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
            regression scorecard
          </p>
          <div className="mt-5 space-y-3">
            {[
              ["Correctness", "-9", "threshold -3"],
              ["Latency", "+18%", "threshold +15%"],
              ["Cost", "+2%", "threshold +10%"],
              ["Artifacts", "missing", "required"],
            ].map(([label, value, threshold]) => (
              <div
                key={label}
                className="rounded-md border border-white/[0.08] bg-white/[0.025] p-4"
              >
                <div className="flex items-center justify-between gap-4">
                  <span className="text-sm font-medium text-white/78">
                    {label}
                  </span>
                  <span className="font-[family-name:var(--font-mono)] text-sm text-white">
                    {value}
                  </span>
                </div>
                <p className="mt-2 font-[family-name:var(--font-mono)] text-2xs text-white/35">
                  {threshold}
                </p>
              </div>
            ))}
          </div>
        </div>
        <div className="bg-[#090909] p-5">
          <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
            replay evidence
          </p>
          <div className="mt-5 space-y-4">
            {[
              "candidate skipped validator retry after tool timeout",
              "baseline produced required patch and attached artifact",
              "candidate final answer passed prose check but failed file check",
            ].map((item, index) => (
              <div key={item} className="flex items-start gap-3">
                <span className="mt-1 flex size-5 items-center justify-center rounded-md border border-emerald-300/25 bg-emerald-300/10 font-[family-name:var(--font-mono)] text-2xs text-emerald-100">
                  {index + 1}
                </span>
                <span className="text-sm leading-6 text-white/62">{item}</span>
              </div>
            ))}
          </div>
          <pre className="mt-6 overflow-hidden rounded-md border border-white/[0.08] bg-black px-4 py-3 font-[family-name:var(--font-mono)] text-2xs leading-5 text-white/55">
            agentclash compare gate --baseline run-stable --candidate run-pr-184
          </pre>
        </div>
      </div>
    </div>
  );
}

export default function AgentRegressionTestingPage() {
  return (
    <>
      <JsonLd
        id="agentclash-platform-agent-regression-testing-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "AI Agent Regression Testing", url: PAGE_PATH },
          ]),
          faqSchema(faqItems),
          productSchema({
            name: PAGE_TITLE,
            description: PAGE_DESCRIPTION,
            url: PAGE_PATH,
            applicationSubCategory: "AI agent regression testing software",
            featureList: gateSignals,
          }),
        ]}
      />
      <StaticHeader />
      <main className="min-h-screen bg-[#060606] text-white">
        <section className="px-6 py-20 sm:px-12 sm:py-28">
          <div className="mx-auto grid max-w-[1440px] gap-14 lg:grid-cols-[0.9fr_1.1fr] lg:items-center">
            <div>
              <nav className="flex items-center gap-2 text-xs text-white/35">
                <Link href="/" className="transition-colors hover:text-white/70">
                  Home
                </Link>
                <span>/</span>
                <span>AI Agent Regression Testing</span>
              </nav>
              <p className="mt-10 font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-emerald-200/70">
                Release gates for agents
              </p>
              <h1 className="mt-5 max-w-[13ch] font-sans text-5xl font-semibold leading-none tracking-tight text-white sm:text-7xl">
                AI agent regression testing from traces to CI
              </h1>
              <p className="mt-8 max-w-[58ch] text-base leading-8 text-white/62 sm:text-lg">
                Agent changes can pass a demo and still get worse in production.
                AgentClash turns traces, datasets, and escaped failures into
                repeatable tests, then blocks pull requests when scorecards or
                evidence regress.
              </p>
              <div className="mt-9 flex flex-col gap-3 sm:flex-row">
                <Link
                  href="/auth/login"
                  className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
                >
                  Start first gate
                  <ArrowRight className="size-4" />
                </Link>
                <Link
                  href="/docs/guides/ci-cd-agent-gates"
                  className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
                >
                  Read CI guide
                  <Terminal className="size-4" />
                </Link>
              </div>
            </div>
            <GateVisual />
          </div>
        </section>

        <section className="border-y border-white/[0.06] px-6 py-20 sm:px-12">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-3xl">
              <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
                What a gate checks
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
                Regression signals from the workflows users actually run
              </h2>
              <p className="mt-5 text-sm leading-7 text-white/55 sm:text-base">
                Prompts, models, tools, retrieval, and sandbox images all move.
                AgentClash makes production-inspired workloads repeatable so the
                release decision is based on behavior, not a one-off transcript.
              </p>
            </div>
            <div className="mt-12 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {gateSignals.map((signal) => (
                <div
                  key={signal}
                  className="rounded-md border border-white/[0.08] bg-white/[0.03] p-5"
                >
                  <CheckCircle2 className="size-5 text-emerald-200" />
                  <p className="mt-4 text-sm leading-6 text-white/78">
                    {signal}
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
                  From production evidence to pull request gate
                </h2>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                {workflow.map((item) => (
                  <div
                    key={item.title}
                    className="rounded-md border border-white/[0.08] bg-white/[0.03] p-5"
                  >
                    <item.icon className="size-5 text-emerald-200" />
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
                Wire regression gates into the release loop
              </h2>
              <p className="mt-5 text-sm leading-7 text-white/55 sm:text-base">
                Start with one dataset, trace import, or challenge pack and one
                release gate. Then add escaped failures as reusable cases
                instead of rebuilding the entire eval stack every time an agent
                changes.
              </p>
            </div>
            <div className="grid gap-3">
              {[
                [
                  "CI/CD agent gates",
                  "Fail a pull request when an agent regresses.",
                  "/docs/guides/ci-cd-agent-gates",
                ],
                [
                  "Datasets overview",
                  "Import examples, record baselines, and sync regression suites.",
                  "/docs/guides/datasets-overview",
                ],
                [
                  "CI/CD workload recipes",
                  "Choose practical workloads for agent release checks.",
                  "/docs/guides/ci-cd-workload-recipes",
                ],
                [
                  "Interpret results",
                  "Read scorecards, replay evidence, and regression signals.",
                  "/docs/guides/interpret-results",
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
              Questions teams ask before gating agent releases
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
                Start first gate
                <ArrowRight className="size-4" />
              </Link>
              <Link
                href="/platform/agent-evaluation"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
              >
                Compare eval platform
                <GitPullRequest className="size-4" />
              </Link>
            </div>
          </div>
        </section>
      </main>
      <StaticFooter />
    </>
  );
}
