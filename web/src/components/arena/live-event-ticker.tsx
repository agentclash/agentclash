"use client";

import { useEffect, useRef } from "react";
import {
  BrainCircuit,
  Wrench,
  Terminal,
  FileCode,
  BarChart3,
  Activity,
  AlertTriangle,
} from "lucide-react";

import { cn } from "@/lib/utils";
import type {
  ArenaEventKind,
  TickerEntry,
} from "@/lib/arena/event-formatter";

const KIND_ICON: Record<
  ArenaEventKind,
  React.ComponentType<{ className?: string }>
> = {
  model: BrainCircuit,
  tool: Wrench,
  sandbox: Terminal,
  file: FileCode,
  scoring: BarChart3,
  system: Activity,
  unknown: Activity,
};

function formatClock(iso: string): string {
  const d = new Date(iso);
  const h = d.getHours().toString().padStart(2, "0");
  const m = d.getMinutes().toString().padStart(2, "0");
  const s = d.getSeconds().toString().padStart(2, "0");
  return `${h}:${m}:${s}`;
}

interface LiveEventTickerProps {
  entries: TickerEntry[];
  /** How many most-recent entries to render. */
  limit?: number;
  /** Scroll to show the newest entry automatically. */
  autoscroll?: boolean;
  className?: string;
}

export function LiveEventTicker({
  entries,
  limit = 8,
  autoscroll = true,
  className,
}: LiveEventTickerProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const recent = entries.slice(-limit);

  useEffect(() => {
    if (!autoscroll) return;
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [autoscroll, recent.length]);

  if (recent.length === 0) {
    return (
      <div
        className={cn(
          "rounded-md border border-dashed border-border/60 px-3 py-2 text-xs text-muted-foreground/70",
          className,
        )}
      >
        Waiting for activity&hellip;
      </div>
    );
  }

  return (
    <div
      ref={scrollRef}
      className={cn(
        "max-h-40 overflow-y-auto rounded-md border border-border bg-muted/20 font-[family-name:var(--font-mono)] text-xs",
        className,
      )}
    >
      <ul className="divide-y divide-border/60">
        {recent.map((entry) => {
          const Icon = KIND_ICON[entry.kind] ?? Activity;
          return (
            <li
              key={entry.id}
              className={cn(
                "flex flex-col gap-1 px-2 py-1.5 animate-in fade-in slide-in-from-bottom-1 duration-300",
                entry.error && "bg-destructive/5",
              )}
            >
              <div className="flex items-center gap-2 w-full">
                <span className="shrink-0 tabular-nums text-muted-foreground/70">
                  {formatClock(entry.occurredAt)}
                </span>
                {entry.error ? (
                  <AlertTriangle className="size-3.5 shrink-0 text-destructive" />
                ) : (
                  <Icon className="size-3.5 shrink-0 text-muted-foreground" />
                )}
                <span
                  className={cn(
                    "min-w-0 flex-1 truncate",
                    entry.error
                      ? "text-destructive"
                      : "text-foreground",
                  )}
                >
                  {entry.headline}
                </span>
                {entry.kind !== "system" || entry.headline !== "Race standings injected" ? (
                  entry.detail && (
                    <span className="hidden sm:inline truncate max-w-[40%] text-muted-foreground">
                      {entry.detail}
                    </span>
                  )
                ) : null}
              </div>
              {entry.kind === "system" && entry.headline === "Race standings injected" && entry.detail && (
                <div className="pl-[4.5rem] w-full">
                  <pre className="text-[10px] text-muted-foreground whitespace-pre-wrap break-words font-[family-name:var(--font-mono)] bg-muted/30 p-2 rounded border border-border/50">
                    {entry.detail}
                  </pre>
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
