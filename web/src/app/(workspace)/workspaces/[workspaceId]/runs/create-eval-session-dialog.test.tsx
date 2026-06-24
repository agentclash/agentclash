import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { workspaceMutationKeys, workspaceResourceKeys } from "@/lib/workspace-resource";

import { CreateEvalSessionDialog } from "./create-eval-session-dialog";

const {
  mockPush,
  mockRefresh,
  mockGetAccessToken,
  mockCreateApiClient,
  mockMutate,
  mockMutateMany,
  toast,
} = vi.hoisted(() => {
  return {
    mockPush: vi.fn(),
    mockRefresh: vi.fn(),
    mockGetAccessToken: vi.fn(),
    mockCreateApiClient: vi.fn(),
    mockMutate: vi.fn(),
    mockMutateMany: vi.fn(),
    toast: Object.assign(vi.fn(), {
      success: vi.fn(),
      error: vi.fn(),
    }),
  };
});

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
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

vi.mock("@/lib/api/swr", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/swr")>();
  return {
    ...actual,
    useApiMutator: () => ({
      mutate: mockMutate,
      mutateMany: mockMutateMany,
    }),
  };
});

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
  Sigma: () => React.createElement("span", null, "sigma"),
}));

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
  });
}

async function waitFor(assertion: () => void, attempts = 30) {
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

function changeSelect(element: HTMLSelectElement, value: string) {
  act(() => {
    element.value = value;
    element.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

function findButton(text: string) {
  const button = Array.from(document.querySelectorAll("button")).find((candidate) =>
    candidate.textContent?.includes(text),
  );
  if (!(button instanceof HTMLButtonElement)) {
    throw new Error(`Button with text ${text} not found`);
  }
  return button;
}

function findSelectForLabel(text: string) {
  const label = Array.from(document.querySelectorAll("label")).find((candidate) =>
    candidate.textContent?.includes(text),
  );
  const select = label?.parentElement?.querySelector("select");
  if (!(select instanceof HTMLSelectElement)) {
    throw new Error(`Select for ${text} not found`);
  }
  return select;
}

function findCheckboxByLabel(text: string) {
  const label = Array.from(document.querySelectorAll("label")).find((candidate) =>
    candidate.textContent?.includes(text),
  );
  const checkbox = label?.querySelector('input[type="checkbox"]');
  if (!(checkbox instanceof HTMLInputElement)) {
    throw new Error(`Checkbox for ${text} not found`);
  }
  return checkbox;
}

function renderDialog() {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  act(() => {
    root.render(
      React.createElement(CreateEvalSessionDialog, { workspaceId: "ws-1" }),
    );
  });

  return {
    cleanup: () => {
      act(() => {
        root.unmount();
      });
      container.remove();
    },
  };
}

function deferredPromise<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((innerResolve, innerReject) => {
    resolve = innerResolve;
    reject = innerReject;
  });
  return { promise, resolve, reject };
}

function buildApiMock(
  status = 201,
  options?: {
    deployments?: Array<Record<string, unknown>>;
    responseData?: Record<string, unknown>;
  },
) {
  const postWithMeta = vi.fn().mockResolvedValue({
    status,
    data:
      status === 422
        ? (options?.responseData ?? {
            errors: [
              {
                field: "eval_session.repetitions",
                code: "eval_session.repetitions.invalid",
                message: "repetitions must be an integer between 1 and 100",
              },
            ],
          })
        : {
            eval_session: {
              id: "session-1",
            },
            run_ids: ["run-1"],
          },
  });

  const get = vi.fn(async (url: string) => {
    if (url === "/v1/workspaces/ws-1/challenge-packs") {
      return {
        items: [
          {
            id: "pack-1",
            name: "Support Pack",
            versions: [
              {
                id: "version-1",
                version_number: 1,
                lifecycle_status: "runnable",
              },
            ],
          },
        ],
      };
    }

    if (url === "/v1/workspaces/ws-1/agent-deployments") {
      return {
        items:
          options?.deployments ??
          [
            {
              id: "deploy-1",
              name: "Primary Agent",
              status: "active",
              current_build_version_id: "build-version-1",
            },
          ],
      };
    }

    if (
      url === "/v1/workspaces/ws-1/challenge-pack-versions/version-1/input-sets"
    ) {
      return {
        items: [],
      };
    }

    throw new Error(`Unexpected GET ${url}`);
  });

  return { get, postWithMeta };
}

beforeEach(() => {
  // These component tests drive React through manual DOM events rather than RTL.
  // Mark the environment explicitly so React's act() warnings stay actionable.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
  document.body.innerHTML = "";
  mockPush.mockReset();
  mockRefresh.mockReset();
  mockGetAccessToken.mockReset();
  mockGetAccessToken.mockResolvedValue("token");
  mockCreateApiClient.mockReset();
  mockMutate.mockReset();
  mockMutateMany.mockReset();
  toast.success.mockReset();
  toast.error.mockReset();
  mockMutate.mockResolvedValue(undefined);
  mockMutateMany.mockResolvedValue(undefined);
});

describe("CreateEvalSessionDialog", () => {
  it("submits a repeated-eval request using deployment ids", async () => {
    const api = buildApiMock();
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Eval Session"));
      await flushPromises();

      changeSelect(findSelectForLabel("Challenge Pack"), "pack-1");
      await flushPromises();

      clickElement(findCheckboxByLabel("Primary Agent"));
      await flushPromises();

      clickElement(findButton("Create Eval Session"));

      await waitFor(() => {
        expect(api.postWithMeta).toHaveBeenCalledTimes(1);
      });

      const [, request] = api.postWithMeta.mock.calls[0];
      expect(request).toMatchObject({
        workspace_id: "ws-1",
        challenge_pack_version_id: "version-1",
        execution_mode: "single_agent",
        participants: [
          {
            agent_deployment_id: "deploy-1",
            label: "Primary Agent",
          },
        ],
        eval_session: {
          repetitions: 5,
          aggregation: {
            method: "median",
            report_variance: true,
            confidence_interval: 0.95,
          },
          routing_task_snapshot: {
            routing: {},
            task: {},
          },
          schema_version: 1,
        },
      });
      expect(mockPush).toHaveBeenCalledWith(
        "/workspaces/ws-1/eval-sessions/session-1",
      );
      expect(toast.success).toHaveBeenCalledWith("Eval session created");
    } finally {
      view.cleanup();
    }
  });

  it("warms dialog data once per open instead of revalidating on later rerenders", async () => {
    const api = buildApiMock();
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Eval Session"));
      await flushPromises();

      await waitFor(() => {
        expect(mockMutateMany).toHaveBeenCalledWith(
          workspaceMutationKeys.createEvalSessionDialog("ws-1"),
        );
      });
      expect(mockMutateMany).toHaveBeenCalledTimes(1);

      changeSelect(findSelectForLabel("Challenge Pack"), "pack-1");
      await flushPromises();

      clickElement(findCheckboxByLabel("Primary Agent"));
      await flushPromises();

      expect(mockMutateMany).toHaveBeenCalledTimes(1);
    } finally {
      view.cleanup();
    }
  });

  it("waits for eval-session list revalidation to finish before redirecting", async () => {
    const api = buildApiMock();
    const revalidation = deferredPromise<void>();
    mockCreateApiClient.mockReturnValue(api);
    mockMutate.mockReturnValue(revalidation.promise);

    const view = renderDialog();
    try {
      clickElement(findButton("New Eval Session"));
      await flushPromises();

      changeSelect(findSelectForLabel("Challenge Pack"), "pack-1");
      await flushPromises();

      clickElement(findCheckboxByLabel("Primary Agent"));
      await flushPromises();

      clickElement(findButton("Create Eval Session"));

      await waitFor(() => {
        expect(mockMutate).toHaveBeenCalledWith(
          workspaceResourceKeys.evalSessions("ws-1", 0),
        );
      });
      expect(mockPush).not.toHaveBeenCalled();

      revalidation.resolve(undefined);
      await flushPromises();

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith(
          "/workspaces/ws-1/eval-sessions/session-1",
        );
      });
    } finally {
      view.cleanup();
    }
  });

  it("shows a follow-up toast and still redirects when eval-session list revalidation fails", async () => {
    const api = buildApiMock();
    mockCreateApiClient.mockReturnValue(api);
    mockMutate.mockRejectedValueOnce(new Error("revalidation failed"));

    const view = renderDialog();
    try {
      clickElement(findButton("New Eval Session"));
      await flushPromises();

      changeSelect(findSelectForLabel("Challenge Pack"), "pack-1");
      await flushPromises();

      clickElement(findCheckboxByLabel("Primary Agent"));
      await flushPromises();

      clickElement(findButton("Create Eval Session"));

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith(
          "/workspaces/ws-1/eval-sessions/session-1",
        );
      });
      expect(toast.success).toHaveBeenCalledWith("Eval session created");
      expect(toast.error).toHaveBeenCalledWith(
        "Eval session created, but the eval sessions list could not be refreshed.",
      );
    } finally {
      view.cleanup();
    }
  });

  it("surfaces validation errors returned by the eval-session API", async () => {
    const api = buildApiMock(422);
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Eval Session"));
      await flushPromises();

      changeSelect(findSelectForLabel("Challenge Pack"), "pack-1");
      await flushPromises();

      clickElement(findCheckboxByLabel("Primary Agent"));
      await flushPromises();

      clickElement(findButton("Create Eval Session"));

      await waitFor(() => {
        expect(api.postWithMeta).toHaveBeenCalledTimes(1);
      });

      expect(toast.error).toHaveBeenCalledWith(
        "repetitions must be an integer between 1 and 100",
      );
      expect(mockPush).not.toHaveBeenCalled();
    } finally {
      view.cleanup();
    }
  });

  it("allows selecting deployments even when they share a build version", async () => {
    const api = buildApiMock(201, {
      deployments: [
        {
          id: "deploy-1",
          name: "Primary Agent",
          status: "active",
          current_build_version_id: "build-version-1",
        },
        {
          id: "deploy-2",
          name: "Primary Agent Copy",
          status: "active",
          current_build_version_id: "build-version-1",
        },
        {
          id: "deploy-3",
          name: "Unique Agent",
          status: "active",
          current_build_version_id: "build-version-2",
        },
      ],
    });
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Eval Session"));
      await flushPromises();

      changeSelect(findSelectForLabel("Challenge Pack"), "pack-1");
      await flushPromises();

      expect(findCheckboxByLabel("Primary Agent").disabled).toBe(false);
      expect(findCheckboxByLabel("Primary Agent Copy").disabled).toBe(false);
      expect(findCheckboxByLabel("Unique Agent").disabled).toBe(false);

      clickElement(findCheckboxByLabel("Primary Agent"));
      await flushPromises();

      clickElement(findButton("Create Eval Session"));

      await waitFor(() => {
        expect(api.postWithMeta).toHaveBeenCalledTimes(1);
      });

      const [, request] = api.postWithMeta.mock.calls[0];
      expect(request).toMatchObject({
        participants: [
          {
            agent_deployment_id: "deploy-1",
            label: "Primary Agent",
          },
        ],
      });
    } finally {
      view.cleanup();
    }
  });

  it("maps unresolved deployment errors to a clearer toast", async () => {
    const api = buildApiMock(422, {
      responseData: {
        errors: [
          {
            field: "participants[0].agent_deployment_id",
            code: "participants.agent_deployment_id.unresolved",
            message:
              "agent_deployment_id must reference an active deployment with a snapshot in the selected workspace",
          },
        ],
      },
    });
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Eval Session"));
      await flushPromises();

      changeSelect(findSelectForLabel("Challenge Pack"), "pack-1");
      await flushPromises();

      clickElement(findCheckboxByLabel("Primary Agent"));
      await flushPromises();

      clickElement(findButton("Create Eval Session"));

      await waitFor(() => {
        expect(api.postWithMeta).toHaveBeenCalledTimes(1);
      });

      expect(toast.error).toHaveBeenCalledWith(
        "One or more selected deployments are no longer active or available in this workspace. Refresh the dialog and choose another deployment.",
      );
    } finally {
      view.cleanup();
    }
  });
});
