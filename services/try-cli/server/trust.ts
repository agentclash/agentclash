import { timingSafeEqual } from "node:crypto";
import { isIP } from "node:net";
import type { Config } from "./config.ts";

export function normalizeIp(value: string | null | undefined): string | undefined {
  if (!value || !isIP(value.trim())) return;
  const ip = value.trim().toLowerCase();
  if (ip.startsWith("::ffff:") && isIP(ip.slice(7)) === 4) return ip.slice(7);
  // Canonicalize alternate spellings of IPv6 to one rate-limit identity.
  return isIP(ip) === 6 ? new URL(`http://[${ip}]/`).hostname.slice(1, -1) : ip;
}

export function requestIdentity(req: Request, peer: string | undefined, config: Config) {
  const expected = config.proxySecret;
  const supplied = req.headers.get("x-agentclash-proxy-secret");
  const signed = Boolean(expected && supplied && Buffer.byteLength(expected) === Buffer.byteLength(supplied) &&
    timingSafeEqual(Buffer.from(expected), Buffer.from(supplied)));
  const socketIp = normalizeIp(peer);
  const trusted = socketIp && config.trustedProxies.check(socketIp, isIP(socketIp) === 4 ? "ipv4" : "ipv6");
  // Caddy must OVERWRITE this header with its independently resolved client IP.
  // The frontend's separate header is accepted only with the shared secret.
  const ip = (signed ? normalizeIp(req.headers.get("x-agentclash-client-ip")) : undefined) ??
    (trusted ? normalizeIp(req.headers.get("x-trycli-client-ip")) : undefined) ?? socketIp;
  return {
    ip,
    tier: signed && req.headers.get("x-agentclash-user") ? "authenticated" as const : "anonymous" as const,
  };
}
