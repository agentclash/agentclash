"use client";

// Maps the current builder selection to its dedicated editor. Adding a future
// piece kind is one entry here — no scattered switch statements.

import type { ComponentType } from "react";

import type { PieceKind } from "../lib/types";
import { usePackDraft } from "../use-pack-draft";
import { ChallengeEditor } from "./challenge-editor";
import { InputSetEditor } from "./input-set-editor";
import { JudgeEditor } from "./judge-editor";
import { PackOverviewEditor } from "./pack-overview-editor";
import { ScorecardEditor } from "./scorecard-editor";
import { ValidatorEditor } from "./validator-editor";

export const PIECE_EDITORS: Record<PieceKind, ComponentType<{ index: number }>> = {
  challenge: ChallengeEditor,
  input_set: InputSetEditor,
  validator: ValidatorEditor,
  judge: JudgeEditor,
};

export function BuilderCenter() {
  const { state } = usePackDraft();
  const selection = state.selection;

  if (selection.section === "overview") return <PackOverviewEditor />;
  if (selection.section === "scorecard") return <ScorecardEditor />;

  const Editor = PIECE_EDITORS[selection.kind];
  return <Editor index={selection.index} />;
}
