import { AuthenticatedAppProviders } from "@/app/providers";
import { getRequiredInitialAuth, getWorkspaceShellData } from "@/lib/auth/server";
import { Sidebar } from "@/components/app-shell/sidebar";
import { TopBar } from "@/components/app-shell/top-bar";
import { TrialUpgradePrompt } from "@/components/billing/trial-upgrade-prompt";
import { WorkspaceBillingBanner } from "@/components/billing/workspace-billing-banner";
import { ActivationBanner } from "@/components/onboarding/activation-banner";
import { PostHogIdentify } from "@/components/posthog-identify";

export default async function WorkspaceLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  const initialAuth = await getRequiredInitialAuth();
  const {
    user,
    session,
    userMe,
    hasMembership,
    hasOrgAccess,
    orgId,
    orgName,
    orgRole,
    orgSlug,
  } = await getWorkspaceShellData(workspaceId);

  if (!hasMembership && !hasOrgAccess) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-4xl font-semibold mb-2">403</h1>
          <p className="text-sm text-muted-foreground mb-4">
            You don&apos;t have access to this workspace.
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

  return (
    <AuthenticatedAppProviders initialAuth={initialAuth}>
      <PostHogIdentify session={session} />
      <div className="flex h-screen overflow-hidden">
        <Sidebar workspaceId={workspaceId} />
        <div className="flex flex-1 flex-col overflow-hidden">
          <TopBar
            workspaceId={workspaceId}
            organizations={userMe.organizations}
            displayName={user.firstName ? `${user.firstName} ${user.lastName ?? ""}`.trim() : undefined}
            email={user.email ?? undefined}
            avatarUrl={user.profilePictureUrl ?? undefined}
            orgName={orgName}
            orgSlug={orgSlug}
          />
          <TrialUpgradePrompt
            orgId={orgId}
            orgSlug={orgSlug}
            isOrgAdmin={orgRole === "org_admin"}
          />
          <WorkspaceBillingBanner workspaceId={workspaceId} orgSlug={orgSlug} />
          <ActivationBanner workspaceId={workspaceId} />
          <main className="flex-1 overflow-y-auto p-6">{children}</main>
        </div>
      </div>
    </AuthenticatedAppProviders>
  );
}
