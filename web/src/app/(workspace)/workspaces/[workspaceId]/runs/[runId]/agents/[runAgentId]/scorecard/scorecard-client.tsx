"use client";

import { useState, useEffect, useCallback } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type {
  Run,
  RunAgent,
  ScorecardResponse,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Loader2,
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  Target,
  Shield,
  Zap,
  DollarSign,
  BarChart3,
} from "lucide-react";
import { scorePercent, scoreColor, barWidth, barColor } from "@/lib/scores";

// --- Constants ---

const POLL_MS = 5000;

const DIMENSIONS = [
  { key: "correctness", label: "Correctness", icon: Target },
  { key: "reliability", label: "Reliability", icon: Shield },
  { key: "latency", label: "Latency", icon: Zap },
  { key: "cost", label: "Cost", icon: DollarSign },
] as const;

// --- Radar chart (pure SVG) ---

interface RadarChartProps {
  scores: { label: string; value: number | undefined }[];
}

function RadarChart({ scores }: RadarChartProps) {
  if (scores.length === 0) return null;

  const cx = 100;
  const cy = 100;
  const r = 70;
  const levels = 5;
  const n = scores.length;

  function polarToXY(angle: number, radius: number) {
    // Start from top (-90deg)
    const a = (angle - 90) * (Math.PI / 180);
    return { x: cx + radius * Math.cos(a), y: cy + radius * Math.sin(a) };
  }

  const angleStep = 360 / n;

  // Grid rings
  const rings = Array.from({ length: levels }, (_, i) => {
    const ringR = (r / levels) * (i + 1);
    const points = scores
      .map((_, j) => {
        const p = polarToXY(j * angleStep, ringR);
        return `${p.x},${p.y}`;
      })
      .join(" ");
    return (
      <polygon
        key={i}
        points={points}
        fill="none"
        stroke="currentColor"
        className="text-border"
        strokeWidth={0.5}
      />
    );
  });

  // Axis lines
  const axes = scores.map((_, i) => {
    const p = polarToXY(i * angleStep, r);
    return (
      <line
        key={i}
        x1={cx}
        y1={cy}
        x2={p.x}
        y2={p.y}
        stroke="currentColor"
        className="text-border"
        strokeWidth={0.5}
      />
    );
  });

  // Data polygon
  const dataPoints = scores.map((s, i) => {
    const val = s.value ?? 0;
    const p = polarToXY(i * angleStep, r * val);
    return `${p.x},${p.y}`;
  });

  // Labels
  const labels = scores.map((s, i) => {
    const p = polarToXY(i * angleStep, r + 18);
    return (
      <text
        key={i}
        x={p.x}
        y={p.y}
        textAnchor="middle"
        dominantBaseline="middle"
        className="fill-muted-foreground text-[9px]"
      >
        {s.label}
      </text>
    );
  });

  return (
    <svg viewBox="0 0 200 200" className="w-full max-w-[240px] mx-auto">
      {rings}
      {axes}
      <polygon
        points={dataPoints.join(" ")}
        className="fill-primary/20 stroke-primary"
        strokeWidth={1.5}
      />
      {/* Data dots */}
      {scores.map((s, i) => {
        const val = s.value ?? 0;
        const p = polarToXY(i * angleStep, r * val);
        return (
          <circle
            key={i}
            cx={p.x}
            cy={p.y}
            r={3}
            className="fill-primary"
          />
        );
      })}
      {labels}
    </svg>
  );
}

// --- Component ---

interface ScorecardClientProps {
  initialScorecard: ScorecardResponse;
  run: Run;
  agent: RunAgent;
}

export function ScorecardClient({
  initialScorecard,
  run,
  agent,
}: ScorecardClientProps) {
  const { getAccessToken } = useAccessToken();
  const [scorecard, setScorecard] =
    useState<ScorecardResponse>(initialScorecard);
  const [jsonExpanded, setJsonExpanded] = useState(false);

  const isPending = scorecard.state === "pending";
  const isErrored = scorecard.state === "errored";
  const isReady = scorecard.state === "ready";

  const fetchScorecard = useCallback(async () => {
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await api.get<ScorecardResponse>(
        `/v1/scorecards/${agent.id}`,
        { allowedStatuses: [202, 409] },
      );
      setScorecard(res);
    } catch {
      // Silently fail on poll
    }
  }, [getAccessToken, agent.id]);

  useEffect(() => {
    if (!isPending) return;
    const interval = setInterval(fetchScorecard, POLL_MS);
    return () => clearInterval(interval);
  }, [isPending, fetchScorecard]);

  const overallScore = scorecard.overall_score;

  const dimensionScores = [
    { key: "correctness", score: scorecard.correctness_score },
    { key: "reliability", score: scorecard.reliability_score },
    { key: "latency", score: scorecard.latency_score },
    { key: "cost", score: scorecard.cost_score },
  ];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-3 mb-1">
            <h1 className="text-lg font-semibold tracking-tight">
              {agent.label}
            </h1>
            <Badge variant="outline">{run.name}</Badge>
          </div>
          {isReady && overallScore != null && (
            <div className="flex items-baseline gap-2">
              <span className={`text-3xl font-bold ${scoreColor(overallScore)}`}>
                {scorePercent(overallScore)}
              </span>
              <span className="text-sm text-muted-foreground">
                overall score
              </span>
            </div>
          )}
        </div>
        <Badge
          variant={
            isReady ? "default" : isErrored ? "destructive" : "secondary"
          }
        >
          {isPending && (
            <Loader2
              data-icon="inline-start"
              className="size-3 animate-spin"
            />
          )}
          {scorecard.state}
        </Badge>
      </div>

      {/* Pending state */}
      {isPending && (
        <div className="rounded-lg border border-border p-8 text-center text-sm text-muted-foreground">
          <Loader2 className="size-6 animate-spin mx-auto mb-3" />
          <p>Evaluation in progress...</p>
          <p className="text-xs mt-1">
            This page will update automatically when scoring completes.
          </p>
        </div>
      )}

      {/* Errored state */}
      {isErrored && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-8 text-center text-sm text-destructive">
          <AlertTriangle className="size-6 mx-auto mb-3" />
          <p className="font-medium">Scorecard unavailable</p>
          <p className="text-xs mt-1 text-destructive/70">
            {scorecard.message || "An error occurred during evaluation."}
          </p>
        </div>
      )}

      {/* Ready state — score breakdown */}
      {isReady && (
        <>
          <div className="grid gap-6 md:grid-cols-2">
            {/* Radar chart */}
            <div className="rounded-lg border border-border p-4">
              <h2 className="text-sm font-semibold mb-4 flex items-center gap-2">
                <BarChart3 className="size-4" />
                Score Dimensions
              </h2>
              <RadarChart
                scores={DIMENSIONS.map((d) => ({
                  label: d.label,
                  value:
                    dimensionScores.find((s) => s.key === d.key)?.score ??
                    undefined,
                }))}
              />
            </div>

            {/* Bar breakdown */}
            <div className="rounded-lg border border-border p-4">
              <h2 className="text-sm font-semibold mb-4">Score Breakdown</h2>
              <div className="space-y-4">
                {DIMENSIONS.map((dim) => {
                  const score = dimensionScores.find(
                    (s) => s.key === dim.key,
                  )?.score;
                  const Icon = dim.icon;
                  const detail = scorecard.scorecard?.dimensions?.[dim.key];

                  return (
                    <div key={dim.key}>
                      <div className="flex items-center justify-between mb-1.5">
                        <div className="flex items-center gap-2 text-sm">
                          <Icon className="size-3.5 text-muted-foreground" />
                          <span>{dim.label}</span>
                          {detail?.state === "error" && (
                            <Badge variant="destructive" className="text-[10px] h-4">
                              error
                            </Badge>
                          )}
                          {detail?.state === "unavailable" && (
                            <Badge variant="secondary" className="text-[10px] h-4">
                              n/a
                            </Badge>
                          )}
                        </div>
                        <span
                          className={`text-sm font-medium ${scoreColor(score)}`}
                        >
                          {scorePercent(score)}
                        </span>
                      </div>
                      <div className="h-2 rounded-full bg-muted overflow-hidden">
                        <div
                          className={`h-full rounded-full transition-all ${barColor(score)}`}
                          style={{ width: barWidth(score) }}
                        />
                      </div>
                      {detail?.reason && (
                        <p className="text-xs text-muted-foreground mt-1">
                          {detail.reason}
                        </p>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          </div>

          {/* Evaluation summaries */}
          {scorecard.scorecard && (
            <div className="grid gap-3 grid-cols-2 md:grid-cols-4">
              {scorecard.scorecard.validator_summary && (
                <>
                  <div className="rounded-lg border border-border p-3 text-center">
                    <div className="text-xs text-muted-foreground mb-1">
                      Validators
                    </div>
                    <div className="text-lg font-semibold">
                      {scorecard.scorecard.validator_summary.total ?? 0}
                    </div>
                  </div>
                  <div className="rounded-lg border border-border p-3 text-center">
                    <div className="text-xs text-muted-foreground mb-1">
                      Passed
                    </div>
                    <div className="text-lg font-semibold text-emerald-400">
                      {scorecard.scorecard.validator_summary.pass ?? 0}
                    </div>
                  </div>
                </>
              )}
              {scorecard.scorecard.metric_summary && (
                <>
                  <div className="rounded-lg border border-border p-3 text-center">
                    <div className="text-xs text-muted-foreground mb-1">
                      Metrics
                    </div>
                    <div className="text-lg font-semibold">
                      {scorecard.scorecard.metric_summary.total ?? 0}
                    </div>
                  </div>
                  <div className="rounded-lg border border-border p-3 text-center">
                    <div className="text-xs text-muted-foreground mb-1">
                      Available
                    </div>
                    <div className="text-lg font-semibold text-emerald-400">
                      {scorecard.scorecard.metric_summary.available ?? 0}
                    </div>
                  </div>
                </>
              )}
            </div>
          )}

          {/* Warnings */}
          {scorecard.scorecard?.warnings &&
            scorecard.scorecard.warnings.length > 0 && (
              <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-4">
                <h3 className="text-sm font-medium text-amber-400 flex items-center gap-2 mb-2">
                  <AlertTriangle className="size-4" />
                  Warnings
                </h3>
                <ul className="space-y-1">
                  {scorecard.scorecard.warnings.map((w, i) => (
                    <li
                      key={i}
                      className="text-xs text-amber-400/80"
                    >
                      {w}
                    </li>
                  ))}
                </ul>
              </div>
            )}

          {/* Raw JSON viewer */}
          <div className="rounded-lg border border-border">
            <Button
              variant="ghost"
              size="sm"
              className="w-full justify-start px-4 py-3 h-auto"
              onClick={() => setJsonExpanded(!jsonExpanded)}
            >
              {jsonExpanded ? (
                <ChevronDown className="size-4 mr-2" />
              ) : (
                <ChevronRight className="size-4 mr-2" />
              )}
              <span className="text-sm">Raw Scorecard JSON</span>
            </Button>
            {jsonExpanded && (
              <div className="px-4 pb-4">
                <pre className="text-xs bg-muted/50 rounded-md p-3 overflow-x-auto max-h-96 overflow-y-auto font-[family-name:var(--font-mono)]">
                  {JSON.stringify(scorecard.scorecard, null, 2)}
                </pre>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
