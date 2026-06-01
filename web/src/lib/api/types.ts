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
  accept_url?: string;
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
  public_packs: boolean;
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
  public_packs: boolean;
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
  accept_url?: string;
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
  current_build_version_id: string;
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

// --- Agent Harnesses ---

export type AgentHarnessAuthMode =
  | "api_key_secret";

/** GET /v1/workspaces/{id}/agent-harnesses list item, POST response */
export interface AgentHarness {
  id: string;
  organization_id: string;
  workspace_id: string;
  created_by_user_id?: string;
  name: string;
  slug: string;
  description: string;
  status: string;
  harness_kind: "codex_e2b" | "claude_e2b";
  task_prompt: string;
  codex_template: string;
  codex_model?: string;
  auth_mode: AgentHarnessAuthMode;
  openai_api_key_secret_name?: string;
  repository_url?: string;
  repository_provider?: "github";
  github_repository_id?: number;
  github_installation_id?: number;
  repository_full_name?: string;
  repository_clone_url?: string;
  base_branch?: string;
  execution_config: unknown;
  evaluation_config: unknown;
  created_at: string;
  updated_at: string;
}

/** POST /v1/workspaces/{id}/agent-harnesses request */
export interface CreateAgentHarnessRequest {
  name: string;
  description?: string;
  harness_kind?: "codex_e2b" | "claude_e2b";
  task_prompt: string;
  codex_template?: string;
  codex_model?: string;
  auth_mode: AgentHarnessAuthMode;
  openai_api_key_secret_name?: string;
  repository_url?: string;
  repository_provider?: "github";
  github_repository_id?: number;
  github_installation_id?: number;
  base_branch?: string;
  execution_config?: unknown;
  evaluation_config?: unknown;
}

export interface StartGitHubInstallationResponse {
  install_url: string;
  state: string;
  expires_at: string;
}

export interface CompleteGitHubInstallationRequest {
  installation_id: number;
  state: string;
}

export interface CompleteGitHubInstallationResponse {
  installation: GitHubInstallation;
  repositories: GitHubRepository[];
}

export interface GitHubInstallation {
  id: string;
  github_installation_id: number;
  github_account_id: number;
  github_account_login: string;
  github_account_type: "User" | "Organization";
  repository_selection: "all" | "selected";
  status: string;
  installed_by_user_id?: string;
  updated_at: string;
}

export interface GitHubRepository {
  id: string;
  github_installation_id: number;
  github_repository_id: number;
  full_name: string;
  owner_login: string;
  name: string;
  private: boolean;
  default_branch: string;
  html_url: string;
  clone_url: string;
  permissions: unknown;
  last_synced_at: string;
}

export interface CreateCISetupPullRequestRequest {
  github_repository_id: number;
  github_installation_id?: number;
  base_branch: string;
  title?: string;
  body?: string;
  draft?: boolean;
  check_only?: boolean;
  overwrite_existing?: boolean;
  files: Array<{
    path: string;
    content: string;
  }>;
}

export interface CISetupFileConflict {
  path: string;
  exists: boolean;
  sha?: string;
}

export interface CreateCISetupPullRequestResponse {
  pull_request?: {
    number: number;
    html_url: string;
    state: string;
    draft: boolean;
  };
  branch: string;
  base_branch: string;
  files: Array<{
    path: string;
  }>;
  conflicts?: CISetupFileConflict[];
}

export interface CIProfile {
  id: string;
  workspace_id: string;
  name: string;
  repository_full_name: string;
  github_repository_id?: number;
  github_installation_id?: number;
  default_branch: string;
  manifest_path: string;
  workflow_path: string;
  config: unknown;
  created_by_user_id?: string;
  created_at: string;
  updated_at: string;
}

export interface SaveCIProfileRequest {
  name: string;
  repository_full_name: string;
  github_repository_id?: number;
  github_installation_id?: number;
  default_branch: string;
  manifest_path: string;
  workflow_path: string;
  config: unknown;
}

export type AgentHarnessExecutionStatus =
  | "queued"
  | "provisioning"
  | "running"
  | "scoring"
  | "completed"
  | "failed"
  | "cancelled";

export interface AgentHarnessExecutionEvent {
  id: string | number;
  agent_harness_execution_id: string;
  sequence_number: number;
  event_type: string;
  actor_type: string;
  occurred_at: string;
  payload: unknown;
}

/** GET /v1/workspaces/{id}/agent-harness-executions item, POST response */
export interface AgentHarnessExecution {
  id: string;
  organization_id: string;
  workspace_id: string;
  agent_harness_id: string;
  run_id?: string;
  run_agent_id?: string;
  evaluation_spec_id?: string;
  status: AgentHarnessExecutionStatus;
  status_reason?: string;
  error_message?: string;
  failure_stage?: "setup" | "agent" | "validator" | "repository" | "infrastructure";
  harness_snapshot: unknown;
  execution_config_snapshot: unknown;
  evaluation_config_snapshot: unknown;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
  cancelled_at?: string;
  events?: AgentHarnessExecutionEvent[];
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

/** GET /v1/workspaces/{id}/tools list item */
export interface WorkspaceTool {
  id: string;
  name: string;
  slug: string;
  tool_kind: string;
  capability_key: string;
  lifecycle_status: string;
  created_at: string;
}

/** GET /v1/workspaces/{id}/knowledge-sources list item */
export interface KnowledgeSource {
  id: string;
  name: string;
  slug: string;
  source_kind: string;
  lifecycle_status: string;
  created_at: string;
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
  provider_account_id?: string;
  model_catalog_entry_id: string;
  provider_key: string;
  provider_model_id: string;
  model_display_name: string;
  alias_key: string;
  display_name: string;
  status: string;
  input_cost_per_million_tokens: number;
  output_cost_per_million_tokens: number;
  catalog_input_cost_per_million_tokens: number;
  catalog_output_cost_per_million_tokens: number;
  pricing_drift_warning?: string;
  created_at: string;
  updated_at: string;
}

// --- Challenge Packs ---

/** GET /v1/workspaces/{id}/challenge-packs list item */
export interface ChallengePack {
  id: string;
  name: string;
  slug: string;
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
  deployment_defaults?: unknown;
  modality?: string;
  interface_transports?: string[];
  created_at: string;
  updated_at: string;
}

export interface ChallengeInputSetSummary {
  id: string;
  challenge_pack_version_id: string;
  input_key: string;
  name: string;
}

// --- Public Shares ---

export type PublicShareResourceType =
  | "challenge_pack_version"
  | "run_scorecard"
  | "run_agent_scorecard"
  | "run_agent_replay";

export interface PublicShareLink {
  id: string;
  resource_type: PublicShareResourceType;
  resource_id: string;
  search_indexing: boolean;
  view_count: number;
  expires_at?: string;
  created_at: string;
  updated_at: string;
  url?: string;
}

export interface CreatePublicShareLinkResponse {
  share: PublicShareLink;
  token: string;
  url: string;
}

export interface PublicShareResponse {
  share: PublicShareLink;
  resource: {
    type: PublicShareResourceType;
    [key: string]: unknown;
  };
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
  official_pack_mode: OfficialPackMode;
  name: string;
  status: RunStatus;
  execution_mode: string; // "single_agent" | "comparison"
  mode?: string;
  modality?: string;
  voice?: RunVoiceMetadata;
  race_context: boolean;
  race_context_min_step_gap?: number;
  ci_metadata?: RunCIMetadata;
  temporal_workflow_id?: string;
  temporal_run_id?: string;
  queued_at?: string;
  started_at?: string;
  finished_at?: string;
  cancelled_at?: string;
  failed_at?: string;
  created_at: string;
  updated_at: string;
  regression_coverage?: RunRegressionCoverage;
  links: {
    self: string;
    agents: string;
  };
}

export interface RunCIMetadata {
  provider?: string;
  repository?: string;
  pull_request_number?: number;
  branch?: string;
  ref?: string;
  commit_sha?: string;
  workflow?: string;
  workflow_run_id?: string;
  workflow_run_attempt?: string;
  workflow_run_url?: string;
  event_name?: string;
  default_branch?: string;
}

export interface RunVoiceMetadata {
  mode: string;
  modality: string;
  transport?: string;
}

export interface RunRegressionCoverage {
  suites: RunRegressionCoverageSuite[];
  unmatched_cases: RunRegressionCoverageCase[];
}

export interface RunRegressionCoverageSuite {
  id: string;
  name: string;
  case_count: number;
  pass_count: number;
  fail_count: number;
}

export interface RunRegressionCoverageCase {
  id: string;
  title: string;
  outcome: "pending" | "pass" | "fail";
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

export type OfficialPackMode = "full" | "suite_only";

/** POST /v1/runs request */
export interface CreateRunRequest {
  workspace_id: string;
  challenge_pack_version_id: string;
  challenge_input_set_id?: string;
  name?: string;
  mode?: string;
  agent_deployment_ids: string[];
  regression_suite_ids?: string[];
  regression_case_ids?: string[];
  official_pack_mode?: OfficialPackMode;
  include_proposed_regressions?: boolean;
  race_context?: boolean;
  race_context_min_step_gap?: number;
  ci_metadata?: RunCIMetadata;
}

/** POST /v1/runs response (201) */
export interface CreateRunResponse {
  id: string;
  workspace_id: string;
  challenge_pack_version_id: string;
  challenge_input_set_id?: string;
  official_pack_mode: OfficialPackMode;
  status: RunStatus;
  execution_mode: string;
  mode?: string;
  modality?: string;
  voice?: RunVoiceMetadata;
  created_at: string;
  queued_at?: string;
  ci_metadata?: RunCIMetadata;
  links: {
    self: string;
    agents: string;
  };
}

// --- Eval Sessions ---

export type EvalSessionStatus =
  | "queued"
  | "running"
  | "aggregating"
  | "completed"
  | "failed"
  | "cancelled";

export type EvalSessionAggregationMethod = "median" | "mean" | "weighted_mean";

export interface EvalSessionAggregationConfig {
  schema_version?: number;
  method: EvalSessionAggregationMethod;
  report_variance: boolean;
  confidence_interval: number;
  reliability_weight?: number;
}

export interface EvalSessionSuccessThresholdConfig {
  schema_version?: number;
  min_pass_rate: number;
  require_all_dimensions?: string[];
}

export interface EvalSessionTaskProperties {
  has_side_effects?: boolean;
  autonomy?: "human" | "semi" | "full";
  step_count?: number;
  output_type?: "artifact" | "action";
}

export interface EvalSessionRoutingTaskSnapshot {
  schema_version?: number;
  routing: Record<string, unknown>;
  task: Record<string, unknown> & {
    task_properties?: EvalSessionTaskProperties;
  };
}

export interface EvalSessionResponse {
  id: string;
  status: EvalSessionStatus;
  repetitions: number;
  aggregation_config: EvalSessionAggregationConfig;
  success_threshold_config: EvalSessionSuccessThresholdConfig | Record<string, never>;
  routing_task_snapshot: EvalSessionRoutingTaskSnapshot;
  schema_version: number;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  updated_at: string;
}

export interface EvalSessionParticipantInput {
  agent_deployment_id: string;
  agent_build_version_id?: string;
  label: string;
}

export interface CreateEvalSessionConfig {
  repetitions: number;
  aggregation: EvalSessionAggregationConfig;
  success_threshold?: EvalSessionSuccessThresholdConfig | null;
  routing_task_snapshot: EvalSessionRoutingTaskSnapshot;
  schema_version: number;
}

export interface CreateEvalSessionRequest {
  workspace_id: string;
  challenge_pack_version_id: string;
  challenge_input_set_id?: string;
  participants: EvalSessionParticipantInput[];
  execution_mode?: "single_agent" | "comparison";
  name?: string;
  eval_session: CreateEvalSessionConfig;
}

export interface CreateEvalSessionResponse {
  eval_session: EvalSessionResponse;
  run_ids: string[];
}

export interface EvalSessionValidationDetail {
  field: string;
  code: string;
  message: string;
}

export interface EvalSessionValidationEnvelope {
  errors: EvalSessionValidationDetail[];
}

export interface EvalSessionRunCounts {
  total: number;
  draft: number;
  queued: number;
  provisioning: number;
  running: number;
  scoring: number;
  completed: number;
  failed: number;
  cancelled: number;
}

export interface EvalSessionRunSummary {
  run_counts: EvalSessionRunCounts;
}

export interface EvalSessionChildRun {
  id: string;
  workspace_id: string;
  challenge_pack_version_id: string;
  challenge_input_set_id?: string;
  eval_session_id?: string;
  official_pack_mode: OfficialPackMode;
  name: string;
  status: RunStatus;
  execution_mode: string;
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

export interface EvalSessionAggregateInterval {
  estimator: string;
  lower: number;
  upper: number;
}

export interface EvalSessionMetricAggregate {
  n: number;
  mean: number;
  median: number;
  std_dev: number;
  min: number;
  max: number;
  interval?: EvalSessionAggregateInterval;
  high_variance: boolean;
  high_variance_rule: string;
}

export interface EvalSessionPassMetricSeries {
  effective_k: number;
  by_k: Record<string, EvalSessionMetricAggregate>;
}

export interface EvalSessionMetricRouting {
  source: string;
  reliability_weight: number;
  reasoning: string;
  primary_metric: "pass_at_k" | "pass_pow_k";
  effective_k: number;
  composite_agent_score: number;
  composite_interval?: EvalSessionAggregateInterval;
}

export interface EvalSessionTaskSuccess {
  task_key: string;
  challenge_identity_id?: string;
  challenge_key?: string;
  title?: string;
  observed_trials: number;
  successful_trials: number;
  success_rate: number;
  source: string;
  pass_at_k?: Record<string, number>;
  pass_pow_k?: Record<string, number>;
}

export interface EvalSessionParticipantAggregate {
  lane_index: number;
  label: string;
  overall?: EvalSessionMetricAggregate;
  dimensions?: Record<string, EvalSessionMetricAggregate>;
  task_success?: EvalSessionTaskSuccess[];
  pass_at_k?: EvalSessionPassMetricSeries;
  pass_pow_k?: EvalSessionPassMetricSeries;
  metric_routing?: EvalSessionMetricRouting;
}

export interface EvalSessionRepeatedComparison {
  status: string;
  reason_code?: string;
  compared_metric?: string;
  effective_k: number;
  winner_lane_index?: number;
  winner_label?: string;
  leader_lane_index?: number;
  leader_label?: string;
  leader_value?: number;
  leader_interval?: EvalSessionAggregateInterval;
  runner_up_lane_index?: number;
  runner_up_label?: string;
  runner_up_value?: number;
  runner_up_interval?: EvalSessionAggregateInterval;
}

export interface EvalSessionAggregateResult {
  schema_version: number;
  child_run_count: number;
  scored_child_count: number;
  top_level_source?: string;
  overall?: EvalSessionMetricAggregate;
  dimensions?: Record<string, EvalSessionMetricAggregate>;
  task_success?: EvalSessionTaskSuccess[];
  pass_at_k?: EvalSessionPassMetricSeries;
  pass_pow_k?: EvalSessionPassMetricSeries;
  metric_routing?: EvalSessionMetricRouting;
  participants?: EvalSessionParticipantAggregate[];
  comparison?: EvalSessionRepeatedComparison;
}

export interface EvalSessionListItem {
  eval_session: EvalSessionResponse;
  summary: EvalSessionRunSummary;
  aggregate_result: EvalSessionAggregateResult | null;
  evidence_warnings: string[];
}

export interface ListEvalSessionsResponse {
  items: EvalSessionListItem[];
  limit: number;
  offset: number;
}

export interface EvalSessionDetail {
  eval_session: EvalSessionResponse;
  runs: EvalSessionChildRun[];
  summary: EvalSessionRunSummary;
  aggregate_result: EvalSessionAggregateResult | null;
  evidence_warnings: string[];
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
  strategy?: string;
  passed?: boolean;
  overall_reason?: string;
  composite_score?: number;
  overall_score?: number;
  correctness_score?: number;
  reliability_score?: number;
  latency_score?: number;
  cost_score?: number;
  cost_per_correct_usd?: number;
  dimensions?: Record<
    string,
    { state: string; score?: number; better_direction?: string }
  >;
}

export interface CreateRunRankingInsightsRequest {
  provider_account_id: string;
  model_alias_id: string;
}

export interface RunRankingInsightsResponse {
  generated_at: string;
  grounding_scope: string;
  provider_key: string;
  provider_model_id: string;
  recommended_winner: RunRankingInsightCandidate;
  why_it_won: string;
  tradeoffs: string[];
  best_for_reliability?: RunRankingInsightRecommendation;
  best_for_cost?: RunRankingInsightRecommendation;
  best_for_latency?: RunRankingInsightRecommendation;
  model_summaries: RunRankingModelInsight[];
  recommended_next_step: string;
  confidence_notes: string;
}

export interface RunRankingInsightCandidate {
  run_agent_id: string;
  label: string;
}

export interface RunRankingInsightRecommendation {
  run_agent_id: string;
  label: string;
  reason: string;
}

export interface RunRankingModelInsight {
  run_agent_id: string;
  label: string;
  strongest_dimension: string;
  weakest_dimension: string;
  summary: string;
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
  turn_index?: number;
  mismatch?: boolean;
  provider_key?: string;
  provider_model_id?: string;
  tool_name?: string;
  sandbox_action?: string;
  metric_key?: string;
  final_output?: string;
  model_output?: string;
  tool_result?: string;
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

// --- Multi-turn transcript ---

/** One reconstructed user↔assistant turn. Mirrors transcriptTurnPayload in Go. */
export interface TranscriptTurn {
  turn_index: number;
  phase_id?: string;
  actor?: string; // "scripted" | "llm" | "human"
  user_message?: string;
  assistant_message?: string;
  mismatch: boolean;
  completed: boolean;
  awaiting_human: boolean;
  awaiting_human_hint?: string;
  user_simulated: boolean;
}

/** GET /v1/replays/{runAgentID}/transcript — mirrors getRunAgentTranscriptResponse in Go. */
export interface TranscriptResponse {
  state: ReplayState; // "ready" | "pending" | "errored"
  message?: string;
  run_agent_id: string;
  run_id: string;
  run_agent_status: RunAgentStatus;
  turn_count: number;
  turns: TranscriptTurn[];
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
  total_cost_usd?: number;
  cost_per_correct_usd?: number;
  behavioral_score?: number;
  llm_judge_results: LLMJudgeResult[];
  scorecard: ScorecardDocument;
  created_at: string;
  updated_at: string;
}

export interface ScorecardDocument {
  run_agent_id: string;
  evaluation_spec_id: string;
  status: "complete" | "partial" | "failed";
  strategy?: string;
  overall_score?: number;
  passed?: boolean;
  overall_reason?: string;
  warnings?: string[];
  dimensions: Record<string, ScorecardDimension>;
  validator_summary: Record<string, number>;
  validator_details?: ValidatorDetail[];
  metric_summary: Record<string, number>;
  metric_details?: MetricDetail[];
  side_metrics?: Record<string, SideMetricDetail>;
}

export interface SideMetricDetail {
  state: string;
  value?: number;
  unit?: string;
  numerator?: number;
  denominator?: number;
  reason?: string;
}

export interface ValidatorDetail {
  key: string;
  type: string;
  verdict: string;
  state: string;
  reason?: string;
  normalized_score?: number;
  evidence?: ValidatorEvidence;
  source?: ScorecardSource;
}

export interface ScorecardSource {
  kind: "run_event" | "tool_call" | "model_call" | "final_output";
  sequence?: number;
  event_type?: string;
  field_path?: string;
}

export type ValidatorEvidence =
  | ValidatorTextCompareEvidence
  | ValidatorRegexEvidence
  | ValidatorJSONSchemaEvidence
  | ValidatorJSONPathEvidence
  | ValidatorToolCallAssertionEvidence
  | ValidatorCustomEvidence;

export interface ValidatorTextCompareEvidence {
  kind: "text_compare";
  expected?: string;
  actual?: string;
  source_field?: string;
}

export interface ValidatorRegexEvidence {
  kind: "regex_match";
  pattern?: string;
  actual?: string;
  matched?: boolean;
  source_field?: string;
}

export interface ValidatorJSONSchemaEvidence {
  kind: "json_schema";
  schema_ref?: string;
  actual?: string;
  validation_errors?: string[];
  source_field?: string;
}

export interface ValidatorJSONPathEvidence {
  kind: "json_path_match";
  path?: string;
  comparator?: string;
  actual?: unknown;
  expected?: unknown;
  exists?: boolean;
  source_field?: string;
}

export interface ValidatorToolCallAssertionEvidence {
  kind: "tool_call_assertion";
  source_field?: string;
  tool_name?: string;
  observed_count?: number;
  failed_count?: number;
  matched_count?: number;
  matched_indices?: number[];
  observed_tool_names?: string[];
  expected_count?: number;
  expected_min_count?: number;
  expected_max_count?: number;
  expected_order?: string[];
  expected_order_mode?: string;
  arguments_contain_set?: boolean;
  matched?: boolean;
}

export interface ValidatorCustomEvidence {
  kind: "custom";
  raw?: unknown;
}

export interface MetricDetail {
  key: string;
  collector: string;
  state: string;
  reason?: string;
  numeric_value?: number;
  text_value?: string;
  boolean_value?: boolean;
}

export interface LLMJudgeResult {
  id: string;
  judge_key: string;
  mode: string;
  normalized_score?: number;
  confidence?: string;
  variance?: number;
  sample_count: number;
  model_count: number;
  payload: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface ScorecardDimension {
  state: "available" | "unavailable" | "error";
  score?: number;
  reason?: string;
  better_direction?: string;
  weight?: number;
  contribution?: number;
  pass_threshold?: number;
  gate?: boolean;
  gate_passed?: boolean;
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
  require_scorecard_pass?: boolean;
  required_dimensions?: string[];
  dimensions?: Record<string, DimensionThreshold>;
  regression_gate_rules?: RegressionGateRules;
}

export interface DimensionThreshold {
  warn_delta?: number;
  fail_delta?: number;
}

/** Mirrors RegressionGateRules in backend/internal/releasegate/releasegate.go. */
export interface RegressionGateRules {
  no_blocking_regression_failure?: boolean;
  no_new_blocking_failure_vs_baseline?: boolean;
  max_warning_regression_failures?: number;
  suite_ids?: string[];
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
  regression_violations?: ReleaseGateRegressionViolation[];
}

/** Mirrors RegressionGateViolation in backend/internal/releasegate/regression_evaluator.go. */
export interface ReleaseGateRegressionViolation {
  rule: string; // "no_blocking_regression_failure" | "no_new_blocking_failure_vs_baseline" | "max_warning_regression_failures"
  severity: string; // "info" | "warning" | "blocking"
  regression_case_id: string;
  suite_id: string;
  observed_count?: number;
  evidence: ReleaseGateRegressionEvidence;
}

export interface ReleaseGateRegressionEvidence {
  scoring_result_id: string;
  scoring_result_type: string;
  replay_step_refs?: ReleaseGateReplayStepRef[];
}

export interface ReleaseGateReplayStepRef {
  sequence_number: number;
  event_type?: string;
  kind?: string;
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

// --- Failure Review ---
// Mirrors schemas under `FailureReview*` in docs/api-server/openapi.yaml.

export type FailureReviewFailureState =
  | "failed"
  | "warning"
  | "flaky"
  | "incomplete_evidence";

export type FailureReviewFailureClass =
  | "incorrect_final_output"
  | "tool_selection_error"
  | "tool_argument_error"
  | "retrieval_grounding_failure"
  | "policy_violation"
  | "timeout_or_budget_exhaustion"
  | "sandbox_failure"
  | "dependency_resolution_failure"
  | "malformed_output"
  | "flaky_non_deterministic"
  | "insufficient_evidence"
  | "other";

export type FailureReviewEvidenceTier =
  | "none"
  | "native_structured"
  | "hosted_structured"
  | "hosted_black_box"
  | "derived_summary";

export type FailureReviewPromotionMode = "full_executable" | "output_only";

export type FailureReviewSeverity = "info" | "warning" | "blocking";

export type FailureReviewRemediationArea =
  | "prompt_or_model"
  | "output_contract"
  | "tool_or_workflow"
  | "runtime_or_infra"
  | "retrieval_or_data"
  | "flakiness"
  | "evidence_gap";

export type FailureReviewTaxonomyFamily =
  | "agent"
  | "workflow"
  | "platform"
  | "evidence";

export interface FailureReviewTaxonomy {
  family: FailureReviewTaxonomyFamily;
  code: string;
  label: string;
  agent_fault: boolean;
}

export interface FailureReviewRemediationHint {
  area: FailureReviewRemediationArea;
  label: string;
  summary: string;
  evidence: string[];
}

export interface FailureReviewReplayStepRef {
  sequence_number: number;
  event_type: string;
  kind: string;
}

export interface FailureReviewArtifactRef {
  key: string;
  kind?: string;
  path?: string;
  media_type?: string;
}

export interface FailureReviewJudgeRef {
  key: string;
  kind: string;
  verdict?: string;
  state?: string;
  normalized_score?: number;
  reason?: string;
  sequence_number?: number;
  event_type?: string;
}

export interface FailureReviewMetricRef {
  key: string;
  metric_type: string;
  state?: string;
  reason?: string;
  numeric_value?: number;
  text_value?: string;
  boolean_value?: boolean;
  unit?: string;
}

export interface FailureReviewItem {
  run_id: string;
  run_agent_id: string;
  challenge_identity_id?: string;
  challenge_key: string;
  case_key: string;
  item_key: string;
  failure_fingerprint: string;
  failure_cluster_key: string;
  failure_state: FailureReviewFailureState;
  failed_dimensions: string[];
  failed_checks: string[];
  failure_class: FailureReviewFailureClass;
  failure_taxonomy: FailureReviewTaxonomy;
  headline: string;
  detail: string;
  recommended_action: string;
  remediation: FailureReviewRemediationHint;
  promotable: boolean;
  promotion_mode_available: FailureReviewPromotionMode[];
  replay_step_refs: FailureReviewReplayStepRef[];
  artifact_refs: FailureReviewArtifactRef[];
  judge_refs: FailureReviewJudgeRef[];
  metric_refs: FailureReviewMetricRef[];
  evidence_tier: FailureReviewEvidenceTier;
  severity: FailureReviewSeverity;
}

export type FailureReviewClusterTrend =
  | "new"
  | "recurring"
  | "increasing"
  | "decreasing";

export interface FailureReviewClusterHistory {
  trend: FailureReviewClusterTrend;
  window_run_count: number;
  prior_run_count: number;
  prior_failure_count: number;
  last_seen_run_id?: string;
  last_seen_at?: string;
  last_run_failure_count: number;
}

export interface FailureReviewClusterSummary {
  failure_cluster_key: string;
  representative_failure_fingerprint: string;
  count: number;
  promotable_count: number;
  severity: FailureReviewSeverity;
  failure_state: FailureReviewFailureState;
  failure_class: FailureReviewFailureClass;
  failure_taxonomy: FailureReviewTaxonomy;
  evidence_tier: FailureReviewEvidenceTier;
  challenge_keys: string[];
  case_keys: string[];
  run_agent_ids: string[];
  headline: string;
  recommended_action: string;
  remediation: FailureReviewRemediationHint;
  history?: FailureReviewClusterHistory;
}

export interface ListRunFailuresResponse {
  items: FailureReviewItem[];
  clusters: FailureReviewClusterSummary[];
  next_cursor?: string;
}

// --- Regression Suites & Cases ---
// Mirrors schemas under `RegressionSuite*` / `RegressionCase*` in docs/api-server/openapi.yaml.

export type RegressionSuiteStatus = "active" | "archived";
export type RegressionCaseStatus =
  | "proposed"
  | "active"
  | "muted"
  | "archived"
  | "rejected";
export type RegressionSeverity = "info" | "warning" | "blocking";
export type RegressionPromotionMode =
  | "full_executable"
  | "output_only"
  | "manual";
export type RegressionSourceMode = "derived_only" | "mixed_manual";
export type RegressionCaseValidationStatus =
  | "not_validated"
  | "collecting_signal"
  | "reproducing"
  | "passing"
  | "flaky";
export type RegressionCaseMaintenanceStatus =
  | "needs_signal"
  | "keep_active"
  | "prune_candidate"
  | "review_flaky";

export interface RegressionPromotion {
  id: string;
  workspace_regression_case_id: string;
  source_run_id: string;
  source_run_agent_id: string;
  source_event_refs: unknown[];
  promoted_by_user_id: string;
  promotion_reason: string;
  promotion_snapshot: Record<string, unknown>;
  created_at: string;
}

export interface RegressionCaseValidation {
  status: RegressionCaseValidationStatus;
  maintenance_status: RegressionCaseMaintenanceStatus;
  run_count: number;
  failure_count: number;
  pass_count: number;
  reproduction_rate?: number;
  reproduction_threshold: number;
  required_runs: number;
  remaining_runs: number;
  last_outcome?: "pass" | "fail";
  last_validated_at?: string;
  recommended_action: string;
  maintenance_action: string;
}

/** GET /v1/workspaces/{ws}/regression-suites list item, POST response, PATCH response */
export interface RegressionSuite {
  id: string;
  workspace_id: string;
  source_challenge_pack_id: string;
  name: string;
  description: string;
  status: RegressionSuiteStatus;
  source_mode: RegressionSourceMode;
  default_gate_severity: RegressionSeverity;
  case_count: number;
  created_by_user_id: string;
  created_at: string;
  updated_at: string;
}

/** GET /v1/workspaces/{ws}/regression-suites/{id}/cases list item, PATCH response */
export interface RegressionCase {
  id: string;
  suite_id: string;
  workspace_id: string;
  suite_name?: string;
  title: string;
  description: string;
  status: RegressionCaseStatus;
  severity: RegressionSeverity;
  promotion_mode: RegressionPromotionMode;
  source_run_id?: string;
  source_run_agent_id?: string;
  source_replay_id?: string;
  source_challenge_pack_version_id: string;
  source_challenge_input_set_id?: string;
  source_challenge_identity_id: string;
  source_challenge_key?: string;
  source_case_key: string;
  source_item_key?: string;
  source_failure_fingerprint?: string;
  source_failure_cluster_key?: string;
  evidence_tier: string;
  failure_class: string;
  failure_summary: string;
  payload_snapshot: Record<string, unknown>;
  expected_contract: Record<string, unknown>;
  validator_overrides?: Record<string, unknown> | null;
  metadata: Record<string, unknown>;
  latest_promotion?: RegressionPromotion;
  validation: RegressionCaseValidation;
  created_at: string;
  updated_at: string;
}

/** POST /v1/workspaces/{ws}/regression-suites request */
export interface CreateRegressionSuiteInput {
  source_challenge_pack_id: string;
  name: string;
  description?: string;
  default_gate_severity?: RegressionSeverity;
}

/** PATCH /v1/workspaces/{ws}/regression-suites/{id} request */
export interface PatchRegressionSuiteInput {
  name?: string;
  description?: string;
  status?: RegressionSuiteStatus;
  default_gate_severity?: RegressionSeverity;
}

/** PATCH /v1/workspaces/{ws}/regression-cases/{id} request */
export interface PatchRegressionCaseInput {
  title?: string;
  description?: string;
  status?: RegressionCaseStatus;
  severity?: RegressionSeverity;
}

/** GET /v1/workspaces/{ws}/regression-suites/{id}/cases response */
export interface ListRegressionCasesResponse {
  items: RegressionCase[];
}

/** GET /v1/workspaces/{ws}/regression-cases response */
export interface ListWorkspaceRegressionCasesResponse {
  items: RegressionCase[];
  total: number;
  limit: number;
  offset: number;
}

// --- Datasets ---

export type DatasetExampleStatus = "active" | "archived" | "muted";
export type DatasetExampleSource =
  | "manual"
  | "import"
  | "trace"
  | "synthetic"
  | "promotion";

export interface Dataset {
  id: string;
  organization_id: string;
  workspace_id: string;
  slug: string;
  name: string;
  description: string;
  input_schema?: Record<string, unknown>;
  input_schema_enforced: boolean;
  default_challenge_pack_version_id?: string;
  active_example_count: number;
  version_count: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  archived_at?: string;
}

export interface DatasetExample {
  id: string;
  dataset_id: string;
  external_id?: string;
  input: unknown;
  expected?: unknown;
  metadata: Record<string, unknown>;
  tags: string[];
  status: DatasetExampleStatus;
  source: DatasetExampleSource;
  source_run_id?: string;
  source_trace_id?: string;
  source_platform?: string;
  artifact_id?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface DatasetVersion {
  id: string;
  dataset_id: string;
  version_number: number;
  label?: string;
  example_count: number;
  manifest_checksum: string;
  created_by: string;
  created_at: string;
}

export interface DatasetBaseline {
  id: string;
  dataset_id: string;
  dataset_version_id: string;
  dataset_version_input_set_id?: string;
  challenge_pack_version_id: string;
  challenge_key: string;
  agent_deployment_id?: string;
  run_id: string;
  pass_rate?: number;
  metrics: Record<string, unknown>;
  example_outcomes: Record<string, unknown>[];
  label?: string;
  created_by: string;
  created_at: string;
}

export interface ListDatasetBaselinesResponse {
  items: DatasetBaseline[];
  total: number;
  limit: number;
  offset: number;
}

export interface DatasetRegressionSuiteLink {
  dataset_id: string;
  regression_suite_id: string;
  synced_version_id?: string;
  created_at: string;
  updated_at: string;
}

// --- Billing ---

export type BillingPlanKey = "free" | "pro" | "team" | "enterprise";
export type BillingPeriod = "monthly" | "yearly" | "custom";
export type BillingStatus = "active" | "trialing" | "expired" | "inactive" | string;

export interface BillingLimit {
  value?: number;
  per_seat?: boolean;
  unlimited?: boolean;
  custom?: boolean;
}

export interface BillingPlanLimits {
  seats: BillingLimit;
  workspaces: BillingLimit;
  races_per_workspace_month: BillingLimit;
  max_models_per_race: BillingLimit;
  replay_retention_days: BillingLimit;
  concurrent_races: BillingLimit;
}

export interface BillingPlan {
  key: BillingPlanKey;
  display_name: string;
  minimum_seats: number;
  default_seats: number;
  billing_periods: BillingPeriod[];
  limits: BillingPlanLimits;
  feature_flags: Record<string, boolean>;
  upgrade_target?: BillingPlanKey;
  dodo_product_ids?: Partial<Record<BillingPeriod, string>>;
}

export interface BillingPlansResponse {
  items: BillingPlan[];
}

export interface EffectiveEntitlements {
  plan_key: BillingPlanKey;
  billing_period: BillingPeriod;
  status: BillingStatus;
  seat_quantity: number;
  seats_limit?: number | null;
  workspaces_limit?: number | null;
  races_per_workspace_month?: number | null;
  max_models_per_race?: number | null;
  replay_retention_days?: number | null;
  concurrent_races?: number | null;
  feature_flags: Record<string, boolean>;
  upgrade_target?: BillingPlanKey;
  expires_at?: string;
}

export interface BillingSubscription {
  id: string;
  organization_id: string;
  dodo_subscription_id: string;
  dodo_customer_id?: string;
  dodo_product_id: string;
  plan_key: BillingPlanKey;
  billing_period: BillingPeriod;
  status: BillingStatus;
  next_billing_date?: string;
  cancel_at_next_billing_date: boolean;
  cancelled_at?: string;
  expires_at?: string;
  trial_period_days?: number;
  seat_quantity: number;
  addon_quantities: unknown;
  latest_dodo_event_at?: string;
  created_at: string;
  updated_at: string;
}

export interface BillingAccount {
  id: string;
  organization_id: string;
  dodo_customer_id?: string;
  billing_email?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface BillingCheckoutIntent {
  id: string;
  organization_id: string;
  requested_plan_key: "pro" | "team";
  billing_period: "monthly" | "yearly";
  seat_quantity: number;
  return_url: string;
  checkout_url: string;
  dodo_checkout_session_id?: string;
  status: string;
  metadata: unknown;
  created_at: string;
  updated_at: string;
}

export interface BillingOverviewResponse {
  entitlements: EffectiveEntitlements;
  account?: BillingAccount;
  subscription?: BillingSubscription;
  latest_checkout_intent?: BillingCheckoutIntent;
}

export interface StartBillingTrialRequest {
  plan_key: "pro" | "team";
  billing_period?: BillingPeriod;
}

export interface CreateBillingCheckoutRequest {
  plan_key: "pro" | "team";
  billing_period: "monthly" | "yearly";
  seat_quantity: number;
  return_url: string;
}

export interface CreateBillingCheckoutResponse {
  checkout_intent_id: string;
  checkout_url: string;
  plan_key: "pro" | "team";
  billing_period: "monthly" | "yearly";
  seat_quantity: number;
}

export interface CreateBillingPortalResponse {
  portal_url: string;
}

export interface BillingGateDecision {
  allowed: boolean;
  code?: string;
  message?: string;
  plan_key: BillingPlanKey;
  upgrade_target?: BillingPlanKey;
  limit?: number | null;
  used?: number;
  remaining?: number | null;
  reset_at?: string;
  expires_at?: string;
}

export interface WorkspaceUsageSnapshot {
  workspace_id: string;
  race_count: number;
  active_runs: number;
  window_start: string;
  window_end: string;
}

export interface WorkspaceEntitlementsResponse {
  organization_id: string;
  workspace_id: string;
  entitlements: EffectiveEntitlements;
  usage: WorkspaceUsageSnapshot;
  gates: {
    run: BillingGateDecision;
  };
}

// --- Errors ---

/** Standard error envelope returned by all backend error responses. */
export interface ApiErrorResponse {
  error: {
    code: string;
    message: string;
    plan_key?: BillingPlanKey;
    upgrade_target?: BillingPlanKey;
    limit?: number | null;
    used?: number;
    remaining?: number | null;
    reset_at?: string;
    expires_at?: string;
  };
}
