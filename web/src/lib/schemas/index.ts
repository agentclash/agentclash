import { z } from "zod";

// ─── Run Schemas ───

export const runStatusSchema = z.enum([
  "draft", "queued", "provisioning", "running", "scoring", "completed", "failed", "cancelled",
]);

export const runAgentStatusSchema = z.enum([
  "queued", "ready", "executing", "evaluating", "completed", "failed",
]);

export const runSchema = z.object({
  id: z.string().uuid(),
  workspace_id: z.string().uuid(),
  challenge_pack_version_id: z.string().uuid(),
  challenge_input_set_id: z.string().uuid().optional(),
  name: z.string(),
  status: runStatusSchema,
  execution_mode: z.string(),
  temporal_workflow_id: z.string().optional(),
  temporal_run_id: z.string().optional(),
  queued_at: z.string().optional(),
  started_at: z.string().optional(),
  finished_at: z.string().optional(),
  cancelled_at: z.string().optional(),
  failed_at: z.string().optional(),
  created_at: z.string(),
  updated_at: z.string(),
  links: z.object({
    self: z.string(),
    agents: z.string(),
  }),
});

export const listRunsResponseSchema = z.object({
  items: z.array(runSchema),
  total: z.number(),
  limit: z.number(),
  offset: z.number(),
});

// ─── Run Agent Schemas ───

export const runAgentSchema = z.object({
  id: z.string().uuid(),
  run_id: z.string().uuid(),
  lane_index: z.number(),
  label: z.string(),
  agent_deployment_id: z.string().uuid(),
  agent_deployment_snapshot_id: z.string().uuid(),
  status: runAgentStatusSchema,
  queued_at: z.string().optional(),
  started_at: z.string().optional(),
  finished_at: z.string().optional(),
  failure_reason: z.string().optional(),
  created_at: z.string(),
  updated_at: z.string(),
});

// ─── Create Run Schemas ───

export const createRunSchema = z.object({
  workspace_id: z.string().uuid("Invalid workspace ID"),
  challenge_pack_version_id: z.string().uuid("Invalid challenge pack version"),
  challenge_input_set_id: z.string().uuid().optional(),
  name: z.string().min(1, "Name is required").max(100, "Name too long"),
  agent_deployment_ids: z.array(z.string().uuid()).min(1, "At least one agent is required"),
});

export type CreateRunForm = z.infer<typeof createRunSchema>;

// ─── Dev Auth Schema ───

export const devAuthSchema = z.object({
  userId: z.string().uuid("Must be a valid UUID"),
  email: z.string().email("Must be a valid email").optional().or(z.literal("")),
  displayName: z.string().optional().or(z.literal("")),
  workspaceMemberships: z.string().min(1, "At least one workspace membership is required"),
});

export type DevAuthForm = z.infer<typeof devAuthSchema>;
