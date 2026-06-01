"use client";

import { useState } from "react";
import type { ReplayStep, ReplayStepType } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  BrainCircuit,
  Wrench,
  Terminal,
  FileCode,
  MessageSquare,
  BarChart3,
  Play,
  Activity,
  ChevronRight,
  ChevronDown,
} from "lucide-react";
import { DownloadArtifactButton } from "@/components/artifacts/download-artifact-button";
import { AgentOutputRenderer } from "./agent-output-renderer";

const stepIcon: Record<ReplayStepType, React.ComponentType<{ className?: string }>> = {
  model_call: BrainCircuit,
  tool_call: Wrench,
  sandbox_command: Terminal,
  sandbox_file: FileCode,
  output: MessageSquare,
  scoring: BarChart3,
  scoring_metric: BarChart3,
  run: Play,
  agent_step: Play,
  event: Activity,
};

const statusVariant: Record<string, "default" | "secondary" | "outline" | "destructive"> = {
  completed: "default",
  running: "secondary",
  failed: "destructive",
};

function formatTime(iso: string): string {
  const d = new Date(iso);
  const h = d.getUTCHours().toString().padStart(2, "0");
  const m = d.getUTCMinutes().toString().padStart(2, "0");
  const s = d.getUTCSeconds().toString().padStart(2, "0");
  return `${h}:${m}:${s}`;
}

function formatDuration(start: string, end?: string): string | null {
  if (!end) return null;
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (ms < 1000) return "<1s";
  const secs = Math.floor(ms / 1000);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  return `${mins}m ${secs % 60}s`;
}

interface ReplayStepCardProps {
  step: ReplayStep;
  index: number;
  highlighted?: boolean;
}

export function ReplayStepCard({
  step,
  index,
  highlighted = false,
}: ReplayStepCardProps) {
  const [expanded, setExpanded] = useState(highlighted);
  const Icon = stepIcon[step.type] ?? Activity;
  const duration = formatDuration(step.occurred_at, step.completed_at);

  const hasArtifacts = step.artifact_ids && step.artifact_ids.length > 0;
  const hasDetail =
    step.provider_key ||
    step.provider_model_id ||
    step.tool_name ||
    step.sandbox_action ||
    step.metric_key ||
    step.final_output ||
    step.model_output ||
    step.tool_result ||
    step.error_message ||
    step.event_types.length > 0 ||
    hasArtifacts;

  return (
    <div
      className={cn(
        "group transition-colors border-b border-white/[0.06] last:border-0",
        highlighted && "bg-amber-400/5 ring-1 ring-amber-400/30",
      )}
    >
      <button
        onClick={() => hasDetail && setExpanded(!expanded)}
        className={cn(
          "flex w-full items-center gap-4 px-4 py-3 text-left text-sm transition-colors",
          hasDetail && "hover:bg-white/[0.02] cursor-pointer",
          !hasDetail && "cursor-default",
        )}
      >
        {/* Step number */}
        <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-white/5 text-[10px] font-medium text-white/50 font-[family-name:var(--font-mono)]">
          {index + 1}
        </span>

        {/* Icon */}
        <div className="flex size-8 shrink-0 items-center justify-center rounded-md border border-white/10 bg-white/5">
          <Icon className="size-4 text-white/60" />
        </div>

        {/* Headline */}
        <span className="flex-1 truncate font-medium text-white/90">{step.headline}</span>

        {/* Duration */}
        {duration && (
          <span className="shrink-0 text-[11px] uppercase tracking-[0.14em] text-white/40">
            {duration}
          </span>
        )}

        {/* Status */}
        <Badge variant={statusVariant[step.status] ?? "outline"} className="shrink-0 bg-white/5 text-white/70 border-white/10">
          {step.status}
        </Badge>

        {/* Timestamp */}
        <span className="shrink-0 text-[11px] font-[family-name:var(--font-mono)] text-white/30 tabular-nums">
          {formatTime(step.occurred_at)}
        </span>

        {/* Expand chevron */}
        {hasDetail && (
          expanded ? (
            <ChevronDown className="size-4 shrink-0 text-white/40" />
          ) : (
            <ChevronRight className="size-4 shrink-0 text-white/40" />
          )
        )}
      </button>

      {/* Expanded detail */}
      {expanded && (
        <div className="ml-[4.5rem] mr-4 mb-4 space-y-4 rounded-lg border border-white/[0.08] bg-white/[0.02] p-4 text-sm">
          {/* Metadata row */}
          <div className="flex flex-wrap gap-x-5 gap-y-2 text-[11px] uppercase tracking-[0.14em] text-white/40">
            {step.provider_key && (
              <span className="flex items-baseline gap-1.5">
                Provider: <span className="text-white/80 font-[family-name:var(--font-mono)] normal-case tracking-normal">{step.provider_key}</span>
              </span>
            )}
            {step.provider_model_id && (
              <span className="flex items-baseline gap-1.5">
                Model: <span className="text-white/80 font-[family-name:var(--font-mono)] normal-case tracking-normal">{step.provider_model_id}</span>
              </span>
            )}
            {step.tool_name && (
              <span className="flex items-baseline gap-1.5">
                Tool: <span className="text-white/80 font-[family-name:var(--font-mono)] normal-case tracking-normal">{step.tool_name}</span>
              </span>
            )}
            {step.sandbox_action && (
              <span className="flex items-baseline gap-1.5">
                Action: <span className="text-white/80 font-[family-name:var(--font-mono)] normal-case tracking-normal">{step.sandbox_action}</span>
              </span>
            )}
            {step.metric_key && (
              <span className="flex items-baseline gap-1.5">
                Metric: <span className="text-white/80 font-[family-name:var(--font-mono)] normal-case tracking-normal">{step.metric_key}</span>
              </span>
            )}
            <span className="flex items-baseline gap-1.5">
              Source: <span className="text-white/80 font-[family-name:var(--font-mono)] normal-case tracking-normal">{step.source}</span>
            </span>
            <span className="flex items-baseline gap-1.5">
              Events: <span className="text-white/80 font-[family-name:var(--font-mono)] normal-case tracking-normal">{step.event_count}</span>
            </span>
          </div>

          {/* Event types */}
          {step.event_types.length > 0 && (
            <div className="flex flex-wrap gap-1.5 pt-1">
              {step.event_types.map((et, i) => (
                <span
                  key={i}
                  className="rounded border border-white/10 bg-white/5 px-1.5 py-0.5 text-[10px] font-[family-name:var(--font-mono)] text-white/60"
                >
                  {et}
                </span>
              ))}
            </div>
          )}

          {/* Error message */}
          {step.error_message && (
            <div className="rounded-md bg-red-500/10 border border-red-500/20 px-3 py-2 text-xs text-red-400">
              {step.error_message}
            </div>
          )}

          {/* Model output */}
          {step.model_output && (
            <div className="pt-2">
              <div className="mb-2 flex items-center gap-2">
                <div className="h-px flex-1 bg-white/[0.06]" />
                <span className="text-[10px] uppercase tracking-[0.16em] text-white/40">
                  Model output
                </span>
                <div className="h-px flex-1 bg-white/[0.06]" />
              </div>
              <div className="max-h-[500px] overflow-auto rounded-lg border border-white/[0.06] bg-[#0d0d12] p-4">
                <AgentOutputRenderer text={step.model_output} />
              </div>
            </div>
          )}

          {/* Tool result */}
          {step.tool_result && (
            <div className="pt-2">
              <div className="mb-2 flex items-center gap-2">
                <div className="h-px flex-1 bg-white/[0.06]" />
                <span className="text-[10px] uppercase tracking-[0.16em] text-white/40">
                  Tool result
                </span>
                <div className="h-px flex-1 bg-white/[0.06]" />
              </div>
              <div className="max-h-[500px] overflow-auto rounded-lg border border-white/[0.06] bg-[#0d0d12] p-4">
                <AgentOutputRenderer text={step.tool_result} />
              </div>
            </div>
          )}

          {/* Final output */}
          {step.final_output && (
            <div className="pt-2">
              <div className="max-h-[500px] overflow-auto rounded-lg border border-white/[0.06] bg-[#0d0d12] p-4">
                <AgentOutputRenderer text={step.final_output} />
              </div>
            </div>
          )}

          {/* Artifacts */}
          {hasArtifacts && (
            <div className="flex flex-wrap items-center gap-2 pt-2">
              <span className="text-[10px] uppercase tracking-[0.16em] text-white/40">Artifacts:</span>
              {step.artifact_ids!.map((id) => (
                <DownloadArtifactButton
                  key={id}
                  artifactId={id}
                  label={id.slice(0, 8)}
                />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
