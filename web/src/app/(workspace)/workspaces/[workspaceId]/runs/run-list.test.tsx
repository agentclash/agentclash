import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { Run } from "@/lib/api/types";

import { RunList } from "./run-list";

const { mockPush, mockRunsPage } = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockRunsPage: {
    current: undefined as
      | {
          items: Run[];
          total: number;
          limit: number;
          offset: number;
        }
      | undefined,
  },
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) =>
    React.createElement("a", { href, ...props }, children),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

vi.mock("@/lib/api/swr", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/swr")>();
  return {
    ...actual,
    usePaginatedApiQuery: () => ({
      data: mockRunsPage.current,
      isLoading: false,
      error: null,
    }),
  };
});

vi.mock("@/components/app-shell/workspace-loading", () => ({
  WorkspaceListLoading: () => React.createElement("div", null, "loading"),
}));

vi.mock("@/components/ui/badge", () => ({
  Badge: ({
    children,
    ...props
  }: React.HTMLAttributes<HTMLSpanElement>) =>
    React.createElement("span", props, children),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement>) =>
    React.createElement("button", props, children),
}));

vi.mock("@/components/ui/empty-state", () => ({
  EmptyState: ({ title }: { title: string }) =>
    React.createElement("div", null, title),
}));

vi.mock("@/components/ui/table", () => ({
  Table: ({ children }: { children: React.ReactNode }) =>
    React.createElement("table", null, children),
  TableBody: ({ children }: { children: React.ReactNode }) =>
    React.createElement("tbody", null, children),
  TableCell: ({ children }: { children: React.ReactNode }) =>
    React.createElement("td", null, children),
  TableHead: ({ children }: { children: React.ReactNode }) =>
    React.createElement("th", null, children),
  TableHeader: ({ children }: { children: React.ReactNode }) =>
    React.createElement("thead", null, children),
  TableRow: ({
    children,
    ...props
  }: React.HTMLAttributes<HTMLTableRowElement>) =>
    React.createElement("tr", props, children),
}));

vi.mock("lucide-react", () => {
  const icon = () => React.createElement("span", null);
  return {
    ChevronLeft: icon,
    ChevronRight: icon,
    GitCompare: icon,
    Headphones: icon,
    Play: icon,
  };
});

function renderRunList() {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  act(() => {
    root.render(React.createElement(RunList, { workspaceId: "ws-1" }));
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

describe("RunList", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    mockPush.mockReset();
    mockRunsPage.current = {
      items: [
        {
          id: "run-1",
          workspace_id: "ws-1",
          eval_pack_version_id: "cpv-1",
          official_pack_mode: "full",
          name: "Voice nested metadata run",
          status: "completed",
          execution_mode: "single_agent",
          race_context: false,
          mode: "text-sim",
          voice: {
            mode: "text-sim",
            modality: "voice",
            transport: "text_sim",
          },
          created_at: "2026-05-13T18:00:00Z",
          updated_at: "2026-05-13T18:00:00Z",
          links: {
            self: "/v1/runs/run-1",
            agents: "/v1/runs/run-1/agents",
          },
        },
      ],
      total: 1,
      limit: 20,
      offset: 0,
    };
  });

  it("shows the voice badge when modality is only nested under voice metadata", () => {
    const { container, cleanup } = renderRunList();
    try {
      expect(container.textContent).toContain("Voice nested metadata run");
      expect(container.textContent).toContain("Voice");
      expect(container.textContent).toContain("Text simulation");
      expect(container.textContent).toContain("text_sim");
    } finally {
      cleanup();
    }
  });
});
