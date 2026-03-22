"use client";

import { FormEvent, useEffect, useState } from "react";
import { ArrowRight, Check, Github, Loader2 } from "lucide-react";

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
      };
      if (!res.ok) {
        setStatus("error");
        setMessage(data.error || "Something went wrong.");
        return;
      }
      setStatus(data.duplicate ? "duplicate" : "success");
      setMessage(
        data.duplicate
          ? "You're already on the list."
          : "You're in. We'll be in touch.",
      );
      if (!data.duplicate) setEmail("");
    } catch {
      setStatus("error");
      setMessage("Could not save. Try again.");
    }
  }

  return (
    <main>
      {/* ── Hero ── */}
      <section className="min-h-screen flex flex-col items-center justify-center px-6 py-16">
        <ClashLogo clashKey={cycle.key} />
        <CyclingHeadline word={cycle.word} isLast={cycle.isLast} cycleKey={cycle.key} />

        <p className="mt-7 text-center max-w-[28rem] text-[15px] sm:text-base leading-relaxed text-white/35">
          Opensource race engine. Pit your models against each other
          on real tasks. Same tools, same constraints, scored
          live&nbsp;&mdash; not benchmarks, not vibes.
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
      </section>

      {/* ── Why ── */}
      <section className="px-6 pb-32 pt-8">
        <div className="mx-auto max-w-lg text-center">
          <h2 className="font-[family-name:var(--font-display)] text-3xl sm:text-4xl tracking-[-0.02em] leading-[1.15] text-white/70">
            Benchmarks are gamed.
            <br />
            You&apos;re still guessing.
          </h2>

          <p className="mt-7 text-[15px] sm:text-base leading-relaxed text-white/30 max-w-md mx-auto">
            Static test sets leak. Crowd-voted rankings reward hype,
            not capability. You test agents in isolation, one at a time,
            and ship based on someone else&apos;s score&nbsp;&mdash; not yours.
          </p>

          <p className="mt-5 text-[15px] sm:text-base leading-relaxed text-white/30 max-w-md mx-auto">
            AgentClash puts your models on the same real task, at the same
            time. Scored live on completion, speed, token efficiency, and
            tool strategy. Step-by-step replays show exactly why one agent
            won and another didn&apos;t.
          </p>

          <p className="mt-5 text-[15px] sm:text-base leading-relaxed text-white/30 max-w-md mx-auto">
            Every failure gets captured, classified, and turned into a
            regression test&nbsp;&mdash; automatically. The more you run,
            the smarter your eval suite gets. A data flywheel that no
            static benchmark can match.
          </p>

          <p className="mt-9 font-[family-name:var(--font-display)] text-xl sm:text-2xl text-white/45 leading-snug">
            Head-to-head races. Composite scoring.
            <br />
            Full replays. Failure-to-eval flywheel.
            <br />
            Open source.
          </p>

          <p className="mt-9 text-sm text-white/30 max-w-xs mx-auto leading-relaxed">
            Ship with evidence, not instinct.
          </p>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="flex items-center justify-between px-6 py-6 text-[11px] font-[family-name:var(--font-mono)] text-white/35">
        <span className="font-medium">AgentClash</span>
        <a
          href="https://github.com/Atharva-Kanherkar/agentclash"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 hover:text-white/55 transition-colors"
        >
          <Github className="size-3.5" />
          GitHub
        </a>
      </footer>
    </main>
  );
}
