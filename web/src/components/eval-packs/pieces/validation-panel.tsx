"use client";

// The "what's left to fix" panel. Replaces the old monospace dump of raw field
// paths with readable breadcrumbs; each issue is a button that jumps straight to
// the offending field via the builder's selection model.

import { AlertCircle, ArrowRight, CheckCircle2 } from "lucide-react";

import { fieldToSelection, humanizeFieldPath } from "../lib/humanize-errors";
import type { ValidationIssue } from "../lib/types";
import { usePackDraft } from "../use-pack-draft";

export function ValidationPanel({
  valid,
  errors,
}: {
  valid: boolean;
  errors: ValidationIssue[];
}) {
  const { select } = usePackDraft();

  if (valid) {
    return (
      <div className="flex items-center gap-2 rounded-md border border-builder-border bg-builder-surface px-3 py-2.5 text-sm text-builder-fg-muted">
        <CheckCircle2 className="size-4 text-builder-fg-subtle" />
        Ready to publish
      </div>
    );
  }

  return (
    <div className="space-y-2 rounded-md border border-builder-warn/30 bg-builder-warn-soft p-3">
      <div className="flex items-center gap-2 text-sm font-medium text-builder-warn">
        <AlertCircle className="size-4" />
        {errors.length} {errors.length === 1 ? "thing" : "things"} to fix
      </div>
      <ul className="space-y-0.5">
        {errors.map((issue, i) => {
          const target = fieldToSelection(issue.field);
          return (
            <li key={`${issue.field}-${i}`}>
              <button
                type="button"
                disabled={!target}
                onClick={() => target && select(target)}
                className="group flex w-full items-start gap-2 rounded px-2 py-1.5 text-left text-xs text-builder-fg-muted transition-colors hover:bg-builder-surface-hover hover:text-builder-fg disabled:cursor-default disabled:hover:bg-transparent"
              >
                <span className="min-w-0 flex-1">
                  <span className="text-builder-fg">{humanizeFieldPath(issue.field)}</span>
                  <span className="text-builder-fg-subtle"> — {issue.message}</span>
                </span>
                {target && (
                  <ArrowRight className="mt-0.5 size-3 shrink-0 text-builder-fg-faint opacity-0 transition-opacity group-hover:opacity-100" />
                )}
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
