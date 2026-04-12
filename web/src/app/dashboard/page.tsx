import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { SessionResponse } from "@/lib/api/types";

/**
 * Route guard: after login the callback sends users here.
 * We check their session and redirect:
 *   - No org memberships → /onboard (first-time user)
 *   - Has workspace memberships → /workspaces/{first workspace id}
 *   - Has org but no workspace → /onboard (shouldn't happen, but safe fallback)
 */
export default async function DashboardPage() {
  const { user, accessToken } = await withAuth();
  if (!user) redirect("/auth/login");

  let session: SessionResponse | null = null;
  let errorMessage: string | null = null;

  try {
    const api = createApiClient(accessToken);
    session = await api.get<SessionResponse>("/v1/auth/session");
  } catch (err) {
    errorMessage = err instanceof Error ? err.message : String(err);
  }

  if (!session) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-lg font-semibold mb-2">
            Unable to load your session
          </h1>
          <p className="text-sm text-muted-foreground mb-4">
            The API server may be unavailable. Please try again.
          </p>
          {errorMessage && (
            <pre className="text-xs text-destructive mb-4 max-w-lg text-left mx-auto bg-card p-3 rounded-lg overflow-auto">
              {errorMessage}
            </pre>
          )}
          <p className="text-xs text-muted-foreground mb-4">
            token: {accessToken ? `yes (${accessToken.length} chars)` : "no"} | api: {process.env.API_URL || process.env.NEXT_PUBLIC_API_URL || "unset"}
          </p>
          <a
            href="/dashboard"
            className="text-sm text-foreground underline underline-offset-4"
          >
            Retry
          </a>
        </div>
      </div>
    );
  }

  // Redirects must be outside try/catch — Next.js redirect() throws internally.
  const isOnboarded = session.organization_memberships.some(
    (m) => m.role === "org_admin",
  );

  if (!isOnboarded) {
    redirect("/onboard");
  }

  const firstWorkspace = session.workspace_memberships[0];
  if (firstWorkspace) {
    redirect(`/workspaces/${firstWorkspace.workspace_id}`);
  }

  redirect("/onboard");
}
