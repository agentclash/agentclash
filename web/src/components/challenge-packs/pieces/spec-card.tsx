"use client";

import { AlertCircle, CheckCircle2 } from "lucide-react";

import { usePackDraft } from "../use-pack-draft";
import { SpecCardView } from "./spec-card-view";

/**
 * Builder preview pane: binds the live draft compile state to the shared
 * SpecCardView and appends a validation footer. All visual rendering lives in
 * SpecCardView so the catalog/library reuses the identical card.
 */
export function SpecCard() {
  const { state } = usePackDraft();
  const compile = state.compile;

  return (
    <SpecCardView
      card={compile?.spec_card}
      yaml={compile?.yaml}
      compiling={state.compiling}
      footer={<ValidationPanel valid={compile?.valid ?? false} errors={compile?.errors ?? []} />}
    />
  );
}

function ValidationPanel({
  valid,
  errors,
}: {
  valid: boolean;
  errors: { field: string; message: string }[];
}) {
  if (valid) {
    return (
      <div className="flex items-center gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600 dark:text-emerald-400">
        <CheckCircle2 className="size-4" /> Ready to publish
      </div>
    );
  }
  return (
    <div className="space-y-1.5 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3">
      <div className="flex items-center gap-2 text-sm font-medium text-amber-600 dark:text-amber-400">
        <AlertCircle className="size-4" /> {errors.length} issue{errors.length === 1 ? "" : "s"} to resolve
      </div>
      <ul className="space-y-1 text-xs text-muted-foreground">
        {errors.map((e, i) => (
          <li key={i}>
            <span className="font-mono text-foreground">{e.field}</span> — {e.message}
          </li>
        ))}
      </ul>
    </div>
  );
}
