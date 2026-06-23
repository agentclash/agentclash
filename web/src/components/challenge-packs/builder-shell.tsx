"use client";

import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { BuilderCenter } from "./editors/editor-registry";
import { PackOutline } from "./outline/pack-outline";
import { SpecCard } from "./pieces/spec-card";
import { usePackDraft } from "./use-pack-draft";

export function BuilderShell() {
  const { state, publish } = usePackDraft();
  const invalid = state.compile != null && !state.compile.valid;
  const status = state.saving
    ? "Saving…"
    : state.compiling
      ? "Updating preview…"
      : state.savedAt
        ? "Saved"
        : "";

  return (
    <div className="flex h-[calc(100vh-7rem)] min-h-[560px] flex-col overflow-hidden rounded-xl border border-border bg-card">
      <header className="flex items-center justify-between border-b border-border px-4 py-2.5">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold">
            {state.composition.pack.name || "Untitled pack"}
          </div>
          <div className="text-xs text-muted-foreground">{status}</div>
        </div>
        <Button
          size="sm"
          onClick={() => void publish()}
          disabled={state.publishing || invalid}
          title={invalid ? "Resolve the validation issues first" : undefined}
        >
          {state.publishing && <Loader2 className="size-4 animate-spin" />}
          Publish
        </Button>
      </header>
      <div className="flex min-h-0 flex-1">
        <aside className="w-64 shrink-0 border-r border-border">
          <PackOutline />
        </aside>
        <main className="min-w-0 flex-1 overflow-y-auto p-6">
          <BuilderCenter />
        </main>
        <aside className="hidden w-80 shrink-0 border-l border-border lg:block">
          <SpecCard />
        </aside>
      </div>
    </div>
  );
}
