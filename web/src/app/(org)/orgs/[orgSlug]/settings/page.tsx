import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { Organization } from "@/lib/api/types";
import { OrgGeneralSettings } from "./org-general-settings";

export default async function OrgSettingsPage({
  params,
}: {
  params: Promise<{ orgSlug: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { orgSlug } = await params;

  const api = createApiClient(accessToken);

  // Need to resolve slug → ID. Use /v1/users/me to find the org.
  const userMe = await api.get<{
    organizations: { id: string; slug: string; role: string }[];
  }>("/v1/users/me");

  const orgRef = userMe.organizations.find((o) => o.slug === orgSlug);
  if (!orgRef || orgRef.role !== "org_admin") redirect(`/orgs/${orgSlug}/members`);

  const org = await api.get<Organization>(
    `/v1/organizations/${orgRef.id}`,
  );

  return (
    <div>
      <h1 className="text-lg font-semibold tracking-tight mb-6">
        General Settings
      </h1>
      <OrgGeneralSettings org={org} orgSlug={orgSlug} />
    </div>
  );
}
