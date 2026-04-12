import { describe, it, expect, vi, beforeEach } from "vitest";
import { createApiClient } from "../client";
import { ApiError, NetworkError } from "../errors";

// Mock environment variable
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

function errorResponse(status: number, code: string, message: string) {
  return new Response(
    JSON.stringify({ error: { code, message } }),
    { status, headers: { "Content-Type": "application/json" } },
  );
}

describe("createApiClient", () => {
  it("attaches Bearer token to requests", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ user_id: "123" }));

    const api = createApiClient("my-token");
    await api.get("/v1/auth/session");

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/auth/session",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer my-token",
        }),
      }),
    );
  });

  it("does not attach Authorization header when no token", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({}));

    const api = createApiClient();
    await api.get("/v1/healthz");

    const headers = mockFetch.mock.calls[0][1].headers;
    expect(headers.Authorization).toBeUndefined();
  });

  it("get() returns parsed JSON response", async () => {
    const session = {
      user_id: "abc",
      email: "test@example.com",
      organization_memberships: [],
      workspace_memberships: [],
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(session));

    const api = createApiClient("token");
    const result = await api.get("/v1/auth/session");

    expect(result).toEqual(session);
  });

  it("post() sends JSON body with Content-Type", async () => {
    const responseData = {
      id: "build-1",
      name: "Test Agent",
      slug: "test-agent",
    };
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify(responseData), { status: 201 }),
    );

    const api = createApiClient("token");
    await api.post("/v1/workspaces/ws-1/agent-builds", {
      name: "Test Agent",
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/agent-builds",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ name: "Test Agent" }),
        headers: expect.objectContaining({
          "Content-Type": "application/json",
        }),
      }),
    );
  });

  it("throws ApiError on 4xx/5xx with error envelope", async () => {
    mockFetch.mockResolvedValueOnce(
      errorResponse(409, "already_onboarded", "you already have an organization"),
    );

    const api = createApiClient("token");

    await expect(api.post("/v1/onboarding", {})).rejects.toThrow(ApiError);

    try {
      mockFetch.mockResolvedValueOnce(
        errorResponse(409, "already_onboarded", "you already have an organization"),
      );
      await api.post("/v1/onboarding", {});
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(409);
      expect(apiErr.code).toBe("already_onboarded");
      expect(apiErr.message).toBe("you already have an organization");
    }
  });

  it("throws ApiError on non-JSON error response", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("Internal Server Error", { status: 500 }),
    );

    const api = createApiClient("token");

    await expect(api.get("/v1/broken")).rejects.toThrow(ApiError);
  });

  it("throws NetworkError when fetch fails", async () => {
    mockFetch.mockRejectedValueOnce(new TypeError("Failed to fetch"));

    const api = createApiClient("token");

    await expect(api.get("/v1/auth/session")).rejects.toThrow(NetworkError);
  });

  it("paginated() appends limit and offset params", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ items: [], total: 0, limit: 10, offset: 0 }),
    );

    const api = createApiClient("token");
    await api.paginated("/v1/workspaces/ws-1/runs", {
      limit: 10,
      offset: 20,
    });

    const url = mockFetch.mock.calls[0][0];
    expect(url).toContain("limit=10");
    expect(url).toContain("offset=20");
  });

  it("paginated() uses defaults when not specified", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ items: [], total: 0, limit: 20, offset: 0 }),
    );

    const api = createApiClient("token");
    await api.paginated("/v1/workspaces/ws-1/runs");

    const url = mockFetch.mock.calls[0][0];
    expect(url).toContain("limit=20");
    expect(url).toContain("offset=0");
  });

  it("patch() sends PATCH request", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ id: "v1" }));

    const api = createApiClient("token");
    await api.patch("/v1/agent-build-versions/v1", { agent_kind: "llm_agent" });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("del() sends DELETE request", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const api = createApiClient("token");
    await api.del("/v1/some-resource/123");

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});
