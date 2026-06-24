import type { Metadata } from "next";
import Link from "next/link";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { RESOURCE_LIBRARY } from "@/lib/resource-library";

export const metadata: Metadata = {
  title: "Download AI Agent Eval PDFs | AgentClash",
  description:
    "Download the AgentClash AI agent evaluation checklist, release gate worksheet, pilot scorecard, procurement questions, and rollout plan.",
  alternates: { canonical: "/resources/eval-checklist/thank-you" },
  robots: { index: false, follow: false },
};

const eyebrowClass =
  "font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.16em] text-white/40";

export default function EvalChecklistThankYouPage() {
  return (
    <MarketingShell showFooter={false}>
      <section className="px-6 py-20 sm:px-12 sm:py-28">
        <div className="mx-auto max-w-[980px]">
          <p className={eyebrowClass}>Resource pack unlocked</p>
          <h1 className="mt-5 max-w-[16ch] text-[clamp(2.4rem,5vw,4.25rem)] font-sans font-semibold leading-[1.05] tracking-tight text-white">
            Download the AI agent eval PDFs
          </h1>
          <p className="mt-6 max-w-[58ch] text-lg leading-8 text-white/60">
            Start with the checklist, then use the worksheets to turn a pilot
            into a baseline, scorecard, and release gate your team can defend.
          </p>

          <div className="mt-12 grid gap-px overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.08] md:grid-cols-2">
            {RESOURCE_LIBRARY.map((resource) => (
              <article key={resource.slug} className="bg-[#060606] p-6">
                <p className={eyebrowClass}>{resource.kicker}</p>
                <h2 className="mt-3 text-xl font-sans font-semibold tracking-tight text-white">
                  {resource.title}
                </h2>
                <p className="mt-3 text-sm leading-6 text-white/50">
                  {resource.description}
                </p>
                <a
                  href={resource.file}
                  download
                  className="mt-5 inline-flex min-h-11 items-center justify-center rounded-lg bg-white px-5 text-sm font-semibold text-[#060606] transition-colors hover:bg-white/90"
                >
                  Download PDF
                </a>
              </article>
            ))}
          </div>

          <div className="mt-12 rounded-xl border border-white/[0.1] bg-white/[0.035] p-7">
            <p className={eyebrowClass}>Next step</p>
            <h2 className="mt-3 text-2xl font-sans font-semibold tracking-tight text-white">
              Want to map this to your own agents?
            </h2>
            <p className="mt-3 max-w-[58ch] text-sm leading-6 text-white/55">
              Book a 30-minute eval architecture review. We will map your
              workloads to eval packs, baselines, scorecards, and a first
              release gate.
            </p>
            <div className="mt-6 flex flex-col gap-3 sm:flex-row">
              <Link
                href="/enterprise"
                className="inline-flex min-h-11 items-center justify-center rounded-lg bg-white px-5 text-sm font-semibold text-[#060606] transition-colors hover:bg-white/90"
              >
                See enterprise rollout
              </Link>
              <a
                href="mailto:hello@agentclash.dev?subject=AgentClash%20eval%20architecture%20review"
                className="inline-flex min-h-11 items-center justify-center rounded-lg border border-white/15 px-5 text-sm font-medium text-white/85 transition-colors hover:border-white/35 hover:text-white"
              >
                Email hello@agentclash.dev
              </a>
            </div>
          </div>
        </div>
      </section>
    </MarketingShell>
  );
}
