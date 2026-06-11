import type { Metadata } from "next";
import Link from "next/link";
import {
  JsonLd,
  breadcrumbSchema,
} from "@/components/marketing/json-ld";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { ResourceLeadForm } from "@/components/marketing/resource-lead-form";
import { PRIMARY_RESOURCE, RESOURCE_LIBRARY } from "@/lib/resource-library";
import { ogImageUrl } from "@/lib/seo";

const PAGE_PATH = "/resources/eval-checklist";
const PAGE_TITLE =
  "Enterprise AI Agent Eval Checklist PDF | AgentClash Resources";
const PAGE_DESCRIPTION =
  "Download clean AI agent evaluation PDFs for enterprise release gates, pilots, procurement, and the first 30 days of agent eval adoption.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "Enterprise AI Agent Eval Checklist",
  subtitle: "Free PDFs for release gates, pilots, and procurement",
  kind: "Resource",
});

const proofPoints = [
  "12-point production readiness checklist",
  "Release gate worksheet for CI policy",
  "Pilot scorecard for model and vendor comparison",
  "Procurement questions for platform and security teams",
  "30-day rollout plan for the first governed eval program",
];

const searchIntents = [
  "AI agent evaluation checklist",
  "AI agent release gate template",
  "AI agent pilot scorecard",
  "agent procurement questions",
  "AI agent evaluation framework PDF",
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

const eyebrowClass =
  "font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-white/40";

export default function EvalChecklistResourcePage() {
  return (
    <MarketingShell>
      <JsonLd
        id="agentclash-eval-checklist-resource-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "Resources", url: PAGE_PATH },
            { name: "Enterprise AI Agent Eval Checklist", url: PAGE_PATH },
          ]),
        ]}
      />
      <section className="px-6 pt-20 sm:px-12 sm:pt-28">
        <div className="mx-auto grid max-w-[1120px] gap-12 lg:grid-cols-[1fr_440px] lg:gap-16">
          <div>
            <nav className="flex items-center gap-2 text-xs text-white/35">
              <Link href="/" className="transition-colors hover:text-white/70">
                Home
              </Link>
              <span aria-hidden>/</span>
              <span>Resources</span>
            </nav>
            <p className={`mt-12 ${eyebrowClass}`}>Free PDF resource pack</p>
            <h1 className="mt-6 max-w-[15ch] text-[clamp(2.5rem,6vw,4.75rem)] font-sans font-semibold leading-[1.04] tracking-tight text-white">
              Enterprise AI agent eval checklist
            </h1>
            <p className="mt-7 max-w-[58ch] text-lg leading-8 text-white/60">
              Download the flagship checklist plus companion handbooks for
              release gates, pilots, procurement, and first-program rollout.
              Built for teams searching for practical AI agent evaluation tips
              they can use in a real committee meeting.
            </p>
            <ul className="mt-10 grid gap-3 border-t border-white/[0.08] pt-7 text-sm leading-6 text-white/65 sm:grid-cols-2">
              {proofPoints.map((point) => (
                <li key={point}>{point}</li>
              ))}
            </ul>
          </div>

          <div className="lg:pt-24">
            <div className="rounded-xl border border-white/[0.1] bg-white/[0.03] p-6 shadow-2xl shadow-black/30">
              <p className={eyebrowClass}>Instant download</p>
              <h2 className="mt-4 text-2xl font-sans font-semibold tracking-tight text-white">
                Get all {RESOURCE_LIBRARY.length} PDFs
              </h2>
              <p className="mt-3 text-sm leading-6 text-white/50">
                The thank-you page includes every PDF link and routes you to an
                eval architecture review when you are ready to scope a pilot.
              </p>
              <ResourceLeadForm
                className="mt-6"
                source="resources-eval-checklist"
                resource={PRIMARY_RESOURCE.slug}
              />
            </div>
          </div>
        </div>
      </section>

      <section className="px-6 pt-24 sm:px-12 sm:pt-32">
        <div className="mx-auto max-w-[1120px]">
          <p className={eyebrowClass}>Included downloads</p>
          <h2 className="mt-4 max-w-[18ch] text-3xl font-sans font-semibold tracking-tight text-white sm:text-4xl">
            Practical guides, not generic AI advice
          </h2>
          <div className="mt-10 grid gap-px overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.08] md:grid-cols-2">
            {RESOURCE_LIBRARY.map((resource) => (
              <article
                key={resource.slug}
                className="bg-[#060606] p-6 transition-colors hover:bg-white/[0.025]"
              >
                <p className={eyebrowClass}>{resource.kicker}</p>
                <h3 className="mt-3 text-xl font-sans font-semibold tracking-tight text-white">
                  {resource.title}
                </h3>
                <p className="mt-3 text-sm leading-6 text-white/50">
                  {resource.description}
                </p>
                <dl className="mt-5 grid gap-3 text-xs text-white/40 sm:grid-cols-2">
                  <div>
                    <dt className="text-white/25">Audience</dt>
                    <dd className="mt-1 text-white/55">{resource.audience}</dd>
                  </div>
                  <div>
                    <dt className="text-white/25">Use in</dt>
                    <dd className="mt-1 text-white/55">{resource.readTime}</dd>
                  </div>
                </dl>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className="px-6 py-24 sm:px-12 sm:py-32">
        <div className="mx-auto grid max-w-[1120px] gap-12 border-t border-white/[0.08] pt-12 lg:grid-cols-[0.8fr_1.2fr]">
          <div>
            <p className={eyebrowClass}>Search intent covered</p>
            <h2 className="mt-4 text-3xl font-sans font-semibold tracking-tight text-white">
              Built for the questions teams actually search
            </h2>
          </div>
          <ul className="grid gap-3 sm:grid-cols-2">
            {searchIntents.map((intent) => (
              <li
                key={intent}
                className="rounded-lg border border-white/[0.08] px-4 py-3 text-sm text-white/60"
              >
                {intent}
              </li>
            ))}
          </ul>
        </div>
      </section>
    </MarketingShell>
  );
}
