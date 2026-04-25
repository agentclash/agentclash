/**
 * Pure helpers that turn a run event envelope into compact human-readable
 * signals for the arena live view.
 *
 * Two outputs:
 *   - `summarizeEvent` → a one-line entry for the live ticker (icon + headline).
 *   - `deriveNowDoing` → a short "what is this agent doing right now" label,
 *     emitted only for events that represent the start of a long-running
 *     action (e.g. `model.call.started`, `tool.call.started`).
 *
 * These stay pure so they can be unit-tested without React.
 */

import type { RunEvent } from "@/hooks/use-run-events";
import type {
  ModelCallCompletedPayload,
  ModelCallStartedPayload,
  RunCompletedPayload,
  RunFailedPayload,
  SandboxCommandPayload,
  SandboxFilePayload,
  ScoringMetricPayload,
  ToolCallPayload,
} from "./payload-types";

export type ArenaEventKind =
  | "model"
  | "tool"
  | "sandbox"
  | "file"
  | "scoring"
  | "system"
  | "unknown";

export interface TickerEntry {
  id: string;
  occurredAt: string;
  kind: ArenaEventKind;
  headline: string;
  detail?: string;
  /** True when the event terminated in an error. */
  error?: boolean;
}

export interface NowDoing {
  kind: ArenaEventKind;
  label: string;
  detail?: string;
  /** Monotonic sequence number of the event that set this state. */
  sequence: number;
  startedAt: string;
}

function payload<T>(event: RunEvent): T {
  return (event.Payload as T) ?? ({} as T);
}

function truncate(value: string | undefined, max = 80): string | undefined {
  if (!value) return value;
  const trimmed = value.replace(/\s+/g, " ").trim();
  if (trimmed.length <= max) return trimmed;
  return trimmed.slice(0, max - 1) + "\u2026";
}

function modelLabel(
  providerKey?: string,
  modelID?: string,
): string {
  if (providerKey && modelID) return `${providerKey}/${modelID}`;
  return modelID || providerKey || "model";
}

export function eventKind(event: RunEvent): ArenaEventKind {
  const t = event.EventType;
  if (t.startsWith("model.")) return "model";
  if (t.startsWith("tool.")) return "tool";
  if (t.startsWith("sandbox.command")) return "sandbox";
  if (t.startsWith("sandbox.file") || t.startsWith("grader.verification.file"))
    return "file";
  if (t.startsWith("grader.verification")) return "sandbox";
  if (t.startsWith("scoring.")) return "scoring";
  if (t.startsWith("race.")) return "system";
  if (t.startsWith("system.")) return "system";
  return "unknown";
}

/**
 * Returns a compact ticker entry for the given event, or `null` when the
 * event is too noisy to surface (e.g. individual stream-delta tokens).
 */
export function summarizeEvent(event: RunEvent): TickerEntry | null {
  const kind = eventKind(event);
  const base = {
    id: event.EventID,
    occurredAt: event.OccurredAt,
    kind,
  };

  switch (event.EventType) {
    case "system.run.started":
      return { ...base, headline: "Run started" };

    case "system.run.completed": {
      const p = payload<RunCompletedPayload>(event);
      return {
        ...base,
        headline: "Run completed",
        detail:
          p.total_tokens != null
            ? `${p.step_count ?? 0} steps \u00b7 ${p.total_tokens} tokens`
            : undefined,
      };
    }

    case "system.run.failed": {
      const p = payload<RunFailedPayload>(event);
      return {
        ...base,
        headline: "Run failed",
        detail: truncate(p.error),
        error: true,
      };
    }

    case "system.output.finalized":
      return { ...base, headline: "Output finalized" };

    case "system.step.started": {
      const step =
        (payload<{ step_index?: number }>(event).step_index ??
          event.Summary?.step_index) as number | undefined;
      return {
        ...base,
        headline: step != null ? `Step ${step} started` : "Step started",
      };
    }

    case "system.step.completed":
      return null; // noisy; step.started already covers it

    case "model.call.started": {
      const p = payload<ModelCallStartedPayload>(event);
      return {
        ...base,
        headline: `Calling ${modelLabel(p.provider_key, p.provider_model_id ?? p.model)}`,
      };
    }

    case "model.call.completed": {
      const p = payload<ModelCallCompletedPayload>(event);
      const tokens = p.usage?.total_tokens;
      return {
        ...base,
        headline: `Model replied${p.finish_reason ? ` (${p.finish_reason})` : ""}`,
        detail: tokens != null ? `${tokens} tokens` : truncate(p.output_text),
      };
    }

    case "model.output.delta":
      return null; // streamed token fragments are rendered separately

    case "model.tool_calls.proposed":
      return { ...base, headline: "Model proposed tool calls" };

    case "tool.call.started": {
      const p = payload<ToolCallPayload>(event);
      return {
        ...base,
        headline: `Tool \u2192 ${p.tool_name ?? "unknown"}`,
      };
    }

    case "tool.call.completed":
    case "tool.call.failed": {
      const p = payload<ToolCallPayload>(event);
      const failed = event.EventType === "tool.call.failed" || p.result?.is_error;
      return {
        ...base,
        headline: `${failed ? "Tool failed" : "Tool ok"}: ${p.tool_name ?? "unknown"}`,
        detail: truncate(p.result?.content),
        error: Boolean(failed),
      };
    }

    case "sandbox.command.started": {
      const p = payload<SandboxCommandPayload>(event);
      return {
        ...base,
        headline: `$ ${truncate(p.command, 60) ?? "command"}`,
      };
    }

    case "sandbox.command.completed":
    case "sandbox.command.failed": {
      const p = payload<SandboxCommandPayload>(event);
      const failed =
        event.EventType === "sandbox.command.failed" ||
        (p.exit_code != null && p.exit_code !== 0);
      return {
        ...base,
        headline: failed
          ? `Command failed (exit ${p.exit_code ?? "?"})`
          : `Command ok${p.duration_ms != null ? ` (${p.duration_ms}ms)` : ""}`,
        detail: truncate(p.stderr || p.stdout),
        error: failed,
      };
    }

    case "sandbox.file.read": {
      const p = payload<SandboxFilePayload>(event);
      return { ...base, headline: `Read ${truncate(p.path, 60) ?? "file"}` };
    }

    case "sandbox.file.written": {
      const p = payload<SandboxFilePayload>(event);
      return { ...base, headline: `Wrote ${truncate(p.path, 60) ?? "file"}` };
    }

    case "sandbox.file.listed": {
      const p = payload<SandboxFilePayload>(event);
      return { ...base, headline: `Listed ${truncate(p.path, 60) ?? "dir"}` };
    }

    case "grader.verification.file_captured":
    case "grader.verification.directory_listed":
    case "grader.verification.code_executed":
      return { ...base, headline: "Grader captured evidence" };

    case "race.standings.injected": {
      const p = payload<{ tokens_added?: number; triggered_by?: string; standings_snapshot?: string }>(event);
      return {
        ...base,
        headline: "Race standings injected",
        detail: p.standings_snapshot || (p.triggered_by ? `Trigger: ${p.triggered_by.replace(/_/g, " ")}` : undefined),
      };
    }

    case "scoring.started":
      return { ...base, headline: "Scoring started" };

    case "scoring.metric.recorded": {
      const p = payload<ScoringMetricPayload>(event);
      const scorePct =
        p.score != null ? `${Math.round(p.score * 100)}%` : undefined;
      return {
        ...base,
        headline: `Metric ${p.metric_key ?? "?"}${scorePct ? `: ${scorePct}` : ""}`,
        detail: p.passed == null ? undefined : p.passed ? "passed" : "failed",
        error: p.passed === false,
      };
    }

    case "scoring.completed":
      return { ...base, headline: "Scoring completed" };

    case "scoring.failed":
      return { ...base, headline: "Scoring failed", error: true };

    default:
      return { ...base, headline: event.EventType };
  }
}

/**
 * Returns a "what is this agent actively doing" label for events that
 * represent the *start* of a long-running action. Returns null for events
 * that complete or terminate an action so callers know to clear the
 * current-action slot.
 */
export function deriveNowDoing(event: RunEvent): NowDoing | "clear" | null {
  switch (event.EventType) {
    case "model.call.started": {
      const p = payload<ModelCallStartedPayload>(event);
      return {
        kind: "model",
        label: `Calling ${modelLabel(p.provider_key, p.provider_model_id ?? p.model)}`,
        sequence: event.SequenceNumber,
        startedAt: event.OccurredAt,
      };
    }

    case "tool.call.started": {
      const p = payload<ToolCallPayload>(event);
      return {
        kind: "tool",
        label: `Tool: ${p.tool_name ?? "unknown"}`,
        sequence: event.SequenceNumber,
        startedAt: event.OccurredAt,
      };
    }

    case "sandbox.command.started": {
      const p = payload<SandboxCommandPayload>(event);
      return {
        kind: "sandbox",
        label: `$ ${truncate(p.command, 60) ?? "command"}`,
        sequence: event.SequenceNumber,
        startedAt: event.OccurredAt,
      };
    }

    case "scoring.started":
      return {
        kind: "scoring",
        label: "Scoring",
        sequence: event.SequenceNumber,
        startedAt: event.OccurredAt,
      };

    case "system.step.started": {
      const p = payload<{ step_index?: number }>(event);
      const step = p.step_index ?? event.Summary?.step_index;
      return {
        kind: "system",
        label: step != null ? `Step ${step}` : "Stepping",
        sequence: event.SequenceNumber,
        startedAt: event.OccurredAt,
      };
    }

    case "model.call.completed":
    case "tool.call.completed":
    case "tool.call.failed":
    case "sandbox.command.completed":
    case "sandbox.command.failed":
    case "scoring.completed":
    case "scoring.failed":
    case "system.run.completed":
    case "system.run.failed":
      return "clear";

    default:
      return null;
  }
}
