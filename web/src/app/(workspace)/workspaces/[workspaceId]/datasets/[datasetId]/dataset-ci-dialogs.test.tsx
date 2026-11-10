import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type {
  DatasetBaseline,
  DatasetVersion,
  Run,
} from "@/lib/api/types";
import { DatasetTestDialog } from "./dataset-ci-dialogs";

const mocks = vi.hoisted(() => ({
  getAccessToken: vi.fn(async () => "token"),
  get: vi.fn(),
  startDatasetEval: vi.fn(),
  evaluateDatasetGate: vi.fn(),
  waitForRunCompletion: vi.fn(),
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
  createApiClient: () => ({ get: mocks.get }),
}));
vi.mock("@/lib/api/datasets", () => ({
  createDatasetBaseline: vi.fn(),
  evaluateDatasetGate: (...args: unknown[]) => mocks.evaluateDatasetGate(...args),
  startDatasetEval: (...args: unknown[]) => mocks.startDatasetEval(...args),
  syncDatasetRegressionSuite: vi.fn(),
}));
vi.mock("@/lib/datasets/wait-for-run", () => ({
  waitForRunCompletion: (...args: unknown[]) =>
    mocks.waitForRunCompletion(...args),
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
        React.createElement("button", {
          "data-testid": "dialog-dismiss",
          onClick: () => onOpenChange(false),
        }),
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

const versions = [{ id: "dv-1", version_number: 1 }] as DatasetVersion[];
const baselines = [{ id: "bl-1", label: "Baseline" }] as DatasetBaseline[];

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  mocks.get.mockImplementation(async (path: string) => {
    if (path.endsWith("/challenge-packs")) {
      return {
        items: [
          {
            id: "pack-1",
            name: "Pack",
            versions: [
              { id: "pv-1", version_number: 1, lifecycle_status: "runnable" },
            ],
          },
        ],
      };
    }
    return { items: [{ id: "dep-1", name: "Agent", status: "active" }] };
  });
  mocks.startDatasetEval.mockResolvedValue({
    run: { id: "run-1", status: "queued" },
  });
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

async function flush() {
  for (let i = 0; i < 5; i += 1) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

function button(label: string) {
  return [...container.querySelectorAll("button")].find(
    (b) => b.textContent === label,
  );
}

function fieldFor(label: string): HTMLInputElement | HTMLSelectElement {
  const labelEl = [...container.querySelectorAll("label")].find((l) =>
    l.textContent?.startsWith(label),
  )!;
  return labelEl.parentElement!.querySelector("input, select")!;
}

function setValue(el: HTMLInputElement | HTMLSelectElement, value: string) {
  const proto =
    el instanceof HTMLSelectElement
      ? HTMLSelectElement.prototype
      : HTMLInputElement.prototype;
  Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
  el.dispatchEvent(
    new Event(el instanceof HTMLSelectElement ? "change" : "input", {
      bubbles: true,
    }),
  );
}

async function openAndFill() {
  await act(async () =>
    root.render(
      <DatasetTestDialog
        workspaceId="ws-1"
        datasetId="ds-1"
        versions={versions}
        baselines={baselines}
      />,
    ),
  );
  await act(async () =>
    container
      .querySelector<HTMLButtonElement>('[data-testid="dialog-trigger"]')!
      .click(),
  );
  await flush();
  await act(async () => setValue(fieldFor("Challenge pack"), "pack-1"));
  await act(async () => setValue(fieldFor("Challenge key"), "case-1"));
  const deployment = [
    ...container.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'),
  ].find((box) => box.parentElement?.textContent === "Agent")!;
  await act(async () => deployment.click());
}

describe("DatasetTestDialog", () => {
  it("leaves the running state when the eval run ends without completing", async () => {
    mocks.waitForRunCompletion.mockResolvedValue({
      id: "run-1",
      status: "cancelled",
    } as Run);
    await openAndFill();

    await act(async () => button("Run test")!.click());
    await flush();

    expect(button("Running eval…")).toBeUndefined();
    expect(button("Run test")).toBeDefined();
    expect(button("Run test")!.disabled).toBe(false);
    expect(container.textContent).toContain("Status: cancelled");
    expect(mocks.evaluateDatasetGate).not.toHaveBeenCalled();
  });

  it("aborts the wait on close so a late result cannot reach a reopened dialog", async () => {
    let signal!: AbortSignal;
    mocks.waitForRunCompletion.mockImplementation(
      (_api: unknown, _id: string, opts: { signal: AbortSignal }) => {
        signal = opts.signal;
        // Never settles on its own, like a long eval run.
        return new Promise(() => {});
      },
    );
    await openAndFill();
    await act(async () => button("Run test")!.click());
    await flush();
    expect(button("Running eval…")).toBeDefined();
    expect(signal.aborted).toBe(false);

    await act(async () =>
      container
        .querySelector<HTMLButtonElement>('[data-testid="dialog-dismiss"]')!
        .click(),
    );
    expect(signal.aborted).toBe(true);

    await act(async () =>
      container
        .querySelector<HTMLButtonElement>('[data-testid="dialog-trigger"]')!
        .click(),
    );
    await flush();
    expect(button("Run test")).toBeDefined();
    expect(container.textContent).not.toContain("Eval run run-1");
    expect(mocks.evaluateDatasetGate).not.toHaveBeenCalled();
  });
});
