"use client";

import { useEffect, useRef } from "react";

import {
  MAX_COMMENTARY_ENTRIES,
  type CommentaryEntry,
} from "@/hooks/use-agent-commentary";

interface RaceCommentaryProps {
  entries: CommentaryEntry[];
  isActive: boolean;
}

export function RaceCommentary({ entries, isActive }: RaceCommentaryProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const recent = entries.slice(-MAX_COMMENTARY_ENTRIES);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [recent.length]);

  return (
    <aside className="rm-booth" aria-label="Commentary booth">
      <header className="rm-booth__head">
        <div>
          <h2 className="rm-booth__title">Commentary</h2>
          <div className="rm-booth__sub">
            <span>
              {recent.length} / {MAX_COMMENTARY_ENTRIES} transmissions
            </span>
          </div>
        </div>
        <div
          className={`rm-onair${isActive ? "" : " rm-onair--off"}`}
          aria-label={isActive ? "on air" : "off air"}
        >
          <span className="rm-onair__led" />
          {isActive ? "On Air" : "Off"}
        </div>
      </header>

      {recent.length === 0 ? (
        <div className="rm-xmits--empty">
          {isActive
            ? "Standing by. Transmissions appear here as events arrive."
            : "Commentary is off."}
        </div>
      ) : (
        <div ref={scrollRef} className="rm-xmits">
          {recent.map((entry) => {
            const cls = [
              "rm-xmit",
              entry.tone === "positive" && "rm-xmit--positive",
              entry.tone === "warning" && "rm-xmit--warning",
            ]
              .filter(Boolean)
              .join(" ");
            return (
              <article key={entry.id} className={cls}>
                <div className="rm-xmit__head">
                  <span className="rm-xmit__caller" title={entry.agentLabel}>
                    {entry.agentLabel}
                  </span>
                  <span className="rm-xmit__time">
                    {formatClock(entry.occurredAt)}
                  </span>
                </div>
                <div className="rm-xmit__line">{entry.line}</div>
                {entry.detail && (
                  <div className="rm-xmit__detail">{entry.detail}</div>
                )}
              </article>
            );
          })}
        </div>
      )}
    </aside>
  );
}

function formatClock(iso: string): string {
  const d = new Date(iso);
  const h = d.getUTCHours().toString().padStart(2, "0");
  const m = d.getUTCMinutes().toString().padStart(2, "0");
  const s = d.getUTCSeconds().toString().padStart(2, "0");
  return `${h}:${m}:${s}`;
}
