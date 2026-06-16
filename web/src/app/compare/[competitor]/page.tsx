import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowRight, CheckCircle2 } from "lucide-react";
import {
  JsonLd,
  breadcrumbSchema,
  faqSchema,
  productSchema,
} from "@/components/marketing/json-ld";
import {
  COMPETITORS,
  MARK_LABEL,
  competitorFaq,
  competitorRows,
  getCompetitorBySlug,
} from "@/lib/comparison-data";
import { ogImageUrl } from "@/lib/seo";
import { AGENT_EVALUATION_FEATURES } from "@/lib/seo-features";
import { CompareShell } from "../_components/compare-shell";

type Props = {
  params: Promise<{ competitor: string }>;
};

// Only the known competitor slugs are valid; anything else 404s.
export const dynamicParams = false;

export function generateStaticParams() {
  return COMPETITORS.map((competitor) => ({ competitor: competitor.slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { competitor: slug } = await params;
  const competitor = getCompetitorBySlug(slug);
  if (!competitor) return {};

  const title = `AgentClash vs ${competitor.name}: Agent Evaluation vs Prompt Evaluation`;
  const description = `How AgentClash compares to ${competitor.name}. ${competitor.name} is a ${competitor.tag} tool; AgentClash runs tool-using agents on the same task in a sandbox, scores the full trajectory, and gates CI on regressions.`;
  const path = `/compare/${competitor.slug}`;
  const image = ogImageUrl({
    title: `AgentClash vs ${competitor.name}`,
    subtitle: "Agent eval vs prompt eval",
    kind: "Compare",
  });

  return {
    title,
    description,
    alternates: { canonical: path },
    openGraph: {
      title,
      description,
      url: path,
      type: "website",
      locale: "en_US",
      siteName: "AgentClash",
      images: [{ url: image, width: 1200, height: 630, alt: title }],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
      images: [{ url: image, alt: title }],
    },
  };
}

export default async function CompareCompetitorPage({ params }: Props) {
  const { competitor: slug } = await params;
  const competitor = getCompetitorBySlug(slug);
  if (!competitor) notFound();

  const path = `/compare/${competitor.slug}`;
  const rows = competitorRows(competitor);
  const faq = competitorFaq(competitor);

  return (
    <>
      <JsonLd
        id={`agentclash-compare-${competitor.slug}-schema`}
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "Compare", url: "/compare" },
            { name: `AgentClash vs ${competitor.name}`, url: path },
          ]),
          faqSchema(faq),
          productSchema({
            name: "AgentClash",
            description: `AgentClash is an agent-evaluation engine — an alternative to ${competitor.name} for teams evaluating tool-using agents on the same real tasks in a sandbox.`,
            url: path,
            applicationSubCategory: "AI agent evaluation platform",
            featureList: AGENT_EVALUATION_FEATURES,
          }),
        ]}
      />
      <CompareShell>
        <section className="px-6 py-20 sm:px-12 sm:py-24">
          <div className="mx-auto max-w-[1080px]">
            <nav className="flex items-center gap-2 text-xs text-white/35">
              <Link href="/" className="transition-colors hover:text-white/70">
                Home
              </Link>
              <span>/</span>
              <Link
                href="/compare"
                className="transition-colors hover:text-white/70"
              >
                Compare
              </Link>
              <span>/</span>
              <span>AgentClash vs {competitor.name}</span>
            </nav>
            <p className="mt-10 font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-cyan-200/70">
              agent eval vs {competitor.tag}
            </p>
            <h1 className="mt-5 font-[family-name:var(--font-display)] text-4xl font-normal leading-[1.05] tracking-tight text-white sm:text-6xl">
              AgentClash vs {competitor.name}
            </h1>
            <p className="mt-8 max-w-[64ch] text-base leading-8 text-white/62 sm:text-lg">
              {competitor.name} is excellent at {competitor.tag}. AgentClash is
              built for agent evaluation: it runs tool-using agents
              on the same task in a fresh sandbox, scores the whole trajectory, and
              turns failures into CI regression gates.
            </p>
          </div>
        </section>

        <section className="border-y border-white/[0.06] px-6 py-16 sm:px-12">
          <div className="mx-auto max-w-[1080px]">
            <h2 className="text-2xl font-semibold tracking-tight text-white sm:text-3xl">
              AgentClash vs {competitor.name}, capability by capability
            </h2>
            <div className="mt-10 overflow-x-auto">
              <table className="w-full min-w-[640px] border-collapse text-left">
                <thead>
                  <tr className="border-b border-white/[0.14]">
                    <th
                      scope="col"
                      className="py-4 pr-4 text-2xs font-medium uppercase tracking-[0.16em] text-white/40"
                    >
                      Capability
                    </th>
                    <th
                      scope="col"
                      className="px-3 py-4 text-center text-sm font-semibold text-white"
                    >
                      AgentClash
                    </th>
                    <th
                      scope="col"
                      className="px-3 py-4 text-center text-sm font-semibold text-white/55"
                    >
                      {competitor.name}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row) => (
                    <tr
                      key={row.label}
                      className="border-b border-white/[0.06] align-top"
                    >
                      <th scope="row" className="py-5 pr-6 font-normal">
                        <span className="block text-base text-white/85">
                          {row.label}
                        </span>
                        <span className="mt-1 block max-w-[52ch] text-xs leading-5 text-white/40">
                          {row.sub}
                        </span>
                      </th>
                      <td className="px-3 py-5 text-center text-sm font-medium text-white">
                        {MARK_LABEL[row.agentclash]}
                      </td>
                      <td className="px-3 py-5 text-center text-sm text-white/55">
                        {MARK_LABEL[row.competitor]}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <section className="px-6 py-16 sm:px-12 sm:py-20">
          <div className="mx-auto grid max-w-[1080px] gap-6 lg:grid-cols-2">
            <div className="rounded-md border border-white/[0.08] bg-white/[0.03] p-6">
              <h2 className="text-lg font-semibold tracking-tight text-white">
                Where {competitor.name} is the better fit
              </h2>
              <p className="mt-4 text-sm leading-7 text-white/60">
                {competitor.whereItFits}
              </p>
            </div>
            <div className="rounded-md border border-white/[0.08] bg-white/[0.03] p-6">
              <h2 className="text-lg font-semibold tracking-tight text-white">
                Where AgentClash is the better fit
              </h2>
              <ul className="mt-4 space-y-3">
                {AGENT_EVALUATION_FEATURES.map((feature) => (
                  <li
                    key={feature}
                    className="flex items-start gap-3 text-sm leading-6 text-white/70"
                  >
                    <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-200" />
                    {feature}
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </section>

        <section className="border-t border-white/[0.06] px-6 py-16 sm:px-12 sm:py-24">
          <div className="mx-auto max-w-[960px]">
            <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-white/35">
              FAQ
            </p>
            <h2 className="mt-4 text-3xl font-semibold tracking-tight text-white sm:text-4xl">
              AgentClash vs {competitor.name}
            </h2>
            <div className="mt-10 divide-y divide-white/[0.08] border-y border-white/[0.08]">
              {faq.map((item) => (
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
                href="/compare"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
              >
                See all comparisons
              </Link>
            </div>
          </div>
        </section>
      </CompareShell>
    </>
  );
}
