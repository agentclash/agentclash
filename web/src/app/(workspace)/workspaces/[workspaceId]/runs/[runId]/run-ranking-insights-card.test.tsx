import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RunRankingInsightsCard } from "./run-ranking-insights-card";

const { mockGetAccessToken, mockCreateApiClient } = vi.hoisted(() => ({
  mockGetAccessToken: vi.fn(),
  mockCreateApiClient: vi.fn(),
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: (...args: unknown[]) => mockCreateApiClient(...args),
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
  element.dispatchEvent(
    new MouseEvent("click", {
      bubbles: true,
      cancelable: true,
    }),
  );
}

function renderCard() {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  act(() => {
    root.render(
      React.createElement(RunRankingInsightsCard, {
        workspaceId: "ws-1",
        run: {
          id: "run-1",
          workspace_id: "ws-1",
          challenge_pack_version_id: "cpv-1",
          official_pack_mode: "full",
          name: "Comparison Run",
          status: "completed",
          execution_mode: "comparison",
          created_at: "2026-04-20T08:00:00Z",
          updated_at: "2026-04-20T08:15:00Z",
          links: {
            self: "/v1/runs/run-1",
            agents: "/v1/runs/run-1/agents",
          },
        },
        ranking: {
          state: "ready",
          ranking: {
            run_id: "run-1",
            evaluation_spec_id: "eval-1",
            sort: {
              field: "composite",
              direction: "desc",
              default_order: true,
            },
            winner: {
              run_agent_id: "agent-1",
              strategy: "weighted_score",
              status: "winner",
              reason_code: "highest_composite",
            },
            items: [
              {
                rank: 1,
                run_agent_id: "agent-1",
                lane_index: 0,
                label: "Alpha",
                status: "completed",
                has_scorecard: true,
                sort_state: "available",
                overall_score: 0.91,
              },
              {
                rank: 2,
                run_agent_id: "agent-2",
                lane_index: 1,
                label: "Beta",
                status: "completed",
                has_scorecard: true,
                sort_state: "available",
                overall_score: 0.84,
              },
            ],
          },
        },
      }),
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

function buildApiMock() {
  const get = vi.fn(async (url: string) => {
    if (url === "/v1/workspaces/ws-1/provider-accounts") {
      return {
        items: [
          {
            id: "pa-1",
            workspace_id: "ws-1",
            provider_key: "openai",
            name: "OpenAI Workspace",
            status: "active",
            created_at: "2026-04-20T08:00:00Z",
            updated_at: "2026-04-20T08:00:00Z",
          },
          {
            id: "pa-2",
            workspace_id: "ws-1",
            provider_key: "anthropic",
            name: "Anthropic Workspace",
            status: "active",
            created_at: "2026-04-20T08:00:00Z",
            updated_at: "2026-04-20T08:00:00Z",
          },
        ],
      };
    }
    if (url === "/v1/workspaces/ws-1/model-aliases") {
      return {
        items: [
          {
            id: "alias-1",
            workspace_id: "ws-1",
            provider_account_id: "pa-1",
            model_catalog_entry_id: "catalog-1",
            alias_key: "gpt-5.4-mini",
            display_name: "GPT-5.4 Mini",
            status: "active",
            created_at: "2026-04-20T08:00:00Z",
            updated_at: "2026-04-20T08:00:00Z",
          },
          {
            id: "alias-2",
            workspace_id: "ws-1",
            provider_account_id: "pa-2",
            model_catalog_entry_id: "catalog-2",
            alias_key: "other-model",
            display_name: "Other Model",
            status: "active",
            created_at: "2026-04-20T08:00:00Z",
            updated_at: "2026-04-20T08:00:00Z",
          },
        ],
      };
    }
    throw new Error(`Unexpected GET ${url}`);
  });

  return {
    get,
    post: vi.fn(),
  };
}

describe("RunRankingInsightsCard", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    mockGetAccessToken.mockReset();
    mockCreateApiClient.mockReset();
    mockGetAccessToken.mockResolvedValue("token");
  });

  it("loads insight controls and renders generated insights", async () => {
    const api = buildApiMock();
    api.post.mockResolvedValue({
      generated_at: "2026-04-20T08:30:00Z",
      grounding_scope: "current_run_only",
      provider_key: "openai",
      provider_model_id: "gpt-5.4-mini",
      recommended_winner: {
        run_agent_id: "agent-1",
        label: "Alpha",
      },
      why_it_won: "Alpha delivered the best overall mix for this run.",
      tradeoffs: ["Beta stayed close on latency."],
      best_for_reliability: {
        run_agent_id: "agent-2",
        label: "Beta",
        reason: "Beta had the strongest reliability score.",
      },
      model_summaries: [
        {
          run_agent_id: "agent-1",
          label: "Alpha",
          strongest_dimension: "correctness",
          weakest_dimension: "latency",
          summary: "Strongest overall performer.",
        },
      ],
      recommended_next_step: "Run a reliability-focused follow-up.",
      confidence_notes: "Confidence is moderate.",
    });
    mockCreateApiClient.mockReturnValue(api);

    const view = renderCard();
    try {
      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith("/v1/workspaces/ws-1/provider-accounts");
        expect(api.get).toHaveBeenCalledWith("/v1/workspaces/ws-1/model-aliases");
      });

      const providerSelect = document.querySelector(
        'select[aria-label="Insight Provider Account"]',
      );
      if (!(providerSelect instanceof HTMLSelectElement)) {
        throw new Error("Insight Provider Account select not found");
      }
      expect(providerSelect.value).toBe("pa-1");

      const modelSelect = document.querySelector(
        'select[aria-label="Insight Model Alias"]',
      );
      if (!(modelSelect instanceof HTMLSelectElement)) {
        throw new Error("Insight Model Alias select not found");
      }
      expect(modelSelect.value).toBe("alias-1");

      const generateButton = Array.from(document.querySelectorAll("button")).find(
        (button) => button.textContent?.includes("Generate insights"),
      );
      if (!generateButton) {
        throw new Error("Generate insights button not found");
      }

      clickElement(generateButton);

      await waitFor(() => {
        expect(api.post).toHaveBeenCalledWith("/v1/runs/run-1/ranking-insights", {
          provider_account_id: "pa-1",
          model_alias_id: "alias-1",
        });
      });

      await waitFor(() => {
        expect(document.body.textContent).toContain("Recommended winner");
        expect(document.body.textContent).toContain("Alpha delivered the best overall mix for this run.");
        expect(document.body.textContent).toContain("Run a reliability-focused follow-up.");
        expect(document.body.textContent).toContain("LLM advisory");
      });
    } finally {
      view.cleanup();
    }
  });

  it("renders a readable error when insight generation fails", async () => {
    const api = buildApiMock();
    api.post.mockRejectedValue(new Error("provider gateway failed"));
    mockCreateApiClient.mockReturnValue(api);

    const view = renderCard();
    try {
      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith("/v1/workspaces/ws-1/provider-accounts");
      });

      const generateButton = Array.from(document.querySelectorAll("button")).find(
        (button) => button.textContent?.includes("Generate insights"),
      );
      if (!generateButton) {
        throw new Error("Generate insights button not found");
      }

      clickElement(generateButton);

      await waitFor(() => {
        expect(document.body.textContent).toContain("provider gateway failed");
      });
    } finally {
      view.cleanup();
    }
  });

  it("clears stale insights when the provider selection changes", async () => {
    const api = buildApiMock();
    api.post.mockResolvedValue({
      generated_at: "2026-04-20T08:30:00Z",
      grounding_scope: "current_run_only",
      provider_key: "openai",
      provider_model_id: "gpt-5.4-mini",
      recommended_winner: {
        run_agent_id: "agent-1",
        label: "Alpha",
      },
      why_it_won: "Alpha delivered the best overall mix for this run.",
      tradeoffs: ["Beta stayed close on latency."],
      model_summaries: [
        {
          run_agent_id: "agent-1",
          label: "Alpha",
          strongest_dimension: "correctness",
          weakest_dimension: "latency",
          summary: "Strongest overall performer.",
        },
      ],
      recommended_next_step: "Run a reliability-focused follow-up.",
      confidence_notes: "Confidence is moderate.",
    });
    mockCreateApiClient.mockReturnValue(api);

    const view = renderCard();
    try {
      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith("/v1/workspaces/ws-1/provider-accounts");
        expect(api.get).toHaveBeenCalledWith("/v1/workspaces/ws-1/model-aliases");
      });

      const generateButton = Array.from(document.querySelectorAll("button")).find(
        (button) => button.textContent?.includes("Generate insights"),
      );
      if (!generateButton) {
        throw new Error("Generate insights button not found");
      }

      clickElement(generateButton);

      await waitFor(() => {
        expect(document.body.textContent).toContain("Recommended winner");
        expect(document.body.textContent).toContain("Generated with openai / gpt-5.4-mini");
      });

      const providerSelect = document.querySelector(
        'select[aria-label="Insight Provider Account"]',
      );
      if (!(providerSelect instanceof HTMLSelectElement)) {
        throw new Error("Insight Provider Account select not found");
      }

      act(() => {
        providerSelect.value = "pa-2";
        providerSelect.dispatchEvent(new Event("change", { bubbles: true }));
      });

      await waitFor(() => {
        expect(document.body.textContent).toContain("No insights yet.");
      });
      expect(document.body.textContent).not.toContain("Recommended winner");
      expect(document.body.textContent).not.toContain("Generated with openai / gpt-5.4-mini");
    } finally {
      view.cleanup();
    }
  });
});
