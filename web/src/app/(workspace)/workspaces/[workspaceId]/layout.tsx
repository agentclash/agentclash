import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { SessionResponse, UserMeResponse } from "@/lib/api/types";
import { Sidebar } from "@/components/app-shell/sidebar";
import { TopBar } from "@/components/app-shell/top-bar";

export default async function WorkspaceLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ workspaceId: string }>;
}) {
  const { user, accessToken } = await withAuth();
  if (!user) redirect("/auth/login");

  const { workspaceId } = await params;

  let session: SessionResponse | null = null;
  let userMe: UserMeResponse | null = null;

  try {
    const api = createApiClient(accessToken);
    [session, userMe] = await Promise.all([
      api.get<SessionResponse>("/v1/auth/session"),
      api.get<UserMeResponse>("/v1/users/me"),
    ]);
  } catch {
    redirect("/auth/login");
  }

  // Validate workspace access
  const hasMembership = session.workspace_memberships.some(
    (m) => m.workspace_id === workspaceId,
  );
  // Also check implicit access via org_admin
  const hasOrgAccess = session.organization_memberships.some(
    (m) => m.role === "org_admin",
  );

  if (!hasMembership && !hasOrgAccess) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-4xl font-semibold mb-2">403</h1>
          <p className="text-sm text-muted-foreground mb-4">
            You don&apos;t have access to this workspace.
          </p>
          <a
            href="/dashboard"
            className="text-sm text-foreground underline underline-offset-4"
          >
            Go to dashboard
          </a>
        </div>
      </div>
    );
  }

  // Find org name for the current workspace
  let orgName: string | undefined;
  for (const org of userMe.organizations) {
    if (org.workspaces.some((w) => w.id === workspaceId)) {
      orgName = org.name;
      break;
    }
  }

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar workspaceId={workspaceId} />
      <div className="flex flex-1 flex-col overflow-hidden">
        <TopBar
          workspaceId={workspaceId}
          organizations={userMe.organizations}
          displayName={user.firstName ? `${user.firstName} ${user.lastName ?? ""}`.trim() : undefined}
          email={user.email ?? undefined}
          avatarUrl={user.profilePictureUrl ?? undefined}
          orgName={orgName}
        />
        <main className="flex-1 overflow-y-auto p-6">{children}</main>
      </div>
    </div>
  );
}
