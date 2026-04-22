"use client";

import { useEffect, useRef } from "react";
import {
  AudioLines,
  AlertTriangle,
  CircleDot,
  Sparkles,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  MAX_COMMENTARY_ENTRIES,
  type CommentaryEntry,
} from "@/hooks/use-agent-commentary";

interface LiveCommentarySidebarProps {
  entries: CommentaryEntry[];
  isActive: boolean;
  className?: string;
}

function formatClock(iso: string): string {
  const d = new Date(iso);
  const h = d.getUTCHours().toString().padStart(2, "0");
  const m = d.getUTCMinutes().toString().padStart(2, "0");
  const s = d.getUTCSeconds().toString().padStart(2, "0");
  return `${h}:${m}:${s} UTC`;
}

const TONE_STYLES: Record<CommentaryEntry["tone"], string> = {
  neutral:
    "border-slate-300/70 bg-white/80 text-slate-700 dark:border-slate-700 dark:bg-slate-950/60 dark:text-slate-200",
  positive:
    "border-emerald-300/70 bg-emerald-50/80 text-emerald-800 dark:border-emerald-900/80 dark:bg-emerald-950/40 dark:text-emerald-200",
  warning:
    "border-amber-300/70 bg-amber-50/80 text-amber-900 dark:border-amber-900/80 dark:bg-amber-950/40 dark:text-amber-200",
};

export function LiveCommentarySidebar({
  entries,
  isActive,
  className,
}: LiveCommentarySidebarProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const recent = entries.slice(-MAX_COMMENTARY_ENTRIES);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [recent.length]);

  return (
    <aside
      className={cn(
        "rounded-2xl border border-amber-400/25 bg-[linear-gradient(180deg,rgba(255,251,235,0.95),rgba(255,255,255,0.95))] p-4 shadow-sm dark:border-amber-500/15 dark:bg-[linear-gradient(180deg,rgba(69,26,3,0.28),rgba(2,6,23,0.92))]",
        className,
      )}
    >
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <div className="mb-1 inline-flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-[0.18em] text-amber-700/80 dark:text-amber-300/80">
            <AudioLines className="size-3" />
            Commentary Booth
          </div>
          <h3 className="text-sm font-semibold tracking-tight">
            Live sidebar callouts
          </h3>
          <p className="mt-1 text-xs text-muted-foreground">
            A lightweight play-by-play generated from the live arena stream.
          </p>
        </div>
        <Badge variant={isActive ? "default" : "outline"}>
          {isActive ? "On Air" : "Idle"}
        </Badge>
      </div>

      {recent.length === 0 ? (
        <div className="rounded-xl border border-dashed border-amber-300/60 bg-white/60 px-3 py-4 text-sm text-muted-foreground dark:border-amber-900/70 dark:bg-slate-950/40">
          <div className="mb-1 flex items-center gap-2 font-medium text-foreground">
            <Sparkles className="size-4 text-amber-600 dark:text-amber-300" />
            Waiting for the next call
          </div>
          <p className="text-xs leading-5">
            Turn commentary on during a live run to watch the sidebar narrate
            model calls, tool usage, scoring beats, and finishes.
          </p>
        </div>
      ) : (
        <div
          ref={scrollRef}
          className="max-h-[34rem] space-y-2 overflow-y-auto pr-1"
        >
          {recent.map((entry) => (
            <article
              key={entry.id}
              className={cn(
                "rounded-xl border px-3 py-2.5 backdrop-blur-sm",
                TONE_STYLES[entry.tone],
              )}
            >
              <div className="mb-1 flex items-center gap-2 text-[11px] uppercase tracking-[0.12em] text-current/70">
                {entry.tone === "warning" ? (
                  <AlertTriangle className="size-3.5" />
                ) : (
                  <CircleDot className="size-3.5" />
                )}
                <span className="truncate">{entry.agentLabel}</span>
                <span className="ml-auto shrink-0 tabular-nums">
                  {formatClock(entry.occurredAt)}
                </span>
              </div>
              <p className="text-sm leading-5 text-foreground dark:text-inherit">
                {entry.line}
              </p>
              {entry.detail && (
                <p className="mt-1 text-xs leading-5 text-current/75">
                  {entry.detail}
                </p>
              )}
            </article>
          ))}
        </div>
      )}
    </aside>
  );
}
