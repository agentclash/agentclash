import { describe, expect, it } from "vitest";

import type { GetEvalSetResponse } from "@/lib/api/types";
import {
  agentRefFromMatrixKey,
  buildMatrixGrid,
  buildRefLabelMap,
  comboRepeatLabel,
  completionPercent,
  countCellsByState,
  displayRef,
  evalSetLiveKey,
  evalSetStatusLabel,
  inFlightCount,
  matrixCellStateLabel,
  shortRef,
} from "../eval-sets";

function detailFixture(): GetEvalSetResponse {
  const packs = ["pack-a", "pack-b", "pack-c"];
  const agents = ["agent-1", "agent-2", "agent-3", "agent-4"];
  const combinations = [];
  for (const pack of packs) {
    for (const agent of agents) {
      for (let r = 1; r <= 5; r++) {
        combinations.push({
          matrix_key: `${pack}/${agent}/${r}`,
          pack_ref: pack,
          agent_ref: agent,
          repeat: r,
        });
      }
    }
  }
  return {
    eval_set: {
      id: "es-1",
      workspace_id: "ws-1",
      organization_id: "org-1",
      name: "sweep",
      status: "running",
      combination_count: 60,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      expansion: { combinations, count: 60 },
    },
  };
}

describe("agentRefFromMatrixKey", () => {
  it("strips multi-segment pack refs", () => {
    expect(
      agentRefFromMatrixKey(
        "catalog/code-review/claude-opus-5-default/1",
        "catalog/code-review",
      ),
    ).toBe("claude-opus-5-default");
  });
});

describe("buildMatrixGrid", () => {
  it("builds a 4×3 agent×pack grid for 3 packs × 4 agents × 5 repeats", () => {
    const grid = buildMatrixGrid(detailFixture());
    expect(grid.rowAxis).toBe("agent");
    expect(grid.rowLabels).toHaveLength(4);
    expect(grid.colLabels).toHaveLength(3);
    const cell = grid.cells[`agent-1\0pack-a`];
    expect(cell?.totalCount).toBe(5);
    expect(cell?.state).toBe("queued");
  });

  it("updates cell state from live run statuses without full page rebuild", () => {
    const detail = detailFixture();
    const live = {
      "pack-a/agent-1/1": { status: "running", runId: "run-1" },
      "pack-a/agent-1/2": { status: "completed", runId: "run-2" },
      "pack-a/agent-1/3": { status: "queued", runId: "run-3" },
      "pack-a/agent-1/4": { status: "queued" },
      "pack-a/agent-1/5": { status: "queued" },
    };
    const grid = buildMatrixGrid(detail, live);
    expect(grid.cells[`agent-1\0pack-a`]?.state).toBe("running");
    expect(grid.cells[`agent-2\0pack-a`]?.state).toBe("queued");

    const scoredLive = Object.fromEntries(
      [1, 2, 3, 4, 5].map((r) => [
        `pack-a/agent-1/${r}`,
        { status: "completed", runId: `run-${r}` },
      ]),
    );
    const scored = buildMatrixGrid(detail, scoredLive);
    expect(scored.cells[`agent-1\0pack-a`]?.state).toBe("scored");
    expect(scored.cells[`agent-1\0pack-a`]?.passCount).toBe(5);
  });

  it("flips axes when packs outnumber agents", () => {
    const detail = detailFixture();
    detail.eval_set.expansion = {
      combinations: [
        {
          matrix_key: "p1/a1/1",
          pack_ref: "p1",
          agent_ref: "a1",
          repeat: 1,
        },
        {
          matrix_key: "p2/a1/1",
          pack_ref: "p2",
          agent_ref: "a1",
          repeat: 1,
        },
        {
          matrix_key: "p3/a1/1",
          pack_ref: "p3",
          agent_ref: "a1",
          repeat: 1,
        },
      ],
      count: 3,
    };
    detail.eval_set.combination_count = 3;
    const grid = buildMatrixGrid(detail);
    expect(grid.rowAxis).toBe("pack");
    expect(grid.rowLabels).toEqual(["p1", "p2", "p3"]);
    expect(grid.colLabels).toEqual(["a1"]);
  });
});

describe("completionPercent / inFlightCount", () => {
  it("computes completion from live map", () => {
    const detail = detailFixture();
    const live: Record<string, { status: string }> = {};
    for (let i = 0; i < 30; i++) {
      live[`k-${i}`] = { status: "completed" };
    }
    for (let i = 30; i < 60; i++) {
      live[`k-${i}`] = { status: i === 30 ? "running" : "queued" };
    }
    expect(completionPercent(detail, live)).toBe(50);
    expect(inFlightCount(live)).toBe(1);
  });
});

describe("displayRef / buildRefLabelMap", () => {
  it("shortens UUIDs and keeps human names", () => {
    expect(shortRef("code-exec-smoke")).toBe("code-exec-smoke");
    expect(shortRef("037535f2-8b36-4ce0-9ee5-21a2984f5e47")).toBe(
      "037535f2…4f5e47",
    );
  });

  it("resolves deployment and pack version names", () => {
    const detail = detailFixture();
    detail.eval_set.expansion = {
      combinations: [
        {
          matrix_key: "pv-1/dep-1/1",
          pack_ref: "pv-1",
          agent_ref: "dep-1",
          agent_label: "From Manifest",
          repeat: 1,
        },
      ],
      count: 1,
    };
    const labels = buildRefLabelMap(
      detail,
      [
        {
          id: "dep-1",
          organization_id: "o",
          workspace_id: "w",
          current_build_version_id: "bv",
          name: "Gemini Flash",
          status: "active",
          created_at: "",
          updated_at: "",
        },
      ],
      [
        {
          id: "pack-1",
          name: "Code Exec Smoke",
          slug: "code-exec-smoke",
          versions: [
            {
              id: "pv-1",
              challenge_pack_id: "pack-1",
              version_number: 2,
              lifecycle_status: "runnable",
              created_at: "",
              updated_at: "",
            },
          ],
          created_at: "",
          updated_at: "",
        },
      ],
    );
    // manifest agent_label wins over deployment name
    expect(displayRef("dep-1", labels)).toBe("From Manifest");
    expect(displayRef("pv-1", labels)).toBe("Code Exec Smoke v2");
  });

  it("counts cell states and formats helpers", () => {
    const grid = buildMatrixGrid(detailFixture());
    const counts = countCellsByState(grid);
    expect(counts.queued).toBe(12);
    expect(comboRepeatLabel("pack/agent/3")).toBe("3");
    expect(evalSetStatusLabel("budget_exceeded")).toBe("Budget exceeded");
    // scored = runs finished, not case-verdict pass
    expect(matrixCellStateLabel("scored")).toBe("Complete");
  });
});

describe("evalSetLiveKey", () => {
  it("has no key before the set loads or without sessions", () => {
    expect(evalSetLiveKey("es-1", undefined, ["s-1"])).toBeNull();
    expect(evalSetLiveKey("es-1", "running", [])).toBeNull();
  });

  it("keeps a key after the set ends, with a different phase", () => {
    const live = evalSetLiveKey("es-1", "running", ["s-1"]);
    expect(live).toEqual(["eval-set-live", "es-1", "live", "s-1"]);
    for (const status of ["completed", "failed", "cancelled", "budget_exceeded"] as const) {
      expect(evalSetLiveKey("es-1", status, ["s-1"])).toEqual([
        "eval-set-live",
        "es-1",
        "final",
        "s-1",
      ]);
    }
  });
});

describe("terminal sets", () => {
  it("shows final cell states from the kept live map of a cancelled set", () => {
    const detail = detailFixture();
    detail.eval_set.status = "cancelled";
    const live = Object.fromEntries(
      [1, 2, 3, 4, 5].map((r) => [
        `pack-a/agent-1/${r}`,
        { status: r <= 2 ? "completed" : "cancelled", runId: `run-${r}` },
      ]),
    );
    const grid = buildMatrixGrid(detail, live);
    const cell = grid.cells[`agent-1\0pack-a`];
    expect(cell?.state).toBe("failed");
    expect(cell?.doneCount).toBe(5);
    expect(cell?.passCount).toBe(2);
  });

  it("does not treat the unused canceled spelling as terminal", () => {
    const detail = detailFixture();
    detail.eval_set.combination_count = 2;
    expect(
      completionPercent(detail, {
        a: { status: "cancelled" },
        b: { status: "canceled" },
      }),
    ).toBe(50);
  });
});
