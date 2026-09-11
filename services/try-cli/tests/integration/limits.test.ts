import { expect, test } from "bun:test";
import { Redis } from "ioredis";
import { readFileSync } from "node:fs";
import { loadConfig } from "../../server/config.ts";
import { createLimits } from "../../server/limits.ts";

const url = process.env.TRY_CLI_TEST_REDIS_URL;
const integration = url ? test : test.skip;
export function redisConfig() {
  if (!url || !["localhost", "127.0.0.1"].includes(new URL(url).hostname)) throw new Error("Isolated loopback Redis required");
  return loadConfig({ REDIS_URL: url, REDIS_TLS_CA_FILE: process.env.TRY_CLI_TEST_CA_FILE, GW_TRIAL_RESET_HOURS: String(1/3600) });
}

integration("verified TLS Redis persists atomic admission and spending across client/server restart", async () => {
  const config = redisConfig();
  const admin = new Redis(config.redisUrl!, { lazyConnect: true, tls: { rejectUnauthorized: true, ca: readFileSync(config.redisCaFile!, "utf8") } });
  await admin.connect();
  const key = `trycli:gw:daily:${new Date().toISOString().slice(0,10)}`;
  // This DB belongs solely to the disposable test container.
  await admin.call("FLUSHDB");
  const first = createLimits(config);
  try {
    await first.health();
    expect((await Promise.all(Array.from({ length: 10 }, () => first.claimTrial("192.0.2.1")))).filter(Boolean)).toHaveLength(1);
    const ttl = Number(await admin.call("TTL", "trycli:trial:used:192.0.2.1")); expect(ttl).toBe(1);
    const holds = await Promise.all(Array.from({ length: 10 }, () => first.reserve(0.05, 0.1)));
    expect(holds.filter(Boolean)).toHaveLength(2);
    await holds[0]!.settle(0.01); await holds[0]!.settle(0.01);
    expect(Number(await admin.get(key))).toBeCloseTo(0.06);
    first.close();
    const second = createLimits(config);
    try {
      await second.health();
      expect(await second.claimTrial("192.0.2.1")).toBe(false);
      expect(await second.reserve(0.05, 0.1)).toBeNull();
      await Bun.sleep(1100); expect(await second.claimTrial("192.0.2.1")).toBe(true);
      // A read or denied admission must not extend an existing day's retention.
      await admin.call("EXPIRE", key, "60");
      expect(await second.reserve(0.05, 0.1)).toBeNull();
      expect(Number(await admin.call("TTL", key))).toBeLessThanOrEqual(60);
    } finally { second.close(); }
    admin.disconnect();
    const name = process.env.TRY_CLI_TEST_CONTAINER!;
    if (!/^trycli-isolated-[a-f0-9]+$/.test(name)) throw new Error("Disposable container required");
    const restart = Bun.spawn(["docker", "restart", name], { stdout: "ignore", stderr: "ignore" });
    expect(await restart.exited).toBe(0);
    await Bun.sleep(1000);
    const third = createLimits(config);
    try { await third.health(); expect(await third.reserve(0.05, 0.1)).toBeNull(); }
    finally { third.close(); }
  } finally { first.close(); admin.disconnect(); }
});

integration("TLS certificate errors and corrupt persisted counters fail closed", async () => {
  const cfg = redisConfig();
  const bad = createLimits({ ...cfg, redisCaFile: undefined });
  try { await expect(bad.health()).rejects.toThrow("unavailable"); await expect(bad.claimTrial("192.0.2.8")).rejects.toThrow("unavailable"); }
  finally { bad.close(); }
  const wrongHost = new URL(cfg.redisUrl!); wrongHost.hostname = "127.0.0.1";
  const mismatch = createLimits({ ...cfg, redisUrl: wrongHost.toString() });
  try { await expect(mismatch.health()).rejects.toThrow("unavailable"); }
  finally { mismatch.close(); }
  const admin = new Redis(cfg.redisUrl!, { lazyConnect: true, tls: { ca: readFileSync(cfg.redisCaFile!, "utf8"), rejectUnauthorized: true } });
  await admin.connect();
  const key = `trycli:gw:daily:${new Date().toISOString().slice(0,10)}`;
  const limits = createLimits(cfg);
  try {
    await admin.set(key, "corrupt");
    await expect(limits.health()).rejects.toThrow("unavailable");
    await expect(limits.reserve(0.05, 5)).rejects.toThrow("unavailable");
    await admin.del(key);
  } finally { limits.close(); admin.disconnect(); }
});
