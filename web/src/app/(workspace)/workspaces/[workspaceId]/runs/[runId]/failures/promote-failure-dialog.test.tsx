import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/errors";
import type { FailureReviewItem } from "@/lib/api/types";
import { PromoteFailureDialog } from "./promote-failure-dialog";

const {
  mockPush,
  mockGetAccessToken,
  mockCreateApiClient,
  mockListRegressionSuites,
  mockPromoteFailure,
  mockBuildPromotionOverrides,
  mockDefaultPromotionSeverityForFailure,
  toast,
} = vi.hoisted(() => {
  return {
    mockPush: vi.fn(),
    mockGetAccessToken: vi.fn(),
    mockCreateApiClient: vi.fn(),
    mockListRegressionSuites: vi.fn(),
    mockPromoteFailure: vi.fn(),
    mockBuildPromotionOverrides: vi.fn(),
    mockDefaultPromotionSeverityForFailure: vi.fn(),
    toast: Object.assign(vi.fn(), {
      success: vi.fn(),
      error: vi.fn(),
    }),
  };
});

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

vi.mock("@/lib/api/regression", () => ({
  listRegressionSuites: (...args: unknown[]) => mockListRegressionSuites(...args),
  promoteFailure: (...args: unknown[]) => mockPromoteFailure(...args),
  buildPromotionOverrides: (...args: unknown[]) =>
    mockBuildPromotionOverrides(...args),
  defaultPromotionSeverityForFailure: (...args: unknown[]) =>
    mockDefaultPromotionSeverityForFailure(...args),
}));

vi.mock("next/link", () => {
  return {
    default: ({
      href,
      children,
      ...props
    }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => {
      return React.createElement("a", { href, ...props }, children);
    },
  };
});

vi.mock("@/components/ui/button", () => {
  return {
    Button: ({
      children,
      ...props
    }: React.ButtonHTMLAttributes<HTMLButtonElement>) => {
      return React.createElement("button", props, children);
    },
  };
});

vi.mock("@/components/ui/input", () => {
  return {
    Input: (props: React.InputHTMLAttributes<HTMLInputElement>) => {
      return React.createElement("input", props);
    },
  };
});

vi.mock("@/components/ui/dialog", () => {
  return {
    Dialog: ({
      open,
      children,
    }: {
      open: boolean;
      children: React.ReactNode;
    }) => {
      return open
        ? React.createElement("div", { "data-testid": "dialog-root" }, children)
        : null;
    },
    DialogContent: ({ children }: { children: React.ReactNode }) => {
      return React.createElement("div", null, children);
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

vi.mock("@/components/ui/select", () => {
  return {
    Select: ({
      value,
      onValueChange,
      children,
    }: {
      value: string;
      onValueChange: (value: string) => void;
      children: React.ReactNode;
    }) => {
      const options = React.Children.toArray(children).flatMap((child) => {
        if (!React.isValidElement(child)) return [];
        return React.Children.toArray(
          (child as React.ReactElement<{ children?: React.ReactNode }>).props
            .children,
        );
      });
      return (
        React.createElement(
          "select",
          {
            "aria-label": "Destination suite",
            value,
            onChange: (event: React.ChangeEvent<HTMLSelectElement>) =>
              onValueChange(event.target.value),
          },
          options,
        )
      );
    },
    SelectTrigger: ({ children }: { children: React.ReactNode }) => {
      return React.createElement(React.Fragment, null, children);
    },
    SelectValue: ({ placeholder }: { placeholder?: string }) => {
      return React.createElement(
        "option",
        { value: "" },
        placeholder ?? "Select",
      );
    },
    SelectContent: ({ children }: { children: React.ReactNode }) => {
      return React.createElement(React.Fragment, null, children);
    },
    SelectItem: ({
      value,
      children,
    }: {
      value: string;
      children: React.ReactNode;
    }) => {
      return React.createElement("option", { value }, children);
    },
  };
});

function makeItem(
  overrides: Partial<FailureReviewItem> = {},
): FailureReviewItem {
  return {
    run_id: "run-1",
    run_agent_id: "agent-1",
    challenge_identity_id: "challenge-1",
    challenge_key: "challenge-a",
    case_key: "case-a",
    item_key: "item-a",
    failure_state: "failed",
    failed_dimensions: ["correctness"],
    failed_checks: ["capture.files"],
    failure_class: "policy_violation",
    headline: "Filesystem write regression",
    detail: "The model attempted a forbidden file write.",
    recommended_action: "",
    promotable: true,
    promotion_mode_available: ["full_executable", "output_only"],
    replay_step_refs: [],
    artifact_refs: [],
    judge_refs: [
      {
        key: "policy.filesystem",
        kind: "threshold",
        normalized_score: 0.4,
        reason: "Filesystem policy check failed.",
      },
    ],
    metric_refs: [],
    evidence_tier: "native_structured",
    severity: "blocking",
    ...overrides,
  };
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

function setInputValue(element: HTMLInputElement | HTMLTextAreaElement, value: string) {
  element.value = value;
  element.dispatchEvent(new Event("input", { bubbles: true }));
  element.dispatchEvent(new Event("change", { bubbles: true }));
}

function clickElement(element: Element) {
  element.dispatchEvent(
    new MouseEvent("click", {
      bubbles: true,
      cancelable: true,
    }),
  );
}

function renderDialog(props: Partial<React.ComponentProps<typeof PromoteFailureDialog>> = {}) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);
  const onClose = vi.fn();

  act(() => {
    root.render(
      React.createElement(PromoteFailureDialog, {
        workspaceId: "ws-1",
        runId: "run-1",
        item: makeItem(),
        sourceChallengePackId: "pack-1",
        sourceChallengePackName: "Source Pack",
        onClose,
        ...props,
      }),
    );
  });

  return {
    container,
    onClose,
    unmount() {
      act(() => {
        root.unmount();
      });
      container.remove();
    },
  };
}

describe("PromoteFailureDialog", () => {
  beforeEach(() => {
    mockPush.mockReset();
    mockGetAccessToken.mockReset();
    mockCreateApiClient.mockReset();
    mockListRegressionSuites.mockReset();
    mockPromoteFailure.mockReset();
    mockBuildPromotionOverrides.mockReset();
    mockDefaultPromotionSeverityForFailure.mockReset();
    toast.mockReset();
    toast.success.mockReset();
    toast.error.mockReset();

    mockGetAccessToken.mockResolvedValue("token");
    mockCreateApiClient.mockReturnValue({ client: true });
    mockBuildPromotionOverrides.mockReturnValue(undefined);
    mockDefaultPromotionSeverityForFailure.mockReturnValue("blocking");
    mockListRegressionSuites.mockResolvedValue({
      items: [
        {
          id: "suite-1",
          workspace_id: "ws-1",
          source_challenge_pack_id: "pack-1",
          name: "Suite One",
          description: "",
          status: "active",
          source_mode: "derived_only",
          default_gate_severity: "warning",
          created_by_user_id: "user-1",
          created_at: "2026-04-19T00:00:00Z",
          updated_at: "2026-04-19T00:00:00Z",
        },
      ],
      total: 1,
      limit: 100,
      offset: 0,
    });
  });

  it("shows a success toast for newly created promotions", async () => {
    mockPromoteFailure.mockResolvedValue({
      created: true,
      case: { id: "case-1", suite_id: "suite-1" },
    });

    const view = renderDialog();
    try {
      await waitFor(() => {
        expect(mockListRegressionSuites).toHaveBeenCalled();
      });

      const submitButton = Array.from(
        view.container.querySelectorAll("button"),
      ).find((button) => button.textContent === "Promote");
      expect(submitButton).toBeTruthy();

      await act(async () => {
        clickElement(submitButton!);
      });

      await waitFor(() => {
        expect(mockPromoteFailure).toHaveBeenCalledWith(
          { client: true },
          "ws-1",
          "run-1",
          "challenge-1",
          expect.objectContaining({
            suite_id: "suite-1",
            title: "Filesystem write regression",
            severity: "blocking",
          }),
        );
      });
      expect(toast.success).toHaveBeenCalledWith(
        "Failure promoted",
        expect.objectContaining({
          action: expect.objectContaining({ label: "Open case" }),
        }),
      );
      expect(toast).not.toHaveBeenCalled();
      expect(view.onClose).toHaveBeenCalled();
    } finally {
      view.unmount();
    }
  });

  it("shows the idempotent toast path for existing promotions", async () => {
    mockPromoteFailure.mockResolvedValue({
      created: false,
      case: { id: "case-2", suite_id: "suite-1" },
    });

    const view = renderDialog();
    try {
      await waitFor(() => {
        expect(mockListRegressionSuites).toHaveBeenCalled();
      });

      const submitButton = Array.from(
        view.container.querySelectorAll("button"),
      ).find((button) => button.textContent === "Promote");

      await act(async () => {
        clickElement(submitButton!);
      });

      await waitFor(() => {
        expect(toast).toHaveBeenCalledWith(
          "Already promoted - open case",
          expect.objectContaining({
            action: expect.objectContaining({ label: "Open case" }),
          }),
        );
      });
      expect(toast.success).not.toHaveBeenCalled();
    } finally {
      view.unmount();
    }
  });

  it("surfaces API errors verbatim", async () => {
    mockPromoteFailure.mockRejectedValue(
      new ApiError(400, "validation_error", "promotion mode unavailable"),
    );

    const view = renderDialog();
    try {
      await waitFor(() => {
        expect(mockListRegressionSuites).toHaveBeenCalled();
      });

      const summary = view.container.querySelector(
        "#promote-failure-summary",
      ) as HTMLTextAreaElement;
      await act(async () => {
        setInputValue(summary, "Updated summary");
      });

      const submitButton = Array.from(
        view.container.querySelectorAll("button"),
      ).find((button) => button.textContent === "Promote");
      await act(async () => {
        clickElement(submitButton!);
      });

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith("promotion mode unavailable");
      });
    } finally {
      view.unmount();
    }
  });

  it("builds the create-suite deep link when no matching suites exist", async () => {
    mockListRegressionSuites.mockResolvedValue({
      items: [],
      total: 0,
      limit: 100,
      offset: 0,
    });

    const view = renderDialog();
    try {
      await waitFor(() => {
        const link = view.container.querySelector("a");
        expect(link?.getAttribute("href")).toBe(
          "/workspaces/ws-1/regression-suites?create=1&sourcePackId=pack-1",
        );
      });
    } finally {
      view.unmount();
    }
  });
});
