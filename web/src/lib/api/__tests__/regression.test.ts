import { beforeEach, describe, expect, it, vi } from "vitest";

import { createApiClient } from "../client";
import {
  buildPromotionOverrides,
  captureProductionFailure,
  defaultPromotionSeverityForFailure,
  listRegressionSuites,
  promoteFailure,
} from "../regression";

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

describe("Regression API helpers", () => {
  it("lists regression suites with pagination params", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        items: [],
        total: 0,
        limit: 50,
        offset: 10,
      }),
    );

    const api = createApiClient("token");
    await listRegressionSuites(api, "ws-1", { limit: 50, offset: 10 });

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/regression-suites?limit=50&offset=10",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer token",
        }),
      }),
    );
  });

  it("posts the promote failure payload and exposes created status", async () => {
    const promotedCase = {
      id: "case-1",
      suite_id: "suite-1",
      workspace_id: "ws-1",
      title: "Filesystem regression",
      description: "",
      status: "active",
      severity: "blocking",
      promotion_mode: "full_executable",
      source_challenge_pack_version_id: "cpv-1",
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
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(promotedCase, 201));

    const api = createApiClient("token");
    const result = await promoteFailure(
      api,
      "ws-1",
      "run-1",
      "challenge-1",
      {
        run_agent_id: "agent-1",
        suite_id: "suite-1",
        promotion_mode: "full_executable",
        title: "Filesystem regression",
        failure_summary: "Attempted forbidden write",
        severity: "blocking",
      },
    );

    expect(result).toEqual({
      case: promotedCase,
      created: true,
    });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/runs/run-1/failures/challenge-1/promote",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          run_agent_id: "agent-1",
          suite_id: "suite-1",
          promotion_mode: "full_executable",
          title: "Filesystem regression",
          failure_summary: "Attempted forbidden write",
          severity: "blocking",
        }),
      }),
    );
  });

  it("posts production failures to a regression suite", async () => {
    const capturedCase = {
      id: "case-1",
      suite_id: "suite-1",
      workspace_id: "ws-1",
      title: "Production incident",
      description: "",
      status: "proposed",
      severity: "warning",
      promotion_mode: "output_only",
      source_challenge_pack_version_id: "cpv-1",
      source_challenge_identity_id: "challenge-1",
      source_case_key: "prod-incident-123",
      evidence_tier: "hosted_black_box",
      failure_class: "tool_argument_error",
      failure_summary: "Agent emitted an invalid tool argument.",
      payload_snapshot: {},
      expected_contract: {},
      metadata: { origin: "production_failure" },
      created_at: "2026-05-05T00:00:00Z",
      updated_at: "2026-05-05T00:00:00Z",
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(capturedCase, 201));

    const api = createApiClient("token");
    const result = await captureProductionFailure(api, "ws-1", "suite-1", {
      source_challenge_pack_version_id: "cpv-1",
      source_challenge_identity_id: "challenge-1",
      source_case_key: "prod-incident-123",
      title: "Production incident",
      failure_summary: "Agent emitted an invalid tool argument.",
      failure_class: "tool_argument_error",
      payload_snapshot: { ticket: "example" },
      metadata: { incident_id: "INC-123" },
    });

    expect(result).toEqual(capturedCase);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/regression-suites/suite-1/production-failures",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          source_challenge_pack_version_id: "cpv-1",
          source_challenge_identity_id: "challenge-1",
          source_case_key: "prod-incident-123",
          title: "Production incident",
          failure_summary: "Agent emitted an invalid tool argument.",
          failure_class: "tool_argument_error",
          payload_snapshot: { ticket: "example" },
          metadata: { incident_id: "INC-123" },
        }),
      }),
    );
  });

  it("omits empty override groups", () => {
    expect(
      buildPromotionOverrides({
        judgeThresholdOverrides: {},
        assertionToggles: {},
      }),
    ).toBeUndefined();

    expect(
      buildPromotionOverrides({
        judgeThresholdOverrides: {
          "policy.filesystem": 0.9,
        },
        assertionToggles: {
          "capture.files": true,
        },
      }),
    ).toEqual({
      judge_threshold_overrides: {
        "policy.filesystem": 0.9,
      },
      assertion_toggles: {
        "capture.files": true,
      },
    });
  });

  it("mirrors backend default severity rules", () => {
    expect(defaultPromotionSeverityForFailure("policy_violation")).toBe(
      "blocking",
    );
    expect(defaultPromotionSeverityForFailure("sandbox_failure")).toBe(
      "blocking",
    );
    expect(defaultPromotionSeverityForFailure("incorrect_final_output")).toBe(
      "warning",
    );
  });
});
