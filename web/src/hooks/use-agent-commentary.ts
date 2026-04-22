"use client";

import { useCallback, useReducer } from "react";

import type { RunAgent } from "@/lib/api/types";
import type { RunEvent } from "@/hooks/use-run-events";
import type {
  ModelCallStartedPayload,
  RunFailedPayload,
  SandboxCommandPayload,
  ScoringMetricPayload,
  ToolCallPayload,
} from "@/lib/arena/payload-types";

export interface CommentaryEntry {
  id: string;
  occurredAt: string;
  agentId: string;
  agentLabel: string;
  line: string;
  detail?: string;
  tone: "neutral" | "positive" | "warning";
}

export const MAX_COMMENTARY_ENTRIES = 24;

type CommentaryAction =
  | { type: "event"; event: RunEvent; agentLabel: string }
  | { type: "reset" };

function truncate(value: string | undefined, max = 96): string | undefined {
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
  return modelID || providerKey || "its model";
}

function shortAgentLabel(runAgentID: string): string {
  if (!runAgentID) return "An agent";
  return `Agent ${runAgentID.slice(0, 8)}`;
}

function payload<T>(event: RunEvent): T {
  return (event.Payload as T) ?? ({} as T);
}

export function buildCommentaryEntry(
  event: RunEvent,
  agentLabel: string,
): CommentaryEntry | null {
  const label = agentLabel || shortAgentLabel(event.RunAgentID);

  switch (event.EventType) {
    case "system.run.started":
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} is out of the gate.`,
        tone: "neutral",
      };

    case "system.step.started": {
      const stepIndex =
        (event.Summary?.step_index as number | undefined) ??
        (payload<{ step_index?: number }>(event).step_index);
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line:
          stepIndex != null
            ? `${label} moves into step ${stepIndex}.`
            : `${label} lines up the next step.`,
        tone: "neutral",
      };
    }

    case "model.call.started": {
      const p = payload<ModelCallStartedPayload>(event);
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} checks in with ${modelLabel(
          p.provider_key,
          p.provider_model_id ?? p.model,
        )}.`,
        tone: "neutral",
      };
    }

    case "tool.call.started": {
      const p = payload<ToolCallPayload>(event);
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} reaches for ${p.tool_name ?? "a tool"}.`,
        tone: "neutral",
      };
    }

    case "tool.call.completed": {
      const p = payload<ToolCallPayload>(event);
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} gets a result back from ${p.tool_name ?? "that tool"}.`,
        detail: truncate(p.result?.content),
        tone: p.result?.is_error ? "warning" : "positive",
      };
    }

    case "tool.call.failed": {
      const p = payload<ToolCallPayload>(event);
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} hits a snag in ${p.tool_name ?? "a tool call"}.`,
        detail: truncate(p.result?.content),
        tone: "warning",
      };
    }

    case "sandbox.command.started": {
      const p = payload<SandboxCommandPayload>(event);
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} drops into the sandbox.`,
        detail: truncate(p.command, 72),
        tone: "neutral",
      };
    }

    case "scoring.started":
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} heads to scoring.`,
        tone: "neutral",
      };

    case "scoring.metric.recorded": {
      const p = payload<ScoringMetricPayload>(event);
      const score =
        p.score != null ? `${Math.round(p.score * 100)}%` : undefined;
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} posts ${
          p.metric_key ?? "a metric"
        }${score ? ` at ${score}` : ""}.`,
        detail:
          p.passed == null ? undefined : p.passed ? "Passed" : "Failed",
        tone: p.passed === false ? "warning" : "positive",
      };
    }

    case "system.run.completed":
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} crosses the line with a final answer.`,
        tone: "positive",
      };

    case "system.run.failed": {
      const p = payload<RunFailedPayload>(event);
      return {
        id: event.EventID,
        occurredAt: event.OccurredAt,
        agentId: event.RunAgentID,
        agentLabel: label,
        line: `${label} drops out of the live run.`,
        detail: truncate(p.error ?? p.stop_reason),
        tone: "warning",
      };
    }

    default:
      return null;
  }
}

function reducer(
  state: CommentaryEntry[],
  action: CommentaryAction,
): CommentaryEntry[] {
  switch (action.type) {
    case "event": {
      const entry = buildCommentaryEntry(action.event, action.agentLabel);
      if (!entry || state.some((item) => item.id === entry.id)) {
        return state;
      }
      const next = [...state, entry];
      if (next.length > MAX_COMMENTARY_ENTRIES) {
        next.splice(0, next.length - MAX_COMMENTARY_ENTRIES);
      }
      return next;
    }

    case "reset":
      return [];
  }
}

export interface UseAgentCommentaryResult {
  entries: CommentaryEntry[];
  handleEvent: (event: RunEvent) => void;
  reset: () => void;
}

export function useAgentCommentary(
  agents: RunAgent[],
): UseAgentCommentaryResult {
  const [entries, dispatch] = useReducer(reducer, []);

  const handleEvent = useCallback(
    (event: RunEvent) => {
      const agentLabel =
        agents.find((agent) => agent.id === event.RunAgentID)?.label ??
        shortAgentLabel(event.RunAgentID);
      dispatch({ type: "event", event, agentLabel });
    },
    [agents],
  );

  const reset = useCallback(() => {
    dispatch({ type: "reset" });
  }, []);

  return { entries, handleEvent, reset };
}
