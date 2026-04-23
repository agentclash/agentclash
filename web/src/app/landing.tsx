"use client";

import type React from "react";
import Link from "next/link";
import { useAuth } from "@workos-inc/authkit-nextjs/components";
import { ArrowRight, Calendar, ExternalLink, LogIn, Star } from "lucide-react";
import {
  Anthropic,
  Gemini,
  Mistral,
  OpenAI,
  OpenRouter,
  XAI,
} from "@lobehub/icons";

const DEMO_URL = "https://cal.com/agentclash/demo";

function ClashMark({
  className = "",
  animated = false,
}: {
  className?: string;
  animated?: boolean;
}) {
  return (
    <svg
      viewBox="0 0 512 512"
      className={className}
      aria-label="AgentClash"
      role="img"
    >
      <g className={animated ? "animate-clash-left" : undefined}>
        <polygon
          points="80,180 240,256 80,332"
          fill="#ffffff"
          opacity="0.95"
        />
      </g>
      <g className={animated ? "animate-clash-right" : undefined}>
        <polygon
          points="432,180 272,256 432,332"
          fill="#ffffff"
          opacity="0.5"
        />
      </g>
      <g className={animated ? "animate-clash-sparks" : undefined}>
        <line
          x1="256" y1="96" x2="256" y2="168"
          stroke="#ffffff" strokeWidth="10" strokeLinecap="round" opacity="0.75"
        />
        <line
          x1="256" y1="344" x2="256" y2="416"
          stroke="#ffffff" strokeWidth="10" strokeLinecap="round" opacity="0.75"
        />
        <line
          x1="186" y1="130" x2="216" y2="188"
          stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55"
        />
        <line
          x1="326" y1="130" x2="296" y2="188"
          stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55"
        />
        <line
          x1="186" y1="382" x2="216" y2="324"
          stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55"
        />
        <line
          x1="326" y1="382" x2="296" y2="324"
          stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55"
        />
      </g>
    </svg>
  );
}

const PROVIDERS: Array<{ name: string; render: (size: number) => React.ReactNode }> = [
  { name: "OpenAI", render: (size) => <OpenAI size={size} color="#74AA9C" /> },
  { name: "Anthropic", render: (size) => <Anthropic size={size} color="#D97757" /> },
  { name: "Gemini", render: (size) => <Gemini.Color size={size} /> },
  { name: "xAI", render: (size) => <XAI size={size} color="#FFFFFF" /> },
  { name: "Mistral", render: (size) => <Mistral.Color size={size} /> },
  { name: "OpenRouter", render: (size) => <OpenRouter size={size} color="#6566F1" /> },
];

function DottedSpotlight({
  children,
  className = "",
}: {
  children?: React.ReactNode;
  className?: string;
}) {
  function handleMouseMove(e: React.MouseEvent<HTMLDivElement>) {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    e.currentTarget.style.setProperty("--mx", `${x}px`);
    e.currentTarget.style.setProperty("--my", `${y}px`);
  }

  const ambientHalo =
    "radial-gradient(ellipse 60% 60% at center, rgba(255,255,255,0.07) 0%, rgba(255,255,255,0.025) 40%, transparent 78%)";
  const baseMask =
    "radial-gradient(ellipse 75% 75% at center, black 20%, transparent 88%)";
  const cursorMask =
    "radial-gradient(320px circle at var(--mx) var(--my), black 0%, black 25%, transparent 72%)";
  const dotImage =
    "radial-gradient(rgba(255,255,255,1) 1px, transparent 1px)";
  const cursorBloom =
    "radial-gradient(240px circle at var(--mx) var(--my), rgba(255,255,255,0.09) 0%, rgba(255,255,255,0.03) 35%, transparent 70%)";

  return (
    <div
      onMouseMove={handleMouseMove}
      className={`group relative ${className}`}
      style={{ ["--mx" as string]: "50%", ["--my" as string]: "50%" }}
    >
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0"
        style={{ backgroundImage: ambientHalo }}
      />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-30"
        style={{
          backgroundImage: dotImage,
          backgroundSize: "22px 22px",
          maskImage: baseMask,
          WebkitMaskImage: baseMask,
        }}
      />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 ease-out group-hover:opacity-100"
        style={{ backgroundImage: cursorBloom }}
      />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 ease-out group-hover:opacity-90"
        style={{
          backgroundImage: dotImage,
          backgroundSize: "22px 22px",
          maskImage: cursorMask,
          WebkitMaskImage: cursorMask,
        }}
      />
      <div className="relative">{children}</div>
    </div>
  );
}

function TargetGlyph() {
  return (
    <svg
      viewBox="0 0 48 48"
      className="size-7 text-white/90"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      aria-hidden
    >
      <circle cx="24" cy="24" r="19" opacity="0.32" />
      <circle cx="24" cy="24" r="12" opacity="0.6" />
      <circle cx="24" cy="24" r="5" opacity="0.9" />
      <circle cx="24" cy="24" r="1.5" fill="currentColor" stroke="none" />
    </svg>
  );
}

function LineupGlyph() {
  return (
    <svg
      viewBox="0 0 48 48"
      className="size-7 text-white/90"
      fill="currentColor"
      aria-hidden
    >
      <polygon points="6,13 14,18 6,23" opacity="0.95" />
      <polygon points="20,13 28,18 20,23" opacity="0.8" />
      <polygon points="34,13 42,18 34,23" opacity="0.65" />
      <polygon points="6,28 14,33 6,38" opacity="0.5" />
      <polygon points="20,28 28,33 20,38" opacity="0.4" />
      <polygon points="34,28 42,33 34,38" opacity="0.3" />
    </svg>
  );
}

function LightFlowArrows() {
  const COUNT = 9;
  const DURATION = 3.6;
  return (
    <div
      className="flex flex-col items-center justify-center gap-5 py-8 sm:gap-7 sm:py-12"
      aria-hidden
    >
      {Array.from({ length: COUNT }).map((_, i) => (
        <svg
          key={i}
          viewBox="0 0 48 24"
          className="animate-arrow-flow h-5 w-11 text-white"
          style={{
            animationDelay: `${(-(i / COUNT) * DURATION).toFixed(2)}s`,
          }}
          focusable="false"
        >
          <path
            d="M6 7 L24 19 L42 7"
            stroke="currentColor"
            strokeWidth="2.25"
            strokeLinecap="round"
            strokeLinejoin="round"
            fill="none"
          />
        </svg>
      ))}
    </div>
  );
}

function HorizontalArrowFlow() {
  const COUNT = 7;
  const DURATION = 3.2;
  return (
    <div
      className="flex items-center justify-center gap-6 py-4 sm:gap-8"
      aria-hidden
    >
      {Array.from({ length: COUNT }).map((_, i) => (
        <svg
          key={i}
          viewBox="0 0 24 48"
          className="animate-arrow-flow h-8 w-4 text-white"
          style={{
            animationDelay: `${(-(i / COUNT) * DURATION).toFixed(2)}s`,
          }}
          focusable="false"
        >
          <path
            d="M7 8 L19 24 L7 40"
            stroke="currentColor"
            strokeWidth="2.25"
            strokeLinecap="round"
            strokeLinejoin="round"
            fill="none"
          />
        </svg>
      ))}
    </div>
  );
}

function ConvergenceScoring() {
  return (
    <div
      className="flex items-center justify-center py-6 sm:py-10"
      aria-hidden
    >
      <svg
        viewBox="0 0 320 320"
        className="w-full max-w-[400px]"
        focusable="false"
      >
        <circle
          cx="160"
          cy="160"
          r="10"
          fill="white"
          opacity="0.9"
        />
        <circle
          cx="160"
          cy="160"
          r="22"
          fill="none"
          stroke="white"
          strokeWidth="1"
          opacity="0.3"
        />
        <circle
          cx="160"
          cy="160"
          r="34"
          fill="none"
          stroke="white"
          strokeWidth="1"
          opacity="0.12"
        />

        <line
          x1="160"
          y1="44"
          x2="160"
          y2="136"
          stroke="white"
          strokeWidth="1.1"
          opacity="0.38"
        />
        <line
          x1="276"
          y1="160"
          x2="184"
          y2="160"
          stroke="white"
          strokeWidth="1.1"
          opacity="0.38"
        />
        <line
          x1="160"
          y1="276"
          x2="160"
          y2="184"
          stroke="white"
          strokeWidth="1.1"
          opacity="0.38"
        />
        <line
          x1="44"
          y1="160"
          x2="136"
          y2="160"
          stroke="white"
          strokeWidth="1.1"
          opacity="0.38"
        />

        <circle r="3.2" fill="white" className="animate-converge-top" />
        <circle r="3.2" fill="white" className="animate-converge-right" />
        <circle r="3.2" fill="white" className="animate-converge-bottom" />
        <circle r="3.2" fill="white" className="animate-converge-left" />

        <text
          x="160"
          y="28"
          textAnchor="middle"
          fill="white"
          opacity="0.55"
          fontSize="11"
          fontFamily="var(--font-mono), monospace"
          letterSpacing="0.05em"
        >
          validators
        </text>
        <text
          x="294"
          y="164"
          textAnchor="start"
          fill="white"
          opacity="0.55"
          fontSize="11"
          fontFamily="var(--font-mono), monospace"
          letterSpacing="0.05em"
        >
          metrics
        </text>
        <text
          x="160"
          y="304"
          textAnchor="middle"
          fill="white"
          opacity="0.55"
          fontSize="11"
          fontFamily="var(--font-mono), monospace"
          letterSpacing="0.05em"
        >
          judges
        </text>
        <text
          x="26"
          y="164"
          textAnchor="end"
          fill="white"
          opacity="0.55"
          fontSize="11"
          fontFamily="var(--font-mono), monospace"
          letterSpacing="0.05em"
        >
          signals
        </text>
      </svg>
    </div>
  );
}

function SandboxLanes() {
  const DURATION = 3.8;
  return (
    <div
      className="flex flex-col items-stretch justify-center gap-3.5 py-6 sm:gap-4 sm:py-10"
      aria-hidden
    >
      {PROVIDERS.map(({ name, render }, i) => (
        <div
          key={name}
          className="relative h-12 overflow-hidden rounded-md border border-white/[0.14]"
        >
          <div
            className="animate-sandbox-travel absolute inset-0 flex items-center justify-center"
            style={{
              animationDelay: `${(-(i / PROVIDERS.length) * DURATION).toFixed(2)}s`,
            }}
          >
            {render(24)}
          </div>
        </div>
      ))}
    </div>
  );
}

function FeedbackLoop() {
  return (
    <div className="flex items-center justify-center py-6 sm:py-10" aria-hidden>
      <svg
        viewBox="0 0 400 230"
        className="w-full max-w-[480px]"
        focusable="false"
      >
        <defs>
          <marker
            id="feedback-arrow-head"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto"
          >
            <polygon points="0,0 10,5 0,10" fill="white" opacity="0.7" />
          </marker>
        </defs>

        <circle
          cx="70"
          cy="115"
          r="30"
          fill="none"
          stroke="white"
          strokeWidth="1.2"
          opacity="0.7"
        />
        <circle
          cx="330"
          cy="115"
          r="30"
          fill="none"
          stroke="white"
          strokeWidth="1.2"
          opacity="0.7"
        />

        <path
          d="M 92 99 Q 200 10 308 99"
          stroke="white"
          strokeWidth="1.25"
          fill="none"
          opacity="0.42"
          markerEnd="url(#feedback-arrow-head)"
        />
        <circle r="3.2" fill="white" className="animate-travel-top" />

        <path
          d="M 308 131 Q 200 220 92 131"
          stroke="white"
          strokeWidth="1.25"
          fill="none"
          opacity="0.42"
          markerEnd="url(#feedback-arrow-head)"
        />
        <circle r="3.2" fill="white" className="animate-travel-bottom" />
      </svg>
    </div>
  );
}

function TrackGlyph() {
  return (
    <svg
      viewBox="0 0 48 48"
      className="size-7 text-white/90"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      aria-hidden
    >
      <line x1="5" y1="24" x2="34" y2="24" opacity="0.45" />
      <circle cx="10" cy="24" r="1.3" fill="currentColor" opacity="0.4" stroke="none" />
      <circle cx="18" cy="24" r="1.3" fill="currentColor" opacity="0.65" stroke="none" />
      <circle cx="26" cy="24" r="2" fill="currentColor" opacity="0.95" stroke="none" />
      <line x1="36" y1="12" x2="36" y2="36" strokeWidth="1.2" opacity="0.55" />
      <g fill="currentColor" stroke="none" opacity="0.9">
        <rect x="37" y="12" width="3" height="3" />
        <rect x="40" y="15" width="3" height="3" />
        <rect x="37" y="18" width="3" height="3" />
        <rect x="40" y="21" width="3" height="3" />
        <rect x="37" y="24" width="3" height="3" />
      </g>
    </svg>
  );
}

export default function HomePage() {
  const { user, loading: authLoading } = useAuth();

  return (
    <main className="min-h-screen flex flex-col">
      {/* ── Header ──────────────────────────────────────────────── */}
      <header className="px-8 sm:px-12 py-6 border-b border-white/[0.06]">
        <div className="mx-auto flex max-w-[1440px] items-center justify-between">
          <Link
            href="/"
            className="inline-flex items-center gap-2.5 text-white/90"
          >
            <ClashMark className="size-6" />
            <span className="font-[family-name:var(--font-display)] text-xl tracking-[-0.01em]">
              AgentClash
            </span>
          </Link>
          <nav className="flex items-center gap-1 sm:gap-2 text-xs">
            <Link
              href="/docs"
              className="hidden sm:inline-flex px-3 py-1.5 text-white/55 hover:text-white/85 transition-colors"
            >
              Docs
            </Link>
            <Link
              href="/blog"
              className="hidden sm:inline-flex px-3 py-1.5 text-white/55 hover:text-white/85 transition-colors"
            >
              Blog
            </Link>
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-1.5 text-white/60 hover:text-white/85 hover:border-white/15 transition-colors"
            >
              <Star className="size-3.5" />
              GitHub
            </a>
            {authLoading ? (
              <span className="inline-flex h-[30px] w-[88px] rounded-md border border-white/[0.08] bg-white/[0.04]" />
            ) : user ? (
              <Link
                href="/dashboard"
                className="inline-flex items-center gap-1.5 rounded-md bg-white px-3 py-1.5 font-medium text-[#060606] hover:bg-white/90 transition-colors"
              >
                Dashboard
                <ArrowRight className="size-3" />
              </Link>
            ) : (
              <Link
                href="/auth/login"
                className="inline-flex items-center gap-1.5 rounded-md border border-white/15 bg-white/[0.04] px-3 py-1.5 text-white/75 hover:text-white hover:border-white/25 transition-colors"
              >
                <LogIn className="size-3.5" />
                Sign in
              </Link>
            )}
          </nav>
        </div>
      </header>

      {/* ── Hero ────────────────────────────────────────────────── */}
      <section className="px-8 sm:px-12 pt-32 pb-20 sm:pt-44 sm:pb-28">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-[1.5fr_1fr] md:gap-20 items-center">
          <div>
            <h1 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.04em] leading-[0.95] text-[clamp(3rem,7vw,7.5rem)] max-w-[16ch]">
              Ship the right agent.
              <br />
              <span className="text-white/40">Not the loudest one.</span>
            </h1>

            <p className="mt-10 max-w-[44ch] text-lg sm:text-xl leading-[1.5] text-white/55">
              AgentClash races your models head-to-head on real tasks. Same
              challenge, same tools, same time budget — scored live across
              completion, speed, and efficiency.
            </p>

            <div className="mt-10 flex flex-wrap items-center gap-3">
              {user ? (
                <>
                  <Link
                    href="/dashboard"
                    className="inline-flex items-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
                  >
                    Go to dashboard
                    <ArrowRight className="size-4" />
                  </Link>
                  <a
                    href="https://github.com/agentclash/agentclash"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
                  >
                    <Star className="size-4" />
                    View on GitHub
                  </a>
                </>
              ) : (
                <>
                  <a
                    href={DEMO_URL}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
                  >
                    <Calendar className="size-4" />
                    Book a demo
                  </a>
                  <Link
                    href="/auth/login"
                    className="inline-flex items-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
                  >
                    Get started
                    <ArrowRight className="size-4" />
                  </Link>
                  <a
                    href="https://github.com/agentclash/agentclash"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.02] px-6 py-3 text-sm font-medium text-white/60 hover:text-white/90 hover:border-white/20 transition-colors"
                  >
                    <Star className="size-4" />
                    GitHub
                  </a>
                </>
              )}
            </div>
          </div>

          <div className="hidden md:flex items-center justify-center">
            <DottedSpotlight className="flex aspect-square w-full max-w-[520px] items-center justify-center">
              <ClashMark
                animated
                className="w-full max-w-[360px] aspect-square"
              />
            </DottedSpotlight>
          </div>
        </div>
      </section>

      {/* ── Why we built this ───────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.5rem,6vw,5.5rem)] max-w-[22ch]">
            We got tired of being lied to.
          </h2>

          <div className="mt-14 max-w-[58ch] space-y-7 text-lg leading-[1.65] text-white/65">
            <p>
              A few months ago we were picking a model for a production
              agent — the kind that reads a ticket, opens a PR, runs the
              tests, writes a comment. The benchmarks said one thing. MMLU
              said another. Vendor blog posts told a third. We ran our own
              evals; they were flaky and painful to reason about. We picked
              a model. A week in, it started failing on the exact shape of
              ticket we&apos;d built it for — the same shape it had passed
              every eval we threw at it.
            </p>
            <p>
              We re-read every score. None of them had touched our task.
              They had measured one kind of intelligence, and we had
              shipped another.
            </p>
            <p className="font-[family-name:var(--font-display)] text-2xl sm:text-3xl leading-[1.3] tracking-[-0.015em] text-white/90 !mt-12">
              Static benchmarks leak. Leaderboards reward hype. The only
              eval you can trust is the one you ran yourself, on your own
              task, against every other model you were considering, at the
              same time.
            </p>
            <p>
              AgentClash is what we wish had existed that week. You
              describe the task the way your product actually does it. Pick
              six models. They race, live, on the same inputs, with the
              same tools, scored on what matters in production —
              correctness, cost, latency, behaviour under pressure. When
              one fails, the failing trace becomes a test. Every mistake
              ratchets the eval tighter.
            </p>
            <p>
              We&apos;re building it in the open because no closed
              benchmark has ever stayed honest for long. If this feels
              familiar — run a race. Your task. Your models. Your
              scoreboard.
            </p>
          </div>
        </div>
      </section>

      {/* ── How it works ────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <div className="flex flex-col gap-10 md:flex-row md:items-end md:justify-between md:gap-16">
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.5rem,6vw,5.5rem)] max-w-[20ch]">
              From challenge to scoreboard.
            </h2>
            <p className="max-w-[38ch] text-base leading-[1.6] text-white/50">
              Set up a head-to-head race in under a minute. Watch a verdict
              arrive in the time it takes to finish a coffee.
            </p>
          </div>

          <div className="relative mt-24">
            <div
              className="hidden md:block pointer-events-none absolute left-0 right-0 top-[32px] border-t border-dashed border-white/10"
              aria-hidden
            />

            <ol className="relative grid gap-20 md:grid-cols-3 md:gap-14">
              {[
                {
                  n: "01",
                  title: "Pick a challenge",
                  body:
                    "Write your own or pull from the library. Real tasks — a broken auth server, a SQL bug, a spec to implement — not trivia.",
                  glyph: <TargetGlyph />,
                },
                {
                  n: "02",
                  title: "Pick your models",
                  body:
                    "Line up six or eight contestants across providers. Same tool policy, same time budget, same starting state.",
                  glyph: <LineupGlyph />,
                },
                {
                  n: "03",
                  title: "Watch them race",
                  body:
                    "Live scoring as they work. Composite metric across completion, speed, token efficiency, and tool strategy.",
                  glyph: <TrackGlyph />,
                },
              ].map((step) => (
                <li key={step.n} className="relative">
                  <div className="relative z-10 inline-flex size-16 items-center justify-center rounded-full border border-white/15 bg-[#060606]">
                    {step.glyph}
                  </div>
                  <p className="mt-10 font-[family-name:var(--font-display)] text-6xl leading-none tracking-[-0.03em] text-white/15">
                    {step.n}
                  </p>
                  <h3 className="mt-4 font-[family-name:var(--font-display)] text-3xl sm:text-4xl tracking-[-0.02em] leading-[1.08] text-white/95">
                    {step.title}
                  </h3>
                  <p className="mt-5 max-w-[34ch] text-base leading-[1.65] text-white/55">
                    {step.body}
                  </p>
                </li>
              ))}
            </ol>
          </div>
        </div>
      </section>

      {/* ── Feature · Replay ────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4.5rem)] max-w-[20ch]">
              Scrub the replay. See exactly where it got stuck.
            </h2>
            <p className="mt-10 max-w-[48ch] text-lg leading-[1.6] text-white/55">
              Every think, every tool call, every observation is captured.
              Step back to the moment a model went sideways — the prompt
              it saw, the output it produced, the state it worked from. No
              more guessing why one model won and another flunked.
            </p>
          </div>
          <div>
            <LightFlowArrows />
          </div>
        </div>
      </section>

      {/* ── Feature · Regression tests ──────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,4rem)]">
              Failures become your regression suite.
            </h2>
            <div className="mt-10 space-y-6">
              <p className="text-lg leading-[1.6] text-white/60">
                When a model flunks a challenge, the failing trace is
                frozen into a permanent test. Next week&apos;s race
                replays it. The following month&apos;s does too.
              </p>
              <p className="text-lg leading-[1.6] text-white/60">
                Your eval suite sharpens itself with use. By the time a
                new model arrives, it walks into a track that was paved
                by every mistake the last model made.
              </p>
            </div>
          </div>
          <div>
            <FeedbackLoop />
          </div>
        </div>
      </section>

      {/* ── Providers ───────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <div className="flex flex-col gap-10 md:flex-row md:items-end md:justify-between md:gap-16">
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.5rem,6vw,5.5rem)] max-w-[20ch]">
              Any model.
              <br />
              <span className="text-white/40">Any provider.</span>
            </h2>
            <p className="max-w-[42ch] text-base leading-[1.6] text-white/50">
              Normalised tool-calls, normalised errors, same scoring rules.
              First-class adapters for the providers below, plus OpenRouter
              for the long tail — three hundred more models, no extra code.
            </p>
          </div>

          <ul className="mt-20 grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-px border-y border-white/[0.06] bg-white/[0.06]">
            {PROVIDERS.map(({ name, render }, i) => (
              <li
                key={name}
                className="group relative flex flex-col items-center justify-center gap-4 overflow-hidden bg-[#060606] py-14 transition-colors hover:bg-white/[0.02]"
              >
                <div
                  aria-hidden
                  className="animate-provider-glow pointer-events-none absolute left-1/2 top-[44%] size-32 -translate-x-1/2 -translate-y-1/2 rounded-full transition-opacity duration-500 group-hover:opacity-[0.8]"
                  style={{
                    background:
                      "radial-gradient(circle, rgba(255,255,255,0.18), transparent 70%)",
                    animationDelay: `${(-(i / PROVIDERS.length) * 9).toFixed(2)}s`,
                  }}
                />
                <div className="relative opacity-90 transition-opacity group-hover:opacity-100">
                  {render(40)}
                </div>
                <span className="relative text-sm text-white/55 transition-colors group-hover:text-white/85">
                  {name}
                </span>
              </li>
            ))}
          </ul>

          <p className="mt-8 text-sm text-white/40">
            Plus 300 more via OpenRouter. New first-class providers landing
            every month.
          </p>
        </div>
      </section>

      {/* ── Sandbox ─────────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4.5rem)] max-w-[20ch]">
              A fresh microVM for every agent.
            </h2>
            <p className="mt-10 max-w-[48ch] text-lg leading-[1.6] text-white/60">
              Each racer boots into its own Firecracker microVM — isolated
              filesystem, isolated network, no shared kernel. When the race
              ends, the sandbox is torn down. The next one spins up clean.
            </p>
            <p className="mt-6 max-w-[48ch] text-lg leading-[1.6] text-white/60">
              That isolation isn&apos;t just safety. It&apos;s what makes
              the race fair. No model gets a warm cache. No prompt leaks
              between lanes. The only variable in the race is the model.
            </p>
            <p className="mt-10 max-w-[48ch] text-sm text-white/40">
              Powered by{" "}
              <a
                href="https://e2b.dev"
                target="_blank"
                rel="noopener noreferrer"
                className="text-white/65 underline decoration-white/20 underline-offset-4 transition-colors hover:text-white/90 hover:decoration-white/40"
              >
                E2B
              </a>
              &nbsp;— the sandbox infrastructure behind AI products at
              Perplexity, Hugging Face, and Groq.
            </p>
          </div>
          <div>
            <SandboxLanes />
          </div>
        </div>
      </section>

      {/* ── Scoring ─────────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4.5rem)] max-w-[20ch]">
              One number is a lie.
            </h2>
            <p className="mt-10 max-w-[48ch] text-lg leading-[1.6] text-white/60">
              A passing benchmark and a working product are two different
              things. Every run is judged across independent signals —
              answer correctness, runtime efficiency, how the agent
              behaves when the easy path breaks, and peer review from a
              panel of judge models. You weight them however your
              workload demands.
            </p>

            <dl className="mt-10 grid gap-x-10 gap-y-6 sm:grid-cols-2">
              <div>
                <dt className="font-[family-name:var(--font-display)] text-lg tracking-[-0.015em] text-white/90">
                  Validators
                </dt>
                <dd className="mt-2 text-[13.5px] leading-[1.55] text-white/50">
                  Exact, regex, JSON Schema, math equivalence, BLEU /
                  ROUGE / ChrF, code execution, file-tree assertions. If
                  you can define &ldquo;right,&rdquo; we can check it.
                </dd>
              </div>
              <div>
                <dt className="font-[family-name:var(--font-display)] text-lg tracking-[-0.015em] text-white/90">
                  Runtime metrics
                </dt>
                <dd className="mt-2 text-[13.5px] leading-[1.55] text-white/50">
                  Latency, cost per run, reliability across retries — the
                  numbers that decide whether you can ship it.
                </dd>
              </div>
              <div>
                <dt className="font-[family-name:var(--font-display)] text-lg tracking-[-0.015em] text-white/90">
                  Behavioural signals
                </dt>
                <dd className="mt-2 text-[13.5px] leading-[1.55] text-white/50">
                  Recovery after failure, tool-use diversity, scope
                  adherence, confidence calibration. How an agent{" "}
                  <em>acts</em> when things go sideways.
                </dd>
              </div>
              <div>
                <dt className="font-[family-name:var(--font-display)] text-lg tracking-[-0.015em] text-white/90">
                  LLM-as-judge
                </dt>
                <dd className="mt-2 text-[13.5px] leading-[1.55] text-white/50">
                  Rubric, assertion, reference, and pairwise modes. Run
                  multiple judges, take the median or majority — no
                  single model gets to be the sole arbiter.
                </dd>
              </div>
            </dl>

            <p className="mt-10 max-w-[52ch] text-sm text-white/40">
              Three composition strategies — weighted average, binary
              pass/fail, or hybrid with gates — so the verdict matches
              the bar you&apos;d actually ship against.
            </p>
          </div>
          <div>
            <ConvergenceScoring />
          </div>
        </div>
      </section>

      {/* ── Closing CTA ─────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-40 sm:py-56">
        <div className="mx-auto max-w-[1440px]">
          <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.04em] leading-[0.95] text-[clamp(3rem,8vw,7rem)] max-w-[16ch]">
            Stop guessing.
            <br />
            <span className="text-white/40">Start racing.</span>
          </h2>
          <div className="mt-12">
            <HorizontalArrowFlow />
          </div>
          <div className="mt-8 flex flex-wrap gap-3">
            {user ? (
              <Link
                href="/dashboard"
                className="inline-flex items-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
              >
                Go to dashboard
                <ArrowRight className="size-4" />
              </Link>
            ) : (
              <>
                <a
                  href={DEMO_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
                >
                  <Calendar className="size-4" />
                  Book a demo
                </a>
                <Link
                  href="/auth/login"
                  className="inline-flex items-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
                >
                  Start your first race
                  <ArrowRight className="size-4" />
                </Link>
              </>
            )}
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.02] px-7 py-3 text-sm font-medium text-white/60 hover:text-white/90 hover:border-white/20 transition-colors"
            >
              <Star className="size-4" />
              Star on GitHub
              <ExternalLink className="size-3.5 text-white/40" />
            </a>
          </div>
        </div>
      </section>

      {/* ── Footer ──────────────────────────────────────────────── */}
      <footer className="mt-auto border-t border-white/[0.06] px-8 sm:px-12 py-10">
        <div className="mx-auto max-w-[1440px] flex flex-wrap items-center justify-between gap-4 text-[11px] font-[family-name:var(--font-mono)] text-white/35">
          <div className="flex items-center gap-6">
            <span className="font-medium text-white/55">AgentClash</span>
            <span>FSL-1.1-MIT</span>
          </div>
          <div className="flex items-center gap-5">
            <Link href="/blog" className="hover:text-white/70 transition-colors">
              Blog
            </Link>
            <Link href="/team" className="hover:text-white/70 transition-colors">
              Team
            </Link>
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-white/70 transition-colors"
            >
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </main>
  );
}
