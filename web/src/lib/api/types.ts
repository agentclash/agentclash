/**
 * Backend API response types.
 * These mirror the Go structs defined in backend/internal/api/.
 */

// --- Auth & Session ---

/** GET /v1/auth/session — mirrors sessionResponse in routes.go */
export interface SessionResponse {
  user_id: string;
  workos_user_id?: string;
  email?: string;
  display_name?: string;
  organization_memberships: OrganizationMembership[];
  workspace_memberships: WorkspaceMembership[];
}

export interface OrganizationMembership {
  organization_id: string;
  role: string; // "org_admin" | "org_member"
}

export interface WorkspaceMembership {
  workspace_id: string;
  role: string; // "workspace_admin" | "workspace_member" | "workspace_viewer"
}

// --- Users ---

/** GET /v1/users/me — mirrors GetUserMeResult in users.go */
export interface UserMeResponse {
  user_id: string;
  workos_user_id?: string;
  email?: string;
  display_name?: string;
  organizations: UserMeOrganization[];
}

export interface UserMeOrganization {
  id: string;
  name: string;
  slug: string;
  role: string;
  workspaces: UserMeWorkspace[];
}

export interface UserMeWorkspace {
  id: string;
  name: string;
  slug: string;
  role: string;
}

// --- Onboarding ---

/** POST /v1/onboarding — mirrors OnboardResult in onboarding.go */
export interface OnboardResult {
  organization: OrganizationResult;
  workspace: WorkspaceResult;
}

export interface OrganizationResult {
  id: string;
  name: string;
  slug: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface WorkspaceResult {
  id: string;
  organization_id: string;
  name: string;
  slug: string;
  status: string;
  created_at: string;
  updated_at: string;
}

// --- Organization Management ---

export type OrgRole = "org_admin" | "org_member";

export type OrgMembershipStatus =
  | "invited"
  | "active"
  | "suspended"
  | "archived";

/** GET /v1/organizations/{id} response, PATCH response */
export interface Organization {
  id: string;
  name: string;
  slug: string;
  status: string; // "active" | "archived"
  created_at: string;
  updated_at: string;
}

/** GET /v1/organizations/{id}/memberships list item */
export interface OrgMember {
  id: string;
  organization_id: string;
  user_id: string;
  email: string;
  display_name: string;
  role: OrgRole;
  membership_status: OrgMembershipStatus;
  created_at: string;
  updated_at?: string;
}

/** POST /v1/organizations/{id}/memberships request */
export interface InviteOrgMemberRequest {
  email: string;
  role: OrgRole;
}

/** PATCH /v1/organization-memberships/{id} request */
export interface UpdateOrgMembershipRequest {
  role?: OrgRole;
  status?: OrgMembershipStatus;
}

/** GET /v1/organizations/{id}/workspaces list item */
export interface OrgWorkspace {
  id: string;
  organization_id: string;
  name: string;
  slug: string;
  status: string; // "active" | "archived"
  created_at: string;
  updated_at: string;
}

// --- Workspace Management ---

export type WorkspaceRole =
  | "workspace_admin"
  | "workspace_member"
  | "workspace_viewer";

/** GET /v1/workspaces/{id}/details response */
export interface WorkspaceDetail {
  id: string;
  organization_id: string;
  name: string;
  slug: string;
  status: string; // "active" | "archived"
  created_at: string;
  updated_at: string;
}

/** GET /v1/workspaces/{id}/memberships list item */
export interface WorkspaceMember {
  id: string;
  workspace_id: string;
  organization_id: string;
  user_id: string;
  email: string;
  display_name: string;
  role: WorkspaceRole;
  membership_status: OrgMembershipStatus; // same enum: invited/active/suspended/archived
  created_at: string;
  updated_at?: string;
}

// --- Agent Builds ---

/** GET /v1/workspaces/{id}/agent-builds item, POST response */
export interface AgentBuild {
  id: string;
  workspace_id: string;
  name: string;
  slug: string;
  description?: string;
  lifecycle_status: string;
  created_at: string;
  updated_at: string;
}

/** GET /v1/agent-builds/{id} — build with versions */
export interface AgentBuildDetail extends AgentBuild {
  versions: AgentBuildVersion[];
}

/** POST /v1/workspaces/{id}/agent-builds request */
export interface CreateAgentBuildRequest {
  name: string;
  description?: string;
}

// --- Agent Build Versions ---

export interface AgentBuildVersion {
  id: string;
  agent_build_id: string;
  version_number: number;
  version_status: string;
  agent_kind: string;
  interface_spec: unknown;
  policy_spec: unknown;
  reasoning_spec: unknown;
  memory_spec: unknown;
  workflow_spec: unknown;
  guardrail_spec: unknown;
  model_spec: unknown;
  output_schema: unknown;
  trace_contract: unknown;
  publication_spec: unknown;
  tools: ToolBinding[];
  knowledge_sources: KnowledgeSourceBinding[];
  created_at: string;
}

export interface ToolBinding {
  tool_id: string;
  binding_role: string;
  binding_config?: unknown;
}

export interface KnowledgeSourceBinding {
  knowledge_source_id: string;
  binding_role: string;
  binding_config?: unknown;
}

/** POST/PATCH agent build version request body */
export interface AgentBuildVersionInput {
  agent_kind: string;
  interface_spec: unknown;
  policy_spec: unknown;
  reasoning_spec?: unknown;
  memory_spec?: unknown;
  workflow_spec?: unknown;
  guardrail_spec?: unknown;
  model_spec?: unknown;
  output_schema?: unknown;
  trace_contract?: unknown;
  publication_spec?: unknown;
  tools?: ToolBinding[];
  knowledge_sources?: KnowledgeSourceBinding[];
}

/** POST /v1/agent-build-versions/{id}/validate response */
export interface ValidationResult {
  valid: boolean;
  errors: ValidationError[];
}

export interface ValidationError {
  field: string;
  message: string;
}

/** Agent kind enum values */
export const AGENT_KINDS = [
  "llm_agent",
  "workflow_agent",
  "programmatic_agent",
  "multi_agent_system",
  "hosted_external",
] as const;

export type AgentKind = (typeof AGENT_KINDS)[number];

// --- Agent Deployments ---

/** GET /v1/workspaces/{id}/agent-deployments list item */
export interface AgentDeployment {
  id: string;
  organization_id: string;
  workspace_id: string;
  name: string;
  status: string; // "active" | "paused" | "archived"
  latest_snapshot_id?: string;
  created_at: string;
  updated_at: string;
}

/** POST /v1/workspaces/{id}/agent-deployments request */
export interface CreateAgentDeploymentRequest {
  name: string;
  agent_build_id: string;
  build_version_id: string;
  runtime_profile_id: string;
  provider_account_id?: string;
  model_alias_id?: string;
  deployment_config?: unknown;
}

/** POST /v1/workspaces/{id}/agent-deployments response */
export interface AgentDeploymentCreateResponse {
  id: string;
  workspace_id: string;
  agent_build_id: string;
  current_build_version_id: string;
  name: string;
  slug: string;
  deployment_type: string;
  status: string;
  created_at: string;
  updated_at: string;
}

// --- Infrastructure Resources ---

/** GET /v1/workspaces/{id}/runtime-profiles list item */
export interface RuntimeProfile {
  id: string;
  workspace_id?: string;
  name: string;
  slug: string;
  execution_target: string;
  trace_mode: string;
  created_at: string;
  updated_at: string;
}

/** GET /v1/workspaces/{id}/provider-accounts list item */
export interface ProviderAccount {
  id: string;
  workspace_id?: string;
  provider_key: string;
  name: string;
  status: string;
  created_at: string;
  updated_at: string;
}

/** GET /v1/workspaces/{id}/model-aliases list item */
export interface ModelAlias {
  id: string;
  workspace_id?: string;
  alias_key: string;
  display_name: string;
  status: string;
  created_at: string;
  updated_at: string;
}

// --- Challenge Packs ---

/** GET /v1/workspaces/{id}/challenge-packs list item */
export interface ChallengePack {
  id: string;
  name: string;
  description?: string;
  versions: ChallengePackVersion[];
  created_at: string;
  updated_at: string;
}

export interface ChallengePackVersion {
  id: string;
  challenge_pack_id: string;
  version_number: number;
  lifecycle_status: string; // "draft" | "runnable" | "deprecated" | "archived"
  created_at: string;
  updated_at: string;
}

/** POST /v1/workspaces/{id}/challenge-packs/validate response */
export interface ValidateChallengePackResponse {
  valid: boolean;
  errors: ValidationError[];
}

/** POST /v1/workspaces/{id}/challenge-packs response (201) */
export interface PublishChallengePackResponse {
  challenge_pack_id: string;
  challenge_pack_version_id: string;
  evaluation_spec_id: string;
  input_set_ids: string[];
  bundle_artifact_id?: string;
}

// --- Runs ---

/** GET /v1/workspaces/{id}/runs list item, GET /v1/runs/{id} detail */
export interface Run {
  id: string;
  workspace_id: string;
  challenge_pack_version_id: string;
  challenge_input_set_id?: string;
  name: string;
  status: RunStatus;
  execution_mode: string; // "single_agent" | "comparison"
  temporal_workflow_id?: string;
  temporal_run_id?: string;
  queued_at?: string;
  started_at?: string;
  finished_at?: string;
  cancelled_at?: string;
  failed_at?: string;
  created_at: string;
  updated_at: string;
  links: {
    self: string;
    agents: string;
  };
}

export type RunStatus =
  | "draft"
  | "queued"
  | "provisioning"
  | "running"
  | "scoring"
  | "completed"
  | "failed"
  | "cancelled";

/** POST /v1/runs request */
export interface CreateRunRequest {
  workspace_id: string;
  challenge_pack_version_id: string;
  challenge_input_set_id?: string;
  name?: string;
  agent_deployment_ids: string[];
}

/** POST /v1/runs response (201) */
export interface CreateRunResponse {
  id: string;
  workspace_id: string;
  challenge_pack_version_id: string;
  challenge_input_set_id?: string;
  status: RunStatus;
  execution_mode: string;
  created_at: string;
  queued_at?: string;
  links: {
    self: string;
    agents: string;
  };
}

// --- Run Agents ---

export type RunAgentStatus =
  | "queued"
  | "ready"
  | "executing"
  | "evaluating"
  | "completed"
  | "failed";

/** GET /v1/runs/{id}/agents list item */
export interface RunAgent {
  id: string;
  run_id: string;
  lane_index: number;
  label: string;
  agent_deployment_id: string;
  agent_deployment_snapshot_id: string;
  status: RunAgentStatus;
  queued_at?: string;
  started_at?: string;
  finished_at?: string;
  failure_reason?: string;
  created_at: string;
  updated_at: string;
}

// --- Run Ranking ---

export interface RunRankingResponse {
  state: "ready" | "pending" | "errored";
  message?: string;
  ranking?: RunRanking;
}

export interface RunRanking {
  run_id: string;
  evaluation_spec_id: string;
  sort: {
    field: string;
    direction: string;
    default_order: boolean;
  };
  winner: {
    run_agent_id?: string;
    strategy: string;
    status: string;
    reason_code: string;
  };
  items: RankingItem[];
}

export interface RankingItem {
  rank: number;
  run_agent_id: string;
  lane_index: number;
  label: string;
  status: string;
  has_scorecard: boolean;
  sort_value?: number;
  delta_from_top?: number;
  sort_state: string;
  composite_score?: number;
  overall_score?: number;
  correctness_score?: number;
  reliability_score?: number;
  latency_score?: number;
  cost_score?: number;
  dimensions?: Record<string, { state: string; score?: number }>;
}

// --- Workspace Secrets ---

/** GET /v1/workspaces/{id}/secrets list item — metadata only, never the value */
export interface WorkspaceSecret {
  key: string;
  created_at: string;
  updated_at: string;
  created_by?: string;
  updated_by?: string;
}

// --- Replay ---

export type ReplayState = "ready" | "pending" | "errored";

export type ReplayStepType =
  | "run"
  | "agent_step"
  | "model_call"
  | "tool_call"
  | "sandbox_command"
  | "sandbox_file"
  | "output"
  | "scoring"
  | "scoring_metric"
  | "event";

export type ReplayStepStatus = "completed" | "running" | "failed";

/** Single step in a replay timeline — mirrors runAgentReplayStepDocument in Go. */
export interface ReplayStep {
  type: ReplayStepType;
  status: ReplayStepStatus;
  headline: string;
  source: string;
  started_sequence: number;
  completed_sequence?: number;
  occurred_at: string;
  completed_at?: string;
  event_count: number;
  event_types: string[];
  artifact_ids?: string[];
  step_index?: number;
  provider_key?: string;
  provider_model_id?: string;
  tool_name?: string;
  sandbox_action?: string;
  metric_key?: string;
  final_output?: string;
  error_message?: string;
}

export interface ReplaySummaryCounts {
  events: number;
  replay_steps: number;
  agent_steps: number;
  model_calls: number;
  tool_calls: number;
  sandbox_commands: number;
  sandbox_file_events: number;
  outputs: number;
  scoring_events: number;
}

export interface ReplaySummary {
  schema_version: string;
  status: string;
  headline: string;
  counts: ReplaySummaryCounts;
  artifact_ids?: string[];
  terminal_state?: {
    status: string;
    event_type: string;
    source: string;
    sequence_number: number;
    occurred_at: string;
    headline: string;
    error_message?: string;
  };
}

export interface ReplayPagination {
  next_cursor?: string;
  limit: number;
  total_steps: number;
  has_more: boolean;
}

/** GET /v1/replays/{runAgentID} — mirrors getRunAgentReplayResponse in Go. */
export interface ReplayResponse {
  state: ReplayState;
  message?: string;
  run_agent_id: string;
  run_id: string;
  run_agent_status: string;
  replay?: {
    id: string;
    artifact_id?: string;
    summary: ReplaySummary;
    latest_sequence_number?: number;
    event_count: number;
    created_at: string;
    updated_at: string;
  };
  steps: ReplayStep[];
  pagination: ReplayPagination;
}

// --- Scorecards ---

/** GET /v1/scorecards/{runAgentID} — mirrors getRunAgentScorecardResponse in Go. */
export interface ScorecardResponse {
  state: ReplayState; // "ready" | "pending" | "errored"
  message?: string;
  run_agent_status: RunAgentStatus;
  id: string;
  run_agent_id: string;
  run_id: string;
  evaluation_spec_id: string;
  overall_score?: number;
  correctness_score?: number;
  reliability_score?: number;
  latency_score?: number;
  cost_score?: number;
  scorecard: ScorecardDocument;
  created_at: string;
  updated_at: string;
}

export interface ScorecardDocument {
  run_agent_id: string;
  evaluation_spec_id: string;
  status: "complete" | "partial" | "failed";
  warnings?: string[];
  dimensions: Record<string, ScorecardDimension>;
  validator_summary: Record<string, number>;
  metric_summary: Record<string, number>;
}

export interface ScorecardDimension {
  state: "available" | "unavailable" | "error";
  score?: number;
  reason?: string;
}

// --- Comparisons ---

export type ComparisonReadState =
  | "comparable"
  | "partial_evidence"
  | "not_comparable";

/** GET /v1/compare — mirrors getRunComparisonResponse in compare_reads.go */
export interface ComparisonResponse {
  state: ComparisonReadState;
  status: string; // "comparable" | "not_comparable"
  reason_code?: string;
  baseline_run_id: string;
  candidate_run_id: string;
  baseline_run_agent_id?: string;
  candidate_run_agent_id?: string;
  generated_at: string;
  key_deltas: DeltaHighlight[];
  regression_reasons: string[];
  evidence_quality: EvidenceQuality;
  summary: ComparisonSummary;
  links: { viewer: string };
}

export interface DeltaHighlight {
  metric: string;
  baseline_value?: number;
  candidate_value?: number;
  delta?: number;
  better_direction: string; // "higher" | "lower"
  outcome: string; // "better" | "worse" | "same" | "unknown"
  state: string; // "available" | "unavailable"
}

export interface EvidenceQuality {
  missing_fields?: string[];
  warnings?: string[];
}

export interface ComparisonSummary {
  schema_version: string;
  status: string;
  reason_code?: string;
  baseline_refs: { run_id: string; run_agent_id?: string };
  candidate_refs: { run_id: string; run_agent_id?: string };
  dimension_deltas?: Record<
    string,
    {
      baseline_value?: number;
      candidate_value?: number;
      delta?: number;
      better_direction: string;
      state: string;
    }
  >;
  failure_divergence: {
    baseline_run_agent_status: string;
    candidate_run_agent_status: string;
    baseline_failure_reason?: string;
    candidate_failure_reason?: string;
    candidate_failed_baseline_succeeded: boolean;
    candidate_succeeded_baseline_failed: boolean;
    both_failed_differently: boolean;
  };
  evidence_quality: EvidenceQuality;
}

// --- Release Gates ---

export type ReleaseGateVerdict =
  | "pass"
  | "warn"
  | "fail"
  | "insufficient_evidence";

export type ReleaseGateEvidenceStatus = "sufficient" | "insufficient";

/** Individual release gate — mirrors releaseGateResponse in release_gates.go */
export interface ReleaseGate {
  id: string;
  run_comparison_id: string;
  policy_key: string;
  policy_version: number;
  policy_fingerprint: string;
  policy_snapshot: ReleaseGatePolicy;
  verdict: ReleaseGateVerdict;
  reason_code: string;
  summary: string;
  evidence_status: ReleaseGateEvidenceStatus;
  evaluation_details: ReleaseGateEvaluationDetails;
  generated_at: string;
  updated_at: string;
}

/** GET /v1/release-gates response */
export interface ListReleaseGatesResponse {
  baseline_run_id: string;
  candidate_run_id: string;
  release_gates: ReleaseGate[];
}

/** POST /v1/release-gates/evaluate request */
export interface EvaluateReleaseGateRequest {
  baseline_run_id: string;
  candidate_run_id: string;
  baseline_run_agent_id?: string;
  candidate_run_agent_id?: string;
  policy: ReleaseGatePolicy;
}

/** POST /v1/release-gates/evaluate response */
export interface EvaluateReleaseGateResponse {
  baseline_run_id: string;
  candidate_run_id: string;
  release_gate: ReleaseGate;
}

export interface ReleaseGatePolicy {
  policy_key: string;
  policy_version: number;
  require_comparable?: boolean;
  require_evidence_quality?: boolean;
  fail_on_candidate_failure?: boolean;
  fail_on_both_failed_differently?: boolean;
  required_dimensions?: string[];
  dimensions?: Record<string, DimensionThreshold>;
}

export interface DimensionThreshold {
  warn_delta?: number;
  fail_delta?: number;
}

export interface ReleaseGateEvaluationDetails {
  policy_key: string;
  policy_version: number;
  comparison_status: string;
  missing_fields?: string[];
  warnings?: string[];
  triggered_conditions?: string[];
  required_dimensions?: string[];
  dimension_results?: Record<string, DimensionEvaluation>;
}

export interface DimensionEvaluation {
  state: string;
  better_direction?: string;
  observed_delta?: number;
  worsening_delta?: number;
  warn_threshold?: number;
  fail_threshold?: number;
  outcome: string;
}

// --- Artifacts ---

/** POST /v1/workspaces/{workspaceID}/artifacts response (201) */
export interface ArtifactUploadResponse {
  id: string;
  workspace_id: string;
  run_id?: string;
  run_agent_id?: string;
  artifact_type: string;
  content_type?: string;
  size_bytes?: number;
  checksum_sha256?: string;
  visibility: string;
  metadata: Record<string, unknown>;
  created_at: string;
}

/** GET /v1/artifacts/{artifactID}/download response */
export interface ArtifactDownloadResponse {
  id: string;
  workspace_id: string;
  artifact_type: string;
  content_type?: string;
  size_bytes?: number;
  checksum_sha256?: string;
  metadata: Record<string, unknown>;
  url: string;
  expires_at: string;
}

// --- Playgrounds ---

export interface Playground {
  id: string;
  workspace_id: string;
  name: string;
  prompt_template: string;
  system_prompt: string;
  evaluation_spec: unknown;
  created_at: string;
  updated_at: string;
}

export interface PlaygroundTestCase {
  id: string;
  playground_id: string;
  case_key: string;
  variables: Record<string, unknown>;
  expectations: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface PlaygroundExperiment {
  id: string;
  workspace_id: string;
  playground_id: string;
  provider_account_id: string;
  model_alias_id: string;
  name: string;
  status: "queued" | "running" | "completed" | "failed";
  request_config: Record<string, unknown>;
  summary: Record<string, unknown>;
  temporal_workflow_id?: string;
  temporal_run_id?: string;
  queued_at?: string;
  started_at?: string;
  finished_at?: string;
  failed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface PlaygroundExperimentResult {
  id: string;
  playground_experiment_id: string;
  playground_test_case_id: string;
  case_key: string;
  status: "completed" | "failed";
  variables: Record<string, unknown>;
  expectations: Record<string, unknown>;
  rendered_prompt: string;
  actual_output: string;
  provider_key: string;
  provider_model_id: string;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  latency_ms: number;
  cost_usd?: number;
  validator_results: unknown[];
  llm_judge_results: unknown[];
  dimension_results: unknown[];
  dimension_scores: Record<string, number | null>;
  warnings: string[];
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export interface PlaygroundDimensionDelta {
  baseline_value?: number | null;
  candidate_value?: number | null;
  delta?: number | null;
  state: string;
}

export interface PlaygroundCaseComparison {
  case_key: string;
  baseline_status: "completed" | "failed";
  candidate_status: "completed" | "failed";
  baseline_output: string;
  candidate_output: string;
  baseline_error_message?: string;
  candidate_error_message?: string;
  dimension_deltas: Record<string, PlaygroundDimensionDelta>;
}

export interface PlaygroundExperimentComparison {
  baseline_experiment: PlaygroundExperiment;
  candidate_experiment: PlaygroundExperiment;
  aggregated_dimension_deltas: Record<string, PlaygroundDimensionDelta>;
  per_case: PlaygroundCaseComparison[];
}

// --- Errors ---

/** Standard error envelope returned by all backend error responses. */
export interface ApiErrorResponse {
  error: {
    code: string;
    message: string;
  };
}
