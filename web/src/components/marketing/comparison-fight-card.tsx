"use client";

import { useState } from "react";

type Matchup = {
  name: string;
  tag: string;
  claim: string;
  limit: string;
  counter: string;
  edge: string;
};

const OPPONENTS: Matchup[] = [
  {
    name: "Braintrust",
    tag: "prompt eval",
    claim: "Grades text.",
    limit: "Stops where agents start.",
    counter: "Grades the whole loop.",
    edge: "Multi-turn, sandboxed, scored on the path — not just the answer.",
  },
  {
    name: "LangSmith",
    tag: "prompt tracing",
    claim: "Traces LangChain.",
    limit: "One ecosystem. One agent at a time.",
    counter: "Races any provider.",
    edge: "Six adapters, head-to-head, on a shared budget.",
  },
  {
    name: "Langfuse",
    tag: "open-source observability",
    claim: "Watches your LLM.",
    limit: "No execution. No verdict.",
    counter: "Runs the race.",
    edge: "Observability is a side effect. The verdict is the point.",
  },
  {
    name: "Arize Phoenix",
    tag: "LLM observability",
    claim: "Monitors what shipped.",
    limit: "Tells you it broke. After.",
    counter: "Decides what ships.",
    edge: "Picked on the real task before it ever reaches prod.",
  },
];

export function ComparisonFightCard() {
  const [activeIndex, setActiveIndex] = useState(0);
  const m = OPPONENTS[activeIndex];

  const goPrev = () =>
    setActiveIndex((i) => (i - 1 + OPPONENTS.length) % OPPONENTS.length);
  const goNext = () => setActiveIndex((i) => (i + 1) % OPPONENTS.length);

  return (
    <div>
      {/* Tab strip — editorial underline */}
      <div
        role="tablist"
        aria-label="Choose competitor"
        className="flex flex-wrap items-baseline gap-x-7 gap-y-4 border-b border-white/[0.08] sm:gap-x-10"
      >
        {OPPONENTS.map((o, i) => {
          const active = i === activeIndex;
          return (
            <button
              key={o.name}
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => setActiveIndex(i)}
              className={`group relative inline-flex items-baseline gap-2.5 pb-4 transition-colors ${
                active ? "text-white/95" : "text-white/40 hover:text-white/75"
              }`}
            >
              <span className="font-[family-name:var(--font-display)] text-[18px] tracking-[-0.01em] sm:text-xl">
                {o.name}
              </span>
              {active && (
                <span
                  aria-hidden
                  className="absolute inset-x-0 -bottom-px h-px bg-white"
                />
              )}
            </button>
          );
        })}
      </div>

      {/* Glass poster */}
      <div className="relative mt-16 sm:mt-20">
        {/* Decorative gradient blobs that the backdrop-blur picks up */}
        <div
          aria-hidden
          className="pointer-events-none absolute -inset-12 -z-10 overflow-hidden sm:-inset-20"
        >
          <div
            className="absolute left-[8%] top-1/2 size-[28rem] -translate-y-1/2 rounded-full opacity-70 blur-3xl"
            style={{
              background:
                "radial-gradient(circle, rgba(126,184,230,0.18) 0%, transparent 70%)",
            }}
          />
          <div
            className="absolute right-[6%] top-[18%] size-[26rem] rounded-full opacity-60 blur-3xl"
            style={{
              background:
                "radial-gradient(circle, rgba(255,255,255,0.10) 0%, transparent 70%)",
            }}
          />
          <div
            className="absolute right-[20%] bottom-[10%] size-[20rem] rounded-full opacity-50 blur-3xl"
            style={{
              background:
                "radial-gradient(circle, rgba(180,150,255,0.12) 0%, transparent 70%)",
            }}
          />
        </div>

        <div
          key={m.name}
          className="relative overflow-hidden rounded-3xl border border-white/[0.08] bg-white/[0.025] p-8 backdrop-blur-2xl backdrop-saturate-150 motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:duration-500 sm:p-12 lg:p-16"
        >
          {/* top edge highlight */}
          <div
            aria-hidden
            className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/25 to-transparent"
          />
          {/* inner glow on AgentClash side */}
          <div
            aria-hidden
            className="pointer-events-none absolute inset-y-0 right-0 hidden w-2/3 md:block"
            style={{
              background:
                "radial-gradient(60% 70% at 80% 50%, rgba(255,255,255,0.04) 0%, transparent 75%)",
            }}
          />

          <div className="relative grid gap-16 md:grid-cols-[0.85fr_1.15fr] md:gap-20 lg:gap-28">
            {/* Opponent — dim, smaller */}
            <div className="md:pt-3">
              <h3 className="font-[family-name:var(--font-display)] text-2xl tracking-[-0.01em] text-white/55 sm:text-[28px]">
                {m.name}
              </h3>
              <p className="mt-1.5 text-[13.5px] leading-[1.5] text-white/30">
                {m.tag}
              </p>
              <p className="mt-10 font-[family-name:var(--font-display)] tracking-[-0.035em] leading-[0.96] text-white/35 text-[clamp(2.25rem,4.6vw,4rem)]">
                {m.claim}
              </p>
              <p className="mt-6 max-w-[26ch] text-[15px] leading-[1.55] text-white/30">
                {m.limit}
              </p>
            </div>

            {/* AgentClash — bright, larger */}
            <div className="relative md:pl-12 lg:pl-16">
              <div
                aria-hidden
                className="pointer-events-none absolute inset-y-2 left-0 hidden w-px bg-gradient-to-b from-white/0 via-white/35 to-white/0 md:block"
              />
              <h3 className="font-[family-name:var(--font-display)] text-2xl tracking-[-0.01em] text-white/95 sm:text-[28px]">
                AgentClash
              </h3>
              <p className="mt-1.5 text-[13.5px] leading-[1.5] text-white/55">
                agent race engine
              </p>
              <p className="mt-10 font-[family-name:var(--font-display)] tracking-[-0.035em] leading-[0.94] text-white/95 text-[clamp(2.75rem,6.4vw,5.75rem)]">
                {m.counter}
              </p>
              <p className="mt-6 max-w-[34ch] text-[16px] leading-[1.55] text-white/65">
                {m.edge}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Counter + nav */}
      <div className="mt-16 flex items-center justify-between border-t border-white/[0.06] pt-6">
        <p className="font-[family-name:var(--font-mono)] text-[12px] text-white/40">
          {String(activeIndex + 1).padStart(2, "0")}
          <span className="text-white/15"> / </span>
          {String(OPPONENTS.length).padStart(2, "0")}
        </p>
        <div className="flex items-center gap-5 text-[13px]">
          <button
            type="button"
            aria-label="Previous competitor"
            onClick={goPrev}
            className="font-[family-name:var(--font-display)] text-white/50 transition-colors hover:text-white/95"
          >
            ← prev
          </button>
          <button
            type="button"
            aria-label="Next competitor"
            onClick={goNext}
            className="font-[family-name:var(--font-display)] text-white/50 transition-colors hover:text-white/95"
          >
            next →
          </button>
        </div>
      </div>

    </div>
  );
}
