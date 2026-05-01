import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AgentHarnessesClient } from "./agent-harnesses-client";

const {
  mockGetAccessToken,
  mockHarnesses,
  mockExecutions,
  mockMutate,
} = vi.hoisted(() => ({
  mockGetAccessToken: vi.fn(),
  mockHarnesses: vi.fn(),
  mockExecutions: vi.fn(),
  mockMutate: vi.fn(),
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({ post: vi.fn() }),
}));

vi.mock("@/lib/api/swr", () => ({
  useApiListQuery: (path: string) => {
    if (path.includes("agent-harness-executions")) {
      return { data: { items: mockExecutions() }, isLoading: false };
    }
    return { data: { items: mockHarnesses() }, isLoading: false };
  },
  useApiMutator: () => ({ mutate: mockMutate }),
}));

vi.mock("@/components/app-shell/workspace-loading", () => ({
  WorkspaceListLoading: () => React.createElement("div", null, "loading"),
}));

vi.mock("@/components/ui/badge", () => ({
  Badge: ({ children }: { children: React.ReactNode }) =>
    React.createElement("span", null, children),
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

vi.mock("@/components/ui/input", () => ({
  Input: (props: React.InputHTMLAttributes<HTMLInputElement>) =>
    React.createElement("input", props),
}));

vi.mock("@/components/ui/page-header", () => ({
  PageHeader: ({
    title,
    actions,
  }: {
    title: string;
    actions?: React.ReactNode;
  }) => React.createElement("header", null, title, actions),
}));

vi.mock("@/components/ui/table", () => ({
  Table: ({ children }: { children: React.ReactNode }) =>
    React.createElement("table", null, children),
  TableBody: ({ children }: { children: React.ReactNode }) =>
    React.createElement("tbody", null, children),
  TableCell: ({
    children,
    ...props
  }: React.TdHTMLAttributes<HTMLTableCellElement>) =>
    React.createElement("td", props, children),
  TableHead: ({
    children,
    ...props
  }: React.ThHTMLAttributes<HTMLTableCellElement>) =>
    React.createElement("th", props, children),
  TableHeader: ({ children }: { children: React.ReactNode }) =>
    React.createElement("thead", null, children),
  TableRow: ({
    children,
    ...props
  }: React.HTMLAttributes<HTMLTableRowElement>) =>
    React.createElement("tr", props, children),
}));

vi.mock("./create-agent-harness-dialog", () => ({
  CreateAgentHarnessDialog: () =>
    React.createElement("button", null, "New Harness"),
}));

vi.mock("lucide-react", () => ({
  Activity: () => React.createElement("span", null, "activity"),
  ChevronDown: () => React.createElement("span", null, "expand"),
  Loader2: () => React.createElement("span", null, "loader"),
  MessageSquare: () => React.createElement("span", null, "message"),
  PackageCheck: () => React.createElement("span", null, "package"),
  Play: () => React.createElement("span", null, "play"),
  Send: () => React.createElement("span", null, "send"),
}));

function renderClient() {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);
  act(() => {
    root.render(React.createElement(AgentHarnessesClient, { workspaceId: "ws-1" }));
  });
  return {
    cleanup: () => {
      act(() => root.unmount());
      container.remove();
    },
  };
}

function clickButtonByLabel(label: string) {
  const button = document.querySelector(`button[aria-label="${label}"]`);
  if (!button) throw new Error(`button ${label} not found`);
  act(() => {
    button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
}

describe("AgentHarnessesClient", () => {
  beforeEach(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
    document.body.innerHTML = "";
    vi.clearAllMocks();
    mockGetAccessToken.mockResolvedValue("token");
    mockHarnesses.mockReturnValue([
      {
        id: "harness-1",
        workspace_id: "ws-1",
        organization_id: "org-1",
        name: "agentclash Codex",
        slug: "agentclash-codex",
        description: "",
        status: "draft",
        harness_kind: "codex_e2b",
        task_prompt: "Patch the failing test.",
        codex_template: "codex",
        auth_mode: "api_key_secret",
        execution_config: {},
        evaluation_config: {},
        created_at: "2026-05-01T00:00:00Z",
        updated_at: "2026-05-01T00:00:00Z",
      },
    ]);
    mockExecutions.mockReturnValue([
      {
        id: "execution-1",
        workspace_id: "ws-1",
        organization_id: "org-1",
        agent_harness_id: "harness-1",
        status: "running",
        harness_snapshot: {},
        execution_config_snapshot: {},
        evaluation_config_snapshot: {},
        created_at: "2026-05-01T00:01:00Z",
        updated_at: "2026-05-01T00:02:00Z",
        events: [
          {
            id: "event-1",
            agent_harness_execution_id: "execution-1",
            sequence_number: 1,
            event_type: "repository.clone.started",
            actor_type: "worker",
            occurred_at: "2026-05-01T00:01:05Z",
            payload: {
              command: ["git", "clone", "https://github.com/acme/repo"],
              working_directory: "",
            },
          },
          {
            id: "event-2",
            agent_harness_execution_id: "execution-1",
            sequence_number: 2,
            event_type: "codex.exec.started",
            actor_type: "worker",
            occurred_at: "2026-05-01T00:01:30Z",
            payload: {
              command: ["codex", "exec", "--full-auto"],
              working_directory: "/workspace",
            },
          },
          {
            id: "event-3",
            agent_harness_execution_id: "execution-1",
            sequence_number: 3,
            event_type: "scoring.completed",
            actor_type: "worker",
            occurred_at: "2026-05-01T00:01:40Z",
            payload: {
              score: 1,
              passed: 1,
              failed: 0,
              skipped: 1,
            },
          },
        ],
      },
    ]);
  });

  it("shows live activity from the latest execution event", () => {
    const rendered = renderClient();

    expect(document.body.textContent).toContain("Live Activity");
    expect(document.body.textContent).toContain("running");
    expect(document.body.textContent).toContain("Scoring · Completed");
    expect(document.body.textContent).toContain("Score: 1");
    expect(document.body.textContent).toContain("Passed: 1");

    rendered.cleanup();
  });

  it("expands the latest execution into a readable event timeline", () => {
    const rendered = renderClient();

    clickButtonByLabel("Show activity for agentclash Codex");

    expect(document.body.textContent).toContain("Execution timeline");
    expect(document.body.textContent).toContain("3 events");
    expect(document.body.textContent).toContain("#1 · worker");
    expect(document.body.textContent).toContain("Repository · Clone · Started");
    expect(document.body.textContent).toContain(
      "Command: git clone https://github.com/acme/repo",
    );

    rendered.cleanup();
  });
});
