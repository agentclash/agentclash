import { describe, expect, test } from "bun:test";
import { loadConfig } from "../server/config.ts";
import { normalizeIp, requestIdentity } from "../server/trust.ts";
import { createLimits } from "../server/limits.ts";

const production = () => ({
  NODE_ENV: "production", E2B_API_KEY: "synthetic-test-value",
  REDIS_URL: "rediss://localhost:6379", TRY_CLI_GATEWAY_URL: "https://terminal.example.test",
  TRY_CLI_PROXY_SECRET: crypto.randomUUID(), TRY_CLI_CORS_ORIGINS: "https://frontend.example.test",
});

describe("production configuration", () => {
  test("requires dependencies without leaking values", () => {
    const env: Record<string, string> = production();
    expect(loadConfig(env).production).toBe(true);
    for (const key of Object.keys(env).filter(k => k !== "NODE_ENV")) {
      const copy = { ...env }; delete copy[key];
      expect(() => loadConfig(copy)).toThrow(key);
    }
    for (const [key, value] of Object.entries({
      REDIS_URL: "redis://private:secret@localhost", TRY_CLI_GATEWAY_URL: "https://secret@host.test/path",
      TRY_CLI_CORS_ORIGINS: "*", GW_DAILY_CEILING_USD: "NaN", GW_AUTH_MINUTES: "-1",
      TRY_CLI_PROXY_SECRET: "short", NODE_TLS_REJECT_UNAUTHORIZED: "0",
      TRY_CLI_TRUSTED_PROXY_CIDRS: "0.0.0.0/0", PORT: "1.5", GW_CODEX_MODEL: 'bad"model',
    })) {
      try { loadConfig({ ...env, [key]: value }); throw new Error("Expected rejection"); }
      catch (err) { expect(String(err)).toBe(`Error: Invalid or missing configuration: ${key}`); }
    }
    expect(loadConfig({}).redisUrl).toBeUndefined();
    expect(() => loadConfig({ REDIS_TLS_CA_FILE: "/unused" })).toThrow("REDIS_TLS_CA_FILE");
  });
});

test("forwarded identities require frontend secret or exact immediate proxy trust", () => {
  const secret = crypto.randomUUID();
  const config = loadConfig({ TRY_CLI_PROXY_SECRET: secret, TRY_CLI_TRUSTED_PROXY_CIDRS: "127.0.0.1/32" });
  const headers = {
    "x-forwarded-for": "198.51.100.1", "x-agentclash-user": "test-user",
    "x-agentclash-client-ip": "198.51.100.2", "x-trycli-client-ip": "198.51.100.3",
  };
  const req = (extra = {}) => new Request("http://localhost", { headers: { ...headers, ...extra } });
  expect(requestIdentity(req(), "192.0.2.1", config)).toEqual({ ip: "192.0.2.1", tier: "anonymous" });
  expect(requestIdentity(req(), "::ffff:127.0.0.1", config)).toEqual({ ip: "198.51.100.3", tier: "anonymous" });
  expect(requestIdentity(req({ "x-agentclash-proxy-secret": secret }), "192.0.2.1", config)).toEqual({ ip: "198.51.100.2", tier: "authenticated" });
  expect(requestIdentity(req({ "x-agentclash-proxy-secret": "wrong" }), "192.0.2.1", config).tier).toBe("anonymous");
  expect(normalizeIp("198.51.100.1, 192.0.2.1")).toBeUndefined();
  expect(normalizeIp("2001:0db8:0:0::1")).toBe("2001:db8::1");
});

test("development limits make concurrent trial and budget admissions atomic", async () => {
  const limits = createLimits(loadConfig({}));
  expect((await Promise.all(Array.from({ length: 8 }, () => limits.claimTrial("192.0.2.1")))).filter(Boolean)).toHaveLength(1);
  const holds = await Promise.all(Array.from({ length: 8 }, () => limits.reserve(0.05, 0.1)));
  expect(holds.filter(Boolean)).toHaveLength(2);
  await holds[0]!.settle(0);
  await holds[0]!.settle(0); // idempotent; cannot release another request's hold
  expect(await limits.reserve(0.05, 0.1)).not.toBeNull();
  expect(await limits.reserve(0.05, 0.1)).toBeNull();
  limits.close();
  await expect(limits.claimTrial("192.0.2.2")).rejects.toThrow();
});
