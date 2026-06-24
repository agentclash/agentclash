import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { AgentDeployment, EvalPack } from "@/lib/api/types";
import { useWorkspaceReadiness } from "./workspace-readiness";

// Controllable per-endpoint SWR state.
const { state } = vi.hoisted(() => ({
  state: {
    providers: [] as unknown[],
    deployments: [] as Partial<AgentDeployment>[],
    packs: [] as Partial<EvalPack>[],
    runsTotal: 0,
    loading: false,
  },
}));

vi.mock("@/lib/api/swr", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/swr")>();
  return {
    ...actual,
    useApiListQuery: (path: string) => {
      const items = path.endsWith("/provider-accounts")
        ? state.providers
        : path.endsWith("/agent-deployments")
          ? state.deployments
          : path.endsWith("/eval-packs")
            ? state.packs
            : [];
      return {
        data: state.loading ? undefined : { items },
        isLoading: state.loading,
        error: null,
        mutate: vi.fn(),
      };
    },
    usePaginatedApiQuery: () => ({
      data: state.loading
        ? undefined
        : { items: [], total: state.runsTotal, limit: 1, offset: 0 },
      isLoading: state.loading,
      error: null,
      mutate: vi.fn(),
    }),
  };
});

// Render readiness to the DOM and read it back (avoids mutating outer scope
// from inside the component, matching the repo's other hook tests).
function Harness() {
  const r = useWorkspaceReadiness("ws_1");
  return React.createElement(
    "div",
    { "data-testid": "out" },
    JSON.stringify({
      ready: r.ready,
      allComplete: r.allComplete,
      nextStep: r.nextStep?.key ?? null,
      isLoading: r.isLoading,
      done: r.steps.map((s) => s.done),
    }),
  );
}

let container: HTMLDivElement;
let root: Root;

interface Snapshot {
  ready: boolean;
  allComplete: boolean;
  nextStep: string | null;
  isLoading: boolean;
  done: boolean[];
}

function render(): Snapshot {
  act(() => {
    root.render(React.createElement(Harness));
  });
  const text = container.querySelector("[data-testid=out]")?.textContent ?? "{}";
  return JSON.parse(text) as Snapshot;
}

beforeEach(() => {
  state.providers = [];
  state.deployments = [];
  state.packs = [];
  state.runsTotal = 0;
  state.loading = false;
  container = document.createElement("div");
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
});

const runnablePack: Partial<EvalPack> = {
  id: "p1",
  versions: [{ lifecycle_status: "runnable" } as EvalPack["versions"][number]],
};
const activeDeployment: Partial<AgentDeployment> = { id: "d1", status: "active" };

describe("useWorkspaceReadiness", () => {
  it("orders the chain from a fresh workspace", () => {
    const r = render();
    expect(r.ready).toBe(false);
    expect(r.allComplete).toBe(false);
    expect(r.nextStep).toBe("provider");
    expect(r.done).toEqual([false, false, false, false]);
  });

  it("advances to deployment once a provider exists", () => {
    state.providers = [{ id: "pa1" }];
    const r = render();
    expect(r.nextStep).toBe("deployment");
    expect(r.done[0]).toBe(true);
  });

  it("does not count a non-active deployment", () => {
    state.providers = [{ id: "pa1" }];
    state.deployments = [{ id: "d1", status: "paused" }];
    const r = render();
    expect(r.nextStep).toBe("deployment");
  });

  it("does not count a pack without a runnable version", () => {
    state.providers = [{ id: "pa1" }];
    state.deployments = [activeDeployment];
    state.packs = [{ id: "p1", versions: [] }];
    const r = render();
    expect(r.nextStep).toBe("eval_pack");
    expect(r.ready).toBe(false);
  });

  it("is ready to run with provider + active deployment + runnable pack", () => {
    state.providers = [{ id: "pa1" }];
    state.deployments = [activeDeployment];
    state.packs = [runnablePack];
    const r = render();
    expect(r.ready).toBe(true);
    expect(r.allComplete).toBe(false);
    expect(r.nextStep).toBe("first_run");
  });

  it("is fully complete once a run exists", () => {
    state.providers = [{ id: "pa1" }];
    state.deployments = [activeDeployment];
    state.packs = [runnablePack];
    state.runsTotal = 1;
    const r = render();
    expect(r.allComplete).toBe(true);
    expect(r.nextStep).toBeNull();
  });

  it("reports loading while data is in flight", () => {
    state.loading = true;
    const r = render();
    expect(r.isLoading).toBe(true);
  });
});
