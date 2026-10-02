import { createHash, createHmac, timingSafeEqual } from "node:crypto";
import { sanitizeReturnTo } from "./return-to";

const TTL = 10 * 60;
export const callbackIntentTTL = TTL;
export function callbackIntentCookie(state: string) {
  return `ac-auth-intent-${createHash("sha256").update(state).digest("hex").slice(0, 24)}`;
}
function signature(body: string) {
  const key = process.env.WORKOS_COOKIE_PASSWORD;
  if (!key) throw new Error("Auth cookie signing is not configured");
  return createHmac("sha256", key).update(body).digest("base64url");
}
export function signCallbackIntent(state: string, destination: string, mode: "signin" | "signup") {
  const sanitized = sanitizeReturnTo(destination);
  const boundedDestination = Buffer.byteLength(sanitized, "utf8") <= 2048 ? sanitized : "/dashboard";
  const body = Buffer.from(JSON.stringify({ state: createHash("sha256").update(state).digest("hex"), destination: boundedDestination, mode, expires: Date.now() + TTL * 1000 })).toString("base64url");
  return `${body}.${signature(body)}`;
}
export function readCallbackIntent(state: string, value?: string) {
  if (!state || state.length > 8192 || !value || value.length > 4096) return null;
  try {
    const [body, mac, extra] = value.split(".");
    if (extra || !body || !mac) return null;
    const expected = Buffer.from(signature(body));
    const actual = Buffer.from(mac);
    if (expected.length !== actual.length || !timingSafeEqual(expected, actual)) return null;
    const intent = JSON.parse(Buffer.from(body, "base64url").toString());
    if (intent.state !== createHash("sha256").update(state).digest("hex") || !Number.isFinite(intent.expires) || intent.expires <= Date.now() || intent.expires > Date.now() + TTL * 1000 || !["signin", "signup"].includes(intent.mode)) return null;
    return { destination: sanitizeReturnTo(intent.destination), mode: intent.mode as "signin" | "signup" };
  } catch { return null; }
}
