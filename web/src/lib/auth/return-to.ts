const DEFAULT_RETURN_TO = "/dashboard";
const DEVICE_PATH = "/auth/device";
const ALLOWED_RETURN_PATHS = new Set([DEFAULT_RETURN_TO, DEVICE_PATH]);
const RETURN_TO_BASE_URL = "http://agentclash.local";

export function sanitizeReturnTo(raw: string | null | undefined): string {
  if (!raw || !raw.startsWith("/") || raw.startsWith("//")) {
    return DEFAULT_RETURN_TO;
  }

  let parsed: URL;
  try {
    parsed = new URL(raw, RETURN_TO_BASE_URL);
  } catch {
    return DEFAULT_RETURN_TO;
  }

  if (parsed.origin !== RETURN_TO_BASE_URL) {
    return DEFAULT_RETURN_TO;
  }

  if (!ALLOWED_RETURN_PATHS.has(parsed.pathname)) {
    return DEFAULT_RETURN_TO;
  }

  if (parsed.pathname === DEVICE_PATH) {
    return buildDeviceReturnTo(parsed.searchParams.get("user_code"));
  }

  return DEFAULT_RETURN_TO;
}

export function buildDeviceReturnTo(
  rawUserCode: string | null | undefined,
): string {
  const userCode = normalizeDeviceUserCode(rawUserCode);
  if (!userCode) {
    return DEVICE_PATH;
  }

  const params = new URLSearchParams({ user_code: userCode });
  return `${DEVICE_PATH}?${params.toString()}`;
}

export function normalizeDeviceUserCode(
  rawUserCode: string | FormDataEntryValue | null | undefined,
): string {
  if (typeof rawUserCode !== "string") {
    return "";
  }

  const normalized = rawUserCode
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, "")
    .slice(0, 8);

  if (!normalized) {
    return "";
  }

  if (normalized.length <= 4) {
    return normalized;
  }

  return `${normalized.slice(0, 4)}-${normalized.slice(4)}`;
}
