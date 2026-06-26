import { Check, Cpu, PenLine, Sparkles, Wrench } from "lucide-react";
import type { ComponentType } from "react";

import type { TryoutCoaching, TryoutCoachingKind, TryoutCoachingSuggestion } from "@/lib/api/types";
import { cn } from "@/lib/utils";

const MICRO = "font-mono text-2xs uppercase tracking-[0.22em]";

/*
 * The coach. After a run is graded, the worker turns the judge's verdict +
 * reasoning + trace into specific, applyable fixes (mostly prompt edits). This
 * renders them with one-click Apply — the bridge from "it failed" to "here's
 * how to make it pass", which is the whole point of running an eval.
 */

const KIND_ICON: Record<TryoutCoachingKind, ComponentType<{ className?: string }>> = {
  prompt: PenLine,
  tool: Wrench,
  model: Cpu,
};

const KIND_LABEL: Record<TryoutCoachingKind, string> = {
  prompt: "Prompt",
  tool: "Tools",
  model: "Model",
};

function canApply(suggestion: TryoutCoachingSuggestion): boolean {
  if (suggestion.kind === "prompt") return Boolean(suggestion.proposed_instructions?.trim());
  if (suggestion.kind === "tool") return (suggestion.add_tool_slugs?.length ?? 0) > 0;
  return false;
}

export function CoachCard({
  coaching,
  appliedIds,
  onApply,
}: {
  coaching: TryoutCoaching;
  appliedIds?: string[];
  onApply?: (suggestion: TryoutCoachingSuggestion) => void;
}) {
  if (!coaching.suggestions || coaching.suggestions.length === 0) return null;
  const applied = new Set(appliedIds ?? []);

  return (
    <div className="rounded-lg border border-white/12 bg-white/[0.02]">
      <div className="flex items-center gap-2.5 border-b border-white/[0.07] px-5 py-3.5">
        <Sparkles className="size-4 text-white/70" />
        <div>
          <p className="text-sm font-medium text-white/90">Make it better</p>
          <p className={cn(MICRO, "mt-0.5 text-white/35")}>What the eval says to fix</p>
        </div>
      </div>

      <ul className="divide-y divide-white/[0.06]">
        {coaching.suggestions.map((suggestion) => {
          const Icon = KIND_ICON[suggestion.kind];
          const isApplied = applied.has(suggestion.id);
          const applyable = canApply(suggestion);
          return (
            <li key={suggestion.id} className="flex gap-3 px-5 py-4">
              <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full border border-white/12 text-white/55">
                <Icon className="size-3.5" />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex items-baseline justify-between gap-3">
                  <p className="text-sm leading-6 text-white/85">{suggestion.title}</p>
                  <span className={cn(MICRO, "shrink-0 text-white/30")}>
                    {KIND_LABEL[suggestion.kind]}
                  </span>
                </div>
                <p className="mt-1 text-xs leading-5 text-white/45">{suggestion.detail}</p>

                {onApply && applyable ? (
                  <button
                    type="button"
                    onClick={() => onApply(suggestion)}
                    disabled={isApplied}
                    className={cn(
                      "mt-2.5 inline-flex h-7 items-center gap-1.5 rounded-sm border px-2.5 text-xs transition",
                      isApplied
                        ? "border-white/15 text-white/45"
                        : "border-white/25 text-white/80 hover:border-white/45 hover:text-white",
                    )}
                  >
                    {isApplied ? (
                      <>
                        <Check className="size-3.5" strokeWidth={2.5} />
                        Applied
                      </>
                    ) : suggestion.kind === "prompt" ? (
                      "Apply to prompt"
                    ) : (
                      "Add to agent"
                    )}
                  </button>
                ) : null}
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
