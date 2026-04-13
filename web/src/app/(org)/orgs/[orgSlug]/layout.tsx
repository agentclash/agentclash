import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect, notFound } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { UserMeResponse, SessionResponse } from "@/lib/api/types";
import { OrgSettingsSidebar } from "./org-settings-sidebar";
import { OrgProvider } from "./org-context";

export default async function OrgLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ orgSlug: string }>;
}) {
  const { user, accessToken } = await withAuth();
  if (!user) redirect("/auth/login");

  const { orgSlug } = await params;

  let userMe: UserMeResponse;
  let session: SessionResponse;
  try {
    const api = createApiClient(accessToken);
    [userMe, session] = await Promise.all([
      api.get<UserMeResponse>("/v1/users/me"),
      api.get<SessionResponse>("/v1/auth/session"),
    ]);
  } catch {
    redirect("/auth/login");
  }

  const org = userMe.organizations.find((o) => o.slug === orgSlug);
  if (!org) notFound();

  const isAdmin = org.role === "org_admin";

  return (
    <OrgProvider
      value={{
        orgId: org.id,
        orgSlug: org.slug,
        orgName: org.name,
        isAdmin,
        currentUserId: session.user_id,
      }}
    >
      <div className="flex min-h-screen">
        <OrgSettingsSidebar
          orgSlug={orgSlug}
          orgName={org.name}
          isAdmin={isAdmin}
        />
        <main className="flex-1 overflow-y-auto p-6 max-w-4xl">
          {children}
        </main>
      </div>
    </OrgProvider>
  );
}
