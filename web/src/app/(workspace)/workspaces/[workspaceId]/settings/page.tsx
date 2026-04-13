import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { WorkspaceDetail, SessionResponse } from "@/lib/api/types";
import { WsGeneralSettings } from "./ws-general-settings";

export default async function WorkspaceSettingsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;
  const api = createApiClient(accessToken);

  const [ws, session] = await Promise.all([
    api.get<WorkspaceDetail>(`/v1/workspaces/${workspaceId}/details`),
    api.get<SessionResponse>("/v1/auth/session"),
  ]);

  // Check admin access (workspace_admin or org_admin)
  const isWsAdmin = session.workspace_memberships.some(
    (m) => m.workspace_id === workspaceId && m.role === "workspace_admin",
  );
  const isOrgAdmin = session.organization_memberships.some(
    (m) => m.role === "org_admin",
  );
  if (!isWsAdmin && !isOrgAdmin) {
    redirect(`/workspaces/${workspaceId}`);
  }

  return (
    <div>
      <h1 className="text-lg font-semibold tracking-tight mb-6">
        Workspace Settings
      </h1>
      <WsGeneralSettings workspace={ws} />
    </div>
  );
}
