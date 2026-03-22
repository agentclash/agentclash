"use client";

import { FormEvent, useState } from "react";
import { ArrowRight, Check, Loader2 } from "lucide-react";

type WaitlistStatus = "idle" | "loading" | "success" | "duplicate" | "error";

export default function HomePage() {
  const [email, setEmail] = useState("");
  const [status, setStatus] = useState<WaitlistStatus>("idle");
  const [message, setMessage] = useState("");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!email.trim()) {
      setStatus("error");
      setMessage("Enter an email to join the waitlist.");
      return;
    }

    setStatus("loading");
    setMessage("");

    try {
      const response = await fetch("/api/waitlist", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ email }),
      });

      const payload = (await response.json()) as {
        duplicate?: boolean;
        error?: string;
      };

      if (!response.ok) {
        setStatus("error");
        setMessage(payload.error || "Something went wrong. Try again.");
        return;
      }

      setStatus(payload.duplicate ? "duplicate" : "success");
      setMessage(
        payload.duplicate
          ? "You are already on the waitlist."
          : "You are in. We will reach out when the beta opens."
      );

      if (!payload.duplicate) {
        setEmail("");
      }
    } catch {
      setStatus("error");
      setMessage("Could not save your signup. Try again.");
    }
  }

  return (
    <main className="min-h-screen bg-[#050505] text-white">
      <section className="relative flex min-h-screen items-center overflow-hidden px-6 py-16 sm:px-8">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top,_rgba(255,255,255,0.08),_transparent_35%),linear-gradient(180deg,rgba(255,255,255,0.02),transparent_55%)]" />

        <div className="relative mx-auto flex w-full max-w-5xl flex-col items-center text-center">
          <div className="inline-flex items-center rounded-full border border-white/10 bg-white/[0.03] px-4 py-2 text-[10px] font-medium uppercase tracking-[0.28em] text-white/58">
            AgentClash prelaunch waitlist
          </div>

          <h1 className="mt-8 max-w-4xl text-balance font-[family-name:var(--font-display)] text-5xl font-semibold tracking-[-0.06em] sm:text-6xl lg:text-7xl">
            Know the better agent before it ships.
          </h1>

          <p className="mt-6 max-w-2xl text-pretty text-sm leading-7 text-white/62 sm:text-base">
            Compare agent systems on the same task, read the verdict, and gate
            releases with evidence instead of taste.
          </p>

          <form
            onSubmit={handleSubmit}
            className="mt-10 flex w-full max-w-2xl flex-col gap-3 sm:flex-row"
          >
            <label className="sr-only" htmlFor="waitlist-email">
              Email address
            </label>
            <input
              id="waitlist-email"
              type="email"
              autoComplete="email"
              placeholder="Email address"
              value={email}
              onChange={(event) => {
                setEmail(event.target.value);
                if (status !== "idle") {
                  setStatus("idle");
                  setMessage("");
                }
              }}
              disabled={status === "loading"}
              className="h-12 w-full rounded-full border border-white/10 bg-white/5 px-5 text-sm text-white outline-none transition focus:border-white/30 focus:ring-2 focus:ring-white/10 placeholder:text-white/38 disabled:cursor-not-allowed disabled:opacity-60"
            />
            <button
              type="submit"
              disabled={status === "loading"}
              className="inline-flex h-12 shrink-0 items-center justify-center gap-2 rounded-full bg-white px-5 text-sm font-medium text-black transition hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-70"
            >
              {status === "loading" ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  Joining
                </>
              ) : status === "success" || status === "duplicate" ? (
                <>
                  <Check className="size-4" />
                  Joined
                </>
              ) : (
                <>
                  Join waitlist
                  <ArrowRight className="size-4" />
                </>
              )}
            </button>
          </form>

          {message ? (
            <p
              className={`mt-3 text-xs ${
                status === "error" ? "text-rose-400" : "text-white/58"
              }`}
            >
              {message}
            </p>
          ) : null}
        </div>
      </section>
    </main>
  );
}
