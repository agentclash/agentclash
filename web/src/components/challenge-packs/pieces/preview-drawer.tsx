"use client";

// The live preview, as an on-demand slide-over so the editor column stays calm.
// A slim close bar sits above the shared SpecCard (which carries its own
// Preview / readable / YAML header).

import { X } from "lucide-react";
import { useEffect } from "react";

import { SpecCard } from "./spec-card";

export function PreviewDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <>
      <button
        type="button"
        aria-label="Close preview"
        onClick={onClose}
        className="fixed inset-0 z-40 bg-black/50"
      />
      <aside className="fixed inset-y-0 right-0 z-50 flex w-[min(26rem,100vw)] animate-in flex-col border-l border-builder-border bg-builder-panel slide-in-from-right-2 duration-200">
        <div className="flex items-center justify-end border-b border-builder-border px-2 py-1.5">
          <button
            type="button"
            onClick={onClose}
            aria-label="Close preview"
            className="rounded p-1 text-builder-fg-subtle transition-colors hover:bg-builder-surface hover:text-builder-fg"
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-hidden">
          <SpecCard />
        </div>
      </aside>
    </>
  );
}
