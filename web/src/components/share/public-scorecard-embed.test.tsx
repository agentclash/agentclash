import React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";

import {
  PublicScorecardEmbed,
} from "./public-scorecard-embed";
import { canRenderScorecardEmbed } from "./public-scorecard-embed-utils";

let root: Root | null = null;
let container: HTMLDivElement | null = null;

function render(element: React.ReactElement) {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root?.render(element);
  });
}

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
});

describe("PublicScorecardEmbed", () => {
  it("renders run scorecard shares as a compact embeddable ranking", () => {
    render(<PublicScorecardEmbed resource={runScorecardResource()} />);

    expect(
      container?.querySelector('[data-agentclash-embed="scorecard"]'),
    ).toBeTruthy();
    expect(container?.textContent).toContain("Quarterly support run");
    expect(container?.textContent).toContain("Ranking");
    expect(container?.textContent).toContain("Careful agent");
    expect(container?.textContent).toContain("92.0%");
    expect(container?.textContent).toContain("Winner");
  });

  it("renders agent scorecard dimensions in an embed-safe widget", () => {
    render(<PublicScorecardEmbed resource={agentScorecardResource()} />);

    expect(container?.textContent).toContain("Careful agent");
    expect(container?.textContent).toContain("Dimensions");
    expect(container?.textContent).toContain("Correctness");
    expect(container?.textContent).toContain("95.0%");
    expect(container?.textContent).toContain("Pass");
  });

  it("does not badge a top-ranked agent as winner until one is recorded", () => {
    const resource = runScorecardResource();
    resource.scorecard = {};

    render(<PublicScorecardEmbed resource={resource} />);

    expect(container?.textContent).toContain("Pending");
    expect(container?.textContent?.match(/Winner/g)?.length).toBe(1);
  });

  it("only accepts public scorecard resource types", () => {
    expect(canRenderScorecardEmbed({ type: "run_scorecard" })).toBe(true);
    expect(canRenderScorecardEmbed({ type: "run_agent_scorecard" })).toBe(true);
    expect(canRenderScorecardEmbed({ type: "run_agent_replay" })).toBe(false);
    expect(canRenderScorecardEmbed({ type: "eval_pack_version" })).toBe(false);
  });
});

function runScorecardResource() {
  return {
    type: "run_scorecard",
    run: {
      id: "run-1",
      name: "Quarterly support run",
      status: "completed",
      started_at: "2026-05-11T10:00:00Z",
      finished_at: "2026-05-11T10:01:40Z",
    },
    agents: [
      { id: "agent-1", label: "Fast agent", lane_index: 0, status: "completed" },
      {
        id: "agent-2",
        label: "Careful agent",
        lane_index: 1,
        status: "completed",
      },
    ],
    agent_scorecards: [
      {
        run_agent_id: "agent-1",
        overall_score: 0.74,
        passed: true,
      },
      {
        run_agent_id: "agent-2",
        overall_score: 0.92,
        passed: true,
      },
    ],
    scorecard: {
      winning_run_agent_id: "agent-2",
    },
  };
}

function agentScorecardResource() {
  return {
    type: "run_agent_scorecard",
    run: {
      id: "run-1",
      name: "Quarterly support run",
      status: "completed",
    },
    run_agent: {
      id: "agent-2",
      label: "Careful agent",
      lane_index: 1,
      status: "completed",
      started_at: "2026-05-11T10:00:00Z",
      finished_at: "2026-05-11T10:01:40Z",
    },
    scorecard: {
      overall_score: 0.92,
      passed: true,
      scorecard: {
        dimensions: {
          correctness: { state: "available", score: 0.95 },
          reliability: { state: "available", score: 0.88 },
          latency: { state: "missing" },
        },
      },
    },
  };
}
