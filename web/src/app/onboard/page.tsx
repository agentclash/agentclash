import { redirect } from "next/navigation";
import { AuthenticatedAppProviders } from "@/app/providers";
import { getServerApiClient } from "@/lib/api/server";
import { getRequiredServerAuth, toInitialAuth } from "@/lib/auth/server";
import type { SessionResponse } from "@/lib/api/types";
import { OnboardingWizard } from "./onboarding-wizard";

export default async function OnboardPage() {
  const auth = await getRequiredServerAuth();
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

  // Redirects must be outside try/catch — Next.js redirect() throws internally.
  if (session) {
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
      <OnboardingWizard />
    </AuthenticatedAppProviders>
  );
}
