"use client";

import { FormEvent, useEffect, useState } from "react";
import { ArrowRight, Check, Loader2, Star, LogIn } from "lucide-react";
import Link from "next/link";
import { useAuth } from "@workos-inc/authkit-nextjs/components";

type WaitlistStatus = "idle" | "loading" | "success" | "duplicate" | "error";

const CYCLE_WORDS = ["fastest", "smartest", "cheapest", "right"];

function useCycleWords() {
  const [index, setIndex] = useState(0);
  const [key, setKey] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setIndex((i) => {
        if (i === CYCLE_WORDS.length - 1) return i;
        setKey((k) => k + 1);
        return i + 1;
      });
    }, 1800);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (index !== CYCLE_WORDS.length - 1) return;
    const timer = setTimeout(() => {
      setIndex(0);
      setKey((k) => k + 1);
    }, 3500);
    return () => clearTimeout(timer);
  }, [index]);

  return { word: CYCLE_WORDS[index], isLast: index === CYCLE_WORDS.length - 1, key };
}

function ClashLogo({ clashKey }: { clashKey: number }) {
  return (
    <svg
      viewBox="0 0 512 512"
      className="size-14 mb-10"
      aria-label="AgentClash"
    >
      <rect width="512" height="512" fill="#060606" />
      <g key={`l-${clashKey}`} className="animate-clash-left">
        <polygon points="80,180 240,256 80,332" fill="#ffffff" opacity="0.9" />
      </g>
      <g key={`r-${clashKey}`} className="animate-clash-right">
        <polygon points="432,180 272,256 432,332" fill="#ffffff" opacity="0.5" />
      </g>
      <g key={`s-${clashKey}`} className="animate-clash-sparks">
        <line x1="256" y1="100" x2="256" y2="160" stroke="#ffffff" strokeWidth="10" strokeLinecap="round" />
        <line x1="256" y1="352" x2="256" y2="412" stroke="#ffffff" strokeWidth="10" strokeLinecap="round" />
        <line x1="192" y1="138" x2="214" y2="184" stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.8" />
        <line x1="320" y1="138" x2="298" y2="184" stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.8" />
        <line x1="192" y1="374" x2="214" y2="328" stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.8" />
        <line x1="320" y1="374" x2="298" y2="328" stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.8" />
      </g>
    </svg>
  );
}

function CyclingHeadline({ word, isLast, cycleKey }: { word: string; isLast: boolean; cycleKey: number }) {
  return (
    <h1 className="font-[family-name:var(--font-display)] text-4xl sm:text-5xl lg:text-6xl font-normal text-center tracking-[-0.025em] leading-[1.1]">
      Ship the{" "}
      <span
        key={cycleKey}
        className={`inline-block animate-word-in ${isLast ? "text-white" : "text-white/50"}`}
      >
        {word}
      </span>{" "}
      agent.
    </h1>
  );
}

export default function HomePage() {
  const cycle = useCycleWords();
  const [email, setEmail] = useState("");
  const [status, setStatus] = useState<WaitlistStatus>("idle");
  const [message, setMessage] = useState("");
  const [waitlistCount, setWaitlistCount] = useState<number | null>(null);
  const { user, loading: authLoading } = useAuth();

  useEffect(() => {
    fetch("/api/waitlist")
      .then((r) => r.json())
      .then((d) => { if (d.count) setWaitlistCount(d.count); })
      .catch(() => {});
  }, []);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!email.trim()) {
      setStatus("error");
      setMessage("Enter an email to join.");
      return;
    }
    setStatus("loading");
    setMessage("");
    try {
      const res = await fetch("/api/waitlist", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      const data = (await res.json()) as {
        duplicate?: boolean;
        error?: string;
        position?: number;
        total?: number;
      };
      if (!res.ok) {
        setStatus("error");
        setMessage(data.error || "Something went wrong.");
        return;
      }
      if (data.total) setWaitlistCount(data.total);
      setStatus(data.duplicate ? "duplicate" : "success");
      setMessage(
        data.duplicate
          ? `You're already #${data.position ?? ""} on the list.`
          : `You're #${data.position ?? ""}. Check your inbox.`,
      );
      if (!data.duplicate) setEmail("");
    } catch {
      setStatus("error");
      setMessage("Could not save. Try again.");
    }
  }

  return (
    <main>
      {/* ── Top bar ── */}
      <nav className="fixed top-0 left-0 right-0 z-50 flex items-center justify-between p-4">
        <div />
        <div className="flex items-center gap-2">
          {authLoading ? (
            <span className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.06] px-3 py-1.5">
              <span className="h-3.5 w-14 rounded bg-white/[0.08] animate-pulse" />
            </span>
          ) : user ? (
            <Link
              href="/dashboard"
              className="inline-flex items-center gap-1.5 rounded-md border border-white/20 bg-white px-3 py-1.5 text-xs font-medium text-[#060606] hover:bg-white/90 transition-colors"
            >
              Go to Dashboard
              <ArrowRight className="size-3" />
            </Link>
          ) : (
            <Link
              href="/auth/login"
              className="inline-flex items-center gap-1.5 rounded-md border border-white/15 bg-white/[0.06] px-3 py-1.5 text-xs font-medium text-white/70 hover:text-white/90 hover:bg-white/10 hover:border-white/25 transition-colors"
            >
              <LogIn className="size-3.5" />
              Sign in
            </Link>
          )}
          <a
            href="https://github.com/agentclash/agentclash"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-1.5 text-xs font-medium text-white/50 hover:text-white/80 hover:border-white/15 transition-colors"
          >
            <Star className="size-3.5" />
            Star on GitHub
          </a>
        </div>
      </nav>

      {/* ── Hero ── */}
      <section className="min-h-screen flex flex-col items-center justify-center px-6 py-16">
        <ClashLogo clashKey={cycle.key} />
        <CyclingHeadline word={cycle.word} isLast={cycle.isLast} cycleKey={cycle.key} />

        <p className="mt-7 text-center max-w-[28rem] text-[15px] sm:text-base leading-relaxed text-white/35">
          Your benchmarks are lying to you.
          <br />
          Race your models head-to-head on real tasks.
          Same tools, same constraints, scored live.
        </p>

        <form
          onSubmit={handleSubmit}
          className="mt-10 flex w-full max-w-md flex-col gap-3 sm:flex-row"
        >
          <label className="sr-only" htmlFor="waitlist-email">
            Email address
          </label>
          <input
            id="waitlist-email"
            type="email"
            autoComplete="email"
            placeholder="your awesome email"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value);
              if (status !== "idle") {
                setStatus("idle");
                setMessage("");
              }
            }}
            disabled={status === "loading"}
            className="h-11 w-full rounded-md border border-white/[0.08] bg-white/[0.03] px-4 text-sm text-white outline-none transition-colors focus:border-white/20 placeholder:text-white/18 disabled:opacity-50 disabled:cursor-not-allowed"
          />
          <button
            type="submit"
            disabled={status === "loading"}
            className="h-11 shrink-0 cursor-pointer rounded-md bg-white/90 px-5 text-sm font-medium text-[#060606] transition-colors hover:bg-white disabled:opacity-50 disabled:cursor-not-allowed inline-flex items-center justify-center gap-2"
          >
            {status === "loading" ? (
              <>
                <Loader2 className="size-4 animate-spin" />
                Joining
              </>
            ) : status === "success" || status === "duplicate" ? (
              <>
                <Check className="size-4" />
                Done
              </>
            ) : (
              <>
                Join waitlist
                <ArrowRight className="size-4" />
              </>
            )}
          </button>
        </form>

        {message && (
          <p
            className={`mt-3 text-xs ${status === "error" ? "text-red-400" : "text-white/30"}`}
          >
            {message}
          </p>
        )}

        {waitlistCount !== null && waitlistCount > 0 && (
          <p className="mt-6 text-xs text-white/20 font-[family-name:var(--font-mono)]">
            {waitlistCount} {waitlistCount === 1 ? "engineer" : "engineers"} waiting
          </p>
        )}
      </section>

      {/* ── Why ── */}
      <section className="px-6 pb-32 pt-8">
        <div className="mx-auto max-w-md text-center">
          <p className="font-[family-name:var(--font-display)] text-xl sm:text-2xl text-white/45 leading-snug">
            Static benchmarks leak. Leaderboards reward hype.
            <br />
            You ship based on someone else&apos;s score.
          </p>

          <p className="mt-8 text-[15px] sm:text-base leading-relaxed text-white/30">
            AgentClash runs your models on the same task, at the same time.
            Failures auto-convert into regression tests.
            The more you run, the smarter your evals get.
          </p>

          <p className="mt-8 font-[family-name:var(--font-mono)] text-xs text-white/20 tracking-wide uppercase">
            Head-to-head races &middot; Composite scoring &middot; Full replays &middot; Open source
          </p>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="flex items-center justify-between px-6 py-6 text-[11px] font-[family-name:var(--font-mono)] text-white/35">
        <span className="font-medium">AgentClash</span>
        <div className="flex items-center gap-4">
          <Link
            href="/blog"
            className="hover:text-white/55 transition-colors"
          >
            Blog
          </Link>
          <Link
            href="/team"
            className="inline-flex items-center gap-1 hover:text-white/55 transition-colors"
          >
            <svg viewBox="0 0 24 24" className="size-2.5" fill="currentColor"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>
            Team
          </Link>
        </div>
      </footer>
    </main>
  );
}
