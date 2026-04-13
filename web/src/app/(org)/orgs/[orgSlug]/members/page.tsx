import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { OrgMember } from "@/lib/api/types";
import { OrgMembersClient } from "./org-members-client";

export default async function OrgMembersPage({
  params,
}: {
  params: Promise<{ orgSlug: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { orgSlug } = await params;

  const api = createApiClient(accessToken);
  const userMe = await api.get<{
    user_id: string;
    organizations: { id: string; slug: string; role: string }[];
  }>("/v1/users/me");

  const orgRef = userMe.organizations.find((o) => o.slug === orgSlug);
  if (!orgRef) redirect("/dashboard");

  const res = await api.get<{
    items: OrgMember[];
    total: number;
    limit: number;
    offset: number;
  }>(`/v1/organizations/${orgRef.id}/memberships`, {
    params: { limit: 50, offset: 0 },
  });

  return (
    <div>
      <h1 className="text-lg font-semibold tracking-tight mb-6">Members</h1>
      <OrgMembersClient
        orgId={orgRef.id}
        orgSlug={orgSlug}
        isAdmin={orgRef.role === "org_admin"}
        currentUserId={userMe.user_id}
        initialMembers={res.items}
        initialTotal={res.total}
      />
    </div>
  );
}
