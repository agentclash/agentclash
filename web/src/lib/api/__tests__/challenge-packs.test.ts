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

describe("Challenge Packs API", () => {
  it("lists challenge packs with nested versions", async () => {
    const packs = {
      items: [
        {
          id: "pack-1",
          name: "Code Quality",
          description: "Tests code quality skills",
          versions: [
            {
              id: "ver-1",
              challenge_pack_id: "pack-1",
              version_number: 1,
              lifecycle_status: "runnable",
              created_at: "2026-04-12T00:00:00Z",
              updated_at: "2026-04-12T00:00:00Z",
            },
          ],
          created_at: "2026-04-12T00:00:00Z",
          updated_at: "2026-04-12T00:00:00Z",
        },
        {
          id: "pack-2",
          name: "Debugging",
          versions: [],
          created_at: "2026-04-12T01:00:00Z",
          updated_at: "2026-04-12T01:00:00Z",
        },
      ],
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(packs));

    const api = createApiClient("token");
    const result = await api.get("/v1/workspaces/ws-1/challenge-packs");

    expect(result).toEqual(packs);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/challenge-packs",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer token",
        }),
      }),
    );

    // Verify nested versions structure
    const items = (result as typeof packs).items;
    expect(items).toHaveLength(2);
    expect(items[0].versions).toHaveLength(1);
    expect(items[0].versions[0].lifecycle_status).toBe("runnable");
    expect(items[1].versions).toHaveLength(0);
  });

  it("validates a challenge pack bundle — valid", async () => {
    const validResponse = { valid: true, errors: [] };
    mockFetch.mockResolvedValueOnce(jsonResponse(validResponse));

    const yaml = "pack:\n  slug: test-pack\n  name: Test\n";
    const api = createApiClient("token");
    const result = await api.postRaw(
      "/v1/workspaces/ws-1/challenge-packs/validate",
      yaml,
      "application/yaml",
    );

    expect(result).toEqual(validResponse);

    // Verify raw YAML body was sent (not JSON-serialized)
    const [url, init] = mockFetch.mock.calls[0];
    expect(url).toBe(
      "http://localhost:8080/v1/workspaces/ws-1/challenge-packs/validate",
    );
    expect(init.method).toBe("POST");
    expect(init.body).toBe(yaml);
    expect(init.headers["Content-Type"]).toBe("application/yaml");
  });

  it("validates a challenge pack bundle — invalid returns errors", async () => {
    const invalidResponse = {
      valid: false,
      errors: [
        { field: "pack.slug", message: "is required" },
        {
          field: "challenges[0].difficulty",
          message: "must be one of easy, medium, hard, expert",
        },
      ],
    };
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify(invalidResponse), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const api = createApiClient("token");

    try {
      await api.postRaw(
        "/v1/workspaces/ws-1/challenge-packs/validate",
        "bad:\n  yaml\n",
        "application/yaml",
      );
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(400);
    }
  });

  it("publishes a challenge pack bundle — success", async () => {
    const publishResponse = {
      challenge_pack_id: "pack-1",
      challenge_pack_version_id: "ver-1",
      evaluation_spec_id: "eval-1",
      input_set_ids: ["is-1", "is-2"],
      bundle_artifact_id: "art-1",
    };
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify(publishResponse), { status: 201 }),
    );

    const yaml = "pack:\n  slug: my-pack\n  name: My Pack\n";
    const api = createApiClient("token");
    const result = await api.postRaw(
      "/v1/workspaces/ws-1/challenge-packs",
      yaml,
      "application/yaml",
    );

    expect(result).toEqual(publishResponse);

    const [, init] = mockFetch.mock.calls[0];
    expect(init.method).toBe("POST");
    expect(init.body).toBe(yaml);
    expect(init.headers["Content-Type"]).toBe("application/yaml");
  });

  it("publishes — handles version conflict (409)", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: "challenge_pack_version_exists",
            message: "version 1 already exists for pack my-pack",
          },
        }),
        { status: 409 },
      ),
    );

    const api = createApiClient("token");

    try {
      await api.postRaw(
        "/v1/workspaces/ws-1/challenge-packs",
        "pack:\n  slug: my-pack\n",
        "application/yaml",
      );
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(409);
      expect(apiErr.code).toBe("challenge_pack_version_exists");
    }
  });

  it("publishes — handles metadata conflict (409)", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: "challenge_pack_metadata_conflict",
            message: "pack name conflicts with existing pack",
          },
        }),
        { status: 409 },
      ),
    );

    const api = createApiClient("token");

    try {
      await api.postRaw(
        "/v1/workspaces/ws-1/challenge-packs",
        "pack:\n  slug: conflict\n",
        "application/yaml",
      );
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(409);
      expect(apiErr.code).toBe("challenge_pack_metadata_conflict");
    }
  });

  it("postRaw sends raw string body without JSON.stringify", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ valid: true, errors: [] }));

    const rawBody = "some: yaml\ncontent: here\n";
    const api = createApiClient("token");
    await api.postRaw("/v1/test", rawBody, "text/plain");

    const [, init] = mockFetch.mock.calls[0];
    // Body should be the raw string, not JSON-serialized
    expect(init.body).toBe(rawBody);
    expect(init.body).not.toBe(JSON.stringify(rawBody));
    expect(init.headers["Content-Type"]).toBe("text/plain");
  });
});
