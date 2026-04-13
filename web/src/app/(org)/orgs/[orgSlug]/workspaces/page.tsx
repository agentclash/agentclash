import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { OrgWorkspace } from "@/lib/api/types";
import { OrgWorkspacesClient } from "./org-workspaces-client";

export default async function OrgWorkspacesPage({
  params,
}: {
  params: Promise<{ orgSlug: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { orgSlug } = await params;

  const api = createApiClient(accessToken);
  const userMe = await api.get<{
    organizations: { id: string; slug: string; role: string }[];
  }>("/v1/users/me");

  const orgRef = userMe.organizations.find((o) => o.slug === orgSlug);
  if (!orgRef) redirect("/dashboard");

  const res = await api.get<{
    items: OrgWorkspace[];
    total: number;
  }>(`/v1/organizations/${orgRef.id}/workspaces`, {
    params: { limit: 50, offset: 0 },
  });

  return (
    <div>
      <h1 className="text-lg font-semibold tracking-tight mb-6">Workspaces</h1>
      <OrgWorkspacesClient
        orgId={orgRef.id}
        isAdmin={orgRef.role === "org_admin"}
        initialWorkspaces={res.items}
        initialTotal={res.total}
      />
    </div>
  );
}
