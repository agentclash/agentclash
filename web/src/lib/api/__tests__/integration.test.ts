import { describe, it, expect } from "vitest";
import { createApiClient } from "../client";
import { ApiError } from "../errors";

/**
 * Integration smoke tests that hit the real backend.
 * Skip these in CI unless the backend is running.
 *
 * Run with: API_URL=http://localhost:8080 npx vitest run --reporter=verbose src/lib/api/__tests__/integration.test.ts
 */

const API_URL = process.env.API_URL || process.env.NEXT_PUBLIC_API_URL;
// Only run if INTEGRATION_TESTS=1 is explicitly set — avoids false failures in CI
const canRun = process.env.INTEGRATION_TESTS === "1" && !!API_URL;

describe.skipIf(!canRun)("Backend integration smoke tests", () => {
  it("healthz returns ok", async () => {
    const res = await fetch(`${API_URL}/healthz`);
    const body = await res.json();
    expect(body).toEqual({ ok: true, service: "api-server" });
  });

  it("unauthenticated session request returns 401 ApiError", async () => {
    const api = createApiClient(); // no token
    try {
      await api.get("/v1/auth/session");
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(401);
      expect(apiErr.code).toBe("unauthorized");
    }
  });

  it("backend error envelope matches ApiErrorResponse type shape", async () => {
    const res = await fetch(`${API_URL}/v1/auth/session`);
    expect(res.status).toBe(401);

    const body = await res.json();
    expect(body).toHaveProperty("error");
    expect(body.error).toHaveProperty("code");
    expect(body.error).toHaveProperty("message");
    expect(typeof body.error.code).toBe("string");
    expect(typeof body.error.message).toBe("string");
  });
});
