"use client";

import { Loader2 } from "lucide-react";

import { useApiQuery } from "@/lib/api/swr";
import { BuilderShell } from "./builder-shell";
import { draftPath } from "./lib/api";
import { emptyComposition } from "./lib/draft";
import type { ChallengePackDraft, Composition } from "./lib/types";
import { PackDraftProvider } from "./use-pack-draft";

/** Normalize a possibly-empty stored composition onto a complete shape. */
function toComposition(raw: unknown): Composition {
  const base = emptyComposition();
  if (!raw || typeof raw !== "object") return base;
  const r = raw as Partial<Composition>;
  return {
    ...base,
    ...r,
    pack: { ...base.pack, ...(r.pack ?? {}) },
    version: { ...base.version, ...(r.version ?? {}) },
    scorecard: { ...base.scorecard, ...(r.scorecard ?? {}) },
  };
}

export function PackBuilder({ workspaceId, draftId }: { workspaceId: string; draftId: string }) {
  const { data, error, isLoading } = useApiQuery<ChallengePackDraft>(draftPath(workspaceId, draftId));

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center text-muted-foreground">
        <Loader2 className="size-5 animate-spin" />
      </div>
    );
  }
  if (error || !data) {
    return <div className="p-8 text-sm text-destructive">Couldn&apos;t load this draft.</div>;
  }

  return (
    <PackDraftProvider
      workspaceId={workspaceId}
      draftId={draftId}
      initialComposition={toComposition(data.composition)}
    >
      <BuilderShell />
    </PackDraftProvider>
  );
}
