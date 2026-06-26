import {
  Check,
  Container,
  FilePlus2,
  Flag,
  Loader2,
  PackageCheck,
  Scale,
  ShieldCheck,
  Sparkles,
  SquareTerminal,
  TriangleAlert,
  Wrench,
} from "lucide-react";
import type { ComponentType } from "react";

import type { TryoutTimelineEvent } from "@/lib/api/types";
import { cn } from "@/lib/utils";

const MICRO = "font-mono text-2xs uppercase tracking-[0.22em]";
const MONO = "font-mono text-2xs tracking-tight";

/*
 * The agent-at-work timeline.
 *
 * The backend already ships rich, typed events (tool_name, exit_code,
 * duration_ms, path, provider_model_id, verdict, score) in `event.payload`.
 * This module folds the raw started/completed/failed pairs into settled steps
 * and renders them as a single connected spine — so a tryout reads like
 * "here is what the agent actually did", not a raw event log.
 */

type NodeStatus = "running" | "done" | "failed";

type StepKind =
  | "setup"
  | "plan"
  | "tool"
  | "command"
  | "files"
  | "validation"
  | "scoring"
  | "finished"
  | "activity";

export type TimelineNode = {
  id: string;
  kind: StepKind;
  title: string;
  /** Mono technical detail rendered beside the title (tool name, judge model). */
  detail?: string;
  status: NodeStatus;
  at: number;
  durationMs?: number;
  files?: string[];
  exitCode?: number;
  verdict?: string;
  score?: number;
  /** Optional per-node icon override (e.g. the output-finalized moment). */
  icon?: ComponentType<{ className?: string }>;
};

const ICONS: Record<StepKind, ComponentType<{ className?: string }>> = {
  setup: Container,
  plan: Sparkles,
  tool: Wrench,
  command: SquareTerminal,
  files: FilePlus2,
  validation: ShieldCheck,
  scoring: Scale,
  finished: Flag,
  activity: Sparkles,
};

function str(payload: Record<string, unknown> | undefined, key: string): string | undefined {
  const value = payload?.[key];
  return typeof value === "string" && value.trim() !== "" ? value : undefined;
}

function num(payload: Record<string, unknown> | undefined, key: string): number | undefined {
  const value = payload?.[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function pathOf(payload: Record<string, unknown> | undefined): string | undefined {
  return str(payload, "path") ?? str(payload, "file_path") ?? str(payload, "relative_path");
}

function lifecycleOf(summary: string): "start" | "end" | "fail" {
  const s = summary.toLowerCase();
  if (s.includes("fail")) return "fail";
  if (/(started|planning|is grading|grading against)/.test(s)) return "start";
  return "end";
}

function prettyVerdict(verdict: string): string {
  return verdict.replace(/_/g, " ");
}

/** Humanize a snake/camel tool name into a readable label, keeping it honest. */
function prettyTool(name: string): string {
  return name.replace(/[._-]+/g, " ").replace(/([a-z])([A-Z])/g, "$1 $2").trim();
}

export function formatDuration(ms: number | undefined): string | null {
  if (ms == null || !Number.isFinite(ms) || ms < 0) return null;
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const seconds = ms / 1000;
  if (seconds < 60) return `${seconds.toFixed(seconds < 10 ? 1 : 0)}s`;
  const mins = Math.floor(seconds / 60);
  const rem = Math.round(seconds % 60);
  return `${mins}m ${rem}s`;
}

/**
 * Fold the raw event stream into settled timeline nodes. Started/completed
 * pairs collapse into one node; consecutive file writes roll up into a single
 * "created files" group; everything keeps its real technical substance.
 */
export function foldTimeline(events: TryoutTimelineEvent[]): TimelineNode[] {
  const nodes: TimelineNode[] = [];
  const openByKind = new Map<StepKind, number>();

  const open = (kind: StepKind, node: Omit<TimelineNode, "status">) => {
    nodes.push({ ...node, status: "running" });
    openByKind.set(kind, nodes.length - 1);
  };

  const close = (
    kind: StepKind,
    status: NodeStatus,
    at: number,
    patch: Partial<TimelineNode>,
    fallbackTitle: string,
  ) => {
    const index = openByKind.get(kind);
    if (index != null) {
      const node = nodes[index];
      node.status = status;
      node.durationMs = patch.durationMs ?? (at - node.at >= 0 ? at - node.at : undefined);
      if (patch.title) node.title = patch.title;
      if (patch.detail) node.detail = patch.detail;
      if (patch.exitCode != null) node.exitCode = patch.exitCode;
      if (patch.verdict) node.verdict = patch.verdict;
      if (patch.score != null) node.score = patch.score;
      openByKind.delete(kind);
    } else {
      nodes.push({
        id: patch.id ?? `n${nodes.length}`,
        kind,
        title: patch.title ?? fallbackTitle,
        detail: patch.detail,
        status,
        at,
        durationMs: patch.durationMs,
        exitCode: patch.exitCode,
        verdict: patch.verdict,
        score: patch.score,
      });
    }
  };

  for (const event of events) {
    const at = new Date(event.occurred_at).getTime();
    const id = `e${event.cursor}`;
    const payload = event.payload;
    const life = lifecycleOf(event.summary);

    switch (event.type) {
      case "started":
        nodes.push({ id, kind: "setup", title: "Spun up a fresh sandbox", status: "done", at });
        break;

      case "planning": {
        const model = str(payload, "provider_model_id");
        if (life === "start") {
          open("plan", { id, kind: "plan", title: "Reasoning about the task", detail: model, at });
        } else {
          close("plan", "done", at, { detail: model, durationMs: num(payload, "duration_ms") }, "Reasoned about the task");
        }
        break;
      }

      case "tool_call": {
        const tool = str(payload, "tool_name");
        if (life === "start") {
          open("tool", { id, kind: "tool", title: tool ? prettyTool(tool) : "Used a tool", detail: tool, at });
        } else {
          close(
            "tool",
            life === "fail" ? "failed" : "done",
            at,
            { detail: tool, durationMs: num(payload, "duration_ms"), title: tool ? prettyTool(tool) : undefined },
            tool ? prettyTool(tool) : "Used a tool",
          );
        }
        break;
      }

      case "sandbox_command": {
        if (life === "start") {
          open("command", { id, kind: "command", title: "Ran a shell command", at });
        } else {
          close(
            "command",
            life === "fail" ? "failed" : "done",
            at,
            { exitCode: num(payload, "exit_code"), durationMs: num(payload, "duration_ms") },
            "Ran a shell command",
          );
        }
        break;
      }

      case "file_written":
      case "file_activity": {
        const file = pathOf(payload);
        const last = nodes[nodes.length - 1];
        if (last && last.kind === "files" && last.status === "done") {
          if (file && !last.files?.includes(file)) last.files = [...(last.files ?? []), file];
          last.at = at;
        } else {
          nodes.push({
            id,
            kind: "files",
            title: "Wrote files",
            status: "done",
            at,
            files: file ? [file] : [],
          });
        }
        break;
      }

      case "validation":
        nodes.push({ id, kind: "validation", title: "Checked the output against rules", status: "done", at });
        break;

      case "scoring": {
        const model = str(payload, "provider_model_id");
        const verdict = str(payload, "verdict");
        const score = num(payload, "score");
        if (life === "start") {
          open("scoring", { id, kind: "scoring", title: "Judge is grading against your bar", detail: model, at });
        } else {
          close(
            "scoring",
            "done",
            at,
            {
              title: verdict ? "Judge reached a verdict" : "Scoring complete",
              detail: model,
              verdict,
              score,
              durationMs: num(payload, "duration_ms"),
            },
            "Scoring complete",
          );
        }
        break;
      }

      case "finished":
        nodes.push({
          id,
          kind: "finished",
          title: event.summary.toLowerCase().includes("fail") ? "Run ended early" : "Run complete",
          status: event.summary.toLowerCase().includes("fail") ? "failed" : "done",
          at,
        });
        break;

      default: {
        const isFinal = /final/i.test(event.summary);
        const title = isFinal ? "Finalized the outputs" : event.summary || "Working";
        nodes.push({ id, kind: "activity", title, status: "done", at, icon: isFinal ? PackageCheck : undefined });
      }
    }
  }

  return nodes;
}

function statusTone(status: NodeStatus): string {
  if (status === "failed") return "border-[#d97757]/40 text-[#e0a085]";
  if (status === "running") return "border-white/35 text-white";
  return "border-white/12 text-white/55";
}

function NodeIcon({ node }: { node: TimelineNode }) {
  const Icon = node.icon ?? ICONS[node.kind];
  return (
    <span
      className={cn(
        "relative z-10 flex size-7 shrink-0 items-center justify-center rounded-full border bg-[#131312]",
        statusTone(node.status),
      )}
    >
      {node.status === "running" ? (
        <>
          <span className="absolute inline-flex size-7 animate-ping rounded-full bg-white/10 motion-reduce:hidden" />
          <Loader2 className="size-3.5 animate-spin motion-reduce:animate-none" />
        </>
      ) : node.kind === "finished" && node.status === "done" ? (
        <Check className="size-3.5" />
      ) : node.status === "failed" ? (
        <TriangleAlert className="size-3.5" />
      ) : (
        <Icon className="size-3.5" />
      )}
    </span>
  );
}

function fileName(path: string): string {
  const parts = path.split("/");
  return parts[parts.length - 1] || path;
}

function TimelineNodeRow({ node, isLast }: { node: TimelineNode; isLast: boolean }) {
  const duration = formatDuration(node.durationMs);
  const grade = node.score != null ? (1 + node.score * 4).toFixed(1) : null;
  const singleFile = node.kind === "files" && node.files?.length === 1 ? node.files[0] : null;
  const title =
    node.kind === "files" && node.files && node.files.length > 0
      ? node.files.length === 1
        ? `Wrote ${fileName(node.files[0])}`
        : `Wrote ${node.files.length} files`
      : node.title;

  return (
    <li className="relative flex gap-3 pb-5 last:pb-0">
      {!isLast ? (
        <span aria-hidden className="absolute left-[13.5px] top-7 bottom-0 w-px bg-white/12" />
      ) : null}
      <NodeIcon node={node} />
      <div className="min-w-0 flex-1 pt-0.5">
        <div className="flex items-baseline justify-between gap-3">
          <p
            className={cn(
              "text-sm leading-6",
              node.status === "running" ? "text-white/90" : node.status === "failed" ? "text-[#e0a085]" : "text-white/75",
            )}
          >
            {title}
            {node.detail && node.detail !== title ? (
              <span className={cn(MONO, "ml-2 align-middle text-white/40")}>{node.detail}</span>
            ) : null}
          </p>
          {duration ? (
            <span className={cn(MONO, "shrink-0 tabular-nums text-white/30")}>{duration}</span>
          ) : null}
        </div>

        {node.exitCode != null ? (
          <span
            className={cn(
              MONO,
              "mt-1.5 inline-flex items-center gap-1 rounded-sm border px-1.5 py-0.5",
              node.exitCode === 0
                ? "border-white/12 text-white/45"
                : "border-[#d97757]/40 text-[#e0a085]",
            )}
          >
            exit {node.exitCode}
          </span>
        ) : null}

        {node.verdict ? (
          <div className="mt-2 flex items-center gap-2">
            <span
              className={cn(
                MICRO,
                "rounded-sm border px-2 py-1",
                node.verdict === "approved"
                  ? "border-white/30 text-white/85"
                  : node.verdict === "rejected"
                    ? "border-[#d97757]/40 text-[#e0a085]"
                    : "border-white/15 text-white/55",
              )}
            >
              {prettyVerdict(node.verdict)}
            </span>
            {grade ? <span className="text-xs text-white/40">{grade} / 5</span> : null}
          </div>
        ) : null}

        {singleFile && singleFile.includes("/") ? (
          <p className={cn(MONO, "mt-1 truncate text-white/30")} title={singleFile}>
            {singleFile}
          </p>
        ) : null}

        {node.files && node.files.length > 1 ? (
          <ul className="mt-2 space-y-1">
            {node.files.map((file) => (
              <li key={file} className="flex items-baseline gap-2 text-xs text-white/45">
                <span className="size-1 shrink-0 translate-y-1 rounded-full bg-white/25" aria-hidden />
                <span className={cn(MONO, "shrink-0 text-white/55")} title={file}>
                  {fileName(file)}
                </span>
                {file.includes("/") ? (
                  <span className="truncate text-2xs text-white/25">{file}</span>
                ) : null}
              </li>
            ))}
          </ul>
        ) : null}
      </div>
    </li>
  );
}

export function RunTimeline({
  events,
  active,
  thinkingLabel,
}: {
  events: TryoutTimelineEvent[];
  active?: boolean;
  thinkingLabel?: string | null;
}) {
  const nodes = foldTimeline(events);

  if (active && !nodes.some((node) => node.status === "running")) {
    nodes.push({
      id: "live",
      kind: "activity",
      title: thinkingLabel || "Working",
      status: "running",
      at: Number.MAX_SAFE_INTEGER,
    });
  }

  if (nodes.length === 0) {
    return (
      <div className="flex items-center gap-2 py-1 text-sm text-white/45">
        <Loader2 className="size-3.5 animate-spin motion-reduce:animate-none" />
        {thinkingLabel || "Waiting for the agent…"}
      </div>
    );
  }

  return (
    <ol className="relative">
      {nodes.map((node, index) => (
        <TimelineNodeRow key={node.id} node={node} isLast={index === nodes.length - 1} />
      ))}
    </ol>
  );
}
