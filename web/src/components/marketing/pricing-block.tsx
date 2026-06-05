"use client";

import Link from "next/link";
import { useState } from "react";
import { ArrowRight } from "lucide-react";
import { TiltCard } from "@/app/auth/login/tilt-card";
import { PRICING_TIERS, type Cta, type Period, type Tier } from "@/lib/pricing-data";
import { ShaderLines } from "./shader-lines";

export function PricingBlock() {
  const [period, setPeriod] = useState<Period>("monthly");

  return (
    <section
      id="pricing"
      className="relative isolate border-t border-white/[0.06] py-32 sm:py-44 overflow-hidden"
    >
      <div className="relative px-6 sm:px-12">
        <div className="mx-auto max-w-[1440px]">
          <div className="text-center">
            <h2 className="text-3xl sm:text-5xl font-semibold tracking-tight text-white">
              Free for 45 days.
            </h2>
            <p className="mt-4 mx-auto max-w-[44ch] text-sm leading-6 text-white/75">
              No credit card. Self-host the engine for free, or skip the ops
              with hosted.
            </p>

            <div className="mt-10 flex justify-center">
              <PeriodToggle period={period} onChange={setPeriod} />
            </div>
          </div>

          <div className="mt-14 grid gap-5 md:grid-cols-2 lg:grid-cols-4">
            {PRICING_TIERS.map((tier) => (
              <TierCard key={tier.name} tier={tier} period={period} />
            ))}
          </div>

          <p className="mt-10 mx-auto max-w-[64ch] text-center text-sm leading-6 text-white/60">
            BYOK on every tier — we never mark up tokens. Race quota pools at
            the workspace level.
          </p>
        </div>
      </div>
    </section>
  );
}

function PeriodToggle({
  period,
  onChange,
}: {
  period: Period;
  onChange: (p: Period) => void;
}) {
  return (
    <div
      className="inline-flex items-center rounded-full border border-white/10 bg-white/[0.04] p-1 backdrop-blur"
      role="group"
      aria-label="Billing period"
    >
      <ToggleButton
        active={period === "monthly"}
        onClick={() => onChange("monthly")}
      >
        Monthly
      </ToggleButton>
      <ToggleButton
        active={period === "yearly"}
        onClick={() => onChange("yearly")}
      >
        <span>Yearly</span>
        <span
          className={`rounded-full px-1.5 py-0.5 text-[10px] font-semibold tracking-tight ${
            period === "yearly"
              ? "bg-[#060606]/15 text-[#060606]"
              : "bg-emerald-400/15 text-emerald-300"
          }`}
        >
          -20%
        </span>
      </ToggleButton>
    </div>
  );
}

function ToggleButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`inline-flex items-center gap-2 rounded-full px-4 py-1.5 text-xs font-medium transition-colors ${
        active ? "bg-white text-[#060606]" : "text-white/65 hover:text-white"
      }`}
    >
      {children}
    </button>
  );
}

function TierCard({ tier, period }: { tier: Tier; period: Period }) {
  const price = tier.prices[period];

  // The strip sits behind only the tier name + blurb so price and CTA stay
  // on a clean glass surface. The mask fades most of the way down so the
  // streaks read as a soft colored glow that dissolves into the card body.
  const shaderMask =
    "linear-gradient(to bottom, black 0%, rgba(0,0,0,0.7) 40%, transparent 100%)";

  return (
    <TiltCard className="h-full">
      <div
        className="glass-card glass-shine relative isolate h-full rounded-2xl"
        style={{ backgroundColor: "rgba(255, 255, 255, 0.06)" }}
      >
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 z-0 h-32 sm:h-36 overflow-hidden rounded-t-2xl"
          style={{
            maskImage: shaderMask,
            WebkitMaskImage: shaderMask,
          }}
        >
          <ShaderLines
            colorA={tier.shader.colorA}
            colorB={tier.shader.colorB}
            colorIntensity={0.3}
            animationSpeed={0.035}
            mosaicScale={{ x: 10, y: 5 }}
            backgroundColor="#0a0a0a"
          />
        </div>

        <div className="relative z-10 flex h-full flex-col p-6 sm:p-7">
          <h3 className="text-2xl font-semibold text-white">{tier.name}</h3>
          <p className="mt-2 text-sm leading-6 text-white/75">{tier.blurb}</p>

          <div className="mt-6 flex items-baseline gap-2">
            <span className="text-4xl font-semibold tracking-tight text-white">
              {price.value}
            </span>
            {price.suffix && (
              <span className="text-sm text-white/55">{price.suffix}</span>
            )}
          </div>
          <div className="mt-1 min-h-[1.25rem] text-xs text-white/55">
            {price.note ?? " "}
          </div>

          <CtaButton cta={tier.cta} />

          <div className="my-6 h-px bg-white/10" />

          <ul className="flex flex-col gap-2.5 text-[14px] leading-6 text-white/85">
            {tier.features.map((feature) => (
              <li key={feature} className="flex items-start gap-2.5">
                <span
                  aria-hidden
                  className="select-none text-white/45 leading-6"
                >
                  —
                </span>
                <span>{feature}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </TiltCard>
  );
}

function CtaButton({ cta }: { cta: Cta }) {
  const base =
    "inline-flex w-full items-center justify-center gap-2 rounded-md px-5 py-2.5 text-sm font-medium transition-colors";
  const primary = "bg-white text-[#060606] hover:bg-white/90";
  const secondary =
    "border border-white/15 bg-white/[0.04] text-white/85 hover:text-white hover:border-white/25";
  const className = `${base} ${cta.primary ? primary : secondary}`;

  const inner = (
    <>
      {cta.label}
      <ArrowRight className="size-3.5" />
    </>
  );

  return (
    <div className="mt-6 flex flex-col">
      {cta.external || cta.href.startsWith("mailto:") ? (
        <a
          href={cta.href}
          target={cta.external ? "_blank" : undefined}
          rel={cta.external ? "noopener noreferrer" : undefined}
          className={className}
        >
          {inner}
        </a>
      ) : (
        <Link href={cta.href} className={className}>
          {inner}
        </Link>
      )}
      {cta.sublabel && (
        <p className="mt-2 text-center text-[11px] text-white/40">
          {cta.sublabel}
        </p>
      )}
    </div>
  );
}
