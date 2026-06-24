// Canonical client-side mirror of backend/internal/evalpack (Composition +
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

// --- challenges + input sets (backend/internal/evalpack) ---

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

// --- multi-turn user simulator (mirror of evalpack.UserSimulatorSpec) ---

export type UserSimulatorActor = "scripted" | "llm" | "human";

export type UserSimulatorTrigger =
  | "always"
  | "on_assistant_mismatch"
  | "on_validator_fail"
  | "on_judge_below"
  | "on_agent_loop"
  | "on_max_llm_turns"
  | "manual"
  | "never";

export interface UserSimulatorTurn {
  message: string;
  expects?: CaseExpectation[];
}

export interface UserSimulatorPhase {
  id: string;
  actor: UserSimulatorActor;
  trigger?: UserSimulatorTrigger;
  turns?: UserSimulatorTurn[];
  persona?: string;
  max_turns?: number;
  until?: string[];
  timeout_ms?: number;
  on_timeout?: string;
  model?: string;
}

export interface UserSimulatorSpec {
  schema_version?: number;
  kind?: string;
  max_turns?: number;
  phases: UserSimulatorPhase[];
}

export interface CaseDefinition {
  challenge_key: string;
  case_key?: string;
  payload?: Record<string, unknown>;
  inputs?: CaseInput[];
  expectations?: CaseExpectation[];
  // Required for multi_turn execution mode; ignored otherwise.
  user_simulator?: UserSimulatorSpec;
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

/** A built-in starter piece from GET /v1/challenge-piece-library. */
export interface StarterPiece {
  kind: PieceKind;
  slug: string;
  name: string;
  description: string;
  definition: Record<string, unknown>;
}

export type EvalPackDraftStatus = "draft" | "published" | "discarded";

export interface EvalPackDraft {
  id: string;
  workspace_id: string;
  name: string;
  execution_mode: ExecutionMode;
  eval_pack_id?: string;
  composition: Composition | Record<string, unknown>;
  status: EvalPackDraftStatus;
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

/** The readable "rubric" preview (mirror of evalpack.SpecCard). */
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

/** Response of POST .../eval-pack-drafts/{id}/compile. */
export interface CompileDraftResponse {
  valid: boolean;
  errors: ValidationIssue[];
  spec_card: SpecCard;
  yaml: string;
}

// --- eval pack catalog (the curated library / "templates") ---

/** Editorial grouping for the library gallery (mirror of evalpack catalog categories). */
export type CatalogCategory = "enterprise" | "agent_capability" | "safety";

/** A curated, ready-to-run pack from GET /v1/eval-pack-catalog. */
export interface CatalogPack {
  slug: string;
  name: string;
  family: string;
  category?: CatalogCategory;
  tags?: string[];
  description?: string;
  difficulty?: string;
  execution_mode: ExecutionMode;
  estimated_cost_usd?: number;
  estimated_runtime_ms?: number;
  spec_card: SpecCard;
}

/** Catalog detail (GET /v1/eval-pack-catalog/{slug}) — adds the runnable YAML. */
export interface CatalogPackDetail extends CatalogPack {
  yaml: string;
}

/** Response of POST .../eval-pack-catalog/{slug}/instantiate. */
export interface InstantiateCatalogPackResponse {
  eval_pack_id: string;
  eval_pack_version_id: string;
  evaluation_spec_id?: string;
  input_set_ids?: string[];
  slug: string;
  already_existed: boolean;
  runnable: boolean;
}
