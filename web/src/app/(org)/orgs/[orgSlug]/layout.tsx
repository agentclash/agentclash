import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { UserMeResponse } from "@/lib/api/types";
import { OrgSettingsSidebar } from "./org-settings-sidebar";

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
  try {
    const api = createApiClient(accessToken);
    userMe = await api.get<UserMeResponse>("/v1/users/me");
  } catch {
    redirect("/auth/login");
  }

  const org = userMe.organizations.find((o) => o.slug === orgSlug);
  if (!org) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-4xl font-semibold mb-2">404</h1>
          <p className="text-sm text-muted-foreground mb-4">
            Organization not found.
          </p>
          <a
            href="/dashboard"
            className="text-sm text-foreground underline underline-offset-4"
          >
            Go to dashboard
          </a>
        </div>
      </div>
    );
  }

  const isAdmin = org.role === "org_admin";

  return (
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
  );
}
