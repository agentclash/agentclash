"use client";

import { AlertCircle, Eye, Loader2 } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { BuilderCenter } from "./editors/editor-registry";
import { PackOutline } from "./outline/pack-outline";
import { PreviewDrawer } from "./pieces/preview-drawer";
import { usePackDraft } from "./use-pack-draft";

export function BuilderShell() {
  const { state, publish } = usePackDraft();
  const invalid = state.compile != null && !state.compile.valid;
  const issueCount = state.compile?.errors?.length ?? 0;
  const status = state.saving
    ? "Saving…"
    : state.compiling
      ? "Updating preview…"
      : state.savedAt
        ? "Saved"
        : "";

  // The preview is an on-demand slide-over (the "N to fix" pill and the rail's
  // issue dots keep validation discoverable while it's closed).
  const [previewOpen, setPreviewOpen] = useState(false);

  return (
    <div className="flex h-[calc(100dvh-7rem)] min-h-[560px] flex-col">
      <header className="flex items-center justify-between gap-3 border-b border-builder-border pb-3">
        <div className="min-w-0">
          <h1 className="truncate font-[family-name:var(--font-display)] text-xl leading-tight text-builder-fg">
            {state.composition.pack.name || "Untitled pack"}
          </h1>
          <div className="mt-1 flex items-center gap-1.5 text-xs text-builder-fg-subtle">
            {(state.saving || state.compiling) && <Loader2 className="size-3 animate-spin" />}
            {status}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {invalid && (
            <button
              type="button"
              onClick={() => setPreviewOpen(true)}
              className="flex items-center gap-1.5 rounded-md border border-builder-warn/30 bg-builder-warn-soft px-2.5 py-1 text-xs font-medium text-builder-warn transition-colors hover:bg-builder-warn/15"
            >
              <AlertCircle className="size-3.5" />
              {issueCount} to fix
            </button>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPreviewOpen((v) => !v)}
            aria-pressed={previewOpen}
            className={cn(
              "border-builder-border bg-transparent text-builder-fg-muted hover:bg-builder-surface hover:text-builder-fg",
              previewOpen && "bg-builder-surface text-builder-fg",
            )}
          >
            <Eye className="size-4" />
            Preview
          </Button>
          <Button
            size="sm"
            onClick={() => void publish()}
            disabled={state.publishing || invalid}
            title={invalid ? "Resolve the issues first" : undefined}
          >
            {state.publishing && <Loader2 className="size-4 animate-spin" />}
            Publish
          </Button>
        </div>
      </header>

      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <aside className="shrink-0 overflow-y-auto border-b border-builder-border max-md:max-h-44 md:w-56 md:border-r md:border-b-0">
          <PackOutline />
        </aside>
        <main className="min-w-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-[680px] px-6 py-8 lg:px-10">
            <BuilderCenter />
          </div>
        </main>
        <PreviewDrawer open={previewOpen} onClose={() => setPreviewOpen(false)} />
      </div>
    </div>
  );
}
