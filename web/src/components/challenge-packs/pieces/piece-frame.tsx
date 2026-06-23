"use client";

// Wraps a selected piece: a library-referenced piece renders read-only (with
// "edit a copy" to detach it into an editable inline piece); an inline piece
// renders its dedicated editor plus a "Save to library" action to promote it
// into the reusable workspace library.

import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { Library, Loader2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/errors";
import { useApiQuery } from "@/lib/api/swr";
import { PIECE_EDITORS } from "../editors/piece-editors";
import { createPiece, piecePath } from "../lib/api";
import { pieceRefs, updatePieceRef } from "../lib/draft";
import type { ChallengePiece, PieceKind } from "../lib/types";
import { usePackDraft } from "../use-pack-draft";

export function PieceFrame({ kind, index }: { kind: PieceKind; index: number }) {
  const { state } = usePackDraft();
  const ref = pieceRefs(state.composition, kind)[index];

  if (ref?.ref_id && !ref.inline) {
    return <ReferencedPieceView kind={kind} index={index} pieceId={ref.ref_id} />;
  }

  const Editor = PIECE_EDITORS[kind];
  return (
    <div>
      <PieceToolbar kind={kind} index={index} />
      <Editor index={index} />
    </div>
  );
}

function slugify(value: string): string {
  return (
    value
      .toLowerCase()
      .trim()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "") || "piece"
  );
}

function PieceToolbar({ kind, index }: { kind: PieceKind; index: number }) {
  const { state, workspaceId } = usePackDraft();
  const { getAccessToken } = useAccessToken();
  const [saving, setSaving] = useState(false);
  const def = (pieceRefs(state.composition, kind)[index]?.inline ?? {}) as Record<string, unknown>;

  const save = async () => {
    const key = (def.key as string) || (def.name as string) || `${kind}-${index + 1}`;
    setSaving(true);
    try {
      const token = await getAccessToken();
      await createPiece(token, workspaceId, {
        kind,
        slug: slugify(key),
        name: (def.title as string) || (def.name as string) || key,
        definition: def,
      });
      toast.success("Saved to library — reuse it from the library picker");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't save to library");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mb-4 flex justify-end">
      <Button size="xs" variant="outline" onClick={save} disabled={saving}>
        {saving ? <Loader2 className="size-3.5 animate-spin" /> : <Library className="size-3.5" />}
        Save to library
      </Button>
    </div>
  );
}

function ReferencedPieceView({
  kind,
  index,
  pieceId,
}: {
  kind: PieceKind;
  index: number;
  pieceId: string;
}) {
  const { workspaceId, update } = usePackDraft();
  const { data, isLoading } = useApiQuery<ChallengePiece>(piecePath(workspaceId, pieceId));

  const detach = () =>
    update((c) =>
      updatePieceRef(c, kind, index, { inline: (data?.definition ?? {}) as Record<string, unknown> }),
    );

  return (
    <div className="max-w-2xl space-y-4">
      <div className="flex items-center gap-2">
        <Library className="size-4 text-muted-foreground" />
        <h2 className="text-base font-semibold">{data?.name ?? "Library piece"}</h2>
      </div>
      <p className="text-sm text-muted-foreground">
        Referenced from your workspace library. Edit it in the library to update every pack that uses
        it, or edit a copy to customize it just for this pack.
      </p>
      {isLoading ? (
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      ) : (
        <pre className="overflow-auto rounded-lg border border-border bg-muted/40 p-3 text-xs">
          {JSON.stringify(data?.definition ?? {}, null, 2)}
        </pre>
      )}
      <Button size="sm" variant="outline" onClick={detach} disabled={isLoading}>
        Edit a copy
      </Button>
    </div>
  );
}
