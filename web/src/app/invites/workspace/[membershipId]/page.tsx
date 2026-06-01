import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { WorkspaceMember } from "@/lib/api/types";
import { InviteError } from "../../invite-error";

const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

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
    const acceptPath = UUID_PATTERN.test(membershipId)
      ? `/v1/workspace-memberships/${membershipId}`
      : `/v1/invites/workspace/${membershipId}`;
    const accepted = await api.patch<WorkspaceMember>(
      acceptPath,
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
