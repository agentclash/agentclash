import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CreateAgentHarnessDialog } from "./create-agent-harness-dialog";

const { mockGetAccessToken, mockCreateApiClient, mockMutate, toast } =
  vi.hoisted(() => ({
    mockGetAccessToken: vi.fn(),
    mockCreateApiClient: vi.fn(),
    mockMutate: vi.fn(),
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
    useApiMutator: () => ({ mutate: mockMutate }),
  };
});

vi.mock("sonner", () => ({
  toast,
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

vi.mock("@/components/ui/json-field", () => ({
  JsonField: ({
    label,
    value,
    onChange,
    error,
  }: {
    label: string;
    value: string;
    onChange: (value: string) => void;
    error?: string;
  }) =>
    React.createElement(
      "label",
      null,
      label,
      React.createElement("textarea", {
        "aria-label": label,
        value,
        onChange: (event: React.ChangeEvent<HTMLTextAreaElement>) =>
          onChange(event.target.value),
      }),
      error ? React.createElement("span", null, error) : null,
    ),
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

function changeSelect(value: string) {
  const select = document.querySelector("select");
  if (!select) throw new Error("select not found");
  act(() => {
    select.value = value;
    select.dispatchEvent(new Event("change", { bubbles: true }));
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
  });

  it("posts Codex/E2B harness payload with API-key secret auth", async () => {
    const post = vi.fn().mockResolvedValue({ id: "harness-1" });
    mockCreateApiClient.mockReturnValue({ post });
    const rendered = renderDialog();

    clickButton("New Harness");
    changeInput(0, "Codex long runner");
    changeSelect("api_key_secret");
    changeTextarea(0, "Implement the requested feature and run tests.");
    changeInput(6, "OPENAI_API_KEY");
    clickButton("Create Harness");
    await flushPromises();

    expect(post).toHaveBeenCalledWith(
      "/v1/workspaces/ws-1/agent-harnesses",
      expect.objectContaining({
        name: "Codex long runner",
        task_prompt: "Implement the requested feature and run tests.",
        codex_template: "codex",
        auth_mode: "api_key_secret",
        openai_api_key_secret_name: "OPENAI_API_KEY",
        evaluation_config: expect.objectContaining({
          validators: expect.any(Array),
          llm_judges: expect.any(Array),
        }),
      }),
    );
    expect(mockMutate).toHaveBeenCalled();
    rendered.cleanup();
  });

  it("blocks API-key auth without an OpenAI secret", async () => {
    const post = vi.fn();
    mockCreateApiClient.mockReturnValue({ post });
    const rendered = renderDialog();

    clickButton("New Harness");
    changeInput(0, "Codex long runner");
    changeSelect("api_key_secret");
    changeTextarea(0, "Implement the requested feature.");
    const submit = findButton("Create Harness");
    await flushPromises();

    expect(submit.hasAttribute("disabled")).toBe(true);
    expect(post).not.toHaveBeenCalled();
    rendered.cleanup();
  });
});
