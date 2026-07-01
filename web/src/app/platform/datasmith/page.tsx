import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  Database,
  Download,
  GitBranch,
  Sparkles,
  Terminal,
  Workflow,
} from "lucide-react";
import { ClashMark } from "@/components/marketing/clash-mark";
import {
  JsonLd,
  breadcrumbSchema,
  faqSchema,
  productSchema,
} from "@/components/marketing/json-ld";

const PAGE_PATH = "/platform/datasmith";
const PAGE_TITLE =
  "DataSmith Synthetic Data Generation for AI Agents - AgentClash";
const PAGE_DESCRIPTION =
  "Generate high-signal synthetic datasets with weak-vs-strong Agentic Self-Instruct. Open-source Python SDK for SFT and DPO export, plus hosted generation inside AgentClash for eval and regression.";
const SOCIAL_IMAGE_ALT =
  "AgentClash DataSmith synthetic data generation social preview.";

const faqItems = [
  {
    question: "What is DataSmith?",
    answer:
      "DataSmith is an open-source Python SDK for synthetic dataset generation using a weak-vs-strong agentic loop. A challenger proposes examples, weak and strong solvers attempt them, and a judge accepts only high-signal rows ready for fine-tuning or evaluation.",
  },
  {
    question: "How is DataSmith related to AgentClash?",
    answer:
      "DataSmith handles offline dataset creation and training export (SFT, DPO, Hugging Face). AgentClash runs the same Agentic Self-Instruct loop in hosted workspaces, then scores, replays, and gates regressions on the generated examples.",
  },
  {
    question: "What models does DataSmith support?",
    answer:
      "Any provider that implements DataSmith's complete() protocol: OpenAI-compatible APIs, local inference, or your own wrapper. Weak and strong solvers can be different models, prompts, or compute budgets.",
  },
  {
    question: "Can I turn production traces into training data?",
    answer:
      "Yes. DataSmith ingests OpenTelemetry JSON and span JSONL as seeds. AgentClash also supports trace import into workspace datasets for eval-oriented workflows.",
  },
];

const workflow = [
  {
    icon: Sparkles,
    title: "Construct seeds",
    text: "Start from a domain brief, web-grounded search, production traces, or source documents.",
  },
  {
    icon: Workflow,
    title: "Run weak vs strong",
    text: "Challenger proposes examples. Weak and strong solvers attempt them. Judge filters for the useful difficulty zone.",
  },
  {
    icon: Download,
    title: "Export for training",
    text: "Ship ShareGPT, ChatML, DPO pairs, or push accepted rows to Hugging Face Hub.",
  },
  {
    icon: GitBranch,
    title: "Eval and gate in AgentClash",
    text: "Run hosted Agentic Self-Instruct generation, baseline the dataset, and wire CI regression gates.",
  },
];

const capabilities = [
  "Web-grounded seed construction from domain briefs",
  "Weak-vs-strong Agentic Self-Instruct judge loop",
  "OpenTelemetry trace ingestion for seed material",
  "Accepted and rejected JSONL with reason codes",
  "ShareGPT, ChatML, DPO, and prompt-completion export",
  "Provider-agnostic model interface (OpenAI-compatible or custom)",
  "Hosted Agentic Self-Instruct inside AgentClash workspaces",
  "Dataset baselines, eval runs, and CI regression gates",
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
            href="https://github.com/Atharva-Kanherkar/datasmith"
            target="_blank"
            rel="noopener noreferrer"
            className="hidden px-3 py-1.5 text-white/55 transition-colors hover:text-white/85 sm:inline-flex"
          >
            DataSmith GitHub
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
          <Link href="/blog" className="transition-colors hover:text-white/75">
            Blog
          </Link>
          <a
            href="https://github.com/Atharva-Kanherkar/datasmith"
            target="_blank"
            rel="noopener noreferrer"
            className="transition-colors hover:text-white/75"
          >
            DataSmith
          </a>
        </nav>
      </div>
    </footer>
  );
}

function PipelineVisual() {
  return (
    <div className="overflow-hidden rounded-md border border-white/[0.08] bg-[#090909] shadow-2xl shadow-violet-950/20">
      <div className="border-b border-white/[0.08] px-4 py-3">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <span className="size-2 rounded-full bg-violet-300" />
            <span className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/45">
              agentic self-instruct
            </span>
          </div>
          <span className="rounded-md border border-violet-300/20 bg-violet-300/[0.06] px-2 py-1 font-[family-name:var(--font-mono)] text-2xs text-violet-100/75">
            avg_gap 0.31
          </span>
        </div>
      </div>
      <div className="grid gap-px bg-white/[0.06] md:grid-cols-[0.92fr_1.08fr]">
        <div className="bg-[#090909] p-5">
          <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
            acceptance policy
          </p>
          <div className="mt-5 space-y-3">
            {[
              ["Strong solver", "0.92", "pass >= 0.85"],
              ["Weak solver", "0.41", "pass <= 0.55"],
              ["Score gap", "0.51", "gap >= 0.20"],
              ["Judge verdict", "accepted", "grounded + rubric"],
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
            export targets
          </p>
          <div className="mt-5 space-y-4">
            {[
              "ShareGPT / ChatML for supervised fine-tuning",
              "DPO pairs when weak and strong outputs diverge",
              "AgentClash dataset eval and CI regression gates",
            ].map((item, index) => (
              <div key={item} className="flex items-start gap-3">
                <span className="mt-1 flex size-5 items-center justify-center rounded-md border border-violet-300/25 bg-violet-300/10 font-[family-name:var(--font-mono)] text-2xs text-violet-100">
                  {index + 1}
                </span>
                <span className="text-sm leading-6 text-white/62">{item}</span>
              </div>
            ))}
          </div>
          <pre className="mt-6 overflow-hidden rounded-md border border-white/[0.08] bg-black px-4 py-3 font-[family-name:var(--font-mono)] text-2xs leading-5 text-white/55">
            pip install datasmith{"\n"}datasmith run --seeds seeds.jsonl
          </pre>
        </div>
      </div>
    </div>
  );
}

export default function DataSmithPage() {
  return (
    <>
      <JsonLd
        id="agentclash-platform-datasmith-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "DataSmith", url: PAGE_PATH },
          ]),
          faqSchema(faqItems),
          productSchema({
            name: PAGE_TITLE,
            description: PAGE_DESCRIPTION,
            url: PAGE_PATH,
            applicationSubCategory: "Synthetic data generation software",
            featureList: capabilities,
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
                <span>DataSmith</span>
              </nav>
              <p className="mt-10 font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-violet-200/70">
                Synthetic data for agents
              </p>
              <h1 className="mt-5 max-w-[14ch] font-sans text-5xl font-semibold leading-none tracking-tight text-white sm:text-7xl">
                High-signal synthetic data, not noisy bulk
              </h1>
              <p className="mt-8 max-w-[58ch] text-base leading-8 text-white/62 sm:text-lg">
                DataSmith generates training and eval datasets with a
                weak-vs-strong agentic loop inspired by Meta FAIR Autodata. Use
                the open-source Python SDK locally, or run Agentic Self-Instruct
                inside AgentClash for replay, scoring, and CI gates.
              </p>
              <div className="mt-9 flex flex-col gap-3 sm:flex-row">
                <a
                  href="https://github.com/Atharva-Kanherkar/datasmith"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
                >
                  Star DataSmith on GitHub
                  <ArrowRight className="size-4" />
                </a>
                <Link
                  href="/auth/login"
                  className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
                >
                  Try hosted generation
                  <Database className="size-4" />
                </Link>
              </div>
            </div>
            <PipelineVisual />
          </div>
        </section>

        <section className="border-y border-white/[0.06] px-6 py-20 sm:px-12">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-3xl">
              <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
                Why weak vs strong
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
                Examples in the useful difficulty zone
              </h2>
              <p className="mt-5 text-sm leading-7 text-white/55 sm:text-base">
                Prompt-only self-instruct often produces tasks that are too easy
                or too hard. The weak-vs-strong loop accepts rows only when a
                strong solver succeeds and a weak solver struggles: the zone
                where fine-tuning actually teaches something.
              </p>
            </div>
            <div className="mt-12 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {capabilities.map((signal) => (
                <div
                  key={signal}
                  className="rounded-md border border-white/[0.08] bg-white/[0.03] p-5"
                >
                  <CheckCircle2 className="size-5 text-violet-200" />
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
                  From domain brief to trainer-ready export
                </h2>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                {workflow.map((item) => (
                  <div
                    key={item.title}
                    className="rounded-md border border-white/[0.08] bg-white/[0.03] p-5"
                  >
                    <item.icon className="size-5 text-violet-200" />
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
                Start here
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
                Local SDK or hosted workspace generation
              </h2>
              <p className="mt-5 text-sm leading-7 text-white/55 sm:text-base">
                Pick DataSmith when you need offline SFT and DPO export. Pick
                AgentClash when the same examples should baseline evals and
                block regressions in CI.
              </p>
            </div>
            <div className="grid gap-3">
              {[
                [
                  "Introducing DataSmith",
                  "Launch blog: weak-vs-strong loop, trace ingestion, and export formats.",
                  "/blog/introducing-datasmith-synthetic-agent-data",
                ],
                [
                  "Synthetic dataset generation guide",
                  "Run Agentic Self-Instruct inside AgentClash workspaces.",
                  "/docs/guides/synthetic-dataset-generation",
                ],
                [
                  "Agentic Self-Instruct SEO hub",
                  "Keyword landing for the Autodata-inspired generation pattern.",
                  "/agentic-self-instruct",
                ],
                [
                  "Datasets overview",
                  "Baselines, eval runs, and regression suite sync after generation.",
                  "/docs/guides/datasets-overview",
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
              Questions teams ask about synthetic data generation
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
            <div className="mt-12 flex flex-col gap-3 sm:flex-row">
              <Link
                href="/auth/login"
                className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
              >
                Start hosted generation
                <ArrowRight className="size-4" />
              </Link>
              <Link
                href="/docs/guides/synthetic-dataset-generation"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
              >
                Read the docs
                <Terminal className="size-4" />
              </Link>
            </div>
          </div>
        </section>
      </main>
      <StaticFooter />
    </>
  );
}
