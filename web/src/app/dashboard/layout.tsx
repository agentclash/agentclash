import { redirect } from "next/navigation";
import { getSession, destroySession, createWorkOSSession } from "@/lib/auth/session";
import { getUserMe, ApiError } from "@/lib/api/client";
import { AuthProvider } from "@/lib/auth/context";
import type { UserMeResponse } from "@/lib/api/types";
import { DashboardShell } from "./dashboard-shell";

/**
 * Protected layout for all /dashboard/* routes.
 *
 * Validates the session, refreshes tokens if needed, fetches user data,
 * and wraps children in AuthProvider so client components can access
 * the authenticated user via useAuth().
 */
export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await getSession();
  if (!session) {
    redirect("/auth/login");
  }

  // In WorkOS mode, attempt token refresh if expired.
  if (session.mode === "workos") {
    const now = Math.floor(Date.now() / 1000);
    if (now >= session.expiresAt && session.refreshToken) {
      try {
        const { getWorkOSClient } = await import("@/lib/auth/workos");
        const { getWorkOSConfig } = await import("@/lib/auth/config");
        const workos = getWorkOSClient();
        const { clientId } = getWorkOSConfig();
        const refreshed =
          await workos.userManagement.authenticateWithRefreshToken({
            clientId,
            refreshToken: session.refreshToken,
          });
        await createWorkOSSession(
          refreshed.accessToken,
          refreshed.refreshToken,
          3600,
        );
      } catch {
        await destroySession();
        redirect("/auth/login?error=session_expired");
      }
    }
  }

  // Fetch user profile from the backend (source of truth).
  let user: UserMeResponse;
  try {
    user = await getUserMe();
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      await destroySession();
      redirect("/auth/login?error=session_expired");
    }
    throw err;
  }

  return (
    <AuthProvider user={user}>
      <DashboardShell user={user}>{children}</DashboardShell>
    </AuthProvider>
  );
}
