import type { Metadata } from "next";
import Link from "next/link";
import { CalEmbedInit } from "@/components/marketing/cal-embed-init";
import { EnterprisePageCTA } from "@/components/marketing/enterprise-page-cta";
import { FAQBlock } from "@/components/marketing/faq-block";
import {
  JsonLd,
  breadcrumbSchema,
  productSchema,
} from "@/components/marketing/json-ld";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { ogImageUrl } from "@/lib/seo";

const PAGE_PATH = "/services";
const PAGE_TITLE = "Agent Evaluation Services — Fixed Offerings | AgentClash";
const PAGE_DESCRIPTION =
  "Fixed-scope eval services that end in customer-owned challenge packs, baselines, and CI gates. First governed benchmark in 2 weeks on the AgentClash platform.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "Agent Evaluation Services",
  subtitle: "Platform adoption, not generic consulting",
  kind: "Services",
});

const offerings = [
  {
    id: "eval-discovery",
    name: "Eval Discovery",
    duration: "1 week",
    deliverable:
      "Audit your agents, document 5 concrete failure modes, and deliver a prioritized challenge pack roadmap.",
    fit: "You have agents in production but no frozen benchmark yet.",
    emailSubject: "AgentClash%20Eval%20Discovery",
  },
  {
    id: "challenge-pack-build",
    name: "Challenge Pack Build",
    duration: "2 to 4 weeks",
    deliverable:
      "Ship 3–10 custom challenge packs from your real workflows, scored and versioned in your workspace.",
    fit: "You know what to test and need packs built from live tasks and tools.",
    emailSubject: "AgentClash%20Challenge%20Pack%20Build",
  },
  {
    id: "benchmark-gate-setup",
    name: "Benchmark & Gate Setup",
    duration: "2 weeks",
    deliverable:
      "Run a baseline, wire a CI release gate, and hand off an executive scorecard template your committee can defend.",
    fit: "You have packs and need a governed release decision in CI.",
    emailSubject: "AgentClash%20Benchmark%20%26%20Gate%20Setup",
  },
  {
    id: "managed-eval-retainer",
    name: "Managed Eval Retainer",
    duration: "Monthly",
    deliverable:
      "Release benchmarks on every ship candidate plus a monthly reliability report with regression trends.",
    fit: "You ship agents often and want ongoing benchmark coverage without building an eval team.",
    emailSubject: "AgentClash%20Managed%20Eval%20Retainer",
  },
] as const;

const guardrails = [
  "Every engagement produces artifacts in your AgentClash workspace: packs, baselines, gates, or CI handoff.",
  "We do not run black-box evals outside your tenancy. You own the packs and the evidence.",
  "Services are paid engagements. The free 45-day Team pilot is still the default path if you want to self-serve first.",
  "No vague SOWs. Each package above has a fixed duration and named deliverable.",
];

const intakeFields = [
  "Agent workflow and tools in scope",
  "Recent failure examples or incident tickets",
  "Compliance, residency, or policy constraints",
  "Target release decision and stakeholders",
  "Current eval or observability tooling",
  "Success criteria for the first governed benchmark",
];

const crossLinks = [
  {
    href: "/enterprise",
    label: "Enterprise pilot",
    description: "45-day Team pilot with no credit card. Self-serve product access first.",
  },
  {
    href: "/pricing",
    label: "Product pricing",
    description: "Free, Pro, Team, and Enterprise tiers. BYOK on every plan.",
  },
  {
    href: "/platform/agent-evaluation",
    label: "Evaluation platform",
    description: "Same-tools races, sandbox execution, replay, and scorecards.",
  },
  {
    href: "/docs",
    label: "Documentation",
    description: "Challenge packs, CI gates, and self-host guides.",
  },
];

const faqItems = [
  {
    question: "How is this different from the Team pilot?",
    answer:
      "The Team pilot is product access in your workspace. Services are fixed-scope engagements where our team builds packs, baselines, or gates with you. Many teams start the pilot and add a 2-week Benchmark & Gate Setup sprint.",
  },
  {
    question: "Do we keep the challenge packs after the engagement?",
    answer:
      "Yes. All packs, baselines, scorecards, and gate configs live in your workspace. You can extend them without us.",
  },
  {
    question: "Can we self-host instead of hosted AgentClash?",
    answer:
      "Yes. AgentClash is MIT-licensed. We can deliver packs and gate templates against your self-hosted stack. Discuss deployment during discovery.",
  },
  {
    question: "What do you need before discovery?",
    answer:
      "A short description of the agent workflow, one or two real failure examples, and who signs the release decision. We handle the rest on the first call.",
  },
  {
    question: "Are services free?",
    answer:
      "No. The four packages above are paid, fixed-scope engagements. The discovery call is free. Product access starts free on the Team pilot and paid tiers on the pricing page.",
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

const eyebrowClass =
  "font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.16em] text-white/40";

export default function ServicesPage() {
  return (
    <MarketingShell>
      <CalEmbedInit />
      <JsonLd
        id="agentclash-services-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "Services", url: PAGE_PATH },
          ]),
          productSchema({
            name: "AgentClash Evaluation Services",
            description: PAGE_DESCRIPTION,
            url: `https://www.agentclash.dev${PAGE_PATH}`,
            applicationSubCategory: "AI agent evaluation services",
          }),
        ]}
      />

      <section className="px-6 pt-20 sm:px-12 sm:pt-32">
        <div className="mx-auto max-w-[960px]">
          <nav className="flex items-center gap-2 text-xs text-white/35">
            <Link href="/" className="transition-colors hover:text-white/70">
              Home
            </Link>
            <span aria-hidden>/</span>
            <span>Services</span>
          </nav>
          <p className={`mt-12 ${eyebrowClass}`}>Eval program</p>
          <h1 className="mt-6 max-w-[18ch] text-[clamp(2.25rem,5.5vw,4rem)] font-sans font-semibold leading-[1.06] tracking-[-0.03em] text-white">
            First governed benchmark in 2 weeks
          </h1>
          <p className="mt-8 max-w-[58ch] text-lg leading-8 text-white/60">
            AgentClash is the platform. Our team gets you from live agents to
            frozen challenge packs, baseline evidence, and CI gates in your
            workspace. Fixed offerings, not open-ended consulting.
          </p>
          <p className="mt-4 max-w-[58ch] text-sm leading-6 text-white/45">
            These packages are paid engagements, scoped after discovery. The{" "}
            <Link
              href="/enterprise"
              className="text-white/65 underline decoration-white/20 underline-offset-4 transition-colors hover:text-white/85"
            >
              Team pilot
            </Link>{" "}
            is free self-serve product access if you want to start on your own.
          </p>
          <EnterprisePageCTA className="mt-10" />
        </div>
      </section>

      <section className="px-6 pt-28 sm:px-12 sm:pt-40">
        <div className="mx-auto max-w-[1080px]">
          <p className={eyebrowClass}>Offerings</p>
          <h2 className="mt-4 max-w-[20ch] text-3xl font-sans font-semibold tracking-[-0.02em] text-white sm:text-[2.5rem] sm:leading-[1.1]">
            Four fixed packages
          </h2>
          <p className="mt-5 max-w-[56ch] text-base leading-7 text-white/55">
            Each package is a paid, fixed-scope engagement that ends with
            customer-owned artifacts in your workspace. We quote scope on
            discovery; no public rate card.
          </p>
          <ul className="mt-14 grid gap-px overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.08] sm:grid-cols-2">
            {offerings.map((offering) => (
              <li key={offering.id} className="bg-[#060606]">
                <div className="flex h-full flex-col px-6 py-7 sm:px-7">
                  <div className="flex flex-wrap items-baseline justify-between gap-3">
                    <h3 className="text-lg font-sans font-semibold tracking-[-0.01em] text-white">
                      {offering.name}
                    </h3>
                    <span className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.12em] text-white/40">
                      {offering.duration}
                    </span>
                  </div>
                  <p className="mt-4 text-base leading-7 text-white/65">
                    {offering.deliverable}
                  </p>
                  <p className="mt-4 text-sm leading-6 text-white/45">
                    Best when: {offering.fit}
                  </p>
                  <a
                    href={`mailto:hello@agentclash.dev?subject=${offering.emailSubject}`}
                    className="mt-6 inline-flex text-sm font-medium text-white/70 underline decoration-white/20 underline-offset-4 transition-colors hover:text-white hover:decoration-white/50"
                  >
                    Email about {offering.name}
                  </a>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="px-6 pt-28 sm:px-12 sm:pt-40">
        <div className="mx-auto max-w-[960px]">
          <p className={eyebrowClass}>Guardrails</p>
          <h2 className="mt-4 max-w-[22ch] text-3xl font-sans font-semibold tracking-[-0.02em] text-white sm:text-[2.25rem] sm:leading-[1.1]">
            Platform adoption, not a consulting shop
          </h2>
          <ul className="mt-10 divide-y divide-white/[0.08] border-t border-white/[0.08]">
            {guardrails.map((item) => (
              <li key={item} className="py-5 text-base leading-7 text-white/60">
                {item}
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="px-6 pt-28 sm:px-12 sm:pt-40">
        <div className="mx-auto max-w-[960px]">
          <p className={eyebrowClass}>Discovery intake</p>
          <h2 className="mt-4 max-w-[20ch] text-3xl font-sans font-semibold tracking-[-0.02em] text-white sm:text-[2.25rem] sm:leading-[1.1]">
            What we capture on the first call
          </h2>
          <p className="mt-5 max-w-[56ch] text-base leading-7 text-white/55">
            A free 30-minute discovery maps your agents to the right package and
            quotes scope. Bring what you have; we structure the benchmark plan.
          </p>
          <ul className="mt-10 grid gap-3 sm:grid-cols-2">
            {intakeFields.map((field) => (
              <li
                key={field}
                className="rounded-lg border border-white/[0.08] bg-white/[0.02] px-4 py-3.5 text-sm leading-6 text-white/70"
              >
                {field}
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="px-6 pt-28 sm:px-12 sm:pt-40">
        <div className="mx-auto max-w-[1080px]">
          <p className={eyebrowClass}>Explore</p>
          <h2 className="mt-4 text-2xl font-sans font-semibold tracking-[-0.02em] text-white sm:text-3xl">
            Product paths
          </h2>
          <ul className="mt-10 grid gap-px overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.08] sm:grid-cols-2">
            {crossLinks.map((link) => (
              <li key={link.href}>
                <Link
                  href={link.href}
                  className="flex h-full flex-col bg-[#060606] px-6 py-6 transition-colors hover:bg-white/[0.025]"
                >
                  <span className="text-base font-sans font-semibold text-white">
                    {link.label}
                  </span>
                  <span className="mt-2 text-sm leading-6 text-white/45">
                    {link.description}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </section>

      <div className="mt-28 sm:mt-40">
        <FAQBlock
          schemaId="agentclash-services-faq-schema"
          eyebrow="FAQ"
          title="Eval services questions"
          items={faqItems}
          sansHeadlines
        />
      </div>

      <section className="border-t border-white/[0.06] px-6 py-28 sm:px-12 sm:py-40">
        <div className="mx-auto max-w-[960px]">
          <h2 className="max-w-[18ch] text-[clamp(2rem,4.5vw,3.25rem)] font-sans font-semibold leading-[1.08] tracking-[-0.03em] text-white">
            Book a discovery call
          </h2>
          <p className="mt-6 max-w-[52ch] text-lg leading-8 text-white/55">
            Tell us about your agents and release process. We will recommend a
            package and timeline, or point you to the{" "}
            <Link
              href="/enterprise"
              className="text-white/80 underline decoration-white/25 underline-offset-4 transition-colors hover:text-white"
            >
              Team pilot
            </Link>{" "}
            if that is the better first step.
          </p>
          <EnterprisePageCTA className="mt-10" />
        </div>
      </section>
    </MarketingShell>
  );
}
