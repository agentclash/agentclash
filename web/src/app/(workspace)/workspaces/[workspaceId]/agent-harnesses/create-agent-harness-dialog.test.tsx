import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CreateAgentHarnessDialog } from "./create-agent-harness-dialog";

const {
  mockGetAccessToken,
  mockCreateApiClient,
  mockMutate,
  mockSecrets,
  mockInstallations,
  mockRepositories,
  toast,
} =
  vi.hoisted(() => ({
    mockGetAccessToken: vi.fn(),
    mockCreateApiClient: vi.fn(),
    mockMutate: vi.fn(),
    mockSecrets: vi.fn(),
    mockInstallations: vi.fn(),
    mockRepositories: vi.fn(),
    toast: Object.assign(vi.fn(), {
      success: vi.fn(),
      error: vi.fn(),
    }),
  }));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: (...args: unknown[]) => mockCreateApiClient(...args),
}));

vi.mock("@/lib/api/swr", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/swr")>();
  return {
    ...actual,
    useApiListQuery: (path: string) => {
      if (path.includes("/github/installations")) {
        return { data: { items: mockInstallations() }, isLoading: false, error: null };
      }
      if (path.includes("/github/repositories")) {
        return { data: { items: mockRepositories() }, isLoading: false, error: null };
      }
      return {
        data: { items: mockSecrets() },
        isLoading: false,
        error: null,
      };
    },
    useApiMutator: () => ({ mutate: mockMutate }),
  };
});

vi.mock("sonner", () => ({
  toast,
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) =>
    React.createElement("a", { href, ...props }, children),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement>) =>
    React.createElement("button", props, children),
}));

vi.mock("@/components/ui/input", () => ({
  Input: (props: React.InputHTMLAttributes<HTMLInputElement>) =>
    React.createElement("input", props),
}));

vi.mock("@/components/ui/dialog", async () => {
  const React = await import("react");
  const DialogOpenContext = React.createContext(false);
  const DialogToggleContext = React.createContext<(open: boolean) => void>(
    () => {},
  );

  return {
    Dialog: ({
      open,
      onOpenChange,
      children,
    }: {
      open: boolean;
      onOpenChange: (open: boolean) => void;
      children: React.ReactNode;
    }) =>
      React.createElement(
        DialogOpenContext.Provider,
        { value: open },
        React.createElement(
          DialogToggleContext.Provider,
          { value: onOpenChange },
          children,
        ),
      ),
    DialogTrigger: ({
      render,
      children,
    }: {
      render?: React.ReactElement;
      children?: React.ReactNode;
    }) => {
      const setOpen = React.useContext(DialogToggleContext);
      const element = render ?? React.createElement("button");
      return React.cloneElement(element, {
        onClick: () => setOpen(true),
        children,
      });
    },
    DialogContent: ({ children }: { children: React.ReactNode }) => {
      const open = React.useContext(DialogOpenContext);
      return open ? React.createElement("div", null, children) : null;
    },
    DialogDescription: ({ children }: { children: React.ReactNode }) =>
      React.createElement("p", null, children),
    DialogFooter: ({ children }: { children: React.ReactNode }) =>
      React.createElement("div", null, children),
    DialogHeader: ({ children }: { children: React.ReactNode }) =>
      React.createElement("div", null, children),
    DialogTitle: ({ children }: { children: React.ReactNode }) =>
      React.createElement("h1", null, children),
  };
});

vi.mock("lucide-react", () => ({
  GitBranch: () => React.createElement("span", null, "branch"),
  Github: () => React.createElement("span", null, "github"),
  Loader2: () => React.createElement("span", null, "loader"),
  Plus: () => React.createElement("span", null, "plus"),
}));

function renderDialog() {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);
  act(() => {
    root.render(
      React.createElement(CreateAgentHarnessDialog, { workspaceId: "ws-1" }),
    );
  });
  return {
    cleanup: () => {
      act(() => root.unmount());
      container.remove();
    },
  };
}

function clickButton(text: string) {
  const button = findButton(text);
  act(() => {
    button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
}

function findButton(text: string) {
  const button = Array.from(document.querySelectorAll("button")).find((item) =>
    item.textContent?.includes(text),
  );
  if (!button) throw new Error(`button ${text} not found`);
  return button;
}

function changeInput(index: number, value: string) {
  const input = document.querySelectorAll("input")[index];
  const descriptor = Object.getOwnPropertyDescriptor(
    HTMLInputElement.prototype,
    "value",
  );
  act(() => {
    descriptor?.set?.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

function changeTextarea(index: number, value: string) {
  const textarea = document.querySelectorAll("textarea")[index];
  const descriptor = Object.getOwnPropertyDescriptor(
    HTMLTextAreaElement.prototype,
    "value",
  );
  act(() => {
    descriptor?.set?.call(textarea, value);
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
    textarea.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

function changeSelect(index: number, value: string) {
  const select = document.querySelectorAll("select")[index];
  const descriptor = Object.getOwnPropertyDescriptor(
    HTMLSelectElement.prototype,
    "value",
  );
  act(() => {
    descriptor?.set?.call(select, value);
    select.dispatchEvent(new Event("input", { bubbles: true }));
    select.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
  });
}

describe("CreateAgentHarnessDialog", () => {
  beforeEach(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
    document.body.innerHTML = "";
    vi.clearAllMocks();
    mockGetAccessToken.mockResolvedValue("token");
    mockSecrets.mockReturnValue([
      {
        key: "OPENAI_API_KEY",
        created_at: "2026-05-01T00:00:00Z",
        updated_at: "2026-05-01T00:00:00Z",
      },
    ]);
    mockInstallations.mockReturnValue([]);
    mockRepositories.mockReturnValue([]);
  });

  it("posts Codex/E2B harness payload using the inferred OpenAI secret", async () => {
    mockInstallations.mockReturnValue([
      {
        id: "install-1",
        github_installation_id: 42,
        github_account_id: 9,
        github_account_login: "acme",
        github_account_type: "Organization",
        repository_selection: "selected",
        status: "active",
        updated_at: "2026-05-01T00:00:00Z",
      },
    ]);
    mockRepositories.mockReturnValue([
      {
        id: "repo-1",
        github_installation_id: 42,
        github_repository_id: 100,
        full_name: "acme/agent-app",
        owner_login: "acme",
        name: "agent-app",
        private: true,
        default_branch: "main",
        html_url: "https://github.com/acme/agent-app",
        clone_url: "https://github.com/acme/agent-app.git",
        permissions: {},
        last_synced_at: "2026-05-01T00:00:00Z",
      },
    ]);
    const post = vi.fn().mockResolvedValue({ id: "harness-1" });
    mockCreateApiClient.mockReturnValue({ post });
    const rendered = renderDialog();

    clickButton("New Harness");
    changeSelect(0, "100");
    changeInput(0, "main");
    changeTextarea(0, "Implement the requested feature and run tests.");
    clickButton("Create Harness");
    await flushPromises();

    expect(post).toHaveBeenCalledWith(
      "/v1/workspaces/ws-1/agent-harnesses",
      expect.objectContaining({
        name: "acme/agent-app Codex",
        task_prompt: "Implement the requested feature and run tests.",
        codex_template: "codex",
        auth_mode: "api_key_secret",
        openai_api_key_secret_name: "OPENAI_API_KEY",
        repository_provider: "github",
        github_repository_id: 100,
        github_installation_id: 42,
        evaluation_config: expect.objectContaining({
          validators: expect.any(Array),
          llm_judges: expect.any(Array),
        }),
      }),
    );
    expect(mockMutate).toHaveBeenCalled();
    rendered.cleanup();
  });

  it("falls back to the task prompt for names when the repo URL has no owner and repo path", async () => {
    const post = vi.fn().mockResolvedValue({ id: "harness-1" });
    mockCreateApiClient.mockReturnValue({ post });
    const rendered = renderDialog();

    clickButton("New Harness");
    clickButton("URL");
    changeInput(0, "https://github.com");
    changeTextarea(0, "Implement the requested feature and run tests.");
    clickButton("Create Harness");
    await flushPromises();

    expect(post).toHaveBeenCalledWith(
      "/v1/workspaces/ws-1/agent-harnesses",
      expect.objectContaining({
        name: "Implement the requested feature Codex",
      }),
    );
    rendered.cleanup();
  });

  it("shows the GitHub connect empty state when no installation is cached", () => {
    const post = vi.fn();
    mockCreateApiClient.mockReturnValue({ post });
    const rendered = renderDialog();

    clickButton("New Harness");

    expect(document.body.textContent).toContain("Connect GitHub first");
    expect(post).not.toHaveBeenCalled();
    rendered.cleanup();
  });

  it("disables creation when no OpenAI secret is available", async () => {
    mockSecrets.mockReturnValue([]);
    const post = vi.fn();
    mockCreateApiClient.mockReturnValue({ post });
    const rendered = renderDialog();

    clickButton("New Harness");
    clickButton("URL");
    changeInput(0, "https://github.com/acme/agent-app");
    changeTextarea(0, "Implement the requested feature.");
    const submit = findButton("Create Harness");
    await flushPromises();

    expect(submit.hasAttribute("disabled")).toBe(true);
    expect(post).not.toHaveBeenCalled();
    rendered.cleanup();
  });
});
