"use client";

import type React from "react";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
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

const REPLAY_EVENTS = [
  { n: "01", type: "think", label: "reasoning through the failure" },
  { n: "02", type: "model", label: "grok-4-reasoning" },
  { n: "03", type: "tool", label: "read_file  ·  auth/session.go" },
  { n: "04", type: "observe", label: "tests failing  ·  2 / 10" },
  { n: "05", type: "think", label: "locating the timestamp bug" },
  { n: "06", type: "tool", label: "write_file  ·  auth/session.go" },
  { n: "07", type: "submit", label: "tests green  ·  10 / 10" },
];

const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

function subscribeReducedMotion(callback: () => void) {
  if (typeof window === "undefined") return () => {};
  const mq = window.matchMedia(REDUCED_MOTION_QUERY);
  mq.addEventListener("change", callback);
  return () => mq.removeEventListener("change", callback);
}

function getReducedMotion() {
  return window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

function ReplayScrubber() {
  const [activeIdx, setActiveIdx] = useState(0);
  const [isVisible, setIsVisible] = useState(false);
  const reducedMotion = useSyncExternalStore(
    subscribeReducedMotion,
    getReducedMotion,
    () => false,
  );
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!ref.current) return;
    const observer = new IntersectionObserver(
      ([entry]) => setIsVisible(entry.isIntersecting),
      { threshold: 0.25 },
    );
    observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!isVisible || reducedMotion) return;
    const interval = setInterval(() => {
      setActiveIdx((i) => (i + 1) % REPLAY_EVENTS.length);
    }, 1800);
    return () => clearInterval(interval);
  }, [isVisible, reducedMotion]);

  return (
    <div
      ref={ref}
      className="rounded-xl border border-white/[0.08] bg-white/[0.015] p-5 sm:p-6"
    >
      <div className="flex items-center justify-between pb-4 mb-4 border-b border-white/[0.06]">
        <div className="flex items-center gap-2 text-[11px] text-white/45 font-[family-name:var(--font-mono)]">
          <span className="inline-block size-1.5 rounded-full bg-[#c7ff3c]/80" />
          replay · run 7dd0c04c · grok-4
        </div>
        <div className="text-[11px] text-white/30 font-[family-name:var(--font-mono)] tabular-nums">
          {String(activeIdx + 1).padStart(2, "0")} / {REPLAY_EVENTS.length}
        </div>
      </div>

      <ol className="space-y-1">
        {REPLAY_EVENTS.map((event, i) => {
          const isActive = i === activeIdx;
          const isPast = i < activeIdx;
          return (
            <li
              key={event.n}
              className={`flex items-center gap-3 rounded-md border-l-2 px-3 py-2.5 transition-[background-color,border-color,color] duration-500 ease-out ${
                isActive
                  ? "border-[#c7ff3c] bg-white/[0.04]"
                  : "border-transparent"
              }`}
            >
              <span
                className={`w-7 font-[family-name:var(--font-mono)] text-[11px] tabular-nums ${
                  isActive
                    ? "text-white/75"
                    : isPast
                    ? "text-white/20"
                    : "text-white/35"
                }`}
              >
                {event.n}
              </span>
              <span
                className={`w-16 font-[family-name:var(--font-mono)] text-[10px] tracking-wider ${
                  isActive ? "text-white/55" : "text-white/25"
                }`}
              >
                {event.type}
              </span>
              <span
                className={`truncate text-sm ${
                  isActive
                    ? "text-white/95"
                    : isPast
                    ? "text-white/30"
                    : "text-white/50"
                }`}
              >
                {event.label}
              </span>
              {isActive && (
                <span className="ml-auto text-[10px] text-[#c7ff3c]/80 font-[family-name:var(--font-mono)]">
                  ▸
                </span>
              )}
            </li>
          );
        })}
      </ol>

      <div className="mt-5 h-[2px] w-full overflow-hidden rounded-full bg-white/[0.06]">
        <div
          className="h-full bg-[#c7ff3c]/55 transition-[width] duration-[1500ms] ease-out"
          style={{
            width: `${((activeIdx + 1) / REPLAY_EVENTS.length) * 100}%`,
          }}
        />
      </div>
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
      <circle cx="18" cy="24" r="1.3" fill="currentColor" opacity="0.6" stroke="none" />
      <circle cx="26" cy="24" r="2" fill="#c7ff3c" opacity="1" stroke="none" />
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
            <ReplayScrubber />
          </div>
        </div>
      </section>

      {/* ── Feature · Regression tests ──────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-12 md:gap-20 items-start">
          <div className="md:col-span-5">
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,4rem)]">
              Failures become your regression suite.
            </h2>
          </div>
          <div className="md:col-span-7 md:pt-4 space-y-8">
            <p className="text-lg leading-[1.6] text-white/60">
              When a model flunks a challenge, the failing trace is frozen
              into a permanent test. Next week&apos;s race replays it. The
              following month&apos;s does too.
            </p>
            <p className="text-lg leading-[1.6] text-white/60">
              Your eval suite sharpens itself with use. By the time a new
              model arrives, it walks into a track that was paved by every
              mistake the last model made.
            </p>
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
            {PROVIDERS.map(({ name, render }) => (
              <li
                key={name}
                className="group flex flex-col items-center justify-center gap-4 bg-[#060606] py-14 transition-colors hover:bg-white/[0.02]"
              >
                <div className="opacity-85 transition-opacity group-hover:opacity-100">
                  {render(40)}
                </div>
                <span className="text-sm text-white/55 transition-colors group-hover:text-white/85">
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

      {/* ── Closing CTA ─────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-40 sm:py-56">
        <div className="mx-auto max-w-[1440px]">
          <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.04em] leading-[0.95] text-[clamp(3rem,8vw,7rem)] max-w-[16ch]">
            Stop guessing.
            <br />
            <span className="text-white/40">Start racing.</span>
          </h2>
          <div className="mt-14 flex flex-wrap gap-3">
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
