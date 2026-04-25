import { cache } from "react";
import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient, type ApiClient } from "@/lib/api/client";
import type { SessionResponse, UserMeResponse } from "@/lib/api/types";

export const getServerAuth = cache(async () => withAuth());

export const getRequiredServerAuth = cache(async () => {
  const auth = await withAuth();
  if (!auth.user || !auth.accessToken) {
    redirect("/auth/login");
  }
  return auth;
});

export const getServerApiClient = cache(async (): Promise<ApiClient> => {
  const { accessToken } = await getRequiredServerAuth();
  return createApiClient(accessToken);
});

export const getServerSession = cache(async (): Promise<SessionResponse> => {
  const api = await getServerApiClient();
  return api.get<SessionResponse>("/v1/auth/session");
});

export const getServerUserMe = cache(async (): Promise<UserMeResponse> => {
  const api = await getServerApiClient();
  return api.get<UserMeResponse>("/v1/users/me");
});

export const getWorkspaceShellData = cache(async (workspaceId: string) => {
  const { user } = await getRequiredServerAuth();
  const [session, userMe] = await Promise.all([
    getServerSession(),
    getServerUserMe(),
  ]);

  const hasMembership = session.workspace_memberships.some(
    (membership) => membership.workspace_id === workspaceId,
  );
  const hasOrgAccess = session.organization_memberships.some(
    (membership) => membership.role === "org_admin",
  );

  let orgName: string | undefined;
  let orgSlug: string | undefined;
  for (const organization of userMe.organizations) {
    if (organization.workspaces.some((workspace) => workspace.id === workspaceId)) {
      orgName = organization.name;
      orgSlug = organization.slug;
      break;
    }
  }

  return {
    user,
    session,
    userMe,
    hasMembership,
    hasOrgAccess,
    orgName,
    orgSlug,
  };
});

export const requireWorkspaceAdminAccess = cache(async (workspaceId: string) => {
  const session = await getServerSession();

  const isWsAdmin = session.workspace_memberships.some(
    (membership) =>
      membership.workspace_id === workspaceId &&
      membership.role === "workspace_admin",
  );
  const isOrgAdmin = session.organization_memberships.some(
    (membership) => membership.role === "org_admin",
  );

  if (!isWsAdmin && !isOrgAdmin) {
    redirect(`/workspaces/${workspaceId}`);
  }

  return {
    session,
    isWsAdmin,
    isOrgAdmin,
  };
});
