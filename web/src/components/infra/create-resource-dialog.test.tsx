import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CreateResourceDialog } from "./create-resource-dialog";

const {
  mockRefresh,
  mockGetAccessToken,
  mockCreateApiClient,
  mockMutateMany,
  toast,
} = vi.hoisted(() => ({
  mockRefresh: vi.fn(),
  mockGetAccessToken: vi.fn(),
  mockCreateApiClient: vi.fn(),
  mockMutateMany: vi.fn(),
  toast: Object.assign(vi.fn(), {
    success: vi.fn(),
    error: vi.fn(),
  }),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: mockRefresh }),
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: (...args: unknown[]) => mockCreateApiClient(...args),
}));

vi.mock("@/lib/api/swr", () => ({
  useApiMutator: () => ({ mutateMany: mockMutateMany }),
}));

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
      return open
        ? React.createElement("div", { "data-testid": "dialog-content" }, children)
        : null;
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

function clickElement(element: Element) {
  act(() => {
    element.dispatchEvent(
      new MouseEvent("click", {
        bubbles: true,
        cancelable: true,
      }),
    );
  });
}

function changeInput(element: HTMLInputElement, value: string) {
  const descriptor = Object.getOwnPropertyDescriptor(
    HTMLInputElement.prototype,
    "value",
  );
  act(() => {
    descriptor?.set?.call(element, value);
    element.dispatchEvent(new Event("input", { bubbles: true }));
    element.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
  });
}

async function waitFor(assertion: () => void, attempts = 20) {
  let lastError: unknown;
  for (let index = 0; index < attempts; index += 1) {
    try {
      assertion();
      return;
    } catch (error) {
      lastError = error;
      await flushPromises();
    }
  }
  throw lastError;
}

describe("CreateResourceDialog", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    document.body.innerHTML = "";
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);

    mockRefresh.mockReset();
    mockGetAccessToken.mockReset();
    mockCreateApiClient.mockReset();
    mockMutateMany.mockReset();
    toast.success.mockReset();
    toast.error.mockReset();

    mockGetAccessToken.mockResolvedValue("token");
    mockCreateApiClient.mockReturnValue({
      post: vi.fn().mockResolvedValue({ id: "resource-1" }),
    });
    mockMutateMany.mockResolvedValue(undefined);
  });

  it("invalidates SWR keys instead of refreshing when invalidateKeys are provided", async () => {
    act(() => {
      root.render(
        <CreateResourceDialog
          title="New Resource"
          description="Create a resource."
          endpoint="/v1/resources"
          buttonLabel="Create Resource"
          fields={[{ key: "name", label: "Name", required: true }]}
          invalidateKeys={[["/v1/resources"]]}
        />,
      );
    });

    const trigger = Array.from(document.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("Create Resource"),
    );
    expect(trigger).toBeTruthy();
    clickElement(trigger as HTMLButtonElement);
    await flushPromises();

    const input = document.querySelector("input");
    expect(input).toBeInstanceOf(HTMLInputElement);
    changeInput(input as HTMLInputElement, "Fresh resource");
    await flushPromises();

    const submit = Array.from(document.querySelectorAll("button")).findLast((button) =>
      button.textContent?.includes("Create Resource"),
    );
    expect(submit).toBeTruthy();
    clickElement(submit as HTMLButtonElement);
    await waitFor(() => {
      expect(mockMutateMany).toHaveBeenCalledWith([["/v1/resources"]]);
    });

    expect(mockRefresh).not.toHaveBeenCalled();
  });
});
