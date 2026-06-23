// Registry of all 21 validator types (mirror of backend/internal/scoring
// ValidatorType + its IsFileValidator / RequiresExpectedFrom flags). Drives the
// validator editor: the grouped type picker, whether a file path / expected
// value / config sub-form is shown, and which config knobs each type exposes.

import type { ValidatorType } from "../lib/types";

export type ConfigFieldType = "number" | "text" | "boolean" | "json";

export interface ConfigField {
  key: string;
  label: string;
  type: ConfigFieldType;
  hint?: string;
  placeholder?: string;
}

export interface ValidatorTypeMeta {
  value: ValidatorType;
  label: string;
  group: string;
  /** Mirrors scoring.ValidatorType.RequiresExpectedFrom. */
  needsExpected: boolean;
  /** Mirrors scoring.ValidatorType.IsFileValidator — target is a file path. */
  isFile: boolean;
  /** tool_call_assertion forces target = tool_calls. */
  forcesToolCalls?: boolean;
  config?: ConfigField[];
}

export const VALIDATOR_TYPES: ValidatorTypeMeta[] = [
  { value: "contains", label: "Output contains text", group: "String matching", needsExpected: true, isFile: false },
  { value: "exact_match", label: "Output exactly equals", group: "String matching", needsExpected: true, isFile: false },
  { value: "regex_match", label: "Output matches regex", group: "String matching", needsExpected: true, isFile: false },
  {
    value: "fuzzy_match",
    label: "Fuzzy match (similarity)",
    group: "String matching",
    needsExpected: true,
    isFile: false,
    config: [{ key: "threshold", label: "Similarity threshold (0–1)", type: "number", placeholder: "0.8" }],
  },
  {
    value: "normalized_match",
    label: "Normalized match",
    group: "String matching",
    needsExpected: true,
    isFile: false,
    config: [{ key: "steps", label: "Normalization steps", type: "json", hint: '["lowercase","strip_punctuation"]' }],
  },
  { value: "boolean_assert", label: "Boolean assertion", group: "String matching", needsExpected: true, isFile: false },
  {
    value: "numeric_match",
    label: "Numeric match",
    group: "Numeric & math",
    needsExpected: true,
    isFile: false,
    config: [
      { key: "tolerance", label: "Tolerance", type: "number", placeholder: "0.01" },
      { key: "extract_number", label: "Extract number from text", type: "boolean" },
    ],
  },
  { value: "math_equivalence", label: "Math equivalence", group: "Numeric & math", needsExpected: true, isFile: false },
  {
    value: "json_schema",
    label: "Matches JSON schema",
    group: "JSON",
    needsExpected: true,
    isFile: false,
    config: [{ key: "schema", label: "JSON schema", type: "json" }],
  },
  {
    value: "json_path_match",
    label: "JSON path match",
    group: "JSON",
    needsExpected: true,
    isFile: false,
    config: [{ key: "path", label: "JSON path", type: "text", placeholder: "$.status" }],
  },
  {
    value: "token_f1",
    label: "Token F1",
    group: "Generation overlap",
    needsExpected: true,
    isFile: false,
    config: [{ key: "threshold", label: "Min F1 (0–1)", type: "number" }],
  },
  {
    value: "bleu_score",
    label: "BLEU score",
    group: "Generation overlap",
    needsExpected: true,
    isFile: false,
    config: [{ key: "threshold", label: "Min BLEU (0–1)", type: "number" }],
  },
  {
    value: "rouge_score",
    label: "ROUGE score",
    group: "Generation overlap",
    needsExpected: true,
    isFile: false,
    config: [{ key: "threshold", label: "Min ROUGE (0–1)", type: "number" }],
  },
  {
    value: "chrf_score",
    label: "chrF score",
    group: "Generation overlap",
    needsExpected: true,
    isFile: false,
    config: [{ key: "threshold", label: "Min chrF (0–1)", type: "number" }],
  },
  { value: "file_exists", label: "File exists", group: "Files & sandbox", needsExpected: false, isFile: true },
  { value: "file_content_match", label: "File content matches", group: "Files & sandbox", needsExpected: true, isFile: true },
  {
    value: "file_json_schema",
    label: "File matches JSON schema",
    group: "Files & sandbox",
    needsExpected: false,
    isFile: true,
    config: [{ key: "schema", label: "JSON schema", type: "json" }],
  },
  {
    value: "directory_structure",
    label: "Directory structure",
    group: "Files & sandbox",
    needsExpected: false,
    isFile: true,
    config: [
      { key: "required_files", label: "Required files", type: "json", hint: '["src/main.go"]' },
      { key: "forbidden_files", label: "Forbidden files", type: "json" },
    ],
  },
  {
    value: "code_execution",
    label: "Code execution",
    group: "Files & sandbox",
    needsExpected: false,
    isFile: false,
    config: [{ key: "command", label: "Command", type: "text", placeholder: "pytest -q" }],
  },
  {
    value: "postcondition",
    label: "Postcondition",
    group: "Files & sandbox",
    needsExpected: false,
    isFile: false,
    config: [{ key: "check", label: "Check config", type: "json" }],
  },
  {
    value: "tool_call_assertion",
    label: "Tool call assertion",
    group: "Tool calls",
    needsExpected: false,
    isFile: false,
    forcesToolCalls: true,
    config: [
      { key: "tool_name", label: "Tool name", type: "text" },
      { key: "min_calls", label: "Min calls", type: "number" },
    ],
  },
];

export const VALIDATOR_GROUPS: string[] = Array.from(new Set(VALIDATOR_TYPES.map((t) => t.group)));

export function validatorMeta(type: ValidatorType | undefined): ValidatorTypeMeta {
  return VALIDATOR_TYPES.find((t) => t.value === type) ?? VALIDATOR_TYPES[0];
}

export function defaultTargetForType(meta: ValidatorTypeMeta): string {
  if (meta.forcesToolCalls) return "tool_calls";
  if (meta.isFile) return "file:";
  return "final_output";
}
