const DEFAULT_RETURN_TO = "/dashboard";
const DEVICE_PATH = "/auth/device";
const GITHUB_SETUP_PATH = "/github/setup";
const INVITE_ORG_PATH_PREFIX = "/invites/organization/";
const INVITE_WORKSPACE_PATH_PREFIX = "/invites/workspace/";
const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const ALLOWED_RETURN_PATHS = new Set([
  DEFAULT_RETURN_TO,
  DEVICE_PATH,
  GITHUB_SETUP_PATH,
]);
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

  if (parsed.pathname.startsWith(INVITE_ORG_PATH_PREFIX)) {
    return buildInviteReturnTo("organization", parsed.pathname);
  }

  if (parsed.pathname.startsWith(INVITE_WORKSPACE_PATH_PREFIX)) {
    return buildInviteReturnTo("workspace", parsed.pathname);
  }

  if (!ALLOWED_RETURN_PATHS.has(parsed.pathname)) {
    return DEFAULT_RETURN_TO;
  }

  if (parsed.pathname === DEVICE_PATH) {
    return buildDeviceReturnTo(parsed.searchParams.get("user_code"));
  }

  if (parsed.pathname === GITHUB_SETUP_PATH) {
    return buildGitHubSetupReturnTo(parsed.searchParams);
  }

  return DEFAULT_RETURN_TO;
}

export function buildInviteReturnTo(
  inviteType: "organization" | "workspace",
  pathname: string,
): string {
  const prefix =
    inviteType === "organization"
      ? INVITE_ORG_PATH_PREFIX
      : INVITE_WORKSPACE_PATH_PREFIX;
  if (!pathname.startsWith(prefix)) {
    return DEFAULT_RETURN_TO;
  }
  const membershipId = pathname.slice(prefix.length);
  if (!UUID_PATTERN.test(membershipId)) {
    return DEFAULT_RETURN_TO;
  }
  return `${prefix}${membershipId}`;
}

export function buildGitHubSetupReturnTo(searchParams: URLSearchParams): string {
  const installationID = searchParams.get("installation_id");
  const state = searchParams.get("state");
  if (
    !installationID?.match(/^\d+$/) ||
    !state?.match(/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/)
  ) {
    return GITHUB_SETUP_PATH;
  }

  const params = new URLSearchParams({
    installation_id: installationID,
    state: state.slice(0, 4096),
  });
  const setupAction = searchParams.get("setup_action");
  if (setupAction?.match(/^[A-Za-z0-9_-]{1,64}$/)) {
    params.set("setup_action", setupAction);
  }
  return `${GITHUB_SETUP_PATH}?${params.toString()}`;
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
