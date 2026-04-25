import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { workspaceMutationKeys, workspaceResourceKeys } from "@/lib/workspace-resource";

import { CreateRunDialog } from "./create-run-dialog";

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
  }: React.ButtonHTMLAttributes<HTMLButtonElement>) => {
    return React.createElement("button", props, children);
  },
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
    }) => {
      return React.createElement(
        DialogOpenContext.Provider,
        { value: open },
        React.createElement(
          DialogToggleContext.Provider,
          { value: onOpenChange },
          children,
        ),
      );
    },
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
    DialogDescription: ({ children }: { children: React.ReactNode }) => {
      return React.createElement("p", null, children);
    },
    DialogFooter: ({ children }: { children: React.ReactNode }) => {
      return React.createElement("div", null, children);
    },
    DialogHeader: ({ children }: { children: React.ReactNode }) => {
      return React.createElement("div", null, children);
    },
    DialogTitle: ({ children }: { children: React.ReactNode }) => {
      return React.createElement("h1", null, children);
    },
  };
});

vi.mock("lucide-react", () => ({
  Loader2: () => React.createElement("span", null, "loader"),
  Plus: () => React.createElement("span", null, "plus"),
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

function findButton(text: string) {
  const buttons = Array.from(document.querySelectorAll("button"));
  const button = buttons.find((candidate) =>
    candidate.textContent?.includes(text),
  );
  if (!button) {
    throw new Error(`Button with text ${text} not found`);
  }
  return button;
}

function findCheckboxByLabel(text: string) {
  const labels = Array.from(document.querySelectorAll("label"));
  const label = labels.find((candidate) =>
    candidate.textContent?.includes(text),
  );
  if (!label) {
    throw new Error(`Checkbox label ${text} not found`);
  }
  const checkbox = label.querySelector('input[type="checkbox"]');
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
    root.render(React.createElement(CreateRunDialog, { workspaceId: "ws-1" }));
  });

  return {
    container,
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

interface BuildApiMockOptions {
  versions?: Array<{
    id: string;
    version_number: number;
    lifecycle_status?: string;
  }>;
  inputSetsByVersionId?: Record<
    string,
    Array<{
      id: string;
      challenge_pack_version_id: string;
      input_key: string;
      name: string;
    }>
  >;
}

function buildApiMock(options: BuildApiMockOptions = {}) {
  const versions = options.versions ?? [
    {
      id: "version-1",
      version_number: 1,
      lifecycle_status: "runnable",
    },
  ];
  const inputSetsByVersionId = options.inputSetsByVersionId ?? {
    "version-1": [],
  };
  const post = vi.fn().mockResolvedValue({ id: "run-1" });
  const get = vi.fn(async (url: string) => {
    if (url === "/v1/workspaces/ws-1/challenge-packs") {
      return {
        items: [
          {
            id: "pack-1",
            name: "Support Pack",
            versions,
          },
        ],
      };
    }
    if (url.startsWith("/v1/workspaces/ws-1/challenge-pack-versions/")) {
      const parts = url.split("/");
      const versionId = parts[5];
      return {
        items: inputSetsByVersionId[versionId] ?? [],
      };
    }
    if (url === "/v1/workspaces/ws-1/agent-deployments") {
      return {
        items: [
          { id: "deploy-1", name: "Primary Agent", status: "active" },
          { id: "deploy-2", name: "Archived Agent", status: "archived" },
        ],
      };
    }
    if (url === "/v1/workspaces/ws-1/regression-suites/suite-1/cases") {
      return {
        items: [
          {
            id: "case-1",
            suite_id: "suite-1",
            workspace_id: "ws-1",
            title: "Filesystem Regression",
            description: "",
            status: "active",
            severity: "blocking",
            promotion_mode: "full_executable",
            source_challenge_pack_version_id: "version-1",
            source_challenge_identity_id: "challenge-1",
            source_case_key: "case-a",
            evidence_tier: "native_structured",
            failure_class: "policy_violation",
            failure_summary: "Attempted forbidden write",
            payload_snapshot: {},
            expected_contract: {},
            metadata: {},
            created_at: "2026-04-19T00:00:00Z",
            updated_at: "2026-04-19T00:00:00Z",
          },
        ],
      };
    }
    throw new Error(`Unexpected GET ${url}`);
  });
  const paginated = vi.fn(async (url: string) => {
    if (url === "/v1/workspaces/ws-1/regression-suites") {
      return {
        items: [
          {
            id: "suite-1",
            workspace_id: "ws-1",
            source_challenge_pack_id: "pack-1",
            name: "Regression Suite",
            description: "Focused failures",
            status: "active",
            source_mode: "derived_only",
            default_gate_severity: "warning",
            case_count: 1,
            created_by_user_id: "user-1",
            created_at: "2026-04-19T00:00:00Z",
            updated_at: "2026-04-19T00:00:00Z",
          },
        ],
        total: 1,
        limit: 100,
        offset: 0,
      };
    }
    throw new Error(`Unexpected paginated ${url}`);
  });

  return { get, paginated, post };
}

describe("CreateRunDialog", () => {
  beforeEach(() => {
    // These component tests drive React through manual DOM events rather than RTL.
    // Mark the environment explicitly so React's act() warnings stay actionable.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
    document.body.innerHTML = "";
    mockPush.mockReset();
    mockRefresh.mockReset();
    mockGetAccessToken.mockReset();
    mockCreateApiClient.mockReset();
    mockMutate.mockReset();
    mockMutateMany.mockReset();
    toast.mockReset();
    toast.success.mockReset();
    toast.error.mockReset();

    mockGetAccessToken.mockResolvedValue("token");
    mockMutate.mockResolvedValue(undefined);
    mockMutateMany.mockResolvedValue(undefined);
  });

  it("submits regression selections and official pack mode in the create run request", async () => {
    const api = buildApiMock();
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Run"));

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-packs",
        );
      });

      const packSelect = document.querySelector(
        'select[aria-label="Challenge Pack"]',
      );
      if (!(packSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Pack select not found");
      }
      changeSelect(packSelect, "pack-1");

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/regression-suites/suite-1/cases",
        );
      });
      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-pack-versions/version-1/input-sets",
        );
      });

      clickElement(findCheckboxByLabel("Primary Agent"));
      clickElement(findCheckboxByLabel("Regression Suite"));
      clickElement(findCheckboxByLabel("Filesystem Regression"));

      const officialPackModeSelect = document.querySelector(
        'select[aria-label="Official Pack Mode"]',
      );
      if (!(officialPackModeSelect instanceof HTMLSelectElement)) {
        throw new Error("Official Pack Mode select not found");
      }
      changeSelect(officialPackModeSelect, "suite_only");

      clickElement(findButton("Create Run"));

      await waitFor(() => {
        expect(api.post).toHaveBeenCalledWith("/v1/runs", {
          workspace_id: "ws-1",
          challenge_pack_version_id: "version-1",
          challenge_input_set_id: undefined,
          name: undefined,
          agent_deployment_ids: ["deploy-1"],
          regression_suite_ids: ["suite-1"],
          regression_case_ids: ["case-1"],
          official_pack_mode: "suite_only",
        });
      });
    } finally {
      view.cleanup();
    }
  });

  it("warms dialog data once per open instead of revalidating on later rerenders", async () => {
    const api = buildApiMock();
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Run"));

      await waitFor(() => {
        expect(mockMutateMany).toHaveBeenCalledWith(
          workspaceMutationKeys.createRunDialog("ws-1"),
        );
      });
      expect(mockMutateMany).toHaveBeenCalledTimes(1);

      const nameInput = document.querySelector('input[type="text"]');
      if (!(nameInput instanceof HTMLInputElement)) {
        throw new Error("Name input not found");
      }
      changeInput(nameInput, "Fresh run");
      await flushPromises();

      expect(mockMutateMany).toHaveBeenCalledTimes(1);
    } finally {
      view.cleanup();
    }
  });

  it("waits for run list revalidation to finish before redirecting", async () => {
    const api = buildApiMock();
    const revalidation = deferredPromise<void>();
    mockCreateApiClient.mockReturnValue(api);
    mockMutate.mockReturnValue(revalidation.promise);

    const view = renderDialog();
    try {
      clickElement(findButton("New Run"));

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-packs",
        );
      });

      const packSelect = document.querySelector(
        'select[aria-label="Challenge Pack"]',
      );
      if (!(packSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Pack select not found");
      }
      changeSelect(packSelect, "pack-1");

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-pack-versions/version-1/input-sets",
        );
      });

      clickElement(findCheckboxByLabel("Primary Agent"));
      clickElement(findButton("Create Run"));

      await waitFor(() => {
        expect(mockMutate).toHaveBeenCalledWith(
          workspaceResourceKeys.runs("ws-1", 0),
        );
      });
      expect(mockPush).not.toHaveBeenCalled();

      revalidation.resolve(undefined);
      await flushPromises();

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/workspaces/ws-1/runs/run-1");
      });
    } finally {
      view.cleanup();
    }
  });

  it("shows a follow-up toast and still redirects when run list revalidation fails", async () => {
    const api = buildApiMock();
    mockCreateApiClient.mockReturnValue(api);
    mockMutate.mockRejectedValueOnce(new Error("revalidation failed"));

    const view = renderDialog();
    try {
      clickElement(findButton("New Run"));

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-packs",
        );
      });

      const packSelect = document.querySelector(
        'select[aria-label="Challenge Pack"]',
      );
      if (!(packSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Pack select not found");
      }
      changeSelect(packSelect, "pack-1");

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-pack-versions/version-1/input-sets",
        );
      });

      clickElement(findCheckboxByLabel("Primary Agent"));
      clickElement(findButton("Create Run"));

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/workspaces/ws-1/runs/run-1");
      });
      expect(toast.success).toHaveBeenCalledWith("Run created");
      expect(toast.error).toHaveBeenCalledWith(
        "Run created, but the runs list could not be refreshed.",
      );
    } finally {
      view.cleanup();
    }
  });

  it("resets official pack mode back to full when regression selections are cleared", async () => {
    const api = buildApiMock();
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Run"));

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-packs",
        );
      });

      const packSelect = document.querySelector(
        'select[aria-label="Challenge Pack"]',
      );
      if (!(packSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Pack select not found");
      }
      changeSelect(packSelect, "pack-1");

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/regression-suites/suite-1/cases",
        );
      });

      const suiteCheckbox = findCheckboxByLabel("Regression Suite");
      clickElement(suiteCheckbox);

      const officialPackModeSelect = document.querySelector(
        'select[aria-label="Official Pack Mode"]',
      );
      if (!(officialPackModeSelect instanceof HTMLSelectElement)) {
        throw new Error("Official Pack Mode select not found");
      }
      changeSelect(officialPackModeSelect, "suite_only");
      expect(officialPackModeSelect.value).toBe("suite_only");

      clickElement(suiteCheckbox);

      await waitFor(() => {
        expect(officialPackModeSelect.value).toBe("full");
        expect(officialPackModeSelect.disabled).toBe(true);
      });
    } finally {
      view.cleanup();
    }
  });

  it("auto-selects the only input set for the selected version", async () => {
    const api = buildApiMock({
      inputSetsByVersionId: {
        "version-1": [
          {
            id: "input-1",
            challenge_pack_version_id: "version-1",
            input_key: "support_ticket_triage",
            name: "Support Ticket Triage",
          },
        ],
      },
    });
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Run"));

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-packs",
        );
      });

      const packSelect = document.querySelector(
        'select[aria-label="Challenge Pack"]',
      );
      if (!(packSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Pack select not found");
      }
      changeSelect(packSelect, "pack-1");

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-pack-versions/version-1/input-sets",
        );
      });

      const inputSetSelect = document.querySelector(
        'select[aria-label="Challenge Input Set"]',
      );
      if (!(inputSetSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Input Set select not found");
      }
      expect(inputSetSelect.value).toBe("input-1");
      expect(inputSetSelect.disabled).toBe(true);

      clickElement(findCheckboxByLabel("Primary Agent"));
      clickElement(findButton("Create Run"));

      await waitFor(() => {
        expect(api.post).toHaveBeenCalledWith("/v1/runs", {
          workspace_id: "ws-1",
          challenge_pack_version_id: "version-1",
          challenge_input_set_id: "input-1",
          name: undefined,
          agent_deployment_ids: ["deploy-1"],
          regression_suite_ids: undefined,
          regression_case_ids: undefined,
          official_pack_mode: undefined,
        });
      });
    } finally {
      view.cleanup();
    }
  });

  it("requires an explicit input-set choice when a version has multiple input sets", async () => {
    const api = buildApiMock({
      inputSetsByVersionId: {
        "version-1": [
          {
            id: "input-1",
            challenge_pack_version_id: "version-1",
            input_key: "support_ticket_triage",
            name: "Support Ticket Triage",
          },
          {
            id: "input-2",
            challenge_pack_version_id: "version-1",
            input_key: "incident_summary",
            name: "Incident Summary",
          },
        ],
      },
    });
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Run"));

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-packs",
        );
      });

      const packSelect = document.querySelector(
        'select[aria-label="Challenge Pack"]',
      );
      if (!(packSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Pack select not found");
      }
      changeSelect(packSelect, "pack-1");

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-pack-versions/version-1/input-sets",
        );
      });

      clickElement(findCheckboxByLabel("Primary Agent"));

      const createRunButton = findButton("Create Run");
      expect(createRunButton).toHaveProperty("disabled", true);

      const inputSetSelect = document.querySelector(
        'select[aria-label="Challenge Input Set"]',
      );
      if (!(inputSetSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Input Set select not found");
      }
      changeSelect(inputSetSelect, "input-2");

      await waitFor(() => {
        expect(createRunButton).toHaveProperty("disabled", false);
      });
    } finally {
      view.cleanup();
    }
  });

  it("clears stale input-set selection when the chosen version changes", async () => {
    const api = buildApiMock({
      versions: [
        {
          id: "version-1",
          version_number: 1,
          lifecycle_status: "runnable",
        },
        {
          id: "version-2",
          version_number: 2,
          lifecycle_status: "runnable",
        },
      ],
      inputSetsByVersionId: {
        "version-1": [
          {
            id: "input-1",
            challenge_pack_version_id: "version-1",
            input_key: "support_ticket_triage",
            name: "Support Ticket Triage",
          },
          {
            id: "input-2",
            challenge_pack_version_id: "version-1",
            input_key: "incident_summary",
            name: "Incident Summary",
          },
        ],
        "version-2": [
          {
            id: "input-3",
            challenge_pack_version_id: "version-2",
            input_key: "invoice_total_extraction",
            name: "Invoice Total Extraction",
          },
        ],
      },
    });
    mockCreateApiClient.mockReturnValue(api);

    const view = renderDialog();
    try {
      clickElement(findButton("New Run"));

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-packs",
        );
      });

      const packSelect = document.querySelector(
        'select[aria-label="Challenge Pack"]',
      );
      if (!(packSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Pack select not found");
      }
      changeSelect(packSelect, "pack-1");

      const versionSelect = document.querySelector(
        'select[aria-label="Challenge Pack Version"]',
      );
      if (!(versionSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Pack Version select not found");
      }
      changeSelect(versionSelect, "version-1");

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-pack-versions/version-1/input-sets",
        );
      });

      const inputSetSelect = document.querySelector(
        'select[aria-label="Challenge Input Set"]',
      );
      if (!(inputSetSelect instanceof HTMLSelectElement)) {
        throw new Error("Challenge Input Set select not found");
      }
      changeSelect(inputSetSelect, "input-2");
      expect(inputSetSelect.value).toBe("input-2");

      changeSelect(versionSelect, "version-2");

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          "/v1/workspaces/ws-1/challenge-pack-versions/version-2/input-sets",
        );
      });

      await waitFor(() => {
        const refreshedInputSetSelect = document.querySelector(
          'select[aria-label="Challenge Input Set"]',
        );
        if (!(refreshedInputSetSelect instanceof HTMLSelectElement)) {
          throw new Error("Challenge Input Set select not found");
        }
        expect(refreshedInputSetSelect.value).toBe("input-3");
        expect(refreshedInputSetSelect.disabled).toBe(true);
      });
    } finally {
      view.cleanup();
    }
  });
});
