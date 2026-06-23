"use client";

// A lightweight modal that lists the workspace's reusable pieces of a kind so
// the builder can reference one into the current pack ("add from library").

import { Loader2, Search, X } from "lucide-react";
import { useState } from "react";

import { controlClass } from "@/components/tools/field";
import { useApiListQuery } from "@/lib/api/swr";
import { cn } from "@/lib/utils";
import { piecesPath } from "../lib/api";
import { PIECE_KIND_META } from "../lib/draft";
import type { ChallengePiece, PieceKind } from "../lib/types";

export function LibraryPicker({
  workspaceId,
  kind,
  onPick,
  onClose,
}: {
  workspaceId: string;
  kind: PieceKind;
  onPick: (piece: ChallengePiece) => void;
  onClose: () => void;
}) {
  const { data, isLoading } = useApiListQuery<ChallengePiece>(piecesPath(workspaceId), { kind });
  const [query, setQuery] = useState("");
  const meta = PIECE_KIND_META[kind];
  const needle = query.trim().toLowerCase();
  const pieces = (data?.items ?? []).filter(
    (p) => !needle || p.name.toLowerCase().includes(needle) || p.slug.toLowerCase().includes(needle),
  );

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
          {isLoading ? (
            <div className="flex justify-center p-6">
              <Loader2 className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : pieces.length === 0 ? (
            <p className="p-6 text-center text-sm text-muted-foreground">
              No saved {meta.pluralLabel.toLowerCase()} yet. Build one and use &ldquo;Save to
              library&rdquo; to reuse it across packs.
            </p>
          ) : (
            pieces.map((piece) => (
              <button
                key={piece.id}
                type="button"
                onClick={() => onPick(piece)}
                className="flex w-full flex-col items-start rounded-md px-3 py-2 text-left transition-colors hover:bg-muted"
              >
                <span className="text-sm font-medium">{piece.name}</span>
                <span className="text-xs text-muted-foreground">
                  {piece.slug}
                  {piece.description ? ` — ${piece.description}` : ""}
                </span>
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
