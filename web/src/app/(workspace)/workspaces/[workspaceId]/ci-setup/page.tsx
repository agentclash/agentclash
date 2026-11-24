import { getServerSession, getServerUserMe } from "@/lib/auth/server";
import { CISetupClient } from "./ci-setup-client";

export default async function CISetupPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  let canManageGitHubApp: boolean;
  try {
    const [session, userMe] = await Promise.all([
      getServerSession(),
      getServerUserMe(),
    ]);
    const isWorkspaceAdmin = session.workspace_memberships.some(
      (membership) =>
        membership.workspace_id === workspaceId &&
        membership.role === "workspace_admin",
    );
    const isOwningOrgAdmin = userMe.organizations.some(
      (organization) =>
        organization.role === "org_admin" &&
        organization.workspaces.some((workspace) => workspace.id === workspaceId),
    );
    canManageGitHubApp = isWorkspaceAdmin || isOwningOrgAdmin;
  } catch {
    canManageGitHubApp = false;
  }

  return (
    <CISetupClient
      workspaceId={workspaceId}
      canManageGitHubApp={canManageGitHubApp}
    />
  );
}
