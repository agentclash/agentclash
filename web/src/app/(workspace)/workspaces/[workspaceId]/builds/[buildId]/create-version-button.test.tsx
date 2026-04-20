import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CreateVersionButton } from "./create-version-button";

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const {
  mockPush,
  mockGetAccessToken,
  mockCreateApiClient,
  toast,
} = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockGetAccessToken: vi.fn(),
  mockCreateApiClient: vi.fn(),
  toast: Object.assign(vi.fn(), {
    success: vi.fn(),
    error: vi.fn(),
  }),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("sonner", () => ({
  toast,
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: (...args: unknown[]) => mockCreateApiClient(...args),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement>) =>
    React.createElement("button", props, children),
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
        ...(children === undefined ? {} : { children }),
      });
    },
    DialogContent: ({ children }: { children: React.ReactNode }) => {
      const open = React.useContext(DialogOpenContext);
      return open
        ? React.createElement("div", { "data-testid": "dialog" }, children)
        : null;
    },
    DialogHeader: ({ children }: { children: React.ReactNode }) =>
      React.createElement("div", null, children),
    DialogTitle: ({ children }: { children: React.ReactNode }) =>
      React.createElement("h1", null, children),
    DialogDescription: ({ children }: { children: React.ReactNode }) =>
      React.createElement("p", null, children),
  };
});

vi.mock("lucide-react", () => ({
  Loader2: () => React.createElement("span", null, "loader"),
  Plus: () => React.createElement("span", null, "plus"),
  Sparkles: () => React.createElement("span", null, "sparkles"),
}));

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
  });
}

function clickElement(element: Element) {
  element.dispatchEvent(
    new MouseEvent("click", {
      bubbles: true,
      cancelable: true,
    }),
  );
}

function findButton(text: string) {
  const button = Array.from(document.querySelectorAll("button")).find((candidate) =>
    candidate.textContent?.includes(text),
  );
  if (!button) throw new Error(`Button with text ${text} not found`);
  return button;
}

function renderComponent() {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  act(() => {
    root.render(
      React.createElement(CreateVersionButton, {
        buildId: "build-1",
        workspaceId: "ws-1",
      }),
    );
  });

  return {
    cleanup: () => {
      act(() => root.unmount());
      container.remove();
    },
  };
}

describe("CreateVersionButton", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    mockPush.mockReset();
    mockGetAccessToken.mockReset();
    mockCreateApiClient.mockReset();
    toast.success.mockReset();
    toast.error.mockReset();
  });

  it("shows starter templates before creating a version", async () => {
    mockGetAccessToken.mockResolvedValue("token");
    const post = vi.fn().mockResolvedValue({ id: "version-1", version_number: 4 });
    mockCreateApiClient.mockReturnValue({ post });

    const { cleanup } = renderComponent();

    act(() => {
      clickElement(findButton("New Version"));
    });

    expect(document.body.textContent).toContain(
      "Start With a Guided Version Template",
    );
    expect(document.body.textContent).toContain("Research Analyst");
    expect(document.body.textContent).toContain("Blank Starter");

    act(() => {
      clickElement(findButton("Research Analyst"));
    });
    await flushPromises();

    expect(post).toHaveBeenCalledWith(
      "/v1/agent-builds/build-1/versions",
      expect.objectContaining({
        agent_kind: "llm_agent",
        workflow_spec: expect.objectContaining({
          tool_strategy: "prefer_tools_first",
        }),
      }),
    );
    expect(mockPush).toHaveBeenCalledWith(
      "/workspaces/ws-1/builds/build-1/versions/version-1",
    );
    expect(toast.success).toHaveBeenCalledWith("Created version 4");

    cleanup();
  });

  it("still supports creating from the blank starter", async () => {
    mockGetAccessToken.mockResolvedValue("token");
    const post = vi.fn().mockResolvedValue({ id: "version-2", version_number: 1 });
    mockCreateApiClient.mockReturnValue({ post });

    const { cleanup } = renderComponent();

    act(() => {
      clickElement(findButton("New Version"));
    });
    act(() => {
      clickElement(findButton("Blank Starter"));
    });
    await flushPromises();

    expect(post).toHaveBeenCalledWith(
      "/v1/agent-builds/build-1/versions",
      expect.objectContaining({
        policy_spec: {},
        output_schema: {},
      }),
    );

    cleanup();
  });
});
