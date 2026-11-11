"use client";

import { Lock } from "lucide-react";
import Link from "next/link";

import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import { EditPackInBuilderButton } from "./edit-pack-button";
import type { ChallengePackDraft } from "./lib/types";

/**
 * Shows in place of the builder when a draft is final. The server refuses an
 * edit or a second publish of a published draft with HTTP 409, so the builder
 * must not start its autosave. To change the pack, the user opens the
 * published version as a new draft (edit-in-builder).
 */
export function PublishedDraftNotice({
  workspaceId,
  draft,
}: {
  workspaceId: string;
  draft: ChallengePackDraft;
}) {
  const versionId = draft.last_published_version_id;
  const packId = draft.challenge_pack_id;

  return (
    <div className="flex flex-col items-center justify-center px-6 py-16 text-center">
      <Lock className="mb-4 size-6 text-muted-foreground" aria-hidden />
      <h2 className="text-sm font-medium text-foreground">This draft is published</h2>
      <p className="mt-1 max-w-md text-sm text-muted-foreground">
        A published draft is final, so you cannot change {draft.name ? <strong>{draft.name}</strong> : "it"} here.
        {versionId
          ? " To change the pack, open the published version as a new draft."
          : " Its published version is no longer available."}
      </p>
      <div className="mt-4 flex items-center gap-2">
        {versionId ? (
          <EditPackInBuilderButton workspaceId={workspaceId} versionId={versionId} packName={draft.name} />
        ) : null}
        {packId ? (
          <Link
            href={`/workspaces/${workspaceId}/challenge-packs/${packId}`}
            className={cn(buttonVariants({ variant: "ghost", size: "sm" }))}
          >
            Open pack
          </Link>
        ) : null}
      </div>
    </div>
  );
}
