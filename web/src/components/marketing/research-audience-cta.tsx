"use client";

import { CalEmbedInit } from "./cal-embed-init";
import { CTAStrip } from "./cta-strip";

const DEFAULT_SECONDARY_HREF = "/enterprise";
const DEFAULT_SECONDARY_LABEL = "Enterprise";
const DEFAULT_DEMO_LABEL = "Book eval workshop";

type Props = {
  headline?: string;
  body?: string;
  demoLabel?: string;
  secondaryHref?: string;
  secondaryLabel?: string;
  showGithub?: boolean;
  className?: string;
};

export function ResearchAudienceCTA({
  headline = "Evaluating agents for your team?",
  body = "Book a 30-minute eval architecture review. We'll map your workflows to eval packs, release gates, and a pilot plan.",
  demoLabel = DEFAULT_DEMO_LABEL,
  secondaryHref = DEFAULT_SECONDARY_HREF,
  secondaryLabel = DEFAULT_SECONDARY_LABEL,
  showGithub = false,
  className = "",
}: Props) {
  return (
    <aside
      className={`rounded-lg border border-white/[0.08] bg-white/[0.03] px-5 py-6 sm:px-6 ${className}`}
    >
      <CalEmbedInit />
      {headline ? (
        <h2 className="text-base font-medium text-white">{headline}</h2>
      ) : null}
      {body ? (
        <p className="mt-2 text-sm leading-relaxed text-white/50">{body}</p>
      ) : null}
      <div className="mt-5">
        <CTAStrip
          variant="demo-first"
          demoLabel={demoLabel}
          secondaryHref={secondaryHref}
          secondaryLabel={secondaryLabel}
          showGithub={showGithub}
        />
      </div>
    </aside>
  );
}

export function BenchmarkRunCTA({ className }: { className?: string }) {
  return (
    <ResearchAudienceCTA
      headline="Run this benchmark on your agents"
      body="We'll set up the same eval pack on your workloads, baseline your current agent, and deliver a scorecard your team can gate on."
      className={className}
    />
  );
}
