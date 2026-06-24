"use client";

// Maps the current builder selection to its editor. Piece sections route
// through PieceFrame, which handles library-referenced vs inline pieces.

import { PieceFrame } from "../pieces/piece-frame";
import { usePackDraft } from "../use-pack-draft";
import { PackOverviewEditor } from "./pack-overview-editor";
import { ScorecardEditor } from "./scorecard-editor";

export function BuilderCenter() {
  const { state } = usePackDraft();
  const selection = state.selection;

  if (selection.section === "overview") return <PackOverviewEditor />;
  if (selection.section === "scorecard") return <ScorecardEditor />;
  return <PieceFrame kind={selection.kind} index={selection.index} />;
}
