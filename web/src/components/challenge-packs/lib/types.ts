// Canonical client-side mirror of backend/internal/challengepack (Composition +
// Bundle pieces) and backend/internal/scoring (validators, judges, scorecard).
// The visual pack builder edits a Composition directly; it is persisted as the
// draft `composition` and compiled server-side into a runnable pack version.
//
// Field names are snake_case to match the Go JSON tags so these types
// deserialize API payloads verbatim.

export type PieceKind = "validator" | "judge" | "input_set" | "challenge";

export type ExecutionMode = "native" | "prompt_eval" | "responses" | "multi_turn";

// --- validators (backend/internal/scoring) ---

export type ValidatorType =
  | "exact_match"
  | "contains"
  | "regex_match"
  | "json_schema"
  | "json_path_match"
  | "boolean_assert"
  | "fuzzy_match"
  | "numeric_match"
  | "normalized_match"
  | "token_f1"
  | "math_equivalence"
  | "bleu_score"
  | "rouge_score"
  | "chrf_score"
  | "file_content_match"
  | "file_exists"
  | "file_json_schema"
  | "directory_structure"
  | "code_execution"
  | "tool_call_assertion"
  | "postcondition";

export interface ValidatorDeclaration {
  key: string;
  type: ValidatorType;
  target: string;
  expected_from?: string;
  config?: unknown;
}

// --- LLM judges ---

export type JudgeMethodMode = "rubric" | "assertion" | "n_wise" | "reference";

export interface ScoreScale {
  min: number;
  max: number;
}

export type ConsensusAggregation = "median" | "mean" | "majority_vote" | "unanimous";

export interface ConsensusConfig {
  aggregation: ConsensusAggregation;
  min_agreement_threshold?: number;
  flag_on_disagreement?: boolean;
}

export interface LLMJudgeDeclaration {
  mode: JudgeMethodMode;
  key: string;
  model?: string;
  models?: string[];
  samples?: number;
  context_from?: string[];
  output_schema?: unknown;
  score_scale?: ScoreScale;
  rubric?: string;
  assertion?: string;
  expect?: boolean;
  prompt?: string;
  position_debiasing?: boolean;
  reference_from?: string;
  consensus?: ConsensusConfig;
  anti_gaming_clauses?: string[];
  timeout_ms?: number;
}

// --- scorecard ---

export type DimensionSource =
  | "validators"
  | "metric"
  | "reliability"
  | "latency"
  | "cost"
  | "behavioral"
  | "llm_judge"
  | "human_preference";

export type ScoringStrategy = "weighted" | "binary" | "hybrid";

export type JudgeMode = "deterministic" | "llm_judge" | "hybrid";

export interface DimensionDeclaration {
  key: string;
  source: DimensionSource;
  validators?: string[];
  metric?: string;
  better_direction?: string;
  weight?: number;
  judge_key?: string;
  gate?: boolean;
  pass_threshold?: number;
}

// --- challenges + input sets (backend/internal/challengepack) ---

export interface ChallengeDefinition {
  key: string;
  title: string;
  category: string;
  difficulty: string;
  instructions?: string;
  definition?: Record<string, unknown>;
}

export interface CaseInput {
  key: string;
  kind: string;
  value?: unknown;
  artifact_key?: string;
  path?: string;
}

export interface CaseExpectation {
  key: string;
  kind: string;
  value?: unknown;
  artifact_key?: string;
  source?: string;
}

export interface CaseDefinition {
  challenge_key: string;
  case_key?: string;
  payload?: Record<string, unknown>;
  inputs?: CaseInput[];
  expectations?: CaseExpectation[];
  // user_simulator is the multi-turn phase flow; typed in the multi-turn phase.
  user_simulator?: unknown;
}

export interface InputSetDefinition {
  key: string;
  name: string;
  description?: string;
  cases?: CaseDefinition[];
}

/** The concrete definition shape stored in a piece of each kind. */
export type PieceDefinition =
  | ValidatorDeclaration
  | LLMJudgeDeclaration
  | ChallengeDefinition
  | InputSetDefinition;

// --- composition (the builder's working document) ---

/**
 * A reference to a reusable library piece (by id) OR an inline, not-yet-promoted
 * definition. Exactly one of ref_id / inline is set.
 */
export interface PieceRef {
  ref_id?: string;
  inline?: PieceDefinition | Record<string, unknown>;
}

export interface PackMetadata {
  slug: string;
  name: string;
  family: string;
  description?: string;
}

export interface CompositionVersion {
  number?: number;
  execution_mode?: ExecutionMode;
  tool_policy?: Record<string, unknown>;
  sandbox?: Record<string, unknown>;
}

export interface CompositionScorecard {
  name?: string;
  version_number?: number;
  judge_mode?: JudgeMode;
  strategy?: ScoringStrategy;
  pass_threshold?: number;
  dimensions?: DimensionDeclaration[];
}

export interface Composition {
  schema_version?: number;
  pack: PackMetadata;
  version: CompositionVersion;
  challenges?: PieceRef[];
  input_sets?: PieceRef[];
  validators?: PieceRef[];
  judges?: PieceRef[];
  scorecard: CompositionScorecard;
}

// --- API resources ---

export interface ChallengePiece {
  id: string;
  workspace_id: string;
  kind: PieceKind;
  slug: string;
  name: string;
  description: string;
  definition: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export type ChallengePackDraftStatus = "draft" | "published" | "discarded";

export interface ChallengePackDraft {
  id: string;
  workspace_id: string;
  name: string;
  execution_mode: ExecutionMode;
  challenge_pack_id?: string;
  composition: Composition | Record<string, unknown>;
  status: ChallengePackDraftStatus;
  last_published_version_id?: string;
  created_at: string;
  updated_at: string;
}

export interface SpecCardDimension {
  key: string;
  source: string;
  weight?: number;
  gate: boolean;
  pass_threshold?: number;
  references?: string[];
  summary: string;
}

/** The readable "rubric" preview (mirror of challengepack.SpecCard). */
export interface SpecCard {
  pack_name: string;
  slug: string;
  family: string;
  description?: string;
  execution_mode: string;
  challenge_count: number;
  case_count: number;
  validator_count: number;
  judge_count: number;
  strategy: string;
  dimensions: SpecCardDimension[];
  pass_criteria: string;
}

/** A validation problem surfaced to the user, scoped to a field. */
export interface ValidationIssue {
  field: string;
  message: string;
}

/** Response of POST .../challenge-pack-drafts/{id}/compile. */
export interface CompileDraftResponse {
  valid: boolean;
  errors: ValidationIssue[];
  spec_card: SpecCard;
  yaml: string;
}
