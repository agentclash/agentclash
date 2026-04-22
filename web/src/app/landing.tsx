"use client";

import { useState } from "react";
import Link from "next/link";
import { useAuth } from "@workos-inc/authkit-nextjs/components";
import { ArrowRight, Calendar, ExternalLink, LogIn, Star } from "lucide-react";

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
  "OpenAI",
  "Anthropic",
  "Gemini",
  "xAI",
  "Mistral",
  "OpenRouter",
];

function ProductShot({
  src,
  alt,
  caption,
  aspect = "16 / 10",
}: {
  src: string;
  alt: string;
  caption: string;
  aspect?: string;
}) {
  const [loaded, setLoaded] = useState(false);
  const [errored, setErrored] = useState(false);

  return (
    <figure
      className="relative w-full overflow-hidden rounded-xl border border-white/10 bg-white/[0.02]"
      style={{ aspectRatio: aspect }}
    >
      {!errored && (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={src}
          alt={alt}
          onLoad={() => setLoaded(true)}
          onError={() => setErrored(true)}
          className={`absolute inset-0 h-full w-full object-cover ${loaded ? "opacity-100" : "opacity-0"}`}
        />
      )}
      {(!loaded || errored) && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 p-8 text-center">
          <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.18em] text-white/30">
            drop in · {src}
          </span>
          <span className="max-w-md font-[family-name:var(--font-display)] text-xl leading-snug text-white/60">
            {caption}
          </span>
        </div>
      )}
    </figure>
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

      {/* ── Hero product shot (race arena) ──────────────────────── */}
      <section className="px-4 sm:px-8 pb-32 sm:pb-48">
        <div className="mx-auto max-w-[1600px]">
          <ProductShot
            src="/product/race-arena.png"
            alt="AgentClash Race Arena — six AI models racing head-to-head on the same challenge, with live step counts, tool calls, and composite scores"
            caption="Race Arena — six models racing head-to-head, live scoring, streaming output, lap timeline across the top."
            aspect="16 / 10"
          />
          <p className="mt-6 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/30 text-center">
            Live run view &middot; same challenge, six agents, one leader
          </p>
        </div>
      </section>

      {/* ── Problem ─────────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-12">
            The problem
          </p>
          <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.5rem,6vw,5.5rem)] max-w-[22ch]">
            Static benchmarks leak. Leaderboards reward hype.
          </h2>
          <p className="mt-12 max-w-[52ch] text-lg leading-[1.55] text-white/55">
            Your workload is not MMLU. It is your codebase, your schema, your
            broken auth server, your three-month-old ticket. The only honest
            way to pick a model is to run it against the same task you&apos;d
            pay it to do — next to every other model you&apos;re considering
            — and watch what happens.
          </p>
        </div>
      </section>

      {/* ── How it works ────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-12">
            How it works
          </p>
          <div className="grid gap-16 md:grid-cols-3 md:gap-12">
            {[
              {
                n: "01",
                title: "Pick a challenge",
                body:
                  "Write your own or pull from the library. Real tasks — a broken auth server, a SQL bug, a spec to implement — not trivia.",
              },
              {
                n: "02",
                title: "Pick your models",
                body:
                  "Line up six or eight contestants across providers. Same tool policy, same time budget, same starting state.",
              },
              {
                n: "03",
                title: "Watch them race",
                body:
                  "Live scoring as they work. Composite metric across completion, speed, token efficiency, and tool strategy.",
              },
            ].map((step) => (
              <div key={step.n} className="space-y-6">
                <div className="font-[family-name:var(--font-mono)] text-xs text-white/30 tracking-[0.18em]">
                  {step.n}
                </div>
                <h3 className="font-[family-name:var(--font-display)] text-3xl sm:text-4xl tracking-[-0.02em] leading-[1.1] text-white/90">
                  {step.title}
                </h3>
                <p className="text-base leading-[1.6] text-white/55">
                  {step.body}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Feature · Replay ────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-4 sm:px-8 py-32 sm:py-48">
        <div className="mx-auto max-w-[1600px]">
          <div className="px-4 sm:px-4 mb-16">
            <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-10">
              Every step, on record
            </p>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4.5rem)] max-w-[22ch]">
              Scrub the replay. See exactly where it got stuck.
            </h2>
            <p className="mt-10 max-w-[48ch] text-lg leading-[1.55] text-white/55">
              Every think, every tool call, every observation is captured.
              Step back to the moment a model went sideways — the prompt it
              saw, the output it produced, the state it worked from.
            </p>
          </div>

          <ProductShot
            src="/product/replay-scrubber.png"
            alt="AgentClash Replay — step-by-step playback of an agent's run, showing its reasoning, tool calls, and model state at each step"
            caption="Replay scrubber — step through any finished agent, frame by frame."
            aspect="16 / 9"
          />
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
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-24 sm:py-32">
        <div className="mx-auto max-w-[1440px]">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35 mb-10">
            Works with
          </p>
          <div className="flex flex-wrap gap-2">
            {PROVIDERS.map((p) => (
              <span
                key={p}
                className="inline-flex items-center rounded-md border border-white/[0.08] bg-white/[0.03] px-4 py-2 text-sm text-white/70"
              >
                {p}
              </span>
            ))}
          </div>
          <p className="mt-8 max-w-[56ch] text-base leading-[1.6] text-white/45">
            Normalised tool-calls, normalised errors, same scoring rules. Drop
            in a new provider without rewriting the scoreboard.
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
