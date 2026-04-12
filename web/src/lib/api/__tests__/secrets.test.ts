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

describe("Workspace Secrets API", () => {
  it("lists secrets — returns metadata only, no values", async () => {
    const secrets = {
      items: [
        {
          key: "OPENAI_API_KEY",
          created_at: "2026-04-12T00:00:00Z",
          updated_at: "2026-04-12T00:00:00Z",
          created_by: "user-1",
        },
        {
          key: "DB_URL",
          created_at: "2026-04-12T00:00:00Z",
          updated_at: "2026-04-12T01:00:00Z",
        },
      ],
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(secrets));

    const api = createApiClient("token");
    const result = await api.get("/v1/workspaces/ws-1/secrets");

    expect(result).toEqual(secrets);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/secrets",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer token",
        }),
      }),
    );
    // Verify no "value" field in response items
    const items = (result as typeof secrets).items;
    for (const item of items) {
      expect(item).not.toHaveProperty("value");
      expect(item).not.toHaveProperty("encrypted_value");
    }
  });

  it("upserts a secret via PUT with key in URL", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const api = createApiClient("token");
    await api.put("/v1/workspaces/ws-1/secrets/MY_SECRET", {
      value: "super-secret-value",
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/secrets/MY_SECRET",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ value: "super-secret-value" }),
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          Authorization: "Bearer token",
        }),
      }),
    );
  });

  it("deletes a secret via DELETE", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const api = createApiClient("token");
    await api.del("/v1/workspaces/ws-1/secrets/OLD_KEY");

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/workspaces/ws-1/secrets/OLD_KEY",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("handles invalid secret key error (400)", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: "invalid_secret_key",
            message:
              "secret key must match [A-Za-z_][A-Za-z0-9_]* and be 1..128 characters",
          },
        }),
        { status: 400 },
      ),
    );

    const api = createApiClient("token");

    try {
      await api.put("/v1/workspaces/ws-1/secrets/123-bad-key", {
        value: "test",
      });
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(400);
      expect(apiErr.code).toBe("invalid_secret_key");
    }
  });

  it("handles secret not found on delete (404)", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: "secret_not_found",
            message: "secret does not exist",
          },
        }),
        { status: 404 },
      ),
    );

    const api = createApiClient("token");

    try {
      await api.del("/v1/workspaces/ws-1/secrets/GONE");
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(404);
      expect(apiErr.code).toBe("secret_not_found");
    }
  });

  it("put method sends correct HTTP method", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const api = createApiClient("token");
    await api.put("/v1/test", { data: true });

    expect(mockFetch.mock.calls[0][1].method).toBe("PUT");
  });
});
