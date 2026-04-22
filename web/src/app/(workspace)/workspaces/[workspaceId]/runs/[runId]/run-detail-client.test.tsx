import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { RunEvent } from "@/hooks/use-run-events";

import { RunDetailClient } from "./run-detail-client";

const { mockGetAccessToken, mockUseRunEvents, mockCreateApiClient } =
  vi.hoisted(() => ({
    mockGetAccessToken: vi.fn(),
    mockUseRunEvents: vi.fn(),
    mockCreateApiClient: vi.fn(),
  }));

let latestOnEvent: ((event: RunEvent) => void) | undefined;

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
  usePathname: () =>
    "/workspaces/workspace-1/runs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("@/hooks/use-run-events", async () => {
  const actual = await vi.importActual<typeof import("@/hooks/use-run-events")>(
    "@/hooks/use-run-events",
  );

  return {
    ...actual,
    useRunEvents: (options: {
      onEvent?: (event: RunEvent) => void;
    }) => {
      latestOnEvent = options.onEvent;
      return mockUseRunEvents(options);
    },
  };
});

vi.mock("@/lib/api/client", () => ({
  createApiClient: (...args: unknown[]) => mockCreateApiClient(...args),
}));

vi.mock("./compare-run-picker", () => ({
  CompareRunPicker: () => React.createElement("div", null, "compare-picker"),
}));

vi.mock("./scorecard-summary-card", () => ({
  ScorecardSummaryCard: () =>
    React.createElement("div", null, "scorecard-summary"),
}));

vi.mock("./run-ranking-insights-card", () => ({
  RunRankingInsightsCard: () =>
    React.createElement("div", null, "ranking-insights"),
}));

vi.mock("@/components/artifacts/upload-artifact-dialog", () => ({
  UploadArtifactDialog: () =>
    React.createElement("button", { type: "button" }, "upload-artifact"),
}));

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
  });
}

function clickElement(element: Element) {
  element.dispatchEvent(
    new MouseEvent("click", {
      bubbles: true,
      cancelable: true,
    }),
  );
}

function renderClient() {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  act(() => {
    root.render(
      React.createElement(RunDetailClient, {
        workspaceId: "ws-1",
        initialRun: {
          id: "run-1",
          workspace_id: "ws-1",
          challenge_pack_version_id: "cpv-1",
          official_pack_mode: "full",
          name: "Live Arena Run",
          status: "running",
          execution_mode: "comparison",
          created_at: "2026-04-22T12:00:00Z",
          updated_at: "2026-04-22T12:00:00Z",
          started_at: "2026-04-22T12:00:05Z",
          links: {
            self: "/v1/runs/run-1",
            agents: "/v1/runs/run-1/agents",
          },
        },
        initialAgents: [
          {
            id: "agent-1",
            run_id: "run-1",
            lane_index: 0,
            label: "Alpha",
            agent_deployment_id: "dep-1",
            agent_deployment_snapshot_id: "snap-1",
            status: "executing",
            started_at: "2026-04-22T12:00:05Z",
            created_at: "2026-04-22T12:00:00Z",
            updated_at: "2026-04-22T12:00:05Z",
          },
        ],
      }),
    );
  });

  return {
    container,
    cleanup() {
      act(() => {
        root.unmount();
      });
      container.remove();
    },
  };
}

describe("RunDetailClient", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    latestOnEvent = undefined;
    mockGetAccessToken.mockReset();
    mockUseRunEvents.mockReset();
    mockCreateApiClient.mockReset();
    mockGetAccessToken.mockResolvedValue("token-123");
    mockUseRunEvents.mockReturnValue({ connected: true, error: null });
    mockCreateApiClient.mockReturnValue({
      get: vi.fn(),
    });
  });

  it("toggles commentary, updates live state, and resets hidden commentary history", async () => {
    const { container, cleanup } = renderClient();
    await flushPromises();

    const toggle = Array.from(container.querySelectorAll("button")).find(
      (element) => element.textContent?.includes("Commentary Off"),
    );
    expect(toggle).toBeTruthy();

    act(() => {
      clickElement(toggle!);
    });

    expect(container.textContent).toContain("Commentary On");
    expect(container.textContent).toContain("Live sidebar callouts");

    act(() => {
      latestOnEvent?.({
        EventID: "evt-model-1",
        SchemaVersion: "2026-03-15",
        RunID: "run-1",
        RunAgentID: "agent-1",
        SequenceNumber: 12,
        EventType: "model.call.started",
        Source: "native_engine",
        OccurredAt: "2026-04-22T12:01:00Z",
        Payload: {
          provider_key: "openai",
          provider_model_id: "gpt-5.4-mini",
        },
        Summary: {},
      });
    });

    expect(container.textContent).toContain("Calling openai/gpt-5.4-mini");
    expect(container.textContent).toContain(
      "Alpha checks in with openai/gpt-5.4-mini.",
    );
    expect(container.textContent).toContain("12:01:00 UTC");

    act(() => {
      clickElement(toggle!);
    });

    expect(container.textContent).not.toContain("Live sidebar callouts");

    act(() => {
      latestOnEvent?.({
        EventID: "evt-hidden-tool",
        SchemaVersion: "2026-03-15",
        RunID: "run-1",
        RunAgentID: "agent-1",
        SequenceNumber: 13,
        EventType: "tool.call.started",
        Source: "native_engine",
        OccurredAt: "2026-04-22T12:01:10Z",
        Payload: {
          tool_name: "search_query",
        },
        Summary: {},
      });
    });

    expect(container.textContent).toContain("Tool: search_query");

    act(() => {
      clickElement(toggle!);
    });

    expect(container.textContent).toContain("Live sidebar callouts");
    expect(container.textContent).toContain("Waiting for the next call");
    expect(container.textContent).not.toContain(
      "Alpha checks in with openai/gpt-5.4-mini.",
    );
    expect(container.textContent).not.toContain(
      "Alpha reaches for search_query.",
    );

    cleanup();
  });
});
