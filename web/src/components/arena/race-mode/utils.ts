import type { ArenaLaneState } from "@/hooks/use-agent-arena";
import type { RunAgent } from "@/lib/api/types";

/**
 * Minimum number of segments we render on the step progress bar and on the
 * race track before either scales up to accommodate the live maximum. 12 is
 * a reasonable visual baseline — short enough to show meaningful segments on
 * a 3-step run, large enough not to feel sparse on typical runs.
 */
export const MIN_TARGET_STEPS = 12;

/**
 * Target step count for display. Track + lane must agree on this value or the
 * two panels render the same agent at different percentages.
 */
export function computeTargetSteps(
  agents: RunAgent[],
  lanes: Record<string, ArenaLaneState>,
): number {
  let max = MIN_TARGET_STEPS;
  for (const a of agents) {
    const lane = lanes[a.id];
    if (lane && lane.stepIndex > max) max = lane.stepIndex;
  }
  return max;
}

/**
 * Ranks agents by: (1) non-failed first, (2) higher stepIndex first,
 * (3) more model calls as tiebreak, (4) lane_index ascending.
 *
 * Used by both `RaceTrack` and `RaceModeArena` — must live in exactly one
 * place so the two views never disagree on who's ahead.
 */
export function rankAgents(
  agents: RunAgent[],
  lanes: Record<string, ArenaLaneState>,
): { agent: RunAgent; position: number }[] {
  const sorted = [...agents].sort((a, b) => {
    const aFailed = a.status === "failed" ? 1 : 0;
    const bFailed = b.status === "failed" ? 1 : 0;
    if (aFailed !== bFailed) return aFailed - bFailed;
    const aStep = lanes[a.id]?.stepIndex ?? 0;
    const bStep = lanes[b.id]?.stepIndex ?? 0;
    if (aStep !== bStep) return bStep - aStep;
    const aCalls = lanes[a.id]?.modelCalls ?? 0;
    const bCalls = lanes[b.id]?.modelCalls ?? 0;
    if (aCalls !== bCalls) return bCalls - aCalls;
    return a.lane_index - b.lane_index;
  });
  return sorted.map((agent, i) => ({ agent, position: i + 1 }));
}

/**
 * A single source of truth for "is this lane the leader?". Before the run
 * finishes (no winnerAgentId), we only flag position-1 as leader when the
 * agent is actually racing — otherwise a queued-but-first-in-order agent
 * would wear the green leader stripe before it's even started.
 */
export function deriveLeader(
  agent: RunAgent,
  position: number,
  winnerAgentId: string | undefined,
): boolean {
  if (winnerAgentId) return agent.id === winnerAgentId;
  if (position !== 1) return false;
  return agent.status === "executing" || agent.status === "evaluating";
}
