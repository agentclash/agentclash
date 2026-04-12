import { describe, it, expect, vi, beforeEach } from "vitest";
import { createApiClient } from "../client";
import { ApiError } from "../errors";

vi.stubEnv("NEXT_PUBLIC_API_URL", "http://localhost:8080");

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

beforeEach(() => {
  mockFetch.mockReset();
});

function jsonResponse(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("Runs API", () => {
  it("lists runs with pagination", async () => {
    const runsResponse = {
      items: [
        {
          id: "run-1",
          workspace_id: "ws-1",
          challenge_pack_version_id: "cpv-1",
          name: "Run 2026-04-12T00:00:00Z",
          status: "completed",
          execution_mode: "single_agent",
          created_at: "2026-04-12T00:00:00Z",
          updated_at: "2026-04-12T01:00:00Z",
          links: {
            self: "/v1/runs/run-1",
            agents: "/v1/runs/run-1/agents",
          },
        },
        {
          id: "run-2",
          workspace_id: "ws-1",
          challenge_pack_version_id: "cpv-1",
          name: "Comparison Run",
          status: "running",
          execution_mode: "comparison",
          created_at: "2026-04-12T02:00:00Z",
          updated_at: "2026-04-12T02:30:00Z",
          links: {
            self: "/v1/runs/run-2",
            agents: "/v1/runs/run-2/agents",
          },
        },
      ],
      total: 42,
      limit: 20,
      offset: 0,
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(runsResponse));

    const api = createApiClient("token");
    const result = await api.get("/v1/workspaces/ws-1/runs", {
      params: { limit: 20, offset: 0 },
    });

    expect(result).toEqual(runsResponse);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/runs?limit=20&offset=0",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer token",
        }),
      }),
    );

    const items = (result as typeof runsResponse).items;
    expect(items).toHaveLength(2);
    expect(items[0].status).toBe("completed");
    expect(items[1].execution_mode).toBe("comparison");
  });

  it("lists runs with offset for page 2", async () => {
    const page2 = {
      items: [
        {
          id: "run-21",
          workspace_id: "ws-1",
          challenge_pack_version_id: "cpv-1",
          name: "Page 2 Run",
          status: "queued",
          execution_mode: "single_agent",
          created_at: "2026-04-10T00:00:00Z",
          updated_at: "2026-04-10T00:00:00Z",
          links: { self: "/v1/runs/run-21", agents: "/v1/runs/run-21/agents" },
        },
      ],
      total: 42,
      limit: 20,
      offset: 20,
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(page2));

    const api = createApiClient("token");
    await api.get("/v1/workspaces/ws-1/runs", {
      params: { limit: 20, offset: 20 },
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/runs?limit=20&offset=20",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("creates a single-agent run", async () => {
    const createResponse = {
      id: "run-new",
      workspace_id: "ws-1",
      challenge_pack_version_id: "cpv-1",
      status: "queued",
      execution_mode: "single_agent",
      created_at: "2026-04-12T03:00:00Z",
      queued_at: "2026-04-12T03:00:00Z",
      links: {
        self: "/v1/runs/run-new",
        agents: "/v1/runs/run-new/agents",
      },
    };
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify(createResponse), { status: 201 }),
    );

    const api = createApiClient("token");
    const result = await api.post("/v1/runs", {
      workspace_id: "ws-1",
      challenge_pack_version_id: "cpv-1",
      agent_deployment_ids: ["dep-1"],
    });

    expect(result).toEqual(createResponse);

    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.workspace_id).toBe("ws-1");
    expect(body.challenge_pack_version_id).toBe("cpv-1");
    expect(body.agent_deployment_ids).toEqual(["dep-1"]);
  });

  it("creates a comparison run with multiple deployments", async () => {
    const createResponse = {
      id: "run-compare",
      workspace_id: "ws-1",
      challenge_pack_version_id: "cpv-1",
      status: "queued",
      execution_mode: "comparison",
      created_at: "2026-04-12T03:00:00Z",
      queued_at: "2026-04-12T03:00:00Z",
      links: {
        self: "/v1/runs/run-compare",
        agents: "/v1/runs/run-compare/agents",
      },
    };
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify(createResponse), { status: 201 }),
    );

    const api = createApiClient("token");
    const result = await api.post("/v1/runs", {
      workspace_id: "ws-1",
      challenge_pack_version_id: "cpv-1",
      agent_deployment_ids: ["dep-1", "dep-2", "dep-3"],
      name: "My Comparison",
    });

    expect((result as typeof createResponse).execution_mode).toBe("comparison");

    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.agent_deployment_ids).toHaveLength(3);
    expect(body.name).toBe("My Comparison");
  });

  it("handles duplicate deployment IDs error (400)", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: "validation_error",
            message: "agent_deployment_ids contains duplicates",
          },
        }),
        { status: 400 },
      ),
    );

    const api = createApiClient("token");

    try {
      await api.post("/v1/runs", {
        workspace_id: "ws-1",
        challenge_pack_version_id: "cpv-1",
        agent_deployment_ids: ["dep-1", "dep-1"],
      });
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(400);
      expect(apiErr.code).toBe("validation_error");
    }
  });

  it("handles missing deployments error (400)", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: "validation_error",
            message: "agent_deployment_ids is required and must not be empty",
          },
        }),
        { status: 400 },
      ),
    );

    const api = createApiClient("token");

    try {
      await api.post("/v1/runs", {
        workspace_id: "ws-1",
        challenge_pack_version_id: "cpv-1",
        agent_deployment_ids: [],
      });
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as ApiError).status).toBe(400);
    }
  });

  it("gets a single run by ID", async () => {
    const run = {
      id: "run-1",
      workspace_id: "ws-1",
      challenge_pack_version_id: "cpv-1",
      name: "My Run",
      status: "completed",
      execution_mode: "single_agent",
      started_at: "2026-04-12T00:01:00Z",
      finished_at: "2026-04-12T00:05:00Z",
      created_at: "2026-04-12T00:00:00Z",
      updated_at: "2026-04-12T00:05:00Z",
      links: {
        self: "/v1/runs/run-1",
        agents: "/v1/runs/run-1/agents",
      },
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(run));

    const api = createApiClient("token");
    const result = await api.get("/v1/runs/run-1");

    expect(result).toEqual(run);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/runs/run-1",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("lists agents for a run", async () => {
    const agentsResponse = {
      items: [
        {
          id: "agent-1",
          run_id: "run-1",
          lane_index: 0,
          label: "GPT-4o Agent",
          agent_deployment_id: "dep-1",
          agent_deployment_snapshot_id: "snap-1",
          status: "completed",
          started_at: "2026-04-12T00:01:00Z",
          finished_at: "2026-04-12T00:03:00Z",
          created_at: "2026-04-12T00:00:00Z",
          updated_at: "2026-04-12T00:03:00Z",
        },
        {
          id: "agent-2",
          run_id: "run-1",
          lane_index: 1,
          label: "Claude Agent",
          agent_deployment_id: "dep-2",
          agent_deployment_snapshot_id: "snap-2",
          status: "failed",
          started_at: "2026-04-12T00:01:00Z",
          finished_at: "2026-04-12T00:02:00Z",
          failure_reason: "sandbox timeout after 60s",
          created_at: "2026-04-12T00:00:00Z",
          updated_at: "2026-04-12T00:02:00Z",
        },
      ],
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(agentsResponse));

    const api = createApiClient("token");
    const result = await api.get("/v1/runs/run-1/agents");

    expect(result).toEqual(agentsResponse);
    const items = (result as typeof agentsResponse).items;
    expect(items).toHaveLength(2);
    expect(items[0].status).toBe("completed");
    expect(items[1].failure_reason).toBe("sandbox timeout after 60s");
  });

  it("fetches ranking for a completed run", async () => {
    const rankingResponse = {
      state: "ready",
      ranking: {
        run_id: "run-1",
        evaluation_spec_id: "eval-1",
        sort: { field: "composite", direction: "desc", default_order: true },
        winner: {
          run_agent_id: "agent-1",
          strategy: "highest_score",
          status: "determined",
          reason_code: "clear_winner",
        },
        items: [
          {
            rank: 1,
            run_agent_id: "agent-1",
            lane_index: 0,
            label: "GPT-4o Agent",
            status: "completed",
            has_scorecard: true,
            sort_value: 0.92,
            delta_from_top: 0,
            sort_state: "available",
            composite_score: 0.92,
            correctness_score: 0.95,
            reliability_score: 0.88,
            latency_score: 0.9,
            cost_score: 0.85,
          },
          {
            rank: 2,
            run_agent_id: "agent-2",
            lane_index: 1,
            label: "Claude Agent",
            status: "completed",
            has_scorecard: true,
            sort_value: 0.78,
            delta_from_top: -0.14,
            sort_state: "available",
            composite_score: 0.78,
            correctness_score: 0.82,
            reliability_score: 0.75,
            latency_score: 0.8,
            cost_score: 0.72,
          },
        ],
      },
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(rankingResponse));

    const api = createApiClient("token");
    const result = await api.get("/v1/runs/run-1/ranking", {
      params: { sort_by: "composite" },
    });

    expect(result).toEqual(rankingResponse);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/runs/run-1/ranking?sort_by=composite",
      expect.objectContaining({ method: "GET" }),
    );

    const items = (result as typeof rankingResponse).ranking.items;
    expect(items[0].rank).toBe(1);
    expect(items[0].composite_score).toBe(0.92);
    expect(items[1].delta_from_top).toBe(-0.14);
  });

  it("handles pending ranking (202)", async () => {
    const pendingResponse = {
      state: "pending",
      message: "Scoring in progress",
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(pendingResponse));

    const api = createApiClient("token");
    const result = await api.get("/v1/runs/run-1/ranking");

    expect((result as typeof pendingResponse).state).toBe("pending");
  });
});
