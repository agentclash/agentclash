"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowRight, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";
import { useWorkspaceReadiness } from "@/lib/workspace-readiness";

function dismissKey(workspaceId: string): string {
  return `agentclash:activation-dismissed:${workspaceId}`;
}

/**
 * Compact, dismissible setup nudge mounted in the workspace shell. Shows only
 * while the workspace cannot yet create a run, pointing at the next step.
 */
export function ActivationBanner({ workspaceId }: { workspaceId: string }) {
  const { ready, isLoading, nextStep, steps } =
    useWorkspaceReadiness(workspaceId);
  // Lazy init reads persisted dismissal once, client-side only. SSR returns
  // false, but the banner renders null until SWR data loads (post-hydration),
  // so there is no hydration mismatch.
  const [dismissed, setDismissed] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    try {
      return window.localStorage.getItem(dismissKey(workspaceId)) === "1";
    } catch {
      return false;
    }
  });

  if (isLoading || ready || dismissed || !nextStep) return null;

  const doneCount = steps.filter((s) => s.done).length;

  function handleDismiss() {
    setDismissed(true);
    try {
      window.localStorage.setItem(dismissKey(workspaceId), "1");
    } catch {
      // Non-fatal: banner just won't persist its dismissal.
    }
  }

  return (
    <div className="border-b border-white/[0.06] bg-white/[0.03] px-4 py-2">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
        <span className="font-medium text-foreground">Finish setup</span>
        <span className="tabular-nums">
          {doneCount}/{steps.length}
        </span>
        <span className="text-muted-foreground/60">·</span>
        <span>
          Next: <span className="text-foreground">{nextStep.label}</span>
        </span>
        <Link
          href={nextStep.href}
          className={cn(
            buttonVariants({ variant: "outline", size: "xs" }),
            "ml-auto",
          )}
        >
          {nextStep.cta}
          <ArrowRight data-icon="inline-end" className="size-3" />
        </Link>
        <button
          type="button"
          onClick={handleDismiss}
          aria-label="Dismiss setup banner"
          className="rounded p-1 text-muted-foreground transition-colors hover:text-foreground"
        >
          <X className="size-3.5" />
        </button>
      </div>
    </div>
  );
}
