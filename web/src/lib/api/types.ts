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

// --- Workspace Secrets ---

/** GET /v1/workspaces/{id}/secrets list item — metadata only, never the value */
export interface WorkspaceSecret {
  key: string;
  created_at: string;
  updated_at: string;
  created_by?: string;
  updated_by?: string;
}

// --- Errors ---

/** Standard error envelope returned by all backend error responses. */
export interface ApiErrorResponse {
  error: {
    code: string;
    message: string;
  };
}
