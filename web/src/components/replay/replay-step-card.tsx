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
}

export function ReplayStepCard({ step, index }: ReplayStepCardProps) {
  const [expanded, setExpanded] = useState(false);
  const Icon = stepIcon[step.type] ?? Activity;
  const duration = formatDuration(step.occurred_at, step.completed_at);

  const hasDetail =
    step.provider_key ||
    step.provider_model_id ||
    step.tool_name ||
    step.sandbox_action ||
    step.metric_key ||
    step.final_output ||
    step.error_message ||
    step.event_types.length > 0;

  return (
    <div className="group">
      <button
        onClick={() => hasDetail && setExpanded(!expanded)}
        className={cn(
          "flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition-colors",
          hasDetail && "hover:bg-muted/50 cursor-pointer",
          !hasDetail && "cursor-default",
        )}
      >
        {/* Step number */}
        <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-medium text-muted-foreground">
          {index + 1}
        </span>

        {/* Icon */}
        <Icon className="size-4 shrink-0 text-muted-foreground" />

        {/* Headline */}
        <span className="flex-1 truncate font-medium">{step.headline}</span>

        {/* Duration */}
        {duration && (
          <span className="shrink-0 text-xs text-muted-foreground">
            {duration}
          </span>
        )}

        {/* Status */}
        <Badge variant={statusVariant[step.status] ?? "outline"} className="shrink-0">
          {step.status}
        </Badge>

        {/* Timestamp */}
        <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
          {formatTime(step.occurred_at)}
        </span>

        {/* Expand chevron */}
        {hasDetail && (
          expanded ? (
            <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
          ) : (
            <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
          )
        )}
      </button>

      {/* Expanded detail */}
      {expanded && (
        <div className="ml-12 mr-3 mb-2 space-y-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
          {/* Metadata row */}
          <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
            {step.provider_key && (
              <span>
                Provider: <span className="text-foreground">{step.provider_key}</span>
              </span>
            )}
            {step.provider_model_id && (
              <span>
                Model: <span className="text-foreground">{step.provider_model_id}</span>
              </span>
            )}
            {step.tool_name && (
              <span>
                Tool: <span className="text-foreground">{step.tool_name}</span>
              </span>
            )}
            {step.sandbox_action && (
              <span>
                Action: <span className="text-foreground">{step.sandbox_action}</span>
              </span>
            )}
            {step.metric_key && (
              <span>
                Metric: <span className="text-foreground">{step.metric_key}</span>
              </span>
            )}
            <span>
              Source: <span className="text-foreground">{step.source}</span>
            </span>
            <span>
              Events: <span className="text-foreground">{step.event_count}</span>
            </span>
          </div>

          {/* Event types */}
          {step.event_types.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {step.event_types.map((et, i) => (
                <span
                  key={i}
                  className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono text-muted-foreground"
                >
                  {et}
                </span>
              ))}
            </div>
          )}

          {/* Error message */}
          {step.error_message && (
            <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive">
              {step.error_message}
            </div>
          )}

          {/* Final output */}
          {step.final_output && (
            <pre className="max-h-60 overflow-auto rounded-md bg-background border border-border p-3 text-xs font-[family-name:var(--font-mono)] whitespace-pre-wrap break-words">
              {step.final_output}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}
