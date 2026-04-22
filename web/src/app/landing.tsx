"use client";

import { FormEvent, useEffect, useState } from "react";
import { ArrowRight, Check, Loader2, LogIn, Star } from "lucide-react";
import Link from "next/link";
import { useAuth } from "@workos-inc/authkit-nextjs/components";

type WaitlistStatus = "idle" | "loading" | "success" | "duplicate" | "error";

export default function HomePage() {
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
    <main className="min-h-screen flex flex-col">
      {/* Top-right account controls — deliberately small, no logo, no nav. */}
      <div className="flex items-center justify-end gap-2 p-4">
        {authLoading ? (
          <span className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.04] px-3 py-1.5">
            <span className="h-3.5 w-14 rounded bg-white/[0.08]" />
          </span>
        ) : user ? (
          <Link
            href="/dashboard"
            className="inline-flex items-center gap-1.5 rounded-md border border-white/20 bg-white px-3 py-1.5 text-xs font-medium text-[#060606] hover:bg-white/90 transition-colors"
          >
            Go to dashboard
            <ArrowRight className="size-3" />
          </Link>
        ) : (
          <Link
            href="/auth/login"
            className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-1.5 text-xs font-medium text-white/50 hover:text-white/80 hover:border-white/15 transition-colors"
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
          GitHub
        </a>
      </div>

      {/* Hero */}
      <section className="flex-1 flex flex-col items-center justify-center px-6 pb-24">
        <h1 className="font-[family-name:var(--font-display)] text-5xl sm:text-6xl lg:text-7xl font-normal text-center tracking-[-0.025em] leading-[1.05] max-w-3xl">
          Ship the right agent.
        </h1>

        <p className="mt-8 text-center max-w-[30rem] text-[15px] sm:text-base leading-relaxed text-white/40">
          Your benchmarks are lying to you. Race your models head-to-head on
          real tasks — same tools, same constraints, scored live.
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
            placeholder="you@work.com"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value);
              if (status !== "idle") {
                setStatus("idle");
                setMessage("");
              }
            }}
            disabled={status === "loading"}
            className="h-11 w-full rounded-md border border-white/[0.08] bg-white/[0.03] px-4 text-sm text-white outline-none transition-colors focus:border-white/25 placeholder:text-white/20 disabled:opacity-50 disabled:cursor-not-allowed"
          />
          <button
            type="submit"
            disabled={status === "loading"}
            className="h-11 shrink-0 cursor-pointer rounded-md bg-white px-5 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90 disabled:opacity-50 disabled:cursor-not-allowed inline-flex items-center justify-center gap-2"
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
          <p className={`mt-3 text-xs ${status === "error" ? "text-red-400" : "text-white/35"}`}>
            {message}
          </p>
        )}

        {waitlistCount !== null && waitlistCount > 0 && (
          <p className="mt-6 text-[11px] text-white/25 font-[family-name:var(--font-mono)] uppercase tracking-wide">
            {waitlistCount} {waitlistCount === 1 ? "engineer" : "engineers"} on the list
          </p>
        )}
      </section>

      {/* Why — a single quiet beat, no feature cards. */}
      <section className="px-6 pb-32">
        <div className="mx-auto max-w-xl text-center space-y-6">
          <p className="font-[family-name:var(--font-display)] text-2xl sm:text-3xl text-white/50 leading-snug tracking-[-0.015em]">
            Static benchmarks leak. Leaderboards reward hype.
            <br />
            You ship based on someone else&apos;s score.
          </p>
          <p className="text-[15px] leading-relaxed text-white/35 max-w-md mx-auto">
            AgentClash runs your models on the same task at the same time.
            Failures convert into regression tests. The more you run, the
            sharper your evals get.
          </p>
          <p className="pt-2 font-[family-name:var(--font-mono)] text-[11px] text-white/25 tracking-wide uppercase">
            Head-to-head races &middot; Composite scoring &middot; Full replays &middot; Open source
          </p>
        </div>
      </section>

      {/* Footer */}
      <footer className="mt-auto flex items-center justify-between px-6 py-6 text-[11px] font-[family-name:var(--font-mono)] text-white/35">
        <span className="font-medium">AgentClash</span>
        <div className="flex items-center gap-4">
          <Link href="/blog" className="hover:text-white/60 transition-colors">
            Blog
          </Link>
          <Link
            href="/team"
            className="inline-flex items-center gap-1 hover:text-white/60 transition-colors"
          >
            <svg viewBox="0 0 24 24" className="size-2.5" fill="currentColor">
              <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
            </svg>
            Team
          </Link>
        </div>
      </footer>
    </main>
  );
}
