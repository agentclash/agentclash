"use client";

import { useEffect, useRef, useState } from "react";

export interface RunEvent {
  EventID: string;
  SchemaVersion: string;
  RunID: string;
  RunAgentID: string;
  SequenceNumber: number;
  EventType: string;
  Source: string;
  OccurredAt: string;
  Payload: unknown;
  Summary: Record<string, unknown>;
}

interface UseRunEventsOptions {
  runId: string;
  token: string | undefined;
  enabled?: boolean;
  onEvent?: (event: RunEvent) => void;
}

interface UseRunEventsResult {
  connected: boolean;
  error: string | null;
}

function recordFromUnknown(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function stringField(
  record: Record<string, unknown>,
  ...keys: string[]
): string {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "string") return value;
  }
  return "";
}

function numberField(
  record: Record<string, unknown>,
  ...keys: string[]
): number {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "number" && Number.isFinite(value)) return value;
    if (typeof value === "string" && value.trim() !== "") {
      const parsed = Number(value);
      if (Number.isFinite(parsed)) return parsed;
    }
  }
  return 0;
}

function unknownField(
  record: Record<string, unknown>,
  ...keys: string[]
): unknown {
  for (const key of keys) {
    if (key in record) return record[key];
  }
  return undefined;
}

export function normalizeRunEvent(value: unknown): RunEvent | null {
  const record = recordFromUnknown(value);
  if (!record) return null;

  const summary = recordFromUnknown(
    unknownField(record, "Summary", "summary"),
  );

  return {
    EventID: stringField(record, "EventID", "event_id"),
    SchemaVersion: stringField(record, "SchemaVersion", "schema_version"),
    RunID: stringField(record, "RunID", "run_id"),
    RunAgentID: stringField(record, "RunAgentID", "run_agent_id"),
    SequenceNumber: numberField(
      record,
      "SequenceNumber",
      "sequence_number",
    ),
    EventType: stringField(record, "EventType", "event_type"),
    Source: stringField(record, "Source", "source"),
    OccurredAt: stringField(record, "OccurredAt", "occurred_at"),
    Payload: unknownField(record, "Payload", "payload") ?? {},
    Summary: summary ?? {},
  };
}

export function useRunEvents({
  runId,
  token,
  enabled = true,
  onEvent,
}: UseRunEventsOptions): UseRunEventsResult {
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const onEventRef = useRef(onEvent);
  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  useEffect(() => {
    if (!enabled || !token) return;

    const baseUrl = process.env.NEXT_PUBLIC_API_URL?.replace(/\/+$/, "");
    if (!baseUrl) return;

    const url = `${baseUrl}/v1/runs/${runId}/events/stream?token=${encodeURIComponent(token)}`;
    const es = new EventSource(url);

    es.onopen = () => {
      setConnected(true);
      setError(null);
    };

    es.addEventListener("run_event", (e: MessageEvent) => {
      try {
        const event = normalizeRunEvent(JSON.parse(e.data));
        if (event) onEventRef.current?.(event);
      } catch {
        // Malformed event data, skip
      }
    });

    es.onerror = () => {
      setConnected(false);
      setError("Connection lost, reconnecting...");
    };

    return () => {
      es.close();
      setConnected(false);
    };
  }, [runId, token, enabled]);

  return { connected, error };
}
