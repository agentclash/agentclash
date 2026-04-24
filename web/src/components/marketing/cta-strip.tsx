import Link from "next/link";
import { ArrowRight, Star } from "lucide-react";
import { DemoButton } from "./demo-button";

type Variant = "demo-first" | "cli-first" | "github-first";

type Props = {
  variant?: Variant;
  primaryLabel?: string;
  primaryHref?: string;
  secondaryLabel?: string;
  secondaryHref?: string;
  showGithub?: boolean;
};

export function CTAStrip({
  variant = "demo-first",
  primaryLabel,
  primaryHref,
  secondaryLabel,
  secondaryHref,
  showGithub = true,
}: Props) {
  const primaryCTA =
    variant === "demo-first" ? (
      <DemoButton />
    ) : primaryHref ? (
      <Link
        href={primaryHref}
        className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
      >
        {primaryLabel ?? "Get started"}
        <ArrowRight className="size-4" />
      </Link>
    ) : null;

  const secondaryCTA = secondaryHref ? (
    <Link
      href={secondaryHref}
      className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
    >
      {secondaryLabel ?? "Learn more"}
      <ArrowRight className="size-4" />
    </Link>
  ) : variant === "demo-first" ? (
    <Link
      href="/auth/login"
      className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
    >
      Get started
      <ArrowRight className="size-4" />
    </Link>
  ) : null;

  return (
    <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
      {primaryCTA}
      {secondaryCTA}
      {showGithub ? (
        <a
          href="https://github.com/agentclash/agentclash"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center justify-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.02] px-6 py-3 text-sm font-medium text-white/60 hover:text-white/90 hover:border-white/20 transition-colors"
        >
          <Star className="size-4" />
          GitHub
        </a>
      ) : null}
    </div>
  );
}

export function CLIInstallStrip({
  command = "npm i -g agentclash",
  learnMoreHref = "/v2/oss",
  learnMoreLabel = "self-host",
}: {
  command?: string;
  learnMoreHref?: string;
  learnMoreLabel?: string;
}) {
  return (
    <div className="inline-flex items-center gap-3 rounded-md border border-white/[0.06] bg-white/[0.02] px-4 py-2.5 font-[family-name:var(--font-mono)] text-[12px] text-white/55">
      <span className="text-white/30 select-none">$</span>
      <code className="text-white/85">{command}</code>
      <span className="text-white/20">·</span>
      <Link
        href={learnMoreHref}
        className="text-white/45 hover:text-white/80 transition-colors"
      >
        {learnMoreLabel}
      </Link>
    </div>
  );
}
