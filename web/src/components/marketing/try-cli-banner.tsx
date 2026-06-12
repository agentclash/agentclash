"use client";

import type React from "react";
import { useSyncExternalStore } from "react";
import Link from "next/link";
import { ArrowRight, X } from "lucide-react";
import { Anthropic, Moonshot, OpenAI, Qwen, XAI } from "@lobehub/icons";

const DISMISS_KEY = "ac-try-banner-dismissed";

const listeners = new Set<() => void>();

function subscribe(cb: () => void) {
  listeners.add(cb);
  window.addEventListener("storage", cb);
  return () => {
    listeners.delete(cb);
    window.removeEventListener("storage", cb);
  };
}

function emit() {
  for (const cb of listeners) cb();
}

function getSnapshot() {
  return localStorage.getItem(DISMISS_KEY) === "1";
}

// Hidden during SSR; revealed on the client unless previously dismissed. Using
// useSyncExternalStore keeps server/client renders consistent (no mismatch).
function getServerSnapshot() {
  return true;
}

const LOGOS: { name: string; render: (size: number) => React.ReactNode }[] = [
  { name: "Claude Code", render: (s) => <Anthropic size={s} color="#D97757" /> },
  { name: "Codex", render: (s) => <OpenAI size={s} color="#74AA9C" /> },
  { name: "Grok", render: (s) => <XAI size={s} color="#FFFFFF" /> },
  { name: "Qwen3 Coder", render: (s) => <Qwen.Color size={s} /> },
  { name: "Kimi K2", render: (s) => <Moonshot size={s} color="#FFFFFF" /> },
];

export function TryCliBanner() {
  const dismissed = useSyncExternalStore(
    subscribe,
    getSnapshot,
    getServerSnapshot,
  );

  if (dismissed) return null;

  const dismiss = () => {
    localStorage.setItem(DISMISS_KEY, "1");
    emit();
  };

  return (
    <div className="relative border-b border-white/[0.06] bg-white/[0.02]">
      <Link
        href="/try"
        className="group mx-auto flex max-w-[1440px] flex-wrap items-center justify-center gap-x-3 gap-y-1.5 px-12 sm:px-14 py-2.5 text-xs sm:text-sm"
      >
        <span className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.18em] text-white/35">
          New
        </span>
        <span className="text-white/70">
          Run your favorite CLIs in the browser
          <span className="hidden text-white/40 sm:inline">
            {" "}
            — zero install, no signup
          </span>
        </span>
        <span className="hidden items-center gap-2 sm:flex" aria-hidden>
          {LOGOS.map((l) => (
            <span
              key={l.name}
              title={l.name}
              className="opacity-55 transition-opacity group-hover:opacity-100"
            >
              {l.render(15)}
            </span>
          ))}
        </span>
        <span className="inline-flex items-center gap-1 font-medium text-white/85 transition-colors group-hover:text-white">
          Try it
          <ArrowRight className="size-3.5 transition-transform group-hover:translate-x-0.5" />
        </span>
      </Link>
      <button
        type="button"
        onClick={dismiss}
        aria-label="Dismiss"
        className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded p-1 text-white/30 transition-colors hover:text-white/70"
      >
        <X className="size-3.5" />
      </button>
    </div>
  );
}
