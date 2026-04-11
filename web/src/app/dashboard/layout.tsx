import { redirect } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { getUserMe, ApiError } from "@/lib/api/client";
import { AuthProvider } from "@/lib/auth/context";
import type { UserMeResponse } from "@/lib/api/types";
import { DashboardShell } from "./dashboard-shell";

/**
 * Protected layout for all /dashboard/* routes.
 *
 * Reads the session (no writes — Server Components can't write cookies
 * in Next.js 16), fetches user data, and wraps children in AuthProvider.
 * Cookie mutations (refresh, destroy) are delegated to Route Handlers.
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

  // If WorkOS token is expired, redirect to the refresh Route Handler
  // which can write cookies, then come back here.
  if (session.mode === "workos" && session.refreshToken) {
    const now = Math.floor(Date.now() / 1000);
    if (now >= session.expiresAt) {
      redirect("/auth/refresh?returnTo=/dashboard");
    }
  }

  // Fetch user profile from the backend (source of truth).
  let user: UserMeResponse;
  try {
    user = await getUserMe();
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      // Session exists but backend rejected it — clear cookie via
      // the sign-out Route Handler.
      redirect("/auth/sign-out");
    }
    throw err;
  }

  return (
    <AuthProvider user={user}>
      <DashboardShell user={user}>{children}</DashboardShell>
    </AuthProvider>
  );
}
