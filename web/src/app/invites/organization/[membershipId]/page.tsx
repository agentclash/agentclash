import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { OrgMember, UserMeResponse } from "@/lib/api/types";
import { InviteError } from "../../invite-error";

const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export default async function OrganizationInvitePage({
  params,
}: {
  params: Promise<{ membershipId: string }>;
}) {
  const { membershipId } = await params;
  const returnTo = `/invites/organization/${membershipId}`;
  const { user, accessToken } = await withAuth();

  if (!user) {
    redirect(`/auth/login?returnTo=${encodeURIComponent(returnTo)}`);
  }

  const api = createApiClient(accessToken);
  let accepted: OrgMember;
  try {
    const acceptPath = UUID_PATTERN.test(membershipId)
      ? `/v1/organization-memberships/${membershipId}`
      : `/v1/invites/organization/${membershipId}`;
    accepted = await api.patch<OrgMember>(
      acceptPath,
      { status: "active" },
    );
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

  let redirectTarget = "/dashboard";
  try {
    const userMe = await api.get<UserMeResponse>("/v1/users/me");
    const org = userMe.organizations.find(
      (item) => item.id === accepted.organization_id,
    );
    if (org) {
      redirectTarget = `/orgs/${org.slug}/workspaces`;
    }
  } catch {
    // The invite is already accepted; fall back to dashboard if lookup fails.
  }

  redirect(redirectTarget);
}
