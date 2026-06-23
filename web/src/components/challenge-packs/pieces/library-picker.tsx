"use client";

// "Add from library" modal. Lists two sources for the chosen kind: the
// workspace's own reusable pieces (added as a reference by id) and the built-in
// starter catalog (cloned inline). Returns the chosen PieceRef to the caller.

import { Loader2, Search, X } from "lucide-react";
import { useState } from "react";

import { controlClass } from "@/components/tools/field";
import { useApiListQuery } from "@/lib/api/swr";
import { cn } from "@/lib/utils";
import { pieceLibraryPath, piecesPath } from "../lib/api";
import { PIECE_KIND_META } from "../lib/draft";
import type { ChallengePiece, PieceKind, PieceRef, StarterPiece } from "../lib/types";

export function LibraryPicker({
  workspaceId,
  kind,
  onAdd,
  onClose,
}: {
  workspaceId: string;
  kind: PieceKind;
  onAdd: (ref: PieceRef) => void;
  onClose: () => void;
}) {
  const { data: wsData, isLoading: wsLoading } = useApiListQuery<ChallengePiece>(piecesPath(workspaceId), { kind });
  const { data: starterData, isLoading: starterLoading } = useApiListQuery<StarterPiece>(pieceLibraryPath(), { kind });
  const [query, setQuery] = useState("");
  const meta = PIECE_KIND_META[kind];
  const needle = query.trim().toLowerCase();
  const match = (name: string, slug: string) =>
    !needle || name.toLowerCase().includes(needle) || slug.toLowerCase().includes(needle);
  const workspacePieces = (wsData?.items ?? []).filter((p) => match(p.name, p.slug));
  const starters = (starterData?.items ?? []).filter((p) => match(p.name, p.slug));

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <div
        className="flex max-h-[70vh] w-full max-w-lg flex-col rounded-xl border border-border bg-card shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-border px-4 py-3">
          <span className="text-sm font-semibold">Add {meta.label.toLowerCase()} from library</span>
          <button
            type="button"
            onClick={onClose}
            className="text-muted-foreground transition-colors hover:text-foreground"
            aria-label="Close"
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="border-b border-border p-3">
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <input
              className={cn(controlClass, "pl-8")}
              placeholder={`Search ${meta.pluralLabel.toLowerCase()}…`}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              autoFocus
            />
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          {wsLoading || starterLoading ? (
            <div className="flex justify-center p-6">
              <Loader2 className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <>
              <div className="mb-2">
                <div className="px-2 py-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  Your library
                </div>
                {workspacePieces.length === 0 ? (
                  <p className="px-2 py-1 text-xs text-muted-foreground/70">
                    Nothing saved yet — build a piece and use &ldquo;Save to library&rdquo;.
                  </p>
                ) : (
                  workspacePieces.map((p) => (
                    <PickerRow
                      key={p.id}
                      name={p.name}
                      subtitle={p.slug + (p.description ? ` — ${p.description}` : "")}
                      onClick={() => onAdd({ ref_id: p.id })}
                    />
                  ))
                )}
              </div>
              {starters.length > 0 && (
                <div className="mb-2">
                  <div className="px-2 py-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Starters
                  </div>
                  {starters.map((s) => (
                    <PickerRow
                      key={s.slug}
                      name={s.name}
                      subtitle={s.description}
                      onClick={() => onAdd({ inline: s.definition })}
                    />
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function PickerRow({
  name,
  subtitle,
  onClick,
}: {
  name: string;
  subtitle?: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex w-full flex-col items-start rounded-md px-3 py-2 text-left transition-colors hover:bg-muted"
    >
      <span className="text-sm font-medium">{name}</span>
      {subtitle && <span className="text-xs text-muted-foreground">{subtitle}</span>}
    </button>
  );
}
