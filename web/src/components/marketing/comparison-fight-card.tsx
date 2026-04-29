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
      {/* Tab strip — editorial, not pills */}
      <div
        role="tablist"
        aria-label="Choose competitor"
        className="flex flex-wrap items-baseline gap-x-7 gap-y-4 border-b border-white/[0.08] pb-5 sm:gap-x-10"
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
              className={`group relative inline-flex items-baseline gap-2.5 pb-3 transition-colors -mb-[22px] ${
                active
                  ? "text-white/95"
                  : "text-white/40 hover:text-white/75"
              }`}
            >
              <span
                className={`text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.24em] ${
                  active ? "text-white/55" : "text-white/30"
                }`}
              >
                vs
              </span>
              <span className="font-[family-name:var(--font-display)] text-[18px] tracking-[-0.01em] sm:text-xl">
                {o.name}
              </span>
              {active && (
                <span
                  aria-hidden
                  className="absolute inset-x-0 bottom-0 h-px bg-white"
                />
              )}
            </button>
          );
        })}
      </div>

      {/* Poster */}
      <div
        key={m.name}
        className="mt-14 grid gap-14 sm:mt-16 md:grid-cols-[0.9fr_1.1fr] md:gap-20 lg:gap-24"
      >
        {/* Opponent — dim, smaller */}
        <div className="md:pt-2">
          <div className="flex items-baseline gap-3 text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.24em] text-white/30">
            <span className="text-white/45">{m.name}</span>
            <span className="text-white/15">·</span>
            <span>{m.tag}</span>
          </div>
          <p className="mt-7 font-[family-name:var(--font-display)] tracking-[-0.035em] leading-[0.96] text-white/35 text-[clamp(2.5rem,5vw,4.5rem)]">
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
            className="hidden md:block pointer-events-none absolute inset-y-2 left-0 w-px bg-gradient-to-b from-white/0 via-white/30 to-white/0"
          />
          <div className="flex items-baseline gap-3 text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.24em] text-white/55">
            <span className="text-white/85">AgentClash</span>
            <span className="text-white/25">·</span>
            <span>agent race engine</span>
          </div>
          <p className="mt-7 font-[family-name:var(--font-display)] tracking-[-0.035em] leading-[0.94] text-white/95 text-[clamp(3rem,6.5vw,6rem)]">
            {m.counter}
          </p>
          <p className="mt-6 max-w-[34ch] text-[16px] leading-[1.55] text-white/65">
            {m.edge}
          </p>
        </div>
      </div>

      {/* Counter + nav */}
      <div className="mt-14 flex items-center justify-between border-t border-white/[0.06] pt-6">
        <p className="text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/35">
          {String(activeIndex + 1).padStart(2, "0")}
          <span className="text-white/15"> / </span>
          {String(OPPONENTS.length).padStart(2, "0")}
        </p>
        <div className="flex items-center gap-3">
          <button
            type="button"
            aria-label="Previous competitor"
            onClick={goPrev}
            className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.22em] text-white/45 transition-colors hover:text-white/85"
          >
            ← prev
          </button>
          <span aria-hidden className="text-white/15">
            ·
          </span>
          <button
            type="button"
            aria-label="Next competitor"
            onClick={goNext}
            className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.22em] text-white/45 transition-colors hover:text-white/85"
          >
            next →
          </button>
        </div>
      </div>
    </div>
  );
}
