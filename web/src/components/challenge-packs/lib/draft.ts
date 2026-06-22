// Pure helpers for the builder's working document (a Composition). The reducer
// store (use-pack-draft) and the editors mutate a Composition through these
// immutable helpers so state transitions stay in one testable place.

import type {
  Composition,
  DimensionDeclaration,
  PieceKind,
  PieceRef,
} from "./types";

/** Sidebar/outline order for the piece groups. */
export const PIECE_KINDS: PieceKind[] = ["challenge", "input_set", "validator", "judge"];

export interface PieceKindMeta {
  kind: PieceKind;
  label: string;
  pluralLabel: string;
  description: string;
}

export const PIECE_KIND_META: Record<PieceKind, PieceKindMeta> = {
  challenge: {
    kind: "challenge",
    label: "Challenge",
    pluralLabel: "Challenges",
    description: "The task an agent is asked to do.",
  },
  input_set: {
    kind: "input_set",
    label: "Input set",
    pluralLabel: "Input sets",
    description: "The cases (test data) a challenge runs against.",
  },
  validator: {
    kind: "validator",
    label: "Validator",
    pluralLabel: "Validators",
    description: "A deterministic check on the agent's output.",
  },
  judge: {
    kind: "judge",
    label: "LLM judge",
    pluralLabel: "Judges",
    description: "An LLM-as-judge grader scored against a rubric or assertion.",
  },
};

type PieceField = "challenges" | "input_sets" | "validators" | "judges";

const PIECE_FIELD: Record<PieceKind, PieceField> = {
  challenge: "challenges",
  input_set: "input_sets",
  validator: "validators",
  judge: "judges",
};

/** A fresh, empty composition for a new draft. */
export function emptyComposition(): Composition {
  return {
    schema_version: 1,
    pack: { slug: "", name: "", family: "general" },
    version: { number: 1, execution_mode: "native" },
    challenges: [],
    input_sets: [],
    validators: [],
    judges: [],
    scorecard: { strategy: "weighted", dimensions: [] },
  };
}

/** Reads the piece refs of a kind, always returning an array. */
export function pieceRefs(composition: Composition, kind: PieceKind): PieceRef[] {
  return composition[PIECE_FIELD[kind]] ?? [];
}

/** Immutably append a piece reference of the given kind. */
export function addPieceRef(composition: Composition, kind: PieceKind, ref: PieceRef): Composition {
  const field = PIECE_FIELD[kind];
  return { ...composition, [field]: [...(composition[field] ?? []), ref] };
}

/** Immutably replace the piece reference at index (no-op if out of range). */
export function updatePieceRef(
  composition: Composition,
  kind: PieceKind,
  index: number,
  ref: PieceRef,
): Composition {
  const field = PIECE_FIELD[kind];
  const current = composition[field] ?? [];
  if (index < 0 || index >= current.length) return composition;
  const next = [...current];
  next[index] = ref;
  return { ...composition, [field]: next };
}

/** Immutably remove the piece reference at index. */
export function removePieceRef(composition: Composition, kind: PieceKind, index: number): Composition {
  const field = PIECE_FIELD[kind];
  const next = (composition[field] ?? []).filter((_, i) => i !== index);
  return { ...composition, [field]: next };
}

/** Immutably replace the scorecard dimensions. */
export function setDimensions(composition: Composition, dimensions: DimensionDeclaration[]): Composition {
  return { ...composition, scorecard: { ...composition.scorecard, dimensions } };
}

/**
 * Count of cases across inline input-set pieces. Library-referenced input sets
 * are counted server-side at compile time (their definitions aren't resolved
 * client-side), so this is a lower bound used only for the live preview.
 */
export function inlineCaseCount(composition: Composition): number {
  let count = 0;
  for (const ref of composition.input_sets ?? []) {
    const inline = ref.inline as { cases?: unknown[] } | undefined;
    if (Array.isArray(inline?.cases)) count += inline.cases.length;
  }
  return count;
}
