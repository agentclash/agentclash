import { beforeEach, describe, expect, it, vi } from "vitest";

import { createApiClient } from "../client";
import { listRunFailures } from "../failure-reviews";

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

describe("failure review API", () => {
  it("serializes the failure cluster filter", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ items: [], clusters: [] }));

    const api = createApiClient("token");
    await listRunFailures(api, "ws-1", "run-1", {
      failureClusterKey: "frc-test",
      limit: 25,
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/runs/run-1/failures?failure_cluster_key=frc-test&limit=25",
      expect.objectContaining({ method: "GET" }),
    );
  });
});
