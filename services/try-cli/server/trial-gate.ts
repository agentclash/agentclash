/**
 * One-shot free-trial gate for anonymous users. Once an IP has started a free
 * trial, it can't start another until the window resets — a reload just tells
 * them to sign in. Redis-backed so it survives restarts; in-memory fallback for
 * local dev. (IP-based: imperfect behind shared NATs, but a low-friction
 * anti-abuse gate, not a hard wall — signing in is always the escape hatch.)
 */
const RESET_HOURS = Number(process.env.GW_TRIAL_RESET_HOURS ?? 24);
const TTL_SECONDS = Math.max(1, Math.floor(RESET_HOURS * 3600));

export interface TrialGate {
  isUsed(ip: string): Promise<boolean>;
  markUsed(ip: string): Promise<void>;
}

function key(ip: string): string {
  return `trycli:trial:used:${ip}`;
}

export function createTrialGate(): TrialGate {
  const url = process.env.REDIS_URL || process.env.VALKEY_URL;

  if (url) {
    const { RedisClient } = require("bun") as typeof import("bun");
    const client = new RedisClient(url);
    return {
      async isUsed(ip) {
        try {
          return Boolean(await client.get(key(ip)));
        } catch (err) {
          // Fail OPEN: a Redis blip shouldn't lock everyone out of the trial.
          console.error("[trial-gate] redis read error — allowing:", err);
          return false;
        }
      },
      async markUsed(ip) {
        try {
          await client.set(key(ip), "1", "EX", TTL_SECONDS);
        } catch (err) {
          console.error("[trial-gate] redis write error:", err);
        }
      },
    };
  }

  console.warn("[try-cli] REDIS_URL not set — trial gate is in-memory (dev only)");
  const used = new Map<string, number>();
  return {
    async isUsed(ip) {
      const exp = used.get(ip);
      if (exp && exp > Date.now()) return true;
      if (exp) used.delete(ip);
      return false;
    },
    async markUsed(ip) {
      used.set(ip, Date.now() + TTL_SECONDS * 1000);
    },
  };
}
