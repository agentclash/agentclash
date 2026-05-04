import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  FailureReviewClusterSummary,
  FailureReviewItem,
  ListRunFailuresResponse,
  RunAgent,
} from "@/lib/api/types";

import { FailuresClient } from "./failures-client";

const {
  mockReplace,
  mockGetAccessToken,
  mockCreateApiClient,
  mockListRunFailures,
  searchState,
} = vi.hoisted(() => ({
  mockReplace: vi.fn(),
  mockGetAccessToken: vi.fn(),
  mockCreateApiClient: vi.fn(),
  mockListRunFailures: vi.fn(),
  searchState: { params: new URLSearchParams() },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mockReplace }),
  useSearchParams: () => searchState.params,
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: (...args: unknown[]) => mockCreateApiClient(...args),
}));

vi.mock("@/lib/api/failure-reviews", () => ({
  listRunFailures: (...args: unknown[]) => mockListRunFailures(...args),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) =>
    React.createElement("a", { href, ...props }, children),
}));

vi.mock("./failure-detail-drawer", () => ({
  FailureDetailDrawer: () => null,
}));

vi.mock("./promote-failure-dialog", () => ({
  PromoteFailureDialog: () => null,
}));

const agents: RunAgent[] = [
  {
    id: "agent-1",
    run_id: "run-1",
    lane_index: 0,
    label: "Alpha",
    agent_deployment_id: "deployment-1",
    agent_deployment_snapshot_id: "snapshot-1",
    status: "completed",
    started_at: "2026-04-22T12:00:05Z",
    finished_at: "2026-04-22T12:01:05Z",
    created_at: "2026-04-22T12:00:00Z",
    updated_at: "2026-04-22T12:01:05Z",
  },
];

function makeItem(overrides: Partial<FailureReviewItem> = {}): FailureReviewItem {
  return {
    run_id: "run-1",
    run_agent_id: "agent-1",
    challenge_identity_id: "challenge-identity-1",
    challenge_key: "challenge-a",
    case_key: "case-a",
    item_key: "item-a",
    failure_fingerprint: "fingerprint-a",
    failure_cluster_key: "cluster-a",
    failure_state: "failed",
    failed_dimensions: ["correctness"],
    failed_checks: ["capture.files"],
    failure_class: "policy_violation",
    headline: "Filesystem write regression",
    detail: "The model attempted a forbidden file write.",
    recommended_action: "Add a regression case for filesystem access.",
    promotable: true,
    promotion_mode_available: ["full_executable", "output_only"],
    replay_step_refs: [],
    artifact_refs: [],
    judge_refs: [],
    metric_refs: [],
    evidence_tier: "native_structured",
    severity: "blocking",
    ...overrides,
  };
}

function makeCluster(
  overrides: Partial<FailureReviewClusterSummary> = {},
): FailureReviewClusterSummary {
  return {
    failure_cluster_key: "cluster-a",
    representative_failure_fingerprint: "fingerprint-a",
    count: 2,
    promotable_count: 1,
    severity: "blocking",
    failure_state: "failed",
    failure_class: "policy_violation",
    evidence_tier: "native_structured",
    challenge_keys: ["challenge-a", "challenge-b"],
    case_keys: ["case-a", "case-b", "case-c", "case-d"],
    run_agent_ids: ["agent-1"],
    headline: "Filesystem write failures cluster together",
    recommended_action: "Promote one representative filesystem regression.",
    ...overrides,
  };
}

function makePage(
  overrides: Partial<ListRunFailuresResponse> = {},
): ListRunFailuresResponse {
  return {
    items: [makeItem()],
    clusters: [makeCluster()],
    ...overrides,
  };
}

function renderClient(
  props: Partial<React.ComponentProps<typeof FailuresClient>> = {},
) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  const render = (
    nextProps: Partial<React.ComponentProps<typeof FailuresClient>> = {},
  ) => {
    act(() => {
      root.render(
        React.createElement(FailuresClient, {
          workspaceId: "ws-1",
          runId: "run-1",
          runName: "Run One",
          agents,
          initialPage: makePage(),
          initialLimit: 50,
          sourceChallengePackId: "pack-1",
          sourceChallengePackName: "Source Pack",
          ...props,
          ...nextProps,
        }),
      );
    });
  };

  render();

  return {
    container,
    render,
    cleanup() {
      act(() => {
        root.unmount();
      });
      container.remove();
    },
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

function clickElement(element: Element) {
  element.dispatchEvent(
    new MouseEvent("click", {
      bubbles: true,
      cancelable: true,
    }),
  );
}

describe("FailuresClient", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    searchState.params = new URLSearchParams();
    mockReplace.mockReset();
    mockGetAccessToken.mockReset();
    mockCreateApiClient.mockReset();
    mockListRunFailures.mockReset();
    mockGetAccessToken.mockResolvedValue("token-123");
    mockCreateApiClient.mockReturnValue({ client: true });
  });

  it("renders failure cluster rollups from the initial page", () => {
    const view = renderClient();
    try {
      expect(view.container.textContent).toContain("Failure clusters");
      expect(view.container.textContent).toContain("2 failures");
      expect(view.container.textContent).toContain("1 promotable");
      expect(view.container.textContent).toContain(
        "Filesystem write failures cluster together",
      );
      expect(view.container.textContent).toContain("challenge-a, challenge-b");
      expect(view.container.textContent).toContain(
        "case-a, case-b, case-c +1",
      );
    } finally {
      view.cleanup();
    }
  });

  it("refreshes cluster rollups when filters change", async () => {
    mockListRunFailures.mockResolvedValue(
      makePage({
        items: [
          makeItem({
            challenge_key: "challenge-filtered",
            case_key: "case-filtered",
            item_key: "item-filtered",
            failure_class: "tool_selection_error",
            headline: "Wrong tool selected",
          }),
        ],
        clusters: [
          makeCluster({
            failure_cluster_key: "cluster-filtered",
            count: 1,
            promotable_count: 1,
            failure_class: "tool_selection_error",
            headline: "Filtered tool selection failures",
            challenge_keys: ["challenge-filtered"],
            case_keys: ["case-filtered"],
          }),
        ],
      }),
    );

    const view = renderClient();
    try {
      searchState.params = new URLSearchParams("severity=blocking");
      view.render();

      await waitFor(() => {
        expect(mockListRunFailures).toHaveBeenCalledWith(
          { client: true },
          "ws-1",
          "run-1",
          expect.objectContaining({
            limit: 50,
            severity: "blocking",
          }),
        );
      });

      await waitFor(() => {
        expect(view.container.textContent).toContain(
          "Filtered tool selection failures",
        );
      });
      expect(view.container.textContent).not.toContain(
        "Filesystem write failures cluster together",
      );
      expect(view.container.textContent).toContain("1 failure");
      expect(view.container.textContent).toContain("challenge-filtered");
    } finally {
      view.cleanup();
    }
  });

  it("sets the cluster filter from a cluster rollup", async () => {
    mockListRunFailures.mockResolvedValue(
      makePage({
        items: [
          makeItem({
            failure_cluster_key: "cluster-a",
          }),
        ],
        clusters: [
          makeCluster({
            failure_cluster_key: "cluster-a",
            count: 1,
            promotable_count: 1,
            challenge_keys: ["challenge-a"],
            case_keys: ["case-a"],
          }),
        ],
      }),
    );

    const view = renderClient();
    try {
      const clusterButton = Array.from(
        view.container.querySelectorAll("button"),
      ).find((button) =>
        button.textContent?.includes(
          "Filesystem write failures cluster together",
        ),
      );
      expect(clusterButton).toBeTruthy();

      act(() => {
        clickElement(clusterButton!);
      });

      expect(mockReplace).toHaveBeenCalledWith(
        "/workspaces/ws-1/runs/run-1/failures?cluster=cluster-a",
        { scroll: false },
      );

      searchState.params = new URLSearchParams("cluster=cluster-a");
      view.render();

      await waitFor(() => {
        expect(mockListRunFailures).toHaveBeenCalledWith(
          { client: true },
          "ws-1",
          "run-1",
          expect.objectContaining({
            failureClusterKey: "cluster-a",
            limit: 50,
          }),
        );
      });

      await waitFor(() => {
        const activeClusterButton = view.container.querySelector(
          'button[aria-pressed="true"]',
        );
        expect(activeClusterButton).toBeTruthy();
      });

      mockReplace.mockReset();
      const activeClusterButton = view.container.querySelector(
        'button[aria-pressed="true"]',
      );
      act(() => {
        clickElement(activeClusterButton!);
      });

      expect(mockReplace).toHaveBeenCalledWith(
        "/workspaces/ws-1/runs/run-1/failures",
        { scroll: false },
      );
    } finally {
      view.cleanup();
    }
  });
});
