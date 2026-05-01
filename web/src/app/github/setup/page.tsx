import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type {
  CompleteGitHubInstallationRequest,
  CompleteGitHubInstallationResponse,
} from "@/lib/api/types";
import { sanitizeReturnTo } from "@/lib/auth/return-to";

export const dynamic = "force-dynamic";

interface GitHubSetupState {
  workspace_id?: string;
  return_path?: string;
}

export default async function GitHubSetupPage({
  searchParams,
}: {
  searchParams: Promise<{
    installation_id?: string;
    setup_action?: string;
    state?: string;
  }>;
}) {
  const params = await searchParams;
  const callbackPath = sanitizeReturnTo(buildCallbackPath(params));
  const { user, accessToken } = await withAuth();
  if (!user || !accessToken) {
    redirect(`/auth/login?returnTo=${encodeURIComponent(callbackPath)}`);
  }

  const installationID = Number(params.installation_id);
  const state = params.state ?? "";
  const decoded = decodeGitHubSetupState(state);
  const workspaceID = decoded?.workspace_id;

  if (
    !Number.isSafeInteger(installationID) ||
    installationID <= 0 ||
    !workspaceID
  ) {
    return (
      <GitHubSetupFailure message="GitHub did not return a valid installation." />
    );
  }

  try {
    const api = createApiClient(accessToken);
    await api.post<CompleteGitHubInstallationResponse>(
      `/v1/workspaces/${workspaceID}/github/installations/complete`,
      {
        installation_id: installationID,
        state,
      } satisfies CompleteGitHubInstallationRequest,
    );
  } catch (err) {
    return (
      <GitHubSetupFailure
        message={
          err instanceof Error
            ? err.message
            : "Unable to finish the GitHub connection."
        }
      />
    );
  }

  redirect(safeWorkspaceReturnPath(decoded?.return_path, workspaceID));
}

function buildCallbackPath(params: {
  installation_id?: string;
  setup_action?: string;
  state?: string;
}): string {
  const query = new URLSearchParams();
  if (params.installation_id) {
    query.set("installation_id", params.installation_id);
  }
  if (params.setup_action) query.set("setup_action", params.setup_action);
  if (params.state) query.set("state", params.state);
  const encoded = query.toString();
  return encoded ? `/github/setup?${encoded}` : "/github/setup";
}

function decodeGitHubSetupState(raw: string): GitHubSetupState | null {
  const payload = raw.split(".")[0];
  if (!payload) return null;
  try {
    return JSON.parse(Buffer.from(payload, "base64url").toString("utf8"));
  } catch {
    return null;
  }
}

function safeWorkspaceReturnPath(
  raw: string | undefined,
  workspaceID: string,
): string {
  if (
    raw &&
    raw.startsWith(`/workspaces/${workspaceID}/`) &&
    !raw.startsWith("//")
  ) {
    return raw;
  }
  return `/workspaces/${workspaceID}/agent-harnesses`;
}

function GitHubSetupFailure({ message }: { message: string }) {
  return (
    <main className="flex min-h-screen items-center justify-center px-6">
      <div className="max-w-md text-center">
        <h1 className="text-lg font-semibold">Unable to connect GitHub</h1>
        <p className="mt-2 text-sm text-muted-foreground">{message}</p>
        <a
          href="/dashboard"
          className="mt-5 inline-flex text-sm font-medium underline underline-offset-4"
        >
          Back to dashboard
        </a>
      </div>
    </main>
  );
}
