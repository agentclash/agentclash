import type { RunAgentStatus, RunStatus } from "@/lib/api/types";

export const ACTIVE_RUN_STATUSES: RunStatus[] = [
  "queued",
  "provisioning",
  "running",
  "scoring",
];

export const ACTIVE_AGENT_STATUSES: RunAgentStatus[] = [
  "queued",
  "ready",
  "executing",
  "evaluating",
];

export function isRunActive(status: RunStatus): boolean {
  return ACTIVE_RUN_STATUSES.includes(status);
}

export function isAgentAwaitingHumanInput(status: RunAgentStatus): boolean {
  return status === "executing" || status === "evaluating";
}
