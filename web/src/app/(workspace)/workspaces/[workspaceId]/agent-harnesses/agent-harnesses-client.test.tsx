import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AgentHarnessesClient } from "./agent-harnesses-client";

const {
  mockGetAccessToken,
  mockHarnessQuery,
  mockExecutions,
  mockMutate,
  mockPost,
} = vi.hoisted(() => ({
  mockGetAccessToken: vi.fn(),
  mockHarnessQuery: vi.fn(),
  mockExecutions: vi.fn(),
  mockMutate: vi.fn(),
  mockPost: vi.fn(),
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({ post: mockPost }),
}));

vi.mock("@/lib/api/swr", () => ({
  apiQueryKey: (path: string) => [path],
  useApiListQuery: (path: string) => {
    if (path.includes("agent-harness-executions")) {
      return { data: { items: mockExecutions() }, isLoading: false };
    }
    return mockHarnessQuery();
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
  EmptyState: ({
    title,
    description,
  }: {
    title: string;
    description?: string;
  }) => React.createElement("div", null, title, description),
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

vi.mock("./create-agent-harness-dialog", () => ({
  CreateAgentHarnessDialog: () =>
    React.createElement("button", null, "New Harness"),
}));

vi.mock("lucide-react", () => ({
  Activity: () => React.createElement("span", null, "activity"),
  AlertCircle: () => React.createElement("span", null, "alert"),
  Bot: () => React.createElement("span", null, "bot"),
  CheckCircle2: () => React.createElement("span", null, "done"),
  ChevronDown: () => React.createElement("span", null, "expand"),
  Clock3: () => React.createElement("span", null, "clock"),
  Loader2: () => React.createElement("span", null, "loader"),
  MessageSquare: () => React.createElement("span", null, "message"),
  PackageCheck: () => React.createElement("span", null, "package"),
  Send: () => React.createElement("span", null, "send"),
  Settings2: () => React.createElement("span", null, "settings"),
  TerminalSquare: () => React.createElement("span", null, "terminal"),
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

function changeTextarea(value: string) {
  const textarea = document.querySelector("textarea");
  if (!textarea) throw new Error("textarea not found");
  act(() => {
    setNativeValue(textarea, value);
    textarea.dispatchEvent(new InputEvent("input", { bubbles: true, data: value }));
  });
}

function setNativeValue(
  element: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement,
  value: string,
) {
  const valueSetter = Object.getOwnPropertyDescriptor(element, "value")?.set;
  const prototype = Object.getPrototypeOf(element);
  const prototypeValueSetter = Object.getOwnPropertyDescriptor(
    prototype,
    "value",
  )?.set;
  if (prototypeValueSetter && valueSetter !== prototypeValueSetter) {
    prototypeValueSetter.call(element, value);
  } else if (valueSetter) {
    valueSetter.call(element, value);
  } else {
    element.value = value;
  }
}

function changeSelect(value: string) {
  const select = document.querySelector("select");
  if (!select) throw new Error("select not found");
  act(() => {
    select.value = value;
    select.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

function clickButtonByText(text: string) {
  const button = Array.from(document.querySelectorAll("button")).find((item) =>
    item.textContent?.includes(text),
  );
  if (!button) throw new Error(`button ${text} not found`);
  act(() => {
    button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
}

function baseHarness(overrides: Record<string, unknown> = {}) {
  return {
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
    repository_full_name: "acme/repo",
    repository_url: "https://github.com/acme/repo",
    base_branch: "main",
    execution_config: {},
    evaluation_config: {},
    created_at: "2026-05-01T00:00:00Z",
    updated_at: "2026-05-01T00:00:00Z",
    ...overrides,
  };
}

function baseExecution(overrides: Record<string, unknown> = {}) {
  return {
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
    ...overrides,
  };
}

describe("AgentHarnessesClient", () => {
  beforeEach(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
    document.body.innerHTML = "";
    vi.clearAllMocks();
    mockGetAccessToken.mockResolvedValue("token");
    mockPost.mockResolvedValue({});
    mockMutate.mockResolvedValue(undefined);
    mockHarnessQuery.mockReturnValue({
      data: { items: [baseHarness()] },
      isLoading: false,
    });
    mockExecutions.mockReturnValue([baseExecution()]);
  });

  it("renders a chat-first workbench around the selected harness", () => {
    const rendered = renderClient();

    expect(document.body.textContent).toContain("Chat with a coding harness");
    expect(document.body.textContent).toContain("Active harness");
    expect(document.body.textContent).toContain("Setup context");
    expect(document.body.textContent).toContain("acme/repo");
    expect(document.body.textContent).toContain("Base branch");
    expect(document.body.textContent).toContain("agentclash Codex");
    expect(document.body.textContent).not.toContain("Live Activity");

    rendered.cleanup();
  });

  it("preserves prompt text when starting a run fails", async () => {
    mockPost.mockRejectedValueOnce(new Error("network failed"));
    const rendered = renderClient();
    const message = "retry this task";

    changeTextarea(message);
    await act(async () => {
      clickButtonByText("Send");
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(document.body.textContent).toContain("network failed");
    expect(document.querySelector("textarea")?.value).toBe(message);

    rendered.cleanup();
  });

  it("posts a GitHub issue URL as plain follow-up text and clears the prompt", async () => {
    const rendered = renderClient();
    const message = "https://github.com/agentclash/agentclash/issues/610";

    changeTextarea(message);
    await act(async () => {
      clickButtonByText("Send");
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(mockPost).toHaveBeenCalledWith(
      "/v1/workspaces/ws-1/agent-harnesses/harness-1/executions",
      { message },
    );
    expect(mockMutate).toHaveBeenCalledTimes(2);
    expect(document.querySelector("textarea")?.value).toBe("");

    rendered.cleanup();
  });

  it("keeps empty prompts disabled", () => {
    const rendered = renderClient();

    const sendButton = Array.from(document.querySelectorAll("button")).find(
      (button) => button.textContent?.includes("Send"),
    );
    expect(sendButton).toHaveProperty("disabled", true);

    rendered.cleanup();
  });

  it("switches harnesses without leaving the chat workbench", () => {
    mockHarnessQuery.mockReturnValue({
      data: {
        items: [
          baseHarness(),
          baseHarness({
            id: "harness-2",
            name: "repo Claude",
            task_prompt: "Review the migration.",
            codex_template: "agentclash-claude-fullstack",
          }),
        ],
      },
      isLoading: false,
    });
    const rendered = renderClient();

    changeSelect("harness-2");

    expect(document.body.textContent).toContain("repo Claude");
    expect(document.body.textContent).toContain("Review the migration.");

    rendered.cleanup();
  });

  it("shows approachable live progress and summarized latest activity", () => {
    const rendered = renderClient();

    expect(document.body.textContent).toContain("Live activity");
    expect(document.body.textContent).toContain("running");
    expect(document.body.textContent).toContain("Setup");
    expect(document.body.textContent).toContain("Repository");
    expect(document.body.textContent).toContain("Agent work");
    expect(document.body.textContent).toContain("Validation");
    expect(document.body.textContent).toContain("Scoring · Completed");
    expect(document.body.textContent).toContain("Score: 1");
    expect(document.body.textContent).toContain("Passed: 1");

    rendered.cleanup();
  });

  it("expands the latest execution into safe summarized details", () => {
    const rendered = renderClient();

    clickButtonByText("Show summarized details");

    expect(document.body.textContent).toContain("Summarized activity details");
    expect(document.body.textContent).toContain("3 events");
    expect(document.body.textContent).toContain("#1 · worker");
    expect(document.body.textContent).toContain("Repository · Clone · Started");
    expect(document.body.textContent).toContain(
      "Command: git clone https://github.com/acme/repo",
    );

    rendered.cleanup();
  });

  it("does not render full diff payloads in the summarized timeline", () => {
    mockExecutions.mockReturnValue([
      baseExecution({
        events: [
          {
            id: "event-diff",
            agent_harness_execution_id: "execution-1",
            sequence_number: 1,
            event_type: "artifact.git_diff",
            actor_type: "worker",
            occurred_at: "2026-05-01T00:01:05Z",
            payload: {
              diff: "SECRET DIFF BODY",
              changed_files: "M web/app.tsx",
            },
          },
        ],
      }),
    ]);
    const rendered = renderClient();

    clickButtonByText("Show summarized details");

    expect(document.body.textContent).toContain("Changed Files: M web/app.tsx");
    expect(document.body.textContent).not.toContain("SECRET DIFF BODY");

    rendered.cleanup();
  });

  it("does not render raw runner output messages in the summarized timeline", () => {
    mockExecutions.mockReturnValue([
      baseExecution({
        events: [
          {
            id: "event-output",
            agent_harness_execution_id: "execution-1",
            sequence_number: 1,
            event_type: "codex.exec.output",
            actor_type: "codex",
            occurred_at: "2026-05-01T00:01:05Z",
            payload: {
              message: "RAW RUNNER STDOUT",
              raw: "RAW JSON LINE",
              type: "agent_message",
            },
          },
        ],
      }),
    ]);
    const rendered = renderClient();

    clickButtonByText("Show summarized details");

    expect(document.body.textContent).toContain("Type: agent_message");
    expect(document.body.textContent).not.toContain("RAW RUNNER STDOUT");
    expect(document.body.textContent).not.toContain("RAW JSON LINE");

    rendered.cleanup();
  });

  it.each(["queued", "provisioning", "running", "scoring", "completed", "failed"])(
    "renders %s execution status",
    (status) => {
      mockExecutions.mockReturnValue([baseExecution({ status })]);
      const rendered = renderClient();

      expect(document.body.textContent).toContain(status);

      rendered.cleanup();
    },
  );

  it("shows a waiting state when an execution has no events", () => {
    mockExecutions.mockReturnValue([baseExecution({ events: [] })]);
    const rendered = renderClient();

    expect(document.body.textContent).toContain(
      "Waiting for the first execution event...",
    );

    rendered.cleanup();
  });

  it("renders actionable failure copy from error_message", () => {
    mockExecutions.mockReturnValue([
      baseExecution({
        status: "failed",
        error_message: "Codex exited with status 1",
      }),
    ]);
    const rendered = renderClient();

    expect(document.body.textContent).toContain("This run needs attention");
    expect(document.body.textContent).toContain("Codex exited with status 1");

    rendered.cleanup();
  });

  it("falls back to failed event payloads for actionable failure copy", () => {
    mockExecutions.mockReturnValue([
      baseExecution({
        status: "failed",
        events: [
          {
            id: "event-failed",
            agent_harness_execution_id: "execution-1",
            sequence_number: 1,
            event_type: "codex.exec.failed",
            actor_type: "worker",
            occurred_at: "2026-05-01T00:01:05Z",
            payload: {
              error: "command timed out",
            },
          },
        ],
      }),
    ]);
    const rendered = renderClient();

    expect(document.body.textContent).toContain("command timed out");

    rendered.cleanup();
  });

  it("classifies setup failures separately from agent failures", () => {
    mockExecutions.mockReturnValue([
      baseExecution({
        status: "failed",
        failure_stage: "setup",
        error_message: "Setup command failed",
        events: [
          {
            id: "event-setup-failed",
            agent_harness_execution_id: "execution-1",
            sequence_number: 1,
            event_type: "setup.command.exec.failed",
            actor_type: "worker",
            occurred_at: "2026-05-01T00:01:05Z",
            payload: {
              command: ["bash", "-lc", "go mod download"],
              working_directory: "/workspace",
              exit_code: 1,
            },
          },
        ],
      }),
    ]);
    const rendered = renderClient();

    expect(document.body.textContent).toContain("Setup failed");
    expect(document.body.textContent).toContain("Setup command failed");
    expect(document.body.textContent).toContain("Setupfailed");
    expect(document.body.textContent).toContain("Agent workwaiting");

    rendered.cleanup();
  });

  it("keeps loading, error, and empty states readable", () => {
    mockHarnessQuery.mockReturnValueOnce({ isLoading: true });
    let rendered = renderClient();
    expect(document.body.textContent).toContain("loading");
    rendered.cleanup();

    mockHarnessQuery.mockReturnValueOnce({ error: new Error("boom") });
    rendered = renderClient();
    expect(document.body.textContent).toContain("Failed to load agent harnesses.");
    rendered.cleanup();

    mockHarnessQuery.mockReturnValueOnce({
      data: { items: [] },
      isLoading: false,
    });
    rendered = renderClient();
    expect(document.body.textContent).toContain("No agent harnesses yet");
    rendered.cleanup();
  });
});
