import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  ChallengePack,
  RegressionCase,
  RegressionSuite,
} from "@/lib/api/types";
import { ApiError } from "@/lib/api/errors";

import { RegressionSuitesClient } from "./regression-suites-client";

const {
  mockGetAccessToken,
  mockCreateApiClient,
  mockPatch,
  mockMutate,
  mockRouterReplace,
  mockConfirm,
  mockSuitesPage,
  mockProposedCasesPage,
  mockPacksResponse,
  toast,
} = vi.hoisted(() => ({
  mockGetAccessToken: vi.fn(),
  mockCreateApiClient: vi.fn(),
  mockPatch: vi.fn(),
  mockMutate: vi.fn(),
  mockRouterReplace: vi.fn(),
  mockConfirm: vi.fn(),
  mockSuitesPage: {
    current: undefined as
      | {
          items: RegressionSuite[];
          total: number;
          limit: number;
          offset: number;
        }
      | undefined,
  },
  mockProposedCasesPage: {
    current: undefined as
      | {
          items: RegressionCase[];
          total: number;
          limit: number;
          offset: number;
        }
      | undefined,
  },
  mockPacksResponse: {
    current: undefined as { items: ChallengePack[] } | undefined,
  },
  toast: Object.assign(vi.fn(), {
    success: vi.fn(),
    error: vi.fn(),
  }),
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
  useRouter: () => ({
    push: vi.fn(),
    replace: mockRouterReplace,
  }),
  useSearchParams: () => new URLSearchParams(),
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
    useApiListQuery: (path: string) => {
      if (path.includes("challenge-packs")) {
        return {
          data: mockPacksResponse.current ?? { items: [pack] },
          error: undefined,
          isLoading: false,
        };
      }
      return { data: { items: [] }, error: undefined, isLoading: false };
    },
    usePaginatedApiQuery: (path: string) => {
      if (path.includes("regression-cases")) {
        return {
          data: mockProposedCasesPage.current ?? {
            items: [proposedCase],
            total: 1,
            limit: 20,
            offset: 0,
          },
          error: undefined,
          isLoading: false,
        };
      }
      return {
        data: mockSuitesPage.current ?? {
          items: [suite],
          total: 1,
          limit: 50,
          offset: 0,
        },
        error: undefined,
        isLoading: false,
      };
    },
    useApiMutator: () => ({ mutate: mockMutate }),
  };
});

vi.mock("@/components/ui/confirm-dialog", () => ({
  ConfirmProvider: ({ children }: { children: React.ReactNode }) => children,
  useConfirm: () => mockConfirm,
}));

vi.mock("@/components/ui/page-header", () => ({
  PageHeader: ({ title, actions }: { title: string; actions?: React.ReactNode }) => (
    <header>
      <h1>{title}</h1>
      {actions}
    </header>
  ),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button type="button" {...props}>
      {children}
    </button>
  ),
}));

vi.mock("@/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuItem: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button type="button" {...props}>
      {children}
    </button>
  ),
  DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => (
    <button type="button">{children}</button>
  ),
}));

vi.mock("@/components/ui/select", () => ({
  Select: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SelectItem: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SelectTrigger: ({ children }: { children: React.ReactNode }) => (
    <button type="button">{children}</button>
  ),
  SelectValue: () => <span>value</span>,
}));

vi.mock("./create-suite-dialog", () => ({
  CreateSuiteDialog: () => null,
}));

const suite: RegressionSuite = {
  id: "suite-1",
  workspace_id: "ws-1",
  source_challenge_pack_id: "pack-1",
  name: "Critical regressions",
  description: "",
  status: "active",
  source_mode: "derived_only",
  default_gate_severity: "warning",
  case_count: 1,
  created_by_user_id: "user-1",
  created_at: "2026-05-01T12:00:00Z",
  updated_at: "2026-05-01T12:00:00Z",
};

const pack: ChallengePack = {
  id: "pack-1",
  name: "Support Tickets",
  description: "",
  slug: "support-tickets",
  versions: [],
  created_at: "2026-05-01T12:00:00Z",
  updated_at: "2026-05-01T12:00:00Z",
};

const proposedCase: RegressionCase = {
  id: "case-1",
  suite_id: "suite-1",
  workspace_id: "ws-1",
  suite_name: "Critical regressions",
  title: "Missing escalation",
  description: "",
  status: "proposed",
  severity: "blocking",
  promotion_mode: "output_only",
  source_challenge_pack_version_id: "version-1",
  source_challenge_identity_id: "identity-1",
  source_case_key: "ticket-1",
  evidence_tier: "native_structured",
  failure_class: "behavioral_regression",
  failure_summary: "The agent failed to escalate a critical ticket.",
  payload_snapshot: {},
  expected_contract: {},
  validator_overrides: null,
  metadata: {},
  validation: {
    status: "collecting_signal",
    maintenance_status: "needs_signal",
    run_count: 2,
    failure_count: 1,
    pass_count: 1,
    reproduction_rate: 0.5,
    reproduction_threshold: 0.6,
    required_runs: 5,
    remaining_runs: 3,
    recommended_action: "Collect more signal.",
    maintenance_action: "Keep collecting signal.",
  },
  created_at: "2026-05-01T12:00:00Z",
  updated_at: "2026-05-01T12:00:00Z",
};

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
  });
}

function render(element: React.ReactElement) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);
  act(() => {
    root.render(element);
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

describe("RegressionSuitesClient proposed queue", () => {
  beforeEach(() => {
    mockGetAccessToken.mockReset().mockResolvedValue("token");
    mockPatch.mockReset().mockResolvedValue(proposedCase);
    mockCreateApiClient.mockReset().mockReturnValue({ patch: mockPatch });
    mockMutate.mockReset().mockResolvedValue(undefined);
    mockRouterReplace.mockReset();
    mockConfirm.mockReset().mockResolvedValue(true);
    mockSuitesPage.current = { items: [suite], total: 1, limit: 50, offset: 0 };
    mockProposedCasesPage.current = {
      items: [proposedCase],
      total: 1,
      limit: 20,
      offset: 0,
    };
    mockPacksResponse.current = { items: [pack] };
    toast.success.mockReset();
    toast.error.mockReset();
  });

  it("renders proposed cases and promotes one from the workspace queue", async () => {
    mockSuitesPage.current = { items: [], total: 0, limit: 50, offset: 0 };
    const view = render(<RegressionSuitesClient workspaceId="ws-1" />);
    try {
      expect(view.container.textContent).toContain("Proposed Cases");
      expect(view.container.textContent).toContain("Missing escalation");
      expect(view.container.textContent).toContain("Critical regressions");
      expect(view.container.textContent).toContain("50% repro");

      const promote = Array.from(view.container.querySelectorAll("button")).find(
        (button) => button.textContent?.includes("Promote"),
      );
      expect(promote).toBeTruthy();

      await act(async () => {
        promote?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      });
      await flushPromises();

      expect(mockPatch).toHaveBeenCalledWith(
        "/v1/workspaces/ws-1/regression-cases/case-1",
        { status: "active" },
      );
      expect(toast.success).toHaveBeenCalledWith("Case promoted");
      expect(mockMutate).toHaveBeenCalledTimes(2);
      expect(mockMutate.mock.calls[0][0]).toEqual(expect.any(Function));
      expect(
        mockMutate.mock.calls[0][0]([
          "/v1/workspaces/ws-1/regression-cases",
          { status: "proposed", limit: 20, offset: 0 },
        ]),
      ).toBe(true);
      expect(
        mockMutate.mock.calls[1][0]([
          "/v1/workspaces/ws-1/regression-suites",
          { limit: 50, offset: 50 },
        ]),
      ).toBe(true);
    } finally {
      view.cleanup();
    }
  });

  it("rejects a proposed case after confirmation", async () => {
    const view = render(<RegressionSuitesClient workspaceId="ws-1" />);
    try {
      const reject = Array.from(view.container.querySelectorAll("button")).find(
        (button) => button.getAttribute("aria-label") === "Reject proposed case",
      );
      expect(reject).toBeTruthy();

      await act(async () => {
        reject?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      });
      await flushPromises();

      expect(mockConfirm).toHaveBeenCalledWith(
        expect.objectContaining({
          confirmLabel: "Reject",
          variant: "danger",
        }),
      );
      expect(mockPatch).toHaveBeenCalledWith(
        "/v1/workspaces/ws-1/regression-cases/case-1",
        { status: "rejected" },
      );
      expect(toast.success).toHaveBeenCalledWith("Case rejected");
    } finally {
      view.cleanup();
    }
  });

  it("shows API errors when case status patching fails", async () => {
    mockPatch.mockRejectedValue(
      new ApiError(409, "transition_conflict", "cannot transition"),
    );
    const view = render(<RegressionSuitesClient workspaceId="ws-1" />);
    try {
      const promote = Array.from(view.container.querySelectorAll("button")).find(
        (button) => button.textContent?.includes("Promote"),
      );
      expect(promote).toBeTruthy();

      await act(async () => {
        promote?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      });
      await flushPromises();

      expect(toast.error).toHaveBeenCalledWith("cannot transition");
    } finally {
      view.cleanup();
    }
  });

  it("hides the proposed queue when there are no proposed cases", () => {
    mockProposedCasesPage.current = {
      items: [],
      total: 0,
      limit: 20,
      offset: 0,
    };
    const view = render(<RegressionSuitesClient workspaceId="ws-1" />);
    try {
      expect(view.container.textContent).not.toContain("Proposed Cases");
    } finally {
      view.cleanup();
    }
  });
});
