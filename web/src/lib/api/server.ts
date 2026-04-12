import { withAuth } from "@workos-inc/authkit-nextjs";
import { createApiClient, type ApiClient } from "./client";

/**
 * Create an API client pre-configured with the current user's access token.
 * Server-side only (RSC, server actions, route handlers).
 *
 * Usage:
 *   const api = await getServerApiClient();
 *   const session = await api.get<SessionResponse>("/v1/auth/session");
 */
export async function getServerApiClient(): Promise<ApiClient> {
  const { accessToken } = await withAuth();
  return createApiClient(accessToken);
}
