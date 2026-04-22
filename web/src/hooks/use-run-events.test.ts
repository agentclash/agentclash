import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  normalizeRunEvent,
  useRunEvents,
  type RunEvent,
} from "./use-run-events";

vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.agentclash.test");

type EventListener = (event: MessageEvent) => void;

class MockEventSource {
  static instances: MockEventSource[] = [];

  url: string;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  private listeners = new Map<string, EventListener[]>();

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: EventListener) {
    const next = this.listeners.get(type) ?? [];
    next.push(listener);
    this.listeners.set(type, next);
  }

  emit(type: string, data: unknown) {
    const event = { data: JSON.stringify(data) } as MessageEvent;
    for (const listener of this.listeners.get(type) ?? []) {
      listener(event);
    }
  }
}

vi.stubGlobal("EventSource", MockEventSource as unknown as typeof EventSource);

function HookHarness({
  onEvent,
}: {
  onEvent: (event: RunEvent) => void;
}) {
  useRunEvents({
    runId: "run-1",
    token: "token-123",
    enabled: true,
    onEvent,
  });
  return null;
}

function renderHarness(onEvent: (event: RunEvent) => void) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  act(() => {
    root.render(React.createElement(HookHarness, { onEvent }));
  });

  return {
    cleanup() {
      act(() => {
        root.unmount();
      });
      container.remove();
    },
  };
}

describe("normalizeRunEvent", () => {
  beforeEach(() => {
    MockEventSource.instances = [];
    document.body.innerHTML = "";
  });

  it("converts snake_case SSE envelopes into the frontend event shape", () => {
    const event = normalizeRunEvent({
      event_id: "evt-1",
      schema_version: "2026-03-15",
      run_id: "run-1",
      run_agent_id: "agent-1",
      sequence_number: 7,
      event_type: "tool.call.started",
      source: "native_engine",
      occurred_at: "2026-04-22T12:00:00Z",
      payload: { tool_name: "search_query" },
      summary: { step_index: 3 },
    });

    expect(event).toEqual({
      EventID: "evt-1",
      SchemaVersion: "2026-03-15",
      RunID: "run-1",
      RunAgentID: "agent-1",
      SequenceNumber: 7,
      EventType: "tool.call.started",
      Source: "native_engine",
      OccurredAt: "2026-04-22T12:00:00Z",
      Payload: { tool_name: "search_query" },
      Summary: { step_index: 3 },
    });
  });

  it("preserves already-normalized event objects", () => {
    const event = normalizeRunEvent({
      EventID: "evt-2",
      SchemaVersion: "2026-03-15",
      RunID: "run-1",
      RunAgentID: "agent-2",
      SequenceNumber: 9,
      EventType: "system.run.completed",
      Source: "worker_scoring",
      OccurredAt: "2026-04-22T12:05:00Z",
      Payload: { final_output: "done" },
      Summary: { status: "completed" },
    });

    expect(event?.EventType).toBe("system.run.completed");
    expect(event?.RunAgentID).toBe("agent-2");
    expect(event?.Summary).toEqual({ status: "completed" });
  });
});

describe("useRunEvents", () => {
  beforeEach(() => {
    MockEventSource.instances = [];
    document.body.innerHTML = "";
  });

  it("normalizes incoming run_event messages before invoking onEvent", () => {
    const seen: RunEvent[] = [];
    const { cleanup } = renderHarness((event) => {
      seen.push(event);
    });

    const source = MockEventSource.instances[0];
    expect(source?.url).toContain("/v1/runs/run-1/events/stream?token=token-123");

    act(() => {
      source.emit("run_event", {
        event_id: "evt-9",
        schema_version: "2026-03-15",
        run_id: "run-1",
        run_agent_id: "agent-9",
        sequence_number: 11,
        event_type: "model.call.started",
        source: "native_engine",
        occurred_at: "2026-04-22T12:10:00Z",
        payload: {
          provider_key: "openai",
          provider_model_id: "gpt-5.4-mini",
        },
        summary: {},
      });
    });

    expect(seen).toHaveLength(1);
    expect(seen[0]).toMatchObject({
      EventID: "evt-9",
      RunAgentID: "agent-9",
      SequenceNumber: 11,
      EventType: "model.call.started",
    });

    cleanup();
  });
});
