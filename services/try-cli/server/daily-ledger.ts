/**
 * Durable per-day spend ledger for the free-trial gateway. Backed by Redis so
 * the global daily ceiling survives restarts/redeploys and multiple instances
 * (an in-memory counter would reset on every deploy — useless as a money cap).
 * Falls back to in-memory when REDIS_URL is unset (local dev only).
 */
import type { DailyLedger } from "./gateway.ts";

function todayKey(): string {
  const d = new Date();
  const day = `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}-${String(
    d.getUTCDate(),
  ).padStart(2, "0")}`;
  return `trycli:gw:daily:${day}`;
}

export function createDailyLedger(): DailyLedger {
  const url = process.env.REDIS_URL || process.env.VALKEY_URL;

  if (url) {
    // Bun ships a native Redis client.
    const { RedisClient } = require("bun") as typeof import("bun");
    const client = new RedisClient(url);
    let warned = false;
    const safe = async <T>(fn: () => Promise<T>, fallback: T): Promise<T> => {
      try {
        return await fn();
      } catch (err) {
        if (!warned) {
          warned = true;
          console.error("[gw] redis ledger error — failing closed:", err);
        }
        return fallback;
      }
    };
    return {
      // Fail CLOSED on Redis error: report the ceiling as reached so we never
      // overspend blind. (Returns Infinity so callers see "at/over ceiling".)
      async get() {
        return safe(async () => {
          const v = await client.get(todayKey());
          return v ? parseFloat(v) : 0;
        }, Number.POSITIVE_INFINITY);
      },
      async add(usd) {
        // Fail closed on error (Infinity), matching get(), so an intermittent
        // Redis failure can't silently under-count the daily total.
        return safe(async () => {
          const key = todayKey();
          const total = await client.send("INCRBYFLOAT", [key, String(usd)]);
          await client.send("EXPIRE", [key, "172800"]); // 48h
          return parseFloat(String(total));
        }, Number.POSITIVE_INFINITY);
      },
    };
  }

  console.warn("[try-cli] REDIS_URL not set — daily gateway ceiling is in-memory (dev only)");
  let day = todayKey();
  let total = 0;
  const roll = () => {
    const k = todayKey();
    if (k !== day) {
      day = k;
      total = 0;
    }
  };
  return {
    async get() {
      roll();
      return total;
    },
    async add(usd) {
      roll();
      total += usd;
      return total;
    },
  };
}
