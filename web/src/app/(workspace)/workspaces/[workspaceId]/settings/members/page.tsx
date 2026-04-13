import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { WorkspaceMember, SessionResponse } from "@/lib/api/types";
import { WsMembersClient } from "./ws-members-client";
import Link from "next/link";

export default async function WorkspaceMembersPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;
  const api = createApiClient(accessToken);

  const [res, session] = await Promise.all([
    api.get<{ items: WorkspaceMember[]; total: number }>(
      `/v1/workspaces/${workspaceId}/memberships`,
      { params: { limit: 50, offset: 0 } },
    ),
    api.get<SessionResponse>("/v1/auth/session"),
  ]);

  const isWsAdmin = session.workspace_memberships.some(
    (m) => m.workspace_id === workspaceId && m.role === "workspace_admin",
  );
  const isOrgAdmin = session.organization_memberships.some(
    (m) => m.role === "org_admin",
  );
  const isAdmin = isWsAdmin || isOrgAdmin;

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
        isAdmin={isAdmin}
        currentUserId={session.user_id}
        initialMembers={res.items}
        initialTotal={res.total}
      />
    </div>
  );
}
