import { getServerApiClient } from "@/lib/api/server";
import { requireWorkspaceAdminAccess } from "@/lib/auth/server";
import type { WorkspaceDetail } from "@/lib/api/types";
import { WsGeneralSettings } from "./ws-general-settings";

export default async function WorkspaceSettingsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  const api = await getServerApiClient();

  const [ws] = await Promise.all([
    api.get<WorkspaceDetail>(`/v1/workspaces/${workspaceId}/details`),
    requireWorkspaceAdminAccess(workspaceId),
  ]);

  return (
    <div>
      <h1 className="text-lg font-semibold tracking-tight mb-6">
        Workspace Settings
      </h1>
      <WsGeneralSettings workspace={ws} />
    </div>
  );
}
