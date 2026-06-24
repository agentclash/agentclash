// The piece-kind -> editor map, split out so both the center dispatcher
// (editor-registry) and the piece frame can import it without a cycle.

import type { ComponentType } from "react";

import type { PieceKind } from "../lib/types";
import { ChallengeEditor } from "./challenge-editor";
import { InputSetEditor } from "./input-set-editor";
import { JudgeEditor } from "./judge-editor";
import { ValidatorEditor } from "./validator-editor";

export const PIECE_EDITORS: Record<PieceKind, ComponentType<{ index: number }>> = {
  challenge: ChallengeEditor,
  input_set: InputSetEditor,
  validator: ValidatorEditor,
  judge: JudgeEditor,
};
