"use client";

import { useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

type Stat = {
  label: string;
  body: string;
};

type Opponent = {
  name: string;
  tag: string;
  stats: Stat[];
  verdict: string;
};

const AGENTCLASH = {
  name: "AgentClash",
  tag: "agent race engine",
  stats: [
    {
      label: "Strengths",
      body: "Multi-turn loops in real microVMs. Head-to-head races on a shared budget. Trajectory scoring on the path, not just the answer.",
    },
    {
      label: "Best for",
      body: "Picking the right agent for a real task — and gating CI on the result.",
    },
    {
      label: "The edge",
      body: "Same challenge, same tools, same clock. Failures auto-promote to regressions.",
    },
  ],
  verdict: "We race agents. Live, and on the same budget.",
};

const OPPONENTS: Opponent[] = [
  {
    name: "Braintrust",
    tag: "prompt eval",
    stats: [
      {
        label: "Strengths",
        body: "Clean eval primitives. Dataset versioning. Solid scoring helpers for one-shot completions.",
      },
      {
        label: "Best for",
        body: "Scoring text a model produces from a single prompt, at scale.",
      },
      {
        label: "Where it breaks",
        body: "No sandboxed tool execution. No multi-turn agent loops. No concurrent race on a shared budget.",
      },
    ],
    verdict: "Built to grade text. Stops where agents start.",
  },
  {
    name: "LangSmith",
    tag: "prompt tracing + eval",
    stats: [
      {
        label: "Strengths",
        body: "Deep tracing inside the LangChain ecosystem. Rich span trees, prompt playground, dataset capture.",
      },
      {
        label: "Best for",
        body: "Debugging LangChain pipelines and prompt regressions inside that ecosystem.",
      },
      {
        label: "Where it breaks",
        body: "LangChain-shaped. No microVM sandbox. No agent-vs-agent race on shared inputs.",
      },
    ],
    verdict: "A trace viewer for LangChain. Not a race engine.",
  },
  {
    name: "Langfuse",
    tag: "open-source observability",
    stats: [
      {
        label: "Strengths",
        body: "Open-source, self-hostable. Generous tracing, prompt management, cost tracking.",
      },
      {
        label: "Best for",
        body: "LLM observability without a vendor lock-in.",
      },
      {
        label: "Where it breaks",
        body: "No native agent racing. No sandbox provisioning. No trajectory scoring.",
      },
    ],
    verdict: "Watches your LLM. Doesn't evaluate your agent.",
  },
  {
    name: "Arize Phoenix",
    tag: "LLM observability",
    stats: [
      {
        label: "Strengths",
        body: "Deep telemetry, drift detection, OpenTelemetry-native, production-grade dashboards.",
      },
      {
        label: "Best for",
        body: "Monitoring LLM apps already in production.",
      },
      {
        label: "Where it breaks",
        body: "Observation-only. No eval orchestration. Doesn't run agents — only watches them.",
      },
    ],
    verdict: "Watches your agents. Doesn't race them.",
  },
];

export function ComparisonFightCard() {
  const [activeIndex, setActiveIndex] = useState(0);
  const opponent = OPPONENTS[activeIndex];

  const goPrev = () => {
    setActiveIndex((i) => (i - 1 + OPPONENTS.length) % OPPONENTS.length);
  };
  const goNext = () => {
    setActiveIndex((i) => (i + 1) % OPPONENTS.length);
  };

  return (
    <div>
      {/* Tab strip */}
      <div
        role="tablist"
        aria-label="Choose competitor"
        className="-mx-1 flex flex-wrap items-center gap-1.5 sm:gap-2"
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
              className={`group inline-flex items-center gap-2 rounded-full border px-4 py-2 text-[13px] transition-all ${
                active
                  ? "border-white/30 bg-white/[0.06] text-white/95"
                  : "border-white/[0.08] bg-white/[0.015] text-white/55 hover:border-white/15 hover:text-white/80"
              }`}
            >
              <span
                aria-hidden
                className={`text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.2em] ${
                  active ? "text-white/55" : "text-white/30"
                }`}
              >
                vs
              </span>
              <span className="font-[family-name:var(--font-display)] tracking-[-0.005em]">
                {o.name}
              </span>
            </button>
          );
        })}
      </div>

      {/* Fight card */}
      <article
        key={opponent.name}
        className="mt-10 overflow-hidden rounded-2xl border border-white/[0.08] bg-[#0a0a0a]"
      >
        {/* Header bar */}
        <header className="grid grid-cols-[1fr_auto_1fr] items-center gap-4 border-b border-white/[0.08] px-5 py-5 sm:px-8 sm:py-6">
          <div className="flex flex-col items-start">
            <span className="text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/30">
              {opponent.tag}
            </span>
            <h3 className="mt-2 font-[family-name:var(--font-display)] text-2xl tracking-[-0.01em] text-white/70 sm:text-[28px]">
              {opponent.name}
            </h3>
          </div>

          <div className="flex flex-col items-center justify-center">
            <span
              aria-hidden
              className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.32em] text-white/30"
            >
              vs
            </span>
            <span
              aria-hidden
              className="mt-1 inline-block size-1.5 rounded-full bg-white/30"
            />
          </div>

          <div className="flex flex-col items-end text-right">
            <span className="text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/55">
              {AGENTCLASH.tag}
            </span>
            <h3 className="mt-2 font-[family-name:var(--font-display)] text-2xl tracking-[-0.01em] text-white/95 sm:text-[28px]">
              {AGENTCLASH.name}
            </h3>
          </div>
        </header>

        {/* Body — opponent left, AgentClash right */}
        <div className="grid grid-cols-1 md:grid-cols-2">
          <div className="bg-white/[0.012] p-7 sm:p-9 md:border-r md:border-white/[0.06]">
            <ul className="space-y-7">
              {opponent.stats.map((s) => (
                <li key={s.label}>
                  <p className="text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/35">
                    {s.label}
                  </p>
                  <p className="mt-2 text-[14.5px] leading-[1.55] text-white/55">
                    {s.body}
                  </p>
                </li>
              ))}
            </ul>
          </div>

          <div className="relative bg-white/[0.025] p-7 sm:p-9">
            <div
              aria-hidden
              className="pointer-events-none absolute inset-y-0 left-0 w-px bg-gradient-to-b from-transparent via-white/40 to-transparent"
            />
            <ul className="space-y-7">
              {AGENTCLASH.stats.map((s) => (
                <li key={s.label}>
                  <p className="text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/55">
                    {s.label}
                  </p>
                  <p className="mt-2 text-[14.5px] leading-[1.55] text-white/85">
                    {s.body}
                  </p>
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* Verdict bar */}
        <footer className="grid grid-cols-1 border-t border-white/[0.08] md:grid-cols-2">
          <div className="bg-white/[0.012] px-7 py-5 sm:px-9 md:border-r md:border-white/[0.06]">
            <p className="text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/30">
              Verdict
            </p>
            <p className="mt-1.5 text-[14px] leading-[1.5] text-white/55">
              {opponent.verdict}
            </p>
          </div>
          <div className="bg-white/[0.025] px-7 py-5 sm:px-9">
            <p className="text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/55">
              Verdict
            </p>
            <p className="mt-1.5 text-[14px] leading-[1.5] text-white/90">
              {AGENTCLASH.verdict}
            </p>
          </div>
        </footer>
      </article>

      {/* Prev / next + count */}
      <div className="mt-6 flex items-center justify-between">
        <p className="text-[12px] font-[family-name:var(--font-mono)] text-white/35">
          {String(activeIndex + 1).padStart(2, "0")}
          <span className="text-white/20"> / </span>
          {String(OPPONENTS.length).padStart(2, "0")}
        </p>
        <div className="flex items-center gap-2">
          <button
            type="button"
            aria-label="Previous competitor"
            onClick={goPrev}
            className="inline-flex size-9 items-center justify-center rounded-full border border-white/[0.08] bg-white/[0.015] text-white/55 transition-colors hover:border-white/20 hover:text-white/85"
          >
            <ChevronLeft className="size-4" />
          </button>
          <button
            type="button"
            aria-label="Next competitor"
            onClick={goNext}
            className="inline-flex size-9 items-center justify-center rounded-full border border-white/[0.08] bg-white/[0.015] text-white/55 transition-colors hover:border-white/20 hover:text-white/85"
          >
            <ChevronRight className="size-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
