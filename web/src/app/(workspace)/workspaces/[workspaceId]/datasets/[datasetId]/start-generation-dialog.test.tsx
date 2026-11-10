import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { DatasetGenerationJob } from "@/lib/api/types";
import { storeGenerationJobId } from "../dataset-ui-shared";
import { StartGenerationDialog } from "./start-generation-dialog";

const mocks = vi.hoisted(() => ({
  getAccessToken: vi.fn(async () => "token"),
  getJob: vi.fn(),
  refresh: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: mocks.refresh, push: vi.fn() }),
}));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mocks.getAccessToken }),
}));
vi.mock("sonner", () => ({ toast: mocks.toast }));
vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({
    get: vi.fn(async () => ({ items: [] })),
    paginated: vi.fn(async () => ({ items: [] })),
  }),
}));
vi.mock("@/lib/api/datasets", () => ({
  getDatasetGenerationJob: (...args: unknown[]) => mocks.getJob(...args),
  listDatasetGenerationRejections: vi.fn(async () => ({ items: [] })),
  startDatasetGeneration: vi.fn(),
}));

// A minimal Dialog that still honours open/onOpenChange.
vi.mock("@/components/ui/dialog", () => {
  const Ctx = React.createContext<{
    open: boolean;
    setOpen: (value: boolean) => void;
  }>({ open: false, setOpen: () => {} });
  const passthrough = ({ children }: { children: React.ReactNode }) =>
    React.createElement("div", null, children);
  return {
    Dialog: ({
      open,
      onOpenChange,
      children,
    }: {
      open: boolean;
      onOpenChange: (value: boolean) => void;
      children: React.ReactNode;
    }) =>
      React.createElement(
        Ctx.Provider,
        { value: { open, setOpen: onOpenChange } },
        children,
      ),
    DialogTrigger: ({ children }: { children: React.ReactNode }) => {
      const { setOpen } = React.useContext(Ctx);
      return React.createElement(
        "button",
        { "data-testid": "dialog-trigger", onClick: () => setOpen(true) },
        children,
      );
    },
    DialogContent: ({ children }: { children: React.ReactNode }) => {
      const { open } = React.useContext(Ctx);
      return open ? React.createElement("div", null, children) : null;
    },
    DialogDescription: passthrough,
    DialogFooter: passthrough,
    DialogHeader: passthrough,
    DialogTitle: passthrough,
  };
});

function job(status: DatasetGenerationJob["status"]): DatasetGenerationJob {
  return {
    id: "job-1",
    status,
    summary: {},
    target_count: 10,
    accepted_count: status === "completed" ? 10 : 2,
    rejected_count: 0,
  } as unknown as DatasetGenerationJob;
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  vi.useFakeTimers();
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  window.localStorage.clear();
  storeGenerationJobId("ds-1", "job-1");
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

async function openRunningJob() {
  // The first call loads the recent jobs list.
  mocks.getJob.mockResolvedValueOnce(job("running"));
  await act(async () =>
    root.render(<StartGenerationDialog workspaceId="ws-1" datasetId="ds-1" />),
  );
  await act(async () =>
    container
      .querySelector<HTMLButtonElement>('[data-testid="dialog-trigger"]')!
      .click(),
  );
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0);
  });
  const recent = [...container.querySelectorAll("button")].find((b) =>
    b.textContent?.startsWith("job-1"),
  )!;
  await act(async () => recent.click());
  expect(container.textContent).toContain("Polling for completion…");
  mocks.getJob.mockClear();
}

async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

describe("StartGenerationDialog polling", () => {
  it("shows an error instead of a stuck polling message when a poll fails", async () => {
    await openRunningJob();
    mocks.getJob.mockRejectedValueOnce(new Error("network down"));

    await advance(3000);

    expect(container.textContent).not.toContain("Polling for completion…");
    expect(container.querySelector('[role="alert"]')?.textContent).toContain(
      "Stopped checking for updates",
    );
    await advance(10_000);
    expect(mocks.getJob).toHaveBeenCalledTimes(1);
  });

  it("never overlaps poll requests, so a late response cannot win", async () => {
    await openRunningJob();
    let resolveSlow!: (value: DatasetGenerationJob) => void;
    mocks.getJob.mockReturnValueOnce(
      new Promise<DatasetGenerationJob>((resolve) => (resolveSlow = resolve)),
    );
    mocks.getJob.mockResolvedValue(job("running"));

    await advance(3000);
    expect(mocks.getJob).toHaveBeenCalledTimes(1);
    // A slow request must not let more requests pile up behind it.
    await advance(9000);
    expect(mocks.getJob).toHaveBeenCalledTimes(1);

    await act(async () => resolveSlow(job("completed")));
    expect(container.textContent).toContain("completed");
    expect(container.textContent).not.toContain("Polling for completion…");

    await advance(10_000);
    expect(mocks.getJob).toHaveBeenCalledTimes(1);
    expect(container.textContent).not.toContain("Polling for completion…");
  });
});
