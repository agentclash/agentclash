"use client";

import {VibeReturn} from "./vibe-return";
import { Loader2 } from "lucide-react";

import { useApiQuery } from "@/lib/api/swr";
import { BuilderShell } from "./builder-shell";
import { draftPath } from "./lib/api";
import { emptyComposition, isDraftEditable } from "./lib/draft";
import type { ChallengePackDraft, Composition } from "./lib/types";
import { PublishedDraftNotice } from "./published-draft-notice";
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

export function PackBuilder({ workspaceId, draftId, returnTo, returnTitle }: { workspaceId: string; draftId: string;returnTo?:string;returnTitle?:string }) {
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
  // A published draft is final. Do not mount the editor, because its
  // autosave would send changes that the server refuses with HTTP 409.
  if (!isDraftEditable(data)) {
    return (
      <>
        <VibeReturn path={returnTo} draft={draftId} title={returnTitle} />
        <PublishedDraftNotice workspaceId={workspaceId} draft={data} />
      </>
    );
  }

  return (
    <PackDraftProvider
      workspaceId={workspaceId}
      draftId={draftId}
      initialComposition={toComposition(data.composition)}
    >
      <VibeReturn path={returnTo} draft={draftId} title={returnTitle}/><BuilderShell />
    </PackDraftProvider>
  );
}
