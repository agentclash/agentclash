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

describe("Deployment API calls", () => {
  it("lists deployments for a workspace", async () => {
    const deployments = {
      items: [
        {
          id: "dep-1",
          organization_id: "org-1",
          workspace_id: "ws-1",
          current_build_version_id: "ver-1",
          name: "prod-deploy",
          status: "active",
          created_at: "2026-04-12T00:00:00Z",
          updated_at: "2026-04-12T00:00:00Z",
        },
      ],
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(deployments));

    const api = createApiClient("token");
    const result = await api.get("/v1/workspaces/ws-1/agent-deployments");

    expect(result).toEqual(deployments);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/agent-deployments",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer token",
        }),
      }),
    );
  });

  it("creates a deployment", async () => {
    const response = {
      id: "dep-new",
      workspace_id: "ws-1",
      agent_build_id: "build-1",
      current_build_version_id: "ver-1",
      name: "my-deployment",
      slug: "my-deployment",
      deployment_type: "native",
      status: "active",
      created_at: "2026-04-12T00:00:00Z",
      updated_at: "2026-04-12T00:00:00Z",
    };
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify(response), { status: 201 }),
    );

    const api = createApiClient("token");
    const result = await api.post("/v1/workspaces/ws-1/agent-deployments", {
      name: "my-deployment",
      agent_build_id: "build-1",
      build_version_id: "ver-1",
      runtime_profile_id: "profile-1",
    });

    expect(result).toEqual(response);
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.name).toBe("my-deployment");
    expect(body.agent_build_id).toBe("build-1");
    expect(body.build_version_id).toBe("ver-1");
    expect(body.runtime_profile_id).toBe("profile-1");
  });

  it("handles deployment creation errors", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: { code: "validation_error", message: "name is required" },
        }),
        { status: 400 },
      ),
    );

    const api = createApiClient("token");

    try {
      await api.post("/v1/workspaces/ws-1/agent-deployments", {});
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(400);
      expect(apiErr.code).toBe("validation_error");
    }
  });

  it("lists builds for build selector", async () => {
    const builds = {
      items: [
        { id: "b1", name: "Agent A", slug: "agent-a", lifecycle_status: "active" },
        { id: "b2", name: "Agent B", slug: "agent-b", lifecycle_status: "active" },
      ],
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(builds));

    const api = createApiClient("token");
    const result = await api.get("/v1/workspaces/ws-1/agent-builds");

    expect(result).toEqual(builds);
    expect((result as typeof builds).items).toHaveLength(2);
  });

  it("fetches build detail to get ready versions", async () => {
    const build = {
      id: "b1",
      name: "Agent A",
      versions: [
        { id: "v1", version_number: 1, version_status: "ready", agent_kind: "llm_agent" },
        { id: "v2", version_number: 2, version_status: "draft", agent_kind: "llm_agent" },
      ],
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(build));

    const api = createApiClient("token");
    const result = await api.get<typeof build>("/v1/agent-builds/b1");

    const readyVersions = result.versions.filter(
      (v) => v.version_status === "ready",
    );
    expect(readyVersions).toHaveLength(1);
    expect(readyVersions[0].id).toBe("v1");
  });
});
