"use client";

import type { RunAgent } from "@/lib/api/types";
import type { ArenaLaneState } from "@/hooks/use-agent-arena";
import { deriveLeader } from "./utils";

/**
 * Unified horizontal timeline — all agents plotted on the same 0→N-step axis,
 * racing against each other. Position is derived from each agent's live
 * stepIndex; failed agents stop at their last-seen step.
 *
 * The parent (`RaceModeArena`) computes `ranked` + `targetSteps` once and
 * passes them down so the track and the lane rows always agree on the
 * denominator.
 */

interface RaceTrackProps {
  ranked: { agent: RunAgent; position: number }[];
  lanes: Record<string, ArenaLaneState>;
  targetSteps: number;
  winnerAgentId?: string;
}

export function RaceTrack({
  ranked,
  lanes,
  targetSteps,
  winnerAgentId,
}: RaceTrackProps) {
  const agentCount = ranked.length;
  const activeCount = ranked.filter(
    ({ agent }) =>
      agent.status === "executing" || agent.status === "evaluating",
  ).length;
  const maxStep = ranked.reduce(
    (m, { agent }) => Math.max(m, lanes[agent.id]?.stepIndex ?? 0),
    0,
  );

  return (
    <section className="rm-track" aria-label="Race track">
      <div className="rm-track__head">
        <h2 className="rm-track__title">Lap progress</h2>
        <div className="rm-track__meta">
          <span>
            {agentCount} {agentCount === 1 ? "lane" : "lanes"}
          </span>
          <span className="rm-sep">·</span>
          <span>
            {activeCount > 0
              ? `${activeCount} active`
              : maxStep > 0
                ? `step ${maxStep} / ${targetSteps}`
                : "standing by"}
          </span>
          <span className="rm-sep">·</span>
          <span className="rm-finish">finish</span>
        </div>
      </div>
      <div className="rm-track__body">
        {ranked.map(({ agent, position }) => {
          const lane = lanes[agent.id];
          const step = lane?.stepIndex ?? 0;
          const pct = clamp((step / Math.max(targetSteps, 1)) * 100, 0, 100);
          const isLeader = deriveLeader(agent, position, winnerAgentId);
          const isFailed = agent.status === "failed";

          const rowClass = [
            "rm-track-row",
            isLeader && "rm-track-row--leader",
            isFailed && "rm-track-row--failed",
          ]
            .filter(Boolean)
            .join(" ");

          const wakeLeft = Math.max(pct - 18, 0);
          const wakeWidth = Math.min(pct, 18);

          return (
            <div key={agent.id} className={rowClass}>
              <span className="rm-track-row__pos">{position}</span>
              <span className="rm-track-row__name" title={agent.label}>
                {agent.label}
              </span>
              <div className="rm-track-row__lane">
                <div
                  className="rm-track-row__fill"
                  style={{ width: `${pct}%` }}
                />
                {!isFailed && wakeWidth > 0 && (
                  <span
                    className="rm-track-row__wake"
                    style={{
                      left: `${wakeLeft}%`,
                      width: `${wakeWidth}%`,
                    }}
                  />
                )}
                <span
                  className="rm-track-row__dot"
                  style={{ left: `${pct}%` }}
                />
              </div>
              <span className="rm-track-row__step">
                {isFailed ? "DNF" : `${step}/${targetSteps}`}
              </span>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function clamp(n: number, lo: number, hi: number): number {
  return Math.min(Math.max(n, lo), hi);
}
