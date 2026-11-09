const ATTRIBUTION_KEY = "agentclash:analytics-attribution:v1";
const SESSION_KEY_PREFIX = "agentclash:analytics-session:";
const ATTRIBUTION_TTL_MS = 30 * 24 * 60 * 60 * 1_000;
const SAFE_UTM_KEYS = [
  "utm_source",
  "utm_medium",
  "utm_campaign",
  "utm_content",
  "utm_term",
] as const;
const EMAIL_LIKE_VALUE = /[\w.%+-]+@[\w.-]+\.[a-z]{2,}/i;

interface FirstTouch {
  entry_path: string;
  referrer_hostname?: string;
  utm?: Record<string, string>;
  expires_at: number;
}

interface AcquisitionCTA {
  cta_id: string;
  expires_at: number;
}

interface AttributionState {
  first_touch?: FirstTouch;
  acquisition_cta?: AcquisitionCTA;
}

function safePathname(pathname: string): string {
  const pathOnly = pathname.split(/[?#]/, 1)[0] || "/";
  const normalized = pathOnly.startsWith("/") ? pathOnly : `/${pathOnly}`;
  return normalized
    .replace(
      /\/(invites\/(?:organization|workspace)|public\/shares|agent-tryouts\/shared|share)\/[^/]+/gi,
      "/$1/{token}",
    )
    .replace(/\/auth\/callback(?:\/[^/]*)?/gi, "/auth/callback");
}

function hostname(raw: string): string {
  if (!raw) return "";
  try {
    return new URL(raw).hostname.toLowerCase();
  } catch {
    return "";
  }
}

function safeTag(value: unknown, limit: number): string {
  if (typeof value !== "string") return "";
  const tag = value.trim();
  return tag.length <= limit && /^[a-zA-Z0-9._-]+$/.test(tag) && !EMAIL_LIKE_VALUE.test(tag) ? tag : "";
}

function validExpiry(value: unknown, now: number): value is number {
  return typeof value === "number" && Number.isFinite(value) && value > now && value <= now + ATTRIBUTION_TTL_MS;
}

function readState(now = Date.now()): AttributionState {
  if (typeof window === "undefined") return {};
  try {
    const parsed = JSON.parse(window.localStorage.getItem(ATTRIBUTION_KEY) ?? "{}");
    const state: AttributionState = {};
    const touch = parsed?.first_touch;
    if (touch && validExpiry(touch.expires_at, now) && typeof touch.entry_path === "string") {
      const utm: Record<string, string> = {};
      for (const key of SAFE_UTM_KEYS) {
        const value = safeTag(touch.utm?.[key], 200);
        if (value) utm[key] = value;
      }
      const referrer = typeof touch.referrer_hostname === "string" &&
        /^[a-z0-9.-]{1,253}$/.test(touch.referrer_hostname) ? touch.referrer_hostname : "";
      state.first_touch = {
        entry_path: safePathname(touch.entry_path), expires_at: touch.expires_at,
        ...(referrer ? { referrer_hostname: referrer } : {}),
        ...(Object.keys(utm).length ? { utm } : {}),
      };
    }
    const cta = parsed?.acquisition_cta;
    const ctaId = safeTag(cta?.cta_id, 160);
    if (ctaId && validExpiry(cta?.expires_at, now)) {
      state.acquisition_cta = { cta_id: ctaId, expires_at: cta.expires_at };
    }
    // Storage is untrusted input, including previously written legacy state.
    writeState(state);
    return state;
  } catch {
    return {};
  }
}

function writeState(state: AttributionState): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(ATTRIBUTION_KEY, JSON.stringify(state));
  } catch {
    // Analytics is best effort; storage can be unavailable in privacy modes.
  }
}

export function recordFirstTouch(): void {
  if (typeof window === "undefined") return;
  const now = Date.now();
  const state = readState(now);
  if (state.first_touch) return;

  const utm: Record<string, string> = {};
  const params = new URLSearchParams(window.location.search);
  for (const key of SAFE_UTM_KEYS) {
    const value = params.get(key);
    const safeValue = safeTag(value, 200);
    if (safeValue && !EMAIL_LIKE_VALUE.test(safeValue)) utm[key] = safeValue;
  }
  const referrer = hostname(document.referrer);
  state.first_touch = {
    entry_path: safePathname(window.location.pathname),
    ...(referrer ? { referrer_hostname: referrer } : {}),
    ...(Object.keys(utm).length > 0 ? { utm } : {}),
    expires_at: now + ATTRIBUTION_TTL_MS,
  };
  writeState(state);
}

export function rememberAcquisitionCTA(ctaId: string): void {
  ctaId = safeTag(ctaId, 160);
  if (!ctaId) return;
  const now = Date.now();
  const state = readState(now);
  state.acquisition_cta = {
    cta_id: ctaId.slice(0, 160),
    expires_at: now + ATTRIBUTION_TTL_MS,
  };
  writeState(state);
}

export function attributionSetOnce(): Record<string, string> {
  const state = readState();
  const properties: Record<string, string> = {};
  if (state.first_touch) {
    properties.acquisition_entry_path = state.first_touch.entry_path;
    const source = state.first_touch.utm?.utm_source;
    properties.acquisition_channel = source === "claude" || source === "codex" ? source : "unknown";
    if (state.first_touch.referrer_hostname) {
      properties.acquisition_referrer_hostname =
        state.first_touch.referrer_hostname;
    }
    for (const [key, value] of Object.entries(state.first_touch.utm ?? {})) {
      properties[`acquisition_${key}`] = value;
    }
  }
  if (state.acquisition_cta) {
    properties.acquisition_cta_id = state.acquisition_cta.cta_id;
  }
  return properties;
}

export function analyticsSessionGuardKey(userId: string, sessionId: string): string {
  return `${SESSION_KEY_PREFIX}${userId}:${sessionId}`;
}

export function clearAnalyticsAttribution(): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(ATTRIBUTION_KEY);
    const toRemove: string[] = [];
    for (let index = 0; index < window.sessionStorage.length; index += 1) {
      const key = window.sessionStorage.key(index);
      if (key?.startsWith(SESSION_KEY_PREFIX)) toRemove.push(key);
    }
    for (const key of toRemove) window.sessionStorage.removeItem(key);
  } catch {
    // Analytics cleanup is best effort in browsers that disable storage.
  }
}

export const attributionStorageKeyForTests = ATTRIBUTION_KEY;
