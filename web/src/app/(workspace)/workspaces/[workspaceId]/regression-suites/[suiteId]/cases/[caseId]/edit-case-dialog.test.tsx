import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { RegressionCase, RegressionCaseStatus } from "@/lib/api/types";

import { EditCaseDialog } from "./edit-case-dialog";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: vi.fn(), push: vi.fn() }),
}));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: vi.fn(async () => "token") }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({ patch: vi.fn() }),
}));
vi.mock("@/components/ui/dialog", () => {
  const passthrough = ({ children }: { children: React.ReactNode }) =>
    React.createElement("div", null, children);
  return {
    Dialog: passthrough,
    DialogTrigger: () => null,
    DialogContent: passthrough,
    DialogDescription: passthrough,
    DialogFooter: passthrough,
    DialogHeader: passthrough,
    DialogTitle: passthrough,
  };
});
vi.mock("@/components/ui/select", () => ({
  Select: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="select">{children}</div>
  ),
  SelectContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SelectItem: ({
    value,
    children,
  }: {
    value: string;
    children: React.ReactNode;
  }) => <div data-option={value}>{children}</div>,
  SelectTrigger: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SelectValue: () => null,
}));

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
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

async function statusOptionsFor(status: RegressionCaseStatus) {
  const regressionCase = {
    id: "case-1",
    title: "Refund case",
    description: "",
    status,
    severity: "warning",
  } as RegressionCase;
  await act(async () =>
    root.render(
      <EditCaseDialog workspaceId="ws-1" regressionCase={regressionCase} />,
    ),
  );
  // The first select is the status select; the second is severity.
  const statusSelect = container.querySelectorAll('[data-testid="select"]')[0];
  return [...statusSelect.querySelectorAll("[data-option]")].map((option) =>
    option.getAttribute("data-option"),
  );
}

describe("EditCaseDialog status options", () => {
  it.each<[RegressionCaseStatus, RegressionCaseStatus[]]>([
    ["proposed", ["proposed", "active", "archived", "rejected"]],
    ["active", ["active", "muted", "archived"]],
    ["muted", ["muted", "active", "archived"]],
    ["archived", ["archived"]],
    ["rejected", ["rejected"]],
  ])(
    "offers only the current status and its legal next statuses for %s",
    async (status, expected) => {
      expect(await statusOptionsFor(status)).toEqual(expected);
    },
  );

  it("shows the final-state note only for a terminal case", async () => {
    await statusOptionsFor("archived");
    expect(container.textContent).toContain("these states are final");

    await statusOptionsFor("active");
    expect(container.textContent).not.toContain("these states are final");
  });
});
