import type { RunAgentStatus, RunStatus } from "@/lib/api/types";
import { nonTerminalStates, terminalStates } from "@/lib/status-lifecycles";

// Read from the server lifecycles (runtime/domain/run.go) through
// status-lifecycles.ts.

/** A run that was submitted and has not ended. A draft run is not active. */
export const ACTIVE_RUN_STATUSES: RunStatus[] = nonTerminalStates("run").filter(
  (status) => status !== "draft",
);

export const TERMINAL_RUN_STATUSES: RunStatus[] = [...terminalStates("run")];

export const ACTIVE_AGENT_STATUSES: RunAgentStatus[] =
  nonTerminalStates("run_agent");

export const TERMINAL_AGENT_STATUSES: RunAgentStatus[] = [
  ...terminalStates("run_agent"),
];

export function isRunActive(status: RunStatus): boolean {
  return ACTIVE_RUN_STATUSES.includes(status);
}

export function isRunTerminal(status: RunStatus): boolean {
  return TERMINAL_RUN_STATUSES.includes(status);
}

export function isAgentTerminal(status: RunAgentStatus): boolean {
  return TERMINAL_AGENT_STATUSES.includes(status);
}

/**
 * Whether the run page should treat the run as live (spinner, polling, SSE).
 * A terminal run status wins over agent statuses: when a run is cancelled the
 * backend leaves its agents in queued or executing, and they never move again.
 */
export function isRunLive(
  runStatus: RunStatus,
  agentStatuses: readonly RunAgentStatus[],
): boolean {
  if (isRunTerminal(runStatus)) return false;
  return (
    isRunActive(runStatus) ||
    agentStatuses.some((status) => ACTIVE_AGENT_STATUSES.includes(status))
  );
}

/**
 * Whether one agent lane should render as live. An agent of a terminal run is
 * never live, even if its own status was left non-terminal.
 */
export function isAgentLive(
  agentStatus: RunAgentStatus,
  runStatus: RunStatus,
): boolean {
  if (isRunTerminal(runStatus)) return false;
  return ACTIVE_AGENT_STATUSES.includes(agentStatus);
}

export function isAgentAwaitingHumanInput(status: RunAgentStatus): boolean {
  return status === "executing" || status === "evaluating";
}
