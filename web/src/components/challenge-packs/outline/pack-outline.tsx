"use client";

import { Library, Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";

import { cn } from "@/lib/utils";
import { addPieceRef, PIECE_KIND_META, pieceRefs, removePieceRef } from "../lib/draft";
import { fieldToSelection } from "../lib/humanize-errors";
import type { Composition, PieceDefinition, PieceKind, PieceRef } from "../lib/types";
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

  // Sections/pieces that have an unresolved validation issue, derived from the
  // latest compile so the rail can flag exactly where work remains.
  const issues = useMemo(
    () =>
      (state.compile?.errors ?? [])
        .map((e) => fieldToSelection(e.field))
        .filter((s): s is BuilderSelection => s != null),
    [state.compile],
  );
  const sectionIssue = (section: "overview" | "scorecard") =>
    issues.some((t) => t.section === section);
  const pieceIssue = (kind: PieceKind, index: number) =>
    issues.some((t) => t.section === "piece" && t.kind === kind && t.index === index);

  const addPiece = (kind: PieceKind) => {
    const count = pieceRefs(composition, kind).length;
    update((c) => addPieceRef(c, kind, { inline: defaultPieceInline(kind, count + 1) }));
    select({ section: "piece", kind, index: count });
  };
  const addFromLibrary = (kind: PieceKind, ref: PieceRef) => {
    const count = pieceRefs(composition, kind).length;
    update((c) => addPieceRef(c, kind, ref));
    select({ section: "piece", kind, index: count });
    setPickerKind(null);
  };
  const removePiece = (kind: PieceKind, index: number) => {
    update((c) => removePieceRef(c, kind, index));
    if (selection.section === "piece" && selection.kind === kind) {
      select({ section: "overview" });
    }
  };

  const blockProps = {
    composition,
    selection,
    pieceIssue,
    onSelect: (kind: PieceKind, index: number) => select({ section: "piece", kind, index }),
    onAddNew: addPiece,
    onAddLibrary: setPickerKind,
    onRemove: removePiece,
  };

  return (
    <nav className="flex h-full flex-col gap-px overflow-y-auto p-3 text-sm">
      <RailNav
        num={1}
        label="Overview"
        active={isSelected(selection, { section: "overview" })}
        issue={sectionIssue("overview")}
        onClick={() => select({ section: "overview" })}
      />

      <PieceBlock num={2} title="Challenges" kind="challenge" {...blockProps} />
      <PieceBlock num={3} title="Inputs" kind="input_set" {...blockProps} />

      <div className="mt-4">
        <div className="mb-1 flex items-center gap-2 px-2">
          <SectionNumber n={4} />
          <span className="text-xs font-medium uppercase tracking-wide text-builder-fg-subtle">
            Checks
          </span>
        </div>
        <PieceBlock title="Validators" kind="validator" sub {...blockProps} />
        <PieceBlock title="Judges" kind="judge" sub {...blockProps} />
      </div>

      <div className="mt-4">
        <RailNav
          num={5}
          label="Scoring"
          active={isSelected(selection, { section: "scorecard" })}
          issue={sectionIssue("scorecard")}
          onClick={() => select({ section: "scorecard" })}
        />
      </div>

      {pickerKind && (
        <LibraryPicker
          workspaceId={workspaceId}
          kind={pickerKind}
          onAdd={(ref) => addFromLibrary(pickerKind, ref)}
          onClose={() => setPickerKind(null)}
        />
      )}
    </nav>
  );
}

interface BlockProps {
  composition: Composition;
  selection: BuilderSelection;
  pieceIssue: (kind: PieceKind, index: number) => boolean;
  onSelect: (kind: PieceKind, index: number) => void;
  onAddNew: (kind: PieceKind) => void;
  onAddLibrary: (kind: PieceKind) => void;
  onRemove: (kind: PieceKind, index: number) => void;
}

function PieceBlock({
  kind,
  title,
  num,
  sub,
  composition,
  selection,
  pieceIssue,
  onSelect,
  onAddNew,
  onAddLibrary,
  onRemove,
}: BlockProps & { kind: PieceKind; title: string; num?: number; sub?: boolean }) {
  const meta = PIECE_KIND_META[kind];
  const refs = pieceRefs(composition, kind);

  return (
    <div className={cn(num != null ? "mt-4" : "mt-2")}>
      <div className="mb-1 flex items-center gap-2 px-2">
        {num != null && <SectionNumber n={num} />}
        <span
          className={cn(
            "flex-1 truncate text-xs font-medium tracking-wide",
            sub ? "text-builder-fg-faint" : "uppercase text-builder-fg-subtle",
          )}
        >
          {title}
        </span>
        {refs.length > 0 && (
          <span className="text-[11px] tabular-nums text-builder-fg-faint">{refs.length}</span>
        )}
        <button
          type="button"
          onClick={() => onAddLibrary(kind)}
          aria-label={`Add ${meta.label} from library`}
          title="Add from library"
          className="rounded p-0.5 text-builder-fg-subtle transition-colors hover:bg-builder-surface hover:text-builder-fg"
        >
          <Library className="size-3.5" />
        </button>
        <button
          type="button"
          onClick={() => onAddNew(kind)}
          aria-label={`Add ${meta.label}`}
          title={`New ${meta.label.toLowerCase()}`}
          className="rounded p-0.5 text-builder-fg-subtle transition-colors hover:bg-builder-surface hover:text-builder-fg"
        >
          <Plus className="size-3.5" />
        </button>
      </div>
      {refs.length === 0 ? (
        <p className="px-2 pb-1 text-xs leading-relaxed text-builder-fg-faint">{meta.description}</p>
      ) : (
        refs.map((ref, index) => (
          <RailNav
            key={index}
            label={pieceLabel(kind, ref, index)}
            indent
            active={isSelected(selection, { section: "piece", kind, index })}
            issue={pieceIssue(kind, index)}
            onClick={() => onSelect(kind, index)}
            onRemove={() => onRemove(kind, index)}
          />
        ))
      )}
    </div>
  );
}

function RailNav({
  num,
  label,
  indent,
  active,
  issue,
  onClick,
  onRemove,
}: {
  num?: number;
  label: string;
  indent?: boolean;
  active: boolean;
  issue?: boolean;
  onClick: () => void;
  onRemove?: () => void;
}) {
  return (
    <div className="group/row relative">
      <button
        type="button"
        onClick={onClick}
        className={cn(
          "flex w-full items-center gap-2 truncate rounded-md py-1.5 pr-2 text-left transition-colors",
          indent ? "pl-7" : "pl-2",
          active
            ? "bg-builder-surface-hover text-builder-fg"
            : "text-builder-fg-muted hover:bg-builder-surface hover:text-builder-fg",
        )}
      >
        {num != null && <SectionNumber n={num} active={active} />}
        <span className="flex-1 truncate">{label}</span>
        {issue && (
          <span
            className="size-1.5 shrink-0 rounded-full bg-builder-warn group-hover/row:opacity-0"
            aria-label="has issues"
          />
        )}
      </button>
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          aria-label="Remove"
          className="absolute top-1/2 right-1 -translate-y-1/2 rounded p-1 text-builder-fg-faint opacity-0 transition-colors transition-opacity hover:text-builder-warn group-hover/row:opacity-100"
        >
          <Trash2 className="size-3.5" />
        </button>
      )}
    </div>
  );
}

function SectionNumber({ n, active }: { n: number; active?: boolean }) {
  return (
    <span
      className={cn(
        "w-4 shrink-0 text-center text-[11px] tabular-nums",
        active ? "text-builder-fg" : "text-builder-fg-faint",
      )}
    >
      {n}
    </span>
  );
}
