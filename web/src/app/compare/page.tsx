import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import {
  JsonLd,
  SITE_URL,
  breadcrumbSchema,
  faqSchema,
  productSchema,
} from "@/components/marketing/json-ld";
import {
  COMPARISON_COLUMNS,
  COMPARISON_ROWS,
  COMPETITORS,
  MARK_LABEL,
} from "@/lib/comparison-data";
import { ogImageUrl } from "@/lib/seo";
import { AGENT_EVALUATION_FEATURES } from "@/lib/seo-features";
import { CompareShell } from "./_components/compare-shell";

const PAGE_PATH = "/compare";
const PAGE_TITLE =
  "Best AI Agent Evaluation Tools: AgentClash vs Prompt-Eval Platforms";
const PAGE_DESCRIPTION =
  "Compare AgentClash with Braintrust, LangSmith, Promptfoo, Langfuse, Arize Phoenix, and OpenAI Evals. See how sandboxed, multi-turn, same-task agent evaluation differs from prompt evaluation.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "AgentClash vs prompt-eval tools",
  subtitle: "Agent evaluation, not prompt evaluation",
  kind: "Compare",
});

const faqItems = [
  {
    question:
      "What is the difference between agent evaluation and prompt evaluation?",
    answer:
      "Prompt evaluation scores the text a model returns from a single call against a dataset or rubric. Agent evaluation runs a model as an agent — it takes actions, calls tools, and works for minutes in a real environment — then scores the whole trajectory, not just the final answer. AgentClash is built for agent evaluation; most other tools focus on prompt evaluation.",
  },
  {
    question: "What are the best AI agent evaluation tools?",
    answer:
      "Braintrust, LangSmith, Promptfoo, Langfuse, Arize Phoenix, and OpenAI Evals are excellent for prompt-level evaluation, tracing, and observability. AgentClash is purpose-built for evaluating tool-using agents on the same task in a sandbox, with replay, scorecards, and CI regression gates.",
  },
  {
    question: "Is AgentClash open source?",
    answer:
      "Yes. AgentClash is open source under the MIT license. You can self-host it or run against the hosted backend, and the CLI ships on npm as the agentclash package.",
  },
  {
    question: "Which AgentClash alternative should I choose?",
    answer:
      "If you mainly need prompt and output scoring, datasets, and tracing, a prompt-eval tool may be enough. If you need to evaluate multi-turn, tool-using agents end-to-end — with sandboxed execution, trajectory scoring, and CI gates — AgentClash is the closer fit. Many teams use both.",
  },
];

export const metadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  alternates: { canonical: PAGE_PATH },
  openGraph: {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    url: PAGE_PATH,
    type: "website",
    locale: "en_US",
    siteName: "AgentClash",
    images: [{ url: SOCIAL_IMAGE, width: 1200, height: 630, alt: PAGE_TITLE }],
  },
  twitter: {
    card: "summary_large_image",
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    images: [{ url: SOCIAL_IMAGE, alt: PAGE_TITLE }],
  },
};

const itemListSchema = {
  "@context": "https://schema.org",
  "@type": "ItemList",
  name: "AgentClash comparisons",
  url: `${SITE_URL}${PAGE_PATH}`,
  itemListElement: COMPETITORS.map((competitor, index) => ({
    "@type": "ListItem",
    position: index + 1,
    name: `AgentClash vs ${competitor.name}`,
    url: `${SITE_URL}/compare/${competitor.slug}`,
  })),
};

export default function ComparePage() {
  return (
    <>
      <JsonLd
        id="agentclash-compare-hub-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "Compare", url: PAGE_PATH },
          ]),
          faqSchema(faqItems),
          productSchema({
            name: "AgentClash",
            description: PAGE_DESCRIPTION,
            url: PAGE_PATH,
            applicationSubCategory: "AI agent evaluation platform",
            featureList: AGENT_EVALUATION_FEATURES,
          }),
          itemListSchema,
        ]}
      />
      <CompareShell>
        <section className="px-6 py-20 sm:px-12 sm:py-28">
          <div className="mx-auto max-w-[1440px]">
            <nav className="flex items-center gap-2 text-xs text-white/35">
              <Link href="/" className="transition-colors hover:text-white/70">
                Home
              </Link>
              <span>/</span>
              <span>Compare</span>
            </nav>
            <p className="mt-10 font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-cyan-200/70">
              Agent eval vs prompt eval
            </p>
            <h1 className="mt-5 max-w-[18ch] font-[family-name:var(--font-display)] text-4xl font-normal leading-[1.05] tracking-tight text-white sm:text-6xl">
              They test prompts. AgentClash debugs agents.
            </h1>
            <p className="mt-8 max-w-[64ch] text-base leading-8 text-white/62 sm:text-lg">
              {PAGE_DESCRIPTION}
            </p>
          </div>
        </section>

        <section className="border-y border-white/[0.06] px-6 py-16 sm:px-12">
          <div className="mx-auto max-w-[1440px]">
            <h2 className="text-2xl font-semibold tracking-tight text-white sm:text-3xl">
              Capability comparison
            </h2>
            <p className="mt-3 max-w-[70ch] text-sm leading-7 text-white/55">
              The tools below are excellent at prompt engineering — scoring the
              text a model produces from a single call. AgentClash is built for
              the next problem over: evaluating agents that take actions, use
              tools, and run for minutes at a time in a real sandbox.
            </p>
            <div className="mt-10 overflow-x-auto">
              <table className="w-full min-w-[900px] border-collapse text-left">
                <thead>
                  <tr className="border-b border-white/[0.14]">
                    <th
                      scope="col"
                      className="py-4 pr-4 text-2xs font-medium uppercase tracking-[0.16em] text-white/40"
                    >
                      Capability
                    </th>
                    {COMPARISON_COLUMNS.map((column) => (
                      <th
                        key={column.name}
                        scope="col"
                        className={`px-3 py-4 text-center align-bottom ${
                          column.highlight ? "text-white" : "text-white/55"
                        }`}
                      >
                        <span className="block text-sm font-semibold">
                          {column.name}
                        </span>
                        <span className="mt-1 block text-2xs font-[family-name:var(--font-mono)] uppercase tracking-[0.2em] text-white/35">
                          {column.tag}
                        </span>
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {COMPARISON_ROWS.map((row) => (
                    <tr
                      key={row.label}
                      className="border-b border-white/[0.06] align-top"
                    >
                      <th scope="row" className="py-5 pr-6 font-normal">
                        <span className="block text-base text-white/85">
                          {row.label}
                        </span>
                        <span className="mt-1 block max-w-[42ch] text-xs leading-5 text-white/40">
                          {row.sub}
                        </span>
                      </th>
                      {row.cells.map((cell, index) => (
                        <td
                          key={COMPARISON_COLUMNS[index].name}
                          className={`px-3 py-5 text-center text-sm ${
                            index === 0
                              ? "font-medium text-white"
                              : "text-white/55"
                          }`}
                        >
                          {MARK_LABEL[cell]}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <section className="px-6 py-16 sm:px-12 sm:py-24">
          <div className="mx-auto max-w-[1440px]">
            <h2 className="text-2xl font-semibold tracking-tight text-white sm:text-3xl">
              Compare AgentClash side by side
            </h2>
            <div className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {COMPETITORS.map((competitor) => (
                <Link
                  key={competitor.slug}
                  href={`/compare/${competitor.slug}`}
                  className="group rounded-md border border-white/[0.08] bg-white/[0.03] p-5 transition-colors hover:border-white/20 hover:bg-white/[0.05]"
                >
                  <div className="flex items-center justify-between gap-4">
                    <span className="text-base font-semibold text-white">
                      AgentClash vs {competitor.name}
                    </span>
                    <ArrowRight className="size-4 shrink-0 text-white/35 transition-colors group-hover:text-white/75" />
                  </div>
                  <span className="mt-2 block text-sm leading-6 text-white/50">
                    How agent evaluation differs from {competitor.name}’s{" "}
                    {competitor.tag}.
                  </span>
                </Link>
              ))}
            </div>
          </div>
        </section>

        <section className="border-t border-white/[0.06] px-6 py-16 sm:px-12 sm:py-24">
          <div className="mx-auto max-w-[960px]">
            <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
              FAQ
            </p>
            <h2 className="mt-4 text-3xl font-semibold tracking-tight text-white sm:text-4xl">
              Agent evaluation vs prompt evaluation
            </h2>
            <div className="mt-10 divide-y divide-white/[0.08] border-y border-white/[0.08]">
              {faqItems.map((item) => (
                <section key={item.question} className="py-6">
                  <h3 className="text-lg font-semibold tracking-tight text-white">
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
                Run your first eval
                <ArrowRight className="size-4" />
              </Link>
              <Link
                href="/docs/getting-started/quickstart"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
              >
                Read quickstart
              </Link>
            </div>
          </div>
        </section>
      </CompareShell>
    </>
  );
}
