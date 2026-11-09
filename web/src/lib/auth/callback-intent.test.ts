import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { callbackIntentCookie, readCallbackIntent, signCallbackIntent } from "./callback-intent";

beforeEach(() => { vi.stubEnv("WORKOS_COOKIE_PASSWORD", "local-test-signing-key-which-is-never-used-in-production"); vi.useFakeTimers(); });
afterEach(() => { vi.useRealTimers(); vi.unstubAllEnvs(); });

it("keeps concurrent callback destinations separate and rejects cross-attempt replay", () => {
  const first = signCallbackIntent("state-a", "/auth/device?user_code=ABCD1234", "signin");
  const second = signCallbackIntent("state-b", "/vibe-evals", "signup");
  expect(callbackIntentCookie("state-a")).not.toBe(callbackIntentCookie("state-b"));
  expect(readCallbackIntent("state-a", first)).toEqual({ destination: "/auth/device?user_code=ABCD-1234", mode: "signin" });
  expect(readCallbackIntent("state-b", second)).toEqual({ destination: "/vibe-evals", mode: "signup" });
  expect(readCallbackIntent("state-b", first)).toBeNull();
});

it("rejects modified and expired intent and sanitizes an external destination", () => {
  const signed = signCallbackIntent("state", "https://evil.invalid", "signin");
  expect(readCallbackIntent("state", signed)).toEqual({ destination: "/dashboard", mode: "signin" });
  expect(readCallbackIntent("state", signed.replace(/\.[^.]/, match => match === ".x" ? ".y" : ".x"))).toBeNull();
  const oversized = signCallbackIntent("state", `/github/setup?installation_id=1&state=${"a".repeat(2100)}.b`, "signin");
  expect(Buffer.byteLength(oversized)).toBeLessThan(4096);
  expect(readCallbackIntent("state", oversized)?.destination).toBe("/dashboard");
  vi.advanceTimersByTime(10 * 60 * 1000);
  expect(readCallbackIntent("state", signed)).toBeNull();
});
