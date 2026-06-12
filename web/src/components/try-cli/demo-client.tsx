"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  Check,
  Copy,
  ExternalLink,
  KeyRound,
  LogIn,
  RotateCcw,
  Sparkles,
  TimerReset,
} from "lucide-react";
import { TryCliTerminal } from "@/components/try-cli/terminal";
import { getTryCliApiBase, tryCliPublicOrigin } from "@/lib/try-cli/config";
import type { DemoMeta, TrySession } from "@/lib/try-cli/types";

interface Props {
  slug: string;
  initialDemo?: DemoMeta | null;
}

export function TryCliDemoClient({ slug, initialDemo = null }: Props) {
  const apiBase = getTryCliApiBase();
  const [demo, setDemo] = useState<DemoMeta | null>(initialDemo);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [status, setStatus] = useState("loading");
  const [expiresAt, setExpiresAt] = useState(0);
  const [remaining, setRemaining] = useState("");
  const [low, setLow] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [tier, setTier] = useState<"anonymous" | "authenticated">("anonymous");
  const [trial, setTrial] = useState(false);
  const [trialUsed, setTrialUsed] = useState(false);

  const loginHref = `/auth/login?returnTo=${encodeURIComponent(`/try/${slug}`)}`;
  const SESSION_KEY = "trycli:session";
  const saveSession = (id: string, exp: number) => {
    try {
      localStorage.setItem(SESSION_KEY, JSON.stringify({ id, slug, expiresAt: exp }));
    } catch {
      /* ignore */
    }
  };
  const clearSaved = () => {
    try {
      localStorage.removeItem(SESSION_KEY);
    } catch {
      /* ignore */
    }
  };

  // Track the in-flight poll timer so it can be cancelled on unmount / re-poll.
  const pollTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearPoll = useCallback(() => {
    if (pollTimeoutRef.current) {
      clearTimeout(pollTimeoutRef.current);
      pollTimeoutRef.current = null;
    }
  }, []);

  const pollSession = useCallback(
    (id: string) => {
      // ~60s of polling at 1.5s intervals before giving up on a stuck sandbox.
      const MAX_ATTEMPTS = 40;
      clearPoll();
      let attempts = 0;

      const poll = async () => {
        pollTimeoutRef.current = null;
        try {
          const res = await fetch(`${apiBase}/sessions/${id}`);
          const data = (await res.json()) as TrySession & { error?: string };
          setStatus(data.status);
          if (data.tier) setTier(data.tier);
          if (typeof data.trial === "boolean") setTrial(data.trial);
          if (data.status === "error") {
            setError(data.error ?? "Sandbox failed");
            return;
          }
          if (data.status === "starting") {
            if (++attempts >= MAX_ATTEMPTS) {
              setStatus("error");
              setError("Sandbox timed out while starting. Try again.");
              return;
            }
            pollTimeoutRef.current = setTimeout(poll, 1500);
          }
        } catch {
          if (++attempts >= MAX_ATTEMPTS) {
            setStatus("error");
            setError("Lost connection to the sandbox service.");
            return;
          }
          pollTimeoutRef.current = setTimeout(poll, 1500);
        }
      };
      void poll();
    },
    [apiBase, clearPoll],
  );

  useEffect(() => clearPoll, [clearPoll]);

  const createSession = useCallback(async () => {
    setStatus("starting");
    setError(null);
    const res = await fetch(`${apiBase}/sessions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ slug }),
    });
    const data = await res.json();
    if (res.status === 403 && data.trialUsed) {
      setTrialUsed(true);
      setStatus("trial_used");
      clearSaved();
      return;
    }
    if (!res.ok) {
      setError(data.error ?? "Failed to start session");
      setStatus("error");
      return;
    }
    setSessionId(data.id);
    setExpiresAt(data.expiresAt);
    if (data.tier) setTier(data.tier);
    saveSession(data.id, data.expiresAt);
    pollSession(data.id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug, apiBase, pollSession]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const res = await fetch(`${apiBase}/demos/${slug}`);
      const d = (await res.json()) as DemoMeta & { error?: string };
      if (cancelled) return;
      if (d.error || !res.ok) {
        setError("Demo not found");
        setStatus("error");
        return;
      }
      setDemo(d);

      // Resume an existing live session for this demo on reload (so a refresh
      // doesn't burn the one free trial); otherwise start a fresh one.
      let saved: { id: string; slug: string; expiresAt: number } | null = null;
      try {
        const raw = localStorage.getItem(SESSION_KEY);
        if (raw) saved = JSON.parse(raw);
      } catch {
        /* ignore */
      }
      if (saved && saved.slug === slug && saved.expiresAt > Date.now()) {
        const r = await fetch(`${apiBase}/sessions/${saved.id}`);
        if (!cancelled && r.ok) {
          const s = (await r.json()) as TrySession;
          if (
            (s.status === "ready" || s.status === "starting") &&
            (s.expiresAt ?? 0) > Date.now()
          ) {
            setSessionId(saved.id);
            setExpiresAt(s.expiresAt);
            setStatus(s.status);
            if (s.tier) setTier(s.tier);
            if (typeof s.trial === "boolean") setTrial(s.trial);
            if (s.status === "starting") pollSession(saved.id);
            return;
          }
        }
        if (cancelled) return;
      }
      await createSession();
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug, apiBase, createSession]);

  useEffect(() => {
    if (!expiresAt) return;
    const tick = () => {
      const ms = expiresAt - Date.now();
      if (ms <= 0) {
        setRemaining("Expired");
        // Anonymous trial is over — gate to sign-in instead of a dead terminal.
        if (tier === "anonymous") {
          setTrialUsed(true);
          setStatus("trial_used");
          clearSaved();
        }
        return;
      }
      const m = Math.floor(ms / 60000);
      const s = Math.floor((ms % 60000) / 1000);
      setRemaining(`${m}:${s.toString().padStart(2, "0")}`);
      setLow(ms < 120000);
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [expiresAt, tier]);

  const reset = async () => {
    if (!sessionId) return;
    const res = await fetch(`${apiBase}/sessions/${sessionId}?action=reset`, { method: "POST" });
    const data = await res.json();
    setSessionId(data.id);
    setExpiresAt(data.expiresAt);
    setStatus(data.status);
    saveSession(data.id, data.expiresAt);
    if (data.status === "starting") pollSession(data.id);
  };

  const copyCmd = (cmd: string) => {
    void navigator.clipboard.writeText(cmd);
    setCopied(cmd);
    setTimeout(() => setCopied(null), 2000);
  };

  const publicOrigin = tryCliPublicOrigin().replace(/\/$/, "");
  const badgeMd = `[![Try on AgentClash](${publicOrigin}/api/try/badge/${slug}.svg)](${publicOrigin}/try/${slug})`;

  if (trialUsed) {
    return (
      <div className="relative flex min-h-[calc(100dvh-4rem)] items-center justify-center px-6 py-20">
        <div
          aria-hidden
          className="pointer-events-none absolute left-1/2 top-1/3 -z-10 h-[420px] w-[420px] -translate-x-1/2 rounded-full bg-[radial-gradient(circle,rgba(255,255,255,0.10),transparent_70%)] blur-2xl"
        />
        <div className="w-full max-w-md text-center">
          <div className="mx-auto mb-6 inline-flex size-12 items-center justify-center rounded-2xl border border-white/10 bg-white/[0.04]">
            <Sparkles className="size-5 text-white/80" />
          </div>
          <h1 className="font-[family-name:var(--font-display)] text-[clamp(2rem,4vw,3rem)] font-normal leading-[1.05] tracking-[-0.03em]">
            That&apos;s your free trial.
          </h1>
          <p className="mx-auto mt-4 max-w-sm text-white/55">
            Hope {demo?.name ?? "it"} felt good. Sign in with AgentClash to keep going —
            longer sessions, your own account, no limits.
          </p>
          <div className="mt-8 flex flex-col items-center gap-3">
            <a
              href={loginHref}
              className="inline-flex w-full max-w-xs items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
            >
              <LogIn className="size-4" />
              Sign in to continue
            </a>
            <Link
              href="/try"
              className="inline-flex items-center gap-1.5 text-sm text-white/50 underline-offset-4 transition-colors hover:text-white/80 hover:underline"
            >
              <ArrowLeft className="size-3.5" /> Back to all demos
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (error && !demo) {
    return (
      <div className="mx-auto max-w-lg px-6 py-32 text-center">
        <h1 className="font-[family-name:var(--font-display)] text-3xl tracking-[-0.02em]">
          Demo not found
        </h1>
        <p className="mt-3 text-white/50">{error}</p>
        <Link
          href="/try"
          className="mt-8 inline-flex items-center gap-1.5 text-sm text-white/70 underline-offset-4 hover:text-white hover:underline"
        >
          <ArrowLeft className="size-3.5" /> All demos
        </Link>
      </div>
    );
  }

  return (
    <div className="flex h-[calc(100dvh-4rem)] flex-col">
      {/* Demo toolbar */}
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-white/[0.06] px-5 py-3 sm:px-8">
        <div className="flex items-center gap-3">
          <Link
            href="/try"
            className="text-white/40 transition-colors hover:text-white/80"
            aria-label="All demos"
          >
            <ArrowLeft className="size-4" />
          </Link>
          <h1 className="font-[family-name:var(--font-display)] text-xl tracking-[-0.01em]">
            {demo?.name ?? slug}
          </h1>
          {demo?.tagline && (
            <span className="hidden text-sm text-white/40 lg:inline">{demo.tagline}</span>
          )}
          <div className="flex items-center gap-3 pl-1">
            {demo?.github && (
              <a
                href={demo.github}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 text-xs text-white/40 hover:text-white/80"
              >
                GitHub <ExternalLink className="size-3" />
              </a>
            )}
            {demo?.docs && (
              <a
                href={demo.docs}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 text-xs text-white/40 hover:text-white/80"
              >
                Docs <ExternalLink className="size-3" />
              </a>
            )}
          </div>
        </div>
        <div className="flex items-center gap-3">
          {tier === "anonymous" ? (
            <span
              className={`inline-flex items-center gap-2 rounded-full border px-3 py-1 font-[family-name:var(--font-mono)] text-2xs transition-colors ${
                low
                  ? "border-amber-500/40 bg-amber-500/10 text-amber-300"
                  : "border-emerald-500/30 bg-emerald-500/10 text-emerald-300"
              }`}
            >
              <span className="relative flex size-1.5">
                <span
                  className={`absolute inline-flex size-full animate-ping rounded-full opacity-75 ${
                    low ? "bg-amber-400" : "bg-emerald-400"
                  }`}
                />
                <span
                  className={`relative inline-flex size-1.5 rounded-full ${
                    low ? "bg-amber-400" : "bg-emerald-400"
                  }`}
                />
              </span>
              Free trial · {remaining || "—"}
            </span>
          ) : (
            <span className="inline-flex items-center gap-1.5 font-[family-name:var(--font-mono)] text-xs text-white/45">
              <TimerReset className="size-3.5" />
              {remaining || "—"}
            </span>
          )}
          <button
            type="button"
            onClick={() => void reset()}
            className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.1] bg-white/[0.03] px-2.5 py-1.5 text-xs text-white/70 transition-colors hover:border-white/20 hover:text-white"
          >
            <RotateCcw className="size-3" />
            Reset
          </button>
        </div>
      </div>

      {error && (
        <div className="border-b border-red-500/20 bg-red-500/10 px-5 py-2 text-sm text-red-300 sm:px-8">
          {error}
        </div>
      )}

      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <aside className="w-full shrink-0 space-y-7 overflow-y-auto border-b border-white/[0.06] p-5 md:w-80 md:border-b-0 md:border-r">
          {/* Free-trial card (anonymous, gateway-backed) */}
          {trial && (
            <div className="rounded-lg border border-emerald-500/25 bg-emerald-500/[0.06] p-4">
              <h2 className="text-xs font-medium uppercase tracking-wide text-emerald-300">
                Free trial · no key needed
              </h2>
              <p className="mt-2 text-xs leading-relaxed text-white/60">
                Running on AgentClash credentials for a few minutes — just run the commands
                below, no sign-in or API key. When it ends, sign in to keep going with your
                own account.
              </p>
              <a
                href={`/auth/login?returnTo=${encodeURIComponent(`/try/${slug}`)}`}
                className="mt-3 inline-flex items-center gap-1.5 rounded-md bg-white px-3 py-1.5 text-xs font-medium text-[#060606] transition-colors hover:bg-white/90"
              >
                Sign in to continue
              </a>
            </div>
          )}

          {/* BYO auth panel */}
          {demo?.auth && (
            <div className="rounded-lg border border-white/[0.1] bg-white/[0.02] p-4">
              <div className="flex items-center gap-2">
                <KeyRound className="size-3.5 text-white/70" />
                <h2 className="text-xs font-medium uppercase tracking-wide text-white/70">
                  {trial ? "Use your own account" : "Sign in"}
                  {demo.auth.provider ? ` · ${demo.auth.provider}` : ""}
                </h2>
              </div>
              <p className="mt-2 text-xs leading-relaxed text-white/50">{demo.auth.summary}</p>
              {demo.auth.steps.length > 0 && (
                <ol className="mt-3 space-y-2">
                  {demo.auth.steps.map((step, i) => (
                    <li key={step} className="flex gap-2.5 text-xs text-white/70">
                      <span className="font-[family-name:var(--font-mono)] text-white/30">
                        {String(i + 1).padStart(2, "0")}
                      </span>
                      <span className="leading-relaxed">{step}</span>
                    </li>
                  ))}
                </ol>
              )}
              {demo.auth.signupUrl && (
                <a
                  href={demo.auth.signupUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="mt-3 inline-flex items-center gap-1 text-xs text-white/70 underline-offset-4 hover:text-white hover:underline"
                >
                  Get a key <ExternalLink className="size-3" />
                </a>
              )}
            </div>
          )}

          {/* Suggested commands */}
          <div>
            <h2 className="text-xs font-medium uppercase tracking-wide text-white/40">
              Suggested commands
            </h2>
            <p className="mt-1 text-xs text-white/30">Click to copy · paste in the terminal</p>
            <ul className="mt-3 space-y-2">
              {demo?.commands.map((c) => (
                <li key={c.run}>
                  <button
                    type="button"
                    onClick={() => copyCmd(c.run)}
                    className="group w-full rounded-lg border border-white/[0.08] bg-white/[0.02] px-3 py-2.5 text-left transition-colors hover:border-white/20 hover:bg-white/[0.04]"
                  >
                    <span className="flex items-center justify-between">
                      <span className="text-sm font-medium text-white/85">{c.label}</span>
                      {copied === c.run ? (
                        <Check className="size-3.5 text-emerald-400" />
                      ) : (
                        <Copy className="size-3.5 text-white/25 transition-colors group-hover:text-white/60" />
                      )}
                    </span>
                    <code className="mt-1 block truncate font-[family-name:var(--font-mono)] text-xs text-white/40">
                      {c.run}
                    </code>
                  </button>
                </li>
              ))}
            </ul>
          </div>

          {/* Badge */}
          <div className="border-t border-white/[0.06] pt-5">
            <button
              type="button"
              onClick={() => copyCmd(badgeMd)}
              className="group flex w-full items-center justify-between text-left"
            >
              <h3 className="text-xs font-medium uppercase tracking-wide text-white/40">
                README badge
              </h3>
              {copied === badgeMd ? (
                <Check className="size-3.5 text-emerald-400" />
              ) : (
                <Copy className="size-3.5 text-white/25 transition-colors group-hover:text-white/60" />
              )}
            </button>
            <code className="mt-2 block break-all rounded-md bg-white/[0.03] p-2.5 font-[family-name:var(--font-mono)] text-2xs leading-relaxed text-white/45">
              {badgeMd}
            </code>
          </div>
        </aside>

        <main className="min-h-0 flex-1 p-4 sm:p-5">
          <TryCliTerminal sessionId={sessionId} status={status} />
        </main>
      </div>
    </div>
  );
}
