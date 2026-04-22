"use client";

import { useMemo } from "react";

import type { RunAgent } from "@/lib/api/types";
import type { ArenaLaneState } from "@/hooks/use-agent-arena";
import { EMPTY_LANE } from "@/hooks/use-agent-arena";
import type { CommentaryEntry } from "@/hooks/use-agent-commentary";

import { RaceCommentary } from "./race-commentary";
import { RaceLane } from "./race-lane";
import { RaceTrack } from "./race-track";
import { computeTargetSteps, deriveLeader, rankAgents } from "./utils";

import "./race-mode.css";

interface RaceModeArenaProps {
  agents: RunAgent[];
  lanes: Record<string, ArenaLaneState>;
  workspaceId: string;
  runId: string;
  winnerAgentId?: string;
  showCommentary: boolean;
  commentaryEntries: CommentaryEntry[];
  isActive: boolean;
  /** Map of agent.id → terminal-only footer node (scorecard, etc.). */
  laneFooters?: Record<string, React.ReactNode>;
}

export function RaceModeArena({
  agents,
  lanes,
  workspaceId,
  runId,
  winnerAgentId,
  showCommentary,
  commentaryEntries,
  isActive,
  laneFooters,
}: RaceModeArenaProps) {
  // Compute once; track + every lane row render against the same numbers.
  const ranked = useMemo(() => rankAgents(agents, lanes), [agents, lanes]);
  const targetSteps = useMemo(
    () => computeTargetSteps(agents, lanes),
    [agents, lanes],
  );

  return (
    <div className="race-mode-root">
      <RaceTrack
        ranked={ranked}
        lanes={lanes}
        targetSteps={targetSteps}
        winnerAgentId={winnerAgentId}
      />

      <div
        className={`rm-grid${showCommentary ? " rm-grid--with-booth" : ""}`}
      >
        <div className="rm-lanes">
          {ranked.map(({ agent, position }) => (
            <RaceLane
              key={agent.id}
              agent={agent}
              lane={lanes[agent.id] ?? EMPTY_LANE}
              position={position}
              isWinner={deriveLeader(agent, position, winnerAgentId)}
              targetSteps={targetSteps}
              workspaceId={workspaceId}
              runId={runId}
              footer={laneFooters?.[agent.id] ?? null}
            />
          ))}
        </div>
        {showCommentary && (
          <RaceCommentary
            entries={commentaryEntries}
            isActive={isActive}
          />
        )}
      </div>
    </div>
  );
}
