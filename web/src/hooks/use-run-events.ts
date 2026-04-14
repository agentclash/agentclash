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
        const event: RunEvent = JSON.parse(e.data);
        onEventRef.current?.(event);
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
