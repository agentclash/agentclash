"use server";

import { withAuth } from "@workos-inc/authkit-nextjs";
import { createApiClient } from "@/lib/api/client";

interface ApproveResult {
  redirectUrl?: string;
  error?: string;
}

export async function approveCLILogin(
  port: number,
  state: string,
): Promise<ApproveResult> {
  try {
    const { accessToken } = await withAuth({ ensureSignedIn: true });
    const api = createApiClient(accessToken);
    const result = await api.post<{ id: string; token: string }>(
      "/v1/auth/cli-tokens",
      { name: "CLI Login" },
    );

    const redirectUrl = `http://127.0.0.1:${port}/callback?token=${encodeURIComponent(result.token)}&state=${encodeURIComponent(state)}`;
    return { redirectUrl };
  } catch (err) {
    return {
      error:
        err instanceof Error ? err.message : "Failed to create CLI token",
    };
  }
}
