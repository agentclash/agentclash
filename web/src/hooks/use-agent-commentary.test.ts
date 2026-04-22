import React, { act, useEffect } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it } from "vitest";

import type { RunAgent } from "@/lib/api/types";
import type { RunEvent } from "@/hooks/use-run-events";

import {
  buildCommentaryEntry,
  MAX_COMMENTARY_ENTRIES,
  useAgentCommentary,
  type UseAgentCommentaryResult,
} from "./use-agent-commentary";

const agents: RunAgent[] = [
  {
    id: "agent-1",
    run_id: "run-1",
    lane_index: 0,
    label: "Alpha",
    agent_deployment_id: "dep-1",
    agent_deployment_snapshot_id: "snap-1",
    status: "executing",
    created_at: "2026-04-22T12:00:00Z",
    updated_at: "2026-04-22T12:00:00Z",
  },
];

function makeEvent(
  overrides: Partial<RunEvent> = {},
): RunEvent {
  return {
    EventID: "evt-1",
    SchemaVersion: "2026-03-15",
    RunID: "run-1",
    RunAgentID: "agent-1",
    SequenceNumber: 1,
    EventType: "system.run.started",
    Source: "native_engine",
    OccurredAt: "2026-04-22T12:00:00Z",
    Payload: {},
    Summary: {},
    ...overrides,
  };
}

function HookHarness({
  onReady,
}: {
  onReady: (value: UseAgentCommentaryResult) => void;
}) {
  const result = useAgentCommentary(agents);

  useEffect(() => {
    onReady(result);
  }, [onReady, result]);

  return null;
}

function renderHarness(onReady: (value: UseAgentCommentaryResult) => void) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  act(() => {
    root.render(React.createElement(HookHarness, { onReady }));
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

describe("buildCommentaryEntry", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  it("creates commentary for meaningful live events", () => {
    const entry = buildCommentaryEntry(
      makeEvent({
        EventID: "evt-tool",
        EventType: "tool.call.started",
        Payload: { tool_name: "search_query" },
      }),
      "Alpha",
    );

    expect(entry?.line).toContain("Alpha reaches for search_query");
    expect(entry?.tone).toBe("neutral");
  });

  it("suppresses noisy delta events", () => {
    const entry = buildCommentaryEntry(
      makeEvent({
        EventID: "evt-delta",
        EventType: "model.output.delta",
        Payload: { text_delta: "hi" },
      }),
      "Alpha",
    );

    expect(entry).toBeNull();
  });
});

describe("useAgentCommentary", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  it("deduplicates entries by event id", () => {
    let current: UseAgentCommentaryResult | null = null;
    const { cleanup } = renderHarness((value) => {
      current = value;
    });

    act(() => {
      current?.handleEvent(
        makeEvent({
          EventID: "evt-dup",
          EventType: "tool.call.started",
          Payload: { tool_name: "search_query" },
        }),
      );
      current?.handleEvent(
        makeEvent({
          EventID: "evt-dup",
          EventType: "tool.call.started",
          Payload: { tool_name: "search_query" },
        }),
      );
    });

    expect(current?.entries).toHaveLength(1);
    cleanup();
  });

  it("keeps the commentary feed bounded", () => {
    let current: UseAgentCommentaryResult | null = null;
    const { cleanup } = renderHarness((value) => {
      current = value;
    });

    act(() => {
      for (let index = 0; index < MAX_COMMENTARY_ENTRIES + 3; index += 1) {
        current?.handleEvent(
          makeEvent({
            EventID: `evt-${index}`,
            SequenceNumber: index + 1,
            OccurredAt: `2026-04-22T12:00:${String(index).padStart(2, "0")}Z`,
            EventType: "system.step.started",
            Summary: { step_index: index + 1 },
          }),
        );
      }
    });

    expect(current?.entries).toHaveLength(MAX_COMMENTARY_ENTRIES);
    expect(current?.entries[0]?.id).toBe("evt-3");
    expect(current?.entries.at(-1)?.id).toBe(
      `evt-${MAX_COMMENTARY_ENTRIES + 2}`,
    );

    cleanup();
  });
});
