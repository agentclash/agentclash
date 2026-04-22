"use client";

import { useCallback, useReducer } from "react";

import type { RunEvent } from "@/hooks/use-run-events";
import {
  deriveNowDoing,
  summarizeEvent,
  type NowDoing,
  type TickerEntry,
} from "@/lib/arena/event-formatter";
import type {
  ModelCallCompletedPayload,
  ModelOutputDeltaPayload,
  RunCompletedPayload,
  ScoringMetricPayload,
} from "@/lib/arena/payload-types";

/**
 * Per-agent arena state. Populated incrementally from the SSE event stream
 * — we never refetch this; it's a live "what is this lane doing right now"
 * projection over the event log since the browser connected.
 */
export interface ArenaLaneState {
  /** Current long-running action (model call, tool call, command). */
  nowDoing: NowDoing | null;
  /** Last seen step index from system.step.started. */
  stepIndex: number;
  /** Number of completed model calls since connection. */
  modelCalls: number;
  /** Number of completed tool calls since connection. */
  toolCalls: number;
  /** Aggregated total tokens from model.call.completed events. */
  totalTokens: number;
  /** Text accumulated from model.output.delta since the last model.call.started. */
  streamingOutput: string;
  /** Bounded ring buffer of ticker entries, oldest-first. */
  ticker: TickerEntry[];
  /** Latest metric outcome, if any. */
  lastMetric?: {
    key: string;
    score?: number;
    passed?: boolean;
  };
  /** Final output text if the run completed. */
  finalOutput?: string;
}

const EMPTY_LANE: ArenaLaneState = {
  nowDoing: null,
  stepIndex: 0,
  modelCalls: 0,
  toolCalls: 0,
  totalTokens: 0,
  streamingOutput: "",
  ticker: [],
};

const MAX_TICKER_ENTRIES = 40;
const MAX_STREAMING_CHARS = 800;

type ArenaState = Record<string, ArenaLaneState>;

type ArenaAction =
  | { type: "event"; event: RunEvent }
  | { type: "reset" };

function lane(state: ArenaState, agentID: string): ArenaLaneState {
  return state[agentID] ?? EMPTY_LANE;
}

function clipOutput(value: string): string {
  if (value.length <= MAX_STREAMING_CHARS) return value;
  return "\u2026" + value.slice(value.length - MAX_STREAMING_CHARS + 1);
}

function applyEvent(
  state: ArenaState,
  event: RunEvent,
): ArenaState {
  const agentID = event.RunAgentID;
  if (!agentID) return state;

  const laneState: ArenaLaneState = { ...lane(state, agentID) };
  const payload = (event.Payload ?? {}) as Record<string, unknown>;

  // 1. Current "now doing" banner.
  const now = deriveNowDoing(event);
  if (now === "clear") {
    // Only clear if the sequence matches (or is newer than) the banner, so a
    // stale completion doesn't wipe a newly-started action.
    if (
      !laneState.nowDoing ||
      event.SequenceNumber >= laneState.nowDoing.sequence
    ) {
      laneState.nowDoing = null;
    }
  } else if (now) {
    laneState.nowDoing = now;
    if (event.EventType === "model.call.started") {
      laneState.streamingOutput = "";
    }
  }

  // 2. Streaming model output.
  if (event.EventType === "model.output.delta") {
    const delta = payload as ModelOutputDeltaPayload;
    if (delta.stream_kind === "text" && delta.text_delta) {
      laneState.streamingOutput = clipOutput(
        laneState.streamingOutput + delta.text_delta,
      );
    }
  }

  // 3. Counters.
  if (event.EventType === "system.step.started") {
    const step =
      (payload.step_index as number | undefined) ??
      event.Summary?.step_index;
    if (typeof step === "number") {
      laneState.stepIndex = Math.max(laneState.stepIndex, step);
    }
  }
  if (event.EventType === "model.call.completed") {
    const p = payload as ModelCallCompletedPayload;
    laneState.modelCalls += 1;
    const tokens = p.usage?.total_tokens;
    if (typeof tokens === "number") {
      laneState.totalTokens += tokens;
    }
  }
  if (
    event.EventType === "tool.call.completed" ||
    event.EventType === "tool.call.failed"
  ) {
    laneState.toolCalls += 1;
  }
  if (event.EventType === "scoring.metric.recorded") {
    const p = payload as ScoringMetricPayload;
    if (p.metric_key) {
      laneState.lastMetric = {
        key: p.metric_key,
        score: p.score,
        passed: p.passed,
      };
    }
  }
  if (event.EventType === "system.run.completed") {
    const p = payload as RunCompletedPayload;
    if (p.final_output) laneState.finalOutput = p.final_output;
  }

  // 4. Ticker. Append meaningful entries, drop duplicates by event ID.
  const entry = summarizeEvent(event);
  if (entry && !laneState.ticker.some((e) => e.id === entry.id)) {
    const next = [...laneState.ticker, entry];
    if (next.length > MAX_TICKER_ENTRIES) {
      next.splice(0, next.length - MAX_TICKER_ENTRIES);
    }
    laneState.ticker = next;
  }

  return { ...state, [agentID]: laneState };
}

function reducer(state: ArenaState, action: ArenaAction): ArenaState {
  switch (action.type) {
    case "event":
      return applyEvent(state, action.event);
    case "reset":
      return {};
  }
}

export interface UseAgentArenaResult {
  lanes: ArenaState;
  handleEvent: (event: RunEvent) => void;
  reset: () => void;
}

/**
 * Reducer-backed projection of the SSE event stream into per-agent arena
 * state. The returned `handleEvent` is stable, so it's safe to pass as the
 * `onEvent` callback for `useRunEvents`.
 */
export function useAgentArena(): UseAgentArenaResult {
  const [lanes, dispatch] = useReducer(reducer, {});

  const handleEvent = useCallback((event: RunEvent) => {
    dispatch({ type: "event", event });
  }, []);

  const reset = useCallback(() => {
    dispatch({ type: "reset" });
  }, []);

  return { lanes, handleEvent, reset };
}

export { EMPTY_LANE };
