"use client";

import Link from "next/link";
import { CheckCircle2, Circle, ArrowRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";
import {
  useWorkspaceReadiness,
  type ReadinessStep,
  type WorkspaceReadiness,
} from "@/lib/workspace-readiness";

interface ActivationChecklistProps {
  workspaceId: string;
  /** Pass a shared readiness result to avoid duplicate fetches. */
  readiness?: WorkspaceReadiness;
  className?: string;
}

/**
 * Full "get to your first run" checklist, shown in the runs empty state while a
 * workspace is not yet ready. Mirrors the CLI `quickstart` next-step model.
 */
export function ActivationChecklist({
  workspaceId,
  readiness,
  className,
}: ActivationChecklistProps) {
  const fallback = useWorkspaceReadiness(workspaceId);
  const { steps, nextStep } = readiness ?? fallback;
  const doneCount = steps.filter((s) => s.done).length;

  return (
    <div
      className={cn(
        "mx-auto w-full max-w-xl rounded-xl border border-border bg-white/[0.02] p-6",
        className,
      )}
    >
      <div className="mb-1 flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold text-foreground">
          Get to your first run
        </h3>
        <span className="text-xs text-muted-foreground">
          {doneCount} of {steps.length} done
        </span>
      </div>
      <p className="mb-5 text-sm text-muted-foreground">
        A few one-time steps connect your models and agents. Knock these out and
        you can race.
      </p>

      <ol className="space-y-1">
        {steps.map((step) => (
          <ChecklistRow
            key={step.key}
            step={step}
            isNext={nextStep?.key === step.key}
          />
        ))}
      </ol>
    </div>
  );
}

function ChecklistRow({
  step,
  isNext,
}: {
  step: ReadinessStep;
  isNext: boolean;
}) {
  return (
    <li
      className={cn(
        "flex items-start gap-3 rounded-lg px-3 py-2.5 transition-colors",
        isNext && "bg-white/[0.03] ring-1 ring-white/[0.06]",
      )}
    >
      <span className="mt-0.5 shrink-0">
        {step.done ? (
          <CheckCircle2 className="size-4 text-foreground" />
        ) : (
          <Circle
            className={cn(
              "size-4",
              isNext ? "text-foreground" : "text-muted-foreground/50",
            )}
          />
        )}
      </span>
      <div className="min-w-0 flex-1">
        <p
          className={cn(
            "text-sm font-medium",
            step.done
              ? "text-muted-foreground line-through decoration-muted-foreground/40"
              : "text-foreground",
          )}
        >
          {step.label}
        </p>
        {!step.done && (
          <p className="mt-0.5 text-xs text-muted-foreground">
            {step.description}
          </p>
        )}
      </div>
      {isNext && (
        <Link
          href={step.href}
          className={cn(buttonVariants({ variant: "default", size: "sm" }), "shrink-0")}
        >
          {step.cta}
          <ArrowRight data-icon="inline-end" className="size-3.5" />
        </Link>
      )}
    </li>
  );
}
