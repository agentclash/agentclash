"use server";

import { withAuth } from "@workos-inc/authkit-nextjs";
import { createApiClient } from "@/lib/api/client";

interface AuthorizeResult {
  ok: boolean;
  error?: string;
}

// normalizeUserCode strips and re-formats to XXXX-YYYY.
function normalizeUserCode(raw: string): string {
  const clean = raw.toUpperCase().replace(/[^A-Z0-9]/g, "");
  if (clean.length >= 8) {
    return clean.slice(0, 4) + "-" + clean.slice(4, 8);
  }
  return clean;
}

export async function authorizeDevice(
  userCode: string,
): Promise<AuthorizeResult> {
  try {
    const { accessToken } = await withAuth({ ensureSignedIn: true });
    const api = createApiClient(accessToken);
    await api.post("/v1/auth/device/approve", {
      user_code: normalizeUserCode(userCode),
    });
    return { ok: true };
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : "Failed to authorize device",
    };
  }
}
