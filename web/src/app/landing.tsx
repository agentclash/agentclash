"use client";

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

const PROVIDERS = [
  { name: "OpenAI", Icon: OpenAI },
  { name: "Anthropic", Icon: Anthropic },
  { name: "Gemini", Icon: Gemini },
  { name: "xAI", Icon: XAI },
  { name: "Mistral", Icon: Mistral },
  { name: "OpenRouter", Icon: OpenRouter },
];

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
            <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-10">
              Open source race engine &middot; FSL-1.1-MIT
            </p>

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
            <ClashMark
              animated
              className="w-full max-w-[360px] aspect-square"
            />
          </div>
        </div>
      </section>

      {/* ── Why we built this ───────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-12">
            Why we built this
          </p>
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
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-10">
            How it works
          </p>
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
                  <p className="mt-10 font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.22em] text-white/35">
                    Step {step.n} / 03
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
        <div className="mx-auto max-w-[1440px]">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-10">
            Every step, on record
          </p>
          <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4.5rem)] max-w-[22ch]">
            Scrub the replay. See exactly where it got stuck.
          </h2>
          <p className="mt-10 max-w-[52ch] text-lg leading-[1.6] text-white/55">
            Every think, every tool call, every observation is captured.
            Step back to the moment a model went sideways — the prompt it
            saw, the output it produced, the state it worked from. No more
            guessing why one model won and another flunked.
          </p>
        </div>
      </section>

      {/* ── Feature · Regression tests ──────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-12 md:gap-20 items-start">
          <div className="md:col-span-5">
            <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-8">
              The flywheel
            </p>
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
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-10">
            Works with
          </p>
          <div className="flex flex-col gap-10 md:flex-row md:items-end md:justify-between md:gap-16">
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.5rem,6vw,5.5rem)] max-w-[20ch]">
              Six providers.
              <br />
              <span className="text-white/40">One finish line.</span>
            </h2>
            <p className="max-w-[40ch] text-base leading-[1.6] text-white/50">
              Normalised tool-calls, normalised errors, same scoring rules.
              Drop in a new provider without rewriting the scoreboard.
            </p>
          </div>

          <ul className="mt-20 grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-px border-y border-white/[0.06] bg-white/[0.06]">
            {PROVIDERS.map(({ name, Icon }) => (
              <li
                key={name}
                className="group flex flex-col items-center justify-center gap-4 bg-[#060606] py-14 transition-colors hover:bg-white/[0.015]"
              >
                <Icon
                  size={40}
                  className="text-white/55 transition-colors group-hover:text-white/95"
                />
                <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.22em] text-white/35 transition-colors group-hover:text-white/65">
                  {name}
                </span>
              </li>
            ))}
          </ul>
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
