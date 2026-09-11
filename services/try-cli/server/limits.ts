import { RedisClient } from "bun";
import { readFileSync } from "node:fs";
import type { Config } from "./config.ts";

export interface BudgetHold { settle(usd: number): Promise<void> }
export interface Limits {
  health(): Promise<void>;
  claimTrial(ip: string): Promise<boolean>;
  reserve(usd: number, ceiling: number): Promise<BudgetHold | null>;
  close(): void;
}

const dayKey = () => `trycli:gw:daily:${new Date().toISOString().slice(0, 10)}`;
const trialKey = (ip: string) => `trycli:trial:used:${ip}`;
const RETENTION = 172800;

// Reserve against the same persisted total used by the old ledger. If the
// process dies, the reservation stays charged. Never blindly retry mutations.
const RESERVE = `
local raw = redis.call('GET', KEYS[1])
local total = tonumber(raw or '0')
if not total or total < 0 then return redis.error_reply('invalid budget') end
if total + tonumber(ARGV[1]) > tonumber(ARGV[2]) then return 0 end
redis.call('INCRBYFLOAT', KEYS[1], ARGV[1])
redis.call('EXPIRE', KEYS[1], ARGV[3], 'NX')
redis.call('SET', KEYS[2], ARGV[1], 'EX', ARGV[3])
return 1`;
const SETTLE = `
local held = redis.call('GET', KEYS[2])
if held == 'done' then return 1 end
if not held then return redis.error_reply('expired reservation') end
local total = tonumber(redis.call('GET', KEYS[1]))
if not total or total < 0 then return redis.error_reply('invalid budget') end
redis.call('SET', KEYS[1], tostring(math.max(0, total + tonumber(ARGV[1]) - tonumber(held))), 'KEEPTTL')
redis.call('SET', KEYS[2], 'done', 'KEEPTTL')
return 1`;

export function createLimits(config: Config): Limits {
  if (!config.redisUrl) {
    if (config.production) throw new Error("Durable limits required");
    return memoryLimits(config.trialSeconds);
  }
  let client: RedisClient;
  try {
    client = new RedisClient(config.redisUrl, {
      connectionTimeout: 2000, idleTimeout: 5000, enableOfflineQueue: false,
      maxRetries: 1,
      tls: config.redisUrl.startsWith("rediss:") ? {
        rejectUnauthorized: true,
        ...(config.redisCaFile ? { ca: readFileSync(config.redisCaFile, "utf8") } : {}),
      } : undefined,
    });
  } catch { throw new Error("Unable to configure durable limits"); }
  // Avoid unhandled connection rejections; health/request calls surface them.
  const connected = client.connect().then(() => true, () => false);
  let closed = false;
  const command = async (name: string, args: string[]) => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    try {
      if (closed) throw new Error();
      return await Promise.race([
        (async () => { await connected; return client.send(name, args); })(),
        new Promise<never>((_, reject) => { timer = setTimeout(() => reject(new Error()), 2500); }),
      ]);
    } catch { throw new Error("Durable limits unavailable"); }
    finally { clearTimeout(timer); }
  };
  return {
    async health() {
      await command("PING", []);
      const value = await command("GET", [dayKey()]);
      if (value !== null && (!Number.isFinite(Number(value)) || Number(value) < 0)) throw new Error("Durable limits unavailable");
    },
    async claimTrial(ip) {
      return await command("SET", [trialKey(ip), "1", "EX", String(config.trialSeconds), "NX"]) === "OK";
    },
    async reserve(usd, ceiling) {
      validAmount(usd); validAmount(ceiling);
      const key = dayKey();
      const hold = `trycli:gw:hold:${crypto.randomUUID()}`;
      if (Number(await command("EVAL", [RESERVE, "2", key, hold, String(usd), String(ceiling), String(RETENTION)])) !== 1) return null;
      return { async settle(actual) {
        validAmount(actual);
        await command("EVAL", [SETTLE, "2", key, hold, String(actual)]);
      } };
    },
    close() { closed = true; client.close(); },
  };
}

function validAmount(value: number) {
  if (!Number.isFinite(value) || value < 0) throw new Error("Invalid budget amount");
}

function memoryLimits(trialSeconds: number): Limits {
  const trials = new Map<string, number>();
  const totals = new Map<string, number>();
  let closed = false;
  const health = async () => { if (closed) throw new Error("Limits closed"); };
  return {
    health,
    async claimTrial(ip) {
      await health();
      const now = Date.now();
      for (const [key, expiry] of trials) if (expiry <= now) trials.delete(key);
      if ((trials.get(ip) ?? 0) > now) return false;
      trials.set(ip, now + trialSeconds * 1000);
      return true;
    },
    async reserve(usd, ceiling) {
      await health(); validAmount(usd); validAmount(ceiling);
      const key = dayKey();
      if ((totals.get(key) ?? 0) + usd > ceiling + 1e-10) return null;
      totals.set(key, (totals.get(key) ?? 0) + usd);
      let settled = false;
      return { async settle(actual) {
        await health(); validAmount(actual);
        if (settled) return;
        settled = true;
        totals.set(key, Math.max(0, (totals.get(key) ?? 0) + actual - usd));
      } };
    },
    close() { closed = true; },
  };
}
