import { sanitizeReturnTo } from "@/lib/auth/return-to";
import { redirect } from "next/navigation";
import { AuthenticatedAppProviders } from "@/app/providers";
import { getServerApiClient } from "@/lib/api/server";
import { getServerAuth, toInitialAuth } from "@/lib/auth/server";
import type { SessionResponse } from "@/lib/api/types";
import { OnboardingWizard } from "./onboarding-wizard";

export default async function OnboardPage({ searchParams }: { searchParams: Promise<{ returnTo?: string }> }) {
  const destination = sanitizeReturnTo((await searchParams).returnTo);
  const returnTo = destination.startsWith("/vibe-evals?") ? destination : undefined;
  const auth = await getServerAuth();
  if (!auth.user || !auth.accessToken) redirect(`/auth/login?${new URLSearchParams({ returnTo: returnTo || "/dashboard" })}`);
  const initialAuth = toInitialAuth(auth);

  // Check if already onboarded — fetch outside redirect logic.
  let session: SessionResponse | null = null;
  try {
    const api = await getServerApiClient();
    session = await api.get<SessionResponse>("/v1/auth/session");
  } catch {
    // If session fetch fails, let them proceed with onboarding —
    // the POST will return 409 if they're already onboarded.
  }

  if (!session && returnTo) {
    return <main className="flex min-h-screen items-center justify-center"><div className="max-w-md p-6"><h1 className="text-lg font-semibold">Couldn’t verify workspace access</h1><p className="my-4">Your Vibe work is preserved. Retry setup when your account is available.</p><a className="underline" href={`/onboard?${new URLSearchParams({ returnTo })}`}>Retry setup</a><p className="mt-4"><a className="underline" href={returnTo}>Return to your work</a></p></div></main>;
  }

  // Redirects must be outside try/catch — Next.js redirect() throws internally.
  if (session) {
    if (returnTo && session.organization_memberships.length) redirect(returnTo);
    const hasOrg = session.organization_memberships.some(
      (m) => m.role === "org_admin",
    );
    if (hasOrg) {
      const firstWorkspace = session.workspace_memberships[0];
      if (firstWorkspace) {
        redirect(`/workspaces/${firstWorkspace.workspace_id}`);
      }
      redirect("/dashboard");
    }
  }

  return (
    <AuthenticatedAppProviders initialAuth={initialAuth}>
      <OnboardingWizard returnTo={returnTo} />
    </AuthenticatedAppProviders>
  );
}
