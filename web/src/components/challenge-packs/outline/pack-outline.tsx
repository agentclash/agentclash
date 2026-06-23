"use client";

import { LayoutGrid, Library, Plus, SlidersHorizontal } from "lucide-react";
import { useState } from "react";

import { cn } from "@/lib/utils";
import { addPieceRef, PIECE_KINDS, PIECE_KIND_META, pieceRefs } from "../lib/draft";
import type { ChallengePiece, PieceDefinition, PieceKind, PieceRef } from "../lib/types";
import { LibraryPicker } from "../pieces/library-picker";
import { type BuilderSelection, usePackDraft } from "../use-pack-draft";

function defaultPieceInline(kind: PieceKind, n: number): PieceDefinition {
  switch (kind) {
    case "challenge":
      return { key: `challenge-${n}`, title: "", category: "", difficulty: "medium" };
    case "input_set":
      return { key: n === 1 ? "default" : `set-${n}`, name: n === 1 ? "Default" : `Set ${n}`, cases: [] };
    case "validator":
      return { key: `validator-${n}`, type: "contains", target: "final_output", expected_from: "" };
    case "judge":
      return { key: `judge-${n}`, mode: "rubric", model: "claude-haiku-4-5-20251001" };
  }
}

function pieceLabel(kind: PieceKind, ref: PieceRef, index: number): string {
  const inline = ref.inline as Record<string, unknown> | undefined;
  if (ref.ref_id && !inline) return `Library piece ${index + 1}`;
  const title = (inline?.title as string) || (inline?.name as string) || (inline?.key as string);
  return title || `${PIECE_KIND_META[kind].label} ${index + 1}`;
}

function isSelected(selection: BuilderSelection, target: BuilderSelection): boolean {
  if (selection.section !== target.section) return false;
  if (selection.section === "piece" && target.section === "piece") {
    return selection.kind === target.kind && selection.index === target.index;
  }
  return true;
}

export function PackOutline() {
  const { state, workspaceId, select, update } = usePackDraft();
  const { composition, selection } = state;
  const [pickerKind, setPickerKind] = useState<PieceKind | null>(null);

  const addPiece = (kind: PieceKind) => {
    const count = pieceRefs(composition, kind).length;
    const inline = defaultPieceInline(kind, count + 1);
    update((c) => addPieceRef(c, kind, { inline }));
    select({ section: "piece", kind, index: count });
  };

  const addFromLibrary = (kind: PieceKind, piece: ChallengePiece) => {
    const count = pieceRefs(composition, kind).length;
    update((c) => addPieceRef(c, kind, { ref_id: piece.id }));
    select({ section: "piece", kind, index: count });
    setPickerKind(null);
  };

  return (
    <nav className="flex h-full flex-col gap-1 overflow-y-auto p-3 text-sm">
      <OutlineRow
        icon={<LayoutGrid className="size-4" />}
        label="Overview"
        active={isSelected(selection, { section: "overview" })}
        onClick={() => select({ section: "overview" })}
      />

      {PIECE_KINDS.map((kind) => {
        const meta = PIECE_KIND_META[kind];
        const refs = pieceRefs(composition, kind);
        return (
          <div key={kind} className="mt-3">
            <div className="mb-1 flex items-center justify-between px-2">
              <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                {meta.pluralLabel}
              </span>
              <div className="flex items-center gap-0.5">
                <button
                  type="button"
                  onClick={() => setPickerKind(kind)}
                  className="rounded p-0.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  aria-label={`Add ${meta.label} from library`}
                  title="Add from library"
                >
                  <Library className="size-3.5" />
                </button>
                <button
                  type="button"
                  onClick={() => addPiece(kind)}
                  className="rounded p-0.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  aria-label={`Add ${meta.label}`}
                  title={`New ${meta.label.toLowerCase()}`}
                >
                  <Plus className="size-3.5" />
                </button>
              </div>
            </div>
            {refs.length === 0 ? (
              <p className="px-2 py-1 text-xs text-muted-foreground/70">{meta.description}</p>
            ) : (
              refs.map((ref, index) => (
                <OutlineRow
                  key={index}
                  label={pieceLabel(kind, ref, index)}
                  indented
                  active={isSelected(selection, { section: "piece", kind, index })}
                  onClick={() => select({ section: "piece", kind, index })}
                />
              ))
            )}
          </div>
        );
      })}

      <div className="mt-3">
        <OutlineRow
          icon={<SlidersHorizontal className="size-4" />}
          label="Scoring"
          active={isSelected(selection, { section: "scorecard" })}
          onClick={() => select({ section: "scorecard" })}
        />
      </div>

      {pickerKind && (
        <LibraryPicker
          workspaceId={workspaceId}
          kind={pickerKind}
          onPick={(piece) => addFromLibrary(pickerKind, piece)}
          onClose={() => setPickerKind(null)}
        />
      )}
    </nav>
  );
}

function OutlineRow({
  icon,
  label,
  active,
  indented,
  onClick,
}: {
  icon?: React.ReactNode;
  label: string;
  active: boolean;
  indented?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex w-full items-center gap-2 truncate rounded-md px-2 py-1.5 text-left transition-colors",
        indented && "pl-7",
        active ? "bg-foreground text-background" : "hover:bg-muted",
      )}
    >
      {icon}
      <span className="truncate">{label}</span>
    </button>
  );
}
