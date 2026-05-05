import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { WorkspaceMember } from "@/lib/api/types";
import { InviteError } from "../../invite-error";

export default async function WorkspaceInvitePage({
  params,
}: {
  params: Promise<{ membershipId: string }>;
}) {
  const { membershipId } = await params;
  const returnTo = `/invites/workspace/${membershipId}`;
  const { user, accessToken } = await withAuth();

  if (!user) {
    redirect(`/auth/login?returnTo=${encodeURIComponent(returnTo)}`);
  }

  let redirectTarget = "/dashboard";
  try {
    const api = createApiClient(accessToken);
    const accepted = await api.patch<WorkspaceMember>(
      `/v1/workspace-memberships/${membershipId}`,
      { status: "active" },
    );
    redirectTarget = `/workspaces/${accepted.workspace_id}`;
  } catch (err) {
    return (
      <InviteError
        title="Unable to accept invite"
        message={
          err instanceof Error
            ? err.message
            : "The invite may have expired or belongs to another account."
        }
      />
    );
  }

  redirect(redirectTarget);
}
