import { ApiError } from "@/lib/api/errors";
import { buildDeviceReturnTo } from "@/lib/auth/return-to";

export function buildDeviceStatusPath(
  userCode: string,
  updates: Record<string, string>,
): string {
  const url = new URL(buildDeviceReturnTo(userCode), "http://agentclash.local");
  for (const [key, value] of Object.entries(updates)) {
    url.searchParams.set(key, value);
  }
  return `${url.pathname}${url.search}`;
}

export function mapDeviceActionError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 401) {
      return "auth_required";
    }
    const message = error.message.toLowerCase();
    if (message.includes("expired")) {
      return "expired_code";
    }
    if (message.includes("required")) {
      return "missing_code";
    }
    if (message.includes("not found")) {
      return "invalid_code";
    }
  }

  return "request_failed";
}
