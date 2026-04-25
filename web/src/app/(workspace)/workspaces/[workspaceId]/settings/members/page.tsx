import { getServerApiClient } from "@/lib/api/server";
import { requireWorkspaceAdminAccess } from "@/lib/auth/server";
import type { WorkspaceMember } from "@/lib/api/types";
import { WsMembersClient } from "./ws-members-client";
import Link from "next/link";

export default async function WorkspaceMembersPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  const api = await getServerApiClient();

  const [res, adminAccess] = await Promise.all([
    api.get<{ items: WorkspaceMember[]; total: number }>(
      `/v1/workspaces/${workspaceId}/memberships`,
      { params: { limit: 50, offset: 0 } },
    ),
    requireWorkspaceAdminAccess(workspaceId),
  ]);

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <Link
          href={`/workspaces/${workspaceId}/settings`}
          className="text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          Settings
        </Link>
        <span className="text-muted-foreground/40">/</span>
        <h1 className="text-lg font-semibold tracking-tight">Members</h1>
      </div>
      <WsMembersClient
        workspaceId={workspaceId}
        isAdmin
        currentUserId={adminAccess.session.user_id}
        initialMembers={res.items}
        initialTotal={res.total}
      />
    </div>
  );
}
