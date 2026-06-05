import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import {
  JsonLd,
  breadcrumbSchema,
  faqSchema,
  pricingSchema,
} from "@/components/marketing/json-ld";
import { PricingBlock } from "@/components/marketing/pricing-block";
import { ogImageUrl } from "@/lib/seo";
import { CompareShell } from "../compare/_components/compare-shell";

const PAGE_PATH = "/pricing";
const PAGE_TITLE = "AgentClash Pricing: Free, Open-Source, Pro, Team & Enterprise";
const PAGE_DESCRIPTION =
  "AgentClash pricing — a free hosted tier and free open-source self-hosting, Pro at $49/seat/mo and Team at $100/seat/mo (cheaper billed annually), and custom Enterprise. Bring your own LLM keys on every tier; we never mark up tokens.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "AgentClash Pricing",
  subtitle: "Free and open-source, with hosted Pro, Team & Enterprise",
  kind: "Pricing",
});

// Visible + FAQPage JSON-LD. Every answer is grounded in the published tier copy
// (see pricing-data.ts) — no claims beyond what the tiers actually offer.
const faqItems = [
  {
    question: "Is AgentClash free?",
    answer:
      "Yes. AgentClash is open source under the MIT license, so you can self-host the engine on your own infrastructure for free. There is also a free hosted tier with 25 races per month, one seat, and bring-your-own LLM keys — no credit card required.",
  },
  {
    question: "How much does AgentClash cost?",
    answer:
      "Self-hosting is free. The hosted Free tier is $0. Pro is $49 per seat per month ($39 billed annually) with a five-seat minimum, Team is $100 per seat per month ($80 billed annually), and Enterprise is custom. Pro and Team include a free 45-day trial with no credit card.",
  },
  {
    question: "Do you mark up LLM tokens?",
    answer:
      "No. AgentClash is bring-your-own-key (BYOK) on every tier — you connect your own LLM provider keys and pay providers directly. We never mark up tokens. Race quota pools at the workspace level.",
  },
  {
    question: "Can I self-host AgentClash instead of paying?",
    answer:
      "Yes. The engine is MIT-licensed and open source. You can run the full stack on your own infrastructure at no license cost; the paid hosted tiers exist to skip the ops and add team features like private challenge packs, CI integration, SSO, and audit logs.",
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

export default function PricingPage() {
  return (
    <>
      <JsonLd
        id="agentclash-pricing-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "Pricing", url: PAGE_PATH },
          ]),
          pricingSchema(),
          faqSchema(faqItems),
        ]}
      />
      <CompareShell>
        <section className="px-6 pt-20 sm:px-12 sm:pt-28">
          <div className="mx-auto max-w-[1080px]">
            <nav className="flex items-center gap-2 text-xs text-white/35">
              <Link href="/" className="transition-colors hover:text-white/70">
                Home
              </Link>
              <span>/</span>
              <span>Pricing</span>
            </nav>
            <p className="mt-10 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-cyan-200/70">
              Pricing
            </p>
            <h1 className="mt-5 max-w-[20ch] font-[family-name:var(--font-display)] text-4xl font-normal leading-[1.05] tracking-tight text-white sm:text-6xl">
              Free and open-source. Hosted when you want to skip the ops.
            </h1>
            <p className="mt-8 max-w-[64ch] text-base leading-8 text-white/62 sm:text-lg">
              {PAGE_DESCRIPTION}
            </p>
          </div>
        </section>

        {/* Reuses the same PricingBlock rendered on the landing page, so the
            tiers here and there stay identical (single source: pricing-data.ts). */}
        <PricingBlock />

        <section className="border-t border-white/[0.06] px-6 py-16 sm:px-12">
          <div className="mx-auto max-w-[960px]">
            <h2 className="text-2xl font-semibold tracking-tight text-white sm:text-3xl">
              Prefer to self-host?
            </h2>
            <p className="mt-4 max-w-[70ch] text-sm leading-7 text-white/60">
              AgentClash is MIT-licensed and open source. Run the full race
              engine — API server, Temporal worker, sandboxes, scoring — on your
              own infrastructure at no license cost. The hosted tiers exist only
              to save you the ops and add team features. Start from the{" "}
              <Link
                href="/docs/getting-started/quickstart"
                className="text-cyan-200/80 underline-offset-4 transition-colors hover:text-cyan-100 hover:underline"
              >
                quickstart
              </Link>{" "}
              or browse the source on{" "}
              <a
                href="https://github.com/agentclash/agentclash"
                target="_blank"
                rel="noopener noreferrer"
                className="text-cyan-200/80 underline-offset-4 transition-colors hover:text-cyan-100 hover:underline"
              >
                GitHub
              </a>
              .
            </p>
          </div>
        </section>

        <section className="border-t border-white/[0.06] px-6 py-16 sm:px-12 sm:py-24">
          <div className="mx-auto max-w-[960px]">
            <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-white/35">
              FAQ
            </p>
            <h2 className="mt-4 text-3xl font-semibold tracking-tight text-white sm:text-4xl">
              Pricing questions
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
                Start first race
                <ArrowRight className="size-4" />
              </Link>
              <Link
                href="/compare"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
              >
                Compare AgentClash
              </Link>
            </div>
          </div>
        </section>
      </CompareShell>
    </>
  );
}
