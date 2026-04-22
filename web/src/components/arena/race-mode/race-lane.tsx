"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Play, CheckCircle2, Upload } from "lucide-react";

import type { RunAgent, RunAgentStatus } from "@/lib/api/types";
import type { ArenaLaneState } from "@/hooks/use-agent-arena";

const ACTIVE_STATUSES: RunAgentStatus[] = [
  "queued",
  "ready",
  "executing",
  "evaluating",
];

interface RaceLaneProps {
  agent: RunAgent;
  lane: ArenaLaneState;
  position: number;
  isWinner: boolean;
  /** Shared across track + lane — parent computes once. */
  targetSteps: number;
  workspaceId: string;
  runId: string;
  /** Terminal-only content (e.g. scorecard summary). */
  footer?: React.ReactNode;
}

export function RaceLane({
  agent,
  lane,
  position,
  isWinner,
  targetSteps,
  workspaceId,
  runId,
  footer,
}: RaceLaneProps) {
  const isActive = ACTIVE_STATUSES.includes(agent.status);
  const isFailed = agent.status === "failed";

  // Keep the elapsed clock ticking between SSE events. One timer per live
  // lane, cleared the moment it goes terminal.
  useElapsedTick(isActive && !agent.finished_at);

  const laneClass = [
    "rm-lane",
    isActive && "rm-lane--active",
    isWinner && "rm-lane--winner",
    isFailed && "rm-lane--failed",
  ]
    .filter(Boolean)
    .join(" ");

  const stepCount = Math.max(lane.stepIndex, 0);
  const stepProgress = Math.min(stepCount, targetSteps);

  return (
    <article className={laneClass}>
      <div className="rm-lane__rank">
        <span className="rm-lane__rank-pos">{position}</span>
        <span className="rm-lane__delta">
          #{agent.lane_index} · {agent.status}
        </span>
      </div>

      <div className="rm-lane__content">
        <header className="rm-lane__head">
          <div>
            <h3 className="rm-lane__name">
              {agent.label}
              {isWinner && (
                <span className="rm-lane__leader-tag">Leader</span>
              )}
            </h3>
            <div className="rm-lane__meta">
              {agent.started_at && (
                <span>{formatElapsed(agent.started_at, agent.finished_at)}</span>
              )}
              {lane.stepIndex > 0 && (
                <>
                  <span className="rm-dot" />
                  <span>
                    step {lane.stepIndex} / {targetSteps}
                  </span>
                </>
              )}
              {lane.modelCalls > 0 && (
                <>
                  <span className="rm-dot" />
                  <span>{lane.modelCalls} model calls</span>
                </>
              )}
            </div>
          </div>
          <div className="rm-lane__status">
            <span className="rm-lane__status-pulse" />
            {agent.status}
          </div>
        </header>

        {/* Step progress — targetSteps segments, agreeing with the track */}
        <div className="rm-lane__steps" aria-label="Step progress">
          {Array.from({ length: targetSteps }).map((_, i) => {
            let cls = "rm-step";
            if (i < stepProgress - 1) cls += " rm-step--done";
            else if (i === stepProgress - 1 && isActive)
              cls += " rm-step--current";
            else if (i === stepProgress - 1 && !isActive)
              cls += " rm-step--done";
            return <span key={i} className={cls} />;
          })}
        </div>

        <div className="rm-tele">
          <div className="rm-tele__item">
            <label>Elapsed</label>
            <div className="rm-val">
              {agent.started_at
                ? formatElapsed(agent.started_at, agent.finished_at)
                : "—"}
            </div>
          </div>
          <div className="rm-tele__item">
            <label>Model calls</label>
            <div className="rm-val">{lane.modelCalls}</div>
          </div>
          <div className="rm-tele__item">
            <label>Tool calls</label>
            <div className="rm-val">{lane.toolCalls}</div>
          </div>
          <div className="rm-tele__item">
            <label>Tokens</label>
            <div className="rm-val">{compactNumber(lane.totalTokens)}</div>
          </div>
          <div className="rm-tele__item">
            <label>Score</label>
            <div
              className={
                lane.lastMetric?.score != null
                  ? "rm-val rm-val--go"
                  : "rm-val"
              }
            >
              {lane.lastMetric?.score != null
                ? lane.lastMetric.score.toFixed(2)
                : "—"}
            </div>
          </div>
        </div>

        {isActive && (
          <div className="rm-now">
            <span className="rm-now__tag">
              <span className="rm-wave" aria-hidden="true">
                <span />
                <span />
                <span />
                <span />
                <span />
              </span>
              Now
            </span>
            <div className="rm-now__text">
              {lane.nowDoing ? (
                <>
                  <span>{lane.nowDoing.label}</span>
                  {lane.nowDoing.detail && (
                    <span className="rm-now__obj" title={lane.nowDoing.detail}>
                      {lane.nowDoing.detail}
                    </span>
                  )}
                </>
              ) : (
                <span>Standing by…</span>
              )}
            </div>
          </div>
        )}

        {isActive && lane.streamingOutput && (
          <div className="rm-stream">{lane.streamingOutput}</div>
        )}

        {isFailed && agent.failure_reason && (
          <div className="rm-fail">
            <div className="rm-fail__ic" aria-hidden="true">
              !
            </div>
            <div className="rm-fail__body">
              <strong>Failed</strong>
              <p>{agent.failure_reason}</p>
            </div>
          </div>
        )}

        {!isActive && footer}

        <footer className="rm-lane__foot">
          <Link
            href={`/workspaces/${workspaceId}/runs/${runId}/agents/${agent.id}/replay`}
          >
            <Play className="size-3" />
            Replay
          </Link>
          <Link
            href={`/workspaces/${workspaceId}/runs/${runId}/agents/${agent.id}/scorecard`}
          >
            <CheckCircle2 className="size-3" />
            Scorecard
          </Link>
          <Link
            href={`/workspaces/${workspaceId}/runs/${runId}?upload=${agent.id}`}
          >
            <Upload className="size-3" />
            Upload
          </Link>
        </footer>
      </div>
    </article>
  );
}

/** Forces a re-render every second while active so elapsed never freezes. */
function useElapsedTick(enabled: boolean): void {
  const [, setTick] = useState(0);
  useEffect(() => {
    if (!enabled) return;
    const id = window.setInterval(() => setTick((n) => n + 1), 1000);
    return () => window.clearInterval(id);
  }, [enabled]);
}

function formatElapsed(start?: string, end?: string): string {
  if (!start) return "—";
  const startMs = new Date(start).getTime();
  const endMs = end ? new Date(end).getTime() : Date.now();
  const ms = Math.max(0, endMs - startMs);
  if (ms < 1000) return "<1s";
  const secs = Math.floor(ms / 1000);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  return `${mins}m ${secs % 60}s`;
}

function compactNumber(n: number): string {
  if (n < 1000) return n.toString();
  if (n < 1_000_000)
    return (n / 1000).toFixed(1).replace(/\.0$/, "") + "k";
  return (n / 1_000_000).toFixed(1).replace(/\.0$/, "") + "M";
}
