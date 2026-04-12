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

const READY_SCORECARD = {
  state: "ready",
  run_agent_status: "completed",
  id: "sc-1",
  run_agent_id: "agent-1",
  run_id: "run-1",
  evaluation_spec_id: "eval-1",
  overall_score: 0.82,
  correctness_score: 0.95,
  reliability_score: 0.75,
  latency_score: 0.85,
  cost_score: 0.65,
  scorecard: {
    run_agent_id: "agent-1",
    evaluation_spec_id: "eval-1",
    status: "complete",
    warnings: [],
    dimensions: {
      correctness: { state: "available", score: 0.95 },
      reliability: { state: "available", score: 0.75, reason: "2 retries" },
      latency: { state: "available", score: 0.85 },
      cost: { state: "available", score: 0.65 },
    },
    validator_summary: { total: 5, available: 5, pass: 4, fail: 1 },
    metric_summary: { total: 3, available: 3 },
  },
  created_at: "2026-04-13T00:00:00Z",
  updated_at: "2026-04-13T00:01:00Z",
};

describe("Scorecards API", () => {
  it("fetches a ready scorecard", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse(READY_SCORECARD));

    const api = createApiClient("token");
    const result = await api.get("/v1/scorecards/agent-1");

    expect(result).toEqual(READY_SCORECARD);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/scorecards/agent-1",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer token",
        }),
      }),
    );

    const sc = result as typeof READY_SCORECARD;
    expect(sc.state).toBe("ready");
    expect(sc.overall_score).toBe(0.82);
    expect(sc.correctness_score).toBe(0.95);
    expect(sc.reliability_score).toBe(0.75);
    expect(sc.latency_score).toBe(0.85);
    expect(sc.cost_score).toBe(0.65);
  });

  it("parses scorecard dimensions correctly", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse(READY_SCORECARD));

    const api = createApiClient("token");
    const result = (await api.get(
      "/v1/scorecards/agent-1",
    )) as typeof READY_SCORECARD;

    const dims = result.scorecard.dimensions;
    expect(dims.correctness.state).toBe("available");
    expect(dims.correctness.score).toBe(0.95);
    expect(dims.reliability.reason).toBe("2 retries");
    expect(dims.latency.state).toBe("available");
  });

  it("parses validator and metric summaries", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse(READY_SCORECARD));

    const api = createApiClient("token");
    const result = (await api.get(
      "/v1/scorecards/agent-1",
    )) as typeof READY_SCORECARD;

    expect(result.scorecard.validator_summary.total).toBe(5);
    expect(result.scorecard.validator_summary.pass).toBe(4);
    expect(result.scorecard.validator_summary.fail).toBe(1);
    expect(result.scorecard.metric_summary.total).toBe(3);
    expect(result.scorecard.metric_summary.available).toBe(3);
  });

  it("handles pending scorecard (202) with allowedStatuses", async () => {
    const pendingResponse = {
      state: "pending",
      message: "Evaluation in progress",
      run_agent_status: "evaluating",
      id: "",
      run_agent_id: "agent-1",
      run_id: "run-1",
      evaluation_spec_id: "",
      scorecard: {},
      created_at: "",
      updated_at: "",
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(pendingResponse, 202));

    const api = createApiClient("token");
    const result = await api.get("/v1/scorecards/agent-1", {
      allowedStatuses: [202, 409],
    });

    const sc = result as typeof pendingResponse;
    expect(sc.state).toBe("pending");
    expect(sc.message).toBe("Evaluation in progress");
  });

  it("handles errored scorecard (409) with allowedStatuses", async () => {
    const erroredResponse = {
      state: "errored",
      message: "Agent failed before evaluation could complete",
      run_agent_status: "failed",
      id: "",
      run_agent_id: "agent-1",
      run_id: "run-1",
      evaluation_spec_id: "",
      scorecard: {},
      created_at: "",
      updated_at: "",
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(erroredResponse, 409));

    const api = createApiClient("token");
    const result = await api.get("/v1/scorecards/agent-1", {
      allowedStatuses: [202, 409],
    });

    const sc = result as typeof erroredResponse;
    expect(sc.state).toBe("errored");
    expect(sc.message).toBe(
      "Agent failed before evaluation could complete",
    );
  });

  it("throws ApiError for 404 without allowedStatuses", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: "not_found",
            message: "run agent not found",
          },
        }),
        { status: 404 },
      ),
    );

    const api = createApiClient("token");

    try {
      await api.get("/v1/scorecards/nonexistent");
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(404);
      expect(apiErr.code).toBe("not_found");
    }
  });

  it("handles scorecard with partial evaluation status", async () => {
    const partial = {
      ...READY_SCORECARD,
      scorecard: {
        ...READY_SCORECARD.scorecard,
        status: "partial",
        warnings: ["latency metric skipped: no timing data"],
        dimensions: {
          ...READY_SCORECARD.scorecard.dimensions,
          latency: { state: "unavailable" as const },
        },
      },
      latency_score: undefined,
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(partial));

    const api = createApiClient("token");
    const result = (await api.get(
      "/v1/scorecards/agent-1",
    )) as typeof partial;

    expect(result.scorecard.status).toBe("partial");
    expect(result.scorecard.warnings).toContain(
      "latency metric skipped: no timing data",
    );
    expect(result.scorecard.dimensions.latency.state).toBe("unavailable");
    expect(result.latency_score).toBeUndefined();
  });

  it("handles scorecard with dimension errors", async () => {
    const withError = {
      ...READY_SCORECARD,
      scorecard: {
        ...READY_SCORECARD.scorecard,
        dimensions: {
          ...READY_SCORECARD.scorecard.dimensions,
          cost: {
            state: "error" as const,
            reason: "cost provider returned invalid data",
          },
        },
      },
      cost_score: undefined,
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(withError));

    const api = createApiClient("token");
    const result = (await api.get(
      "/v1/scorecards/agent-1",
    )) as typeof withError;

    expect(result.scorecard.dimensions.cost.state).toBe("error");
    expect(result.scorecard.dimensions.cost.reason).toBe(
      "cost provider returned invalid data",
    );
    expect(result.cost_score).toBeUndefined();
  });
});
