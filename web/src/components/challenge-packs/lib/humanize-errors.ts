// Turns the backend's raw validation field paths (e.g. "challenges[0].difficulty",
// "input_sets[0].cases[1].payload", "pack.slug") into something a human can read
// and act on: a friendly breadcrumb, and the builder selection that focuses the
// offending field when the issue is clicked.

import type { PieceKind } from "./types";
import type { BuilderSelection } from "../use-pack-draft";

/** Label for a top-level composition root. */
const SECTION_LABEL: Record<string, string> = {
  pack: "Overview",
  version: "Overview",
  challenges: "Challenges",
  input_sets: "Inputs",
  validators: "Checks",
  judges: "Checks",
  scorecard: "Scoring",
};

/** Singular noun for an indexed collection ("challenges[2]" → "Challenge 3"). */
const SINGULAR: Record<string, string> = {
  challenges: "Challenge",
  input_sets: "Input set",
  validators: "Validator",
  judges: "Judge",
  cases: "Case",
  dimensions: "Dimension",
  phases: "Phase",
  turns: "Turn",
};

/** Nicer names for common leaf fields; falls back to title-casing the key. */
const LEAF_LABEL: Record<string, string> = {
  expected_from: "Expected",
  pass_threshold: "Pass threshold",
  judge_key: "Judge",
  case_key: "Case key",
  challenge_key: "Challenge",
  execution_mode: "Execution mode",
  tool_policy: "Tool policy",
};

function titleCase(key: string): string {
  return key.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function leafLabel(key: string): string {
  return LEAF_LABEL[key] ?? titleCase(key);
}

function parseSegment(seg: string): { key: string; indices: number[] } {
  const match = seg.match(/^([^[\]]+)((?:\[\d+\])*)$/);
  if (!match) return { key: seg, indices: [] };
  const indices = match[2] ? [...match[2].matchAll(/\[(\d+)\]/g)].map((m) => Number(m[1])) : [];
  return { key: match[1], indices };
}

/**
 * "challenges[0].difficulty" → "Challenges → Challenge 1 → Difficulty"
 * "pack.slug" → "Overview → Slug"
 * "input_sets[0].cases[1].payload" → "Inputs → Input set 1 → Case 2 → Payload"
 */
export function humanizeFieldPath(field: string): string {
  if (!field) return "This pack";
  const parts: string[] = [];
  field
    .split(".")
    .filter(Boolean)
    .forEach((seg, i) => {
      const { key, indices } = parseSegment(seg);
      if (i === 0 && SECTION_LABEL[key]) {
        parts.push(SECTION_LABEL[key]);
      } else if (indices.length === 0) {
        parts.push(leafLabel(key));
      }
      if (indices.length > 0) {
        const singular = SINGULAR[key] ?? titleCase(key);
        indices.forEach((idx) => parts.push(`${singular} ${idx + 1}`));
      }
    });
  return parts.join(" → ") || "This pack";
}

const ROOT_TO_KIND: Record<string, PieceKind> = {
  challenges: "challenge",
  input_sets: "input_set",
  input_set: "input_set",
  validators: "validator",
  judges: "judge",
};

/** The builder selection that focuses the field an error refers to, if known. */
export function fieldToSelection(field: string): BuilderSelection | null {
  const first = field.split(".")[0] ?? "";
  const { key, indices } = parseSegment(first);
  if (key === "pack" || key === "version") return { section: "overview" };
  if (key === "scorecard") return { section: "scorecard" };
  const kind = ROOT_TO_KIND[key];
  if (kind) return { section: "piece", kind, index: indices[0] ?? 0 };
  return null;
}
